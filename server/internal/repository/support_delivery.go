package repository

import (
	"context"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func supportDeliveryModeSQL(db *gorm.DB, alias string) string {
	return supportMetadataStringSQL(db, alias, "delivery_mode")
}

func supportMetadataStringSQL(db *gorm.DB, alias, key string) string {
	if db.Dialector.Name() == "sqlite" {
		return "COALESCE(json_extract(CASE WHEN json_valid(" + alias + ".metadata) THEN " + alias + ".metadata ELSE '{}' END, '$." + key + "'), '')"
	}
	return "COALESCE(" + alias + ".metadata->>'" + key + "', '')"
}

// UpdateExplicitEmailStatus records delivery state without overwriting other metadata.
func (r *SupportMessageRepository) UpdateExplicitEmailStatus(ctx context.Context, ids []string, status, reason string) error {
	if len(ids) == 0 {
		return nil
	}
	query := r.db.WithContext(ctx).Model(&model.SupportMessage{}).Where("id IN ?", ids).
		Where(supportDeliveryModeSQL(r.db, "support_messages")+" IN ?", []string{model.SupportDeliveryEmailOnly, model.SupportDeliveryChatAndEmail})
	expression := "jsonb_set(jsonb_set(COALESCE(metadata, '{}'::jsonb), '{email_delivery_status}', to_jsonb(?::text)), '{email_delivery_error}', to_jsonb(?::text))"
	if r.db.Dialector.Name() == "sqlite" {
		expression = "json_set(CASE WHEN json_valid(metadata) THEN metadata ELSE '{}' END, '$.email_delivery_status', ?, '$.email_delivery_error', ?)"
	}
	return query.Update("metadata", gorm.Expr(expression, status, reason)).Error
}
