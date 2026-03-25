package handler

import (
	"bytes"
	"encoding/csv"
	"net/http"
	"strconv"
	"strings"
	"time"

	"project-manager/backend/internal/model"

	"github.com/gin-gonic/gin"
)

func formatTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.Format(time.RFC3339)
}

func joinUserNames(users []model.User) string {
	if len(users) == 0 {
		return ""
	}
	names := make([]string, 0, len(users))
	for _, user := range users {
		if strings.TrimSpace(user.Name) != "" {
			names = append(names, user.Name)
		} else {
			names = append(names, user.Username)
		}
	}
	return strings.Join(names, ",")
}

func joinDepartmentNames(items []model.Department) string {
	if len(items) == 0 {
		return ""
	}
	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.Name)
	}
	return strings.Join(names, ",")
}

func writeCSV(c *gin.Context, filename string, header []string, rows [][]string) {
	var buffer bytes.Buffer
	writer := csv.NewWriter(&buffer)
	_ = writer.Write(header)
	_ = writer.WriteAll(rows)
	writer.Flush()

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Data(http.StatusOK, "text/csv; charset=utf-8", buffer.Bytes())
}

func (h *Handler) ExportProjectsCSV(c *gin.Context) {
	var projects []model.Project
	query := h.DB.Model(&model.Project{}).Preload("Users").Preload("Departments").Order("id desc")
	query = h.scopeProjectsQuery(c, query)
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("code LIKE ? OR name LIKE ? OR description LIKE ?", like, like, like)
	}
	if err := query.Find(&projects).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "EXPORT_PROJECTS_FAILED", err)
		return
	}

	rows := make([][]string, 0, len(projects))
	for _, project := range projects {
		rows = append(rows, []string{
			strconv.FormatUint(uint64(project.ID), 10),
			project.Code,
			project.Name,
			project.Description,
			formatTime(project.StartAt),
			formatTime(project.EndAt),
			joinUserNames(project.Users),
			joinDepartmentNames(project.Departments),
			project.CreatedAt.Format(time.RFC3339),
		})
	}
	writeCSV(c, "projects.csv",
		[]string{"ID", "编码", "名称", "描述", "开始时间", "结束时间", "负责人", "参与部门", "创建时间"},
		rows,
	)
}

func (h *Handler) ExportTasksCSV(c *gin.Context) {
	var tasks []model.Task
	query := h.DB.Model(&model.Task{}).Preload("Assignees").Preload("Creator").Order("id desc")
	query = h.scopeTasksQuery(c, query)
	if projectID := strings.TrimSpace(c.Query("projectId")); projectID != "" {
		query = query.Where("project_id = ?", projectID)
	}
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		query = query.Where("status = ?", status)
	}
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("task_no LIKE ? OR title LIKE ? OR description LIKE ?", like, like, like)
	}
	if err := query.Find(&tasks).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "EXPORT_TASKS_FAILED", err)
		return
	}

	rows := make([][]string, 0, len(tasks))
	for _, task := range tasks {
		parentID := ""
		if task.ParentID != nil {
			parentID = strconv.FormatUint(uint64(*task.ParentID), 10)
		}
		rows = append(rows, []string{
			strconv.FormatUint(uint64(task.ID), 10),
			task.TaskNo,
			task.Title,
			task.Description,
			string(task.Status),
			string(task.Priority),
			strconv.Itoa(task.Progress),
			formatTime(task.StartAt),
			formatTime(task.EndAt),
			strconv.FormatUint(uint64(task.ProjectID), 10),
			parentID,
			strconv.FormatUint(uint64(task.CreatorID), 10),
			joinUserNames(task.Assignees),
			task.CreatedAt.Format(time.RFC3339),
		})
	}
	writeCSV(c, "tasks.csv",
		[]string{"ID", "任务编号", "标题", "描述", "状态", "优先级", "进度", "开始时间", "结束时间", "项目ID", "父任务ID", "创建人ID", "执行人", "创建时间"},
		rows,
	)
}
