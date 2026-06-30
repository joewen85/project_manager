package model

import "time"

type TaskStatus string

const (
	TaskPending    TaskStatus = "pending"
	TaskQueued     TaskStatus = "queued"
	TaskProcessing TaskStatus = "processing"
	TaskReviewing  TaskStatus = "reviewing"
	TaskCompleted  TaskStatus = "completed"
)

type TaskPriority string

const (
	TaskPriorityHigh   TaskPriority = "high"
	TaskPriorityMedium TaskPriority = "medium"
	TaskPriorityLow    TaskPriority = "low"
)

type WorkRequestType string

const (
	WorkRequestProject WorkRequestType = "project"
	WorkRequestTask    WorkRequestType = "task"
	WorkRequestBug     WorkRequestType = "bug"
	WorkRequestChange  WorkRequestType = "change"
)

type WorkRequestStatus string

const (
	WorkRequestSubmitted WorkRequestStatus = "submitted"
	WorkRequestApproved  WorkRequestStatus = "approved"
	WorkRequestRejected  WorkRequestStatus = "rejected"
	WorkRequestConverted WorkRequestStatus = "converted"
)

type AutomationTrigger string

const (
	AutomationTriggerTaskOverdue         AutomationTrigger = "task_overdue"
	AutomationTriggerTaskStatusChanged   AutomationTrigger = "task_status_changed"
	AutomationTriggerTaskProgressChanged AutomationTrigger = "task_progress_changed"
	AutomationTriggerTaskAssigneeChanged AutomationTrigger = "task_assignee_changed"
)

type AutomationExecutionStatus string

const (
	AutomationExecutionSuccess AutomationExecutionStatus = "success"
	AutomationExecutionSkipped AutomationExecutionStatus = "skipped"
	AutomationExecutionFailed  AutomationExecutionStatus = "failed"
)

type AssigneeChangeType string

const (
	AssigneeChangeAdded   AssigneeChangeType = "added"
	AssigneeChangeRemoved AssigneeChangeType = "removed"
)

type BaseModel struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Attachment struct {
	FileName     string `gorm:"size:255" json:"fileName"`
	FilePath     string `gorm:"size:600" json:"filePath"`
	RelativePath string `gorm:"size:600" json:"relativePath"`
	FileSize     int64  `json:"fileSize"`
	MimeType     string `gorm:"size:120" json:"mimeType"`
}

type User struct {
	BaseModel
	Username    string       `gorm:"size:64;uniqueIndex;not null" json:"username"`
	Name        string       `gorm:"size:100;not null" json:"name"`
	Email       string       `gorm:"size:120;uniqueIndex;not null" json:"email"`
	Password    string       `gorm:"size:255;not null" json:"-"`
	IsActive    bool         `gorm:"default:true" json:"isActive"`
	Roles       []Role       `gorm:"many2many:user_roles;" json:"roles,omitempty"`
	Departments []Department `gorm:"many2many:user_departments;" json:"departments,omitempty"`
	Projects    []Project    `gorm:"many2many:project_users;" json:"projects,omitempty"`
	Tasks       []Task       `gorm:"many2many:task_users;" json:"tasks,omitempty"`
}

type Role struct {
	BaseModel
	Name        string       `gorm:"size:64;uniqueIndex;not null" json:"name"`
	Description string       `gorm:"size:255" json:"description"`
	Permissions []Permission `gorm:"many2many:role_permissions;" json:"permissions,omitempty"`
}

type Permission struct {
	BaseModel
	Code        string `gorm:"size:128;uniqueIndex;not null" json:"code"`
	Name        string `gorm:"size:100;not null" json:"name"`
	Description string `gorm:"size:255" json:"description"`
}

type Department struct {
	BaseModel
	Name        string    `gorm:"size:100;uniqueIndex;not null" json:"name"`
	Description string    `gorm:"size:255" json:"description"`
	Users       []User    `gorm:"many2many:user_departments;" json:"users,omitempty"`
	Projects    []Project `gorm:"many2many:project_departments;" json:"projects,omitempty"`
}

