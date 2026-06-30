package router

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"

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
	req, _ := http.NewRequest(http.MethodGet, serverURL+"/api/v1/rbac/permissions", nil)
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

func TestChangeOwnPasswordFlow(t *testing.T) {
	ts := setupTestRouter(t)
	defer ts.Close()

	adminToken := loginAndToken(t, ts.URL)
	username := "change_pwd_user"
	originalPassword := "pass1234"
	newPassword := "pass5678"

	createUserResp, _ := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/users", adminToken, map[string]any{
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
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/departments", bytes.NewReader(raw))
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

	logsReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/audit/logs?module=departments", nil)
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
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/audit/logs", nil)
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

	permReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/rbac/permissions", nil)
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

	roleResp, roleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/rbac/roles", adminToken, map[string]any{
		"name":          "scope-reader",
		"description":   "scope reader",
		"permissionIds": readerPerms,
	})
	if roleResp.StatusCode != http.StatusCreated {
		t.Fatalf("create role status expected 201 got %d", roleResp.StatusCode)
	}
	roleID := uint(roleBody["id"].(float64))

	userAResp, userABody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/users", adminToken, map[string]any{
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

	userBResp, userBBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/users", adminToken, map[string]any{
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

func TestProjectHealthUsesVisibleTaskScope(t *testing.T) {
	ts := setupTestRouter(t)
	defer ts.Close()

	adminToken := loginAndToken(t, ts.URL)
	codeToID := permissionCodeMap(t, ts.URL, adminToken)

	roleResp, roleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/rbac/roles", adminToken, map[string]any{
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

	userResp, userBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/users", adminToken, map[string]any{
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

func TestNotificationFlowOnTaskAssign(t *testing.T) {
	ts := setupTestRouter(t)
	defer ts.Close()

	adminToken := loginAndToken(t, ts.URL)

	permReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/rbac/permissions", nil)
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

	roleResp, roleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/rbac/roles", adminToken, map[string]any{
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

	userResp, userBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/users", adminToken, map[string]any{
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

	roleResp, roleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/rbac/roles", adminToken, map[string]any{
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

	assigneeResp, assigneeBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/users", adminToken, map[string]any{
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

	reviewerResp, reviewerBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/users", adminToken, map[string]any{
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

	roleResp, roleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/rbac/roles", adminToken, map[string]any{
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

	assigneeResp, assigneeBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/users", adminToken, map[string]any{
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

	reviewerResp, reviewerBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/users", adminToken, map[string]any{
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

	outsiderResp, _ := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/users", adminToken, map[string]any{
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

	requesterRoleResp, requesterRoleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/rbac/roles", adminToken, map[string]any{
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

	managerRoleResp, managerRoleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/rbac/roles", adminToken, map[string]any{
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

	requesterResp, requesterBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/users", adminToken, map[string]any{
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

	otherResp, _ := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/users", adminToken, map[string]any{
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

	managerResp, managerBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/users", adminToken, map[string]any{
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

func TestProjectTemplateCreateAndGenerateProjectTree(t *testing.T) {
	ts := setupTestRouter(t)
	defer ts.Close()

	adminToken := loginAndToken(t, ts.URL)
	codeToID := permissionCodeMap(t, ts.URL, adminToken)

	creatorOnlyRoleResp, creatorOnlyRoleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/rbac/roles", adminToken, map[string]any{
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

	creatorResp, _ := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/users", adminToken, map[string]any{
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

func TestAutomationRuleOverdueTaskNotification(t *testing.T) {
	ts := setupTestRouter(t)
	defer ts.Close()

	adminToken := loginAndToken(t, ts.URL)
	codeToID := permissionCodeMap(t, ts.URL, adminToken)

	notificationRoleResp, notificationRoleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/rbac/roles", adminToken, map[string]any{
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

	readOnlyRoleResp, readOnlyRoleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/rbac/roles", adminToken, map[string]any{
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

	ownerResp, ownerBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/users", adminToken, map[string]any{
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

	assigneeResp, assigneeBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/users", adminToken, map[string]any{
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

	readOnlyUserResp, _ := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/users", adminToken, map[string]any{
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

	collaboratorRoleResp, collaboratorRoleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/rbac/roles", adminToken, map[string]any{
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

	notificationRoleResp, notificationRoleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/rbac/roles", adminToken, map[string]any{
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

	ownerResp, ownerBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/users", adminToken, map[string]any{
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

	assigneeResp, assigneeBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/users", adminToken, map[string]any{
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
	roleResp, roleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/rbac/roles", adminToken, map[string]any{
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

	userResp, userBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/users", adminToken, map[string]any{
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
	createResp, _ := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/rbac/roles", token, map[string]any{
		"name":          roleName,
		"description":   "rollback test",
		"permissionIds": []uint{1},
	})
	if createResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("create role with failpoint expected 400 got %d", createResp.StatusCode)
	}
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/rbac/roles", nil)
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
	createResp, _ := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/rbac/permissions", token, map[string]any{
		"code": code,
		"name": "Rollback Permission",
	})
	if createResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("create permission with failpoint expected 400 got %d", createResp.StatusCode)
	}

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/rbac/permissions", nil)
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

	roleResp, roleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/rbac/roles", adminToken, map[string]any{
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

	userResp, userBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/users", adminToken, map[string]any{
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

func TestDisabledUserCannotLogin(t *testing.T) {
	ts := setupTestRouter(t)
	defer ts.Close()

	adminToken := loginAndToken(t, ts.URL)
	userResp, userBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/users", adminToken, map[string]any{
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

	disableResp, _ := requestJSON(t, http.MethodPut, ts.URL+"/api/v1/users/"+strconv.Itoa(userID), adminToken, map[string]any{
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

	roleResp, roleBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/rbac/roles", adminToken, map[string]any{
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

	authorResp, authorBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/users", adminToken, map[string]any{
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

	mentionedResp, mentionedBody := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/users", adminToken, map[string]any{
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

	outsiderResp, _ := requestJSON(t, http.MethodPost, ts.URL+"/api/v1/users", adminToken, map[string]any{
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
