package model

import (
	"errors"
	"time"
)

// Sample data entity types recorded in sample_data_items. The values double as
// the keys of SampleDataStatus.Counts.
const (
	SampleEntitySupportConversation = "support_conversation"
	SampleEntityPMTask              = "pm_task"
	SampleEntityPMEpic              = "pm_epic"
	SampleEntityWorkspaceTeam       = "workspace_team"
	SampleEntityDocsDocument        = "docs_document"
	SampleEntityDocsSpace           = "docs_space"
	SampleEntityCRMDeal             = "crm_deal"
	SampleEntityCRMContact          = "crm_contact"
	SampleEntityCRMCompany          = "crm_company"
	SampleEntityCRMPipeline         = "crm_pipeline"
)

// SampleEntityRemovalOrder lists sample entity types in the order they must be
// removed so that dependents are always deleted before the rows they point to.
var SampleEntityRemovalOrder = []string{
	SampleEntitySupportConversation,
	SampleEntityPMTask,
	SampleEntityPMEpic,
	SampleEntityDocsDocument,
	SampleEntityDocsSpace,
	SampleEntityCRMDeal,
	SampleEntityCRMContact,
	SampleEntityCRMCompany,
	SampleEntityCRMPipeline,
	SampleEntityWorkspaceTeam,
}

// ErrSampleDataAlreadyLoaded is returned when a workspace already holds sample data.
var ErrSampleDataAlreadyLoaded = errors.New("sample data is already loaded")

// ErrSampleDataNoModules is returned when none of the seedable modules is enabled.
var ErrSampleDataNoModules = errors.New("no module that supports sample data is enabled")

// SampleDataItem records one row created by the sample data loader.
type SampleDataItem struct {
	ID          string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string    `json:"workspace_id" gorm:"type:uuid;not null"`
	EntityType  string    `json:"entity_type" gorm:"not null"`
	EntityID    string    `json:"entity_id" gorm:"type:uuid;not null"`
	CreatedBy   *string   `json:"created_by" gorm:"type:uuid"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// TableName returns the sample data tracking table.
func (SampleDataItem) TableName() string { return "sample_data_items" }

// SampleDataStatus describes the sample data currently loaded in a workspace.
type SampleDataStatus struct {
	Loaded   bool             `json:"loaded"`
	LoadedAt *time.Time       `json:"loaded_at,omitempty"`
	Counts   map[string]int64 `json:"counts"`
	// Modules lists the modules sample data can be loaded for, given the
	// caller's access and the modules enabled on this deployment.
	Modules []ModuleID `json:"modules"`
	// Retained counts sample containers kept by a removal because they now
	// hold records people added (for example a doc added to the sample space).
	Retained map[string]int64 `json:"retained,omitempty"`
}
