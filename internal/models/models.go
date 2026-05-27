package models

import "time"

const ExportFormatVersion = "1"

const (
	JoinModeFree = "free"
	JoinModeApply = "apply"
	CardStatusActive = "active"
	CardStatusCompleted = "completed"
	MemberStatusActive = "active"
	MemberStatusPending = "pending"
)

type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

type ProjectGroup struct {
	ID          int64     `json:"id"`
	OwnerID     int64     `json:"owner_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	IsPublic    bool      `json:"is_public"`
	JoinMode    string    `json:"join_mode"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	IsOwner     bool      `json:"is_owner,omitempty"`
	IsMember    bool      `json:"is_member,omitempty"`
	JoinPending bool      `json:"join_pending,omitempty"`
}

type GroupMember struct {
	UserID   int64     `json:"user_id"`
	Username string    `json:"username"`
	Status   string    `json:"status"`
	JoinedAt time.Time `json:"joined_at"`
}

type Board struct {
	ID              int64      `json:"id"`
	ProjectGroupID  int64      `json:"project_group_id"`
	OwnerID         int64      `json:"owner_id"`
	Title           string     `json:"title"`
	ExportKey       string     `json:"export_key"`
	LastExportAt    *time.Time `json:"last_export_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type Column struct {
	ID       int64  `json:"id"`
	BoardID  int64  `json:"board_id"`
	Title    string `json:"title"`
	Position int    `json:"position"`
	IsDone   bool   `json:"is_done"`
}

type Card struct {
	ID          int64      `json:"id"`
	ColumnID    int64      `json:"column_id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Position    int       `json:"position"`
	Status      string     `json:"status"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type BoardDetail struct {
	Board   Board    `json:"board"`
	Columns []Column `json:"columns"`
	Cards   []Card   `json:"cards"`
}

type AuditLog struct {
	ID           int64     `json:"id"`
	UserID       int64     `json:"user_id"`
	Username     string    `json:"username,omitempty"`
	Action       string    `json:"action"`
	ResourceType string    `json:"resource_type"`
	ResourceID   int64     `json:"resource_id,omitempty"`
	Detail       string    `json:"detail,omitempty"`
	IP           string    `json:"ip,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

type RegisterRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type AuthResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

type CreateProjectGroupRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IsPublic    bool   `json:"is_public"`
	JoinMode    string `json:"join_mode"`
}

type UpdateProjectGroupRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IsPublic    bool   `json:"is_public"`
	JoinMode    string `json:"join_mode"`
}

type CreateBoardRequest struct {
	Title string `json:"title"`
}

type UpdateBoardRequest struct {
	Title string `json:"title"`
}

type CreateColumnRequest struct {
	Title string `json:"title"`
}

type UpdateColumnRequest struct {
	Title string `json:"title"`
}

type CreateCardRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type UpdateCardRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type MoveCardRequest struct {
	ColumnID int64 `json:"column_id"`
	Position int   `json:"position"`
}

type ExportColumn struct {
	Title    string       `json:"title"`
	Position int          `json:"position"`
	Cards    []ExportCard `json:"cards"`
}

type ExportCard struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Position    int    `json:"position"`
}

type ExportBoard struct {
	ExportKey      string         `json:"export_key"`
	Title          string         `json:"title"`
	ProjectGroup   string         `json:"project_group_name"`
	IsPublicGroup  bool           `json:"is_public_group"`
	Columns        []ExportColumn `json:"columns"`
	ExportedAt     time.Time      `json:"exported_at"`
	LastExportAt   *time.Time     `json:"last_export_at,omitempty"`
}

type BoardExportPayload struct {
	ExportVersion string      `json:"export_version"`
	ExportType    string      `json:"export_type"`
	ExportedAt    time.Time   `json:"exported_at"`
	ExportedBy    string      `json:"exported_by"`
	Board         ExportBoard `json:"board"`
}

type AllExportPayload struct {
	ExportVersion string         `json:"export_version"`
	ExportType    string         `json:"export_type"`
	ExportedAt    time.Time      `json:"exported_at"`
	ExportedBy    string         `json:"exported_by"`
	Groups        []ExportGroup  `json:"project_groups"`
	Boards        []ExportBoard  `json:"boards"`
}

type ExportGroup struct {
	ExportKey   string `json:"export_key"`
	Name        string `json:"name"`
	Description string `json:"description"`
	IsPublic    bool   `json:"is_public"`
}

type ImportBoardResult struct {
	Action  string       `json:"action"`
	BoardID int64        `json:"board_id"`
	Detail  *BoardDetail `json:"detail,omitempty"`
	Message string       `json:"message,omitempty"`
}

type ResearchOverview struct {
	Summary ResearchSummary       `json:"summary"`
	Groups  []ResearchGroupView   `json:"groups"`
}

type ResearchSummary struct {
	ActiveCards     int `json:"active_cards"`
	CompletedCards  int `json:"completed_cards"`
	ProjectGroups   int `json:"project_groups"`
	Boards          int `json:"boards"`
}

type ResearchGroupView struct {
	Group  ProjectGroup         `json:"group"`
	Boards []ResearchBoardView  `json:"boards"`
}

type ResearchBoardView struct {
	Board       Board  `json:"board"`
	ActiveCards []Card `json:"active_cards"`
}

type CardHistoryItem struct {
	Card       Card   `json:"card"`
	BoardID    int64  `json:"board_id"`
	BoardTitle string `json:"board_title"`
	GroupName  string `json:"group_name"`
}

type AgentCollaborationDay struct {
	Date          string         `json:"date"`
	Weekday       string         `json:"weekday"`
	WeekStart     string         `json:"week_start"`
	WeekStartDate string         `json:"week_start_date"`
	WeekEndDate   string         `json:"week_end_date"`
	Users         []AgentUserDay `json:"users"`
}

type AgentUserDay struct {
	Username        string          `json:"username"`
	CompletedToday  []AgentTaskItem `json:"completed_today"`
	InProgressToday []AgentTaskItem `json:"in_progress_today"`
	Tomorrow        []AgentTaskItem `json:"tomorrow"`
}

type AgentTaskItem struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Category    string `json:"category,omitempty"`
	Progress    int    `json:"progress,omitempty"`
	Summary     string `json:"summary"`
	BoardTitle  string `json:"board_title"`
	GroupName   string `json:"group_name"`
	ColumnTitle string `json:"column_title"`
}

type AgentCreateCardRequest struct {
	ColumnID    int64  `json:"column_id"`
	BoardID     int64  `json:"board_id"`
	ColumnTitle string `json:"column_title"`
	Category    string `json:"category"`
	Progress    int    `json:"progress"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type AgentUpdateCardRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Progress    int    `json:"progress"`
}

type AgentMoveCardRequest struct {
	ColumnID int64 `json:"column_id"`
	Position int   `json:"position"`
}

type AgentCapabilities struct {
	Enabled        bool     `json:"enabled"`
	DefaultActAs   string   `json:"default_act_as"`
	TaskCategories []string `json:"task_categories"`
	Endpoints      []string `json:"endpoints"`
}