type Tag struct {
	BaseModel
	Name      string `gorm:"size:100;uniqueIndex;not null" json:"name"`
	TaskCount int64  `gorm:"->;column:task_count;-:migration" json:"taskCount"`
	Tasks     []Task `gorm:"many2many:task_tags;" json:"tasks,omitempty"`
}

type Project struct {
	BaseModel
	Code        string       `gorm:"size:64;uniqueIndex;not null" json:"code"`
	Name        string       `gorm:"size:150;not null" json:"name"`
	Description string       `gorm:"type:text" json:"description"`
	StartAt     *time.Time   `json:"startAt"`
	EndAt       *time.Time   `json:"endAt"`
	Attachment  Attachment   `gorm:"embedded;embeddedPrefix:attachment_" json:"attachment,omitempty"`
	Attachments []Attachment `gorm:"serializer:json" json:"attachments"`
	Users       []User       `gorm:"many2many:project_users;" json:"users,omitempty"`
	Departments []Department `gorm:"many2many:project_departments;" json:"departments,omitempty"`
	Tasks       []Task       `json:"tasks,omitempty"`
}

type TemplateTaskDependency struct {
	DependsOnKey string `json:"dependsOnKey"`
	LagDays      int    `json:"lagDays"`
	Type         string `json:"type"`
}

type TemplateTask struct {
	Key              string                   `json:"key"`
	Title            string                   `json:"title"`
	Description      string                   `json:"description"`
	Priority         TaskPriority             `json:"priority"`
	IsMilestone      bool                     `json:"isMilestone"`
	RelativeStartDay int                      `json:"relativeStartDay"`
	DurationDays     int                      `json:"durationDays"`
	Dependencies     []TemplateTaskDependency `json:"dependencies"`
	Children         []TemplateTask           `json:"children"`
}

type ProjectTemplate struct {
	BaseModel
	Name        string         `gorm:"size:150;uniqueIndex;not null" json:"name"`
	Description string         `gorm:"type:text" json:"description"`
	TaskTree    []TemplateTask `gorm:"serializer:json" json:"taskTree"`
}

type Task struct {
	BaseModel
	TaskNo       string           `gorm:"size:64;uniqueIndex;not null" json:"taskNo"`
	Title        string           `gorm:"size:150;not null" json:"title"`
	Description  string           `gorm:"type:text" json:"description"`
	CustomField1 string           `gorm:"type:text" json:"customField1"`
	CustomField2 string           `gorm:"type:text" json:"customField2"`
	CustomField3 string           `gorm:"type:text" json:"customField3"`
	Status       TaskStatus       `gorm:"size:20;default:'pending';index" json:"status"`
	Priority     TaskPriority     `gorm:"size:20;default:'high';index" json:"priority"`
	IsMilestone  bool             `gorm:"default:false;index" json:"isMilestone"`
	Progress     int              `gorm:"default:0" json:"progress"`
	StartAt      *time.Time       `json:"startAt"`
	EndAt        *time.Time       `json:"endAt"`
	Attachment   Attachment       `gorm:"embedded;embeddedPrefix:attachment_" json:"attachment,omitempty"`
	Attachments  []Attachment     `gorm:"serializer:json" json:"attachments"`
	CreatorID    uint             `gorm:"not null;index" json:"creatorId"`
	Creator      User             `json:"creator,omitempty"`
	ProjectID    uint             `gorm:"not null;index" json:"projectId"`
	ProjectName  string           `gorm:"->;column:project_name;-:migration" json:"projectName,omitempty"`
	Project      Project          `json:"project,omitempty"`
	ParentID     *uint            `gorm:"index" json:"parentId"`
	Children     []Task           `gorm:"foreignKey:ParentID" json:"children,omitempty"`
	Assignees    []User           `gorm:"many2many:task_users;" json:"assignees,omitempty"`
	Reviewers    []User           `gorm:"many2many:task_reviewers;" json:"reviewers,omitempty"`
	Tags         []Tag            `gorm:"many2many:task_tags;" json:"tags,omitempty"`
	Dependencies []TaskDependency `gorm:"foreignKey:TaskID" json:"dependencies,omitempty"`
	Comments     []TaskComment    `gorm:"foreignKey:TaskID" json:"comments,omitempty"`
	Activities   []TaskActivity   `gorm:"foreignKey:TaskID" json:"activities,omitempty"`
}

