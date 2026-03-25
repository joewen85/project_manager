package router

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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

	cfg := config.Config{JWTSecret: "test-secret"}
	h := handler.New(db, cfg)
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
			codeToID["projects.read"], codeToID["tasks.read"], codeToID["notifications.read"], codeToID["notifications.write"],
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
