package model

import "time"

type TaskStatus string

const (
	TaskPending    TaskStatus = "pending"
	TaskQueued     TaskStatus = "queued"
	TaskProcessing TaskStatus = "processing"
	TaskCompleted  TaskStatus = "completed"
)

type TaskPriority string

const (
	TaskPriorityHigh   TaskPriority = "high"
	TaskPriorityMedium TaskPriority = "medium"
	TaskPriorityLow    TaskPriority = "low"
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

type Task struct {
	BaseModel
	TaskNo       string           `gorm:"size:64;uniqueIndex;not null" json:"taskNo"`
	Title        string           `gorm:"size:150;not null" json:"title"`
	Description  string           `gorm:"type:text" json:"description"`
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
	Tags         []Tag            `gorm:"many2many:task_tags;" json:"tags,omitempty"`
	Dependencies []TaskDependency `gorm:"foreignKey:TaskID" json:"dependencies,omitempty"`
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
