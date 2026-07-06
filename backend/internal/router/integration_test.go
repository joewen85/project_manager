package router

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"project-manager/backend/internal/ai"
	"project-manager/backend/internal/config"
	"project-manager/backend/internal/database"
	"project-manager/backend/internal/handler"
	"project-manager/backend/internal/seed"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestRouter(t *testing.T) *httptest.Server {
	return setupTestRouterWithHandler(t, nil)
}

func setupTestRouterWithHandler(t *testing.T, configure func(*handler.Handler)) *httptest.Server {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err = database.Migrate(db); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
	if err = seed.Run(db); err != nil {
		t.Fatalf("seed failed: %v", err)
	}

	cfg := config.Config{
		JWTSecret:        "test-secret",
		UploadDir:        t.TempDir(),
		UploadPublicBase: "/static/uploads",
	}
	h := handler.New(db, cfg)
	if configure != nil {
		configure(h)
	}
	engine := New(cfg, h)
	return httptest.NewServer(engine)
}

func loginAndToken(t *testing.T, serverURL string) string {
	t.Helper()
	payload := map[string]string{"username": "admin", "password": "admin123"}
	body, _ := json.Marshal(payload)
	resp, err := http.Post(serverURL+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login status expected 200 got %d", resp.StatusCode)
	}
	var result map[string]any
	if err = json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode login response failed: %v", err)
	}
	token, _ := result["token"].(string)
	if token == "" {
		t.Fatalf("empty token")
	}
	return token
}

func requestJSON(t *testing.T, method, url, token string, payload any) (*http.Response, map[string]any) {
	t.Helper()
	var body io.Reader
	if payload != nil {
		raw, _ := json.Marshal(payload)
		body = bytes.NewReader(raw)
	}
	req, _ := http.NewRequest(method, url, body)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	resp.Body.Close()
	return resp, out
}

func requestMultipartFile(t *testing.T, method, url, token, fieldName, fileName string, content []byte) (*http.Response, map[string]any) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fileWriter, err := writer.CreateFormFile(fieldName, fileName)
	if err != nil {
		t.Fatalf("create form file failed: %v", err)
	}
	if _, err = fileWriter.Write(content); err != nil {
		t.Fatalf("write form file failed: %v", err)
	}
	if err = writer.Close(); err != nil {
		t.Fatalf("close writer failed: %v", err)
	}

	req, _ := http.NewRequest(method, url, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("multipart request failed: %v", err)
	}
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	resp.Body.Close()
	return resp, out
}

type multipartUploadFile struct {
	FileName     string
	RelativePath string
	Content      []byte
}

type streamingAIClient struct{}

func (streamingAIClient) Chat(context.Context, []ai.Message) (string, error) {
	return "同步模型正文", nil
}

func (streamingAIClient) ChatStream(_ context.Context, _ []ai.Message, onDelta func(string) error) (string, error) {
	for _, delta := range []string{"流", "式", "正文"} {
		if err := onDelta(delta); err != nil {
			return "", err
		}
	}
	return "流式正文", nil
}

func requestMultipartFiles(t *testing.T, method, url, token, fieldName string, files []multipartUploadFile) (*http.Response, map[string]any) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, file := range files {
		fileWriter, err := writer.CreateFormFile(fieldName, file.FileName)
		if err != nil {
			t.Fatalf("create form file failed: %v", err)
		}
		if _, err = fileWriter.Write(file.Content); err != nil {
			t.Fatalf("write form file failed: %v", err)
		}
		if file.RelativePath != "" {
			if err = writer.WriteField("relativePaths", file.RelativePath); err != nil {
				t.Fatalf("write relativePaths failed: %v", err)
			}
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer failed: %v", err)
	}

	req, _ := http.NewRequest(method, url, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("multipart request failed: %v", err)
	}
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	resp.Body.Close()
	return resp, out
}

func loginWithCredentials(t *testing.T, serverURL, username, password string) (int, map[string]any) {
	t.Helper()
	raw, _ := json.Marshal(map[string]string{
		"username": username,
		"password": password,
	})
	resp, err := http.Post(serverURL+"/api/v1/auth/login", "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func permissionCodeMap(t *testing.T, serverURL, token string) map[string]uint {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, serverURL+"/api/v1/system/rbac/permissions", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("query permissions failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("query permissions status expected 200 got %d", resp.StatusCode)
	}
	var permissions []map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&permissions)
	codeToID := map[string]uint{}
	for _, permission := range permissions {
		code, _ := permission["code"].(string)
		id, _ := permission["id"].(float64)
		codeToID[code] = uint(id)
	}
	return codeToID
}

func TestFunctionalAPIRouteAliasesReusePermissions(t *testing.T) {
	ts := setupTestRouter(t)
	defer ts.Close()

	adminToken := loginAndToken(t, ts.URL)
	for _, item := range []struct {
		path string
		name string
	}{
		{path: "/api/v1/workbench/tasks/me", name: "workbench tasks"},
		{path: "/api/v1/workbench/notifications", name: "workbench notifications"},
		{path: "/api/v1/portfolio/projects", name: "portfolio projects"},
		{path: "/api/v1/delivery/tasks", name: "delivery tasks"},
		{path: "/api/v1/insights/stats/dashboard", name: "insights dashboard"},
		{path: "/api/v1/integrations/webhooks", name: "integration webhooks"},
		{path: "/api/v1/settings/tags", name: "settings tags"},
		{path: "/api/v1/system/users", name: "system users"},
	} {
		resp, body := requestJSON(t, http.MethodGet, ts.URL+item.path, adminToken, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s alias expected 200 got %d, body=%v", item.name, resp.StatusCode, body)
		}
	}

	codeToID := permissionCodeMap(t, ts.URL, adminToken)
	roleResp, roleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/rbac/roles", adminToken, map[string]any{
		"name":          "portfolio-reader-only",
		"description":   "functional route alias permission probe",
		"permissionIds": []uint{codeToID["projects.read"]},
	})
	if roleResp.StatusCode != http.StatusCreated {
		t.Fatalf("create portfolio reader role expected 201 got %d, body=%v", roleResp.StatusCode, roleBody)
	}
	roleID := uint(roleBody["id"].(float64))

	userResp, userBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":      "portfolio_alias_reader",
		"name":          "Portfolio Alias Reader",
		"email":         "portfolio_alias_reader@example.com",
		"password":      "pass1234",
		"roleIds":       []uint{roleID},
		"departmentIds": []uint{},
	})
	if userResp.StatusCode != http.StatusCreated {
		t.Fatalf("create portfolio reader user expected 201 got %d, body=%v", userResp.StatusCode, userBody)
	}

	loginStatus, loginBody := loginWithCredentials(t, ts.URL, "portfolio_alias_reader", "pass1234")
	if loginStatus != http.StatusOK {
		t.Fatalf("login portfolio reader expected 200 got %d, body=%v", loginStatus, loginBody)
	}
	userToken := loginBody["token"].(string)

	allowedResp, allowedBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/portfolio/projects", userToken, nil)
	if allowedResp.StatusCode != http.StatusOK {
		t.Fatalf("portfolio alias with projects.read expected 200 got %d, body=%v", allowedResp.StatusCode, allowedBody)
	}
	forbiddenResp, forbiddenBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/delivery/tasks", userToken, nil)
	if forbiddenResp.StatusCode != http.StatusForbidden || forbiddenBody["code"] != "FORBIDDEN" {
		t.Fatalf("delivery alias without tasks.read expected 403 FORBIDDEN got %d, body=%v", forbiddenResp.StatusCode, forbiddenBody)
	}
}

func TestChangeOwnPasswordFlow(t *testing.T) {
	ts := setupTestRouter(t)
	defer ts.Close()

	adminToken := loginAndToken(t, ts.URL)
	username := "change_pwd_user"
	originalPassword := "pass1234"
	newPassword := "pass5678"

	createUserResp, _ := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":      username,
		"name":          "Change Password User",
		"email":         "change_pwd_user@example.com",
		"password":      originalPassword,
		"roleIds":       []uint{},
		"departmentIds": []uint{},
	})
	if createUserResp.StatusCode != http.StatusCreated {
		t.Fatalf("create change password user expected 201 got %d", createUserResp.StatusCode)
	}

	loginRaw, _ := json.Marshal(map[string]string{"username": username, "password": originalPassword})
	loginResp, err := http.Post(ts.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(loginRaw))
	if err != nil {
		t.Fatalf("login change password user failed: %v", err)
	}
	defer loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("login change password user status expected 200 got %d", loginResp.StatusCode)
	}
	var loginBody map[string]any
	_ = json.NewDecoder(loginResp.Body).Decode(&loginBody)
	token, _ := loginBody["token"].(string)
	if token == "" {
		t.Fatalf("change password user token should not be empty")
	}

	invalidResp, invalidBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/auth/change-password", token, map[string]any{
		"oldPassword":     "wrong-old",
		"newPassword":     newPassword,
		"confirmPassword": newPassword,
	})
	if invalidResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("change password with invalid old password expected 400 got %d", invalidResp.StatusCode)
	}
	if invalidBody["code"] != "OLD_PASSWORD_INVALID" {
		t.Fatalf("expected OLD_PASSWORD_INVALID code, got %v", invalidBody["code"])
	}

	changeResp, _ := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/auth/change-password", token, map[string]any{
		"oldPassword":     originalPassword,
		"newPassword":     newPassword,
		"confirmPassword": newPassword,
	})
	if changeResp.StatusCode != http.StatusOK {
		t.Fatalf("change password status expected 200 got %d", changeResp.StatusCode)
	}

	oldLoginRaw, _ := json.Marshal(map[string]string{"username": username, "password": originalPassword})
	oldLoginResp, err := http.Post(ts.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(oldLoginRaw))
	if err != nil {
		t.Fatalf("login with old password failed: %v", err)
	}
	defer oldLoginResp.Body.Close()
	if oldLoginResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("old password login status expected 401 got %d", oldLoginResp.StatusCode)
	}

	newLoginRaw, _ := json.Marshal(map[string]string{"username": username, "password": newPassword})
	newLoginResp, err := http.Post(ts.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(newLoginRaw))
	if err != nil {
		t.Fatalf("login with new password failed: %v", err)
	}
	defer newLoginResp.Body.Close()
	if newLoginResp.StatusCode != http.StatusOK {
		t.Fatalf("new password login status expected 200 got %d", newLoginResp.StatusCode)
	}
}

func TestUploadAndAttachFlow(t *testing.T) {
	ts := setupTestRouter(t)
	defer ts.Close()

	token := loginAndToken(t, ts.URL)
	uploadResp, uploadBody := requestMultipartFiles(t, http.MethodPost, ts.URL+"/api/v1/uploads", token, "files", []multipartUploadFile{
		{FileName: "spec.txt", RelativePath: "docs/spec.txt", Content: []byte("hello upload")},
		{FileName: "a.png", RelativePath: "docs/diagram/a.png", Content: []byte("png-mock")},
	})
	if uploadResp.StatusCode != http.StatusCreated {
		t.Fatalf("upload status expected 201 got %d, body=%v", uploadResp.StatusCode, uploadBody)
	}

	attachmentListAny, ok := uploadBody["attachments"].([]any)
	if !ok || len(attachmentListAny) != 1 {
		t.Fatalf("upload response missing attachments: %v", uploadBody)
	}
	attachmentAny, _ := attachmentListAny[0].(map[string]any)
	filePath, _ := attachmentAny["filePath"].(string)
	matched, _ := regexp.MatchString(`^/static/uploads/\d{4}/\d{2}/\d{2}/`, filePath)
	if !matched {
		t.Fatalf("unexpected upload path: %s", filePath)
	}
	relativePath, _ := attachmentAny["relativePath"].(string)
	if relativePath != "docs.zip" {
		t.Fatalf("unexpected relative path: %s", relativePath)
	}
	fileName, _ := attachmentAny["fileName"].(string)
	if fileName != "docs.zip" {
		t.Fatalf("unexpected upload file name: %s", fileName)
	}

	fileResp, err := http.Get(ts.URL + filePath)
	if err != nil {
		t.Fatalf("request uploaded file failed: %v", err)
	}
	zipBytes, readErr := io.ReadAll(fileResp.Body)
	fileResp.Body.Close()
	if readErr != nil {
		t.Fatalf("read uploaded file failed: %v", readErr)
	}
	if fileResp.StatusCode != http.StatusOK {
		t.Fatalf("uploaded file status expected 200 got %d", fileResp.StatusCode)
	}
	zipReader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		t.Fatalf("uploaded file is not a valid zip: %v", err)
	}
	if len(zipReader.File) != 2 {
		t.Fatalf("zip file entry count expected 2 got %d", len(zipReader.File))
	}
	zipEntries := map[string]bool{}
	for _, entry := range zipReader.File {
		zipEntries[entry.Name] = true
	}
	if !zipEntries["docs/spec.txt"] || !zipEntries["docs/diagram/a.png"] {
		t.Fatalf("zip file entries unexpected: %v", zipEntries)
	}

	projectResp, projectBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/projects", token, map[string]any{
		"code":        "UPLOAD-PROJ-1",
		"name":        "上传项目",
		"description": "包含附件",
		"attachments": attachmentListAny,
	})
	if projectResp.StatusCode != http.StatusCreated {
		t.Fatalf("create project status expected 201 got %d", projectResp.StatusCode)
	}
	projectID := int(projectBody["id"].(float64))
	projectAttachments, _ := projectBody["attachments"].([]any)
	if len(projectAttachments) != 1 {
		t.Fatalf("project attachments not saved: %v", projectBody["attachments"])
	}
	projectAttachment, _ := projectAttachments[0].(map[string]any)
	if projectAttachment["filePath"] == "" {
		t.Fatalf("project attachment path not saved: %v", projectAttachment)
	}

	taskResp, taskBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/tasks", token, map[string]any{
		"title":       "上传任务",
		"projectId":   projectID,
		"status":      "pending",
		"progress":    0,
		"attachments": attachmentListAny,
	})
	if taskResp.StatusCode != http.StatusCreated {
		t.Fatalf("create task status expected 201 got %d, body=%v", taskResp.StatusCode, taskBody)
	}
	taskAttachments, _ := taskBody["attachments"].([]any)
	if len(taskAttachments) != 1 {
		t.Fatalf("task attachments not saved: %v", taskBody["attachments"])
	}
	taskAttachment, _ := taskAttachments[0].(map[string]any)
	if taskAttachment["filePath"] != filePath {
		t.Fatalf("task attachment path not saved: %v", taskAttachment)
	}
}

func TestUploadMixedFilesAndFoldersFlow(t *testing.T) {
	ts := setupTestRouter(t)
	defer ts.Close()

	token := loginAndToken(t, ts.URL)
	uploadResp, uploadBody := requestMultipartFiles(t, http.MethodPost, ts.URL+"/api/v1/uploads", token, "files", []multipartUploadFile{
		{FileName: "notes.txt", RelativePath: "notes.txt", Content: []byte("note")},
		{FileName: "spec.txt", RelativePath: "docs/spec.txt", Content: []byte("doc spec")},
		{FileName: "a.png", RelativePath: "docs/diagram/a.png", Content: []byte("doc image")},
		{FileName: "logo.svg", RelativePath: "images/logo.svg", Content: []byte("logo")},
		{FileName: "banner.jpg", RelativePath: "images/banner.jpg", Content: []byte("banner")},
	})
	if uploadResp.StatusCode != http.StatusCreated {
		t.Fatalf("upload status expected 201 got %d, body=%v", uploadResp.StatusCode, uploadBody)
	}

	attachmentListAny, ok := uploadBody["attachments"].([]any)
	if !ok || len(attachmentListAny) != 3 {
		t.Fatalf("upload response attachments expected 3 got %v", uploadBody)
	}

	attachmentByRelativePath := map[string]map[string]any{}
	for _, item := range attachmentListAny {
		attachment, castOK := item.(map[string]any)
		if !castOK {
			t.Fatalf("unexpected attachment item: %v", item)
		}
		relativePath, _ := attachment["relativePath"].(string)
		attachmentByRelativePath[relativePath] = attachment
	}

	notesAttachment, hasNotes := attachmentByRelativePath["notes.txt"]
	if !hasNotes {
		t.Fatalf("missing standalone file attachment: %v", attachmentByRelativePath)
	}

	docsZipAttachment, hasDocsZip := attachmentByRelativePath["docs.zip"]
	if !hasDocsZip {
		t.Fatalf("missing docs.zip attachment: %v", attachmentByRelativePath)
	}
	imagesZipAttachment, hasImagesZip := attachmentByRelativePath["images.zip"]
	if !hasImagesZip {
		t.Fatalf("missing images.zip attachment: %v", attachmentByRelativePath)
	}

	assertAttachmentPath := func(item map[string]any) string {
		filePath, _ := item["filePath"].(string)
		matched, _ := regexp.MatchString(`^/static/uploads/\d{4}/\d{2}/\d{2}/`, filePath)
		if !matched {
			t.Fatalf("unexpected upload path: %s", filePath)
		}
		return filePath
	}

	notesPath := assertAttachmentPath(notesAttachment)
	docsZipPath := assertAttachmentPath(docsZipAttachment)
	imagesZipPath := assertAttachmentPath(imagesZipAttachment)

	if mimeType, _ := docsZipAttachment["mimeType"].(string); mimeType != "application/zip" {
		t.Fatalf("docs zip mimeType expected application/zip got %s", mimeType)
	}
	if mimeType, _ := imagesZipAttachment["mimeType"].(string); mimeType != "application/zip" {
		t.Fatalf("images zip mimeType expected application/zip got %s", mimeType)
	}

	notesResp, err := http.Get(ts.URL + notesPath)
	if err != nil {
		t.Fatalf("request uploaded note file failed: %v", err)
	}
	notesBytes, readErr := io.ReadAll(notesResp.Body)
	notesResp.Body.Close()
	if readErr != nil {
		t.Fatalf("read uploaded note file failed: %v", readErr)
	}
	if notesResp.StatusCode != http.StatusOK {
		t.Fatalf("uploaded note file status expected 200 got %d", notesResp.StatusCode)
	}
	if string(notesBytes) != "note" {
		t.Fatalf("uploaded note file content unexpected: %s", string(notesBytes))
	}

	assertZipEntries := func(filePath string, expectedEntries []string) {
		fileResp, getErr := http.Get(ts.URL + filePath)
		if getErr != nil {
			t.Fatalf("request uploaded zip failed: %v", getErr)
		}
		zipBytes, zipReadErr := io.ReadAll(fileResp.Body)
		fileResp.Body.Close()
		if zipReadErr != nil {
			t.Fatalf("read uploaded zip failed: %v", zipReadErr)
		}
		if fileResp.StatusCode != http.StatusOK {
			t.Fatalf("uploaded zip status expected 200 got %d", fileResp.StatusCode)
		}
		zipReader, openErr := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
		if openErr != nil {
			t.Fatalf("uploaded file is not a valid zip: %v", openErr)
		}
		entries := map[string]bool{}
		for _, entry := range zipReader.File {
			entries[entry.Name] = true
		}
		if len(entries) != len(expectedEntries) {
			t.Fatalf("zip entry count expected %d got %d, entries=%v", len(expectedEntries), len(entries), entries)
		}
		for _, expected := range expectedEntries {
			if !entries[expected] {
				t.Fatalf("zip missing expected entry %s, entries=%v", expected, entries)
			}
		}
	}

	assertZipEntries(docsZipPath, []string{"docs/spec.txt", "docs/diagram/a.png"})
	assertZipEntries(imagesZipPath, []string{"images/logo.svg", "images/banner.jpg"})
}

func TestAuthCRUDAndAuditFlow(t *testing.T) {
	ts := setupTestRouter(t)
	defer ts.Close()

	token := loginAndToken(t, ts.URL)

	departmentBody := map[string]any{"name": "研发部", "description": "R&D"}
	raw, _ := json.Marshal(departmentBody)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/system/departments", bytes.NewReader(raw))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("create department failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create department status expected 201 got %d", resp.StatusCode)
	}

	logsReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/system/audit/logs?module=departments", nil)
	logsReq.Header.Set("Authorization", "Bearer "+token)
	logsResp, err := http.DefaultClient.Do(logsReq)
	if err != nil {
		t.Fatalf("query audit logs failed: %v", err)
	}
	defer logsResp.Body.Close()
	if logsResp.StatusCode != http.StatusOK {
		t.Fatalf("audit logs status expected 200 got %d", logsResp.StatusCode)
	}

	var logs struct {
		List []struct {
			Module string `json:"module"`
			Action string `json:"action"`
		} `json:"list"`
	}
	if err = json.NewDecoder(logsResp.Body).Decode(&logs); err != nil {
		t.Fatalf("decode logs failed: %v", err)
	}
	if len(logs.List) == 0 {
		t.Fatalf("expected audit logs but got none")
	}
}

