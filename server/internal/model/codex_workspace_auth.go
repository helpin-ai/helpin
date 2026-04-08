package model

import "time"

// CodexWorkspaceAuth stores the canonical encrypted Codex auth.json payload
// for a workspace/runtime auth scope. The per-run .codex/auth.json file is
// materialized from this record when a run starts.
type CodexWorkspaceAuth struct {
	ID                string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID       string    `json:"workspace_id" gorm:"type:uuid;not null;uniqueIndex:idx_codex_workspace_auth_scope,priority:1"`
	Provider          string    `json:"provider" gorm:"not null;uniqueIndex:idx_codex_workspace_auth_scope,priority:2"`
	AuthMode          string    `json:"auth_mode" gorm:"not null;uniqueIndex:idx_codex_workspace_auth_scope,priority:3"`
	AuthJSONEncrypted string    `json:"-" gorm:"column:auth_json_encrypted;type:text;not null"`
	CreatedAt         time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt         time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (CodexWorkspaceAuth) TableName() string { return "codex_workspace_auths" }