type TaskDependency struct {
	BaseModel
	TaskID          uint   `gorm:"not null;index;uniqueIndex:idx_task_dependency_unique" json:"taskId"`
	DependsOnTaskID uint   `gorm:"not null;index;uniqueIndex:idx_task_dependency_unique" json:"dependsOnTaskId"`
	LagDays         int    `gorm:"default:0" json:"lagDays"`
	Type            string `gorm:"size:8;default:'FS'" json:"type"`
	Task            Task   `gorm:"foreignKey:TaskID;constraint:OnDelete:CASCADE;" json:"-"`
	DependsOnTask   Task   `gorm:"foreignKey:DependsOnTaskID;constraint:OnDelete:CASCADE;" json:"-"`
}

type TaskComment struct {
	BaseModel
	TaskID      uint         `gorm:"not null;index;index:idx_task_comment_deleted_created" json:"taskId"`
	Task        Task         `gorm:"foreignKey:TaskID;constraint:OnDelete:CASCADE;" json:"-"`
	AuthorID    uint         `gorm:"not null;index" json:"authorId"`
	Author      User         `json:"author,omitempty"`
	Content     string       `gorm:"type:text;not null" json:"content"`
	Attachments []Attachment `gorm:"serializer:json" json:"attachments"`
	Mentions    []User       `gorm:"many2many:task_comment_mentions;" json:"mentions,omitempty"`
	IsDeleted   bool         `gorm:"default:false;index;index:idx_task_comment_deleted_created" json:"isDeleted"`
}

type TaskActivity struct {
	BaseModel
	TaskID    uint         `gorm:"not null;index" json:"taskId"`
	Task      Task         `gorm:"foreignKey:TaskID;constraint:OnDelete:CASCADE;" json:"-"`
	ActorID   uint         `gorm:"index" json:"actorId"`
	Actor     User         `json:"actor,omitempty"`
	Type      string       `gorm:"size:50;not null;index" json:"type"`
	Summary   string       `gorm:"size:255;not null" json:"summary"`
	Detail    string       `gorm:"type:text" json:"detail"`
	CommentID *uint        `gorm:"index" json:"commentId,omitempty"`
	Comment   *TaskComment `json:"comment,omitempty"`
}

type WorkRequest struct {
	BaseModel
	Type            WorkRequestType   `gorm:"size:20;default:'task';index" json:"type"`
	Title           string            `gorm:"size:150;not null" json:"title"`
	Description     string            `gorm:"type:text" json:"description"`
	Priority        TaskPriority      `gorm:"size:20;default:'medium';index" json:"priority"`
	Status          WorkRequestStatus `gorm:"size:20;default:'submitted';index" json:"status"`
	ProjectID       *uint             `gorm:"index" json:"projectId,omitempty"`
	Project         *Project          `json:"project,omitempty"`
	RequesterID     uint              `gorm:"not null;index" json:"requesterId"`
	Requester       User              `json:"requester,omitempty"`
	ReviewerID      *uint             `gorm:"index" json:"reviewerId,omitempty"`
	Reviewer        *User             `json:"reviewer,omitempty"`
	ApprovalNote    string            `gorm:"type:text" json:"approvalNote"`
	ConvertedTaskID *uint             `gorm:"index" json:"convertedTaskId,omitempty"`
	ConvertedTask   *Task             `json:"convertedTask,omitempty"`
}

