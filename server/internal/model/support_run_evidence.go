package model

import "time"

// SupportRunEvidence snapshots one knowledge chunk returned to a support
// chat run by the search_knowledge tool. support.send_reply re-validates the
// agent's citations against these rows (grounding + numeric checks) without
// re-running retrieval, so the enforcement is over exactly what the agent
// saw. Rows are deleted when the run reaches a terminal state.
type SupportRunEvidence struct {
	ID            string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID   string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	RunID         string    `json:"run_id" gorm:"type:uuid;not null;uniqueIndex:idx_support_run_evidence_run_ref,priority:1"`
	EvidenceID    string    `json:"evidence_id" gorm:"not null;uniqueIndex:idx_support_run_evidence_run_ref,priority:2"`
	ReferenceID   string    `json:"reference_id"`
	SourceType    string    `json:"source_type" gorm:"not null"`
	SourceID      string    `json:"source_id"`
	DocumentID    string    `json:"document_id"`
	Title         string    `json:"title"`
	URL           string    `json:"url"`
	IsInternal    bool      `json:"is_internal" gorm:"not null;default:false"`
	Content       string    `json:"content" gorm:"not null"`
	LexicalScore  float64   `json:"lexical_score" gorm:"not null;default:0"`
	VectorScore   float64   `json:"vector_score" gorm:"not null;default:0"`
	CombinedScore float64   `json:"combined_score" gorm:"not null;default:0"`
	CreatedAt     time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// TableName returns the support run evidence table name.
func (SupportRunEvidence) TableName() string { return "support_run_evidence" }
