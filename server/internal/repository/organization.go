package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// OrganizationRepository handles database operations for organizations.
type OrganizationRepository struct {
	db *gorm.DB
}

// NewOrganizationRepository creates a new OrganizationRepository.
func NewOrganizationRepository(db *gorm.DB) *OrganizationRepository {
	return &OrganizationRepository{db: db}
}

// Create inserts a new organization.
func (r *OrganizationRepository) Create(ctx context.Context, name, slug, ownerID string, logoURL *string) (*model.Organization, error) {
	org := &model.Organization{
		Name:    name,
		Slug:    slug,
		OwnerID: ownerID,
		LogoURL: logoURL,
	}
	if err := r.db.WithContext(ctx).Create(org).Error; err != nil {
		return nil, fmt.Errorf("create organization: %w", err)
	}
	return org, nil
}

// List returns all organizations a user is a member of, along with their role.
func (r *OrganizationRepository) List(ctx context.Context, userID string) ([]model.OrganizationWithRole, error) {
	var results []model.OrganizationWithRole
	err := r.db.WithContext(ctx).
		Table("organizations o").
		Select("o.id, o.name, o.slug, o.owner_id, o.logo_url, o.created_at, o.updated_at, om.role").
		Joins("JOIN organization_members om ON o.id = om.organization_id").
		Where("om.user_id = ?", userID).
		Order("o.created_at DESC").
		Scan(&results).Error
	if err != nil {
		return nil, fmt.Errorf("list organizations: %w", err)
	}
	return results, nil
}

// GetByID returns an organization by its ID.
func (r *OrganizationRepository) GetByID(ctx context.Context, id string) (*model.Organization, error) {
	org := &model.Organization{}
	err := r.db.WithContext(ctx).Where("id = ?", id).First(org).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get organization by id: %w", err)
	}
	return org, nil
}

// GetBySlug returns an organization by its slug.
func (r *OrganizationRepository) GetBySlug(ctx context.Context, slug string) (*model.Organization, error) {
	org := &model.Organization{}
	err := r.db.WithContext(ctx).Where("slug = ?", slug).First(org).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get organization by slug: %w", err)
	}
	return org, nil
}

// Update modifies organization fields.
func (r *OrganizationRepository) Update(ctx context.Context, id string, name, logoURL *string) (*model.Organization, error) {
	updates := map[string]any{}
	if name != nil {
		updates["name"] = *name
	}
	if logoURL != nil {
		updates["logo_url"] = *logoURL
	}

	if err := r.db.WithContext(ctx).Model(&model.Organization{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update organization: %w", err)
	}

	org := &model.Organization{}
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(org).Error; err != nil {
		return nil, fmt.Errorf("update organization: %w", err)
	}
	return org, nil
}

// Delete removes an organization by ID.
func (r *OrganizationRepository) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.Organization{}).Error; err != nil {
		return fmt.Errorf("delete organization: %w", err)
	}
	return nil
}

// AddMember adds a user as a member of an organization (upsert).
func (r *OrganizationRepository) AddMember(ctx context.Context, orgID, userID, role string) (*model.OrganizationMember, error) {
	m := &model.OrganizationMember{
		OrganizationID: orgID,
		UserID:         userID,
		Role:           role,
	}
	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "organization_id"}, {Name: "user_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"role"}),
		}).
		Create(m).Error
	if err != nil {
		return nil, fmt.Errorf("add organization member: %w", err)
	}
	result := &model.OrganizationMember{}
	if err := r.db.WithContext(ctx).Where("organization_id = ? AND user_id = ?", orgID, userID).First(result).Error; err != nil {
		return nil, fmt.Errorf("add organization member: %w", err)
	}
	return result, nil
}

// UpdateMemberRole updates a member's role.
func (r *OrganizationRepository) UpdateMemberRole(ctx context.Context, orgID, userID, role string) error {
	result := r.db.WithContext(ctx).
		Model(&model.OrganizationMember{}).
		Where("organization_id = ? AND user_id = ?", orgID, userID).
		Update("role", role)
	if result.Error != nil {
		return fmt.Errorf("update member role: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("member not found")
	}
	return nil
}

// RemoveMember removes a member from an organization.
func (r *OrganizationRepository) RemoveMember(ctx context.Context, orgID, userID string) error {
	result := r.db.WithContext(ctx).
		Where("organization_id = ? AND user_id = ?", orgID, userID).
		Delete(&model.OrganizationMember{})
	if result.Error != nil {
		return fmt.Errorf("remove member: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("member not found")
	}
	return nil
}

// GetMemberRole returns the role a user has in an organization, or empty string if not a member.
func (r *OrganizationRepository) GetMemberRole(ctx context.Context, orgID, userID string) (string, error) {
	var m model.OrganizationMember
	err := r.db.WithContext(ctx).Where("organization_id = ? AND user_id = ?", orgID, userID).First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", fmt.Errorf("get member role: %w", err)
	}
	return m.Role, nil
}

// ListMembers returns all members of an organization with user details.
func (r *OrganizationRepository) ListMembers(ctx context.Context, orgID string) ([]model.MemberWithUser, error) {
	var results []model.MemberWithUser
	err := r.db.WithContext(ctx).
		Table("organization_members om").
		Select("om.id, om.user_id, om.role, u.email, u.full_name, u.avatar_url").
		Joins("JOIN users u ON u.id = om.user_id").
		Where("om.organization_id = ?", orgID).
		Order("u.full_name ASC").
		Scan(&results).Error
	if err != nil {
		return nil, fmt.Errorf("list organization members: %w", err)
	}
	return results, nil
}
