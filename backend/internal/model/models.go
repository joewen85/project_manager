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
	WorkRequestApplied   WorkRequestStatus = "applied"
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

type ContractAttachment struct {
	FileName     string     `json:"fileName"`
	FilePath     string     `json:"filePath"`
	RelativePath string     `json:"relativePath"`
	FileSize     int64      `json:"fileSize"`
	MimeType     string     `json:"mimeType"`
	Category     string     `json:"category"`
	Version      string     `json:"version"`
	AccessLevel  string     `json:"accessLevel"`
	ExpiresAt    *time.Time `json:"expiresAt,omitempty"`
}

type User struct {
	BaseModel
	Username            string       `gorm:"size:64;uniqueIndex;not null" json:"username"`
	Name                string       `gorm:"size:100;not null" json:"name"`
	Email               string       `gorm:"size:120;uniqueIndex;not null" json:"email"`
	Password            string       `gorm:"size:255;not null" json:"-"`
	IsActive            bool         `gorm:"default:true" json:"isActive"`
	WeeklyCapacityHours float64      `gorm:"type:decimal(10,2)" json:"weeklyCapacityHours"`
	Roles               []Role       `gorm:"many2many:user_roles;" json:"roles,omitempty"`
	Departments         []Department `gorm:"many2many:user_departments;" json:"departments,omitempty"`
	Projects            []Project    `gorm:"many2many:project_users;" json:"projects,omitempty"`
	Tasks               []Task       `gorm:"many2many:task_users;" json:"tasks,omitempty"`
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

type APIToken struct {
	BaseModel
	Name             string     `gorm:"size:150;not null;index" json:"name"`
	Description      string     `gorm:"size:255" json:"description"`
	TokenPrefix      string     `gorm:"size:32;not null;index" json:"tokenPrefix"`
	TokenLastFour    string     `gorm:"size:8;not null" json:"tokenLastFour"`
	TokenHash        string     `gorm:"size:128;uniqueIndex;not null" json:"-"`
	PermissionCodes  []string   `gorm:"serializer:json" json:"permissionCodes"`
	IsEnabled        bool       `gorm:"default:true;index" json:"isEnabled"`
	ExpiresAt        *time.Time `json:"expiresAt"`
	LastUsedAt       *time.Time `json:"lastUsedAt"`
	LastUsedIP       string     `gorm:"size:50" json:"lastUsedIp"`
	RevokedAt        *time.Time `json:"revokedAt"`
	CreatedByID      uint       `gorm:"not null;index" json:"createdById"`
	CreatedBy        User       `json:"createdBy,omitempty"`
	ServiceAccountID uint       `gorm:"not null;index" json:"serviceAccountId"`
	ServiceAccount   User       `gorm:"foreignKey:ServiceAccountID" json:"serviceAccount,omitempty"`
	ServiceRoleID    uint       `gorm:"not null;index" json:"serviceRoleId"`
	ServiceRole      Role       `gorm:"foreignKey:ServiceRoleID" json:"serviceRole,omitempty"`
}

type PortalInvite struct {
	BaseModel
	Name               string       `gorm:"size:150;not null;index" json:"name"`
	Company            string       `gorm:"size:150" json:"company"`
	ContactName        string       `gorm:"size:120" json:"contactName"`
	ContactEmail       string       `gorm:"size:160" json:"contactEmail"`
	ContactType        string       `gorm:"size:40;default:'customer';index" json:"contactType"`
	TokenPrefix        string       `gorm:"size:32;not null;index" json:"tokenPrefix"`
	TokenLastFour      string       `gorm:"size:8;not null" json:"tokenLastFour"`
	TokenHash          string       `gorm:"size:128;uniqueIndex;not null" json:"-"`
	IsEnabled          bool         `gorm:"default:true;index" json:"isEnabled"`
	ExpiresAt          *time.Time   `json:"expiresAt,omitempty"`
	RevokedAt          *time.Time   `gorm:"index" json:"revokedAt,omitempty"`
	LastUsedAt         *time.Time   `json:"lastUsedAt,omitempty"`
	LastUsedIP         string       `gorm:"size:50" json:"lastUsedIp"`
	AllowedAttachments []Attachment `gorm:"serializer:json" json:"allowedAttachments"`
	ProjectID          uint         `gorm:"not null;index" json:"projectId"`
	Project            Project      `json:"project,omitempty"`
	CreatedByID        uint         `gorm:"not null;index" json:"createdById"`
	CreatedBy          User         `json:"createdBy,omitempty"`
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
	Code                  string               `gorm:"size:64;uniqueIndex;not null" json:"code"`
	Name                  string               `gorm:"size:150;not null" json:"name"`
	Description           string               `gorm:"type:text" json:"description"`
	StartAt               *time.Time           `json:"startAt"`
	EndAt                 *time.Time           `json:"endAt"`
	BudgetAmount          float64              `gorm:"type:decimal(14,2);not null;default:0" json:"-"`
	ActualCostAmount      float64              `gorm:"type:decimal(14,2);not null;default:0" json:"-"`
	ExpectedRevenueAmount float64              `gorm:"type:decimal(14,2);not null;default:0" json:"-"`
	ContractNo            string               `gorm:"size:120;index" json:"-"`
	ContractAttachments   []ContractAttachment `gorm:"serializer:json" json:"-"`
	Attachment            Attachment           `gorm:"embedded;embeddedPrefix:attachment_" json:"attachment,omitempty"`
	Attachments           []Attachment         `gorm:"serializer:json" json:"attachments"`
	Users                 []User               `gorm:"many2many:project_users;" json:"users,omitempty"`
	Departments           []Department         `gorm:"many2many:project_departments;" json:"departments,omitempty"`
	Tasks                 []Task               `json:"tasks,omitempty"`
}

type ProjectBaselineTaskSnapshot struct {
	TaskID      uint       `json:"taskId"`
	TaskNo      string     `json:"taskNo"`
	Title       string     `json:"title"`
	Status      TaskStatus `json:"status"`
	Progress    int        `json:"progress"`
	IsMilestone bool       `json:"isMilestone"`
	StartAt     *time.Time `json:"startAt"`
	EndAt       *time.Time `json:"endAt"`
	ParentID    *uint      `json:"parentId"`
}

type ProjectBaseline struct {
	BaseModel
	ProjectID          uint                          `gorm:"not null;index" json:"projectId"`
	Project            Project                       `json:"project,omitempty"`
	Name               string                        `gorm:"size:150;not null;index" json:"name"`
	Description        string                        `gorm:"type:text" json:"description"`
	TaskCount          int                           `json:"taskCount"`
	CompletedTaskCount int                           `json:"completedTaskCount"`
	PlannedStartAt     *time.Time                    `json:"plannedStartAt"`
	PlannedEndAt       *time.Time                    `json:"plannedEndAt"`
	Snapshot           []ProjectBaselineTaskSnapshot `gorm:"serializer:json" json:"snapshot"`
	CreatedByID        uint                          `gorm:"not null;index" json:"createdById"`
	CreatedBy          User                          `json:"createdBy,omitempty"`
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

type SavedReportType string

const (
	SavedReportProjectHealth          SavedReportType = "project_health"
	SavedReportMemberWorkload         SavedReportType = "member_workload"
	SavedReportTaskStatus             SavedReportType = "task_status"
	SavedReportTaskThroughput         SavedReportType = "task_throughput"
	SavedReportOverdueTrend           SavedReportType = "overdue_trend"
	SavedReportDepartmentDistribution SavedReportType = "department_distribution"
)

type ProjectRegisterType string

const (
	ProjectRegisterRisk     ProjectRegisterType = "risk"
	ProjectRegisterIssue    ProjectRegisterType = "issue"
	ProjectRegisterDecision ProjectRegisterType = "decision"
)

type ProjectRegisterStatus string

const (
	ProjectRegisterOpen       ProjectRegisterStatus = "open"
	ProjectRegisterInProgress ProjectRegisterStatus = "in_progress"
	ProjectRegisterResolved   ProjectRegisterStatus = "resolved"
	ProjectRegisterClosed     ProjectRegisterStatus = "closed"
)

type ProjectRegisterSeverity string

const (
	ProjectRegisterSeverityLow      ProjectRegisterSeverity = "low"
	ProjectRegisterSeverityMedium   ProjectRegisterSeverity = "medium"
	ProjectRegisterSeverityHigh     ProjectRegisterSeverity = "high"
	ProjectRegisterSeverityCritical ProjectRegisterSeverity = "critical"
)

type ProjectRegisterProbability string

const (
	ProjectRegisterProbabilityLow    ProjectRegisterProbability = "low"
	ProjectRegisterProbabilityMedium ProjectRegisterProbability = "medium"
	ProjectRegisterProbabilityHigh   ProjectRegisterProbability = "high"
)

type SavedReportFilters struct {
	ProjectID    uint     `json:"projectId,omitempty"`
	DepartmentID uint     `json:"departmentId,omitempty"`
	OwnerID      uint     `json:"ownerId,omitempty"`
	DateFrom     string   `json:"dateFrom,omitempty"`
	DateTo       string   `json:"dateTo,omitempty"`
	Keyword      string   `json:"keyword,omitempty"`
	Statuses     []string `json:"statuses,omitempty"`
}

type SavedReportChartConfig struct {
	DisplayMode string `json:"displayMode,omitempty"`
}

type SavedReport struct {
	BaseModel
	Name        string                 `gorm:"size:150;not null;index" json:"name"`
	Description string                 `gorm:"type:text" json:"description"`
	Type        SavedReportType        `gorm:"size:40;not null;index" json:"type"`
	Filters     SavedReportFilters     `gorm:"serializer:json" json:"filters"`
	ChartConfig SavedReportChartConfig `gorm:"serializer:json" json:"chartConfig"`
	CreatedByID uint                   `gorm:"not null;index" json:"createdById"`
	CreatedBy   User                   `json:"createdBy,omitempty"`
}

type ReportSubscription struct {
	BaseModel
	ReportID         uint        `gorm:"not null;uniqueIndex:idx_report_subscription_owner" json:"reportId"`
	Report           SavedReport `gorm:"foreignKey:ReportID;constraint:OnDelete:CASCADE;" json:"report,omitempty"`
	IsEnabled        bool        `gorm:"default:true;index" json:"isEnabled"`
	Schedule         string      `gorm:"size:20;not null;default:'weekly';index" json:"schedule"`
	Weekday          int         `gorm:"not null;default:1" json:"weekday"`
	Hour             int         `gorm:"not null;default:9" json:"hour"`
	Channels         []string    `gorm:"serializer:json" json:"channels"`
	RecipientUserIDs []uint      `gorm:"serializer:json" json:"recipientUserIds"`
	LastRunAt        *time.Time  `json:"lastRunAt,omitempty"`
	LastStatus       string      `gorm:"size:20" json:"lastStatus"`
	LastError        string      `gorm:"type:text" json:"lastError"`
	CreatedByID      uint        `gorm:"not null;uniqueIndex:idx_report_subscription_owner" json:"createdById"`
	CreatedBy        User        `json:"createdBy,omitempty"`
}

type ProjectRegister struct {
	BaseModel
	Type           ProjectRegisterType        `gorm:"size:24;not null;index:idx_project_register_scope,priority:2" json:"type"`
	ProjectID      uint                       `gorm:"not null;index:idx_project_register_scope,priority:1" json:"projectId"`
	Project        Project                    `json:"project,omitempty"`
	TaskID         *uint                      `gorm:"index" json:"taskId,omitempty"`
	Task           *Task                      `json:"task,omitempty"`
	Title          string                     `gorm:"size:180;not null;index" json:"title"`
	Description    string                     `gorm:"type:text" json:"description"`
	Status         ProjectRegisterStatus      `gorm:"size:24;not null;default:'open';index" json:"status"`
	Severity       ProjectRegisterSeverity    `gorm:"size:24;not null;default:'medium';index" json:"severity"`
	Probability    ProjectRegisterProbability `gorm:"size:24" json:"probability"`
	Impact         ProjectRegisterSeverity    `gorm:"size:24" json:"impact"`
	Source         string                     `gorm:"size:180" json:"source"`
	ResponsePlan   string                     `gorm:"type:text" json:"responsePlan"`
	Resolution     string                     `gorm:"type:text" json:"resolution"`
	DecisionDetail string                     `gorm:"type:text" json:"decisionDetail"`
	Background     string                     `gorm:"type:text" json:"background"`
	ImpactScope    string                     `gorm:"type:text" json:"impactScope"`
	Images         []Attachment               `gorm:"serializer:json" json:"images"`
	DueAt          *time.Time                 `gorm:"index" json:"dueAt,omitempty"`
	OwnerID        *uint                      `gorm:"index" json:"ownerId,omitempty"`
	Owner          *User                      `json:"owner,omitempty"`
	ParticipantIDs []uint                     `gorm:"serializer:json" json:"participantIds,omitempty"`
	CreatedByID    uint                       `gorm:"not null;index" json:"createdById"`
	CreatedBy      User                       `json:"createdBy,omitempty"`
	LastActivityAt *time.Time                 `gorm:"index" json:"lastActivityAt,omitempty"`
	Activities     []ProjectRegisterActivity  `gorm:"foreignKey:RegisterID" json:"activities,omitempty"`
}

type ProjectRegisterActivity struct {
	BaseModel
	RegisterID uint            `gorm:"not null;index" json:"registerId"`
	Register   ProjectRegister `gorm:"foreignKey:RegisterID;constraint:OnDelete:CASCADE;" json:"register,omitempty"`
	ActorID    uint            `gorm:"not null;index" json:"actorId"`
	Actor      User            `json:"actor,omitempty"`
	Type       string          `gorm:"size:64;not null;index" json:"type"`
	Summary    string          `gorm:"size:180;not null" json:"summary"`
	Detail     string          `gorm:"type:text" json:"detail"`
}

type SprintStatus string

const (
	SprintPlanned SprintStatus = "planned"
	SprintActive  SprintStatus = "active"
	SprintClosed  SprintStatus = "closed"
)

type Sprint struct {
	BaseModel
	Name          string       `gorm:"size:150;not null;index" json:"name"`
	Goal          string       `gorm:"type:text" json:"goal"`
	Status        SprintStatus `gorm:"size:20;not null;default:'planned';index" json:"status"`
	StartAt       *time.Time   `json:"startAt"`
	EndAt         *time.Time   `json:"endAt"`
	CapacityHours float64      `gorm:"type:decimal(10,2);default:0" json:"capacityHours"`
	CreatedByID   uint         `gorm:"not null;index" json:"createdById"`
	CreatedBy     User         `json:"createdBy,omitempty"`
}

type SprintTask struct {
	SprintID  uint      `gorm:"primaryKey;index" json:"sprintId"`
	TaskID    uint      `gorm:"primaryKey;index" json:"taskId"`
	CreatedAt time.Time `json:"createdAt"`
	Sprint    Sprint    `gorm:"foreignKey:SprintID;constraint:OnDelete:CASCADE;" json:"-"`
	Task      Task      `gorm:"foreignKey:TaskID;constraint:OnDelete:CASCADE;" json:"-"`
}

type WebhookEvent string

const (
	WebhookEventTaskStatusChanged WebhookEvent = "task_status_changed"
)

type WebhookDeliveryStatus string

const (
	WebhookDeliveryPending WebhookDeliveryStatus = "pending"
	WebhookDeliverySuccess WebhookDeliveryStatus = "success"
	WebhookDeliveryFailed  WebhookDeliveryStatus = "failed"
)

type WebhookSubscription struct {
	BaseModel
	Name               string                `gorm:"size:150;not null;index" json:"name"`
	Event              WebhookEvent          `gorm:"size:60;not null;index" json:"event"`
	URL                string                `gorm:"size:600;not null" json:"url"`
	IsEnabled          bool                  `gorm:"default:true;index" json:"isEnabled"`
	LastDeliveryStatus WebhookDeliveryStatus `gorm:"size:20" json:"lastDeliveryStatus"`
	LastDeliveredAt    *time.Time            `json:"lastDeliveredAt"`
	LastError          string                `gorm:"type:text" json:"lastError"`
	CreatedByID        uint                  `gorm:"not null;index" json:"createdById"`
	CreatedBy          User                  `json:"createdBy,omitempty"`
}

type WebhookDelivery struct {
	BaseModel
	SubscriptionID uint                  `gorm:"not null;index" json:"subscriptionId"`
	Subscription   WebhookSubscription   `gorm:"foreignKey:SubscriptionID;constraint:OnDelete:CASCADE;" json:"subscription,omitempty"`
	Event          WebhookEvent          `gorm:"size:60;not null;index" json:"event"`
	Status         WebhookDeliveryStatus `gorm:"size:20;not null;default:'pending';index" json:"status"`
	Attempts       int                   `gorm:"default:0" json:"attempts"`
	Payload        string                `gorm:"type:longtext" json:"payload"`
	ResponseStatus int                   `json:"responseStatus"`
	ErrorMessage   string                `gorm:"type:text" json:"errorMessage"`
	NextRetryAt    *time.Time            `json:"nextRetryAt"`
	DeliveredAt    *time.Time            `json:"deliveredAt"`
}

type Task struct {
	BaseModel
	TaskNo          string           `gorm:"size:64;uniqueIndex;not null" json:"taskNo"`
	Title           string           `gorm:"size:150;not null" json:"title"`
	Description     string           `gorm:"type:text" json:"description"`
	CustomField1    string           `gorm:"type:text" json:"customField1"`
	CustomField2    string           `gorm:"type:text" json:"customField2"`
	CustomField3    string           `gorm:"type:text" json:"customField3"`
	Status          TaskStatus       `gorm:"size:20;default:'pending';index" json:"status"`
	Priority        TaskPriority     `gorm:"size:20;default:'high';index" json:"priority"`
	IsMilestone     bool             `gorm:"default:false;index" json:"isMilestone"`
	ExternalVisible bool             `gorm:"default:false;index" json:"externalVisible"`
	Progress        int              `gorm:"default:0" json:"progress"`
	EstimatedHours  float64          `gorm:"type:decimal(10,2);default:0" json:"estimatedHours"`
	ActualHours     float64          `gorm:"type:decimal(10,2);default:0" json:"actualHours"`
	RemainingHours  float64          `gorm:"type:decimal(10,2);default:0" json:"remainingHours"`
	StartAt         *time.Time       `json:"startAt"`
	EndAt           *time.Time       `json:"endAt"`
	Attachment      Attachment       `gorm:"embedded;embeddedPrefix:attachment_" json:"attachment,omitempty"`
	Attachments     []Attachment     `gorm:"serializer:json" json:"attachments"`
	CreatorID       uint             `gorm:"not null;index" json:"creatorId"`
	Creator         User             `json:"creator,omitempty"`
	ProjectID       uint             `gorm:"not null;index" json:"projectId"`
	ProjectName     string           `gorm:"->;column:project_name;-:migration" json:"projectName,omitempty"`
	Project         Project          `json:"project,omitempty"`
	ParentID        *uint            `gorm:"index" json:"parentId"`
	Children        []Task           `gorm:"foreignKey:ParentID" json:"children,omitempty"`
	Assignees       []User           `gorm:"many2many:task_users;" json:"assignees,omitempty"`
	Reviewers       []User           `gorm:"many2many:task_reviewers;" json:"reviewers,omitempty"`
	Tags            []Tag            `gorm:"many2many:task_tags;" json:"tags,omitempty"`
	Dependencies    []TaskDependency `gorm:"foreignKey:TaskID" json:"dependencies,omitempty"`
	Comments        []TaskComment    `gorm:"foreignKey:TaskID" json:"comments,omitempty"`
	Activities      []TaskActivity   `gorm:"foreignKey:TaskID" json:"activities,omitempty"`
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
	TaskID          uint          `gorm:"not null;index;index:idx_task_comment_deleted_created" json:"taskId"`
	Task            Task          `gorm:"foreignKey:TaskID;constraint:OnDelete:CASCADE;" json:"-"`
	AuthorID        uint          `gorm:"not null;index" json:"authorId"`
	Author          User          `json:"author,omitempty"`
	Content         string        `gorm:"type:text;not null" json:"content"`
	Attachments     []Attachment  `gorm:"serializer:json" json:"attachments"`
	Mentions        []User        `gorm:"many2many:task_comment_mentions;" json:"mentions,omitempty"`
	Source          string        `gorm:"size:30;not null;default:'internal';index" json:"source"`
	PortalInviteID  *uint         `gorm:"index" json:"portalInviteId,omitempty"`
	PortalInvite    *PortalInvite `json:"portalInvite,omitempty"`
	ExternalName    string        `gorm:"size:120" json:"externalName,omitempty"`
	ExternalEmail   string        `gorm:"size:160" json:"externalEmail,omitempty"`
	ExternalCompany string        `gorm:"size:150" json:"externalCompany,omitempty"`
	IsDeleted       bool          `gorm:"default:false;index;index:idx_task_comment_deleted_created" json:"isDeleted"`
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

type WorkRequestChangePayload struct {
	StartAt          *time.Time   `json:"startAt,omitempty"`
	EndAt            *time.Time   `json:"endAt,omitempty"`
	Priority         TaskPriority `json:"priority,omitempty"`
	AssigneeIDs      []uint       `json:"assigneeIds,omitempty"`
	ScopeDescription string       `json:"scopeDescription,omitempty"`
}

type WorkRequest struct {
	BaseModel
	Type            WorkRequestType          `gorm:"size:20;default:'task';index" json:"type"`
	Title           string                   `gorm:"size:150;not null" json:"title"`
	Description     string                   `gorm:"type:text" json:"description"`
	Attachments     []Attachment             `gorm:"serializer:json" json:"attachments"`
	Priority        TaskPriority             `gorm:"size:20;default:'medium';index" json:"priority"`
	Status          WorkRequestStatus        `gorm:"size:20;default:'submitted';index" json:"status"`
	ProjectID       *uint                    `gorm:"index" json:"projectId,omitempty"`
	Project         *Project                 `json:"project,omitempty"`
	RequesterID     uint                     `gorm:"not null;index" json:"requesterId"`
	Requester       User                     `json:"requester,omitempty"`
	Source          string                   `gorm:"size:30;not null;default:'internal';index" json:"source"`
	PortalInviteID  *uint                    `gorm:"index" json:"portalInviteId,omitempty"`
	PortalInvite    *PortalInvite            `json:"portalInvite,omitempty"`
	ExternalName    string                   `gorm:"size:120" json:"externalName,omitempty"`
	ExternalEmail   string                   `gorm:"size:160" json:"externalEmail,omitempty"`
	ExternalCompany string                   `gorm:"size:150" json:"externalCompany,omitempty"`
	ReviewerID      *uint                    `gorm:"index" json:"reviewerId,omitempty"`
	Reviewer        *User                    `json:"reviewer,omitempty"`
	ApprovalNote    string                   `gorm:"type:text" json:"approvalNote"`
	ConvertedTaskID *uint                    `gorm:"index" json:"convertedTaskId,omitempty"`
	ConvertedTask   *Task                    `json:"convertedTask,omitempty"`
	TargetTaskID    *uint                    `gorm:"index" json:"targetTaskId,omitempty"`
	TargetTask      *Task                    `json:"targetTask,omitempty"`
	ChangePayload   WorkRequestChangePayload `gorm:"serializer:json" json:"changePayload"`
	AppliedAt       *time.Time               `gorm:"index" json:"appliedAt,omitempty"`
	AppliedByID     *uint                    `gorm:"index" json:"appliedById,omitempty"`
	AppliedBy       *User                    `json:"appliedBy,omitempty"`
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
	CallWebhook         bool   `json:"callWebhook"`
	WebhookURL          string `json:"webhookUrl"`
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
