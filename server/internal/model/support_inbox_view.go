package model

import "time"

// SupportInboxViewFilters stores a saved support inbox filter preset.
type SupportInboxViewFilters = ViewFilters

const (
	SupportInboxViewTypeCustom  = "custom"
	SupportInboxViewTypeDefault = "default"
	SupportInboxViewTypeTeam    = "team"
)

// SupportInboxView represents a saved support inbox view.
type SupportInboxView struct {
	ID          string                  `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string                  `json:"workspace_id" gorm:"type:uuid;not null;index"`
	Name        string                  `json:"name" gorm:"not null"`
	Filters     SupportInboxViewFilters `json:"filters" gorm:"type:jsonb;not null;default:'{}'"`
	IsShared    bool                    `json:"is_shared" gorm:"not null;default:false"`
	ViewType    string                  `json:"view_type" gorm:"not null;default:'custom';index"`
	ViewKey     *string                 `json:"view_key,omitempty" gorm:"index"`
	CreatedBy   string                  `json:"created_by" gorm:"type:uuid;not null;index"`
	CreatedAt   time.Time               `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time               `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SupportInboxView) TableName() string { return "support_inbox_views" }

type CreateSupportInboxViewRequest struct {
	Name     string                  `json:"name"`
	Filters  SupportInboxViewFilters `json:"filters"`
	IsShared bool                    `json:"is_shared"`
}

type UpdateSupportInboxViewRequest struct {
	Name     *string                  `json:"name"`
	Filters  *SupportInboxViewFilters `json:"filters"`
	IsShared *bool                    `json:"is_shared"`
}

type UpdateSupportInboxBuiltinViewRequest struct {
	ViewKey string                  `json:"view_key"`
	Filters SupportInboxViewFilters `json:"filters"`
}

type SupportInboxViewCount struct {
	ViewID      string `json:"view_id"`
	TotalCount  int    `json:"total_count"`
	UnreadCount int    `json:"unread_count"`
}
