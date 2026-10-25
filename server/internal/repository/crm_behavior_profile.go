package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// CRMBehaviorProfileRepository stores product-usage profiles for CRM records.
type CRMBehaviorProfileRepository struct {
	db *gorm.DB
}

// NewCRMBehaviorProfileRepository creates the repository.
func NewCRMBehaviorProfileRepository(db *gorm.DB) *CRMBehaviorProfileRepository {
	return &CRMBehaviorProfileRepository{db: db}
}

// CRMBehaviorIdentityLink is one identity link that names a CRM record.
type CRMBehaviorIdentityLink struct {
	AnonymousID    string
	ExternalUserID *string
	ContactID      *string
	CompanyID      *string
}

// CRMBehaviorIdentityIndex holds the workspace identifiers product events can
// be matched on.
type CRMBehaviorIdentityIndex struct {
	Links []CRMBehaviorIdentityLink
	// ContactEmails maps a lower-cased email to the contacts carrying it.
	ContactEmails map[string][]string
	ContactIDs    map[string]bool
	// CompanyKeys maps a company id or external id to its company.
	CompanyKeys map[string]string
	// CompanyKeyMethods records whether a CompanyKeys entry is an id or external id.
	CompanyKeyMethods map[string]string
	// CompanyCount is the number of companies in the workspace.
	CompanyCount int
}

// LoadIdentityIndex reads the identifiers used to tie events to CRM records.
// Links are ordered most recently created first.
func (r *CRMBehaviorProfileRepository) LoadIdentityIndex(ctx context.Context, workspaceID string) (*CRMBehaviorIdentityIndex, error) {
	index := &CRMBehaviorIdentityIndex{
		ContactEmails: map[string][]string{}, ContactIDs: map[string]bool{},
		CompanyKeys: map[string]string{}, CompanyKeyMethods: map[string]string{},
	}
	db := r.db.WithContext(ctx)
	if err := db.Model(&model.CRMIdentityLink{}).
		Select("anonymous_id", "external_user_id", "contact_id", "company_id").
		Where("workspace_id = ? AND (contact_id IS NOT NULL OR company_id IS NOT NULL)", workspaceID).
		Order("created_at DESC").
		Find(&index.Links).Error; err != nil {
		return nil, fmt.Errorf("load identity links: %w", err)
	}
	var contacts []struct {
		ID    string
		Email *string
	}
	if err := db.Model(&model.CRMContact{}).Select("id", "email").
		Where("workspace_id = ?", workspaceID).Find(&contacts).Error; err != nil {
		return nil, fmt.Errorf("load contact identities: %w", err)
	}
	for _, contact := range contacts {
		index.ContactIDs[strings.ToLower(contact.ID)] = true
		if contact.Email == nil {
			continue
		}
		if email := strings.ToLower(strings.TrimSpace(*contact.Email)); email != "" {
			index.ContactEmails[email] = append(index.ContactEmails[email], contact.ID)
		}
	}
	var companies []struct {
		ID         string
		ExternalID *string
	}
	if err := db.Model(&model.CRMCompany{}).Select("id", "external_id").
		Where("workspace_id = ?", workspaceID).Find(&companies).Error; err != nil {
		return nil, fmt.Errorf("load company identities: %w", err)
	}
	index.CompanyCount = len(companies)
	for _, company := range companies {
		index.CompanyKeys[strings.ToLower(company.ID)] = company.ID
		index.CompanyKeyMethods[strings.ToLower(company.ID)] = model.CRMBehaviorMatchRecordID
	}
	// External ids win over ids on collision: customers send their own ids.
	for _, company := range companies {
		if company.ExternalID == nil || strings.TrimSpace(*company.ExternalID) == "" {
			continue
		}
		key := strings.TrimSpace(*company.ExternalID)
		index.CompanyKeys[key] = company.ID
		index.CompanyKeyMethods[key] = model.CRMBehaviorMatchExternalID
	}
	return index, nil
}

