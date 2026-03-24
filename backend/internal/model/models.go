package model

import "time"

type TaskStatus string

const (
	TaskPending   TaskStatus = "pending"
	TaskQueued    TaskStatus = "queued"
	TaskProcessing TaskStatus = "processing"
	TaskCompleted TaskStatus = "completed"
)

type BaseModel struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type User struct {
	BaseModel
	Username   string       `gorm:"size:64;uniqueIndex;not null" json:"username"`
	Name       string       `gorm:"size:100;not null" json:"name"`
	Email      string       `gorm:"size:120;uniqueIndex;not null" json:"email"`
	Password   string       `gorm:"size:255;not null" json:"-"`
	IsActive   bool         `gorm:"default:true" json:"isActive"`
	Roles      []Role       `gorm:"many2many:user_roles;" json:"roles,omitempty"`
	Departments []Department `gorm:"many2many:user_departments;" json:"departments,omitempty"`
	Projects   []Project    `gorm:"many2many:project_users;" json:"projects,omitempty"`
	Tasks      []Task       `gorm:"many2many:task_users;" json:"tasks,omitempty"`
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

type Project struct {
	BaseModel
	Code         string       `gorm:"size:64;uniqueIndex;not null" json:"code"`
	Name         string       `gorm:"size:150;not null" json:"name"`
	Description  string       `gorm:"type:text" json:"description"`
	StartAt      *time.Time   `json:"startAt"`
	EndAt        *time.Time   `json:"endAt"`
	Users        []User       `gorm:"many2many:project_users;" json:"users,omitempty"`
	Departments  []Department `gorm:"many2many:project_departments;" json:"departments,omitempty"`
	Tasks        []Task       `json:"tasks,omitempty"`
}

type Task struct {
	BaseModel
	TaskNo      string      `gorm:"size:64;uniqueIndex;not null" json:"taskNo"`
	Title       string      `gorm:"size:150;not null" json:"title"`
	Description string      `gorm:"type:text" json:"description"`
	Status      TaskStatus  `gorm:"size:20;default:'pending'" json:"status"`
	Progress    int         `gorm:"default:0" json:"progress"`
	StartAt     *time.Time  `json:"startAt"`
	EndAt       *time.Time  `json:"endAt"`
	CreatorID   uint        `gorm:"not null" json:"creatorId"`
	Creator     User        `json:"creator,omitempty"`
	ProjectID   uint        `gorm:"not null;index" json:"projectId"`
	Project     Project     `json:"project,omitempty"`
	ParentID    *uint       `gorm:"index" json:"parentId"`
	Children    []Task      `gorm:"foreignKey:ParentID" json:"children,omitempty"`
	Assignees   []User      `gorm:"many2many:task_users;" json:"assignees,omitempty"`
}

type AuditLog struct {
	BaseModel
	UserID    uint   `gorm:"index" json:"userId"`
	Module    string `gorm:"size:50;index;not null" json:"module"`
	Action    string `gorm:"size:50;not null" json:"action"`
	TargetID  uint   `gorm:"index" json:"targetId"`
	Method    string `gorm:"size:10;not null" json:"method"`
	Path      string `gorm:"size:255;not null" json:"path"`
	Success   bool   `gorm:"default:true" json:"success"`
	Detail    string `gorm:"type:text" json:"detail"`
	ClientIP  string `gorm:"size:50" json:"clientIp"`
	UserAgent string `gorm:"size:255" json:"userAgent"`
}
