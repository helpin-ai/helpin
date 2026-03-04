package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// ViewFilters is a JSONB map for storing view filter criteria.
type ViewFilters map[string]string

func (f ViewFilters) Value() (driver.Value, error) {
	if f == nil {
		return "{}", nil
	}
	b, err := json.Marshal(f)
	if err != nil {
		return nil, fmt.Errorf("marshal view filters: %w", err)
	}
	return string(b), nil
}

func (f *ViewFilters) Scan(src interface{}) error {
	if src == nil {
		*f = ViewFilters{}
		return nil
	}
	var data []byte
	switch v := src.(type) {
	case string:
		data = []byte(v)
	case []byte:
		data = v
	default:
		return fmt.Errorf("unsupported type for ViewFilters: %T", src)
	}
	return json.Unmarshal(data, f)
}

// PMView represents a saved filter view for the kanban board.
type PMView struct {
	ID          string      `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string      `json:"workspace_id" gorm:"type:uuid;not null;index"`
	Name        string      `json:"name" gorm:"not null"`
	Filters     ViewFilters `json:"filters" gorm:"type:jsonb;not null;default:'{}'"`
	IsShared    bool        `json:"is_shared" gorm:"not null;default:false"`
	IsPinned    bool        `json:"is_pinned" gorm:"not null;default:false"`
	Position    int         `json:"position" gorm:"not null;default:0"`
	CreatedBy   string      `json:"created_by" gorm:"type:uuid;not null;index"`
	CreatedAt   time.Time   `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time   `json:"updated_at" gorm:"autoUpdateTime"`
}

func (PMView) TableName() string { return "pm_views" }

// CreateViewRequest is the payload for creating a view.
type CreateViewRequest struct {
	Name     string      `json:"name"`
	Filters  ViewFilters `json:"filters"`
	IsShared bool        `json:"is_shared"`
	IsPinned bool        `json:"is_pinned"`
}

// UpdateViewRequest is the payload for updating a view.
type UpdateViewRequest struct {
	Name     *string      `json:"name"`
	Filters  *ViewFilters `json:"filters"`
	IsShared *bool        `json:"is_shared"`
	IsPinned *bool        `json:"is_pinned"`
	Position *int         `json:"position"`
}
