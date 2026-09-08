package model

import "time"

// CRMSituationSourceLink records one deterministic import destination. Evidence
// references remain many-to-many and never imply that separate work should merge.
type CRMSituationSourceLink struct {
	WorkspaceID string `gorm:"type:uuid;primaryKey"`
	Kind        string `gorm:"primaryKey"`
	SourceID    string `gorm:"type:uuid;primaryKey"`
	SituationID string `gorm:"type:uuid;not null"`
	CreatedAt   time.Time
}

// TableName identifies canonical source-to-work mappings, not execution state.
func (CRMSituationSourceLink) TableName() string { return "crm_situation_source_links" }

// CRMSituationSourceInput is trusted adapter output, never a public creation DTO.
type CRMSituationSourceInput struct {
	Kind                string
	SourceID            string
	Situation           CRMSituation
	ExistingSituationID *string
	References          []CRMSituationReference
}
