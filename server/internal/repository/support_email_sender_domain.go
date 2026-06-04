package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type SupportEmailSenderDomainRepository struct {
	db *gorm.DB
}

func NewSupportEmailSenderDomainRepository(db *gorm.DB) *SupportEmailSenderDomainRepository {
	return &SupportEmailSenderDomainRepository{db: db}
}

func (r *SupportEmailSenderDomainRepository) Create(ctx context.Context, domain *model.SupportEmailSenderDomain) error {
	if err := r.db.WithContext(ctx).Create(domain).Error; err != nil {
		return fmt.Errorf("create support email sender domain: %w", err)
	}
	return nil
}

func (r *SupportEmailSenderDomainRepository) Update(ctx context.Context, domain *model.SupportEmailSenderDomain) error {
	if err := r.db.WithContext(ctx).Save(domain).Error; err != nil {
		return fmt.Errorf("update support email sender domain: %w", err)
	}
	return nil
}

func (r *SupportEmailSenderDomainRepository) ListByWorkspace(ctx context.Context, workspaceID string) ([]model.SupportEmailSenderDomain, error) {
	var domains []model.SupportEmailSenderDomain
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID).
		Order("active DESC, created_at DESC").
		Find(&domains).Error; err != nil {
		return nil, fmt.Errorf("list support email sender domains: %w", err)
	}
	return domains, nil
}

func (r *SupportEmailSenderDomainRepository) GetByID(ctx context.Context, workspaceID, domainID string) (*model.SupportEmailSenderDomain, error) {
	var domain model.SupportEmailSenderDomain
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND id = ?", workspaceID, strings.TrimSpace(domainID)).
		First(&domain).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get support email sender domain: %w", err)
	}
	return &domain, nil
}

func (r *SupportEmailSenderDomainRepository) GetByDomain(ctx context.Context, workspaceID, domainName string) (*model.SupportEmailSenderDomain, error) {
	var domain model.SupportEmailSenderDomain
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND LOWER(domain) = LOWER(?)", workspaceID, strings.TrimSpace(domainName)).
		First(&domain).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get support email sender domain by name: %w", err)
	}
	return &domain, nil
}

func (r *SupportEmailSenderDomainRepository) GetActiveVerifiedByWorkspace(ctx context.Context, workspaceID string) (*model.SupportEmailSenderDomain, error) {
	var domain model.SupportEmailSenderDomain
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND active = ? AND dkim_verified = ? AND return_path_domain_verified = ?", workspaceID, true, true, true).
		First(&domain).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get active support email sender domain: %w", err)
	}
	return &domain, nil
}

func (r *SupportEmailSenderDomainRepository) Activate(ctx context.Context, workspaceID, domainID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		if err := tx.Model(&model.SupportEmailSenderDomain{}).
			Where("workspace_id = ? AND active = ?", workspaceID, true).
			Updates(map[string]any{"active": false, "updated_at": now}).Error; err != nil {
			return fmt.Errorf("deactivate existing support email sender domains: %w", err)
		}
		if err := tx.Model(&model.SupportEmailSenderDomain{}).
			Where("workspace_id = ? AND id = ?", workspaceID, strings.TrimSpace(domainID)).
			Updates(map[string]any{"active": true, "updated_at": now}).Error; err != nil {
			return fmt.Errorf("activate support email sender domain: %w", err)
		}
		return nil
	})
}

func (r *SupportEmailSenderDomainRepository) Deactivate(ctx context.Context, workspaceID, domainID string) error {
	if err := r.db.WithContext(ctx).
		Model(&model.SupportEmailSenderDomain{}).
		Where("workspace_id = ? AND id = ?", workspaceID, strings.TrimSpace(domainID)).
		Updates(map[string]any{
			"active":     false,
			"updated_at": time.Now().UTC(),
		}).Error; err != nil {
		return fmt.Errorf("deactivate support email sender domain: %w", err)
	}
	return nil
}
