//go:build ee

package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Demo workspace build states.
const (
	DemoStatusQueued   = "queued"
	DemoStatusBuilding = "building"
	DemoStatusReady    = "ready"
	DemoStatusFailed   = "failed"
)

// DemoWorkspace tracks one demo build: a workspace created for a company and
// filled from that company's public help center.
type DemoWorkspace struct {
	ID string `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	// HomeWorkspaceID is the workspace that asked for the demo and holds the
	// company record.
	HomeWorkspaceID string          `json:"home_workspace_id" gorm:"type:uuid;not null;uniqueIndex:idx_demo_workspaces_home_company,priority:1"`
	CompanyID       string          `json:"company_id" gorm:"type:uuid;not null;uniqueIndex:idx_demo_workspaces_home_company,priority:2"`
	CompanyName     string          `json:"company_name" gorm:"not null"`
	Domain          string          `json:"domain" gorm:"not null;default:''"`
	HelpCenterURL   string          `json:"help_center_url" gorm:"not null"`
	Status          string          `json:"status" gorm:"not null;default:'queued'"`
	Step            string          `json:"step" gorm:"not null;default:'organization'"`
	Error           *string         `json:"error,omitempty"`
	OrganizationID  *string         `json:"organization_id,omitempty" gorm:"type:uuid"`
	WorkspaceID     *string         `json:"workspace_id,omitempty" gorm:"type:uuid"`
	SpaceID         *string         `json:"space_id,omitempty" gorm:"type:uuid"`
	ImportJobID     *string         `json:"import_job_id,omitempty" gorm:"type:uuid"`
	Subdomain       *string         `json:"subdomain,omitempty"`
	DemoURL         *string         `json:"demo_url,omitempty"`
	Report          json.RawMessage `json:"report" gorm:"type:jsonb;not null;default:'{}'"`
	RequestedBy     string          `json:"requested_by" gorm:"type:uuid;not null"`
	Attempts        int             `json:"attempts" gorm:"not null;default:0"`
	LeaseUntil      *time.Time      `json:"-"`
	StartedAt       *time.Time      `json:"started_at,omitempty"`
	CompletedAt     *time.Time      `json:"completed_at,omitempty"`
	CreatedAt       time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt       time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName returns the demo workspace table name.
func (DemoWorkspace) TableName() string { return "demo_workspaces" }

// Finished reports whether the build has ended, successfully or not.
func (d *DemoWorkspace) Finished() bool {
	return d.Status == DemoStatusReady || d.Status == DemoStatusFailed
}

// DemoRepository stores demo workspace builds.
type DemoRepository struct {
	db *gorm.DB
}

// NewDemoRepository creates a DemoRepository.
func NewDemoRepository(db *gorm.DB) *DemoRepository {
	return &DemoRepository{db: db}
}

// CreateOrGet stores a new build, or returns the one that already exists for
// the same company. created reports which happened.
func (r *DemoRepository) CreateOrGet(ctx context.Context, demo *DemoWorkspace) (stored *DemoWorkspace, created bool, err error) {
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "home_workspace_id"}, {Name: "company_id"}},
		DoNothing: true,
	}).Create(demo)
	if result.Error != nil {
		return nil, false, fmt.Errorf("create demo workspace: %w", result.Error)
	}
	if result.RowsAffected == 1 {
		return demo, true, nil
	}
	existing, err := r.GetByCompany(ctx, demo.HomeWorkspaceID, demo.CompanyID)
	if err != nil {
		return nil, false, err
	}
	if existing == nil {
		return nil, false, fmt.Errorf("create demo workspace: the existing build disappeared")
	}
	return existing, false, nil
}

// GetByID returns a build of the given home workspace, or nil.
func (r *DemoRepository) GetByID(ctx context.Context, homeWorkspaceID, id string) (*DemoWorkspace, error) {
	return r.first(ctx, "home_workspace_id = ? AND id = ?", homeWorkspaceID, id)
}

// GetByCompany returns the build for a company, or nil.
func (r *DemoRepository) GetByCompany(ctx context.Context, homeWorkspaceID, companyID string) (*DemoWorkspace, error) {
	return r.first(ctx, "home_workspace_id = ? AND company_id = ?", homeWorkspaceID, companyID)
}

func (r *DemoRepository) first(ctx context.Context, query string, args ...any) (*DemoWorkspace, error) {
	var demo DemoWorkspace
	if err := r.db.WithContext(ctx).Where(query, args...).First(&demo).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get demo workspace: %w", err)
	}
	return &demo, nil
}

// CountUnfinished counts the builds of a home workspace that are still
// queued or building.
func (r *DemoRepository) CountUnfinished(ctx context.Context, homeWorkspaceID string) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&DemoWorkspace{}).
		Where("home_workspace_id = ? AND status IN ?", homeWorkspaceID, []string{DemoStatusQueued, DemoStatusBuilding}).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count unfinished demo workspaces: %w", err)
	}
	return count, nil
}

// ClaimNext leases the oldest unfinished build nobody is working on, so two
// API instances never advance the same build. It returns nil when there is
// none.
func (r *DemoRepository) ClaimNext(ctx context.Context, now time.Time, lease time.Duration) (*DemoWorkspace, error) {
	var claimed *DemoWorkspace
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var demo DemoWorkspace
		err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status IN ? AND (lease_until IS NULL OR lease_until <= ?)", []string{DemoStatusQueued, DemoStatusBuilding}, now).
			Order("created_at ASC").
			First(&demo).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		until := now.Add(lease)
		if err := tx.Model(&DemoWorkspace{}).Where("id = ?", demo.ID).Update("lease_until", until).Error; err != nil {
			return err
		}
		demo.LeaseUntil = &until
		claimed = &demo
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("claim demo workspace: %w", err)
	}
	return claimed, nil
}

// Save stores every field of a build.
func (r *DemoRepository) Save(ctx context.Context, demo *DemoWorkspace) error {
	if err := r.db.WithContext(ctx).Save(demo).Error; err != nil {
		return fmt.Errorf("save demo workspace: %w", err)
	}
	return nil
}

// Requeue puts a failed build back in the queue from the step it stopped at.
// The build's clock restarts, so a build continued days later is not judged
// against the time limit of its first run.
func (r *DemoRepository) Requeue(ctx context.Context, homeWorkspaceID, id string) error {
	result := r.db.WithContext(ctx).Model(&DemoWorkspace{}).
		Where("home_workspace_id = ? AND id = ? AND status = ?", homeWorkspaceID, id, DemoStatusFailed).
		Updates(map[string]any{"status": DemoStatusQueued, "error": nil, "attempts": 0, "lease_until": nil, "started_at": nil, "completed_at": nil, "updated_at": time.Now()})
	if result.Error != nil {
		return fmt.Errorf("requeue demo workspace: %w", result.Error)
	}
	return nil
}
