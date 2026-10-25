package repository

import (
	"context"
	"fmt"
	"sort"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/querybuilder"
)

// CRMFilterFieldDefinitions returns a copy of the native list-filter fields
// for a CRM object type, keyed by public field name.
func CRMFilterFieldDefinitions(objectType string) (querybuilder.Definitions, bool) {
	var source querybuilder.Definitions
	switch objectType {
	case model.CRMFieldObjectContact:
		source = crmContactFilterDefinitions
	case model.CRMFieldObjectCompany:
		source = crmCompanyFilterDefinitions
	default:
		return nil, false
	}
	result := make(querybuilder.Definitions, len(source))
	for name, definition := range source {
		result[name] = definition
	}
	return result, true
}

// CRMCustomPropertyOperators returns the operators custom_properties.<key>
// filters accept: text operators plus numeric comparisons, which skip
// non-numeric values.
func CRMCustomPropertyOperators() []model.QueryFilterOperator {
	definition := querybuilder.FieldDefinition{
		Type:              querybuilder.FieldTypeText,
		NumericExpression: crmCustomPropertyNumericExpression("custom_properties", "postgres", "key"),
	}
	return definition.AllowedOperators()
}

// crmBehaviorProfileExpression reads one synced behavior column for the
// record identified by table.id, from the record's own workspace. A record
// without a profile yields NULL, so is_empty matches it and comparisons
// never do.
func crmBehaviorProfileExpression(entityType, table, column string) string {
	return "(SELECT crm_behavior_profiles." + column + " FROM crm_behavior_profiles" +
		" WHERE crm_behavior_profiles.entity_type = '" + entityType + "'" +
		" AND crm_behavior_profiles.entity_id = " + table + ".id" +
		" AND crm_behavior_profiles.workspace_id = " + table + ".workspace_id)"
}

// ListFilterSources returns the custom properties of up to limit of the
// workspace's most recently updated contacts.
func (r *CRMContactRepository) ListFilterSources(ctx context.Context, workspaceID string, limit int) ([]model.JSONB, error) {
	var rows []struct {
		CustomProperties model.JSONB
	}
	if err := r.db.WithContext(ctx).
		Model(&model.CRMContact{}).
		Select("custom_properties").
		Where("workspace_id = ?", workspaceID).
		Order("updated_at DESC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list contact filter sources: %w", err)
	}
	result := make([]model.JSONB, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.CustomProperties)
	}
	return result, nil
}

// ExistingCustomPropertyKeys returns the subset of keys that at least one of
// the workspace's contacts already carries.
func (r *CRMContactRepository) ExistingCustomPropertyKeys(ctx context.Context, workspaceID string, keys []string) (map[string]bool, error) {
	return existingCRMCustomPropertyKeys(ctx, r.db, "crm_contacts", workspaceID, keys)
}

// ExistingCustomPropertyKeys returns the subset of keys that at least one of
// the workspace's companies already carries.
func (r *CRMCompanyRepository) ExistingCustomPropertyKeys(ctx context.Context, workspaceID string, keys []string) (map[string]bool, error) {
	return existingCRMCustomPropertyKeys(ctx, r.db, "crm_companies", workspaceID, keys)
}

func existingCRMCustomPropertyKeys(ctx context.Context, db *gorm.DB, table, workspaceID string, keys []string) (map[string]bool, error) {
	result := map[string]bool{}
	unique := make([]string, 0, len(keys))
	seen := map[string]bool{}
	for _, key := range keys {
		if !ValidCRMCustomPropertyKey(key) || seen[key] {
			continue
		}
		seen[key] = true
		unique = append(unique, key)
	}
	if len(unique) == 0 {
		return result, nil
	}
	sort.Strings(unique)
	// One pass over the workspace's records, however many keys are checked.
	// table is a fixed CRM table name, never caller input.
	query := "SELECT DISTINCT keys.key FROM " + table + ", jsonb_each(" + table + ".custom_properties) AS keys" +
		" WHERE " + table + ".workspace_id = ? AND keys.key IN ? AND jsonb_typeof(keys.value) <> 'null'"
	if db.Dialector.Name() != "postgres" {
		query = "SELECT DISTINCT keys.key FROM " + table + ", json_each(" + table + ".custom_properties) AS keys" +
			" WHERE " + table + ".workspace_id = ? AND keys.key IN ? AND keys.type <> 'null'"
	}
	var found []string
	if err := db.WithContext(ctx).Raw(query, workspaceID, unique).Scan(&found).Error; err != nil {
		return nil, fmt.Errorf("check existing custom property keys: %w", err)
	}
	for _, key := range found {
		result[key] = true
	}
	return result, nil
}
