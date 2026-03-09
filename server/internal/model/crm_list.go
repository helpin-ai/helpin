package model

import "time"

// CRM list types.
const (
	CRMListTypeStatic = "static"
	CRMListTypeSmart  = "smart"
)

// CRMList represents a static or smart list of CRM objects.
type CRMList struct {
	ID             string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID    string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	Name           string    `json:"name" gorm:"not null"`
	ListType       string    `json:"list_type" gorm:"not null;default:'static'"` // static, smart
	ObjectType     string    `json:"object_type" gorm:"not null"`                // contact, company, deal
	FilterCriteria JSONB     `json:"filter_criteria" gorm:"type:jsonb;default:'{}'"`
	MemberCount    int       `json:"member_count" gorm:"not null;default:0"`
	CreatedBy      *string   `json:"created_by" gorm:"type:uuid"`
	CreatedAt      time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (CRMList) TableName() string { return "crm_lists" }

// CRMListMember represents a member of a static CRM list.
type CRMListMember struct {
	ID        string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	ListID    string    `json:"list_id" gorm:"type:uuid;not null;index"`
	ObjectID  string    `json:"object_id" gorm:"type:uuid;not null"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (CRMListMember) TableName() string { return "crm_list_members" }

// CreateCRMListRequest is the payload for creating a list.
type CreateCRMListRequest struct {
	WorkspaceID    string                 `json:"workspace_id"`
	Name           string                 `json:"name"`
	ListType       string                 `json:"list_type"`
	ObjectType     string                 `json:"object_type"`
	FilterCriteria map[string]interface{} `json:"filter_criteria"`
}

// UpdateCRMListRequest is the payload for updating a list.
type UpdateCRMListRequest struct {
	Name           *string                `json:"name"`
	FilterCriteria map[string]interface{} `json:"filter_criteria"`
}

// CRMListFilters applies filters when listing CRM lists.
type CRMListFilters struct {
	ObjectType *string
	ListType   *string
	Search     *string
}

// AddCRMListMemberRequest is the payload for adding a member to a static list.
type AddCRMListMemberRequest struct {
	ObjectID string `json:"object_id"`
}
