package model

import "time"

const (
	WorkspaceSkillSourceBuiltIn   = "built_in"
	WorkspaceSkillSourceImported  = "imported"
	WorkspaceSkillSourceWorkspace = "workspace"
)

type WorkspaceSkill struct {
	ID                string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID       string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	SourceKind        string    `json:"source_kind" gorm:"not null"`
	SourceRuntime     *string   `json:"source_runtime,omitempty"`
	Key               string    `json:"key" gorm:"not null"`
	VersionKey        string    `json:"version_key" gorm:"not null"`
	Title             string    `json:"title" gorm:"not null"`
	Description       *string   `json:"description,omitempty"`
	Instructions      string    `json:"instructions" gorm:"type:text;not null"`
	RequiredTools     JSONBlob  `json:"required_tools" gorm:"type:jsonb;not null;default:'[]'"`
	SupportedRuntimes JSONBlob  `json:"supported_runtimes" gorm:"type:jsonb;not null;default:'[]'"`
	InterfaceConfig   JSONBlob  `json:"interface_config" gorm:"type:jsonb;not null;default:'{}'"`
	PolicyConfig      JSONBlob  `json:"policy_config" gorm:"type:jsonb;not null;default:'{}'"`
	PackageObjectKey  string    `json:"package_object_key" gorm:"not null"`
	PackageFileName   string    `json:"package_file_name" gorm:"not null"`
	PackageSize       int64     `json:"package_size" gorm:"not null;default:0"`
	PackageChecksum   string    `json:"package_checksum" gorm:"not null"`
	IsArchived        bool      `json:"is_archived" gorm:"not null;default:false"`
	CreatedBy         *string   `json:"created_by,omitempty" gorm:"type:uuid"`
	CreatedAt         time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt         time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (WorkspaceSkill) TableName() string { return "workspace_skills" }

type CreateWorkspaceSkillRequest struct {
	Key               string   `json:"key"`
	Title             *string  `json:"title,omitempty"`
	Description       string   `json:"description"`
	Instructions      string   `json:"instructions"`
	RequiredTools     []string `json:"required_tools,omitempty"`
	SupportedRuntimes []string `json:"supported_runtimes,omitempty"`
	SourceRuntime     *string  `json:"source_runtime,omitempty"`
}

type UpdateWorkspaceSkillRequest struct {
	Key               *string  `json:"key,omitempty"`
	Title             *string  `json:"title,omitempty"`
	Description       *string  `json:"description,omitempty"`
	Instructions      *string  `json:"instructions,omitempty"`
	RequiredTools     []string `json:"required_tools,omitempty"`
	SupportedRuntimes []string `json:"supported_runtimes,omitempty"`
	SourceRuntime     *string  `json:"source_runtime,omitempty"`
}

type WorkspaceSkillResponse struct {
	ID                string    `json:"id"`
	WorkspaceID       string    `json:"workspace_id"`
	SourceKind        string    `json:"source_kind"`
	SourceRuntime     *string   `json:"source_runtime,omitempty"`
	Key               string    `json:"key"`
	VersionKey        string    `json:"version_key"`
	Title             string    `json:"title"`
	Description       string    `json:"description"`
	Instructions      string    `json:"instructions"`
	RequiredTools     []string  `json:"required_tools,omitempty"`
	SupportedRuntimes []string  `json:"supported_runtimes,omitempty"`
	PackageFileName   string    `json:"package_file_name"`
	PackageSize       int64     `json:"package_size"`
	IsArchived        bool      `json:"is_archived"`
	CreatedBy         *string   `json:"created_by,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}
