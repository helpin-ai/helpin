package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// ErrCustomFieldNotFound means the field does not exist in the workspace.
var ErrCustomFieldNotFound = errors.New("custom field not found")

// ErrCustomFieldKeyTaken means another field of the object has the key.
var ErrCustomFieldKeyTaken = errors.New("custom field key taken")

// CustomFieldRepository stores custom field definitions and reads the JSON
// values records hold.
type CustomFieldRepository struct {
	db *gorm.DB
}

// NewCustomFieldRepository creates the repository.
func NewCustomFieldRepository(db *gorm.DB) *CustomFieldRepository {
	return &CustomFieldRepository{db: db}
}

// customFieldValueSources maps an object type to the table and JSON column
// holding its values.
var customFieldValueSources = map[string][2]string{
	model.CustomFieldObjectConversation: {"support_conversations", "custom_fields"},
	model.CustomFieldObjectCRMContact:   {"crm_contacts", "custom_properties"},
	model.CustomFieldObjectCRMCompany:   {"crm_companies", "custom_properties"},
}

// ListDefinitions returns an object type's fields in display order.
func (r *CustomFieldRepository) ListDefinitions(ctx context.Context, workspaceID, objectType string, includeArchived bool) ([]model.CustomFieldDefinition, error) {
	query := r.db.WithContext(ctx).Where("workspace_id = ? AND object_type = ?", workspaceID, objectType)
	if !includeArchived {
		query = query.Where("archived_at IS NULL")
	}
	var definitions []model.CustomFieldDefinition
	if err := query.Order("position, created_at, id").Find(&definitions).Error; err != nil {
		return nil, fmt.Errorf("list custom fields: %w", err)
	}
	return definitions, nil
}

// GetDefinition returns one field of the workspace.
func (r *CustomFieldRepository) GetDefinition(ctx context.Context, workspaceID, id string) (*model.CustomFieldDefinition, error) {
	var definition model.CustomFieldDefinition
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, id).Take(&definition).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrCustomFieldNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get custom field: %w", err)
	}
	return &definition, nil
}

// CreateDefinition appends a field after the object type's existing ones.
func (r *CustomFieldRepository) CreateDefinition(ctx context.Context, definition *model.CustomFieldDefinition) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var last struct{ Position *int }
		if err := tx.Model(&model.CustomFieldDefinition{}).Select("MAX(position) AS position").
			Where("workspace_id = ? AND object_type = ?", definition.WorkspaceID, definition.ObjectType).Scan(&last).Error; err != nil {
			return fmt.Errorf("find last custom field position: %w", err)
		}
		definition.Position = 0
		if last.Position != nil {
			definition.Position = *last.Position + 1
		}
		if definition.ID == "" {
			definition.ID = uuid.NewString()
		}
		if err := tx.Create(definition).Error; err != nil {
			if isDuplicateKeyError(err) {
				return ErrCustomFieldKeyTaken
			}
			return fmt.Errorf("create custom field: %w", err)
		}
		return nil
	})
}