// ReplaceWorkspaceProfiles swaps the workspace's profiles for profiles in one
// transaction, so a record whose evidence aged out loses its profile.
func (r *CRMBehaviorProfileRepository) ReplaceWorkspaceProfiles(ctx context.Context, workspaceID string, profiles []model.CRMBehaviorProfile) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Serialize replacements of one workspace: a concurrent replace would
		// miss this one's uncommitted rows in its DELETE and then collide on
		// the primary key when inserting.
		if tx.Dialector.Name() == "postgres" {
			if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", "crm_behavior_profiles:"+workspaceID).Error; err != nil {
				return fmt.Errorf("lock behavior profiles: %w", err)
			}
		}
		if err := tx.Where("workspace_id = ?", workspaceID).Delete(&model.CRMBehaviorProfile{}).Error; err != nil {
			return fmt.Errorf("clear behavior profiles: %w", err)
		}
		if len(profiles) == 0 {
			return nil
		}
		if err := tx.CreateInBatches(profiles, 500).Error; err != nil {
			return fmt.Errorf("write behavior profiles: %w", err)
		}
		return nil
	})
}

// DeleteProfilesOutsideWorkspaces removes the profiles of every workspace
// not in keep, so a workspace that stops collecting events does not keep
// filtering on frozen data. An empty keep removes every profile.
func (r *CRMBehaviorProfileRepository) DeleteProfilesOutsideWorkspaces(ctx context.Context, keep []string) (int64, error) {
	query := r.db.WithContext(ctx)
	if len(keep) > 0 {
		query = query.Where("workspace_id NOT IN ?", keep)
	} else {
		query = query.Where("1 = 1")
	}
	result := query.Delete(&model.CRMBehaviorProfile{})
	if result.Error != nil {
		return 0, fmt.Errorf("delete stale behavior profiles: %w", result.Error)
	}
	return result.RowsAffected, nil
}

// Coverage counts records of entityType with and without a profile.
func (r *CRMBehaviorProfileRepository) Coverage(ctx context.Context, workspaceID, entityType string) (*model.CRMBehaviorCoverage, error) {
	table := "crm_contacts"
	if entityType == model.CRMFieldObjectCompany {
		table = "crm_companies"
	}
	coverage := &model.CRMBehaviorCoverage{}
	db := r.db.WithContext(ctx)
	if err := db.Table(table).Where("workspace_id = ?", workspaceID).Count(&coverage.TotalRecords).Error; err != nil {
		return nil, fmt.Errorf("count %s: %w", table, err)
	}
	var stats struct {
		Records int64
		Latest  *string
	}
	// MAX is read as text: SQLite returns timestamps from aggregates as strings.
	if err := db.Model(&model.CRMBehaviorProfile{}).
		Select("COUNT(*) AS records, CAST(MAX(computed_at) AS TEXT) AS latest").
		Where("workspace_id = ? AND entity_type = ?", workspaceID, entityType).
		Scan(&stats).Error; err != nil {
		return nil, fmt.Errorf("count behavior profiles: %w", err)
	}
	coverage.RecordsWithBehavior = stats.Records
	if stats.Latest != nil {
		if latest, ok := parseCRMBehaviorTime(*stats.Latest); ok {
			coverage.LastComputedAt = &latest
		}
	}
	return coverage, nil
}

func parseCRMBehaviorTime(value string) (time.Time, bool) {
	for _, layout := range []string{
		time.RFC3339Nano, "2006-01-02 15:04:05.999999999-07:00", "2006-01-02 15:04:05.999999999-07",
		"2006-01-02 15:04:05.999999999Z07:00", "2006-01-02 15:04:05.999999999",
	} {
		if parsed, err := time.Parse(layout, strings.TrimSpace(value)); err == nil {
			return parsed.UTC(), true
		}
	}
	return time.Time{}, false
}

// SummariesByID returns the behavior summaries for the given records of one
// type in the workspace, keyed by record id.
func (r *CRMBehaviorProfileRepository) SummariesByID(ctx context.Context, workspaceID, entityType string, ids []string) (map[string]model.CRMBehaviorSummary, error) {
	result := map[string]model.CRMBehaviorSummary{}
	if len(ids) == 0 {
		return result, nil
	}
	var profiles []model.CRMBehaviorProfile
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND entity_type = ? AND entity_id IN ?", workspaceID, entityType, ids).
		Find(&profiles).Error; err != nil {
		return nil, fmt.Errorf("load behavior profiles: %w", err)
	}
	for _, profile := range profiles {
		methods := []string{}
		if profile.MatchMethods != "" {
			methods = strings.Split(profile.MatchMethods, ",")
		}
		result[profile.EntityID] = model.CRMBehaviorSummary{
			FirstSeenAt: profile.FirstSeenAt, LastSeenAt: profile.LastSeenAt,
			Sessions30d: profile.Sessions30d, MatchMethods: methods, ComputedAt: profile.ComputedAt,
		}
	}
	return result, nil
}
