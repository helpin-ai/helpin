package repository

import (
	"context"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PortalAuthRepository struct{ db *gorm.DB }

func NewPortalAuthRepository(db *gorm.DB) *PortalAuthRepository { return &PortalAuthRepository{db: db} }

func (r *PortalAuthRepository) CreateLink(ctx context.Context, link *model.PortalMagicLink) error {
	return r.db.WithContext(ctx).Create(link).Error
}

// ConsumeLink locks the link and creates a session in the same transaction. A
// failed identity lookup or insert rolls back consumption, allowing a retry.
func (r *PortalAuthRepository) ConsumeLink(ctx context.Context, workspaceID, hash string, now time.Time, create func(*gorm.DB, *model.PortalMagicLink) (*model.PortalSession, error)) (*model.PortalSession, error) {
	var session *model.PortalSession
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var link model.PortalMagicLink
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND token_hash = ? AND used_at IS NULL AND expires_at > ?", workspaceID, hash, now).First(&link).Error; err != nil {
			return err
		}
		var err error
		session, err = create(tx, &link)
		if err != nil {
			return err
		}
		if err = tx.Create(session).Error; err != nil {
			return err
		}
		return tx.Model(&link).Update("used_at", now).Error
	})
	return session, err
}

func (r *PortalAuthRepository) FindSession(ctx context.Context, workspaceID, hash string, now time.Time) (*model.PortalSession, error) {
	var session model.PortalSession
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND token_hash = ? AND revoked_at IS NULL AND expires_at > ?", workspaceID, hash, now).First(&session).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}
func (r *PortalAuthRepository) FindSessionIdentity(ctx context.Context, workspaceID, identityID string, identity *model.SupportPortalIdentity) error {
	return r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, identityID).First(identity).Error
}
func (r *PortalAuthRepository) RevokeSession(ctx context.Context, workspaceID, hash string, now time.Time) error {
	return r.db.WithContext(ctx).Model(new(model.PortalSession)).Where("workspace_id = ? AND token_hash = ? AND revoked_at IS NULL", workspaceID, hash).Update("revoked_at", now).Error
}