// UpdateDefinition saves a field's label, description, type and options.
func (r *CustomFieldRepository) UpdateDefinition(ctx context.Context, definition *model.CustomFieldDefinition) error {
	result := r.db.WithContext(ctx).Model(&model.CustomFieldDefinition{}).
		Where("workspace_id = ? AND id = ?", definition.WorkspaceID, definition.ID).
		Updates(map[string]any{
			"label": definition.Label, "description": definition.Description, "field_type": definition.FieldType,
			"options": definition.Options, "updated_at": time.Now().UTC(),
		})
	if result.Error != nil {
		return fmt.Errorf("update custom field: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrCustomFieldNotFound
	}
	return nil
}

// SetArchived archives a field at the given time, or restores it with nil.
func (r *CustomFieldRepository) SetArchived(ctx context.Context, workspaceID, id string, at *time.Time) error {
	result := r.db.WithContext(ctx).Model(&model.CustomFieldDefinition{}).
		Where("workspace_id = ? AND id = ?", workspaceID, id).
		Updates(map[string]any{"archived_at": at, "updated_at": time.Now().UTC()})
	if result.Error != nil {
		return fmt.Errorf("archive custom field: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrCustomFieldNotFound
	}
	return nil
}

// CountDefinitions counts an object type's fields, archived ones included.
func (r *CustomFieldRepository) CountDefinitions(ctx context.Context, workspaceID, objectType string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.CustomFieldDefinition{}).
		Where("workspace_id = ? AND object_type = ?", workspaceID, objectType).Count(&count).Error
	return count, err
}

// ReorderDefinitions sets positions to the order given; the list must hold
// every field of the object type exactly once.
func (r *CustomFieldRepository) ReorderDefinitions(ctx context.Context, workspaceID, objectType string, ids []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing []string
		if err := tx.Model(&model.CustomFieldDefinition{}).Where("workspace_id = ? AND object_type = ?", workspaceID, objectType).
			Pluck("id", &existing).Error; err != nil {
			return fmt.Errorf("list custom fields to reorder: %w", err)
		}
		if len(existing) != len(ids) {
			return ErrCustomFieldNotFound
		}
		seen := map[string]bool{}
		for _, id := range ids {
			if !slices.Contains(existing, id) || seen[id] {
				return ErrCustomFieldNotFound
			}
			seen[id] = true
		}
		for position, id := range ids {
			if err := tx.Model(&model.CustomFieldDefinition{}).Where("workspace_id = ? AND id = ?", workspaceID, id).
				Updates(map[string]any{"position": position, "updated_at": time.Now().UTC()}).Error; err != nil {
				return fmt.Errorf("reorder custom fields: %w", err)
			}
		}
		return nil
	})
}

// SampleValues returns the custom field maps of the object type's most
// recently updated records, at most limit of them.
func (r *CustomFieldRepository) SampleValues(ctx context.Context, workspaceID, objectType string, limit int) ([]model.JSONB, error) {
	source, ok := customFieldValueSources[objectType]
	if !ok {
		return nil, fmt.Errorf("unknown custom field object %q", objectType)
	}
	var rows []struct{ FieldValues model.JSONB }
	if err := r.db.WithContext(ctx).Table(source[0]).Select(source[1]+" AS field_values").
		Where("workspace_id = ?", workspaceID).Order("updated_at DESC").Limit(limit).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("sample custom field values: %w", err)
	}
	result := make([]model.JSONB, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.FieldValues)
	}
	return result, nil
}

// SetValues writes field values on one record key by key, leaving other keys
// alone; a nil value removes the key. It reports whether the record exists.
func (r *CustomFieldRepository) SetValues(ctx context.Context, workspaceID, objectType, recordID string, values map[string]any) (bool, error) {
	source, ok := customFieldValueSources[objectType]
	if !ok {
		return false, fmt.Errorf("unknown custom field object %q", objectType)
	}
	set := map[string]any{}
	var removed []string
	for key, value := range values {
		if value == nil {
			removed = append(removed, key)
		} else {
			set[key] = value
		}
	}
	patch, err := json.Marshal(set)
	if err != nil {
		return false, fmt.Errorf("encode custom field values: %w", err)
	}
	var expr clause.Expr
	if r.db.Dialector.Name() == "postgres" {
		sql, args := "(COALESCE("+source[1]+", '{}'::jsonb) || ?::jsonb)", []any{string(patch)}
		for _, key := range removed {
			sql += " - ?::text"
			args = append(args, key)
		}
		expr = gorm.Expr(sql, args...)
	} else {
		// json_patch removes keys whose patch value is null (RFC 7396).
		for _, key := range removed {
			set[key] = nil
		}
		if patch, err = json.Marshal(set); err != nil {
			return false, fmt.Errorf("encode custom field values: %w", err)
		}
		expr = gorm.Expr("json_patch(COALESCE("+source[1]+", '{}'), ?)", string(patch))
	}
	result := r.db.WithContext(ctx).Table(source[0]).Where("workspace_id = ? AND id = ?", workspaceID, recordID).
		Updates(map[string]any{source[1]: expr, "updated_at": time.Now().UTC()})
	if result.Error != nil {
		return false, fmt.Errorf("set custom field values: %w", result.Error)
	}
	return result.RowsAffected > 0, nil
}