func TestAuditLogsDefaultPageSize(t *testing.T) {
	ts := setupTestRouter(t)
	defer ts.Close()

	token := loginAndToken(t, ts.URL)
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/system/audit/logs", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("query audit logs failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("audit logs status expected 200 got %d", resp.StatusCode)
	}

	var page struct {
		PageSize int `json:"pageSize"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&page); err != nil {
		t.Fatalf("decode logs page failed: %v", err)
	}
	if page.PageSize != 20 {
		t.Fatalf("audit logs default pageSize expected 20 got %d", page.PageSize)
	}
}

func TestTagCRUDAndTaskRelationFlow(t *testing.T) {
	ts := setupTestRouter(t)
	defer ts.Close()

	token := loginAndToken(t, ts.URL)

	tagResp, tagBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/tags", token, map[string]any{
		"name": "后端",
	})
	if tagResp.StatusCode != http.StatusCreated {
		t.Fatalf("create tag status expected 201 got %d, body=%v", tagResp.StatusCode, tagBody)
	}
	tagID := int(tagBody["id"].(float64))

	projectResp, projectBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/projects", token, map[string]any{
		"code": "TAG-PROJ-1",
		"name": "标签项目",
	})
	if projectResp.StatusCode != http.StatusCreated {
		t.Fatalf("create project status expected 201 got %d", projectResp.StatusCode)
	}
	projectID := int(projectBody["id"].(float64))

	taskResp, taskBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/tasks", token, map[string]any{
		"title":        "标签任务",
		"projectId":    projectID,
		"tagIds":       []int{tagID},
		"customField1": "扩展内容1",
		"customField2": "扩展内容2",
		"customField3": "扩展内容3",
	})
	if taskResp.StatusCode != http.StatusCreated {
		t.Fatalf("create task with tags status expected 201 got %d, body=%v", taskResp.StatusCode, taskBody)
	}
	taskTags, _ := taskBody["tags"].([]any)
	if len(taskTags) != 1 {
		t.Fatalf("task tags expected 1 got %v", taskBody["tags"])
	}
	if taskBody["customField1"] != "扩展内容1" || taskBody["customField2"] != "扩展内容2" || taskBody["customField3"] != "扩展内容3" {
		t.Fatalf("task custom fields unexpected: %v %v %v", taskBody["customField1"], taskBody["customField2"], taskBody["customField3"])
	}
	taggedTaskID := int(taskBody["id"].(float64))

	plainTaskResp, plainTaskBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/tasks", token, map[string]any{
		"title":     "无标签任务",
		"projectId": projectID,
	})
	if plainTaskResp.StatusCode != http.StatusCreated {
		t.Fatalf("create plain task status expected 201 got %d, body=%v", plainTaskResp.StatusCode, plainTaskBody)
	}

	filteredTaskResp, filteredTaskBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/tasks?page=1&pageSize=20&tagIds="+strconv.Itoa(tagID), token, nil)
	if filteredTaskResp.StatusCode != http.StatusOK {
		t.Fatalf("filter tasks by tag status expected 200 got %d", filteredTaskResp.StatusCode)
	}
	filteredTaskList, _ := filteredTaskBody["list"].([]any)
	if len(filteredTaskList) != 1 {
		t.Fatalf("filter tasks by tag expected 1 got %d", len(filteredTaskList))
	}
	filteredTask, _ := filteredTaskList[0].(map[string]any)
	if int(filteredTask["id"].(float64)) != taggedTaskID {
		t.Fatalf("filter tasks by tag returned unexpected task id=%v", filteredTask["id"])
	}

	listResp, listBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/tags?page=1&pageSize=20", token, nil)
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("list tags status expected 200 got %d", listResp.StatusCode)
	}
	list, _ := listBody["list"].([]any)
	if len(list) == 0 {
		t.Fatalf("tags list should not be empty")
	}
	firstTag, _ := list[0].(map[string]any)
	if int(firstTag["taskCount"].(float64)) != 1 {
		t.Fatalf("tag taskCount expected 1 got %v", firstTag["taskCount"])
	}

	detailResp, detailBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/tags/"+strconv.Itoa(tagID), token, nil)
	if detailResp.StatusCode != http.StatusOK {
		t.Fatalf("get tag status expected 200 got %d", detailResp.StatusCode)
	}
	if detailBody["name"] != "后端" {
		t.Fatalf("tag detail name unexpected: %v", detailBody["name"])
	}
	if int(detailBody["taskCount"].(float64)) != 1 {
		t.Fatalf("tag detail taskCount expected 1 got %v", detailBody["taskCount"])
	}

	updateResp, updateBody := requestJSON(t, http.MethodPut, ts.URL+"/api/v1/tags/"+strconv.Itoa(tagID), token, map[string]any{
		"name": "后端标签",
	})
	if updateResp.StatusCode != http.StatusOK {
		t.Fatalf("update tag status expected 200 got %d, body=%v", updateResp.StatusCode, updateBody)
	}
	if updateBody["name"] != "后端标签" {
		t.Fatalf("updated tag name unexpected: %v", updateBody["name"])
	}

	deleteResp, _ := requestJSON(t, http.MethodDelete, ts.URL+"/api/v1/tags/"+strconv.Itoa(tagID), token, nil)
	if deleteResp.StatusCode != http.StatusOK {
		t.Fatalf("delete tag status expected 200 got %d", deleteResp.StatusCode)
	}
}

func TestGanttAndTaskTreeConsistency(t *testing.T) {
	ts := setupTestRouter(t)
	defer ts.Close()

	token := loginAndToken(t, ts.URL)

	projectPayload := map[string]any{"code": "P-100", "name": "测试项目", "description": "desc"}
	projectRaw, _ := json.Marshal(projectPayload)
	projectReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/projects", bytes.NewReader(projectRaw))
	projectReq.Header.Set("Authorization", "Bearer "+token)
	projectReq.Header.Set("Content-Type", "application/json")
	projectResp, err := http.DefaultClient.Do(projectReq)
	if err != nil {
		t.Fatalf("create project failed: %v", err)
	}
	defer projectResp.Body.Close()
	if projectResp.StatusCode != http.StatusCreated {
		t.Fatalf("create project status expected 201 got %d", projectResp.StatusCode)
	}
	var project map[string]any
	_ = json.NewDecoder(projectResp.Body).Decode(&project)
	projectID := int(project["id"].(float64))

	createTask := func(title string, parentID *int) {
		payload := map[string]any{
			"title":     title,
			"projectId": projectID,
			"status":    "pending",
			"progress":  10,
			"startAt":   "2026-03-24T10:00:00Z",
			"endAt":     "2026-03-25T10:00:00Z",
		}
		if parentID != nil {
			payload["parentId"] = *parentID
		}
		raw, _ := json.Marshal(payload)
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/tasks", bytes.NewReader(raw))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("create task failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("create task status expected 201 got %d", resp.StatusCode)
		}
		var task map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&task)
		if parentID == nil {
			id := int(task["id"].(float64))
			parentID = &id
		}
	}

	createTask("根任务", nil)
	// second root task
	createTask("根任务2", nil)

	ganttReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/projects/"+strconv.Itoa(projectID)+"/gantt", nil)
	ganttReq.Header.Set("Authorization", "Bearer "+token)
	ganttResp, err := http.DefaultClient.Do(ganttReq)
	if err != nil {
		t.Fatalf("query gantt failed: %v", err)
	}
	defer ganttResp.Body.Close()
	if ganttResp.StatusCode != http.StatusOK {
		t.Fatalf("gantt status expected 200 got %d", ganttResp.StatusCode)
	}
	var gantt []map[string]any
	_ = json.NewDecoder(ganttResp.Body).Decode(&gantt)

	treeReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/projects/"+strconv.Itoa(projectID)+"/task-tree", nil)
	treeReq.Header.Set("Authorization", "Bearer "+token)
	treeResp, err := http.DefaultClient.Do(treeReq)
	if err != nil {
		t.Fatalf("query tree failed: %v", err)
	}
	defer treeResp.Body.Close()
	if treeResp.StatusCode != http.StatusOK {
		t.Fatalf("tree status expected 200 got %d", treeResp.StatusCode)
	}
	var tree []map[string]any
	_ = json.NewDecoder(treeResp.Body).Decode(&tree)

	countTree := 0
	var walk func([]map[string]any)
	walk = func(nodes []map[string]any) {
		for _, n := range nodes {
			countTree++
			if children, ok := n["children"].([]any); ok {
				next := make([]map[string]any, 0, len(children))
				for _, child := range children {
					if mapped, ok := child.(map[string]any); ok {
						next = append(next, mapped)
					}
				}
				walk(next)
			}
		}
	}
	walk(tree)

	if len(gantt) != countTree {
		t.Fatalf("gantt count %d != tree count %d", len(gantt), countTree)
	}
}

func TestUserScopeAndExportFlow(t *testing.T) {
	ts := setupTestRouter(t)
	defer ts.Close()

	adminToken := loginAndToken(t, ts.URL)

	permReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/system/rbac/permissions", nil)
	permReq.Header.Set("Authorization", "Bearer "+adminToken)
	permResp, err := http.DefaultClient.Do(permReq)
	if err != nil {
		t.Fatalf("query permissions failed: %v", err)
	}
	if permResp.StatusCode != http.StatusOK {
		t.Fatalf("query permissions status expected 200 got %d", permResp.StatusCode)
	}
	var permissions []map[string]any
	_ = json.NewDecoder(permResp.Body).Decode(&permissions)
	permResp.Body.Close()

	codeToID := map[string]uint{}
	for _, permission := range permissions {
		code, _ := permission["code"].(string)
		id, _ := permission["id"].(float64)
		codeToID[code] = uint(id)
	}
	readerPerms := []uint{
		codeToID["projects.read"],
		codeToID["tasks.read"],
		codeToID["stats.read"],
	}

	roleResp, roleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/rbac/roles", adminToken, map[string]any{
		"name":          "scope-reader",
		"description":   "scope reader",
		"permissionIds": readerPerms,
	})
	if roleResp.StatusCode != http.StatusCreated {
		t.Fatalf("create role status expected 201 got %d", roleResp.StatusCode)
	}
	roleID := uint(roleBody["id"].(float64))

	userAResp, userABody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":      "scope_u1",
		"name":          "Scope U1",
		"email":         "scope_u1@example.com",
		"password":      "pass1234",
		"roleIds":       []uint{roleID},
		"departmentIds": []uint{},
	})
	if userAResp.StatusCode != http.StatusCreated {
		t.Fatalf("create userA status expected 201 got %d", userAResp.StatusCode)
	}
	userAID := uint(userABody["id"].(float64))

	userBResp, userBBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":      "scope_u2",
		"name":          "Scope U2",
		"email":         "scope_u2@example.com",
		"password":      "pass1234",
		"roleIds":       []uint{roleID},
		"departmentIds": []uint{},
	})
	if userBResp.StatusCode != http.StatusCreated {
		t.Fatalf("create userB status expected 201 got %d", userBResp.StatusCode)
	}
	userBID := uint(userBBody["id"].(float64))

	loginUser := func(username string) string {
		payload := map[string]string{"username": username, "password": "pass1234"}
		raw, _ := json.Marshal(payload)
		resp, err := http.Post(ts.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(raw))
		if err != nil {
			t.Fatalf("login %s failed: %v", username, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("login %s status expected 200 got %d", username, resp.StatusCode)
		}
		var result map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&result)
		return result["token"].(string)
	}
	userAToken := loginUser("scope_u1")

	projectAResp, projectABody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/projects", adminToken, map[string]any{
		"code":          "SCOPE-P1",
		"name":          "Scope Project 1",
		"description":   "for user1",
		"userIds":       []uint{userAID},
		"departmentIds": []uint{},
	})
	if projectAResp.StatusCode != http.StatusCreated {
		t.Fatalf("create projectA status expected 201 got %d", projectAResp.StatusCode)
	}
	projectAID := int(projectABody["id"].(float64))

	projectBResp, projectBBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/projects", adminToken, map[string]any{
		"code":          "SCOPE-P2",
		"name":          "Scope Project 2",
		"description":   "for user2",
		"userIds":       []uint{},
		"departmentIds": []uint{},
	})
	if projectBResp.StatusCode != http.StatusCreated {
		t.Fatalf("create projectB status expected 201 got %d", projectBResp.StatusCode)
	}
	projectBID := int(projectBBody["id"].(float64))

	_, _ = requestJSON(t, http.MethodPost, ts.URL+"/api/v1/tasks", adminToken, map[string]any{
		"title":       "Scope Task 1",
		"projectId":   projectAID,
		"status":      "processing",
		"progress":    40,
		"startAt":     "2026-03-24T10:00:00Z",
		"endAt":       "2026-03-25T10:00:00Z",
		"assigneeIds": []uint{userAID},
	})
	_, _ = requestJSON(t, http.MethodPost, ts.URL+"/api/v1/tasks", adminToken, map[string]any{
		"title":       "Scope Task 2",
		"projectId":   projectBID,
		"status":      "queued",
		"progress":    20,
		"startAt":     "2026-03-24T10:00:00Z",
		"endAt":       "2026-03-25T10:00:00Z",
		"assigneeIds": []uint{userBID},
	})

	projectsReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/projects", nil)
	projectsReq.Header.Set("Authorization", "Bearer "+userAToken)
	projectsResp, err := http.DefaultClient.Do(projectsReq)
	if err != nil {
		t.Fatalf("query scoped projects failed: %v", err)
	}
	var projectList struct {
		List []map[string]any `json:"list"`
	}
	_ = json.NewDecoder(projectsResp.Body).Decode(&projectList)
	projectsResp.Body.Close()
	if len(projectList.List) != 1 {
		t.Fatalf("expected 1 scoped project, got %d", len(projectList.List))
	}

	tasksReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/tasks", nil)
	tasksReq.Header.Set("Authorization", "Bearer "+userAToken)
	tasksResp, err := http.DefaultClient.Do(tasksReq)
	if err != nil {
		t.Fatalf("query scoped tasks failed: %v", err)
	}
	var taskList struct {
		List []map[string]any `json:"list"`
	}
	_ = json.NewDecoder(tasksResp.Body).Decode(&taskList)
	tasksResp.Body.Close()
	if len(taskList.List) != 1 {
		t.Fatalf("expected 1 scoped task, got %d", len(taskList.List))
	}

	statsReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/stats/dashboard", nil)
	statsReq.Header.Set("Authorization", "Bearer "+userAToken)
	statsResp, err := http.DefaultClient.Do(statsReq)
	if err != nil {
		t.Fatalf("query scoped stats failed: %v", err)
	}
	var stats map[string]any
	_ = json.NewDecoder(statsResp.Body).Decode(&stats)
	statsResp.Body.Close()
	if int(stats["tasks"].(float64)) != 1 {
		t.Fatalf("expected scoped tasks=1, got %v", stats["tasks"])
	}

	projectExportReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/projects/export", nil)
	projectExportReq.Header.Set("Authorization", "Bearer "+userAToken)
	projectExportResp, err := http.DefaultClient.Do(projectExportReq)
	if err != nil {
		t.Fatalf("export projects failed: %v", err)
	}
	projectExportRaw, _ := io.ReadAll(projectExportResp.Body)
	projectExportResp.Body.Close()
	projectCSV, _ := csv.NewReader(strings.NewReader(string(projectExportRaw))).ReadAll()
	if len(projectCSV) < 2 || strings.Contains(strings.Join(projectCSV[len(projectCSV)-1], ","), "SCOPE-P2") {
		t.Fatalf("project export should not contain SCOPE-P2, got: %s", string(projectExportRaw))
	}

	taskExportReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/tasks/export", nil)
	taskExportReq.Header.Set("Authorization", "Bearer "+userAToken)
	taskExportResp, err := http.DefaultClient.Do(taskExportReq)
	if err != nil {
		t.Fatalf("export tasks failed: %v", err)
	}
	taskExportRaw, _ := io.ReadAll(taskExportResp.Body)
	taskExportResp.Body.Close()
	if strings.Contains(string(taskExportRaw), "Scope Task 2") {
		t.Fatalf("task export should not contain Scope Task 2, got: %s", string(taskExportRaw))
	}
}

func TestProjectFinanceGovernancePermissionsExportAndNotification(t *testing.T) {
	ts := setupTestRouter(t)
	defer ts.Close()

	adminToken := loginAndToken(t, ts.URL)
	codeToID := permissionCodeMap(t, ts.URL, adminToken)

	financeRoleResp, financeRoleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/rbac/roles", adminToken, map[string]any{
		"name":        "project-finance-editor",
		"description": "project finance editor",
		"permissionIds": []uint{
			codeToID["projects.create"],
			codeToID["projects.read"],
			codeToID["projects.update"],
			codeToID["finance.read"],
			codeToID["finance.update"],
			codeToID["notifications.read"],
		},
	})
	if financeRoleResp.StatusCode != http.StatusCreated {
		t.Fatalf("create finance role expected 201 got %d, body=%v", financeRoleResp.StatusCode, financeRoleBody)
	}
	financeRoleID := uint(financeRoleBody["id"].(float64))

	plainRoleResp, plainRoleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/rbac/roles", adminToken, map[string]any{
		"name":        "project-plain-editor",
		"description": "project plain editor",
		"permissionIds": []uint{
			codeToID["projects.read"],
			codeToID["projects.update"],
		},
	})
	if plainRoleResp.StatusCode != http.StatusCreated {
		t.Fatalf("create plain role expected 201 got %d, body=%v", plainRoleResp.StatusCode, plainRoleBody)
	}
	plainRoleID := uint(plainRoleBody["id"].(float64))

	financeUserResp, financeUserBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":      "project_finance_user",
		"name":          "Project Finance User",
		"email":         "project_finance_user@example.com",
		"password":      "pass1234",
		"roleIds":       []uint{financeRoleID},
		"departmentIds": []uint{},
	})
	if financeUserResp.StatusCode != http.StatusCreated {
		t.Fatalf("create finance user expected 201 got %d, body=%v", financeUserResp.StatusCode, financeUserBody)
	}
	financeUserID := uint(financeUserBody["id"].(float64))

	plainUserResp, plainUserBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":      "project_plain_user",
		"name":          "Project Plain User",
		"email":         "project_plain_user@example.com",
		"password":      "pass1234",
		"roleIds":       []uint{plainRoleID},
		"departmentIds": []uint{},
	})
	if plainUserResp.StatusCode != http.StatusCreated {
		t.Fatalf("create plain user expected 201 got %d, body=%v", plainUserResp.StatusCode, plainUserBody)
	}
	plainUserID := uint(plainUserBody["id"].(float64))

	_, financeLoginBody := loginWithCredentials(t, ts.URL, "project_finance_user", "pass1234")
	financeToken, _ := financeLoginBody["token"].(string)
	if financeToken == "" {
		t.Fatalf("finance token should not be empty")
	}
	_, plainLoginBody := loginWithCredentials(t, ts.URL, "project_plain_user", "pass1234")
	plainToken, _ := plainLoginBody["token"].(string)
	if plainToken == "" {
		t.Fatalf("plain token should not be empty")
	}

	createResp, createBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/projects", financeToken, map[string]any{
		"code":                  "FIN-P1",
		"name":                  "Finance Project",
		"description":           "finance guarded project",
		"budgetAmount":          1000,
		"actualCostAmount":      1200,
		"expectedRevenueAmount": 1800,
		"contractNo":            "CNT-2026-001",
		"contractAttachments": []map[string]any{
			{
				"fileName":     "contract.pdf",
				"filePath":     "/static/uploads/contracts/contract.pdf",
				"relativePath": "contracts/contract.pdf",
				"fileSize":     128,
				"mimeType":     "application/pdf",
				"category":     "contract",
				"version":      "v1",
				"accessLevel":  "finance",
				"expiresAt":    "2026-08-01T00:00:00Z",
			},
		},
		"userIds":       []uint{financeUserID, plainUserID},
		"departmentIds": []uint{},
	})
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("create finance project expected 201 got %d, body=%v", createResp.StatusCode, createBody)
	}
	projectID := int(createBody["id"].(float64))
	if createBody["budgetAmount"].(float64) != 1000 || createBody["actualCostAmount"].(float64) != 1200 || createBody["costOverBudget"] != true {
		t.Fatalf("finance fields should be returned to finance user: %v", createBody)
	}
	if attachments, _ := createBody["contractAttachments"].([]any); len(attachments) != 1 {
		t.Fatalf("contract attachment should be returned to finance user: %v", createBody["contractAttachments"])
	}

	plainListResp, plainListBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/projects?page=1&pageSize=20", plainToken, nil)
	if plainListResp.StatusCode != http.StatusOK {
		t.Fatalf("plain project list expected 200 got %d, body=%v", plainListResp.StatusCode, plainListBody)
	}
	plainProjects, _ := plainListBody["list"].([]any)
	if len(plainProjects) != 1 {
		t.Fatalf("plain user should see owned project only, got %d", len(plainProjects))
	}
	plainProject, _ := plainProjects[0].(map[string]any)
	if _, ok := plainProject["budgetAmount"]; ok {
		t.Fatalf("plain list must not expose budgetAmount: %v", plainProject)
	}
	if _, ok := plainProject["contractNo"]; ok {
		t.Fatalf("plain list must not expose contractNo: %v", plainProject)
	}

	plainDetailResp, plainDetailBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/projects/"+strconv.Itoa(projectID), plainToken, nil)
	if plainDetailResp.StatusCode != http.StatusOK {
		t.Fatalf("plain project detail expected 200 got %d, body=%v", plainDetailResp.StatusCode, plainDetailBody)
	}
	if _, ok := plainDetailBody["actualCostAmount"]; ok {
		t.Fatalf("plain detail must not expose actualCostAmount: %v", plainDetailBody)
	}
	if _, ok := plainDetailBody["contractAttachments"]; ok {
		t.Fatalf("plain detail must not expose contractAttachments: %v", plainDetailBody)
	}

	plainUpdateResp, plainUpdateBody := requestJSON(t, http.MethodPut, ts.URL+"/api/v1/projects/"+strconv.Itoa(projectID), plainToken, map[string]any{
		"code":          "FIN-P1",
		"name":          "Finance Project Renamed",
		"description":   "plain update without finance payload",
		"userIds":       []uint{financeUserID, plainUserID},
		"departmentIds": []uint{},
	})
	if plainUpdateResp.StatusCode != http.StatusOK {
		t.Fatalf("plain update without finance expected 200 got %d, body=%v", plainUpdateResp.StatusCode, plainUpdateBody)
	}
	if _, ok := plainUpdateBody["budgetAmount"]; ok {
		t.Fatalf("plain update response must not expose budgetAmount: %v", plainUpdateBody)
	}

	plainFinanceUpdateResp, plainFinanceUpdateBody := requestJSON(t, http.MethodPut, ts.URL+"/api/v1/projects/"+strconv.Itoa(projectID), plainToken, map[string]any{
		"code":          "FIN-P1",
		"name":          "Finance Project Renamed",
		"description":   "plain update with finance payload",
		"budgetAmount":  2000,
		"userIds":       []uint{financeUserID, plainUserID},
		"departmentIds": []uint{},
	})
	if plainFinanceUpdateResp.StatusCode != http.StatusForbidden {
		t.Fatalf("plain finance update expected 403 got %d, body=%v", plainFinanceUpdateResp.StatusCode, plainFinanceUpdateBody)
	}
	if plainFinanceUpdateBody["code"] != "PROJECT_FINANCE_PERMISSION_REQUIRED" {
		t.Fatalf("plain finance update expected PROJECT_FINANCE_PERMISSION_REQUIRED got %v", plainFinanceUpdateBody["code"])
	}

	invalidFinanceResp, invalidFinanceBody := requestJSON(t, http.MethodPut, ts.URL+"/api/v1/projects/"+strconv.Itoa(projectID), financeToken, map[string]any{
		"code":          "FIN-P1",
		"name":          "Finance Project Renamed",
		"description":   "negative budget",
		"budgetAmount":  -1,
		"userIds":       []uint{financeUserID, plainUserID},
		"departmentIds": []uint{},
	})
	if invalidFinanceResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("negative project finance expected 400 got %d, body=%v", invalidFinanceResp.StatusCode, invalidFinanceBody)
	}
	if invalidFinanceBody["code"] != "INVALID_PROJECT_FINANCE_AMOUNT" {
		t.Fatalf("negative project finance expected INVALID_PROJECT_FINANCE_AMOUNT got %v", invalidFinanceBody["code"])
	}

	financeExportReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/projects/export", nil)
	financeExportReq.Header.Set("Authorization", "Bearer "+financeToken)
	financeExportResp, err := http.DefaultClient.Do(financeExportReq)
	if err != nil {
		t.Fatalf("finance export failed: %v", err)
	}
	financeExportRaw, _ := io.ReadAll(financeExportResp.Body)
	financeExportResp.Body.Close()
	financeExportText := string(financeExportRaw)
	if !strings.Contains(financeExportText, "预算") || !strings.Contains(financeExportText, "CNT-2026-001") || !strings.Contains(financeExportText, "true") {
		t.Fatalf("finance export should contain finance columns and values, got: %s", financeExportText)
	}

	plainExportReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/projects/export", nil)
	plainExportReq.Header.Set("Authorization", "Bearer "+plainToken)
	plainExportResp, err := http.DefaultClient.Do(plainExportReq)
	if err != nil {
		t.Fatalf("plain export failed: %v", err)
	}
	plainExportRaw, _ := io.ReadAll(plainExportResp.Body)
	plainExportResp.Body.Close()
	plainExportText := string(plainExportRaw)
	if strings.Contains(plainExportText, "预算") || strings.Contains(plainExportText, "CNT-2026-001") {
		t.Fatalf("plain export must not contain finance columns or values, got: %s", plainExportText)
	}

	notificationResp, notificationBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/notifications?module=projects&keyword=项目成本超预算", financeToken, nil)
	if notificationResp.StatusCode != http.StatusOK {
		t.Fatalf("budget notification query expected 200 got %d, body=%v", notificationResp.StatusCode, notificationBody)
	}
	notifications, _ := notificationBody["list"].([]any)
	if len(notifications) == 0 {
		t.Fatalf("finance user should receive budget exceeded notification: %v", notificationBody)
	}
}

func TestAIAssistantPermissionsScopeAndReadOnlyDrafts(t *testing.T) {
	ts := setupTestRouter(t)
	defer ts.Close()

	adminToken := loginAndToken(t, ts.URL)
	codeToID := permissionCodeMap(t, ts.URL, adminToken)
	for _, code := range []string{"ai.read", "projects.read", "tasks.read", "comments.read", "comments.create", "registers.read"} {
		if codeToID[code] == 0 {
			t.Fatalf("permission seed missing: %s", code)
		}
	}

	noAIRoleResp, noAIRoleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/rbac/roles", adminToken, map[string]any{
		"name":        "ai-without-entry",
		"description": "no ai entry",
		"permissionIds": []uint{
			codeToID["projects.read"],
		},
	})
	if noAIRoleResp.StatusCode != http.StatusCreated {
		t.Fatalf("create no ai role expected 201 got %d, body=%v", noAIRoleResp.StatusCode, noAIRoleBody)
	}
	noAIRoleID := uint(noAIRoleBody["id"].(float64))

	aiOnlyRoleResp, aiOnlyRoleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/rbac/roles", adminToken, map[string]any{
		"name":          "ai-only-entry",
		"description":   "ai entry only",
		"permissionIds": []uint{codeToID["ai.read"]},
	})
	if aiOnlyRoleResp.StatusCode != http.StatusCreated {
		t.Fatalf("create ai only role expected 201 got %d, body=%v", aiOnlyRoleResp.StatusCode, aiOnlyRoleBody)
	}
	aiOnlyRoleID := uint(aiOnlyRoleBody["id"].(float64))

	fullRoleResp, fullRoleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/rbac/roles", adminToken, map[string]any{
		"name":        "ai-project-manager",
		"description": "ai project manager",
		"permissionIds": []uint{
			codeToID["ai.read"],
			codeToID["projects.read"],
			codeToID["tasks.read"],
			codeToID["comments.read"],
			codeToID["comments.create"],
			codeToID["registers.read"],
		},
	})
	if fullRoleResp.StatusCode != http.StatusCreated {
		t.Fatalf("create ai full role expected 201 got %d, body=%v", fullRoleResp.StatusCode, fullRoleBody)
	}
	fullRoleID := uint(fullRoleBody["id"].(float64))

	noAIUserResp, _ := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":      "ai_no_entry",
		"name":          "AI No Entry",
		"email":         "ai_no_entry@example.com",
		"password":      "pass1234",
		"roleIds":       []uint{noAIRoleID},
		"departmentIds": []uint{},
	})
	if noAIUserResp.StatusCode != http.StatusCreated {
		t.Fatalf("create no ai user expected 201 got %d", noAIUserResp.StatusCode)
	}

	aiOnlyUserResp, _ := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":      "ai_only_entry",
		"name":          "AI Only Entry",
		"email":         "ai_only_entry@example.com",
		"password":      "pass1234",
		"roleIds":       []uint{aiOnlyRoleID},
		"departmentIds": []uint{},
	})
	if aiOnlyUserResp.StatusCode != http.StatusCreated {
		t.Fatalf("create ai only user expected 201 got %d", aiOnlyUserResp.StatusCode)
	}

	fullUserResp, fullUserBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":      "ai_pm_user",
		"name":          "AI PM User",
		"email":         "ai_pm_user@example.com",
		"password":      "pass1234",
		"roleIds":       []uint{fullRoleID},
		"departmentIds": []uint{},
	})
	if fullUserResp.StatusCode != http.StatusCreated {
		t.Fatalf("create ai pm user expected 201 got %d, body=%v", fullUserResp.StatusCode, fullUserBody)
	}
	fullUserID := uint(fullUserBody["id"].(float64))

	_, noAILoginBody := loginWithCredentials(t, ts.URL, "ai_no_entry", "pass1234")
	noAIToken := noAILoginBody["token"].(string)
	_, aiOnlyLoginBody := loginWithCredentials(t, ts.URL, "ai_only_entry", "pass1234")
	aiOnlyToken := aiOnlyLoginBody["token"].(string)
	_, fullLoginBody := loginWithCredentials(t, ts.URL, "ai_pm_user", "pass1234")
	fullToken := fullLoginBody["token"].(string)

	projectResp, projectBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/projects", adminToken, map[string]any{
		"code":                  "AI-P1",
		"name":                  "AI Visible Project",
		"description":           "上线一个内部协作能力",
		"budgetAmount":          98765,
		"actualCostAmount":      1000,
		"expectedRevenueAmount": 120000,
		"contractNo":            "AI-CNT-SECRET",
		"userIds":               []uint{fullUserID},
		"departmentIds":         []uint{},
	})
	if projectResp.StatusCode != http.StatusCreated {
		t.Fatalf("create ai project expected 201 got %d, body=%v", projectResp.StatusCode, projectBody)
	}
	projectID := int(projectBody["id"].(float64))

	hiddenProjectResp, hiddenProjectBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/projects", adminToken, map[string]any{
		"code":        "AI-HIDDEN",
		"name":        "AI Hidden Project",
		"description": "hidden ai project",
	})
	if hiddenProjectResp.StatusCode != http.StatusCreated {
		t.Fatalf("create hidden ai project expected 201 got %d, body=%v", hiddenProjectResp.StatusCode, hiddenProjectBody)
	}
	hiddenProjectID := int(hiddenProjectBody["id"].(float64))

	taskResp, taskBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/tasks", adminToken, map[string]any{
		"title":       "完成上线准备",
		"description": "确认上线清单和回滚预案",
		"projectId":   projectID,
		"status":      "processing",
		"priority":    "high",
		"progress":    60,
		"startAt":     "2026-01-01T00:00:00Z",
		"endAt":       "2026-01-10T00:00:00Z",
		"assigneeIds": []uint{fullUserID},
	})
	if taskResp.StatusCode != http.StatusCreated {
		t.Fatalf("create ai task expected 201 got %d, body=%v", taskResp.StatusCode, taskBody)
	}
	taskID := int(taskBody["id"].(float64))

	commentResp, commentBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/tasks/"+strconv.Itoa(taskID)+"/comments", fullToken, map[string]any{
		"content": "本周已完成上线清单初稿，等待负责人确认。",
	})
	if commentResp.StatusCode != http.StatusCreated {
		t.Fatalf("create ai task comment expected 201 got %d, body=%v", commentResp.StatusCode, commentBody)
	}

	registerResp, registerBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/project-registers", adminToken, map[string]any{
		"type":         "risk",
		"projectId":    projectID,
		"title":        "上线窗口被压缩",
		"description":  "外部窗口缩短，回滚验证时间不足",
		"status":       "open",
		"severity":     "high",
		"probability":  "high",
		"impact":       "critical",
		"responsePlan": "提前完成回滚演练",
		"ownerId":      fullUserID,
	})
	if registerResp.StatusCode != http.StatusCreated {
		t.Fatalf("create ai register expected 201 got %d, body=%v", registerResp.StatusCode, registerBody)
	}

	noAIResp, noAIBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/ai/project-weekly-report", noAIToken, map[string]any{
		"projectId": projectID,
	})
	if noAIResp.StatusCode != http.StatusForbidden || noAIBody["code"] != "FORBIDDEN" {
		t.Fatalf("missing ai.read expected 403 FORBIDDEN got %d, body=%v", noAIResp.StatusCode, noAIBody)
	}

	aiOnlyResp, aiOnlyBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/ai/project-weekly-report", aiOnlyToken, map[string]any{
		"projectId": projectID,
	})
	if aiOnlyResp.StatusCode != http.StatusForbidden || aiOnlyBody["code"] != "AI_SOURCE_PERMISSION_REQUIRED" {
		t.Fatalf("ai without projects.read expected 403 AI_SOURCE_PERMISSION_REQUIRED got %d, body=%v", aiOnlyResp.StatusCode, aiOnlyBody)
	}

	hiddenResp, hiddenBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/ai/project-risk-summary", fullToken, map[string]any{
		"projectId": hiddenProjectID,
	})
	if hiddenResp.StatusCode != http.StatusNotFound || hiddenBody["code"] != "PROJECT_NOT_FOUND" {
		t.Fatalf("hidden ai project expected 404 PROJECT_NOT_FOUND got %d, body=%v", hiddenResp.StatusCode, hiddenBody)
	}

	weeklyResp, weeklyBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/ai/project-weekly-report", fullToken, map[string]any{
		"projectId": projectID,
		"weekStart": "2026-01-01T00:00:00Z",
		"weekEnd":   "2026-12-31T23:59:59Z",
	})
	if weeklyResp.StatusCode != http.StatusOK {
		t.Fatalf("weekly ai draft expected 200 got %d, body=%v", weeklyResp.StatusCode, weeklyBody)
	}
	if weeklyBody["requiresConfirmation"] != true || weeklyBody["mode"] != "weekly_report" {
		t.Fatalf("weekly ai draft should require confirmation and mode weekly_report: %v", weeklyBody)
	}
	weeklyDraft, _ := weeklyBody["draft"].(string)
	if !strings.Contains(weeklyDraft, "AI Visible Project") || !strings.Contains(weeklyDraft, "上线窗口被压缩") {
		t.Fatalf("weekly draft should include visible project and register context, got: %s", weeklyDraft)
	}
	weeklyRaw := fmt.Sprint(weeklyBody)
	if strings.Contains(weeklyRaw, "AI-CNT-SECRET") || strings.Contains(weeklyRaw, "98765") {
		t.Fatalf("weekly draft must not leak project finance fields, got: %v", weeklyBody)
	}
	sourceRefs, _ := weeklyBody["sourceRefs"].([]any)
	if len(sourceRefs) < 2 {
		t.Fatalf("weekly draft should include traceable source refs, got: %v", weeklyBody["sourceRefs"])
	}

	streamPayload, _ := json.Marshal(map[string]any{
		"projectId": projectID,
		"weekStart": "2026-01-01T00:00:00Z",
		"weekEnd":   "2026-12-31T23:59:59Z",
	})
	streamReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/ai/project-weekly-report/stream", bytes.NewReader(streamPayload))
	streamReq.Header.Set("Authorization", "Bearer "+fullToken)
	streamReq.Header.Set("Content-Type", "application/json")
	streamReq.Header.Set("Accept", "text/event-stream")
	streamResp, err := http.DefaultClient.Do(streamReq)
	if err != nil {
		t.Fatalf("weekly ai stream request failed: %v", err)
	}
	streamRaw, _ := io.ReadAll(streamResp.Body)
	streamResp.Body.Close()
	streamText := string(streamRaw)
	if streamResp.StatusCode != http.StatusOK {
		t.Fatalf("weekly ai stream expected 200 got %d, body=%s", streamResp.StatusCode, streamText)
	}
	if !strings.Contains(streamResp.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("weekly ai stream should use text/event-stream, got %q", streamResp.Header.Get("Content-Type"))
	}
	if !strings.Contains(streamText, "event: status") || !strings.Contains(streamText, "event: result") || !strings.Contains(streamText, `"mode":"weekly_report"`) {
		t.Fatalf("weekly ai stream should include status and result events, got: %s", streamText)
	}

	riskResp, riskBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/ai/project-risk-summary", fullToken, map[string]any{
		"projectId": projectID,
	})
	if riskResp.StatusCode != http.StatusOK {
		t.Fatalf("risk ai summary expected 200 got %d, body=%v", riskResp.StatusCode, riskBody)
	}
	riskDraft, _ := riskBody["draft"].(string)
	if !strings.Contains(riskDraft, "上线窗口被压缩") || !strings.Contains(riskDraft, "逾期") {
		t.Fatalf("risk draft should include visible risk and overdue reason, got: %s", riskDraft)
	}

	taskListBeforeResp, taskListBeforeBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/tasks?projectId="+strconv.Itoa(projectID), fullToken, nil)
	if taskListBeforeResp.StatusCode != http.StatusOK {
		t.Fatalf("list tasks before ai breakdown expected 200 got %d, body=%v", taskListBeforeResp.StatusCode, taskListBeforeBody)
	}
	breakdownResp, breakdownBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/ai/task-breakdown", fullToken, map[string]any{
		"projectId":   projectID,
		"title":       "AI Visible Project",
		"description": "需要上线内部协作能力并准备回滚预案",
	})
	if breakdownResp.StatusCode != http.StatusOK {
		t.Fatalf("task breakdown expected 200 got %d, body=%v", breakdownResp.StatusCode, breakdownBody)
	}
	if breakdownBody["requiresConfirmation"] != true || breakdownBody["mode"] != "task_breakdown" {
		t.Fatalf("task breakdown should require confirmation and mode task_breakdown: %v", breakdownBody)
	}
	suggestedTasks, _ := breakdownBody["tasks"].([]any)
	if len(suggestedTasks) < 4 {
		t.Fatalf("task breakdown should return task drafts, got: %v", breakdownBody["tasks"])
	}
	taskListAfterResp, taskListAfterBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/tasks?projectId="+strconv.Itoa(projectID), fullToken, nil)
	if taskListAfterResp.StatusCode != http.StatusOK {
		t.Fatalf("list tasks after ai breakdown expected 200 got %d, body=%v", taskListAfterResp.StatusCode, taskListAfterBody)
	}
	if taskListBeforeBody["total"].(float64) != taskListAfterBody["total"].(float64) {
		t.Fatalf("task breakdown must not persist tasks, before=%v after=%v", taskListBeforeBody["total"], taskListAfterBody["total"])
	}
}

func TestAIAssistantStreamEmitsModelDeltas(t *testing.T) {
	ts := setupTestRouterWithHandler(t, func(h *handler.Handler) {
		h.AIClient = streamingAIClient{}
	})
	defer ts.Close()

	token := loginAndToken(t, ts.URL)
	projectResp, projectBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/projects", token, map[string]any{
		"code":        "AI-STREAM",
		"name":        "AI Stream Project",
		"description": "streaming test",
	})
	if projectResp.StatusCode != http.StatusCreated {
		t.Fatalf("create stream project expected 201 got %d, body=%v", projectResp.StatusCode, projectBody)
	}
	projectID := int(projectBody["id"].(float64))

	payload, _ := json.Marshal(map[string]any{
		"projectId": projectID,
		"weekStart": "2026-01-01T00:00:00Z",
		"weekEnd":   "2026-12-31T23:59:59Z",
	})
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/ai/project-weekly-report/stream", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("weekly ai stream request failed: %v", err)
	}
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	text := string(raw)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("weekly ai stream expected 200 got %d, body=%s", resp.StatusCode, text)
	}
	if !strings.Contains(text, "event: delta") || !strings.Contains(text, `"text":"流"`) {
		t.Fatalf("weekly ai stream should include model delta events, got: %s", text)
	}
	if !strings.Contains(text, `"draft":"流式正文"`) {
		t.Fatalf("weekly ai stream result should use streamed model output, got: %s", text)
	}
	if deltaIndex, resultIndex := strings.Index(text, "event: delta"), strings.Index(text, "event: result"); deltaIndex < 0 || resultIndex < 0 || deltaIndex > resultIndex {
		t.Fatalf("delta events should arrive before result event, got: %s", text)
	}
}

func TestTaskWorkHoursCreateUpdateValidationAndExport(t *testing.T) {
	ts := setupTestRouter(t)
	defer ts.Close()

	token := loginAndToken(t, ts.URL)

	projectResp, projectBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/projects", token, map[string]any{
		"code":        "HOURS-P1",
		"name":        "工时项目",
		"description": "work hours",
	})
	if projectResp.StatusCode != http.StatusCreated {
		t.Fatalf("create work hours project expected 201 got %d", projectResp.StatusCode)
	}
	projectID := int(projectBody["id"].(float64))

	createResp, createBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/tasks", token, map[string]any{
		"title":          "工时任务",
		"projectId":      projectID,
		"status":         "processing",
		"progress":       25,
		"estimatedHours": 12.5,
		"actualHours":    2.0,
		"remainingHours": 10.5,
	})
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("create work hours task expected 201 got %d, body=%v", createResp.StatusCode, createBody)
	}
	taskID := int(createBody["id"].(float64))
	if createBody["estimatedHours"].(float64) != 12.5 || createBody["actualHours"].(float64) != 2 || createBody["remainingHours"].(float64) != 10.5 {
		t.Fatalf("work hours not saved on create: %v", createBody)
	}

	invalidResp, invalidBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/tasks", token, map[string]any{
		"title":          "非法工时任务",
		"projectId":      projectID,
		"status":         "pending",
		"progress":       0,
		"estimatedHours": -1,
	})
	if invalidResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("negative task hours expected 400 got %d", invalidResp.StatusCode)
	}
	if invalidBody["code"] != "INVALID_TASK_HOURS" {
		t.Fatalf("negative task hours expected INVALID_TASK_HOURS got %v", invalidBody["code"])
	}

	updateResp, updateBody := requestJSON(t, http.MethodPut, ts.URL+"/api/v1/tasks/"+strconv.Itoa(taskID), token, map[string]any{
		"title":          "工时任务更新",
		"projectId":      projectID,
		"status":         "processing",
		"progress":       60,
		"estimatedHours": 6.5,
		"actualHours":    3.0,
		"remainingHours": 3.5,
	})
	if updateResp.StatusCode != http.StatusOK {
		t.Fatalf("update work hours task expected 200 got %d, body=%v", updateResp.StatusCode, updateBody)
	}
	if updateBody["estimatedHours"].(float64) != 6.5 || updateBody["actualHours"].(float64) != 3 || updateBody["remainingHours"].(float64) != 3.5 {
		t.Fatalf("work hours not saved on update: %v", updateBody)
	}

	exportReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/tasks/export", nil)
	exportReq.Header.Set("Authorization", "Bearer "+token)
	exportResp, err := http.DefaultClient.Do(exportReq)
	if err != nil {
		t.Fatalf("export work hours tasks failed: %v", err)
	}
	exportRaw, _ := io.ReadAll(exportResp.Body)
	exportResp.Body.Close()
	exportText := string(exportRaw)
	if !strings.Contains(exportText, "估算工时") || !strings.Contains(exportText, "工时任务更新") || !strings.Contains(exportText, "6.50") {
		t.Fatalf("task export should contain work hour columns and values, got: %s", exportText)
	}
}

func TestTaskCalendarAndICSUseVisibleScope(t *testing.T) {
	ts := setupTestRouter(t)
	defer ts.Close()

	adminToken := loginAndToken(t, ts.URL)
	codeToID := permissionCodeMap(t, ts.URL, adminToken)
	roleResp, roleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/rbac/roles", adminToken, map[string]any{
		"name":        "calendar-reader",
		"description": "calendar reader",
		"permissionIds": []uint{
			codeToID["tasks.read"],
		},
	})
	if roleResp.StatusCode != http.StatusCreated {
		t.Fatalf("create calendar role expected 201 got %d", roleResp.StatusCode)
	}
	roleID := uint(roleBody["id"].(float64))

	readerResp, readerBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":      "calendar_reader",
		"name":          "Calendar Reader",
		"email":         "calendar_reader@example.com",
		"password":      "pass1234",
		"roleIds":       []uint{roleID},
		"departmentIds": []uint{},
	})
	if readerResp.StatusCode != http.StatusCreated {
		t.Fatalf("create calendar reader expected 201 got %d", readerResp.StatusCode)
	}
	readerID := uint(readerBody["id"].(float64))

	hiddenResp, hiddenBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":      "calendar_hidden",
		"name":          "Calendar Hidden",
		"email":         "calendar_hidden@example.com",
		"password":      "pass1234",
		"roleIds":       []uint{},
		"departmentIds": []uint{},
	})
	if hiddenResp.StatusCode != http.StatusCreated {
		t.Fatalf("create hidden calendar user expected 201 got %d", hiddenResp.StatusCode)
	}
	hiddenID := uint(hiddenBody["id"].(float64))

	projectResp, projectBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/projects", adminToken, map[string]any{
		"code":        "CAL-P1",
		"name":        "日程项目",
		"description": "calendar",
	})
	if projectResp.StatusCode != http.StatusCreated {
		t.Fatalf("create calendar project expected 201 got %d", projectResp.StatusCode)
	}
	projectID := int(projectBody["id"].(float64))

	visibleTaskResp, visibleTaskBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/tasks", adminToken, map[string]any{
		"title":       "Visible Calendar Task",
		"projectId":   projectID,
		"status":      "processing",
		"progress":    30,
		"startAt":     "2026-04-08T09:00:00Z",
		"endAt":       "2026-04-08T11:00:00Z",
		"assigneeIds": []uint{readerID},
	})
	if visibleTaskResp.StatusCode != http.StatusCreated {
		t.Fatalf("create visible calendar task expected 201 got %d, body=%v", visibleTaskResp.StatusCode, visibleTaskBody)
	}
	visibleTaskID := int(visibleTaskBody["id"].(float64))

	reviewTaskResp, _ := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/tasks", adminToken, map[string]any{
		"title":       "Review Calendar Task",
		"projectId":   projectID,
		"status":      "reviewing",
		"progress":    100,
		"endAt":       "2026-04-10T12:00:00Z",
		"reviewerIds": []uint{readerID},
	})
	if reviewTaskResp.StatusCode != http.StatusCreated {
		t.Fatalf("create review calendar task expected 201 got %d", reviewTaskResp.StatusCode)
	}

	_, _ = requestJSON(t, http.MethodPost, ts.URL+"/api/v1/tasks", adminToken, map[string]any{
		"title":       "Hidden Calendar Task",
		"projectId":   projectID,
		"status":      "processing",
		"progress":    20,
		"startAt":     "2026-04-08T09:00:00Z",
		"endAt":       "2026-04-08T11:00:00Z",
		"assigneeIds": []uint{hiddenID},
	})
	_, _ = requestJSON(t, http.MethodPost, ts.URL+"/api/v1/tasks", adminToken, map[string]any{
		"title":       "Outside Calendar Task",
		"projectId":   projectID,
		"status":      "processing",
		"progress":    20,
		"startAt":     "2026-05-08T09:00:00Z",
		"endAt":       "2026-05-08T11:00:00Z",
		"assigneeIds": []uint{readerID},
	})

	loginStatus, loginBody := loginWithCredentials(t, ts.URL, "calendar_reader", "pass1234")
	if loginStatus != http.StatusOK {
		t.Fatalf("login calendar reader expected 200 got %d", loginStatus)
	}
	readerToken := loginBody["token"].(string)

	query := url.Values{}
	query.Set("start", "2026-04-01T00:00:00Z")
	query.Set("end", "2026-04-30T23:59:59Z")
	query.Set("mine", "true")
	calendarResp, calendarBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/tasks/calendar?"+query.Encode(), readerToken, nil)
	if calendarResp.StatusCode != http.StatusOK {
		t.Fatalf("calendar expected 200 got %d, body=%v", calendarResp.StatusCode, calendarBody)
	}
	items, _ := calendarBody["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("calendar reader should see two visible April items got %v", calendarBody)
	}
	foundVisible := false
	for _, raw := range items {
		item, _ := raw.(map[string]any)
		if int(item["id"].(float64)) == visibleTaskID {
			foundVisible = true
			if item["projectName"] != "日程项目" {
				t.Fatalf("calendar item should include project name got %v", item)
			}
		}
		if item["title"] == "Hidden Calendar Task" || item["title"] == "Outside Calendar Task" {
			t.Fatalf("calendar leaked hidden or out-of-range task: %v", item)
		}
	}
	if !foundVisible {
		t.Fatalf("calendar should include visible task id %d got %v", visibleTaskID, items)
	}

	icsReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/tasks/calendar.ics?"+query.Encode(), nil)
	icsReq.Header.Set("Authorization", "Bearer "+readerToken)
	icsResp, err := http.DefaultClient.Do(icsReq)
	if err != nil {
		t.Fatalf("calendar ics request failed: %v", err)
	}
	icsRaw, _ := io.ReadAll(icsResp.Body)
	icsResp.Body.Close()
	icsText := string(icsRaw)
	if icsResp.StatusCode != http.StatusOK {
		t.Fatalf("calendar ics expected 200 got %d: %s", icsResp.StatusCode, icsText)
	}
	if !strings.Contains(icsResp.Header.Get("Content-Type"), "text/calendar") {
		t.Fatalf("calendar ics content type unexpected: %s", icsResp.Header.Get("Content-Type"))
	}
	if !strings.Contains(icsText, "BEGIN:VCALENDAR") || !strings.Contains(icsText, "Visible Calendar Task") || strings.Contains(icsText, "Hidden Calendar Task") {
		t.Fatalf("calendar ics content unexpected: %s", icsText)
	}

	invalidQuery := url.Values{}
	invalidQuery.Set("start", "2026-04-30T00:00:00Z")
	invalidQuery.Set("end", "2026-04-01T00:00:00Z")
	invalidResp, invalidBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/tasks/calendar?"+invalidQuery.Encode(), readerToken, nil)
	if invalidResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid calendar range expected 400 got %d", invalidResp.StatusCode)
	}
	if invalidBody["code"] != "INVALID_CALENDAR_RANGE" {
		t.Fatalf("invalid calendar range expected INVALID_CALENDAR_RANGE got %v", invalidBody["code"])
	}
}

func TestProjectHealthUsesVisibleTaskScope(t *testing.T) {
	ts := setupTestRouter(t)
	defer ts.Close()

	adminToken := loginAndToken(t, ts.URL)
	codeToID := permissionCodeMap(t, ts.URL, adminToken)

	roleResp, roleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/rbac/roles", adminToken, map[string]any{
		"name":        "health-reader",
		"description": "health reader",
		"permissionIds": []uint{
			codeToID["projects.read"],
			codeToID["tasks.read"],
			codeToID["stats.read"],
		},
	})
	if roleResp.StatusCode != http.StatusCreated {
		t.Fatalf("create health role expected 201 got %d", roleResp.StatusCode)
	}
	roleID := uint(roleBody["id"].(float64))

	userResp, userBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":      "health_reader",
		"name":          "Health Reader",
		"email":         "health_reader@example.com",
		"password":      "pass1234",
		"roleIds":       []uint{roleID},
		"departmentIds": []uint{},
	})
	if userResp.StatusCode != http.StatusCreated {
		t.Fatalf("create health user expected 201 got %d", userResp.StatusCode)
	}
	userID := uint(userBody["id"].(float64))

	visibleProjectResp, visibleProjectBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/projects", adminToken, map[string]any{
		"code":        "HEALTH-P1",
		"name":        "Health Visible",
		"description": "visible",
	})
	if visibleProjectResp.StatusCode != http.StatusCreated {
		t.Fatalf("create visible health project expected 201 got %d", visibleProjectResp.StatusCode)
	}
	visibleProjectID := int(visibleProjectBody["id"].(float64))

	hiddenProjectResp, hiddenProjectBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/projects", adminToken, map[string]any{
		"code":        "HEALTH-P2",
		"name":        "Health Hidden",
		"description": "hidden",
	})
	if hiddenProjectResp.StatusCode != http.StatusCreated {
		t.Fatalf("create hidden health project expected 201 got %d", hiddenProjectResp.StatusCode)
	}
	hiddenProjectID := int(hiddenProjectBody["id"].(float64))

	_, _ = requestJSON(t, http.MethodPost, ts.URL+"/api/v1/tasks", adminToken, map[string]any{
		"title":       "Visible Overdue Milestone",
		"projectId":   visibleProjectID,
		"status":      "processing",
		"progress":    20,
		"isMilestone": true,
		"startAt":     "2026-01-01T10:00:00Z",
		"endAt":       "2026-01-02T10:00:00Z",
		"assigneeIds": []uint{userID},
	})
	_, _ = requestJSON(t, http.MethodPost, ts.URL+"/api/v1/tasks", adminToken, map[string]any{
		"title":       "Visible Unscheduled",
		"projectId":   visibleProjectID,
		"status":      "reviewing",
		"progress":    100,
		"assigneeIds": []uint{userID},
	})
	_, _ = requestJSON(t, http.MethodPost, ts.URL+"/api/v1/tasks", adminToken, map[string]any{
		"title":     "Hidden Overdue",
		"projectId": hiddenProjectID,
		"status":    "processing",
		"progress":  10,
		"startAt":   "2026-01-01T10:00:00Z",
		"endAt":     "2026-01-02T10:00:00Z",
	})

	healthResp, healthBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/stats/project-health", adminToken, nil)
	if healthResp.StatusCode != http.StatusOK {
		t.Fatalf("admin health expected 200 got %d", healthResp.StatusCode)
	}
	adminProjects, _ := healthBody["projects"].([]any)
	if len(adminProjects) < 2 {
		t.Fatalf("admin should see both health projects got %v", healthBody)
	}

	loginStatus, loginBody := loginWithCredentials(t, ts.URL, "health_reader", "pass1234")
	if loginStatus != http.StatusOK {
		t.Fatalf("login health reader expected 200 got %d", loginStatus)
	}
	userToken := loginBody["token"].(string)
	scopedResp, scopedBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/stats/project-health", userToken, nil)
	if scopedResp.StatusCode != http.StatusOK {
		t.Fatalf("scoped health expected 200 got %d", scopedResp.StatusCode)
	}
	scopedProjects, _ := scopedBody["projects"].([]any)
	if len(scopedProjects) != 1 {
		t.Fatalf("scoped user should see one health project got %v", scopedBody)
	}
	project, _ := scopedProjects[0].(map[string]any)
	if int(project["projectId"].(float64)) != visibleProjectID {
		t.Fatalf("scoped project id expected %d got %v", visibleProjectID, project["projectId"])
	}
	if project["health"] != "red" {
		t.Fatalf("visible project should be red got %v", project["health"])
	}
	if int(project["overdueTasks"].(float64)) != 1 || int(project["milestoneOverdue"].(float64)) != 1 {
		t.Fatalf("expected overdue and milestone counts got %v", project)
	}
	if int(project["unscheduledTasks"].(float64)) != 1 || int(project["reviewingTasks"].(float64)) != 1 {
		t.Fatalf("expected unscheduled and reviewing counts got %v", project)
	}
}

func TestProjectRegistersCRUDScopeAndHealth(t *testing.T) {
	ts := setupTestRouter(t)
	defer ts.Close()

	adminToken := loginAndToken(t, ts.URL)
	codeToID := permissionCodeMap(t, ts.URL, adminToken)
	for _, code := range []string{
		"projects.read",
		"registers.create",
		"registers.read",
		"registers.update",
		"registers.delete",
		"stats.read",
		"notifications.read",
	} {
		if codeToID[code] == 0 {
			t.Fatalf("permission seed missing: %s", code)
		}
	}

	roleResp, roleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/rbac/roles", adminToken, map[string]any{
		"name":        "register-scope-editor",
		"description": "register scope editor",
		"permissionIds": []uint{
			codeToID["projects.read"],
			codeToID["registers.create"],
			codeToID["registers.read"],
			codeToID["registers.update"],
			codeToID["registers.delete"],
			codeToID["stats.read"],
			codeToID["notifications.read"],
		},
	})
	if roleResp.StatusCode != http.StatusCreated {
		t.Fatalf("create register role expected 201 got %d, body=%v", roleResp.StatusCode, roleBody)
	}
	roleID := uint(roleBody["id"].(float64))

	userResp, userBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":      "register_scope_u1",
		"name":          "Register Scope User",
		"email":         "register_scope_u1@example.com",
		"password":      "pass1234",
		"roleIds":       []uint{roleID},
		"departmentIds": []uint{},
	})
	if userResp.StatusCode != http.StatusCreated {
		t.Fatalf("create register user expected 201 got %d, body=%v", userResp.StatusCode, userBody)
	}
	userID := uint(userBody["id"].(float64))

	visibleProjectResp, visibleProjectBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/projects", adminToken, map[string]any{
		"code":          "REGISTER-P1",
		"name":          "Register Visible",
		"description":   "visible register project",
		"userIds":       []uint{userID},
		"departmentIds": []uint{},
	})
	if visibleProjectResp.StatusCode != http.StatusCreated {
		t.Fatalf("create visible register project expected 201 got %d, body=%v", visibleProjectResp.StatusCode, visibleProjectBody)
	}
	visibleProjectID := int(visibleProjectBody["id"].(float64))

	hiddenProjectResp, hiddenProjectBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/projects", adminToken, map[string]any{
		"code":        "REGISTER-P2",
		"name":        "Register Hidden",
		"description": "hidden register project",
	})
	if hiddenProjectResp.StatusCode != http.StatusCreated {
		t.Fatalf("create hidden register project expected 201 got %d, body=%v", hiddenProjectResp.StatusCode, hiddenProjectBody)
	}
	hiddenProjectID := int(hiddenProjectBody["id"].(float64))

	loginStatus, loginBody := loginWithCredentials(t, ts.URL, "register_scope_u1", "pass1234")
	if loginStatus != http.StatusOK {
		t.Fatalf("login register scope user expected 200 got %d", loginStatus)
	}
	userToken := loginBody["token"].(string)

	riskPayload := map[string]any{
		"type":           "risk",
		"projectId":      visibleProjectID,
		"title":          "供应商交付延期风险",
		"description":    "核心物料可能晚于计划到达",
		"status":         "open",
		"severity":       "high",
		"probability":    "high",
		"impact":         "critical",
		"source":         "周会",
		"responsePlan":   "准备替代供应商",
		"dueAt":          "2026-07-15T00:00:00Z",
		"ownerId":        userID,
		"participantIds": []uint{userID},
	}
	riskResp, riskBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/project-registers", userToken, riskPayload)
	if riskResp.StatusCode != http.StatusCreated {
		t.Fatalf("create risk register expected 201 got %d, body=%v", riskResp.StatusCode, riskBody)
	}
	riskID := int(riskBody["id"].(float64))

	issueResp, issueBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/project-registers", userToken, map[string]any{
		"type":        "issue",
		"projectId":   visibleProjectID,
		"title":       "验收环境不可用",
		"description": "测试团队无法进入验收环境",
		"status":      "in_progress",
		"severity":    "medium",
		"source":      "测试日报",
		"ownerId":     userID,
	})
	if issueResp.StatusCode != http.StatusCreated {
		t.Fatalf("create issue register expected 201 got %d, body=%v", issueResp.StatusCode, issueBody)
	}

	hiddenCreateResp, hiddenCreateBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/project-registers", userToken, map[string]any{
		"type":      "risk",
		"projectId": hiddenProjectID,
		"title":     "隐藏项目风险",
		"status":    "open",
		"severity":  "critical",
	})
	if hiddenCreateResp.StatusCode != http.StatusNotFound || hiddenCreateBody["code"] != "PROJECT_NOT_FOUND" {
		t.Fatalf("create hidden project register expected 404 PROJECT_NOT_FOUND got %d, body=%v", hiddenCreateResp.StatusCode, hiddenCreateBody)
	}

	listResp, listBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/project-registers", userToken, nil)
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("list registers expected 200 got %d, body=%v", listResp.StatusCode, listBody)
	}
	if int(listBody["total"].(float64)) != 2 {
		t.Fatalf("scoped register list should contain two visible registers got %v", listBody)
	}
	list, _ := listBody["list"].([]any)
	for _, raw := range list {
		item, _ := raw.(map[string]any)
		if int(item["projectId"].(float64)) != visibleProjectID {
			t.Fatalf("scoped register list leaked hidden project item: %v", item)
		}
	}

	filterResp, filterBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/project-registers?type=risk&severities=high,critical", userToken, nil)
	if filterResp.StatusCode != http.StatusOK {
		t.Fatalf("filter high risks expected 200 got %d, body=%v", filterResp.StatusCode, filterBody)
	}
	if int(filterBody["total"].(float64)) != 1 {
		t.Fatalf("high risk filter should return one item got %v", filterBody)
	}

	updatePayload := map[string]any{
		"type":           "risk",
		"projectId":      visibleProjectID,
		"title":          "供应商交付延期风险已升级",
		"description":    "核心物料交付风险升级，需要管理层介入",
		"status":         "open",
		"severity":       "critical",
		"probability":    "high",
		"impact":         "critical",
		"source":         "周会",
		"responsePlan":   "启动替代供应商并每日跟进",
		"dueAt":          "2026-07-15T00:00:00Z",
		"ownerId":        userID,
		"participantIds": []uint{userID},
	}
	updateResp, updateBody := requestJSON(t, http.MethodPut, ts.URL+"/api/v1/project-registers/"+strconv.Itoa(riskID), userToken, updatePayload)
	if updateResp.StatusCode != http.StatusOK {
		t.Fatalf("update risk register expected 200 got %d, body=%v", updateResp.StatusCode, updateBody)
	}
	if updateBody["severity"] != "critical" || updateBody["title"] != "供应商交付延期风险已升级" {
		t.Fatalf("updated register should return changed fields got %v", updateBody)
	}

	activityResp, activityBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/project-registers/"+strconv.Itoa(riskID)+"/activities", userToken, nil)
	if activityResp.StatusCode != http.StatusOK {
		t.Fatalf("list register activities expected 200 got %d, body=%v", activityResp.StatusCode, activityBody)
	}
	if int(activityBody["total"].(float64)) < 2 {
		t.Fatalf("register should have create and update activities got %v", activityBody)
	}
	activities, _ := activityBody["list"].([]any)
	foundUpdateActivity := false
	for _, raw := range activities {
		item, _ := raw.(map[string]any)
		if item["type"] == "register.updated" && strings.Contains(item["summary"].(string), "更新") {
			foundUpdateActivity = true
			break
		}
	}
	if !foundUpdateActivity {
		t.Fatalf("expected register.updated activity got %v", activityBody)
	}

	healthResp, healthBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/stats/project-health", userToken, nil)
	if healthResp.StatusCode != http.StatusOK {
		t.Fatalf("register project health expected 200 got %d, body=%v", healthResp.StatusCode, healthBody)
	}
	projects, _ := healthBody["projects"].([]any)
	if len(projects) != 1 {
		t.Fatalf("scoped health should only include visible register project got %v", healthBody)
	}
	healthProject, _ := projects[0].(map[string]any)
	if int(healthProject["projectId"].(float64)) != visibleProjectID {
		t.Fatalf("health project id expected %d got %v", visibleProjectID, healthProject["projectId"])
	}
	if healthProject["health"] != "red" || int(healthProject["highRiskRegisters"].(float64)) != 1 || int(healthProject["unresolvedIssues"].(float64)) != 1 {
		t.Fatalf("register health should be red with one high risk and one issue got %v", healthProject)
	}

	notificationResp, notificationBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/notifications?module=project_registers", userToken, nil)
	if notificationResp.StatusCode != http.StatusOK {
		t.Fatalf("register notifications expected 200 got %d, body=%v", notificationResp.StatusCode, notificationBody)
	}
	notifications, _ := notificationBody["list"].([]any)
	foundRiskNotification := false
	for _, raw := range notifications {
		item, _ := raw.(map[string]any)
		if item["targetId"] == float64(riskID) && strings.Contains(item["title"].(string), "登记项") {
			foundRiskNotification = true
			break
		}
	}
	if !foundRiskNotification {
		t.Fatalf("expected register notification for risk got %v", notificationBody)
	}

	deleteResp, deleteBody := requestJSON(t, http.MethodDelete, ts.URL+"/api/v1/project-registers/"+strconv.Itoa(riskID), userToken, nil)
	if deleteResp.StatusCode != http.StatusOK {
		t.Fatalf("delete risk register expected 200 got %d, body=%v", deleteResp.StatusCode, deleteBody)
	}
	detailAfterDeleteResp, _ := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/project-registers/"+strconv.Itoa(riskID), userToken, nil)
	if detailAfterDeleteResp.StatusCode != http.StatusNotFound {
		t.Fatalf("deleted register detail expected 404 got %d", detailAfterDeleteResp.StatusCode)
	}
}

func TestProjectBaselineAndCriticalPathMVP(t *testing.T) {
	ts := setupTestRouter(t)
	defer ts.Close()

	adminToken := loginAndToken(t, ts.URL)
	codeToID := permissionCodeMap(t, ts.URL, adminToken)
	for _, code := range []string{"baselines.create", "baselines.read", "baselines.delete"} {
		if codeToID[code] == 0 {
			t.Fatalf("baseline permission seed missing: %s", code)
		}
	}

	projectResp, projectBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/projects", adminToken, map[string]any{
		"code":        "BASELINE-P1",
		"name":        "Baseline Project",
		"description": "baseline project",
	})
	if projectResp.StatusCode != http.StatusCreated {
		t.Fatalf("create baseline project expected 201 got %d, body=%v", projectResp.StatusCode, projectBody)
	}
	projectID := int(projectBody["id"].(float64))

	createTask := func(title, status string, progress int, startAt, endAt string) int {
		resp, body := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/tasks", adminToken, map[string]any{
			"title":     title,
			"projectId": projectID,
			"status":    status,
			"progress":  progress,
			"startAt":   startAt,
			"endAt":     endAt,
		})
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("create task %s expected 201 got %d, body=%v", title, resp.StatusCode, body)
		}
		return int(body["id"].(float64))
	}

	taskAID := createTask("Baseline A", "completed", 100, "2000-01-01T00:00:00Z", "2000-01-02T00:00:00Z")
	taskBID := createTask("Baseline B", "completed", 100, "2000-01-02T00:00:00Z", "2000-01-04T00:00:00Z")
	taskCID := createTask("Baseline C", "processing", 60, "2000-01-04T00:00:00Z", "2000-01-05T00:00:00Z")

	dependencyResp, dependencyBody := requestJSON(t, http.MethodPut, ts.URL+"/api/v1/tasks/"+strconv.Itoa(taskBID)+"/dependencies", adminToken, map[string]any{
		"dependencies": []map[string]any{{"dependsOnTaskId": taskAID, "lagDays": 0, "type": "FS"}},
	})
	if dependencyResp.StatusCode != http.StatusOK {
		t.Fatalf("set B dependencies expected 200 got %d, body=%v", dependencyResp.StatusCode, dependencyBody)
	}
	dependencyResp, dependencyBody = requestJSON(t, http.MethodPut, ts.URL+"/api/v1/tasks/"+strconv.Itoa(taskCID)+"/dependencies", adminToken, map[string]any{
		"dependencies": []map[string]any{{"dependsOnTaskId": taskBID, "lagDays": 0, "type": "FS"}},
	})
	if dependencyResp.StatusCode != http.StatusOK {
		t.Fatalf("set C dependencies expected 200 got %d, body=%v", dependencyResp.StatusCode, dependencyBody)
	}

	createBaselineResp, createBaselineBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/project-baselines", adminToken, map[string]any{
		"projectId":   projectID,
		"name":        "Baseline V1",
		"description": "initial plan",
	})
	if createBaselineResp.StatusCode != http.StatusCreated {
		t.Fatalf("create baseline expected 201 got %d, body=%v", createBaselineResp.StatusCode, createBaselineBody)
	}
	baselineID := int(createBaselineBody["id"].(float64))
	if int(createBaselineBody["taskCount"].(float64)) != 3 || int(createBaselineBody["completedTaskCount"].(float64)) != 2 {
		t.Fatalf("baseline snapshot counts unexpected: %v", createBaselineBody)
	}
	snapshot, _ := createBaselineBody["snapshot"].([]any)
	if len(snapshot) != 3 {
		t.Fatalf("baseline snapshot should contain three tasks got %v", createBaselineBody["snapshot"])
	}

	updateScheduleResp, updateScheduleBody := requestJSON(t, http.MethodPatch, ts.URL+"/api/v1/tasks/"+strconv.Itoa(taskCID)+"/schedule?autoResolve=false", adminToken, map[string]any{
		"startAt": "2000-01-06T00:00:00Z",
		"endAt":   "2000-01-08T00:00:00Z",
	})
	if updateScheduleResp.StatusCode != http.StatusOK {
		t.Fatalf("update C schedule expected 200 got %d, body=%v", updateScheduleResp.StatusCode, updateScheduleBody)
	}

	detailResp, detailBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/project-baselines/"+strconv.Itoa(baselineID), adminToken, nil)
	if detailResp.StatusCode != http.StatusOK {
		t.Fatalf("baseline detail expected 200 got %d, body=%v", detailResp.StatusCode, detailBody)
	}
	compare, _ := detailBody["compare"].(map[string]any)
	if int(compare["delayedTaskCount"].(float64)) != 1 || int(compare["endVarianceDays"].(float64)) <= 0 {
		t.Fatalf("baseline compare should detect delayed critical task got %v", compare)
	}
	changedTasks, _ := compare["changedTasks"].([]any)
	if len(changedTasks) == 0 {
		t.Fatalf("baseline compare should include changed tasks got %v", compare)
	}

	criticalResp, criticalBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/projects/"+strconv.Itoa(projectID)+"/critical-path", adminToken, nil)
	if criticalResp.StatusCode != http.StatusOK {
		t.Fatalf("critical path expected 200 got %d, body=%v", criticalResp.StatusCode, criticalBody)
	}
	criticalIDs, _ := criticalBody["criticalTaskIds"].([]any)
	if len(criticalIDs) != 3 ||
		int(criticalIDs[0].(float64)) != taskAID ||
		int(criticalIDs[1].(float64)) != taskBID ||
		int(criticalIDs[2].(float64)) != taskCID {
		t.Fatalf("critical path should follow A->B->C got %v", criticalIDs)
	}
	if int(criticalBody["totalDurationDays"].(float64)) < 5 {
		t.Fatalf("critical path duration should include updated C duration got %v", criticalBody)
	}

	healthResp, healthBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/stats/project-health", adminToken, nil)
	if healthResp.StatusCode != http.StatusOK {
		t.Fatalf("project health expected 200 got %d, body=%v", healthResp.StatusCode, healthBody)
	}
	projects, _ := healthBody["projects"].([]any)
	var healthProject map[string]any
	for _, raw := range projects {
		item, _ := raw.(map[string]any)
		if int(item["projectId"].(float64)) == projectID {
			healthProject = item
			break
		}
	}
	if healthProject == nil {
		t.Fatalf("project health should include baseline project got %v", healthBody)
	}
	if healthProject["health"] != "red" || int(healthProject["criticalOverdueTasks"].(float64)) != 1 {
		t.Fatalf("critical overdue should make project red got %v", healthProject)
	}
	reasons, _ := healthProject["reasons"].([]any)
	hasCriticalReason := false
	for _, raw := range reasons {
		if strings.Contains(raw.(string), "关键路径") {
			hasCriticalReason = true
		}
	}
	if !hasCriticalReason {
		t.Fatalf("health reasons should mention critical path got %v", reasons)
	}

	deleteBaselineResp, deleteBaselineBody := requestJSON(t, http.MethodDelete, ts.URL+"/api/v1/project-baselines/"+strconv.Itoa(baselineID), adminToken, nil)
	if deleteBaselineResp.StatusCode != http.StatusOK {
		t.Fatalf("delete baseline expected 200 got %d, body=%v", deleteBaselineResp.StatusCode, deleteBaselineBody)
	}
}

func TestMemberWorkloadUsesCapacityAndTaskScope(t *testing.T) {
	ts := setupTestRouter(t)
	defer ts.Close()

	adminToken := loginAndToken(t, ts.URL)
	codeToID := permissionCodeMap(t, ts.URL, adminToken)

	roleResp, roleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/rbac/roles", adminToken, map[string]any{
		"name":        "workload-reader",
		"description": "workload reader",
		"permissionIds": []uint{
			codeToID["projects.read"],
			codeToID["tasks.read"],
			codeToID["stats.read"],
		},
	})
	if roleResp.StatusCode != http.StatusCreated {
		t.Fatalf("create workload role expected 201 got %d", roleResp.StatusCode)
	}
	roleID := uint(roleBody["id"].(float64))

	visibleUserResp, visibleUserBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":            "workload_reader",
		"name":                "Workload Reader",
		"email":               "workload_reader@example.com",
		"password":            "pass1234",
		"weeklyCapacityHours": 10,
		"roleIds":             []uint{roleID},
		"departmentIds":       []uint{},
	})
	if visibleUserResp.StatusCode != http.StatusCreated {
		t.Fatalf("create workload reader expected 201 got %d", visibleUserResp.StatusCode)
	}
	visibleUserID := uint(visibleUserBody["id"].(float64))

	hiddenUserResp, hiddenUserBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":            "workload_hidden",
		"name":                "Workload Hidden",
		"email":               "workload_hidden@example.com",
		"password":            "pass1234",
		"weeklyCapacityHours": 40,
		"roleIds":             []uint{},
		"departmentIds":       []uint{},
	})
	if hiddenUserResp.StatusCode != http.StatusCreated {
		t.Fatalf("create hidden workload user expected 201 got %d", hiddenUserResp.StatusCode)
	}
	hiddenUserID := uint(hiddenUserBody["id"].(float64))

	visibleProjectResp, visibleProjectBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/projects", adminToken, map[string]any{
		"code":        "WORKLOAD-P1",
		"name":        "Workload Visible",
		"description": "visible workload",
	})
	if visibleProjectResp.StatusCode != http.StatusCreated {
		t.Fatalf("create visible workload project expected 201 got %d", visibleProjectResp.StatusCode)
	}
	visibleProjectID := int(visibleProjectBody["id"].(float64))

	hiddenProjectResp, hiddenProjectBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/projects", adminToken, map[string]any{
		"code":        "WORKLOAD-P2",
		"name":        "Workload Hidden",
		"description": "hidden workload",
	})
	if hiddenProjectResp.StatusCode != http.StatusCreated {
		t.Fatalf("create hidden workload project expected 201 got %d", hiddenProjectResp.StatusCode)
	}
	hiddenProjectID := int(hiddenProjectBody["id"].(float64))

	visibleTaskResp, visibleTaskBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/tasks", adminToken, map[string]any{
		"title":          "Visible Workload Task",
		"projectId":      visibleProjectID,
		"status":         "processing",
		"progress":       30,
		"estimatedHours": 12,
		"actualHours":    3,
		"remainingHours": 9,
		"assigneeIds":    []uint{visibleUserID},
	})
	if visibleTaskResp.StatusCode != http.StatusCreated {
		t.Fatalf("create visible workload task expected 201 got %d, body=%v", visibleTaskResp.StatusCode, visibleTaskBody)
	}

	_, _ = requestJSON(t, http.MethodPost, ts.URL+"/api/v1/tasks", adminToken, map[string]any{
		"title":          "Completed Workload Task",
		"projectId":      visibleProjectID,
		"status":         "completed",
		"progress":       100,
		"estimatedHours": 90,
		"actualHours":    90,
		"remainingHours": 0,
		"assigneeIds":    []uint{visibleUserID},
	})
	_, _ = requestJSON(t, http.MethodPost, ts.URL+"/api/v1/tasks", adminToken, map[string]any{
		"title":          "Hidden Workload Task",
		"projectId":      hiddenProjectID,
		"status":         "processing",
		"progress":       20,
		"estimatedHours": 50,
		"actualHours":    5,
		"remainingHours": 45,
		"assigneeIds":    []uint{hiddenUserID},
	})

	loginStatus, loginBody := loginWithCredentials(t, ts.URL, "workload_reader", "pass1234")
	if loginStatus != http.StatusOK {
		t.Fatalf("login workload reader expected 200 got %d", loginStatus)
	}
	userToken := loginBody["token"].(string)
	workloadResp, workloadBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/stats/member-workload", userToken, nil)
	if workloadResp.StatusCode != http.StatusOK {
		t.Fatalf("member workload expected 200 got %d, body=%v", workloadResp.StatusCode, workloadBody)
	}
	members, _ := workloadBody["members"].([]any)
	if len(members) != 1 {
		t.Fatalf("scoped workload should contain one member got %v", workloadBody)
	}
	member, _ := members[0].(map[string]any)
	if uint(member["userId"].(float64)) != visibleUserID {
		t.Fatalf("workload member should be visible user got %v", member["userId"])
	}
	if member["overloaded"] != true {
		t.Fatalf("workload member should be overloaded got %v", member)
	}
	if member["estimatedHours"].(float64) != 12 || member["capacityHours"].(float64) != 10 || member["taskCount"].(float64) != 1 {
		t.Fatalf("workload aggregation unexpected: %v", member)
	}
	if member["utilizationRate"].(float64) < 1.19 || member["utilizationRate"].(float64) > 1.21 {
		t.Fatalf("workload utilization expected about 1.2 got %v", member["utilizationRate"])
	}
}

func TestNotificationFlowOnTaskAssign(t *testing.T) {
	ts := setupTestRouter(t)
	defer ts.Close()

	adminToken := loginAndToken(t, ts.URL)

	permReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/system/rbac/permissions", nil)
	permReq.Header.Set("Authorization", "Bearer "+adminToken)
	permResp, err := http.DefaultClient.Do(permReq)
	if err != nil {
		t.Fatalf("query permissions failed: %v", err)
	}
	var permissions []map[string]any
	_ = json.NewDecoder(permResp.Body).Decode(&permissions)
	permResp.Body.Close()
	codeToID := map[string]uint{}
	for _, permission := range permissions {
		code, _ := permission["code"].(string)
		id, _ := permission["id"].(float64)
		codeToID[code] = uint(id)
	}

	roleResp, roleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/rbac/roles", adminToken, map[string]any{
		"name":        "notify-reader",
		"description": "notify reader",
		"permissionIds": []uint{
			codeToID["projects.read"], codeToID["tasks.read"], codeToID["notifications.read"], codeToID["notifications.update"],
		},
	})
	if roleResp.StatusCode != http.StatusCreated {
		t.Fatalf("create role status expected 201 got %d", roleResp.StatusCode)
	}
	roleID := uint(roleBody["id"].(float64))

	userResp, userBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":      "notify_u1",
		"name":          "Notify U1",
		"email":         "notify_u1@example.com",
		"password":      "pass1234",
		"roleIds":       []uint{roleID},
		"departmentIds": []uint{},
	})
	if userResp.StatusCode != http.StatusCreated {
		t.Fatalf("create user status expected 201 got %d", userResp.StatusCode)
	}
	userID := uint(userBody["id"].(float64))

	projectResp, projectBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/projects", adminToken, map[string]any{
		"code": "NOTIFY-P1", "name": "Notify Project", "description": "d",
	})
	if projectResp.StatusCode != http.StatusCreated {
		t.Fatalf("create project status expected 201 got %d", projectResp.StatusCode)
	}
	projectID := int(projectBody["id"].(float64))

	taskResp, taskBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/tasks", adminToken, map[string]any{
		"title":       "Notify Task",
		"projectId":   projectID,
		"status":      "pending",
		"progress":    0,
		"startAt":     "2026-03-24T10:00:00Z",
		"endAt":       "2026-03-25T10:00:00Z",
		"assigneeIds": []uint{userID},
	})
	if taskResp.StatusCode != http.StatusCreated {
		t.Fatalf("create task status expected 201 got %d", taskResp.StatusCode)
	}
	taskID := int(taskBody["id"].(float64))

	loginPayload := map[string]string{"username": "notify_u1", "password": "pass1234"}
	loginRaw, _ := json.Marshal(loginPayload)
	loginResp, err := http.Post(ts.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(loginRaw))
	if err != nil {
		t.Fatalf("login notify_u1 failed: %v", err)
	}
	var loginResult map[string]any
	_ = json.NewDecoder(loginResp.Body).Decode(&loginResult)
	loginResp.Body.Close()
	userToken := loginResult["token"].(string)

	countReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/notifications/unread-count", nil)
	countReq.Header.Set("Authorization", "Bearer "+userToken)
	countResp, err := http.DefaultClient.Do(countReq)
	if err != nil {
		t.Fatalf("count notifications failed: %v", err)
	}
	var countBody map[string]any
	_ = json.NewDecoder(countResp.Body).Decode(&countBody)
	countResp.Body.Close()
	if int(countBody["count"].(float64)) < 1 {
		t.Fatalf("expected unread count >=1 got %v", countBody["count"])
	}

	_, _ = requestJSON(t, http.MethodPut, ts.URL+"/api/v1/tasks/"+strconv.Itoa(taskID), adminToken, map[string]any{
		"title":       "Notify Task",
		"projectId":   projectID,
		"status":      "processing",
		"progress":    60,
		"startAt":     "2026-03-24T10:00:00Z",
		"endAt":       "2026-03-26T10:00:00Z",
		"assigneeIds": []uint{},
	})

	markAllReq, _ := http.NewRequest(http.MethodPatch, ts.URL+"/api/v1/notifications/read-all", nil)
	markAllReq.Header.Set("Authorization", "Bearer "+userToken)
	markAllResp, err := http.DefaultClient.Do(markAllReq)
	if err != nil {
		t.Fatalf("mark all notifications read failed: %v", err)
	}
	if markAllResp.StatusCode != http.StatusOK {
		t.Fatalf("mark all status expected 200 got %d", markAllResp.StatusCode)
	}
	markAllResp.Body.Close()
}

func TestTaskReviewerProgressReviewFlow(t *testing.T) {
	ts := setupTestRouter(t)
	defer ts.Close()

	adminToken := loginAndToken(t, ts.URL)
	codeToID := permissionCodeMap(t, ts.URL, adminToken)

	roleResp, roleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/rbac/roles", adminToken, map[string]any{
		"name":        "task-review-flow",
		"description": "task review flow",
		"permissionIds": []uint{
			codeToID["projects.read"],
			codeToID["tasks.read"],
			codeToID["notifications.read"],
			codeToID["notifications.update"],
		},
	})
	if roleResp.StatusCode != http.StatusCreated {
		t.Fatalf("create role status expected 201 got %d", roleResp.StatusCode)
	}
	roleID := uint(roleBody["id"].(float64))

	assigneeResp, assigneeBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":      "review_flow_assignee",
		"name":          "Review Flow Assignee",
		"email":         "review_flow_assignee@example.com",
		"password":      "pass1234",
		"roleIds":       []uint{roleID},
		"departmentIds": []uint{},
	})
	if assigneeResp.StatusCode != http.StatusCreated {
		t.Fatalf("create assignee status expected 201 got %d", assigneeResp.StatusCode)
	}
	assigneeID := uint(assigneeBody["id"].(float64))

	reviewerResp, reviewerBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":      "review_flow_reviewer",
		"name":          "Review Flow Reviewer",
		"email":         "review_flow_reviewer@example.com",
		"password":      "pass1234",
		"roleIds":       []uint{roleID},
		"departmentIds": []uint{},
	})
	if reviewerResp.StatusCode != http.StatusCreated {
		t.Fatalf("create reviewer status expected 201 got %d", reviewerResp.StatusCode)
	}
	reviewerID := uint(reviewerBody["id"].(float64))

	projectResp, projectBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/projects", adminToken, map[string]any{
		"code":        "REVIEW-FLOW-P1",
		"name":        "Review Flow Project",
		"description": "review flow",
	})
	if projectResp.StatusCode != http.StatusCreated {
		t.Fatalf("create project status expected 201 got %d", projectResp.StatusCode)
	}
	projectID := int(projectBody["id"].(float64))

	taskResp, taskBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/tasks", adminToken, map[string]any{
		"title":       "Review Flow Task",
		"projectId":   projectID,
		"status":      "processing",
		"progress":    20,
		"assigneeIds": []uint{assigneeID},
		"reviewerIds": []uint{reviewerID},
	})
	if taskResp.StatusCode != http.StatusCreated {
		t.Fatalf("create task status expected 201 got %d", taskResp.StatusCode)
	}
	taskID := int(taskBody["id"].(float64))

	assigneeLoginStatus, assigneeLoginBody := loginWithCredentials(t, ts.URL, "review_flow_assignee", "pass1234")
	if assigneeLoginStatus != http.StatusOK {
		t.Fatalf("login assignee expected 200 got %d", assigneeLoginStatus)
	}
	assigneeToken := assigneeLoginBody["token"].(string)
	reviewerLoginStatus, reviewerLoginBody := loginWithCredentials(t, ts.URL, "review_flow_reviewer", "pass1234")
	if reviewerLoginStatus != http.StatusOK {
		t.Fatalf("login reviewer expected 200 got %d", reviewerLoginStatus)
	}
	reviewerToken := reviewerLoginBody["token"].(string)

	progressResp, progressBody := requestJSON(t, http.MethodPatch, ts.URL+"/api/v1/tasks/"+strconv.Itoa(taskID)+"/progress", assigneeToken, map[string]any{
		"progress": 100,
	})
	if progressResp.StatusCode != http.StatusOK {
		t.Fatalf("update progress status expected 200 got %d", progressResp.StatusCode)
	}
	if progressBody["status"] != "reviewing" {
		t.Fatalf("progress 100 should move task to reviewing, got %v", progressBody["status"])
	}

	completeByAssigneeResp, _ := requestJSON(t, http.MethodPatch, ts.URL+"/api/v1/tasks/"+strconv.Itoa(taskID)+"/complete", assigneeToken, nil)
	if completeByAssigneeResp.StatusCode != http.StatusForbidden {
		t.Fatalf("assignee complete status expected 403 got %d", completeByAssigneeResp.StatusCode)
	}

	reviewerTasksResp, reviewerTasksBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/tasks", reviewerToken, nil)
	if reviewerTasksResp.StatusCode != http.StatusOK {
		t.Fatalf("reviewer list tasks status expected 200 got %d", reviewerTasksResp.StatusCode)
	}
	if list, ok := reviewerTasksBody["list"].([]any); !ok || len(list) == 0 {
		t.Fatalf("reviewer should see assigned review task, got %v", reviewerTasksBody["list"])
	}

	countResp, countBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/notifications/unread-count", reviewerToken, nil)
	if countResp.StatusCode != http.StatusOK {
		t.Fatalf("reviewer notification count status expected 200 got %d", countResp.StatusCode)
	}
	if int(countBody["count"].(float64)) < 1 {
		t.Fatalf("reviewer should receive review notification, got %v", countBody["count"])
	}

	completeResp, completeBody := requestJSON(t, http.MethodPatch, ts.URL+"/api/v1/tasks/"+strconv.Itoa(taskID)+"/complete", reviewerToken, nil)
	if completeResp.StatusCode != http.StatusOK {
		t.Fatalf("reviewer complete status expected 200 got %d", completeResp.StatusCode)
	}
	if completeBody["status"] != "completed" || int(completeBody["progress"].(float64)) != 100 {
		t.Fatalf("reviewer complete should finish task with progress 100, got status=%v progress=%v", completeBody["status"], completeBody["progress"])
	}
}

func TestPatchTaskStatusFlow(t *testing.T) {
	ts := setupTestRouter(t)
	defer ts.Close()

	adminToken := loginAndToken(t, ts.URL)
	codeToID := permissionCodeMap(t, ts.URL, adminToken)

	roleResp, roleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/rbac/roles", adminToken, map[string]any{
		"name":        "task-status-flow",
		"description": "task status flow",
		"permissionIds": []uint{
			codeToID["projects.read"],
			codeToID["tasks.read"],
		},
	})
	if roleResp.StatusCode != http.StatusCreated {
		t.Fatalf("create status role expected 201 got %d", roleResp.StatusCode)
	}
	roleID := uint(roleBody["id"].(float64))

	assigneeResp, assigneeBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":      "status_flow_assignee",
		"name":          "Status Flow Assignee",
		"email":         "status_flow_assignee@example.com",
		"password":      "pass1234",
		"roleIds":       []uint{roleID},
		"departmentIds": []uint{},
	})
	if assigneeResp.StatusCode != http.StatusCreated {
		t.Fatalf("create status assignee expected 201 got %d", assigneeResp.StatusCode)
	}
	assigneeID := uint(assigneeBody["id"].(float64))

	reviewerResp, reviewerBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":      "status_flow_reviewer",
		"name":          "Status Flow Reviewer",
		"email":         "status_flow_reviewer@example.com",
		"password":      "pass1234",
		"roleIds":       []uint{roleID},
		"departmentIds": []uint{},
	})
	if reviewerResp.StatusCode != http.StatusCreated {
		t.Fatalf("create status reviewer expected 201 got %d", reviewerResp.StatusCode)
	}
	reviewerID := uint(reviewerBody["id"].(float64))

	outsiderResp, _ := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":      "status_flow_outsider",
		"name":          "Status Flow Outsider",
		"email":         "status_flow_outsider@example.com",
		"password":      "pass1234",
		"roleIds":       []uint{roleID},
		"departmentIds": []uint{},
	})
	if outsiderResp.StatusCode != http.StatusCreated {
		t.Fatalf("create status outsider expected 201 got %d", outsiderResp.StatusCode)
	}

	projectResp, projectBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/projects", adminToken, map[string]any{
		"code":        "STATUS-FLOW-P1",
		"name":        "Status Flow Project",
		"description": "status flow",
	})
	if projectResp.StatusCode != http.StatusCreated {
		t.Fatalf("create status project expected 201 got %d", projectResp.StatusCode)
	}
	projectID := int(projectBody["id"].(float64))

	taskResp, taskBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/tasks", adminToken, map[string]any{
		"title":       "Status Flow Task",
		"projectId":   projectID,
		"status":      "pending",
		"progress":    20,
		"assigneeIds": []uint{assigneeID},
		"reviewerIds": []uint{reviewerID},
	})
	if taskResp.StatusCode != http.StatusCreated {
		t.Fatalf("create status task expected 201 got %d", taskResp.StatusCode)
	}
	taskID := int(taskBody["id"].(float64))

	assigneeLoginStatus, assigneeLoginBody := loginWithCredentials(t, ts.URL, "status_flow_assignee", "pass1234")
	if assigneeLoginStatus != http.StatusOK {
		t.Fatalf("login status assignee expected 200 got %d", assigneeLoginStatus)
	}
	assigneeToken := assigneeLoginBody["token"].(string)
	reviewerLoginStatus, reviewerLoginBody := loginWithCredentials(t, ts.URL, "status_flow_reviewer", "pass1234")
	if reviewerLoginStatus != http.StatusOK {
		t.Fatalf("login status reviewer expected 200 got %d", reviewerLoginStatus)
	}
	reviewerToken := reviewerLoginBody["token"].(string)
	outsiderLoginStatus, outsiderLoginBody := loginWithCredentials(t, ts.URL, "status_flow_outsider", "pass1234")
	if outsiderLoginStatus != http.StatusOK {
		t.Fatalf("login status outsider expected 200 got %d", outsiderLoginStatus)
	}
	outsiderToken := outsiderLoginBody["token"].(string)

	invalidResp, invalidBody := requestJSON(t, http.MethodPatch, ts.URL+"/api/v1/tasks/"+strconv.Itoa(taskID)+"/status", assigneeToken, map[string]any{
		"status": "bogus",
	})
	if invalidResp.StatusCode != http.StatusBadRequest || invalidBody["code"] != "INVALID_TASK_STATUS" {
		t.Fatalf("invalid status expected 400 INVALID_TASK_STATUS got %d %#v", invalidResp.StatusCode, invalidBody)
	}

	processingResp, processingBody := requestJSON(t, http.MethodPatch, ts.URL+"/api/v1/tasks/"+strconv.Itoa(taskID)+"/status", assigneeToken, map[string]any{
		"status": "processing",
	})
	if processingResp.StatusCode != http.StatusOK {
		t.Fatalf("assignee status processing expected 200 got %d", processingResp.StatusCode)
	}
	if processingBody["status"] != "processing" || int(processingBody["progress"].(float64)) != 20 {
		t.Fatalf("processing should keep progress 20 got status=%v progress=%v", processingBody["status"], processingBody["progress"])
	}

	completeByAssigneeResp, completeByAssigneeBody := requestJSON(t, http.MethodPatch, ts.URL+"/api/v1/tasks/"+strconv.Itoa(taskID)+"/status", assigneeToken, map[string]any{
		"status": "completed",
	})
	if completeByAssigneeResp.StatusCode != http.StatusForbidden || completeByAssigneeBody["code"] != "TASK_REVIEWER_REQUIRED" {
		t.Fatalf("assignee complete by status expected 403 TASK_REVIEWER_REQUIRED got %d %#v", completeByAssigneeResp.StatusCode, completeByAssigneeBody)
	}

	invisibleResp, _ := requestJSON(t, http.MethodPatch, ts.URL+"/api/v1/tasks/"+strconv.Itoa(taskID)+"/status", outsiderToken, map[string]any{
		"status": "processing",
	})
	if invisibleResp.StatusCode != http.StatusNotFound {
		t.Fatalf("outsider status update expected 404 got %d", invisibleResp.StatusCode)
	}

	completeResp, completeBody := requestJSON(t, http.MethodPatch, ts.URL+"/api/v1/tasks/"+strconv.Itoa(taskID)+"/status", reviewerToken, map[string]any{
		"status": "completed",
	})
	if completeResp.StatusCode != http.StatusOK {
		t.Fatalf("reviewer complete by status expected 200 got %d", completeResp.StatusCode)
	}
	if completeBody["status"] != "completed" || int(completeBody["progress"].(float64)) != 100 {
		t.Fatalf("reviewer status complete should finish task got status=%v progress=%v", completeBody["status"], completeBody["progress"])
	}
}

func TestWorkRequestSubmitReviewAndConvertToTask(t *testing.T) {
	ts := setupTestRouter(t)
	defer ts.Close()

	adminToken := loginAndToken(t, ts.URL)
	codeToID := permissionCodeMap(t, ts.URL, adminToken)

	requesterRoleResp, requesterRoleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/rbac/roles", adminToken, map[string]any{
		"name":        "request-submitter",
		"description": "request submitter",
		"permissionIds": []uint{
			codeToID["requests.create"],
			codeToID["requests.read"],
			codeToID["notifications.read"],
		},
	})
	if requesterRoleResp.StatusCode != http.StatusCreated {
		t.Fatalf("create requester role expected 201 got %d", requesterRoleResp.StatusCode)
	}
	requesterRoleID := uint(requesterRoleBody["id"].(float64))

	managerRoleResp, managerRoleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/rbac/roles", adminToken, map[string]any{
		"name":        "request-manager",
		"description": "request manager",
		"permissionIds": []uint{
			codeToID["requests.create"],
			codeToID["requests.read"],
			codeToID["requests.update"],
			codeToID["projects.read"],
			codeToID["tasks.read"],
		},
	})
	if managerRoleResp.StatusCode != http.StatusCreated {
		t.Fatalf("create manager role expected 201 got %d", managerRoleResp.StatusCode)
	}
	managerRoleID := uint(managerRoleBody["id"].(float64))

	requesterResp, requesterBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":      "request_submitter_u1",
		"name":          "Request Submitter",
		"email":         "request_submitter_u1@example.com",
		"password":      "pass1234",
		"roleIds":       []uint{requesterRoleID},
		"departmentIds": []uint{},
	})
	if requesterResp.StatusCode != http.StatusCreated {
		t.Fatalf("create requester expected 201 got %d", requesterResp.StatusCode)
	}
	requesterID := uint(requesterBody["id"].(float64))

	otherResp, _ := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":      "request_submitter_other",
		"name":          "Other Submitter",
		"email":         "request_submitter_other@example.com",
		"password":      "pass1234",
		"roleIds":       []uint{requesterRoleID},
		"departmentIds": []uint{},
	})
	if otherResp.StatusCode != http.StatusCreated {
		t.Fatalf("create other requester expected 201 got %d", otherResp.StatusCode)
	}

	managerResp, managerBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":      "request_manager_u1",
		"name":          "Request Manager",
		"email":         "request_manager_u1@example.com",
		"password":      "pass1234",
		"roleIds":       []uint{managerRoleID},
		"departmentIds": []uint{},
	})
	if managerResp.StatusCode != http.StatusCreated {
		t.Fatalf("create manager expected 201 got %d", managerResp.StatusCode)
	}
	managerID := uint(managerBody["id"].(float64))

	projectResp, projectBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/projects", adminToken, map[string]any{
		"code":        "REQ-P1",
		"name":        "Request Project",
		"description": "request project",
		"userIds":     []uint{managerID},
	})
	if projectResp.StatusCode != http.StatusCreated {
		t.Fatalf("create request project expected 201 got %d", projectResp.StatusCode)
	}
	projectID := int(projectBody["id"].(float64))

	requesterLoginStatus, requesterLoginBody := loginWithCredentials(t, ts.URL, "request_submitter_u1", "pass1234")
	if requesterLoginStatus != http.StatusOK {
		t.Fatalf("login requester expected 200 got %d", requesterLoginStatus)
	}
	requesterToken := requesterLoginBody["token"].(string)
	otherLoginStatus, otherLoginBody := loginWithCredentials(t, ts.URL, "request_submitter_other", "pass1234")
	if otherLoginStatus != http.StatusOK {
		t.Fatalf("login other expected 200 got %d", otherLoginStatus)
	}
	otherToken := otherLoginBody["token"].(string)
	managerLoginStatus, managerLoginBody := loginWithCredentials(t, ts.URL, "request_manager_u1", "pass1234")
	if managerLoginStatus != http.StatusOK {
		t.Fatalf("login manager expected 200 got %d", managerLoginStatus)
	}
	managerToken := managerLoginBody["token"].(string)

	createResp, createBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/requests", requesterToken, map[string]any{
		"type":        "task",
		"title":       "Need onboarding task",
		"description": "please create an onboarding task",
		"priority":    "high",
	})
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("create work request expected 201 got %d %#v", createResp.StatusCode, createBody)
	}
	requestID := int(createBody["id"].(float64))
	if int(createBody["requesterId"].(float64)) != int(requesterID) || createBody["status"] != "submitted" {
		t.Fatalf("unexpected created request body %#v", createBody)
	}

	otherListResp, otherListBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/requests", otherToken, nil)
	if otherListResp.StatusCode != http.StatusOK {
		t.Fatalf("other list requests expected 200 got %d", otherListResp.StatusCode)
	}
	if list, ok := otherListBody["list"].([]any); !ok || len(list) != 0 {
		t.Fatalf("other requester should not see unrelated requests got %#v", otherListBody["list"])
	}

	forbiddenReviewResp, _ := requestJSON(t, http.MethodPatch, ts.URL+"/api/v1/requests/"+strconv.Itoa(requestID)+"/review", requesterToken, map[string]any{
		"status": "approved",
	})
	if forbiddenReviewResp.StatusCode != http.StatusForbidden {
		t.Fatalf("requester review expected 403 got %d", forbiddenReviewResp.StatusCode)
	}

	approveResp, approveBody := requestJSON(t, http.MethodPatch, ts.URL+"/api/v1/requests/"+strconv.Itoa(requestID)+"/review", managerToken, map[string]any{
		"status": "approved",
		"note":   "可以转任务",
	})
	if approveResp.StatusCode != http.StatusOK {
		t.Fatalf("manager approve expected 200 got %d %#v", approveResp.StatusCode, approveBody)
	}
	if approveBody["status"] != "approved" || int(approveBody["reviewerId"].(float64)) != int(managerID) {
		t.Fatalf("unexpected approve body %#v", approveBody)
	}

	convertResp, convertBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/requests/"+strconv.Itoa(requestID)+"/convert-task", managerToken, map[string]any{
		"projectId":   projectID,
		"assigneeIds": []uint{requesterID},
		"reviewerIds": []uint{managerID},
	})
	if convertResp.StatusCode != http.StatusCreated {
		t.Fatalf("convert request expected 201 got %d %#v", convertResp.StatusCode, convertBody)
	}
	task, ok := convertBody["task"].(map[string]any)
	if !ok {
		t.Fatalf("convert response should include task got %#v", convertBody)
	}
	if task["title"] != "Need onboarding task" || int(task["projectId"].(float64)) != projectID {
		t.Fatalf("converted task should copy title and project got %#v", task)
	}

	listResp, listBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/requests", managerToken, nil)
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("manager list requests expected 200 got %d", listResp.StatusCode)
	}
	items, _ := listBody["list"].([]any)
	if len(items) == 0 {
		t.Fatalf("manager should see converted request got %#v", listBody)
	}
	first, _ := items[0].(map[string]any)
	if first["status"] != "converted" || first["convertedTaskId"] == nil {
		t.Fatalf("request should be converted with task id got %#v", first)
	}

	rejectedResp, rejectedBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/requests", requesterToken, map[string]any{
		"type":  "bug",
		"title": "Rejected bug",
	})
	if rejectedResp.StatusCode != http.StatusCreated {
		t.Fatalf("create rejected request expected 201 got %d", rejectedResp.StatusCode)
	}
	rejectedID := int(rejectedBody["id"].(float64))
	reviewRejectResp, _ := requestJSON(t, http.MethodPatch, ts.URL+"/api/v1/requests/"+strconv.Itoa(rejectedID)+"/review", managerToken, map[string]any{
		"status": "rejected",
	})
	if reviewRejectResp.StatusCode != http.StatusOK {
		t.Fatalf("reject request expected 200 got %d", reviewRejectResp.StatusCode)
	}
	convertRejectedResp, _ := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/requests/"+strconv.Itoa(rejectedID)+"/convert-task", managerToken, map[string]any{
		"projectId": projectID,
	})
	if convertRejectedResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("convert rejected request expected 400 got %d", convertRejectedResp.StatusCode)
	}
}

func TestWorkRequestChangeControlApplyToTask(t *testing.T) {
	ts := setupTestRouter(t)
	defer ts.Close()

	adminToken := loginAndToken(t, ts.URL)
	codeToID := permissionCodeMap(t, ts.URL, adminToken)

	requesterRoleResp, requesterRoleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/rbac/roles", adminToken, map[string]any{
		"name":        "change-requester",
		"description": "change requester",
		"permissionIds": []uint{
			codeToID["requests.create"],
			codeToID["requests.read"],
			codeToID["notifications.read"],
		},
	})
	if requesterRoleResp.StatusCode != http.StatusCreated {
		t.Fatalf("create change requester role expected 201 got %d", requesterRoleResp.StatusCode)
	}
	requesterRoleID := uint(requesterRoleBody["id"].(float64))

	managerRoleResp, managerRoleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/rbac/roles", adminToken, map[string]any{
		"name":        "change-manager",
		"description": "change manager",
		"permissionIds": []uint{
			codeToID["requests.read"],
			codeToID["requests.update"],
			codeToID["projects.read"],
			codeToID["tasks.read"],
			codeToID["comments.read"],
		},
	})
	if managerRoleResp.StatusCode != http.StatusCreated {
		t.Fatalf("create change manager role expected 201 got %d", managerRoleResp.StatusCode)
	}
	managerRoleID := uint(managerRoleBody["id"].(float64))

	requesterResp, requesterBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":      "change_requester",
		"name":          "Change Requester",
		"email":         "change_requester@example.com",
		"password":      "pass1234",
		"roleIds":       []uint{requesterRoleID},
		"departmentIds": []uint{},
	})
	if requesterResp.StatusCode != http.StatusCreated {
		t.Fatalf("create change requester expected 201 got %d", requesterResp.StatusCode)
	}
	requesterID := uint(requesterBody["id"].(float64))

	managerResp, managerBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":      "change_manager",
		"name":          "Change Manager",
		"email":         "change_manager@example.com",
		"password":      "pass1234",
		"roleIds":       []uint{managerRoleID},
		"departmentIds": []uint{},
	})
	if managerResp.StatusCode != http.StatusCreated {
		t.Fatalf("create change manager expected 201 got %d", managerResp.StatusCode)
	}
	managerID := uint(managerBody["id"].(float64))

	newAssigneeResp, newAssigneeBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":      "change_new_assignee",
		"name":          "Change New Assignee",
		"email":         "change_new_assignee@example.com",
		"password":      "pass1234",
		"roleIds":       []uint{},
		"departmentIds": []uint{},
	})
	if newAssigneeResp.StatusCode != http.StatusCreated {
		t.Fatalf("create change new assignee expected 201 got %d", newAssigneeResp.StatusCode)
	}
	newAssigneeID := uint(newAssigneeBody["id"].(float64))

	projectResp, projectBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/projects", adminToken, map[string]any{
		"code":        "CHANGE-P1",
		"name":        "Change Project",
		"description": "change project",
		"userIds":     []uint{managerID},
	})
	if projectResp.StatusCode != http.StatusCreated {
		t.Fatalf("create change project expected 201 got %d", projectResp.StatusCode)
	}
	projectID := int(projectBody["id"].(float64))

	taskResp, taskBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/tasks", adminToken, map[string]any{
		"title":       "Change Controlled Task",
		"projectId":   projectID,
		"status":      "processing",
		"priority":    "high",
		"progress":    30,
		"startAt":     "2026-01-01T00:00:00Z",
		"endAt":       "2026-01-03T00:00:00Z",
		"assigneeIds": []uint{requesterID},
	})
	if taskResp.StatusCode != http.StatusCreated {
		t.Fatalf("create change target task expected 201 got %d, body=%v", taskResp.StatusCode, taskBody)
	}
	taskID := int(taskBody["id"].(float64))

	requesterLoginStatus, requesterLoginBody := loginWithCredentials(t, ts.URL, "change_requester", "pass1234")
	if requesterLoginStatus != http.StatusOK {
		t.Fatalf("login change requester expected 200 got %d", requesterLoginStatus)
	}
	requesterToken := requesterLoginBody["token"].(string)
	managerLoginStatus, managerLoginBody := loginWithCredentials(t, ts.URL, "change_manager", "pass1234")
	if managerLoginStatus != http.StatusOK {
		t.Fatalf("login change manager expected 200 got %d", managerLoginStatus)
	}
	managerToken := managerLoginBody["token"].(string)

	missingTargetResp, missingTargetBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/requests", requesterToken, map[string]any{
		"type":        "change",
		"title":       "缺少目标任务",
		"description": "should fail",
	})
	if missingTargetResp.StatusCode != http.StatusBadRequest || missingTargetBody["code"] != "CHANGE_TARGET_TASK_REQUIRED" {
		t.Fatalf("change request without target expected 400 CHANGE_TARGET_TASK_REQUIRED got %d %#v", missingTargetResp.StatusCode, missingTargetBody)
	}

	changeResp, changeBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/requests", requesterToken, map[string]any{
		"type":         "change",
		"title":        "调整关键任务排期和负责人",
		"description":  "需要重新排期",
		"priority":     "high",
		"targetTaskId": taskID,
		"changePayload": map[string]any{
			"startAt":          "2026-01-04T00:00:00Z",
			"endAt":            "2026-01-06T00:00:00Z",
			"priority":         "low",
			"assigneeIds":      []uint{newAssigneeID},
			"scopeDescription": "范围收敛到交付验收",
		},
	})
	if changeResp.StatusCode != http.StatusCreated {
		t.Fatalf("create change request expected 201 got %d %#v", changeResp.StatusCode, changeBody)
	}
	changeRequestID := int(changeBody["id"].(float64))
	if changeBody["status"] != "submitted" || int(changeBody["targetTaskId"].(float64)) != taskID || int(changeBody["projectId"].(float64)) != projectID {
		t.Fatalf("unexpected change request body %#v", changeBody)
	}

	convertResp, convertBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/requests/"+strconv.Itoa(changeRequestID)+"/convert-task", managerToken, map[string]any{
		"projectId": projectID,
	})
	if convertResp.StatusCode != http.StatusBadRequest || convertBody["code"] != "CHANGE_REQUEST_NOT_CONVERTIBLE" {
		t.Fatalf("change request convert expected 400 CHANGE_REQUEST_NOT_CONVERTIBLE got %d %#v", convertResp.StatusCode, convertBody)
	}

	earlyApplyResp, earlyApplyBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/requests/"+strconv.Itoa(changeRequestID)+"/apply-change", managerToken, nil)
	if earlyApplyResp.StatusCode != http.StatusBadRequest || earlyApplyBody["code"] != "WORK_REQUEST_NOT_APPROVED" {
		t.Fatalf("change apply before approval expected 400 WORK_REQUEST_NOT_APPROVED got %d %#v", earlyApplyResp.StatusCode, earlyApplyBody)
	}

	approveResp, approveBody := requestJSON(t, http.MethodPatch, ts.URL+"/api/v1/requests/"+strconv.Itoa(changeRequestID)+"/review", managerToken, map[string]any{
		"status": "approved",
		"note":   "批准调整",
	})
	if approveResp.StatusCode != http.StatusOK || approveBody["status"] != "approved" {
		t.Fatalf("approve change request expected 200 approved got %d %#v", approveResp.StatusCode, approveBody)
	}

	applyResp, applyBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/requests/"+strconv.Itoa(changeRequestID)+"/apply-change", managerToken, nil)
	if applyResp.StatusCode != http.StatusOK {
		t.Fatalf("apply change request expected 200 got %d %#v", applyResp.StatusCode, applyBody)
	}
	appliedRequest, _ := applyBody["request"].(map[string]any)
	updatedTask, _ := applyBody["task"].(map[string]any)
	if appliedRequest["status"] != "applied" || appliedRequest["appliedAt"] == nil || int(appliedRequest["appliedById"].(float64)) != int(managerID) {
		t.Fatalf("applied request should record applied status/person got %#v", appliedRequest)
	}
	if updatedTask["priority"] != "low" || updatedTask["startAt"] == nil || updatedTask["endAt"] == nil {
		t.Fatalf("updated task should include changed schedule and priority got %#v", updatedTask)
	}
	assignees, _ := updatedTask["assignees"].([]any)
	if len(assignees) != 1 || int(assignees[0].(map[string]any)["id"].(float64)) != int(newAssigneeID) {
		t.Fatalf("updated task should replace assignees got %#v", updatedTask["assignees"])
	}

	activityResp, activityBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/tasks/"+strconv.Itoa(taskID)+"/activities", adminToken, nil)
	if activityResp.StatusCode != http.StatusOK {
		t.Fatalf("change task activities expected 200 got %d", activityResp.StatusCode)
	}
	activities, _ := activityBody["list"].([]any)
	foundChangeActivity := false
	for _, raw := range activities {
		item, _ := raw.(map[string]any)
		detail, _ := item["detail"].(string)
		if item["type"] == "change_request.applied" && strings.Contains(detail, "范围收敛") {
			foundChangeActivity = true
			break
		}
	}
	if !foundChangeActivity {
		t.Fatalf("expected change_request.applied activity got %#v", activityBody)
	}

	reapplyResp, reapplyBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/requests/"+strconv.Itoa(changeRequestID)+"/apply-change", managerToken, nil)
	if reapplyResp.StatusCode != http.StatusBadRequest || reapplyBody["code"] != "WORK_REQUEST_ALREADY_APPLIED" {
		t.Fatalf("reapply change expected 400 WORK_REQUEST_ALREADY_APPLIED got %d %#v", reapplyResp.StatusCode, reapplyBody)
	}
}

func TestProjectTemplateCreateAndGenerateProjectTree(t *testing.T) {
	ts := setupTestRouter(t)
	defer ts.Close()

	adminToken := loginAndToken(t, ts.URL)
	codeToID := permissionCodeMap(t, ts.URL, adminToken)

	creatorOnlyRoleResp, creatorOnlyRoleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/rbac/roles", adminToken, map[string]any{
		"name":        "template-project-creator-only",
		"description": "can create projects but cannot read templates",
		"permissionIds": []uint{
			codeToID["projects.create"],
		},
	})
	if creatorOnlyRoleResp.StatusCode != http.StatusCreated {
		t.Fatalf("create creator-only role expected 201 got %d", creatorOnlyRoleResp.StatusCode)
	}
	creatorOnlyRoleID := uint(creatorOnlyRoleBody["id"].(float64))

	creatorResp, _ := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":      "template_creator_only",
		"name":          "Template Creator Only",
		"email":         "template_creator_only@example.com",
		"password":      "pass1234",
		"roleIds":       []uint{creatorOnlyRoleID},
		"departmentIds": []uint{},
	})
	if creatorResp.StatusCode != http.StatusCreated {
		t.Fatalf("create creator-only user expected 201 got %d", creatorResp.StatusCode)
	}
	creatorLoginStatus, creatorLoginBody := loginWithCredentials(t, ts.URL, "template_creator_only", "pass1234")
	if creatorLoginStatus != http.StatusOK {
		t.Fatalf("login creator-only expected 200 got %d", creatorLoginStatus)
	}
	creatorOnlyToken := creatorLoginBody["token"].(string)

	templatePayload := map[string]any{
		"name":        "上线项目模板",
		"description": "标准上线项目任务树",
		"taskTree": []map[string]any{
			{
				"key":              "plan",
				"title":            "制定上线计划",
				"description":      "确认范围和节奏",
				"priority":         "high",
				"isMilestone":      true,
				"relativeStartDay": 0,
				"durationDays":     1,
				"children": []map[string]any{
					{
						"key":              "design",
						"title":            "设计发布方案",
						"description":      "设计灰度和回滚方案",
						"priority":         "medium",
						"relativeStartDay": 1,
						"durationDays":     3,
						"dependencies": []map[string]any{
							{"dependsOnKey": "plan", "lagDays": 1, "type": "FS"},
						},
					},
				},
			},
			{
				"key":              "release",
				"title":            "执行发布",
				"priority":         "high",
				"isMilestone":      true,
				"relativeStartDay": 5,
				"durationDays":     1,
				"dependencies": []map[string]any{
					{"dependsOnKey": "design", "type": "FS"},
				},
			},
		},
	}

	createResp, createBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/project-templates", adminToken, templatePayload)
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("create project template expected 201 got %d %#v", createResp.StatusCode, createBody)
	}
	templateID := int(createBody["id"].(float64))
	if createBody["name"] != "上线项目模板" {
		t.Fatalf("unexpected template body %#v", createBody)
	}

	listResp, listBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/project-templates?keyword=上线", adminToken, nil)
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("list templates expected 200 got %d", listResp.StatusCode)
	}
	if list, ok := listBody["list"].([]any); !ok || len(list) != 1 {
		t.Fatalf("template list should include created item got %#v", listBody)
	}

	updatedPayload := map[string]any{
		"name":        "上线项目模板 v2",
		"description": "更新后的标准上线项目任务树",
		"taskTree":    templatePayload["taskTree"],
	}
	updateResp, updateBody := requestJSON(t, http.MethodPut, ts.URL+"/api/v1/project-templates/"+strconv.Itoa(templateID), adminToken, updatedPayload)
	if updateResp.StatusCode != http.StatusOK {
		t.Fatalf("update project template expected 200 got %d %#v", updateResp.StatusCode, updateBody)
	}
	if updateBody["name"] != "上线项目模板 v2" {
		t.Fatalf("template name should update got %#v", updateBody)
	}

	forbiddenGenerateResp, forbiddenGenerateBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/project-templates/"+strconv.Itoa(templateID)+"/create-project", creatorOnlyToken, map[string]any{
		"code":    "TPL-FORBID",
		"name":    "Forbidden Template Project",
		"startAt": "2026-07-01T00:00:00Z",
	})
	if forbiddenGenerateResp.StatusCode != http.StatusForbidden || forbiddenGenerateBody["code"] != "PROJECT_TEMPLATE_READ_REQUIRED" {
		t.Fatalf("create project without template read expected 403 PROJECT_TEMPLATE_READ_REQUIRED got %d %#v", forbiddenGenerateResp.StatusCode, forbiddenGenerateBody)
	}

	generateResp, generateBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/project-templates/"+strconv.Itoa(templateID)+"/create-project", adminToken, map[string]any{
		"code":    "TPL-PROJ-1",
		"name":    "模板生成项目",
		"startAt": "2026-07-01T00:00:00Z",
	})
	if generateResp.StatusCode != http.StatusCreated {
		t.Fatalf("create project from template expected 201 got %d %#v", generateResp.StatusCode, generateBody)
	}
	project, ok := generateBody["project"].(map[string]any)
	if !ok {
		t.Fatalf("generate response should include project got %#v", generateBody)
	}
	projectID := int(project["id"].(float64))
	if project["code"] != "TPL-PROJ-1" || project["name"] != "模板生成项目" {
		t.Fatalf("generated project should keep requested code/name got %#v", project)
	}
	tasks, ok := generateBody["tasks"].([]any)
	if !ok || len(tasks) != 3 {
		t.Fatalf("generated response should include 3 tasks got %#v", generateBody["tasks"])
	}

	taskByTitle := map[string]map[string]any{}
	for _, rawTask := range tasks {
		task, castOK := rawTask.(map[string]any)
		if !castOK {
			t.Fatalf("unexpected task item %#v", rawTask)
		}
		taskByTitle[task["title"].(string)] = task
		if int(task["projectId"].(float64)) != projectID {
			t.Fatalf("generated task should belong to project %d got %#v", projectID, task)
		}
	}
	plan := taskByTitle["制定上线计划"]
	design := taskByTitle["设计发布方案"]
	release := taskByTitle["执行发布"]
	if plan == nil || design == nil || release == nil {
		t.Fatalf("generated tasks missing expected titles: %#v", taskByTitle)
	}
	if design["parentId"] == nil || int(design["parentId"].(float64)) != int(plan["id"].(float64)) {
		t.Fatalf("child task should point to plan parent got plan=%#v design=%#v", plan, design)
	}
	if plan["startAt"] != "2026-07-01T00:00:00Z" || design["startAt"] != "2026-07-02T00:00:00Z" || release["startAt"] != "2026-07-06T00:00:00Z" {
		t.Fatalf("relative dates unexpected plan=%v design=%v release=%v", plan["startAt"], design["startAt"], release["startAt"])
	}
	if design["endAt"] != "2026-07-04T00:00:00Z" {
		t.Fatalf("duration should set design endAt to 2026-07-04 got %v", design["endAt"])
	}

	rawTreeReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/projects/"+strconv.Itoa(projectID)+"/task-tree", nil)
	rawTreeReq.Header.Set("Authorization", "Bearer "+adminToken)
	rawTreeResp, err := http.DefaultClient.Do(rawTreeReq)
	if err != nil {
		t.Fatalf("request task tree failed: %v", err)
	}
	defer rawTreeResp.Body.Close()
	var rawTree []map[string]any
	if err := json.NewDecoder(rawTreeResp.Body).Decode(&rawTree); err != nil {
		t.Fatalf("decode task tree failed: %v", err)
	}
	if len(rawTree) != 2 {
		t.Fatalf("task tree should have 2 roots got %#v", rawTree)
	}
	var planRoot map[string]any
	for _, root := range rawTree {
		if root["title"] == "制定上线计划" {
			planRoot = root
		}
	}
	children, _ := planRoot["children"].([]any)
	if len(children) != 1 {
		t.Fatalf("plan root should include one child got %#v", planRoot)
	}

	ganttReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/projects/"+strconv.Itoa(projectID)+"/gantt", nil)
	ganttReq.Header.Set("Authorization", "Bearer "+adminToken)
	ganttResp, err := http.DefaultClient.Do(ganttReq)
	if err != nil {
		t.Fatalf("request gantt failed: %v", err)
	}
	defer ganttResp.Body.Close()
	if ganttResp.StatusCode != http.StatusOK {
		t.Fatalf("gantt expected 200 got %d", ganttResp.StatusCode)
	}
	var ganttItems []map[string]any
	if err := json.NewDecoder(ganttResp.Body).Decode(&ganttItems); err != nil {
		t.Fatalf("decode gantt failed: %v", err)
	}
	dependencyCount := 0
	for _, item := range ganttItems {
		if dependencies, ok := item["dependencies"].([]any); ok {
			dependencyCount += len(dependencies)
		}
	}
	if dependencyCount != 2 {
		t.Fatalf("generated template dependencies expected 2 got %d items=%#v", dependencyCount, ganttItems)
	}
}

func TestSavedReportsCRUDAndOwnerScope(t *testing.T) {
	ts := setupTestRouter(t)
	defer ts.Close()

	adminToken := loginAndToken(t, ts.URL)
	codeToID := permissionCodeMap(t, ts.URL, adminToken)

	roleResp, roleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/rbac/roles", adminToken, map[string]any{
		"name":        "report-manager",
		"description": "report manager",
		"permissionIds": []uint{
			codeToID["reports.create"],
			codeToID["reports.read"],
			codeToID["reports.update"],
			codeToID["reports.delete"],
		},
	})
	if roleResp.StatusCode != http.StatusCreated {
		t.Fatalf("create report role expected 201 got %d", roleResp.StatusCode)
	}
	roleID := uint(roleBody["id"].(float64))

	userAResp, _ := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":      "report_owner",
		"name":          "Report Owner",
		"email":         "report_owner@example.com",
		"password":      "pass1234",
		"roleIds":       []uint{roleID},
		"departmentIds": []uint{},
	})
	if userAResp.StatusCode != http.StatusCreated {
		t.Fatalf("create report owner expected 201 got %d", userAResp.StatusCode)
	}
	userBResp, _ := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":      "report_other",
		"name":          "Report Other",
		"email":         "report_other@example.com",
		"password":      "pass1234",
		"roleIds":       []uint{roleID},
		"departmentIds": []uint{},
	})
	if userBResp.StatusCode != http.StatusCreated {
		t.Fatalf("create report other expected 201 got %d", userBResp.StatusCode)
	}

	loginStatus, loginBody := loginWithCredentials(t, ts.URL, "report_owner", "pass1234")
	if loginStatus != http.StatusOK {
		t.Fatalf("login report owner expected 200 got %d", loginStatus)
	}
	ownerToken := loginBody["token"].(string)
	otherLoginStatus, otherLoginBody := loginWithCredentials(t, ts.URL, "report_other", "pass1234")
	if otherLoginStatus != http.StatusOK {
		t.Fatalf("login report other expected 200 got %d", otherLoginStatus)
	}
	otherToken := otherLoginBody["token"].(string)

	createResp, createBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/reports", ownerToken, map[string]any{
		"name":        "本部门风险",
		"description": "关注项目健康红色项",
		"type":        "project_health",
		"filters": map[string]any{
			"keyword":  "风险",
			"statuses": []string{"processing", "completed", "bad-status"},
		},
		"chartConfig": map[string]any{"displayMode": "summary"},
	})
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("create saved report expected 201 got %d, body=%v", createResp.StatusCode, createBody)
	}
	reportID := int(createBody["id"].(float64))
	filters, _ := createBody["filters"].(map[string]any)
	statuses, _ := filters["statuses"].([]any)
	if len(statuses) != 2 {
		t.Fatalf("saved report should normalize valid statuses got %v", filters)
	}

	invalidResp, invalidBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/reports", ownerToken, map[string]any{
		"name": "坏报表",
		"type": "unknown",
	})
	if invalidResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid report type expected 400 got %d", invalidResp.StatusCode)
	}
	if invalidBody["code"] != "INVALID_REPORT_TYPE" {
		t.Fatalf("invalid report type expected INVALID_REPORT_TYPE got %v", invalidBody["code"])
	}

	ownerListResp, ownerListBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/reports?page=1&pageSize=20", ownerToken, nil)
	if ownerListResp.StatusCode != http.StatusOK {
		t.Fatalf("owner report list expected 200 got %d", ownerListResp.StatusCode)
	}
	ownerReports, _ := ownerListBody["list"].([]any)
	if len(ownerReports) != 1 {
		t.Fatalf("owner should see one report got %v", ownerListBody)
	}

	otherListResp, otherListBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/reports?page=1&pageSize=20", otherToken, nil)
	if otherListResp.StatusCode != http.StatusOK {
		t.Fatalf("other report list expected 200 got %d", otherListResp.StatusCode)
	}
	otherReports, _ := otherListBody["list"].([]any)
	if len(otherReports) != 0 {
		t.Fatalf("other user should not see owner report got %v", otherListBody)
	}

	otherDetailResp, _ := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/reports/"+strconv.Itoa(reportID), otherToken, nil)
	if otherDetailResp.StatusCode != http.StatusNotFound {
		t.Fatalf("other user report detail expected 404 got %d", otherDetailResp.StatusCode)
	}

	updateResp, updateBody := requestJSON(t, http.MethodPut, ts.URL+"/api/v1/reports/"+strconv.Itoa(reportID), ownerToken, map[string]any{
		"name":        "成员负载周报",
		"description": "关注过载成员",
		"type":        "member_workload",
		"filters":     map[string]any{"keyword": "成员"},
		"chartConfig": map[string]any{"displayMode": "table"},
	})
	if updateResp.StatusCode != http.StatusOK {
		t.Fatalf("update saved report expected 200 got %d, body=%v", updateResp.StatusCode, updateBody)
	}
	if updateBody["type"] != "member_workload" {
		t.Fatalf("saved report type not updated: %v", updateBody["type"])
	}

	deleteResp, _ := requestJSON(t, http.MethodDelete, ts.URL+"/api/v1/reports/"+strconv.Itoa(reportID), ownerToken, nil)
	if deleteResp.StatusCode != http.StatusOK {
		t.Fatalf("delete saved report expected 200 got %d", deleteResp.StatusCode)
	}
	emptyListResp, emptyListBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/reports?page=1&pageSize=20", ownerToken, nil)
	if emptyListResp.StatusCode != http.StatusOK {
		t.Fatalf("owner report list after delete expected 200 got %d", emptyListResp.StatusCode)
	}
	emptyReports, _ := emptyListBody["list"].([]any)
	if len(emptyReports) != 0 {
		t.Fatalf("owner report list should be empty after delete got %v", emptyListBody)
	}
}

func TestSavedReportRunExportAndSubscriptionScope(t *testing.T) {
	ts := setupTestRouter(t)
	defer ts.Close()

	adminToken := loginAndToken(t, ts.URL)
	codeToID := permissionCodeMap(t, ts.URL, adminToken)

	roleResp, roleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/rbac/roles", adminToken, map[string]any{
		"name":        "report-runner",
		"description": "report runner",
		"permissionIds": []uint{
			codeToID["reports.create"],
			codeToID["reports.read"],
			codeToID["reports.update"],
			codeToID["reports.delete"],
			codeToID["notifications.read"],
		},
	})
	if roleResp.StatusCode != http.StatusCreated {
		t.Fatalf("create report runner role expected 201 got %d body=%v", roleResp.StatusCode, roleBody)
	}
	roleID := uint(roleBody["id"].(float64))

	deptResp, deptBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/departments", adminToken, map[string]any{
		"name":        "报表验收部门",
		"description": "用于报表中心验收",
	})
	if deptResp.StatusCode != http.StatusCreated {
		t.Fatalf("create department expected 201 got %d body=%v", deptResp.StatusCode, deptBody)
	}
	departmentID := int(deptBody["id"].(float64))

	ownerResp, ownerBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":      "report_runner",
		"name":          "Report Runner",
		"email":         "report_runner@example.com",
		"password":      "pass1234",
		"roleIds":       []uint{roleID},
		"departmentIds": []uint{uint(departmentID)},
	})
	if ownerResp.StatusCode != http.StatusCreated {
		t.Fatalf("create report runner expected 201 got %d body=%v", ownerResp.StatusCode, ownerBody)
	}
	ownerID := int(ownerBody["id"].(float64))

	hiddenResp, hiddenBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":      "report_hidden_assignee",
		"name":          "Report Hidden",
		"email":         "report_hidden_assignee@example.com",
		"password":      "pass1234",
		"roleIds":       []uint{},
		"departmentIds": []uint{uint(departmentID)},
	})
	if hiddenResp.StatusCode != http.StatusCreated {
		t.Fatalf("create hidden assignee expected 201 got %d body=%v", hiddenResp.StatusCode, hiddenBody)
	}
	hiddenUserID := int(hiddenBody["id"].(float64))

	loginStatus, loginBody := loginWithCredentials(t, ts.URL, "report_runner", "pass1234")
	if loginStatus != http.StatusOK {
		t.Fatalf("login report runner expected 200 got %d body=%v", loginStatus, loginBody)
	}
	ownerToken := loginBody["token"].(string)

	visibleProjectResp, visibleProjectBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/projects", adminToken, map[string]any{
		"code":          "REPORT-VISIBLE",
		"name":          "Visible Risk Project",
		"description":   "visible to report runner",
		"userIds":       []uint{uint(ownerID)},
		"departmentIds": []uint{uint(departmentID)},
	})
	if visibleProjectResp.StatusCode != http.StatusCreated {
		t.Fatalf("create visible project expected 201 got %d body=%v", visibleProjectResp.StatusCode, visibleProjectBody)
	}
	visibleProjectID := int(visibleProjectBody["id"].(float64))

	hiddenProjectResp, hiddenProjectBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/projects", adminToken, map[string]any{
		"code":          "REPORT-HIDDEN",
		"name":          "Hidden Risk Project",
		"description":   "hidden from report runner",
		"departmentIds": []uint{uint(departmentID)},
	})
	if hiddenProjectResp.StatusCode != http.StatusCreated {
		t.Fatalf("create hidden project expected 201 got %d body=%v", hiddenProjectResp.StatusCode, hiddenProjectBody)
	}
	hiddenProjectID := int(hiddenProjectBody["id"].(float64))

	now := time.Now()
	startAt := now.AddDate(0, 0, -8).Format(time.RFC3339)
	endAt := now.AddDate(0, 0, -2).Format(time.RFC3339)
	visibleTaskResp, visibleTaskBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/tasks", adminToken, map[string]any{
		"title":          "Visible Risk Task",
		"description":    "runner can see this overdue task",
		"projectId":      visibleProjectID,
		"status":         "processing",
		"priority":       "high",
		"progress":       20,
		"startAt":        startAt,
		"endAt":          endAt,
		"estimatedHours": 12,
		"assigneeIds":    []uint{uint(ownerID)},
	})
	if visibleTaskResp.StatusCode != http.StatusCreated {
		t.Fatalf("create visible task expected 201 got %d body=%v", visibleTaskResp.StatusCode, visibleTaskBody)
	}
	hiddenTaskResp, hiddenTaskBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/tasks", adminToken, map[string]any{
		"title":       "Hidden Risk Task",
		"description": "runner must not see this overdue task",
		"projectId":   hiddenProjectID,
		"status":      "processing",
		"priority":    "high",
		"progress":    10,
		"startAt":     startAt,
		"endAt":       endAt,
		"assigneeIds": []uint{uint(hiddenUserID)},
	})
	if hiddenTaskResp.StatusCode != http.StatusCreated {
		t.Fatalf("create hidden task expected 201 got %d body=%v", hiddenTaskResp.StatusCode, hiddenTaskBody)
	}

	createResp, createBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/reports", ownerToken, map[string]any{
		"name":        "本部门项目风险",
		"description": "验收部门内可见项目风险",
		"type":        "project_health",
		"filters": map[string]any{
			"departmentId": departmentID,
		},
		"chartConfig": map[string]any{"displayMode": "table"},
	})
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("create project health report expected 201 got %d body=%v", createResp.StatusCode, createBody)
	}
	reportID := int(createBody["id"].(float64))
	filters, _ := createBody["filters"].(map[string]any)
	if int(filters["departmentId"].(float64)) != departmentID {
		t.Fatalf("department filter should be persisted got %v", filters)
	}

	runResp, runBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/reports/"+strconv.Itoa(reportID)+"/run", ownerToken, nil)
	if runResp.StatusCode != http.StatusOK {
		t.Fatalf("run saved report expected 200 got %d body=%v", runResp.StatusCode, runBody)
	}
	runText := fmt.Sprint(runBody)
	if !strings.Contains(runText, "Visible Risk Project") || strings.Contains(runText, "Hidden Risk Project") || strings.Contains(runText, "Hidden Risk Task") {
		t.Fatalf("report run must include only visible scoped data got: %s", runText)
	}

	exportReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/reports/"+strconv.Itoa(reportID)+"/export.csv", nil)
	exportReq.Header.Set("Authorization", "Bearer "+ownerToken)
	exportResp, err := http.DefaultClient.Do(exportReq)
	if err != nil {
		t.Fatalf("csv export failed: %v", err)
	}
	exportRaw, _ := io.ReadAll(exportResp.Body)
	exportResp.Body.Close()
	if exportResp.StatusCode != http.StatusOK {
		t.Fatalf("csv export expected 200 got %d body=%s", exportResp.StatusCode, string(exportRaw))
	}
	exportText := string(exportRaw)
	if !strings.Contains(exportText, "Visible Risk Project") || strings.Contains(exportText, "Hidden Risk Project") {
		t.Fatalf("csv export must respect report scope got: %s", exportText)
	}

	subscribeResp, subscribeBody := requestJSON(t, http.MethodPut, ts.URL+"/api/v1/reports/"+strconv.Itoa(reportID)+"/subscription", ownerToken, map[string]any{
		"isEnabled":        true,
		"schedule":         "weekly",
		"weekday":          int(now.Weekday()),
		"hour":             now.Hour(),
		"channels":         []string{"in_app"},
		"recipientUserIds": []uint{uint(ownerID)},
	})
	if subscribeResp.StatusCode != http.StatusOK {
		t.Fatalf("upsert report subscription expected 200 got %d body=%v", subscribeResp.StatusCode, subscribeBody)
	}

	runSubResp, runSubBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/reports/"+strconv.Itoa(reportID)+"/subscription/run", ownerToken, nil)
	if runSubResp.StatusCode != http.StatusOK {
		t.Fatalf("manual report subscription run expected 200 got %d body=%v", runSubResp.StatusCode, runSubBody)
	}
	if runSubBody["lastStatus"] != "success" {
		t.Fatalf("subscription run should mark success got %v", runSubBody)
	}

	notificationResp, notificationBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/notifications?module=reports&keyword=项目周报", ownerToken, nil)
	if notificationResp.StatusCode != http.StatusOK {
		t.Fatalf("report notification query expected 200 got %d body=%v", notificationResp.StatusCode, notificationBody)
	}
	notificationText := fmt.Sprint(notificationBody)
	if !strings.Contains(notificationText, "Visible Risk Project") || strings.Contains(notificationText, "Hidden Risk Project") {
		t.Fatalf("report subscription notification must respect scope got: %s", notificationText)
	}
}

func TestSprintsCRUDTaskMembershipAndTaskFilter(t *testing.T) {
	ts := setupTestRouter(t)
	defer ts.Close()

	adminToken := loginAndToken(t, ts.URL)
	codeToID := permissionCodeMap(t, ts.URL, adminToken)

	roleResp, roleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/rbac/roles", adminToken, map[string]any{
		"name":        "sprint-manager",
		"description": "sprint manager",
		"permissionIds": []uint{
			codeToID["sprints.create"],
			codeToID["sprints.read"],
			codeToID["sprints.update"],
			codeToID["sprints.delete"],
			codeToID["tasks.read"],
		},
	})
	if roleResp.StatusCode != http.StatusCreated {
		t.Fatalf("create sprint role expected 201 got %d", roleResp.StatusCode)
	}
	roleID := uint(roleBody["id"].(float64))

	userResp, userBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":      "sprint_manager",
		"name":          "Sprint Manager",
		"email":         "sprint_manager@example.com",
		"password":      "pass1234",
		"roleIds":       []uint{roleID},
		"departmentIds": []uint{},
	})
	if userResp.StatusCode != http.StatusCreated {
		t.Fatalf("create sprint manager expected 201 got %d", userResp.StatusCode)
	}
	managerID := uint(userBody["id"].(float64))

	projectResp, projectBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/projects", adminToken, map[string]any{
		"code":    "SPR-P1",
		"name":    "Sprint Project",
		"userIds": []uint{managerID},
	})
	if projectResp.StatusCode != http.StatusCreated {
		t.Fatalf("create sprint project expected 201 got %d", projectResp.StatusCode)
	}
	projectID := uint(projectBody["id"].(float64))

	openTaskResp, openTaskBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/tasks", adminToken, map[string]any{
		"title":       "实现迭代计划",
		"projectId":   projectID,
		"assigneeIds": []uint{managerID},
		"status":      "processing",
	})
	if openTaskResp.StatusCode != http.StatusCreated {
		t.Fatalf("create visible sprint task expected 201 got %d %#v", openTaskResp.StatusCode, openTaskBody)
	}
	openTaskID := uint(openTaskBody["id"].(float64))

	doneTaskResp, doneTaskBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/tasks", adminToken, map[string]any{
		"title":       "完成迭代评审",
		"projectId":   projectID,
		"assigneeIds": []uint{managerID},
		"status":      "completed",
		"progress":    100,
	})
	if doneTaskResp.StatusCode != http.StatusCreated {
		t.Fatalf("create completed sprint task expected 201 got %d %#v", doneTaskResp.StatusCode, doneTaskBody)
	}
	doneTaskID := uint(doneTaskBody["id"].(float64))

	hiddenTaskResp, hiddenTaskBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/tasks", adminToken, map[string]any{
		"title":     "隐藏迭代任务",
		"projectId": projectID,
		"status":    "queued",
	})
	if hiddenTaskResp.StatusCode != http.StatusCreated {
		t.Fatalf("create hidden sprint task expected 201 got %d %#v", hiddenTaskResp.StatusCode, hiddenTaskBody)
	}
	hiddenTaskID := uint(hiddenTaskBody["id"].(float64))

	loginStatus, loginBody := loginWithCredentials(t, ts.URL, "sprint_manager", "pass1234")
	if loginStatus != http.StatusOK {
		t.Fatalf("login sprint manager expected 200 got %d", loginStatus)
	}
	managerToken := loginBody["token"].(string)

	invalidResp, invalidBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/sprints", managerToken, map[string]any{
		"name":   "坏迭代",
		"status": "paused",
	})
	if invalidResp.StatusCode != http.StatusBadRequest || invalidBody["code"] != "INVALID_SPRINT_STATUS" {
		t.Fatalf("invalid sprint status expected 400 INVALID_SPRINT_STATUS got %d %#v", invalidResp.StatusCode, invalidBody)
	}

	createResp, createBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/sprints", managerToken, map[string]any{
		"name":          "研发 Sprint 1",
		"goal":          "交付迭代 MVP",
		"status":        "active",
		"startAt":       "2026-07-01T00:00:00Z",
		"endAt":         "2026-07-14T00:00:00Z",
		"capacityHours": 80,
	})
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("create sprint expected 201 got %d %#v", createResp.StatusCode, createBody)
	}
	sprintID := int(createBody["id"].(float64))
	if createBody["status"] != "active" || int(createBody["taskCount"].(float64)) != 0 {
		t.Fatalf("created sprint should be active with no tasks got %#v", createBody)
	}

	hiddenAddResp, hiddenAddBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/sprints/"+strconv.Itoa(sprintID)+"/tasks", managerToken, map[string]any{
		"taskIds": []uint{openTaskID, hiddenTaskID},
	})
	if hiddenAddResp.StatusCode != http.StatusNotFound || hiddenAddBody["code"] != "TASK_NOT_FOUND" {
		t.Fatalf("add hidden sprint task expected 404 TASK_NOT_FOUND got %d %#v", hiddenAddResp.StatusCode, hiddenAddBody)
	}

	addResp, addBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/sprints/"+strconv.Itoa(sprintID)+"/tasks", managerToken, map[string]any{
		"taskIds": []uint{openTaskID, doneTaskID, openTaskID},
	})
	if addResp.StatusCode != http.StatusOK {
		t.Fatalf("add sprint tasks expected 200 got %d %#v", addResp.StatusCode, addBody)
	}
	if int(addBody["taskCount"].(float64)) != 2 || int(addBody["completedTaskCount"].(float64)) != 1 {
		t.Fatalf("sprint stats should count visible completed tasks got %#v", addBody)
	}
	if int(addBody["completionRate"].(float64)) != 50 {
		t.Fatalf("sprint completion should be 50 got %#v", addBody["completionRate"])
	}

	filterResp, filterBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/tasks?sprintId="+strconv.Itoa(sprintID)+"&page=1&pageSize=20", managerToken, nil)
	if filterResp.StatusCode != http.StatusOK {
		t.Fatalf("task sprint filter expected 200 got %d %#v", filterResp.StatusCode, filterBody)
	}
	filteredTasks, _ := filterBody["list"].([]any)
	if len(filteredTasks) != 2 {
		t.Fatalf("sprint task filter should return two visible tasks got %#v", filterBody)
	}

	removeResp, removeBody := requestJSON(t, http.MethodDelete, ts.URL+"/api/v1/sprints/"+strconv.Itoa(sprintID)+"/tasks/"+strconv.Itoa(int(openTaskID)), managerToken, nil)
	if removeResp.StatusCode != http.StatusOK {
		t.Fatalf("remove sprint task expected 200 got %d %#v", removeResp.StatusCode, removeBody)
	}
	if int(removeBody["taskCount"].(float64)) != 1 || int(removeBody["completedTaskCount"].(float64)) != 1 {
		t.Fatalf("sprint stats after remove should keep completed task got %#v", removeBody)
	}

	updateResp, updateBody := requestJSON(t, http.MethodPut, ts.URL+"/api/v1/sprints/"+strconv.Itoa(sprintID), managerToken, map[string]any{
		"name":          "研发 Sprint 1 复盘",
		"goal":          "完成复盘",
		"status":        "closed",
		"startAt":       "2026-07-01T00:00:00Z",
		"endAt":         "2026-07-14T00:00:00Z",
		"capacityHours": 80,
	})
	if updateResp.StatusCode != http.StatusOK {
		t.Fatalf("update sprint expected 200 got %d %#v", updateResp.StatusCode, updateBody)
	}
	if updateBody["status"] != "closed" || updateBody["name"] != "研发 Sprint 1 复盘" {
		t.Fatalf("sprint should update got %#v", updateBody)
	}

	deleteResp, _ := requestJSON(t, http.MethodDelete, ts.URL+"/api/v1/sprints/"+strconv.Itoa(sprintID), managerToken, nil)
	if deleteResp.StatusCode != http.StatusOK {
		t.Fatalf("delete sprint expected 200 got %d", deleteResp.StatusCode)
	}
}

func TestWebhookSubscriptionTaskStatusDeliveryAndRetry(t *testing.T) {
	var shouldSucceed atomic.Bool
	var receivedCount atomic.Int32
	var lastPayload atomic.Value
	var lastEventHeader atomic.Value
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedCount.Add(1)
		raw, _ := io.ReadAll(r.Body)
		lastPayload.Store(string(raw))
		lastEventHeader.Store(r.Header.Get("X-Project-Manager-Event"))
		if !shouldSucceed.Load() {
			http.Error(w, "temporary failure", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer target.Close()

	ts := setupTestRouterWithHandler(t, func(h *handler.Handler) {
		h.Cfg.WebhookPrivateOK = true
	})
	defer ts.Close()

	adminToken := loginAndToken(t, ts.URL)
	invalidResp, invalidBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/webhooks", adminToken, map[string]any{
		"name":  "bad event",
		"event": "task_created",
		"url":   target.URL,
	})
	if invalidResp.StatusCode != http.StatusBadRequest || invalidBody["code"] != "INVALID_WEBHOOK_EVENT" {
		t.Fatalf("invalid webhook event expected 400 INVALID_WEBHOOK_EVENT got %d %#v", invalidResp.StatusCode, invalidBody)
	}

	createWebhookResp, createWebhookBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/webhooks", adminToken, map[string]any{
		"name":      "状态同步",
		"event":     "task_status_changed",
		"url":       target.URL,
		"isEnabled": true,
	})
	if createWebhookResp.StatusCode != http.StatusCreated {
		t.Fatalf("create webhook expected 201 got %d %#v", createWebhookResp.StatusCode, createWebhookBody)
	}
	webhookID := int(createWebhookBody["id"].(float64))

	projectResp, projectBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/projects", adminToken, map[string]any{
		"code": "WH-P1",
		"name": "Webhook Project",
	})
	if projectResp.StatusCode != http.StatusCreated {
		t.Fatalf("create webhook project expected 201 got %d", projectResp.StatusCode)
	}
	projectID := uint(projectBody["id"].(float64))
	taskResp, taskBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/tasks", adminToken, map[string]any{
		"title":     "Webhook Task",
		"projectId": projectID,
		"status":    "pending",
	})
	if taskResp.StatusCode != http.StatusCreated {
		t.Fatalf("create webhook task expected 201 got %d %#v", taskResp.StatusCode, taskBody)
	}
	taskID := int(taskBody["id"].(float64))

	statusResp, statusBody := requestJSON(t, http.MethodPatch, ts.URL+"/api/v1/tasks/"+strconv.Itoa(taskID)+"/status", adminToken, map[string]any{
		"status": "processing",
	})
	if statusResp.StatusCode != http.StatusOK {
		t.Fatalf("update task status expected 200 got %d %#v", statusResp.StatusCode, statusBody)
	}
	if receivedCount.Load() != 1 {
		t.Fatalf("webhook target should receive first delivery got %d", receivedCount.Load())
	}
	if lastEventHeader.Load() != "task_status_changed" {
		t.Fatalf("webhook event header mismatch: %v", lastEventHeader.Load())
	}
	if !strings.Contains(lastPayload.Load().(string), `"fromStatus":"pending"`) || !strings.Contains(lastPayload.Load().(string), `"toStatus":"processing"`) {
		t.Fatalf("webhook payload should include status change got %s", lastPayload.Load().(string))
	}

	deliveriesResp, deliveriesBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/webhooks/deliveries?subscriptionId="+strconv.Itoa(webhookID), adminToken, nil)
	if deliveriesResp.StatusCode != http.StatusOK {
		t.Fatalf("list webhook deliveries expected 200 got %d %#v", deliveriesResp.StatusCode, deliveriesBody)
	}
	deliveries, _ := deliveriesBody["list"].([]any)
	if len(deliveries) != 1 {
		t.Fatalf("expected one webhook delivery got %#v", deliveriesBody)
	}
	delivery := deliveries[0].(map[string]any)
	if delivery["status"] != "failed" || int(delivery["attempts"].(float64)) != 1 || int(delivery["responseStatus"].(float64)) != http.StatusInternalServerError {
		t.Fatalf("delivery should fail with one attempt got %#v", delivery)
	}
	deliveryID := int(delivery["id"].(float64))

	shouldSucceed.Store(true)
	retryResp, retryBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/webhooks/deliveries/"+strconv.Itoa(deliveryID)+"/retry", adminToken, nil)
	if retryResp.StatusCode != http.StatusOK {
		t.Fatalf("retry webhook delivery expected 200 got %d %#v", retryResp.StatusCode, retryBody)
	}
	if retryBody["status"] != "success" || int(retryBody["attempts"].(float64)) != 2 {
		t.Fatalf("retry should mark success with two attempts got %#v", retryBody)
	}
	if receivedCount.Load() != 2 {
		t.Fatalf("webhook target should receive retry got %d", receivedCount.Load())
	}

	listResp, listBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/webhooks", adminToken, nil)
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("list webhook subscriptions expected 200 got %d", listResp.StatusCode)
	}
	subscriptions, _ := listBody["list"].([]any)
	if len(subscriptions) == 0 {
		t.Fatalf("webhook subscription list should include created item got %#v", listBody)
	}
	firstSubscription := subscriptions[0].(map[string]any)
	if firstSubscription["lastDeliveryStatus"] != "success" {
		t.Fatalf("subscription should track last delivery success got %#v", firstSubscription)
	}
}

func TestWebhookSubscriptionSkipsInvisibleTaskForScopedUser(t *testing.T) {
	var visibleCalls atomic.Int32
	var hiddenCalls atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/visible":
			visibleCalls.Add(1)
		case "/hidden":
			hiddenCalls.Add(1)
		default:
			t.Fatalf("unexpected webhook path %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer target.Close()

	ts := setupTestRouterWithHandler(t, func(h *handler.Handler) {
		h.Cfg.WebhookPrivateOK = true
	})
	defer ts.Close()

	adminToken := loginAndToken(t, ts.URL)
	codeToID := permissionCodeMap(t, ts.URL, adminToken)
	roleResp, roleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/rbac/roles", adminToken, map[string]any{
		"name": "webhook-scoped-user",
		"permissionIds": []uint{
			codeToID["webhooks.create"],
			codeToID["webhooks.read"],
		},
	})
	if roleResp.StatusCode != http.StatusCreated {
		t.Fatalf("create webhook scoped role expected 201 got %d %#v", roleResp.StatusCode, roleBody)
	}
	roleID := uint(roleBody["id"].(float64))

	visibleUserResp, visibleUserBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":      "webhook_visible_user",
		"name":          "Webhook Visible User",
		"email":         "webhook_visible_user@example.com",
		"password":      "pass1234",
		"roleIds":       []uint{roleID},
		"departmentIds": []uint{},
	})
	if visibleUserResp.StatusCode != http.StatusCreated {
		t.Fatalf("create visible webhook user expected 201 got %d %#v", visibleUserResp.StatusCode, visibleUserBody)
	}
	visibleUserID := uint(visibleUserBody["id"].(float64))

	hiddenUserResp, hiddenUserBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":      "webhook_hidden_user",
		"name":          "Webhook Hidden User",
		"email":         "webhook_hidden_user@example.com",
		"password":      "pass1234",
		"roleIds":       []uint{roleID},
		"departmentIds": []uint{},
	})
	if hiddenUserResp.StatusCode != http.StatusCreated {
		t.Fatalf("create hidden webhook user expected 201 got %d %#v", hiddenUserResp.StatusCode, hiddenUserBody)
	}

	visibleLoginStatus, visibleLoginBody := loginWithCredentials(t, ts.URL, "webhook_visible_user", "pass1234")
	if visibleLoginStatus != http.StatusOK {
		t.Fatalf("login visible webhook user expected 200 got %d", visibleLoginStatus)
	}
	visibleToken := visibleLoginBody["token"].(string)
	hiddenLoginStatus, hiddenLoginBody := loginWithCredentials(t, ts.URL, "webhook_hidden_user", "pass1234")
	if hiddenLoginStatus != http.StatusOK {
		t.Fatalf("login hidden webhook user expected 200 got %d", hiddenLoginStatus)
	}
	hiddenToken := hiddenLoginBody["token"].(string)

	visibleWebhookResp, visibleWebhookBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/webhooks", visibleToken, map[string]any{
		"name":      "可见任务同步",
		"event":     "task_status_changed",
		"url":       target.URL + "/visible",
		"isEnabled": true,
	})
	if visibleWebhookResp.StatusCode != http.StatusCreated {
		t.Fatalf("create visible user webhook expected 201 got %d %#v", visibleWebhookResp.StatusCode, visibleWebhookBody)
	}
	hiddenWebhookResp, hiddenWebhookBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/webhooks", hiddenToken, map[string]any{
		"name":      "不可见任务同步",
		"event":     "task_status_changed",
		"url":       target.URL + "/hidden",
		"isEnabled": true,
	})
	if hiddenWebhookResp.StatusCode != http.StatusCreated {
		t.Fatalf("create hidden user webhook expected 201 got %d %#v", hiddenWebhookResp.StatusCode, hiddenWebhookBody)
	}

	projectResp, projectBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/projects", adminToken, map[string]any{
		"code": "WH-SCOPE-P1",
		"name": "Webhook Scope Project",
	})
	if projectResp.StatusCode != http.StatusCreated {
		t.Fatalf("create webhook scope project expected 201 got %d %#v", projectResp.StatusCode, projectBody)
	}
	projectID := uint(projectBody["id"].(float64))
	taskResp, taskBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/tasks", adminToken, map[string]any{
		"title":       "Webhook Visible Scope Task",
		"projectId":   projectID,
		"status":      "pending",
		"assigneeIds": []uint{visibleUserID},
	})
	if taskResp.StatusCode != http.StatusCreated {
		t.Fatalf("create webhook scope task expected 201 got %d %#v", taskResp.StatusCode, taskBody)
	}
	taskID := int(taskBody["id"].(float64))

	statusResp, statusBody := requestJSON(t, http.MethodPatch, ts.URL+"/api/v1/tasks/"+strconv.Itoa(taskID)+"/status", adminToken, map[string]any{
		"status": "processing",
	})
	if statusResp.StatusCode != http.StatusOK {
		t.Fatalf("update webhook scope task status expected 200 got %d %#v", statusResp.StatusCode, statusBody)
	}
	if visibleCalls.Load() != 1 {
		t.Fatalf("visible user webhook should receive one delivery got %d", visibleCalls.Load())
	}
	if hiddenCalls.Load() != 0 {
		t.Fatalf("hidden user webhook should not receive delivery got %d", hiddenCalls.Load())
	}

	visibleDeliveriesResp, visibleDeliveriesBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/webhooks/deliveries", visibleToken, nil)
	if visibleDeliveriesResp.StatusCode != http.StatusOK {
		t.Fatalf("visible deliveries expected 200 got %d %#v", visibleDeliveriesResp.StatusCode, visibleDeliveriesBody)
	}
	visibleDeliveries, _ := visibleDeliveriesBody["list"].([]any)
	if len(visibleDeliveries) != 1 {
		t.Fatalf("visible user should see one delivery got %#v", visibleDeliveriesBody)
	}
	hiddenDeliveriesResp, hiddenDeliveriesBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/webhooks/deliveries", hiddenToken, nil)
	if hiddenDeliveriesResp.StatusCode != http.StatusOK {
		t.Fatalf("hidden deliveries expected 200 got %d %#v", hiddenDeliveriesResp.StatusCode, hiddenDeliveriesBody)
	}
	hiddenDeliveries, _ := hiddenDeliveriesBody["list"].([]any)
	if len(hiddenDeliveries) != 0 {
		t.Fatalf("hidden user should not see delivery for invisible task got %#v", hiddenDeliveriesBody)
	}
}

func TestAPITokenServiceAccountAuthAndAudit(t *testing.T) {
	ts := setupTestRouter(t)
	defer ts.Close()

	adminToken := loginAndToken(t, ts.URL)
	codeToID := permissionCodeMap(t, ts.URL, adminToken)
	for _, code := range []string{"system.api_tokens.create", "system.api_tokens.read", "system.api_tokens.update", "system.api_tokens.delete", "projects.create", "projects.read", "system.audit.read"} {
		if codeToID[code] == 0 {
			t.Fatalf("permission %s should be seeded", code)
		}
	}

	invalidResp, invalidBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/api-tokens", adminToken, map[string]any{
		"name":          "empty permissions",
		"permissionIds": []uint{},
	})
	if invalidResp.StatusCode != http.StatusBadRequest || invalidBody["code"] != "INVALID_API_TOKEN" {
		t.Fatalf("empty api token permissions expected 400 INVALID_API_TOKEN got %d %#v", invalidResp.StatusCode, invalidBody)
	}

	createResp, createBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/api-tokens", adminToken, map[string]any{
		"name":        "Project Sync Token",
		"description": "sync projects from external system",
		"permissionIds": []uint{
			codeToID["projects.create"],
			codeToID["projects.read"],
		},
		"isEnabled": true,
	})
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("create api token expected 201 got %d %#v", createResp.StatusCode, createBody)
	}
	plainToken, _ := createBody["token"].(string)
	if !strings.HasPrefix(plainToken, "pmt_") {
		t.Fatalf("create response should include one-time token with pmt_ prefix got %#v", createBody)
	}
	if _, ok := createBody["tokenHash"]; ok {
		t.Fatalf("create response should not expose token hash got %#v", createBody)
	}
	apiTokenID := int(createBody["id"].(float64))
	serviceAccountID := uint(createBody["serviceAccountId"].(float64))
	if serviceAccountID == 0 {
		t.Fatalf("api token should create service account got %#v", createBody)
	}

	listResp, listBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/system/api-tokens", adminToken, nil)
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("list api tokens expected 200 got %d %#v", listResp.StatusCode, listBody)
	}
	tokens, _ := listBody["list"].([]any)
	if len(tokens) != 1 {
		t.Fatalf("list api tokens should contain one item got %#v", listBody)
	}
	listItem := tokens[0].(map[string]any)
	if _, ok := listItem["token"]; ok {
		t.Fatalf("list response should not expose token got %#v", listItem)
	}
	if _, ok := listItem["tokenHash"]; ok {
		t.Fatalf("list response should not expose token hash got %#v", listItem)
	}
	if listItem["tokenPrefix"] == "" || listItem["tokenLastFour"] == "" {
		t.Fatalf("list response should include masked token metadata got %#v", listItem)
	}

	projectResp, projectBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/projects", plainToken, map[string]any{
		"code":        "API-TOKEN-P1",
		"name":        "API Token Project",
		"description": "created by service account",
	})
	if projectResp.StatusCode != http.StatusCreated {
		t.Fatalf("api token create project expected 201 got %d %#v", projectResp.StatusCode, projectBody)
	}

	projectListResp, projectListBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/projects?page=1&pageSize=20", plainToken, nil)
	if projectListResp.StatusCode != http.StatusOK {
		t.Fatalf("api token list projects expected 200 got %d %#v", projectListResp.StatusCode, projectListBody)
	}
	taskListResp, taskListBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/tasks?page=1&pageSize=20", plainToken, nil)
	if taskListResp.StatusCode != http.StatusForbidden || taskListBody["code"] != "FORBIDDEN" {
		t.Fatalf("api token without tasks.read expected 403 FORBIDDEN got %d %#v", taskListResp.StatusCode, taskListBody)
	}

	auditResp, auditBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/system/audit/logs?module=projects&action=create&page=1&pageSize=20", adminToken, nil)
	if auditResp.StatusCode != http.StatusOK {
		t.Fatalf("list project audit expected 200 got %d %#v", auditResp.StatusCode, auditBody)
	}
	auditItems, _ := auditBody["list"].([]any)
	if len(auditItems) == 0 {
		t.Fatalf("project create should write audit log got %#v", auditBody)
	}
	foundServiceAudit := false
	for _, raw := range auditItems {
		item := raw.(map[string]any)
		if uint(item["userId"].(float64)) == serviceAccountID {
			foundServiceAudit = true
			break
		}
	}
	if !foundServiceAudit {
		t.Fatalf("project audit should use service account id %d got %#v", serviceAccountID, auditBody)
	}

	updateResp, updateBody := requestJSON(t, http.MethodPut, ts.URL+"/api/v1/system/api-tokens/"+strconv.Itoa(apiTokenID), adminToken, map[string]any{
		"name":        "Project Sync Token",
		"description": "disabled token",
		"permissionIds": []uint{
			codeToID["projects.create"],
			codeToID["projects.read"],
		},
		"isEnabled": false,
	})
	if updateResp.StatusCode != http.StatusOK {
		t.Fatalf("disable api token expected 200 got %d %#v", updateResp.StatusCode, updateBody)
	}
	disabledResp, disabledBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/projects?page=1&pageSize=20", plainToken, nil)
	if disabledResp.StatusCode != http.StatusUnauthorized || disabledBody["code"] != "UNAUTHORIZED" {
		t.Fatalf("disabled api token expected 401 UNAUTHORIZED got %d %#v", disabledResp.StatusCode, disabledBody)
	}

	enableResp, enableBody := requestJSON(t, http.MethodPut, ts.URL+"/api/v1/system/api-tokens/"+strconv.Itoa(apiTokenID), adminToken, map[string]any{
		"name":        "Project Sync Token",
		"description": "enabled token",
		"permissionIds": []uint{
			codeToID["projects.create"],
			codeToID["projects.read"],
		},
		"isEnabled": true,
	})
	if enableResp.StatusCode != http.StatusOK {
		t.Fatalf("enable api token expected 200 got %d %#v", enableResp.StatusCode, enableBody)
	}
	enabledResp, enabledBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/projects?page=1&pageSize=20", plainToken, nil)
	if enabledResp.StatusCode != http.StatusOK {
		t.Fatalf("enabled api token expected 200 got %d %#v", enabledResp.StatusCode, enabledBody)
	}

	deleteResp, deleteBody := requestJSON(t, http.MethodDelete, ts.URL+"/api/v1/system/api-tokens/"+strconv.Itoa(apiTokenID), adminToken, nil)
	if deleteResp.StatusCode != http.StatusOK {
		t.Fatalf("delete api token expected 200 got %d %#v", deleteResp.StatusCode, deleteBody)
	}
	revokedResp, revokedBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/projects?page=1&pageSize=20", plainToken, nil)
	if revokedResp.StatusCode != http.StatusUnauthorized || revokedBody["code"] != "UNAUTHORIZED" {
		t.Fatalf("revoked api token expected 401 UNAUTHORIZED got %d %#v", revokedResp.StatusCode, revokedBody)
	}
}

func TestAutomationRuleOverdueTaskNotification(t *testing.T) {
	ts := setupTestRouter(t)
	defer ts.Close()

	adminToken := loginAndToken(t, ts.URL)
	codeToID := permissionCodeMap(t, ts.URL, adminToken)

	notificationRoleResp, notificationRoleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/rbac/roles", adminToken, map[string]any{
		"name":        "automation-notification-reader",
		"description": "can receive notifications",
		"permissionIds": []uint{
			codeToID["notifications.read"],
			codeToID["notifications.update"],
		},
	})
	if notificationRoleResp.StatusCode != http.StatusCreated {
		t.Fatalf("create notification role expected 201 got %d", notificationRoleResp.StatusCode)
	}
	notificationRoleID := uint(notificationRoleBody["id"].(float64))

	readOnlyRoleResp, readOnlyRoleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/rbac/roles", adminToken, map[string]any{
		"name":        "automation-read-only",
		"description": "can read automation only",
		"permissionIds": []uint{
			codeToID["automations.read"],
		},
	})
	if readOnlyRoleResp.StatusCode != http.StatusCreated {
		t.Fatalf("create automation read-only role expected 201 got %d", readOnlyRoleResp.StatusCode)
	}
	readOnlyRoleID := uint(readOnlyRoleBody["id"].(float64))

	ownerResp, ownerBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":      "automation_owner",
		"name":          "Automation Owner",
		"email":         "automation_owner@example.com",
		"password":      "pass1234",
		"roleIds":       []uint{notificationRoleID},
		"departmentIds": []uint{},
	})
	if ownerResp.StatusCode != http.StatusCreated {
		t.Fatalf("create automation owner expected 201 got %d", ownerResp.StatusCode)
	}
	ownerID := uint(ownerBody["id"].(float64))

	assigneeResp, assigneeBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":      "automation_assignee",
		"name":          "Automation Assignee",
		"email":         "automation_assignee@example.com",
		"password":      "pass1234",
		"roleIds":       []uint{notificationRoleID},
		"departmentIds": []uint{},
	})
	if assigneeResp.StatusCode != http.StatusCreated {
		t.Fatalf("create automation assignee expected 201 got %d", assigneeResp.StatusCode)
	}
	assigneeID := uint(assigneeBody["id"].(float64))

	readOnlyUserResp, _ := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":      "automation_read_only",
		"name":          "Automation Read Only",
		"email":         "automation_read_only@example.com",
		"password":      "pass1234",
		"roleIds":       []uint{readOnlyRoleID},
		"departmentIds": []uint{},
	})
	if readOnlyUserResp.StatusCode != http.StatusCreated {
		t.Fatalf("create automation read-only user expected 201 got %d", readOnlyUserResp.StatusCode)
	}
	readOnlyLoginStatus, readOnlyLoginBody := loginWithCredentials(t, ts.URL, "automation_read_only", "pass1234")
	if readOnlyLoginStatus != http.StatusOK {
		t.Fatalf("login automation read-only expected 200 got %d", readOnlyLoginStatus)
	}
	readOnlyToken := readOnlyLoginBody["token"].(string)

	assigneeLoginStatus, assigneeLoginBody := loginWithCredentials(t, ts.URL, "automation_assignee", "pass1234")
	if assigneeLoginStatus != http.StatusOK {
		t.Fatalf("login automation assignee expected 200 got %d", assigneeLoginStatus)
	}
	assigneeToken := assigneeLoginBody["token"].(string)
	ownerLoginStatus, ownerLoginBody := loginWithCredentials(t, ts.URL, "automation_owner", "pass1234")
	if ownerLoginStatus != http.StatusOK {
		t.Fatalf("login automation owner expected 200 got %d", ownerLoginStatus)
	}
	ownerToken := ownerLoginBody["token"].(string)

	projectResp, projectBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/projects", adminToken, map[string]any{
		"code":    "AUTO-P1",
		"name":    "Automation Project",
		"userIds": []uint{ownerID},
	})
	if projectResp.StatusCode != http.StatusCreated {
		t.Fatalf("create automation project expected 201 got %d", projectResp.StatusCode)
	}
	projectID := int(projectBody["id"].(float64))

	taskResp, taskBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/tasks", adminToken, map[string]any{
		"title":       "Automation overdue task",
		"projectId":   projectID,
		"assigneeIds": []uint{assigneeID},
		"startAt":     "2000-01-01T00:00:00Z",
		"endAt":       "2000-01-02T00:00:00Z",
	})
	if taskResp.StatusCode != http.StatusCreated {
		t.Fatalf("create overdue task expected 201 got %d %#v", taskResp.StatusCode, taskBody)
	}

	ruleResp, ruleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/automation-rules", adminToken, map[string]any{
		"name":      "逾期 1 天提醒",
		"trigger":   "task_overdue",
		"isEnabled": true,
		"conditions": map[string]any{
			"overdueDays": 1,
			"projectIds":  []uint{uint(projectID)},
		},
		"actions": map[string]any{
			"notifyAssignees":     true,
			"notifyProjectOwners": true,
		},
	})
	if ruleResp.StatusCode != http.StatusCreated {
		t.Fatalf("create automation rule expected 201 got %d %#v", ruleResp.StatusCode, ruleBody)
	}
	ruleID := int(ruleBody["id"].(float64))

	listResp, listBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/automation-rules?keyword=逾期", adminToken, nil)
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("list automation rules expected 200 got %d", listResp.StatusCode)
	}
	if list, ok := listBody["list"].([]any); !ok || len(list) != 1 {
		t.Fatalf("automation list should include rule got %#v", listBody)
	}

	forbiddenRunResp, _ := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/automation-rules/"+strconv.Itoa(ruleID)+"/run", readOnlyToken, nil)
	if forbiddenRunResp.StatusCode != http.StatusForbidden {
		t.Fatalf("automation read-only run expected 403 got %d", forbiddenRunResp.StatusCode)
	}

	runResp, runBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/automation-rules/"+strconv.Itoa(ruleID)+"/run", adminToken, nil)
	if runResp.StatusCode != http.StatusOK {
		t.Fatalf("run automation rule expected 200 got %d %#v", runResp.StatusCode, runBody)
	}
	if runBody["status"] != "success" || int(runBody["matchedCount"].(float64)) != 1 || int(runBody["actionCount"].(float64)) != 2 {
		t.Fatalf("automation run should match 1 task and notify 2 users got %#v", runBody)
	}

	logResp, logBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/automation-rules/logs?ruleId="+strconv.Itoa(ruleID), adminToken, nil)
	if logResp.StatusCode != http.StatusOK {
		t.Fatalf("list automation logs expected 200 got %d", logResp.StatusCode)
	}
	logItems, _ := logBody["list"].([]any)
	if len(logItems) == 0 {
		t.Fatalf("automation logs should include execution got %#v", logBody)
	}

	assigneeNotificationResp, assigneeNotificationBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/notifications?module=tasks&keyword=逾期", assigneeToken, nil)
	if assigneeNotificationResp.StatusCode != http.StatusOK {
		t.Fatalf("assignee notifications expected 200 got %d", assigneeNotificationResp.StatusCode)
	}
	assigneeNotifications, _ := assigneeNotificationBody["list"].([]any)
	if len(assigneeNotifications) == 0 {
		t.Fatalf("assignee should receive overdue notification got %#v", assigneeNotificationBody)
	}

	ownerNotificationResp, ownerNotificationBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/notifications?module=tasks&keyword=逾期", ownerToken, nil)
	if ownerNotificationResp.StatusCode != http.StatusOK {
		t.Fatalf("owner notifications expected 200 got %d", ownerNotificationResp.StatusCode)
	}
	ownerNotifications, _ := ownerNotificationBody["list"].([]any)
	if len(ownerNotifications) == 0 {
		t.Fatalf("project owner should receive overdue notification got %#v", ownerNotificationBody)
	}
}

func TestAutomationRuleTaskStatusChangedCommentAndNotification(t *testing.T) {
	ts := setupTestRouter(t)
	defer ts.Close()

	adminToken := loginAndToken(t, ts.URL)
	codeToID := permissionCodeMap(t, ts.URL, adminToken)

	collaboratorRoleResp, collaboratorRoleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/rbac/roles", adminToken, map[string]any{
		"name":        "automation-status-collaborator",
		"description": "can update task status and read notifications",
		"permissionIds": []uint{
			codeToID["projects.read"],
			codeToID["tasks.read"],
			codeToID["notifications.read"],
			codeToID["notifications.update"],
		},
	})
	if collaboratorRoleResp.StatusCode != http.StatusCreated {
		t.Fatalf("create automation status collaborator role expected 201 got %d", collaboratorRoleResp.StatusCode)
	}
	collaboratorRoleID := uint(collaboratorRoleBody["id"].(float64))

	notificationRoleResp, notificationRoleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/rbac/roles", adminToken, map[string]any{
		"name":        "automation-status-owner",
		"description": "can read automation status notifications",
		"permissionIds": []uint{
			codeToID["notifications.read"],
			codeToID["notifications.update"],
		},
	})
	if notificationRoleResp.StatusCode != http.StatusCreated {
		t.Fatalf("create automation status owner role expected 201 got %d", notificationRoleResp.StatusCode)
	}
	notificationRoleID := uint(notificationRoleBody["id"].(float64))

	ownerResp, ownerBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":      "automation_status_owner",
		"name":          "Automation Status Owner",
		"email":         "automation_status_owner@example.com",
		"password":      "pass1234",
		"roleIds":       []uint{notificationRoleID},
		"departmentIds": []uint{},
	})
	if ownerResp.StatusCode != http.StatusCreated {
		t.Fatalf("create automation status owner expected 201 got %d", ownerResp.StatusCode)
	}
	ownerID := uint(ownerBody["id"].(float64))

	assigneeResp, assigneeBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":      "automation_status_assignee",
		"name":          "Automation Status Assignee",
		"email":         "automation_status_assignee@example.com",
		"password":      "pass1234",
		"roleIds":       []uint{collaboratorRoleID},
		"departmentIds": []uint{},
	})
	if assigneeResp.StatusCode != http.StatusCreated {
		t.Fatalf("create automation status assignee expected 201 got %d", assigneeResp.StatusCode)
	}
	assigneeID := uint(assigneeBody["id"].(float64))

	assigneeLoginStatus, assigneeLoginBody := loginWithCredentials(t, ts.URL, "automation_status_assignee", "pass1234")
	if assigneeLoginStatus != http.StatusOK {
		t.Fatalf("login automation status assignee expected 200 got %d", assigneeLoginStatus)
	}
	assigneeToken := assigneeLoginBody["token"].(string)
	ownerLoginStatus, ownerLoginBody := loginWithCredentials(t, ts.URL, "automation_status_owner", "pass1234")
	if ownerLoginStatus != http.StatusOK {
		t.Fatalf("login automation status owner expected 200 got %d", ownerLoginStatus)
	}
	ownerToken := ownerLoginBody["token"].(string)

	projectResp, projectBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/projects", adminToken, map[string]any{
		"code":    "AUTO-STATUS-P1",
		"name":    "Automation Status Project",
		"userIds": []uint{ownerID},
	})
	if projectResp.StatusCode != http.StatusCreated {
		t.Fatalf("create automation status project expected 201 got %d", projectResp.StatusCode)
	}
	projectID := int(projectBody["id"].(float64))

	taskResp, taskBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/tasks", adminToken, map[string]any{
		"title":       "Automation status task",
		"projectId":   projectID,
		"status":      "pending",
		"progress":    10,
		"assigneeIds": []uint{assigneeID},
	})
	if taskResp.StatusCode != http.StatusCreated {
		t.Fatalf("create automation status task expected 201 got %d %#v", taskResp.StatusCode, taskBody)
	}
	taskID := int(taskBody["id"].(float64))
	taskNo := taskBody["taskNo"].(string)

	invalidRuleResp, invalidRuleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/automation-rules", adminToken, map[string]any{
		"name":    "无效状态条件",
		"trigger": "task_status_changed",
		"conditions": map[string]any{
			"fromStatuses": []string{"pending"},
			"toStatuses":   []string{"bogus"},
		},
		"actions": map[string]any{
			"addComment": true,
		},
	})
	if invalidRuleResp.StatusCode != http.StatusBadRequest || invalidRuleBody["code"] != "INVALID_AUTOMATION_RULE" {
		t.Fatalf("invalid status condition expected 400 INVALID_AUTOMATION_RULE got %d %#v", invalidRuleResp.StatusCode, invalidRuleBody)
	}

	ruleResp, ruleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/automation-rules", adminToken, map[string]any{
		"name":      "状态变更提醒",
		"trigger":   "task_status_changed",
		"isEnabled": true,
		"conditions": map[string]any{
			"projectIds":   []uint{uint(projectID)},
			"fromStatuses": []string{"pending"},
			"toStatuses":   []string{"processing"},
		},
		"actions": map[string]any{
			"notifyAssignees":     true,
			"notifyProjectOwners": true,
			"addComment":          true,
			"commentContent":      "状态从 {fromStatus} 到 {toStatus}: {taskNo}",
		},
	})
	if ruleResp.StatusCode != http.StatusCreated {
		t.Fatalf("create status automation rule expected 201 got %d %#v", ruleResp.StatusCode, ruleBody)
	}
	ruleID := int(ruleBody["id"].(float64))

	listResp, listBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/automation-rules?trigger=task_status_changed&keyword=状态", adminToken, nil)
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("list status automation rules expected 200 got %d", listResp.StatusCode)
	}
	if list, ok := listBody["list"].([]any); !ok || len(list) != 1 {
		t.Fatalf("status automation list should include rule got %#v", listBody)
	}

	statusResp, statusBody := requestJSON(t, http.MethodPatch, ts.URL+"/api/v1/tasks/"+strconv.Itoa(taskID)+"/status", assigneeToken, map[string]any{
		"status": "processing",
	})
	if statusResp.StatusCode != http.StatusOK {
		t.Fatalf("patch status expected 200 got %d %#v", statusResp.StatusCode, statusBody)
	}

	commentResp, commentBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/tasks/"+strconv.Itoa(taskID)+"/comments", adminToken, nil)
	if commentResp.StatusCode != http.StatusOK {
		t.Fatalf("list automation comments expected 200 got %d", commentResp.StatusCode)
	}
	comments, _ := commentBody["list"].([]any)
	if len(comments) != 1 {
		t.Fatalf("status automation should create one comment got %#v", commentBody)
	}
	comment := comments[0].(map[string]any)
	if !strings.Contains(comment["content"].(string), "状态从 pending 到 processing: "+taskNo) {
		t.Fatalf("automation comment should render status placeholders got %#v", comment)
	}

	logResp, logBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/automation-rules/logs?ruleId="+strconv.Itoa(ruleID)+"&trigger=task_status_changed", adminToken, nil)
	if logResp.StatusCode != http.StatusOK {
		t.Fatalf("list status automation logs expected 200 got %d", logResp.StatusCode)
	}
	logItems, _ := logBody["list"].([]any)
	if len(logItems) != 1 {
		t.Fatalf("status automation should create one event log got %#v", logBody)
	}
	logItem := logItems[0].(map[string]any)
	if logItem["status"] != "success" || logItem["runSource"] != "event" || int(logItem["matchedCount"].(float64)) != 1 || int(logItem["actionCount"].(float64)) != 3 {
		t.Fatalf("status automation log should record event success with 3 actions got %#v", logItem)
	}

	assigneeNotificationResp, assigneeNotificationBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/notifications?module=tasks&keyword=状态", assigneeToken, nil)
	if assigneeNotificationResp.StatusCode != http.StatusOK {
		t.Fatalf("assignee status notifications expected 200 got %d", assigneeNotificationResp.StatusCode)
	}
	assigneeNotifications, _ := assigneeNotificationBody["list"].([]any)
	if len(assigneeNotifications) == 0 {
		t.Fatalf("assignee should receive status automation notification got %#v", assigneeNotificationBody)
	}

	ownerNotificationResp, ownerNotificationBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/notifications?module=tasks&keyword=状态", ownerToken, nil)
	if ownerNotificationResp.StatusCode != http.StatusOK {
		t.Fatalf("owner status notifications expected 200 got %d", ownerNotificationResp.StatusCode)
	}
	ownerNotifications, _ := ownerNotificationBody["list"].([]any)
	if len(ownerNotifications) == 0 {
		t.Fatalf("project owner should receive status automation notification got %#v", ownerNotificationBody)
	}

	sameStatusResp, sameStatusBody := requestJSON(t, http.MethodPatch, ts.URL+"/api/v1/tasks/"+strconv.Itoa(taskID)+"/status", assigneeToken, map[string]any{
		"status": "processing",
	})
	if sameStatusResp.StatusCode != http.StatusOK {
		t.Fatalf("patch same status expected 200 got %d %#v", sameStatusResp.StatusCode, sameStatusBody)
	}
	duplicateLogResp, duplicateLogBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/automation-rules/logs?ruleId="+strconv.Itoa(ruleID), adminToken, nil)
	if duplicateLogResp.StatusCode != http.StatusOK {
		t.Fatalf("list duplicate status automation logs expected 200 got %d", duplicateLogResp.StatusCode)
	}
	duplicateLogItems, _ := duplicateLogBody["list"].([]any)
	if len(duplicateLogItems) != 1 {
		t.Fatalf("same status patch should not trigger automation again got %#v", duplicateLogBody)
	}
}

func TestAutomationRuleTaskProgressChangedCommentAndNotification(t *testing.T) {
	ts := setupTestRouter(t)
	defer ts.Close()

	adminToken := loginAndToken(t, ts.URL)
	codeToID := permissionCodeMap(t, ts.URL, adminToken)

	collaboratorRoleResp, collaboratorRoleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/rbac/roles", adminToken, map[string]any{
		"name":        "automation-progress-collaborator",
		"description": "can update task progress and read notifications",
		"permissionIds": []uint{
			codeToID["projects.read"],
			codeToID["tasks.read"],
			codeToID["notifications.read"],
			codeToID["notifications.update"],
		},
	})
	if collaboratorRoleResp.StatusCode != http.StatusCreated {
		t.Fatalf("create automation progress collaborator role expected 201 got %d", collaboratorRoleResp.StatusCode)
	}
	collaboratorRoleID := uint(collaboratorRoleBody["id"].(float64))

	notificationRoleResp, notificationRoleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/rbac/roles", adminToken, map[string]any{
		"name":        "automation-progress-owner",
		"description": "can read automation progress notifications",
		"permissionIds": []uint{
			codeToID["notifications.read"],
			codeToID["notifications.update"],
		},
	})
	if notificationRoleResp.StatusCode != http.StatusCreated {
		t.Fatalf("create automation progress owner role expected 201 got %d", notificationRoleResp.StatusCode)
	}
	notificationRoleID := uint(notificationRoleBody["id"].(float64))

	ownerResp, ownerBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":      "automation_progress_owner",
		"name":          "Automation Progress Owner",
		"email":         "automation_progress_owner@example.com",
		"password":      "pass1234",
		"roleIds":       []uint{notificationRoleID},
		"departmentIds": []uint{},
	})
	if ownerResp.StatusCode != http.StatusCreated {
		t.Fatalf("create automation progress owner expected 201 got %d", ownerResp.StatusCode)
	}
	ownerID := uint(ownerBody["id"].(float64))

	assigneeResp, assigneeBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":      "automation_progress_assignee",
		"name":          "Automation Progress Assignee",
		"email":         "automation_progress_assignee@example.com",
		"password":      "pass1234",
		"roleIds":       []uint{collaboratorRoleID},
		"departmentIds": []uint{},
	})
	if assigneeResp.StatusCode != http.StatusCreated {
		t.Fatalf("create automation progress assignee expected 201 got %d", assigneeResp.StatusCode)
	}
	assigneeID := uint(assigneeBody["id"].(float64))

	assigneeLoginStatus, assigneeLoginBody := loginWithCredentials(t, ts.URL, "automation_progress_assignee", "pass1234")
	if assigneeLoginStatus != http.StatusOK {
		t.Fatalf("login automation progress assignee expected 200 got %d", assigneeLoginStatus)
	}
	assigneeToken := assigneeLoginBody["token"].(string)
	ownerLoginStatus, ownerLoginBody := loginWithCredentials(t, ts.URL, "automation_progress_owner", "pass1234")
	if ownerLoginStatus != http.StatusOK {
		t.Fatalf("login automation progress owner expected 200 got %d", ownerLoginStatus)
	}
	ownerToken := ownerLoginBody["token"].(string)

	projectResp, projectBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/projects", adminToken, map[string]any{
		"code":    "AUTO-PROGRESS-P1",
		"name":    "Automation Progress Project",
		"userIds": []uint{ownerID},
	})
	if projectResp.StatusCode != http.StatusCreated {
		t.Fatalf("create automation progress project expected 201 got %d", projectResp.StatusCode)
	}
	projectID := int(projectBody["id"].(float64))

	taskResp, taskBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/tasks", adminToken, map[string]any{
		"title":       "Automation progress task",
		"projectId":   projectID,
		"status":      "processing",
		"progress":    10,
		"assigneeIds": []uint{assigneeID},
	})
	if taskResp.StatusCode != http.StatusCreated {
		t.Fatalf("create automation progress task expected 201 got %d %#v", taskResp.StatusCode, taskBody)
	}
	taskID := int(taskBody["id"].(float64))
	taskNo := taskBody["taskNo"].(string)

	invalidRuleResp, invalidRuleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/automation-rules", adminToken, map[string]any{
		"name":    "无效进度条件",
		"trigger": "task_progress_changed",
		"conditions": map[string]any{
			"toProgressMin": 101,
		},
		"actions": map[string]any{
			"addComment": true,
		},
	})
	if invalidRuleResp.StatusCode != http.StatusBadRequest || invalidRuleBody["code"] != "INVALID_AUTOMATION_RULE" {
		t.Fatalf("invalid progress condition expected 400 INVALID_AUTOMATION_RULE got %d %#v", invalidRuleResp.StatusCode, invalidRuleBody)
	}

	missingConditionResp, missingConditionBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/automation-rules", adminToken, map[string]any{
		"name":       "缺少进度条件",
		"trigger":    "task_progress_changed",
		"conditions": map[string]any{},
		"actions": map[string]any{
			"addComment": true,
		},
	})
	if missingConditionResp.StatusCode != http.StatusBadRequest || missingConditionBody["code"] != "INVALID_AUTOMATION_RULE" {
		t.Fatalf("missing progress condition expected 400 INVALID_AUTOMATION_RULE got %d %#v", missingConditionResp.StatusCode, missingConditionBody)
	}

	ruleResp, ruleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/automation-rules", adminToken, map[string]any{
		"name":      "进度达到 50 提醒",
		"trigger":   "task_progress_changed",
		"isEnabled": true,
		"conditions": map[string]any{
			"projectIds":      []uint{uint(projectID)},
			"fromProgressMax": 49,
			"toProgressMin":   50,
		},
		"actions": map[string]any{
			"notifyAssignees":     true,
			"notifyProjectOwners": true,
			"addComment":          true,
			"commentContent":      "进度从 {fromProgress} 到 {toProgress}: {taskNo}",
		},
	})
	if ruleResp.StatusCode != http.StatusCreated {
		t.Fatalf("create progress automation rule expected 201 got %d %#v", ruleResp.StatusCode, ruleBody)
	}
	ruleID := int(ruleBody["id"].(float64))

	runResp, runBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/automation-rules/"+strconv.Itoa(ruleID)+"/run", adminToken, nil)
	if runResp.StatusCode != http.StatusOK {
		t.Fatalf("manual run progress automation expected 200 got %d %#v", runResp.StatusCode, runBody)
	}
	if runBody["status"] != "skipped" {
		t.Fatalf("manual run progress automation should be skipped got %#v", runBody)
	}

	listResp, listBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/automation-rules?trigger=task_progress_changed&keyword=进度", adminToken, nil)
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("list progress automation rules expected 200 got %d", listResp.StatusCode)
	}
	if list, ok := listBody["list"].([]any); !ok || len(list) != 1 {
		t.Fatalf("progress automation list should include rule got %#v", listBody)
	}

	progressResp, progressBody := requestJSON(t, http.MethodPatch, ts.URL+"/api/v1/tasks/"+strconv.Itoa(taskID)+"/progress", assigneeToken, map[string]any{
		"progress": 55,
	})
	if progressResp.StatusCode != http.StatusOK {
		t.Fatalf("patch progress expected 200 got %d %#v", progressResp.StatusCode, progressBody)
	}

	commentResp, commentBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/tasks/"+strconv.Itoa(taskID)+"/comments", adminToken, nil)
	if commentResp.StatusCode != http.StatusOK {
		t.Fatalf("list automation progress comments expected 200 got %d", commentResp.StatusCode)
	}
	comments, _ := commentBody["list"].([]any)
	if len(comments) != 1 {
		t.Fatalf("progress automation should create one comment got %#v", commentBody)
	}
	comment := comments[0].(map[string]any)
	if !strings.Contains(comment["content"].(string), "进度从 10 到 55: "+taskNo) {
		t.Fatalf("automation progress comment should render progress placeholders got %#v", comment)
	}

	logResp, logBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/automation-rules/logs?ruleId="+strconv.Itoa(ruleID)+"&trigger=task_progress_changed", adminToken, nil)
	if logResp.StatusCode != http.StatusOK {
		t.Fatalf("list progress automation logs expected 200 got %d", logResp.StatusCode)
	}
	logItems, _ := logBody["list"].([]any)
	if len(logItems) != 2 {
		t.Fatalf("progress automation should create one event log got %#v", logBody)
	}
	logItem := logItems[0].(map[string]any)
	if logItem["status"] != "success" || logItem["runSource"] != "event" || int(logItem["matchedCount"].(float64)) != 1 || int(logItem["actionCount"].(float64)) != 3 {
		t.Fatalf("progress automation log should record event success with 3 actions got %#v", logItem)
	}

	assigneeNotificationResp, assigneeNotificationBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/notifications?module=tasks&keyword=进度", assigneeToken, nil)
	if assigneeNotificationResp.StatusCode != http.StatusOK {
		t.Fatalf("assignee progress notifications expected 200 got %d", assigneeNotificationResp.StatusCode)
	}
	assigneeNotifications, _ := assigneeNotificationBody["list"].([]any)
	if len(assigneeNotifications) == 0 {
		t.Fatalf("assignee should receive progress automation notification got %#v", assigneeNotificationBody)
	}

	ownerNotificationResp, ownerNotificationBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/notifications?module=tasks&keyword=进度", ownerToken, nil)
	if ownerNotificationResp.StatusCode != http.StatusOK {
		t.Fatalf("owner progress notifications expected 200 got %d", ownerNotificationResp.StatusCode)
	}
	ownerNotifications, _ := ownerNotificationBody["list"].([]any)
	if len(ownerNotifications) == 0 {
		t.Fatalf("project owner should receive progress automation notification got %#v", ownerNotificationBody)
	}

	sameProgressResp, sameProgressBody := requestJSON(t, http.MethodPatch, ts.URL+"/api/v1/tasks/"+strconv.Itoa(taskID)+"/progress", assigneeToken, map[string]any{
		"progress": 55,
	})
	if sameProgressResp.StatusCode != http.StatusOK {
		t.Fatalf("patch same progress expected 200 got %d %#v", sameProgressResp.StatusCode, sameProgressBody)
	}
	duplicateLogResp, duplicateLogBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/automation-rules/logs?ruleId="+strconv.Itoa(ruleID), adminToken, nil)
	if duplicateLogResp.StatusCode != http.StatusOK {
		t.Fatalf("list duplicate progress automation logs expected 200 got %d", duplicateLogResp.StatusCode)
	}
	duplicateLogItems, _ := duplicateLogBody["list"].([]any)
	if len(duplicateLogItems) != 2 {
		t.Fatalf("same progress patch should not trigger automation again got %#v", duplicateLogBody)
	}
}

func TestAutomationRuleTaskAssigneeChangedCommentAndNotification(t *testing.T) {
	ts := setupTestRouter(t)
	defer ts.Close()

	adminToken := loginAndToken(t, ts.URL)
	codeToID := permissionCodeMap(t, ts.URL, adminToken)

	notificationRoleResp, notificationRoleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/rbac/roles", adminToken, map[string]any{
		"name":        "automation-assignee-notification-reader",
		"description": "can read automation assignee notifications",
		"permissionIds": []uint{
			codeToID["notifications.read"],
			codeToID["notifications.update"],
		},
	})
	if notificationRoleResp.StatusCode != http.StatusCreated {
		t.Fatalf("create automation assignee notification role expected 201 got %d", notificationRoleResp.StatusCode)
	}
	notificationRoleID := uint(notificationRoleBody["id"].(float64))

	ownerResp, ownerBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":      "automation_assignee_owner",
		"name":          "Automation Assignee Owner",
		"email":         "automation_assignee_owner@example.com",
		"password":      "pass1234",
		"roleIds":       []uint{notificationRoleID},
		"departmentIds": []uint{},
	})
	if ownerResp.StatusCode != http.StatusCreated {
		t.Fatalf("create automation assignee owner expected 201 got %d", ownerResp.StatusCode)
	}
	ownerID := uint(ownerBody["id"].(float64))

	oldAssigneeResp, oldAssigneeBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":      "automation_assignee_old",
		"name":          "Automation Assignee Old",
		"email":         "automation_assignee_old@example.com",
		"password":      "pass1234",
		"roleIds":       []uint{notificationRoleID},
		"departmentIds": []uint{},
	})
	if oldAssigneeResp.StatusCode != http.StatusCreated {
		t.Fatalf("create old automation assignee expected 201 got %d", oldAssigneeResp.StatusCode)
	}
	oldAssigneeID := uint(oldAssigneeBody["id"].(float64))

	newAssigneeResp, newAssigneeBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":      "automation_assignee_new",
		"name":          "Automation Assignee New",
		"email":         "automation_assignee_new@example.com",
		"password":      "pass1234",
		"roleIds":       []uint{notificationRoleID},
		"departmentIds": []uint{},
	})
	if newAssigneeResp.StatusCode != http.StatusCreated {
		t.Fatalf("create new automation assignee expected 201 got %d", newAssigneeResp.StatusCode)
	}
	newAssigneeID := uint(newAssigneeBody["id"].(float64))

	newAssigneeLoginStatus, newAssigneeLoginBody := loginWithCredentials(t, ts.URL, "automation_assignee_new", "pass1234")
	if newAssigneeLoginStatus != http.StatusOK {
		t.Fatalf("login new automation assignee expected 200 got %d", newAssigneeLoginStatus)
	}
	newAssigneeToken := newAssigneeLoginBody["token"].(string)
	ownerLoginStatus, ownerLoginBody := loginWithCredentials(t, ts.URL, "automation_assignee_owner", "pass1234")
	if ownerLoginStatus != http.StatusOK {
		t.Fatalf("login automation assignee owner expected 200 got %d", ownerLoginStatus)
	}
	ownerToken := ownerLoginBody["token"].(string)

	projectResp, projectBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/projects", adminToken, map[string]any{
		"code":    "AUTO-ASSIGNEE-P1",
		"name":    "Automation Assignee Project",
		"userIds": []uint{ownerID},
	})
	if projectResp.StatusCode != http.StatusCreated {
		t.Fatalf("create automation assignee project expected 201 got %d", projectResp.StatusCode)
	}
	projectID := int(projectBody["id"].(float64))

	taskResp, taskBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/tasks", adminToken, map[string]any{
		"title":       "Automation assignee task",
		"projectId":   projectID,
		"status":      "processing",
		"progress":    20,
		"assigneeIds": []uint{oldAssigneeID},
	})
	if taskResp.StatusCode != http.StatusCreated {
		t.Fatalf("create automation assignee task expected 201 got %d %#v", taskResp.StatusCode, taskBody)
	}
	taskID := int(taskBody["id"].(float64))
	taskNo := taskBody["taskNo"].(string)

	invalidRuleResp, invalidRuleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/automation-rules", adminToken, map[string]any{
		"name":       "缺少执行人变更类型",
		"trigger":    "task_assignee_changed",
		"conditions": map[string]any{},
		"actions": map[string]any{
			"addComment": true,
		},
	})
	if invalidRuleResp.StatusCode != http.StatusBadRequest || invalidRuleBody["code"] != "INVALID_AUTOMATION_RULE" {
		t.Fatalf("missing assignee change type expected 400 INVALID_AUTOMATION_RULE got %d %#v", invalidRuleResp.StatusCode, invalidRuleBody)
	}

	ruleResp, ruleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/automation-rules", adminToken, map[string]any{
		"name":      "执行人新增提醒",
		"trigger":   "task_assignee_changed",
		"isEnabled": true,
		"conditions": map[string]any{
			"projectIds":          []uint{uint(projectID)},
			"assigneeChangeTypes": []string{"added"},
		},
		"actions": map[string]any{
			"notifyAssignees":     true,
			"notifyProjectOwners": true,
			"addComment":          true,
			"commentContent":      "执行人新增 {addedAssignees} 移除 {removedAssignees}: {taskNo}",
		},
	})
	if ruleResp.StatusCode != http.StatusCreated {
		t.Fatalf("create assignee automation rule expected 201 got %d %#v", ruleResp.StatusCode, ruleBody)
	}
	ruleID := int(ruleBody["id"].(float64))

	runResp, runBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/automation-rules/"+strconv.Itoa(ruleID)+"/run", adminToken, nil)
	if runResp.StatusCode != http.StatusOK {
		t.Fatalf("manual run assignee automation expected 200 got %d %#v", runResp.StatusCode, runBody)
	}
	if runBody["status"] != "skipped" {
		t.Fatalf("manual run assignee automation should be skipped got %#v", runBody)
	}

	listResp, listBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/automation-rules?trigger=task_assignee_changed&keyword=执行人", adminToken, nil)
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("list assignee automation rules expected 200 got %d", listResp.StatusCode)
	}
	if list, ok := listBody["list"].([]any); !ok || len(list) != 1 {
		t.Fatalf("assignee automation list should include rule got %#v", listBody)
	}

	updatePayload := map[string]any{
		"title":       "Automation assignee task",
		"projectId":   projectID,
		"status":      "processing",
		"priority":    "high",
		"progress":    20,
		"assigneeIds": []uint{oldAssigneeID, newAssigneeID},
		"reviewerIds": []uint{},
		"tagIds":      []uint{},
	}
	updateResp, updateBody := requestJSON(t, http.MethodPut, ts.URL+"/api/v1/tasks/"+strconv.Itoa(taskID), adminToken, updatePayload)
	if updateResp.StatusCode != http.StatusOK {
		t.Fatalf("update task assignees expected 200 got %d %#v", updateResp.StatusCode, updateBody)
	}

	commentResp, commentBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/tasks/"+strconv.Itoa(taskID)+"/comments", adminToken, nil)
	if commentResp.StatusCode != http.StatusOK {
		t.Fatalf("list automation assignee comments expected 200 got %d", commentResp.StatusCode)
	}
	comments, _ := commentBody["list"].([]any)
	if len(comments) != 1 {
		t.Fatalf("assignee automation should create one comment got %#v", commentBody)
	}
	comment := comments[0].(map[string]any)
	if !strings.Contains(comment["content"].(string), "执行人新增 Automation Assignee New 移除 无: "+taskNo) {
		t.Fatalf("automation assignee comment should render assignee placeholders got %#v", comment)
	}

	logResp, logBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/automation-rules/logs?ruleId="+strconv.Itoa(ruleID)+"&trigger=task_assignee_changed", adminToken, nil)
	if logResp.StatusCode != http.StatusOK {
		t.Fatalf("list assignee automation logs expected 200 got %d", logResp.StatusCode)
	}
	logItems, _ := logBody["list"].([]any)
	if len(logItems) != 2 {
		t.Fatalf("assignee automation should include manual skip and one event log got %#v", logBody)
	}
	logItem := logItems[0].(map[string]any)
	if logItem["status"] != "success" || logItem["runSource"] != "event" || int(logItem["matchedCount"].(float64)) != 1 || int(logItem["actionCount"].(float64)) != 4 {
		t.Fatalf("assignee automation log should record event success with 4 actions got %#v", logItem)
	}

	newAssigneeNotificationResp, newAssigneeNotificationBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/notifications?module=tasks&keyword=执行人已变更", newAssigneeToken, nil)
	if newAssigneeNotificationResp.StatusCode != http.StatusOK {
		t.Fatalf("new assignee notifications expected 200 got %d", newAssigneeNotificationResp.StatusCode)
	}
	newAssigneeNotifications, _ := newAssigneeNotificationBody["list"].([]any)
	if len(newAssigneeNotifications) == 0 {
		t.Fatalf("new assignee should receive assignee automation notification got %#v", newAssigneeNotificationBody)
	}

	ownerNotificationResp, ownerNotificationBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/notifications?module=tasks&keyword=执行人已变更", ownerToken, nil)
	if ownerNotificationResp.StatusCode != http.StatusOK {
		t.Fatalf("owner assignee notifications expected 200 got %d", ownerNotificationResp.StatusCode)
	}
	ownerNotifications, _ := ownerNotificationBody["list"].([]any)
	if len(ownerNotifications) == 0 {
		t.Fatalf("project owner should receive assignee automation notification got %#v", ownerNotificationBody)
	}

	sameAssigneesResp, sameAssigneesBody := requestJSON(t, http.MethodPut, ts.URL+"/api/v1/tasks/"+strconv.Itoa(taskID), adminToken, updatePayload)
	if sameAssigneesResp.StatusCode != http.StatusOK {
		t.Fatalf("update same assignees expected 200 got %d %#v", sameAssigneesResp.StatusCode, sameAssigneesBody)
	}
	duplicateLogResp, duplicateLogBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/automation-rules/logs?ruleId="+strconv.Itoa(ruleID), adminToken, nil)
	if duplicateLogResp.StatusCode != http.StatusOK {
		t.Fatalf("list duplicate assignee automation logs expected 200 got %d", duplicateLogResp.StatusCode)
	}
	duplicateLogItems, _ := duplicateLogBody["list"].([]any)
	if len(duplicateLogItems) != 2 {
		t.Fatalf("same assignee update should not trigger automation again got %#v", duplicateLogBody)
	}
}

func TestAutomationRuleAddTagsAction(t *testing.T) {
	ts := setupTestRouter(t)
	defer ts.Close()

	adminToken := loginAndToken(t, ts.URL)

	overdueTagResp, overdueTagBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/tags", adminToken, map[string]any{
		"name": "自动化逾期标签",
	})
	if overdueTagResp.StatusCode != http.StatusCreated {
		t.Fatalf("create overdue automation tag expected 201 got %d %#v", overdueTagResp.StatusCode, overdueTagBody)
	}
	overdueTagID := uint(overdueTagBody["id"].(float64))

	statusTagResp, statusTagBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/tags", adminToken, map[string]any{
		"name": "自动化状态标签",
	})
	if statusTagResp.StatusCode != http.StatusCreated {
		t.Fatalf("create status automation tag expected 201 got %d %#v", statusTagResp.StatusCode, statusTagBody)
	}
	statusTagID := uint(statusTagBody["id"].(float64))

	projectResp, projectBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/projects", adminToken, map[string]any{
		"code": "AUTO-TAG-P1",
		"name": "Automation Tag Project",
	})
	if projectResp.StatusCode != http.StatusCreated {
		t.Fatalf("create automation tag project expected 201 got %d", projectResp.StatusCode)
	}
	projectID := int(projectBody["id"].(float64))

	overdueTaskResp, overdueTaskBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/tasks", adminToken, map[string]any{
		"title":     "Automation overdue tag task",
		"projectId": projectID,
		"status":    "processing",
		"progress":  25,
		"startAt":   "2000-01-01T00:00:00Z",
		"endAt":     "2000-01-02T00:00:00Z",
	})
	if overdueTaskResp.StatusCode != http.StatusCreated {
		t.Fatalf("create overdue tag task expected 201 got %d %#v", overdueTaskResp.StatusCode, overdueTaskBody)
	}
	overdueTaskID := int(overdueTaskBody["id"].(float64))

	invalidRuleResp, invalidRuleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/automation-rules", adminToken, map[string]any{
		"name":    "缺少标签动作目标",
		"trigger": "task_overdue",
		"actions": map[string]any{
			"notifyAssignees":     false,
			"notifyProjectOwners": false,
			"addTags":             true,
			"tagIds":              []uint{},
		},
	})
	if invalidRuleResp.StatusCode != http.StatusBadRequest || invalidRuleBody["code"] != "INVALID_AUTOMATION_RULE" {
		t.Fatalf("missing tag action target expected 400 INVALID_AUTOMATION_RULE got %d %#v", invalidRuleResp.StatusCode, invalidRuleBody)
	}

	missingTagRuleResp, missingTagRuleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/automation-rules", adminToken, map[string]any{
		"name":    "不存在标签动作目标",
		"trigger": "task_overdue",
		"actions": map[string]any{
			"notifyAssignees":     false,
			"notifyProjectOwners": false,
			"addTags":             true,
			"tagIds":              []uint{999999},
		},
	})
	if missingTagRuleResp.StatusCode != http.StatusBadRequest || missingTagRuleBody["code"] != "INVALID_AUTOMATION_RULE" {
		t.Fatalf("missing tag id expected 400 INVALID_AUTOMATION_RULE got %d %#v", missingTagRuleResp.StatusCode, missingTagRuleBody)
	}

	overdueRuleResp, overdueRuleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/automation-rules", adminToken, map[string]any{
		"name":      "逾期自动添加标签",
		"trigger":   "task_overdue",
		"isEnabled": true,
		"conditions": map[string]any{
			"overdueDays": 1,
			"projectIds":  []uint{uint(projectID)},
		},
		"actions": map[string]any{
			"notifyAssignees":     false,
			"notifyProjectOwners": false,
			"addTags":             true,
			"tagIds":              []uint{overdueTagID},
		},
	})
	if overdueRuleResp.StatusCode != http.StatusCreated {
		t.Fatalf("create overdue tag automation rule expected 201 got %d %#v", overdueRuleResp.StatusCode, overdueRuleBody)
	}
	overdueRuleID := int(overdueRuleBody["id"].(float64))

	runResp, runBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/automation-rules/"+strconv.Itoa(overdueRuleID)+"/run", adminToken, nil)
	if runResp.StatusCode != http.StatusOK {
		t.Fatalf("run overdue tag automation expected 200 got %d %#v", runResp.StatusCode, runBody)
	}
	if runBody["status"] != "success" || int(runBody["matchedCount"].(float64)) != 1 || int(runBody["actionCount"].(float64)) != 1 {
		t.Fatalf("overdue tag automation should add one tag got %#v", runBody)
	}

	taggedTaskResp, taggedTaskBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/tasks?page=1&pageSize=20&tagIds="+strconv.Itoa(int(overdueTagID)), adminToken, nil)
	if taggedTaskResp.StatusCode != http.StatusOK {
		t.Fatalf("list overdue tagged tasks expected 200 got %d", taggedTaskResp.StatusCode)
	}
	taggedTasks, _ := taggedTaskBody["list"].([]any)
	if len(taggedTasks) != 1 || int(taggedTasks[0].(map[string]any)["id"].(float64)) != overdueTaskID {
		t.Fatalf("overdue tag automation should attach tag to task got %#v", taggedTaskBody)
	}

	duplicateRunResp, duplicateRunBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/automation-rules/"+strconv.Itoa(overdueRuleID)+"/run", adminToken, nil)
	if duplicateRunResp.StatusCode != http.StatusOK {
		t.Fatalf("rerun overdue tag automation expected 200 got %d %#v", duplicateRunResp.StatusCode, duplicateRunBody)
	}
	if int(duplicateRunBody["actionCount"].(float64)) != 0 {
		t.Fatalf("rerun overdue tag automation should not add duplicate tag got %#v", duplicateRunBody)
	}

	statusTaskResp, statusTaskBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/tasks", adminToken, map[string]any{
		"title":     "Automation status tag task",
		"projectId": projectID,
		"status":    "pending",
		"progress":  10,
	})
	if statusTaskResp.StatusCode != http.StatusCreated {
		t.Fatalf("create status tag task expected 201 got %d %#v", statusTaskResp.StatusCode, statusTaskBody)
	}
	statusTaskID := int(statusTaskBody["id"].(float64))

	statusRuleResp, statusRuleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/automation-rules", adminToken, map[string]any{
		"name":      "状态变更自动添加标签",
		"trigger":   "task_status_changed",
		"isEnabled": true,
		"conditions": map[string]any{
			"projectIds":   []uint{uint(projectID)},
			"fromStatuses": []string{"pending"},
			"toStatuses":   []string{"processing"},
		},
		"actions": map[string]any{
			"notifyAssignees":     false,
			"notifyProjectOwners": false,
			"addTags":             true,
			"tagIds":              []uint{statusTagID},
		},
	})
	if statusRuleResp.StatusCode != http.StatusCreated {
		t.Fatalf("create status tag automation rule expected 201 got %d %#v", statusRuleResp.StatusCode, statusRuleBody)
	}
	statusRuleID := int(statusRuleBody["id"].(float64))

	statusResp, statusBody := requestJSON(t, http.MethodPatch, ts.URL+"/api/v1/tasks/"+strconv.Itoa(statusTaskID)+"/status", adminToken, map[string]any{
		"status": "processing",
	})
	if statusResp.StatusCode != http.StatusOK {
		t.Fatalf("patch status for tag automation expected 200 got %d %#v", statusResp.StatusCode, statusBody)
	}

	statusLogResp, statusLogBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/automation-rules/logs?ruleId="+strconv.Itoa(statusRuleID)+"&trigger=task_status_changed", adminToken, nil)
	if statusLogResp.StatusCode != http.StatusOK {
		t.Fatalf("list status tag automation logs expected 200 got %d", statusLogResp.StatusCode)
	}
	statusLogItems, _ := statusLogBody["list"].([]any)
	if len(statusLogItems) != 1 {
		t.Fatalf("status tag automation should create one event log got %#v", statusLogBody)
	}
	statusLogItem := statusLogItems[0].(map[string]any)
	if statusLogItem["status"] != "success" || statusLogItem["runSource"] != "event" || int(statusLogItem["matchedCount"].(float64)) != 1 || int(statusLogItem["actionCount"].(float64)) != 1 {
		t.Fatalf("status tag automation log should record one tag action got %#v", statusLogItem)
	}

	statusTaggedTaskResp, statusTaggedTaskBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/tasks?page=1&pageSize=20&tagIds="+strconv.Itoa(int(statusTagID)), adminToken, nil)
	if statusTaggedTaskResp.StatusCode != http.StatusOK {
		t.Fatalf("list status tagged tasks expected 200 got %d", statusTaggedTaskResp.StatusCode)
	}
	statusTaggedTasks, _ := statusTaggedTaskBody["list"].([]any)
	if len(statusTaggedTasks) != 1 || int(statusTaggedTasks[0].(map[string]any)["id"].(float64)) != statusTaskID {
		t.Fatalf("status tag automation should attach tag to task got %#v", statusTaggedTaskBody)
	}
}

func TestAutomationRuleAssignAssigneesAction(t *testing.T) {
	ts := setupTestRouter(t)
	defer ts.Close()

	adminToken := loginAndToken(t, ts.URL)
	codeToID := permissionCodeMap(t, ts.URL, adminToken)

	notificationRoleResp, notificationRoleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/rbac/roles", adminToken, map[string]any{
		"name":        "automation-assign-notification-reader",
		"description": "can read automation assign notifications",
		"permissionIds": []uint{
			codeToID["notifications.read"],
			codeToID["notifications.update"],
		},
	})
	if notificationRoleResp.StatusCode != http.StatusCreated {
		t.Fatalf("create automation assign notification role expected 201 got %d", notificationRoleResp.StatusCode)
	}
	notificationRoleID := uint(notificationRoleBody["id"].(float64))

	overdueAssigneeResp, overdueAssigneeBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":      "automation_assign_overdue",
		"name":          "Automation Assign Overdue",
		"email":         "automation_assign_overdue@example.com",
		"password":      "pass1234",
		"roleIds":       []uint{notificationRoleID},
		"departmentIds": []uint{},
	})
	if overdueAssigneeResp.StatusCode != http.StatusCreated {
		t.Fatalf("create overdue automation assignee expected 201 got %d", overdueAssigneeResp.StatusCode)
	}
	overdueAssigneeID := uint(overdueAssigneeBody["id"].(float64))

	statusAssigneeResp, statusAssigneeBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":      "automation_assign_status",
		"name":          "Automation Assign Status",
		"email":         "automation_assign_status@example.com",
		"password":      "pass1234",
		"roleIds":       []uint{notificationRoleID},
		"departmentIds": []uint{},
	})
	if statusAssigneeResp.StatusCode != http.StatusCreated {
		t.Fatalf("create status automation assignee expected 201 got %d", statusAssigneeResp.StatusCode)
	}
	statusAssigneeID := uint(statusAssigneeBody["id"].(float64))

	overdueAssigneeLoginStatus, overdueAssigneeLoginBody := loginWithCredentials(t, ts.URL, "automation_assign_overdue", "pass1234")
	if overdueAssigneeLoginStatus != http.StatusOK {
		t.Fatalf("login overdue automation assignee expected 200 got %d", overdueAssigneeLoginStatus)
	}
	overdueAssigneeToken := overdueAssigneeLoginBody["token"].(string)
	statusAssigneeLoginStatus, statusAssigneeLoginBody := loginWithCredentials(t, ts.URL, "automation_assign_status", "pass1234")
	if statusAssigneeLoginStatus != http.StatusOK {
		t.Fatalf("login status automation assignee expected 200 got %d", statusAssigneeLoginStatus)
	}
	statusAssigneeToken := statusAssigneeLoginBody["token"].(string)

	projectResp, projectBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/projects", adminToken, map[string]any{
		"code": "AUTO-ASSIGN-P1",
		"name": "Automation Assign Project",
	})
	if projectResp.StatusCode != http.StatusCreated {
		t.Fatalf("create automation assign project expected 201 got %d", projectResp.StatusCode)
	}
	projectID := int(projectBody["id"].(float64))

	overdueTaskResp, overdueTaskBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/tasks", adminToken, map[string]any{
		"title":     "Automation overdue assign task",
		"projectId": projectID,
		"status":    "processing",
		"progress":  30,
		"startAt":   "2000-01-01T00:00:00Z",
		"endAt":     "2000-01-02T00:00:00Z",
	})
	if overdueTaskResp.StatusCode != http.StatusCreated {
		t.Fatalf("create overdue assign task expected 201 got %d %#v", overdueTaskResp.StatusCode, overdueTaskBody)
	}
	overdueTaskID := int(overdueTaskBody["id"].(float64))

	invalidRuleResp, invalidRuleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/automation-rules", adminToken, map[string]any{
		"name":    "缺少指派动作目标",
		"trigger": "task_overdue",
		"actions": map[string]any{
			"notifyAssignees":     false,
			"notifyProjectOwners": false,
			"assignAssignees":     true,
			"assigneeIds":         []uint{},
		},
	})
	if invalidRuleResp.StatusCode != http.StatusBadRequest || invalidRuleBody["code"] != "INVALID_AUTOMATION_RULE" {
		t.Fatalf("missing assign action target expected 400 INVALID_AUTOMATION_RULE got %d %#v", invalidRuleResp.StatusCode, invalidRuleBody)
	}

	missingUserRuleResp, missingUserRuleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/automation-rules", adminToken, map[string]any{
		"name":    "不存在指派动作目标",
		"trigger": "task_overdue",
		"actions": map[string]any{
			"notifyAssignees":     false,
			"notifyProjectOwners": false,
			"assignAssignees":     true,
			"assigneeIds":         []uint{999999},
		},
	})
	if missingUserRuleResp.StatusCode != http.StatusBadRequest || missingUserRuleBody["code"] != "INVALID_AUTOMATION_RULE" {
		t.Fatalf("missing assign user expected 400 INVALID_AUTOMATION_RULE got %d %#v", missingUserRuleResp.StatusCode, missingUserRuleBody)
	}

	overdueRuleResp, overdueRuleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/automation-rules", adminToken, map[string]any{
		"name":      "逾期自动指派执行人",
		"trigger":   "task_overdue",
		"isEnabled": true,
		"conditions": map[string]any{
			"overdueDays": 1,
			"projectIds":  []uint{uint(projectID)},
		},
		"actions": map[string]any{
			"notifyAssignees":     false,
			"notifyProjectOwners": false,
			"assignAssignees":     true,
			"assigneeIds":         []uint{overdueAssigneeID},
		},
	})
	if overdueRuleResp.StatusCode != http.StatusCreated {
		t.Fatalf("create overdue assign automation rule expected 201 got %d %#v", overdueRuleResp.StatusCode, overdueRuleBody)
	}
	overdueRuleID := int(overdueRuleBody["id"].(float64))

	runResp, runBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/automation-rules/"+strconv.Itoa(overdueRuleID)+"/run", adminToken, nil)
	if runResp.StatusCode != http.StatusOK {
		t.Fatalf("run overdue assign automation expected 200 got %d %#v", runResp.StatusCode, runBody)
	}
	if runBody["status"] != "success" || int(runBody["matchedCount"].(float64)) != 1 || int(runBody["actionCount"].(float64)) != 1 {
		t.Fatalf("overdue assign automation should assign one user got %#v", runBody)
	}

	assignedTaskResp, assignedTaskBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/tasks?page=1&pageSize=20&assigneeIds="+strconv.Itoa(int(overdueAssigneeID)), adminToken, nil)
	if assignedTaskResp.StatusCode != http.StatusOK {
		t.Fatalf("list overdue assigned tasks expected 200 got %d", assignedTaskResp.StatusCode)
	}
	assignedTasks, _ := assignedTaskBody["list"].([]any)
	if len(assignedTasks) != 1 || int(assignedTasks[0].(map[string]any)["id"].(float64)) != overdueTaskID {
		t.Fatalf("overdue assign automation should attach assignee to task got %#v", assignedTaskBody)
	}

	overdueNotificationResp, overdueNotificationBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/notifications?module=tasks&keyword=自动化已将你设为任务", overdueAssigneeToken, nil)
	if overdueNotificationResp.StatusCode != http.StatusOK {
		t.Fatalf("overdue assignee notifications expected 200 got %d", overdueNotificationResp.StatusCode)
	}
	overdueNotifications, _ := overdueNotificationBody["list"].([]any)
	if len(overdueNotifications) == 0 {
		t.Fatalf("overdue assignee should receive assignment notification got %#v", overdueNotificationBody)
	}

	duplicateRunResp, duplicateRunBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/automation-rules/"+strconv.Itoa(overdueRuleID)+"/run", adminToken, nil)
	if duplicateRunResp.StatusCode != http.StatusOK {
		t.Fatalf("rerun overdue assign automation expected 200 got %d %#v", duplicateRunResp.StatusCode, duplicateRunBody)
	}
	if int(duplicateRunBody["actionCount"].(float64)) != 0 {
		t.Fatalf("rerun overdue assign automation should not duplicate assignee got %#v", duplicateRunBody)
	}

	statusTaskResp, statusTaskBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/tasks", adminToken, map[string]any{
		"title":     "Automation status assign task",
		"projectId": projectID,
		"status":    "pending",
		"progress":  10,
	})
	if statusTaskResp.StatusCode != http.StatusCreated {
		t.Fatalf("create status assign task expected 201 got %d %#v", statusTaskResp.StatusCode, statusTaskBody)
	}
	statusTaskID := int(statusTaskBody["id"].(float64))

	statusRuleResp, statusRuleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/automation-rules", adminToken, map[string]any{
		"name":      "状态变更自动指派执行人",
		"trigger":   "task_status_changed",
		"isEnabled": true,
		"conditions": map[string]any{
			"projectIds":   []uint{uint(projectID)},
			"fromStatuses": []string{"pending"},
			"toStatuses":   []string{"processing"},
		},
		"actions": map[string]any{
			"notifyAssignees":     false,
			"notifyProjectOwners": false,
			"assignAssignees":     true,
			"assigneeIds":         []uint{statusAssigneeID},
		},
	})
	if statusRuleResp.StatusCode != http.StatusCreated {
		t.Fatalf("create status assign automation rule expected 201 got %d %#v", statusRuleResp.StatusCode, statusRuleBody)
	}
	statusRuleID := int(statusRuleBody["id"].(float64))

	assigneeChangeRuleResp, assigneeChangeRuleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/automation-rules", adminToken, map[string]any{
		"name":      "自动指派不递归执行人变更",
		"trigger":   "task_assignee_changed",
		"isEnabled": true,
		"conditions": map[string]any{
			"projectIds":          []uint{uint(projectID)},
			"assigneeChangeTypes": []string{"added"},
		},
		"actions": map[string]any{
			"addComment": true,
		},
	})
	if assigneeChangeRuleResp.StatusCode != http.StatusCreated {
		t.Fatalf("create assignee change recursion guard rule expected 201 got %d %#v", assigneeChangeRuleResp.StatusCode, assigneeChangeRuleBody)
	}
	assigneeChangeRuleID := int(assigneeChangeRuleBody["id"].(float64))

	statusResp, statusBody := requestJSON(t, http.MethodPatch, ts.URL+"/api/v1/tasks/"+strconv.Itoa(statusTaskID)+"/status", adminToken, map[string]any{
		"status": "processing",
	})
	if statusResp.StatusCode != http.StatusOK {
		t.Fatalf("patch status for assign automation expected 200 got %d %#v", statusResp.StatusCode, statusBody)
	}

	statusLogResp, statusLogBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/automation-rules/logs?ruleId="+strconv.Itoa(statusRuleID)+"&trigger=task_status_changed", adminToken, nil)
	if statusLogResp.StatusCode != http.StatusOK {
		t.Fatalf("list status assign automation logs expected 200 got %d", statusLogResp.StatusCode)
	}
	statusLogItems, _ := statusLogBody["list"].([]any)
	if len(statusLogItems) != 1 {
		t.Fatalf("status assign automation should create one event log got %#v", statusLogBody)
	}
	statusLogItem := statusLogItems[0].(map[string]any)
	if statusLogItem["status"] != "success" || statusLogItem["runSource"] != "event" || int(statusLogItem["matchedCount"].(float64)) != 1 || int(statusLogItem["actionCount"].(float64)) != 1 {
		t.Fatalf("status assign automation log should record one assign action got %#v", statusLogItem)
	}

	statusAssignedTaskResp, statusAssignedTaskBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/tasks?page=1&pageSize=20&assigneeIds="+strconv.Itoa(int(statusAssigneeID)), adminToken, nil)
	if statusAssignedTaskResp.StatusCode != http.StatusOK {
		t.Fatalf("list status assigned tasks expected 200 got %d", statusAssignedTaskResp.StatusCode)
	}
	statusAssignedTasks, _ := statusAssignedTaskBody["list"].([]any)
	if len(statusAssignedTasks) != 1 || int(statusAssignedTasks[0].(map[string]any)["id"].(float64)) != statusTaskID {
		t.Fatalf("status assign automation should attach assignee to task got %#v", statusAssignedTaskBody)
	}

	statusNotificationResp, statusNotificationBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/notifications?module=tasks&keyword=自动化已将你设为任务", statusAssigneeToken, nil)
	if statusNotificationResp.StatusCode != http.StatusOK {
		t.Fatalf("status assignee notifications expected 200 got %d", statusNotificationResp.StatusCode)
	}
	statusNotifications, _ := statusNotificationBody["list"].([]any)
	if len(statusNotifications) == 0 {
		t.Fatalf("status assignee should receive assignment notification got %#v", statusNotificationBody)
	}

	assigneeChangeLogResp, assigneeChangeLogBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/automation-rules/logs?ruleId="+strconv.Itoa(assigneeChangeRuleID)+"&trigger=task_assignee_changed", adminToken, nil)
	if assigneeChangeLogResp.StatusCode != http.StatusOK {
		t.Fatalf("list assignee change recursion logs expected 200 got %d", assigneeChangeLogResp.StatusCode)
	}
	assigneeChangeLogItems, _ := assigneeChangeLogBody["list"].([]any)
	if len(assigneeChangeLogItems) != 0 {
		t.Fatalf("automation assignment should not recursively trigger assignee changed rules got %#v", assigneeChangeLogBody)
	}
}

func TestAutomationRuleWebhookAction(t *testing.T) {
	validationTS := setupTestRouter(t)
	defer validationTS.Close()
	validationAdminToken := loginAndToken(t, validationTS.URL)
	invalidPrivateResp, invalidPrivateBody := requestJSON(t, http.MethodPost, validationTS.URL+"/api/v1/automation-rules", validationAdminToken, map[string]any{
		"name":    "内网 Webhook 拦截",
		"trigger": "task_overdue",
		"actions": map[string]any{
			"notifyAssignees":     false,
			"notifyProjectOwners": false,
			"callWebhook":         true,
			"webhookUrl":          "http://127.0.0.1/webhook",
		},
	})
	if invalidPrivateResp.StatusCode != http.StatusBadRequest || invalidPrivateBody["code"] != "INVALID_AUTOMATION_RULE" {
		t.Fatalf("private webhook URL should be rejected got %d %#v", invalidPrivateResp.StatusCode, invalidPrivateBody)
	}

	payloads := make(chan map[string]any, 2)
	successWebhookTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("webhook method expected POST got %s", r.Method)
		}
		if contentType := r.Header.Get("Content-Type"); !strings.Contains(contentType, "application/json") {
			t.Errorf("webhook content type expected json got %s", contentType)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode webhook payload failed: %v", err)
		}
		payloads <- payload
		w.WriteHeader(http.StatusNoContent)
	}))
	defer successWebhookTS.Close()

	var failedWebhookCalls int32
	failedWebhookTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&failedWebhookCalls, 1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("downstream failed"))
	}))
	defer failedWebhookTS.Close()

	ts := setupTestRouterWithHandler(t, func(h *handler.Handler) {
		h.Cfg.WebhookPrivateOK = true
	})
	defer ts.Close()

	adminToken := loginAndToken(t, ts.URL)
	projectResp, projectBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/projects", adminToken, map[string]any{
		"code": "AUTO-WEBHOOK-P1",
		"name": "Automation Webhook Project",
	})
	if projectResp.StatusCode != http.StatusCreated {
		t.Fatalf("create webhook automation project expected 201 got %d %#v", projectResp.StatusCode, projectBody)
	}
	projectID := int(projectBody["id"].(float64))

	overdueTaskResp, overdueTaskBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/tasks", adminToken, map[string]any{
		"title":     "Automation overdue webhook task",
		"projectId": projectID,
		"status":    "processing",
		"progress":  20,
		"startAt":   "2000-01-01T00:00:00Z",
		"endAt":     "2000-01-02T00:00:00Z",
	})
	if overdueTaskResp.StatusCode != http.StatusCreated {
		t.Fatalf("create overdue webhook task expected 201 got %d %#v", overdueTaskResp.StatusCode, overdueTaskBody)
	}

	overdueRuleResp, overdueRuleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/automation-rules", adminToken, map[string]any{
		"name":      "逾期调用 Webhook",
		"trigger":   "task_overdue",
		"isEnabled": true,
		"conditions": map[string]any{
			"overdueDays": 1,
			"projectIds":  []uint{uint(projectID)},
		},
		"actions": map[string]any{
			"notifyAssignees":     false,
			"notifyProjectOwners": false,
			"callWebhook":         true,
			"webhookUrl":          successWebhookTS.URL,
		},
	})
	if overdueRuleResp.StatusCode != http.StatusCreated {
		t.Fatalf("create overdue webhook rule expected 201 got %d %#v", overdueRuleResp.StatusCode, overdueRuleBody)
	}
	overdueRuleID := int(overdueRuleBody["id"].(float64))

	runResp, runBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/automation-rules/"+strconv.Itoa(overdueRuleID)+"/run", adminToken, nil)
	if runResp.StatusCode != http.StatusOK {
		t.Fatalf("run overdue webhook rule expected 200 got %d %#v", runResp.StatusCode, runBody)
	}
	if runBody["status"] != "success" || int(runBody["matchedCount"].(float64)) != 1 || int(runBody["actionCount"].(float64)) != 1 {
		t.Fatalf("overdue webhook run should count one successful webhook got %#v", runBody)
	}
	select {
	case payload := <-payloads:
		if payload["event"] != "task_overdue" || payload["runSource"] != "manual" {
			t.Fatalf("unexpected overdue webhook payload metadata %#v", payload)
		}
		taskPayload, _ := payload["task"].(map[string]any)
		if taskPayload["title"] != "Automation overdue webhook task" {
			t.Fatalf("webhook payload should include task data got %#v", payload)
		}
	default:
		t.Fatalf("expected overdue webhook request")
	}

	statusTaskResp, statusTaskBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/tasks", adminToken, map[string]any{
		"title":     "Automation failed webhook status task",
		"projectId": projectID,
		"status":    "pending",
		"progress":  10,
	})
	if statusTaskResp.StatusCode != http.StatusCreated {
		t.Fatalf("create status webhook task expected 201 got %d %#v", statusTaskResp.StatusCode, statusTaskBody)
	}
	statusTaskID := int(statusTaskBody["id"].(float64))

	statusRuleResp, statusRuleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/automation-rules", adminToken, map[string]any{
		"name":      "状态变更 Webhook 失败不回滚",
		"trigger":   "task_status_changed",
		"isEnabled": true,
		"conditions": map[string]any{
			"projectIds":   []uint{uint(projectID)},
			"fromStatuses": []string{"pending"},
			"toStatuses":   []string{"processing"},
		},
		"actions": map[string]any{
			"notifyAssignees":     false,
			"notifyProjectOwners": false,
			"callWebhook":         true,
			"webhookUrl":          failedWebhookTS.URL,
		},
	})
	if statusRuleResp.StatusCode != http.StatusCreated {
		t.Fatalf("create status failed webhook rule expected 201 got %d %#v", statusRuleResp.StatusCode, statusRuleBody)
	}
	statusRuleID := int(statusRuleBody["id"].(float64))

	statusResp, statusBody := requestJSON(t, http.MethodPatch, ts.URL+"/api/v1/tasks/"+strconv.Itoa(statusTaskID)+"/status", adminToken, map[string]any{
		"status": "processing",
	})
	if statusResp.StatusCode != http.StatusOK {
		t.Fatalf("patch status with failed webhook expected 200 got %d %#v", statusResp.StatusCode, statusBody)
	}
	if statusBody["status"] != "processing" {
		t.Fatalf("failed webhook should not roll back task status got %#v", statusBody)
	}
	if atomic.LoadInt32(&failedWebhookCalls) != 1 {
		t.Fatalf("failed webhook should be called once got %d", failedWebhookCalls)
	}

	statusLogResp, statusLogBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/automation-rules/logs?ruleId="+strconv.Itoa(statusRuleID)+"&trigger=task_status_changed", adminToken, nil)
	if statusLogResp.StatusCode != http.StatusOK {
		t.Fatalf("list failed webhook logs expected 200 got %d", statusLogResp.StatusCode)
	}
	statusLogItems, _ := statusLogBody["list"].([]any)
	if len(statusLogItems) != 1 {
		t.Fatalf("failed webhook should create one event log got %#v", statusLogBody)
	}
	statusLogItem := statusLogItems[0].(map[string]any)
	message, _ := statusLogItem["message"].(string)
	if statusLogItem["status"] != "failed" || statusLogItem["runSource"] != "event" || int(statusLogItem["matchedCount"].(float64)) != 1 || int(statusLogItem["actionCount"].(float64)) != 0 || !strings.Contains(message, "Webhook 调用成功 0 次，失败 1 次") {
		t.Fatalf("failed webhook log should record non-blocking failure got %#v", statusLogItem)
	}
}

func TestMyTasksReturnsEmptyArrays(t *testing.T) {
	ts := setupTestRouter(t)
	defer ts.Close()

	token := loginAndToken(t, ts.URL)

	resp, body := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/tasks/me", token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("my tasks status expected 200 got %d", resp.StatusCode)
	}
	if _, ok := body["myTasks"].([]any); !ok {
		t.Fatalf("myTasks should be an array, got %T", body["myTasks"])
	}
	if _, ok := body["myCreated"].([]any); !ok {
		t.Fatalf("myCreated should be an array, got %T", body["myCreated"])
	}
	if _, ok := body["myParticipate"].([]any); !ok {
		t.Fatalf("myParticipate should be an array, got %T", body["myParticipate"])
	}
}

func TestMyTasksWithParticipatedTask(t *testing.T) {
	ts := setupTestRouter(t)
	defer ts.Close()

	adminToken := loginAndToken(t, ts.URL)
	codeToID := permissionCodeMap(t, ts.URL, adminToken)
	roleResp, roleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/rbac/roles", adminToken, map[string]any{
		"name": "mytasks-reader",
		"permissionIds": []uint{
			codeToID["tasks.read"],
			codeToID["projects.read"],
		},
	})
	if roleResp.StatusCode != http.StatusCreated {
		t.Fatalf("create mytasks role expected 201 got %d", roleResp.StatusCode)
	}
	roleID := uint(roleBody["id"].(float64))

	userResp, userBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":      "mytasks_u1",
		"name":          "MyTasks User",
		"email":         "mytasks_u1@example.com",
		"password":      "pass1234",
		"roleIds":       []uint{roleID},
		"departmentIds": []uint{},
	})
	if userResp.StatusCode != http.StatusCreated {
		t.Fatalf("create mytasks user expected 201 got %d", userResp.StatusCode)
	}
	userID := uint(userBody["id"].(float64))

	projectResp, projectBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/projects", adminToken, map[string]any{
		"code":        "MYTASKS-P1",
		"name":        "MyTasks Project",
		"description": "project for mytasks",
	})
	if projectResp.StatusCode != http.StatusCreated {
		t.Fatalf("create mytasks project expected 201 got %d", projectResp.StatusCode)
	}
	projectID := int(projectBody["id"].(float64))

	taskResp, _ := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/tasks", adminToken, map[string]any{
		"title":       "Participated Task",
		"projectId":   projectID,
		"status":      "pending",
		"progress":    0,
		"assigneeIds": []uint{userID},
	})
	if taskResp.StatusCode != http.StatusCreated {
		t.Fatalf("create participated task expected 201 got %d", taskResp.StatusCode)
	}

	loginStatus, loginBody := loginWithCredentials(t, ts.URL, "mytasks_u1", "pass1234")
	if loginStatus != http.StatusOK {
		t.Fatalf("login mytasks user expected 200 got %d", loginStatus)
	}
	userToken, _ := loginBody["token"].(string)
	if userToken == "" {
		t.Fatalf("mytasks user token should not be empty")
	}

	resp, body := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/tasks/me", userToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("my tasks status expected 200 got %d", resp.StatusCode)
	}
	participated, ok := body["myParticipate"].([]any)
	if !ok {
		t.Fatalf("myParticipate should be an array, got %T", body["myParticipate"])
	}
	if len(participated) == 0 {
		t.Fatalf("myParticipate should contain assigned task")
	}
}

func TestTaskCreateRollbackOnFailpoint(t *testing.T) {
	ts := setupTestRouterWithHandler(t, func(h *handler.Handler) {
		h.TxFailpoint = func(point string) error {
			if point == "tasks.create.after_assignees" {
				return errors.New("failpoint: tasks create")
			}
			return nil
		}
	})
	defer ts.Close()

	token := loginAndToken(t, ts.URL)

	projectResp, projectBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/projects", token, map[string]any{
		"code": "ROLLBACK-TASK-P1", "name": "Rollback Task Project", "description": "d",
	})
	if projectResp.StatusCode != http.StatusCreated {
		t.Fatalf("create project status expected 201 got %d", projectResp.StatusCode)
	}
	projectID := int(projectBody["id"].(float64))

	taskResp, _ := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/tasks", token, map[string]any{
		"title":       "Rollback Task",
		"projectId":   projectID,
		"status":      "pending",
		"progress":    10,
		"startAt":     "2026-03-24T10:00:00Z",
		"endAt":       "2026-03-25T10:00:00Z",
		"assigneeIds": []uint{1},
	})
	if taskResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("task create with failpoint expected 400 got %d", taskResp.StatusCode)
	}

	listResp, listBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/tasks?page=1&pageSize=20&projectId="+strconv.Itoa(projectID), token, nil)
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("query tasks status expected 200 got %d", listResp.StatusCode)
	}
	total := int(listBody["total"].(float64))
	if total != 0 {
		t.Fatalf("expected task rollback, total should be 0 got %d", total)
	}
}

func TestProjectUpdateRollbackOnFailpoint(t *testing.T) {
	ts := setupTestRouterWithHandler(t, func(h *handler.Handler) {
		h.TxFailpoint = func(point string) error {
			if point == "projects.update.after_relations" {
				return errors.New("failpoint: projects update")
			}
			return nil
		}
	})
	defer ts.Close()

	token := loginAndToken(t, ts.URL)
	projectResp, projectBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/projects", token, map[string]any{
		"code": "ROLLBACK-PROJECT-P1", "name": "Old Name", "description": "old",
	})
	if projectResp.StatusCode != http.StatusCreated {
		t.Fatalf("create project status expected 201 got %d", projectResp.StatusCode)
	}
	projectID := int(projectBody["id"].(float64))

	updateResp, _ := requestJSON(t, http.MethodPut, ts.URL+"/api/v1/projects/"+strconv.Itoa(projectID), token, map[string]any{
		"code":          "ROLLBACK-PROJECT-P1",
		"name":          "New Name Should Rollback",
		"description":   "new",
		"userIds":       []uint{1},
		"departmentIds": []uint{},
	})
	if updateResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("update project with failpoint expected 400 got %d", updateResp.StatusCode)
	}

	detailResp, detailBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/projects/"+strconv.Itoa(projectID), token, nil)
	if detailResp.StatusCode != http.StatusOK {
		t.Fatalf("project detail status expected 200 got %d", detailResp.StatusCode)
	}
	name, _ := detailBody["name"].(string)
	if name != "Old Name" {
		t.Fatalf("expected rollback project name old value, got %s", name)
	}
}

func TestRbacCreateRoleRollbackOnFailpoint(t *testing.T) {
	ts := setupTestRouterWithHandler(t, func(h *handler.Handler) {
		h.TxFailpoint = func(point string) error {
			if point == "rbac.create_role.after_permissions" {
				return errors.New("failpoint: rbac create role")
			}
			return nil
		}
	})
	defer ts.Close()

	token := loginAndToken(t, ts.URL)

	roleName := "rollback-role"
	createResp, _ := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/rbac/roles", token, map[string]any{
		"name":          roleName,
		"description":   "rollback test",
		"permissionIds": []uint{1},
	})
	if createResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("create role with failpoint expected 400 got %d", createResp.StatusCode)
	}
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/system/rbac/roles", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rawResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request roles failed: %v", err)
	}
	defer rawResp.Body.Close()
	var roles []map[string]any
	_ = json.NewDecoder(rawResp.Body).Decode(&roles)
	for _, role := range roles {
		name, _ := role["name"].(string)
		if name == roleName {
			t.Fatalf("role should rollback and not be persisted")
		}
	}
}

func TestRbacCreatePermissionRollbackOnFailpoint(t *testing.T) {
	ts := setupTestRouterWithHandler(t, func(h *handler.Handler) {
		h.TxFailpoint = func(point string) error {
			if point == "rbac.create_permission.after_create" {
				return errors.New("failpoint: rbac create permission")
			}
			return nil
		}
	})
	defer ts.Close()

	token := loginAndToken(t, ts.URL)
	code := "rollback.permission"
	createResp, _ := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/rbac/permissions", token, map[string]any{
		"code": code,
		"name": "Rollback Permission",
	})
	if createResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("create permission with failpoint expected 400 got %d", createResp.StatusCode)
	}

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/system/rbac/permissions", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("query permissions failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("query permissions status expected 200 got %d", resp.StatusCode)
	}
	var permissions []map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&permissions)
	for _, permission := range permissions {
		permissionCode, _ := permission["code"].(string)
		if permissionCode == code {
			t.Fatalf("permission should rollback and not be persisted")
		}
	}
}

func TestScopedUserCannotMutateInvisibleProjectAndTask(t *testing.T) {
	ts := setupTestRouter(t)
	defer ts.Close()

	adminToken := loginAndToken(t, ts.URL)
	codeToID := permissionCodeMap(t, ts.URL, adminToken)

	roleResp, roleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/rbac/roles", adminToken, map[string]any{
		"name":        "scope-editor",
		"description": "scope editor",
		"permissionIds": []uint{
			codeToID["projects.read"],
			codeToID["projects.update"],
			codeToID["projects.delete"],
			codeToID["tasks.read"],
			codeToID["tasks.create"],
			codeToID["tasks.update"],
			codeToID["tasks.delete"],
		},
	})
	if roleResp.StatusCode != http.StatusCreated {
		t.Fatalf("create role status expected 201 got %d", roleResp.StatusCode)
	}
	roleID := uint(roleBody["id"].(float64))

	userResp, userBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":      "scope_editor_u1",
		"name":          "Scope Editor",
		"email":         "scope_editor_u1@example.com",
		"password":      "pass1234",
		"roleIds":       []uint{roleID},
		"departmentIds": []uint{},
	})
	if userResp.StatusCode != http.StatusCreated {
		t.Fatalf("create user status expected 201 got %d", userResp.StatusCode)
	}
	userID := uint(userBody["id"].(float64))

	visibleProjectResp, visibleProjectBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/projects", adminToken, map[string]any{
		"code":          "SCOPE-EDIT-P1",
		"name":          "Visible Project",
		"description":   "visible",
		"userIds":       []uint{userID},
		"departmentIds": []uint{},
	})
	if visibleProjectResp.StatusCode != http.StatusCreated {
		t.Fatalf("create visible project status expected 201 got %d", visibleProjectResp.StatusCode)
	}
	visibleProjectID := int(visibleProjectBody["id"].(float64))

	hiddenProjectResp, hiddenProjectBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/projects", adminToken, map[string]any{
		"code":        "SCOPE-EDIT-P2",
		"name":        "Hidden Project",
		"description": "hidden",
	})
	if hiddenProjectResp.StatusCode != http.StatusCreated {
		t.Fatalf("create hidden project status expected 201 got %d", hiddenProjectResp.StatusCode)
	}
	hiddenProjectID := int(hiddenProjectBody["id"].(float64))

	hiddenProjectNoTaskResp, hiddenProjectNoTaskBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/projects", adminToken, map[string]any{
		"code":        "SCOPE-EDIT-P3",
		"name":        "Hidden Project No Task",
		"description": "hidden no task",
	})
	if hiddenProjectNoTaskResp.StatusCode != http.StatusCreated {
		t.Fatalf("create hidden no-task project status expected 201 got %d", hiddenProjectNoTaskResp.StatusCode)
	}
	hiddenProjectNoTaskID := int(hiddenProjectNoTaskBody["id"].(float64))

	visibleTaskResp, visibleTaskBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/tasks", adminToken, map[string]any{
		"title":     "Visible Task",
		"projectId": visibleProjectID,
		"status":    "pending",
		"progress":  0,
	})
	if visibleTaskResp.StatusCode != http.StatusCreated {
		t.Fatalf("create visible task status expected 201 got %d", visibleTaskResp.StatusCode)
	}
	visibleTaskID := int(visibleTaskBody["id"].(float64))

	hiddenTaskResp, hiddenTaskBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/tasks", adminToken, map[string]any{
		"title":     "Hidden Task",
		"projectId": hiddenProjectID,
		"status":    "pending",
		"progress":  0,
	})
	if hiddenTaskResp.StatusCode != http.StatusCreated {
		t.Fatalf("create hidden task status expected 201 got %d", hiddenTaskResp.StatusCode)
	}
	hiddenTaskID := int(hiddenTaskBody["id"].(float64))

	loginStatus, loginBody := loginWithCredentials(t, ts.URL, "scope_editor_u1", "pass1234")
	if loginStatus != http.StatusOK {
		t.Fatalf("login scoped editor expected 200 got %d", loginStatus)
	}
	editorToken, _ := loginBody["token"].(string)
	if editorToken == "" {
		t.Fatalf("scoped editor token should not be empty")
	}

	updateVisibleProjectResp, _ := requestJSON(t, http.MethodPut, ts.URL+"/api/v1/projects/"+strconv.Itoa(visibleProjectID), editorToken, map[string]any{
		"code":          "SCOPE-EDIT-P1",
		"name":          "Visible Project Updated",
		"description":   "updated",
		"userIds":       []uint{userID},
		"departmentIds": []uint{},
	})
	if updateVisibleProjectResp.StatusCode != http.StatusOK {
		t.Fatalf("update visible project expected 200 got %d", updateVisibleProjectResp.StatusCode)
	}

	updateHiddenProjectResp, _ := requestJSON(t, http.MethodPut, ts.URL+"/api/v1/projects/"+strconv.Itoa(hiddenProjectID), editorToken, map[string]any{
		"code":          "SCOPE-EDIT-P2",
		"name":          "Hidden Project Updated",
		"description":   "should fail",
		"userIds":       []uint{},
		"departmentIds": []uint{},
	})
	if updateHiddenProjectResp.StatusCode != http.StatusNotFound {
		t.Fatalf("update hidden project expected 404 got %d", updateHiddenProjectResp.StatusCode)
	}

	deleteHiddenProjectResp, _ := requestJSON(t, http.MethodDelete, ts.URL+"/api/v1/projects/"+strconv.Itoa(hiddenProjectNoTaskID), editorToken, nil)
	if deleteHiddenProjectResp.StatusCode != http.StatusNotFound {
		t.Fatalf("delete hidden project expected 404 got %d", deleteHiddenProjectResp.StatusCode)
	}

	updateVisibleTaskResp, _ := requestJSON(t, http.MethodPut, ts.URL+"/api/v1/tasks/"+strconv.Itoa(visibleTaskID), editorToken, map[string]any{
		"title":     "Visible Task Updated",
		"projectId": visibleProjectID,
		"status":    "processing",
		"progress":  30,
	})
	if updateVisibleTaskResp.StatusCode != http.StatusOK {
		t.Fatalf("update visible task expected 200 got %d", updateVisibleTaskResp.StatusCode)
	}

	updateHiddenTaskResp, _ := requestJSON(t, http.MethodPut, ts.URL+"/api/v1/tasks/"+strconv.Itoa(hiddenTaskID), editorToken, map[string]any{
		"title":     "Hidden Task Updated",
		"projectId": hiddenProjectID,
		"status":    "processing",
		"progress":  30,
	})
	if updateHiddenTaskResp.StatusCode != http.StatusNotFound {
		t.Fatalf("update hidden task expected 404 got %d", updateHiddenTaskResp.StatusCode)
	}

	deleteHiddenTaskResp, _ := requestJSON(t, http.MethodDelete, ts.URL+"/api/v1/tasks/"+strconv.Itoa(hiddenTaskID), editorToken, nil)
	if deleteHiddenTaskResp.StatusCode != http.StatusNotFound {
		t.Fatalf("delete hidden task expected 404 got %d", deleteHiddenTaskResp.StatusCode)
	}

	createHiddenTaskResp, _ := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/tasks", editorToken, map[string]any{
		"title":     "Create On Hidden Project",
		"projectId": hiddenProjectID,
		"status":    "pending",
		"progress":  0,
	})
	if createHiddenTaskResp.StatusCode != http.StatusNotFound {
		t.Fatalf("create task on hidden project expected 404 got %d", createHiddenTaskResp.StatusCode)
	}
}

func TestUserWeeklyCapacityCreateUpdateValidation(t *testing.T) {
	ts := setupTestRouter(t)
	defer ts.Close()

	adminToken := loginAndToken(t, ts.URL)
	userResp, userBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":            "capacity_u1",
		"name":                "Capacity User",
		"email":               "capacity_u1@example.com",
		"password":            "pass1234",
		"weeklyCapacityHours": 32.5,
		"roleIds":             []uint{},
		"departmentIds":       []uint{},
	})
	if userResp.StatusCode != http.StatusCreated {
		t.Fatalf("create capacity user expected 201 got %d, body=%v", userResp.StatusCode, userBody)
	}
	userID := int(userBody["id"].(float64))
	if userBody["weeklyCapacityHours"].(float64) != 32.5 {
		t.Fatalf("weekly capacity not saved on create: %v", userBody["weeklyCapacityHours"])
	}

	zeroCapacityResp, zeroCapacityBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":            "capacity_zero",
		"name":                "Capacity Zero",
		"email":               "capacity_zero@example.com",
		"password":            "pass1234",
		"weeklyCapacityHours": 0,
		"roleIds":             []uint{},
		"departmentIds":       []uint{},
	})
	if zeroCapacityResp.StatusCode != http.StatusCreated {
		t.Fatalf("create zero capacity user expected 201 got %d, body=%v", zeroCapacityResp.StatusCode, zeroCapacityBody)
	}
	if zeroCapacityBody["weeklyCapacityHours"].(float64) != 0 {
		t.Fatalf("zero weekly capacity should be saved as 0 got %v", zeroCapacityBody["weeklyCapacityHours"])
	}

	invalidCreateResp, invalidCreateBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":            "capacity_invalid",
		"name":                "Capacity Invalid",
		"email":               "capacity_invalid@example.com",
		"password":            "pass1234",
		"weeklyCapacityHours": 169,
		"roleIds":             []uint{},
		"departmentIds":       []uint{},
	})
	if invalidCreateResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid create capacity expected 400 got %d", invalidCreateResp.StatusCode)
	}
	if invalidCreateBody["code"] != "INVALID_WEEKLY_CAPACITY" {
		t.Fatalf("invalid create capacity expected INVALID_WEEKLY_CAPACITY got %v", invalidCreateBody["code"])
	}

	updateResp, updateBody := requestJSON(t, http.MethodPut, ts.URL+"/api/v1/system/users/"+strconv.Itoa(userID), adminToken, map[string]any{
		"name":                "Capacity User Updated",
		"email":               "capacity_u1@example.com",
		"weeklyCapacityHours": 28.5,
		"isActive":            true,
		"roleIds":             []uint{},
		"departmentIds":       []uint{},
	})
	if updateResp.StatusCode != http.StatusOK {
		t.Fatalf("update capacity user expected 200 got %d, body=%v", updateResp.StatusCode, updateBody)
	}
	if updateBody["weeklyCapacityHours"].(float64) != 28.5 {
		t.Fatalf("weekly capacity not saved on update: %v", updateBody["weeklyCapacityHours"])
	}

	invalidUpdateResp, invalidUpdateBody := requestJSON(t, http.MethodPut, ts.URL+"/api/v1/system/users/"+strconv.Itoa(userID), adminToken, map[string]any{
		"name":                "Capacity User Updated",
		"email":               "capacity_u1@example.com",
		"weeklyCapacityHours": -0.5,
		"isActive":            true,
		"roleIds":             []uint{},
		"departmentIds":       []uint{},
	})
	if invalidUpdateResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid update capacity expected 400 got %d", invalidUpdateResp.StatusCode)
	}
	if invalidUpdateBody["code"] != "INVALID_WEEKLY_CAPACITY" {
		t.Fatalf("invalid update capacity expected INVALID_WEEKLY_CAPACITY got %v", invalidUpdateBody["code"])
	}
}

func TestDisabledUserCannotLogin(t *testing.T) {
	ts := setupTestRouter(t)
	defer ts.Close()

	adminToken := loginAndToken(t, ts.URL)
	userResp, userBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":      "disabled_login_u1",
		"name":          "Disabled Login",
		"email":         "disabled_login_u1@example.com",
		"password":      "pass1234",
		"roleIds":       []uint{},
		"departmentIds": []uint{},
	})
	if userResp.StatusCode != http.StatusCreated {
		t.Fatalf("create user status expected 201 got %d", userResp.StatusCode)
	}
	userID := int(userBody["id"].(float64))

	disableResp, _ := requestJSON(t, http.MethodPut, ts.URL+"/api/v1/system/users/"+strconv.Itoa(userID), adminToken, map[string]any{
		"name":          "Disabled Login",
		"email":         "disabled_login_u1@example.com",
		"isActive":      false,
		"roleIds":       []uint{},
		"departmentIds": []uint{},
	})
	if disableResp.StatusCode != http.StatusOK {
		t.Fatalf("disable user status expected 200 got %d", disableResp.StatusCode)
	}

	loginStatus, loginBody := loginWithCredentials(t, ts.URL, "disabled_login_u1", "pass1234")
	if loginStatus != http.StatusForbidden {
		t.Fatalf("disabled user login expected 403 got %d", loginStatus)
	}
	if loginBody["code"] != "USER_DISABLED" {
		t.Fatalf("disabled user login expected USER_DISABLED code got %v", loginBody["code"])
	}
}

func TestTaskCommentsMentionsActivitiesAndScope(t *testing.T) {
	ts := setupTestRouter(t)
	defer ts.Close()

	adminToken := loginAndToken(t, ts.URL)
	codeToID := permissionCodeMap(t, ts.URL, adminToken)

	roleResp, roleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/rbac/roles", adminToken, map[string]any{
		"name":        "comment-collaborator",
		"description": "comment collaborator",
		"permissionIds": []uint{
			codeToID["projects.read"],
			codeToID["tasks.read"],
			codeToID["comments.read"],
			codeToID["comments.create"],
			codeToID["comments.delete"],
			codeToID["notifications.read"],
			codeToID["notifications.update"],
		},
	})
	if roleResp.StatusCode != http.StatusCreated {
		t.Fatalf("create comment role expected 201 got %d", roleResp.StatusCode)
	}
	roleID := uint(roleBody["id"].(float64))

	authorResp, authorBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":      "comment_author",
		"name":          "Comment Author",
		"email":         "comment_author@example.com",
		"password":      "pass1234",
		"roleIds":       []uint{roleID},
		"departmentIds": []uint{},
	})
	if authorResp.StatusCode != http.StatusCreated {
		t.Fatalf("create author expected 201 got %d", authorResp.StatusCode)
	}
	authorID := uint(authorBody["id"].(float64))

	mentionedResp, mentionedBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":      "comment_mentioned",
		"name":          "Comment Mentioned",
		"email":         "comment_mentioned@example.com",
		"password":      "pass1234",
		"roleIds":       []uint{roleID},
		"departmentIds": []uint{},
	})
	if mentionedResp.StatusCode != http.StatusCreated {
		t.Fatalf("create mentioned expected 201 got %d", mentionedResp.StatusCode)
	}
	mentionedID := uint(mentionedBody["id"].(float64))

	outsiderResp, _ := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/system/users", adminToken, map[string]any{
		"username":      "comment_outsider",
		"name":          "Comment Outsider",
		"email":         "comment_outsider@example.com",
		"password":      "pass1234",
		"roleIds":       []uint{roleID},
		"departmentIds": []uint{},
	})
	if outsiderResp.StatusCode != http.StatusCreated {
		t.Fatalf("create outsider expected 201 got %d", outsiderResp.StatusCode)
	}

	projectResp, projectBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/projects", adminToken, map[string]any{
		"code":        "COMMENT-P1",
		"name":        "Comment Project",
		"description": "comment project",
	})
	if projectResp.StatusCode != http.StatusCreated {
		t.Fatalf("create project expected 201 got %d", projectResp.StatusCode)
	}
	projectID := int(projectBody["id"].(float64))

	taskResp, taskBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/tasks", adminToken, map[string]any{
		"title":       "Comment Task",
		"projectId":   projectID,
		"status":      "pending",
		"progress":    0,
		"assigneeIds": []uint{authorID, mentionedID},
	})
	if taskResp.StatusCode != http.StatusCreated {
		t.Fatalf("create task expected 201 got %d", taskResp.StatusCode)
	}
	taskID := int(taskBody["id"].(float64))

	authorStatus, authorLogin := loginWithCredentials(t, ts.URL, "comment_author", "pass1234")
	if authorStatus != http.StatusOK {
		t.Fatalf("login author expected 200 got %d", authorStatus)
	}
	authorToken := authorLogin["token"].(string)

	mentionedStatus, mentionedLogin := loginWithCredentials(t, ts.URL, "comment_mentioned", "pass1234")
	if mentionedStatus != http.StatusOK {
		t.Fatalf("login mentioned expected 200 got %d", mentionedStatus)
	}
	mentionedToken := mentionedLogin["token"].(string)

	outsiderStatus, outsiderLogin := loginWithCredentials(t, ts.URL, "comment_outsider", "pass1234")
	if outsiderStatus != http.StatusOK {
		t.Fatalf("login outsider expected 200 got %d", outsiderStatus)
	}
	outsiderToken := outsiderLogin["token"].(string)

	commentResp, commentBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/tasks/"+strconv.Itoa(taskID)+"/comments", authorToken, map[string]any{
		"content": "请 @comment_mentioned 看一下这个风险点",
	})
	if commentResp.StatusCode != http.StatusCreated {
		t.Fatalf("create comment expected 201 got %d body=%v", commentResp.StatusCode, commentBody)
	}
	commentID := int(commentBody["id"].(float64))
	mentions, _ := commentBody["mentions"].([]any)
	if len(mentions) != 1 {
		t.Fatalf("expected one mention in comment body got %v", commentBody["mentions"])
	}

	commentListResp, commentListBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/tasks/"+strconv.Itoa(taskID)+"/comments", authorToken, nil)
	if commentListResp.StatusCode != http.StatusOK {
		t.Fatalf("list comments expected 200 got %d", commentListResp.StatusCode)
	}
	commentList, _ := commentListBody["list"].([]any)
	if len(commentList) != 1 {
		t.Fatalf("expected one comment got %v", commentListBody)
	}

	activityResp, activityBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/tasks/"+strconv.Itoa(taskID)+"/activities", authorToken, nil)
	if activityResp.StatusCode != http.StatusOK {
		t.Fatalf("list activities expected 200 got %d", activityResp.StatusCode)
	}
	activities, _ := activityBody["list"].([]any)
	foundCommentActivity := false
	for _, raw := range activities {
		item, _ := raw.(map[string]any)
		if item["type"] == "comment.created" {
			foundCommentActivity = true
		}
	}
	if !foundCommentActivity {
		t.Fatalf("expected comment.created activity got %v", activityBody)
	}

	notificationResp, notificationBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/notifications?module=tasks", mentionedToken, nil)
	if notificationResp.StatusCode != http.StatusOK {
		t.Fatalf("mentioned notifications expected 200 got %d", notificationResp.StatusCode)
	}
	notificationList, _ := notificationBody["list"].([]any)
	foundMentionNotification := false
	for _, raw := range notificationList {
		item, _ := raw.(map[string]any)
		if item["targetId"] == float64(taskID) && strings.Contains(item["title"].(string), "提及") {
			foundMentionNotification = true
		}
	}
	if !foundMentionNotification {
		t.Fatalf("expected mention notification got %v", notificationBody)
	}

	outsiderListResp, _ := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/tasks/"+strconv.Itoa(taskID)+"/comments", outsiderToken, nil)
	if outsiderListResp.StatusCode != http.StatusNotFound {
		t.Fatalf("outsider list comments expected 404 got %d", outsiderListResp.StatusCode)
	}

	deleteByMentionedResp, _ := requestJSON(t, http.MethodDelete, ts.URL+"/api/v1/tasks/"+strconv.Itoa(taskID)+"/comments/"+strconv.Itoa(commentID), mentionedToken, nil)
	if deleteByMentionedResp.StatusCode != http.StatusForbidden {
		t.Fatalf("non-author delete expected 403 got %d", deleteByMentionedResp.StatusCode)
	}

	deleteByAuthorResp, _ := requestJSON(t, http.MethodDelete, ts.URL+"/api/v1/tasks/"+strconv.Itoa(taskID)+"/comments/"+strconv.Itoa(commentID), authorToken, nil)
	if deleteByAuthorResp.StatusCode != http.StatusOK {
		t.Fatalf("author delete expected 200 got %d", deleteByAuthorResp.StatusCode)
	}

	commentListAfterDeleteResp, commentListAfterDeleteBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/tasks/"+strconv.Itoa(taskID)+"/comments", authorToken, nil)
	if commentListAfterDeleteResp.StatusCode != http.StatusOK {
		t.Fatalf("list after delete expected 200 got %d", commentListAfterDeleteResp.StatusCode)
	}
	commentListAfterDelete, _ := commentListAfterDeleteBody["list"].([]any)
	if len(commentListAfterDelete) != 0 {
		t.Fatalf("deleted comment should be hidden got %v", commentListAfterDeleteBody)
	}
}

func TestExternalPortalInviteScopeRequestsCommentsAndAudit(t *testing.T) {
	ts := setupTestRouter(t)
	defer ts.Close()

	adminToken := loginAndToken(t, ts.URL)
	codeToID := permissionCodeMap(t, ts.URL, adminToken)
	for _, code := range []string{"portal.create", "portal.read", "portal.update", "portal.delete"} {
		if codeToID[code] == 0 {
			t.Fatalf("permission seed missing: %s", code)
		}
	}

	projectResp, projectBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/projects", adminToken, map[string]any{
		"code":        "PORTAL-P1",
		"name":        "Portal Project",
		"description": "portal scoped project",
	})
	if projectResp.StatusCode != http.StatusCreated {
		t.Fatalf("create portal project expected 201 got %d, body=%v", projectResp.StatusCode, projectBody)
	}
	projectID := int(projectBody["id"].(float64))

	visibleTaskResp, visibleTaskBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/tasks", adminToken, map[string]any{
		"title":           "Visible Portal Task",
		"description":     "customer can read this",
		"projectId":       projectID,
		"status":          "processing",
		"progress":        45,
		"priority":        "medium",
		"externalVisible": true,
		"startAt":         "2026-07-01T00:00:00Z",
		"endAt":           "2026-07-10T00:00:00Z",
	})
	if visibleTaskResp.StatusCode != http.StatusCreated {
		t.Fatalf("create visible portal task expected 201 got %d, body=%v", visibleTaskResp.StatusCode, visibleTaskBody)
	}
	visibleTaskID := int(visibleTaskBody["id"].(float64))

	hiddenTaskResp, hiddenTaskBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/tasks", adminToken, map[string]any{
		"title":           "Hidden Portal Task",
		"description":     "customer must not read this",
		"projectId":       projectID,
		"status":          "queued",
		"progress":        10,
		"externalVisible": false,
	})
	if hiddenTaskResp.StatusCode != http.StatusCreated {
		t.Fatalf("create hidden portal task expected 201 got %d, body=%v", hiddenTaskResp.StatusCode, hiddenTaskBody)
	}
	hiddenTaskID := int(hiddenTaskBody["id"].(float64))

	inviteResp, inviteBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/portal-invites", adminToken, map[string]any{
		"name":         "Customer Portal",
		"company":      "Acme Customer",
		"contactName":  "Alice Customer",
		"contactEmail": "alice.customer@example.com",
		"contactType":  "customer",
		"projectId":    projectID,
		"allowedAttachments": []map[string]any{
			{
				"fileName":     "status.pdf",
				"filePath":     "/static/uploads/customer/status.pdf",
				"relativePath": "customer/status.pdf",
				"fileSize":     64,
				"mimeType":     "application/pdf",
			},
		},
	})
	if inviteResp.StatusCode != http.StatusCreated {
		t.Fatalf("create portal invite expected 201 got %d, body=%v", inviteResp.StatusCode, inviteBody)
	}
	portalToken, _ := inviteBody["token"].(string)
	if portalToken == "" {
		t.Fatalf("portal token should be returned once: %v", inviteBody)
	}
	inviteID := int(inviteBody["id"].(float64))

	statusResp, statusBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/portal/"+portalToken, "", nil)
	if statusResp.StatusCode != http.StatusOK {
		t.Fatalf("portal status expected 200 got %d, body=%v", statusResp.StatusCode, statusBody)
	}
	project, _ := statusBody["project"].(map[string]any)
	if int(project["id"].(float64)) != projectID {
		t.Fatalf("portal should expose authorized project only: %v", statusBody)
	}
	if _, ok := project["budgetAmount"]; ok {
		t.Fatalf("portal project must not expose finance fields: %v", project)
	}
	tasks, _ := statusBody["tasks"].([]any)
	if len(tasks) != 1 {
		t.Fatalf("portal should expose exactly one external task got %v", statusBody["tasks"])
	}
	task, _ := tasks[0].(map[string]any)
	if int(task["id"].(float64)) != visibleTaskID || task["title"] == "Hidden Portal Task" {
		t.Fatalf("portal task scope leaked hidden task: %v", tasks)
	}
	allowedAttachments, _ := statusBody["allowedAttachments"].([]any)
	if len(allowedAttachments) != 1 {
		t.Fatalf("portal should expose explicit allowed attachments only: %v", statusBody["allowedAttachments"])
	}

	uploadResp, uploadBody := requestMultipartFile(t, http.MethodPost, ts.URL+"/api/v1/portal/"+portalToken+"/uploads", "", "file", "external-note.txt", []byte("hello portal"))
	if uploadResp.StatusCode != http.StatusCreated {
		t.Fatalf("portal upload expected 201 got %d, body=%v", uploadResp.StatusCode, uploadBody)
	}
	uploadedAttachments, _ := uploadBody["attachments"].([]any)
	if len(uploadedAttachments) != 1 {
		t.Fatalf("portal upload should return one attachment: %v", uploadBody)
	}

	commentResp, commentBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/portal/"+portalToken+"/tasks/"+strconv.Itoa(visibleTaskID)+"/comments", "", map[string]any{
		"content":       "External delivery note",
		"externalName":  "Alice Customer",
		"externalEmail": "alice.customer@example.com",
		"attachments":   uploadedAttachments,
	})
	if commentResp.StatusCode != http.StatusCreated {
		t.Fatalf("portal comment expected 201 got %d, body=%v", commentResp.StatusCode, commentBody)
	}
	if commentBody["externalName"] != "Alice Customer" {
		t.Fatalf("portal comment response should keep external identity: %v", commentBody)
	}

	hiddenCommentResp, hiddenCommentBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/portal/"+portalToken+"/tasks/"+strconv.Itoa(hiddenTaskID)+"/comments", "", map[string]any{
		"content": "should not be accepted",
	})
	if hiddenCommentResp.StatusCode != http.StatusNotFound {
		t.Fatalf("portal hidden task comment expected 404 got %d, body=%v", hiddenCommentResp.StatusCode, hiddenCommentBody)
	}

	requestResp, requestBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/portal/"+portalToken+"/requests", "", map[string]any{
		"type":          "task",
		"title":         "External onboarding request",
		"description":   "Need an onboarding checklist",
		"priority":      "high",
		"externalName":  "Alice Customer",
		"externalEmail": "alice.customer@example.com",
		"attachments":   uploadedAttachments,
	})
	if requestResp.StatusCode != http.StatusCreated {
		t.Fatalf("portal request expected 201 got %d, body=%v", requestResp.StatusCode, requestBody)
	}
	if requestBody["source"] != "portal" || int(requestBody["projectId"].(float64)) != projectID {
		t.Fatalf("portal request should be marked and scoped: %v", requestBody)
	}

	internalCommentsResp, internalCommentsBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/tasks/"+strconv.Itoa(visibleTaskID)+"/comments", adminToken, nil)
	if internalCommentsResp.StatusCode != http.StatusOK {
		t.Fatalf("internal comments expected 200 got %d, body=%v", internalCommentsResp.StatusCode, internalCommentsBody)
	}
	internalComments, _ := internalCommentsBody["list"].([]any)
	if len(internalComments) != 1 {
		t.Fatalf("expected one portal comment internally got %v", internalCommentsBody)
	}
	internalComment, _ := internalComments[0].(map[string]any)
	if internalComment["source"] != "portal" || internalComment["externalName"] != "Alice Customer" {
		t.Fatalf("internal comment should retain portal source and external identity: %v", internalComment)
	}

	auditResp, auditBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/system/audit/logs?module=portal&page=1&pageSize=20", adminToken, nil)
	if auditResp.StatusCode != http.StatusOK {
		t.Fatalf("portal audit query expected 200 got %d, body=%v", auditResp.StatusCode, auditBody)
	}
	auditItems, _ := auditBody["list"].([]any)
	foundActions := map[string]bool{}
	for _, raw := range auditItems {
		item, _ := raw.(map[string]any)
		action, _ := item["action"].(string)
		foundActions[action] = true
	}
	for _, action := range []string{"create_invite", "view", "upload_attachment", "create_comment", "create_request"} {
		if !foundActions[action] {
			t.Fatalf("expected portal audit action %s got %v", action, auditBody)
		}
	}

	revokeResp, revokeBody := requestJSON(t, http.MethodPatch, ts.URL+"/api/v1/portal-invites/"+strconv.Itoa(inviteID)+"/revoke", adminToken, nil)
	if revokeResp.StatusCode != http.StatusOK {
		t.Fatalf("revoke portal invite expected 200 got %d, body=%v", revokeResp.StatusCode, revokeBody)
	}
	revokedStatusResp, revokedStatusBody := requestJSON(t, http.MethodGet, ts.URL+"/api/v1/portal/"+portalToken, "", nil)
	if revokedStatusResp.StatusCode != http.StatusForbidden {
		t.Fatalf("revoked portal expected 403 got %d, body=%v", revokedStatusResp.StatusCode, revokedStatusBody)
	}
}