type AutomationConditions struct {
	OverdueDays         int                  `json:"overdueDays"`
	ProjectIDs          []uint               `json:"projectIds"`
	FromStatuses        []TaskStatus         `json:"fromStatuses"`
	ToStatuses          []TaskStatus         `json:"toStatuses"`
	FromProgressMin     *int                 `json:"fromProgressMin,omitempty"`
	FromProgressMax     *int                 `json:"fromProgressMax,omitempty"`
	ToProgressMin       *int                 `json:"toProgressMin,omitempty"`
	ToProgressMax       *int                 `json:"toProgressMax,omitempty"`
	AssigneeChangeTypes []AssigneeChangeType `json:"assigneeChangeTypes"`
}

type AutomationActions struct {
	NotifyAssignees     bool   `json:"notifyAssignees"`
	NotifyProjectOwners bool   `json:"notifyProjectOwners"`
	AddComment          bool   `json:"addComment"`
	CommentContent      string `json:"commentContent"`
	AddTags             bool   `json:"addTags"`
	TagIDs              []uint `json:"tagIds"`
	AssignAssignees     bool   `json:"assignAssignees"`
	AssigneeIDs         []uint `json:"assigneeIds"`
}

type AutomationRule struct {
	BaseModel
	Name        string               `gorm:"size:150;not null" json:"name"`
	Trigger     AutomationTrigger    `gorm:"size:50;default:'task_overdue';index" json:"trigger"`
	IsEnabled   bool                 `gorm:"default:true;index" json:"isEnabled"`
	Conditions  AutomationConditions `gorm:"serializer:json" json:"conditions"`
	Actions     AutomationActions    `gorm:"serializer:json" json:"actions"`
	LastRunAt   *time.Time           `json:"lastRunAt,omitempty"`
	CreatedByID uint                 `gorm:"index" json:"createdById"`
	CreatedBy   User                 `json:"createdBy,omitempty"`
}

type AutomationExecutionLog struct {
	BaseModel
	RuleID       uint                      `gorm:"not null;index" json:"ruleId"`
	Rule         AutomationRule            `gorm:"foreignKey:RuleID;constraint:OnDelete:CASCADE;" json:"rule,omitempty"`
	Trigger      AutomationTrigger         `gorm:"size:50;index" json:"trigger"`
	Status       AutomationExecutionStatus `gorm:"size:20;index" json:"status"`
	MatchedCount int                       `json:"matchedCount"`
	ActionCount  int                       `json:"actionCount"`
	Message      string                    `gorm:"type:text" json:"message"`
	ActorID      uint                      `gorm:"index" json:"actorId"`
	RunSource    string                    `gorm:"size:20;default:'manual';index" json:"runSource"`
}

type AuditLog struct {
	BaseModel
	UserID    uint   `gorm:"index" json:"userId"`
	Module    string `gorm:"size:50;index;index:idx_audit_module_action;not null" json:"module"`
	Action    string `gorm:"size:50;index:idx_audit_module_action;not null" json:"action"`
	TargetID  uint   `gorm:"index" json:"targetId"`
	Method    string `gorm:"size:10;not null" json:"method"`
	Path      string `gorm:"size:255;not null" json:"path"`
	Success   bool   `gorm:"default:true" json:"success"`
	Detail    string `gorm:"type:text" json:"detail"`
	ClientIP  string `gorm:"size:50" json:"clientIp"`
	UserAgent string `gorm:"size:255" json:"userAgent"`
}

type Notification struct {
	BaseModel
	UserID   uint   `gorm:"index;index:idx_notification_user_read;not null" json:"userId"`
	Title    string `gorm:"size:150;not null" json:"title"`
	Content  string `gorm:"type:text" json:"content"`
	Module   string `gorm:"size:50;index;not null" json:"module"`
	TargetID uint   `gorm:"index" json:"targetId"`
	IsRead   bool   `gorm:"default:false;index;index:idx_notification_user_read" json:"isRead"`
}
