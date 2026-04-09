package model

import "time"

type ModuleID string

const (
	ModulePM      ModuleID = "pm"
	ModuleDocs    ModuleID = "docs"
	ModuleCRM     ModuleID = "crm"
	ModuleSupport ModuleID = "support"
)

var allWorkspaceModules = []ModuleID{
	ModulePM,
	ModuleDocs,
	ModuleCRM,
	ModuleSupport,
}

var managedWorkspaceModules = []ModuleID{
	ModuleCRM,
	ModuleSupport,
}

func AllWorkspaceModules() []ModuleID {
	modules := make([]ModuleID, len(allWorkspaceModules))
	copy(modules, allWorkspaceModules)
	return modules
}

func ManagedWorkspaceModules() []ModuleID {
	modules := make([]ModuleID, len(managedWorkspaceModules))
	copy(modules, managedWorkspaceModules)
	return modules
}

func IsValidWorkspaceModule(module ModuleID) bool {
	for _, candidate := range allWorkspaceModules {
		if candidate == module {
			return true
		}
	}
	return false
}

func IsManagedWorkspaceModule(module ModuleID) bool {
	for _, candidate := range managedWorkspaceModules {
		if candidate == module {
			return true
		}
	}
	return false
}

type ModuleGrantSubjectType string

const (
	ModuleGrantSubjectTeam            ModuleGrantSubjectType = "team"
	ModuleGrantSubjectWorkspaceMember ModuleGrantSubjectType = "workspace_member"
)

func IsValidModuleGrantSubjectType(subjectType ModuleGrantSubjectType) bool {
	switch subjectType {
	case ModuleGrantSubjectTeam, ModuleGrantSubjectWorkspaceMember:
		return true
	default:
		return false
	}
}

type ModuleGrantAccessLevel string

const (
	ModuleGrantAccessLevelMember ModuleGrantAccessLevel = "member"
)

type WorkspaceModuleGrant struct {
	ID          string                 `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string                 `json:"workspace_id" gorm:"type:uuid;not null;uniqueIndex:idx_workspace_module_grants_unique,priority:1;index"`
	Module      ModuleID               `json:"module" gorm:"type:text;not null;uniqueIndex:idx_workspace_module_grants_unique,priority:2;index"`
	SubjectType ModuleGrantSubjectType `json:"subject_type" gorm:"type:text;not null;uniqueIndex:idx_workspace_module_grants_unique,priority:3;index"`
	SubjectID   string                 `json:"subject_id" gorm:"type:uuid;not null;uniqueIndex:idx_workspace_module_grants_unique,priority:4;index"`
	AccessLevel ModuleGrantAccessLevel `json:"access_level" gorm:"type:text;not null;default:'member'"`
	CreatedByID *string                `json:"created_by_id,omitempty" gorm:"type:uuid"`
	CreatedAt   time.Time              `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time              `json:"updated_at" gorm:"autoUpdateTime"`
}

func (WorkspaceModuleGrant) TableName() string { return "workspace_module_grants" }

type CreateWorkspaceModuleGrantRequest struct {
	WorkspaceID string                 `json:"workspace_id"`
	Module      ModuleID               `json:"module"`
	SubjectType ModuleGrantSubjectType `json:"subject_type"`
	SubjectID   string                 `json:"subject_id"`
}
