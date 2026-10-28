package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// SetMetadataKey stores one value in a message's metadata and returns the saved
// metadata. Other keys keep their current values, so a late enrichment step
// cannot undo a decision another step recorded after the message was created.
func (r *SupportMessageRepository) SetMetadataKey(ctx context.Context, id, key string, value any) (string, error) {
	encodedValue, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("encode metadata %q: %w", key, err)
	}
	var saved string
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var message model.SupportMessage
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id", "metadata").First(&message, "id = ?", id).Error; err != nil {
			return err
		}
		metadata := map[string]json.RawMessage{}
		if current := strings.TrimSpace(message.Metadata); current != "" {
			if err := json.Unmarshal([]byte(current), &metadata); err != nil {
				return fmt.Errorf("decode message metadata: %w", err)
			}
		}
		if metadata == nil {
			metadata = map[string]json.RawMessage{}
		}
		metadata[key] = encodedValue
		raw, err := json.Marshal(metadata)
		if err != nil {
			return fmt.Errorf("encode message metadata: %w", err)
		}
		saved = string(raw)
		return tx.Model(&model.SupportMessage{}).Where("id = ?", id).Update("metadata", saved).Error
	})
	if err != nil {
		return "", fmt.Errorf("set message metadata %q: %w", key, err)
	}
	return saved, nil
}
