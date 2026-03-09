package model

import "time"

// CRM import sources.
const (
	CRMImportSourceCSV     = "csv"
	CRMImportSourceHubspot = "hubspot"
)

// CRM import statuses.
const (
	CRMImportStatusPending    = "pending"
	CRMImportStatusProcessing = "processing"
	CRMImportStatusCompleted  = "completed"
	CRMImportStatusFailed     = "failed"
)

// CRMImportJob represents a CRM data import job.
type CRMImportJob struct {
	ID            string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID   string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	Source        string    `json:"source" gorm:"not null;default:'csv'"`
	Status        string    `json:"status" gorm:"not null;default:'pending'"`
	ObjectType    string    `json:"object_type" gorm:"not null"` // contact, company, deal
	FileURL       *string   `json:"file_url"`
	ColumnMapping JSONB     `json:"column_mapping" gorm:"type:jsonb;default:'{}'"`
	TotalRows     int       `json:"total_rows" gorm:"not null;default:0"`
	ProcessedRows int       `json:"processed_rows" gorm:"not null;default:0"`
	CreatedRows   int       `json:"created_rows" gorm:"not null;default:0"`
	UpdatedRows   int       `json:"updated_rows" gorm:"not null;default:0"`
	ErrorCount    int       `json:"error_count" gorm:"not null;default:0"`
	ErrorLog      JSONB     `json:"error_log" gorm:"type:jsonb;default:'[]'"`
	CreatedBy     *string   `json:"created_by" gorm:"type:uuid"`
	CreatedAt     time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt     time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (CRMImportJob) TableName() string { return "crm_import_jobs" }

// CreateCRMImportRequest is the payload for creating an import job.
type CreateCRMImportRequest struct {
	WorkspaceID   string                 `json:"workspace_id"`
	Source        string                 `json:"source"`
	ObjectType    string                 `json:"object_type"`
	FileURL       *string                `json:"file_url"`
	ColumnMapping map[string]interface{} `json:"column_mapping"`
	TotalRows     int                    `json:"total_rows"`
}

// ImportColumnMapping describes how CSV columns map to CRM fields.
type ImportColumnMapping struct {
	CSVColumn string `json:"csv_column"`
	CRMField  string `json:"crm_field"`
	IsCustom  bool   `json:"is_custom"`
}

// ProcessCRMImportRequest is the payload for processing an import job.
type ProcessCRMImportRequest struct {
	ColumnMapping []ImportColumnMapping `json:"column_mapping"`
	CSVData       [][]string           `json:"csv_data"`
}
