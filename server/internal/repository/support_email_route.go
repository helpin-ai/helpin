package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type SupportEmailRouteRepository struct {
	db *gorm.DB
}

func NewSupportEmailRouteRepository(db *gorm.DB) *SupportEmailRouteRepository {
	return &SupportEmailRouteRepository{db: db}
}

func (r *SupportEmailRouteRepository) Create(ctx context.Context, route *model.SupportEmailRoute) error {
	if err := r.db.WithContext(ctx).Create(route).Error; err != nil {
		return fmt.Errorf("create support email route: %w", err)
	}
	return nil
}

func (r *SupportEmailRouteRepository) Update(ctx context.Context, route *model.SupportEmailRoute) error {
	if err := r.db.WithContext(ctx).Save(route).Error; err != nil {
		return fmt.Errorf("update support email route: %w", err)
	}
	return nil
}

func (r *SupportEmailRouteRepository) ListByWorkspace(ctx context.Context, workspaceID string) ([]model.SupportEmailRoute, error) {
	var routes []model.SupportEmailRoute
	if err := r.baseQuery(ctx).
		Where("ser.workspace_id = ? AND ser.active = ?", workspaceID, true).
		Order("CASE WHEN ser.mailbox_id IS NULL THEN 0 ELSE 1 END ASC, LOWER(COALESCE(sm.name, '')) ASC, ser.created_at ASC").
		Find(&routes).Error; err != nil {
		return nil, fmt.Errorf("list support email routes: %w", err)
	}
	return routes, nil
}

func (r *SupportEmailRouteRepository) GetByID(ctx context.Context, workspaceID, routeID string) (*model.SupportEmailRoute, error) {
	var route model.SupportEmailRoute
	if err := r.baseQuery(ctx).
		Where("ser.workspace_id = ? AND ser.id = ?", workspaceID, routeID).
		First(&route).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get support email route: %w", err)
	}
	return &route, nil
}

func (r *SupportEmailRouteRepository) GetByRouteKey(ctx context.Context, routeKey string) (*model.SupportEmailRoute, error) {
	var route model.SupportEmailRoute
	if err := r.baseQuery(ctx).
		Where("ser.route_key = ? AND ser.active = ?", strings.TrimSpace(routeKey), true).
		First(&route).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get support email route by route key: %w", err)
	}
	return &route, nil
}

func (r *SupportEmailRouteRepository) GetActiveByInboundAddress(ctx context.Context, inboundAddress string) (*model.SupportEmailRoute, error) {
	var route model.SupportEmailRoute
	if err := r.baseQuery(ctx).
		Where("LOWER(ser.inbound_address) = LOWER(?) AND ser.active = ?", strings.TrimSpace(inboundAddress), true).
		First(&route).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get active support email route by inbound address: %w", err)
	}
	return &route, nil
}

func (r *SupportEmailRouteRepository) GetActiveByMailbox(ctx context.Context, workspaceID string, mailboxID *string) (*model.SupportEmailRoute, error) {
	query := r.baseQuery(ctx).
		Where("ser.workspace_id = ? AND ser.active = ?", workspaceID, true)
	if mailboxID == nil || strings.TrimSpace(*mailboxID) == "" {
		query = query.Where("ser.mailbox_id IS NULL")
	} else {
		query = query.Where("ser.mailbox_id = ?", strings.TrimSpace(*mailboxID))
	}

	var route model.SupportEmailRoute
	if err := query.First(&route).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get active support email route: %w", err)
	}
	return &route, nil
}

func (r *SupportEmailRouteRepository) GetByMailbox(ctx context.Context, workspaceID string, mailboxID *string) (*model.SupportEmailRoute, error) {
	query := r.baseQuery(ctx).
		Where("ser.workspace_id = ?", workspaceID).Order("ser.active DESC, ser.updated_at DESC")
	if mailboxID == nil || strings.TrimSpace(*mailboxID) == "" {
		query = query.Where("ser.mailbox_id IS NULL")
	} else {
		query = query.Where("ser.mailbox_id = ?", strings.TrimSpace(*mailboxID))
	}

	var route model.SupportEmailRoute
	if err := query.First(&route).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get active support email route: %w", err)
	}
	return &route, nil
}

// Reactivate preserves the reserved address and resets setup evidence. The active
// predicate prevents a concurrent re-enable from clearing fresh verification.
func (r *SupportEmailRouteRepository) Reactivate(ctx context.Context, workspaceID, routeID string, sourceAddress *string) error {
	return r.db.WithContext(ctx).Model(&model.SupportEmailRoute{}).
		Where("workspace_id = ? AND id = ? AND active = ?", workspaceID, routeID, false).
		Updates(map[string]any{
			"active": true, "source_address": sourceAddress, "confirmation_received_at": nil,
			"confirmation_conversation_id": nil, "verification_sent_at": nil,
			"forwarding_verified_at": nil, "forwarding_verification_token": "",
			"forwarding_last_error": nil, "updated_at": time.Now().UTC(),
		}).Error
}

func (r *SupportEmailRouteRepository) Disable(ctx context.Context, workspaceID, routeID string) error {
	if err := r.db.WithContext(ctx).
		Model(&model.SupportEmailRoute{}).
		Where("workspace_id = ? AND id = ?", workspaceID, routeID).
		Updates(map[string]any{
			"active":     false,
			"updated_at": time.Now().UTC(),
		}).Error; err != nil {
		return fmt.Errorf("disable support email route: %w", err)
	}
	return nil
}

func (r *SupportEmailRouteRepository) TouchInbound(ctx context.Context, routeID string, receivedAt time.Time) error {
	if strings.TrimSpace(routeID) == "" {
		return nil
	}
	if err := r.db.WithContext(ctx).
		Model(&model.SupportEmailRoute{}).
		Where("id = ?", routeID).
		Updates(map[string]any{
			"last_inbound_at": receivedAt.UTC(),
			"updated_at":      time.Now().UTC(),
		}).Error; err != nil {
		return fmt.Errorf("touch support email route inbound timestamp: %w", err)
	}
	return nil
}

func (r *SupportEmailRouteRepository) baseQuery(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).
		Table("support_email_routes ser").
		Joins("LEFT JOIN support_mailboxes sm ON sm.id = ser.mailbox_id").
		Select(`
			ser.*,
			sm.name AS mailbox_name,
			sm.handle AS mailbox_handle,
			sm.icon AS mailbox_icon
		`)
}
