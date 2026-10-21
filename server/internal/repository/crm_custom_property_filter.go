package repository

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/querybuilder"
)

// CRMCustomPropertyFilterPrefix marks query-builder fields that read one
// top-level key from a CRM record's custom_properties JSON.
const CRMCustomPropertyFilterPrefix = "custom_properties."

// _crmCustomPropertyKey is the only key shape that may be embedded in SQL.
// Keys are validated before use because JSON operators take them as literals.
var _crmCustomPropertyKey = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

// ValidCRMCustomPropertyKey reports whether key is a safe custom property key.
func ValidCRMCustomPropertyKey(key string) bool {
	return _crmCustomPropertyKey.MatchString(key)
}

// withCRMCustomPropertyFields returns base plus text field definitions for
// every custom_properties.<key> field referenced by group. Unknown or unsafe
// keys are rejected as client validation errors.
func withCRMCustomPropertyFields(
	base querybuilder.Definitions,
	column string,
	group *model.QueryFilterGroup,
) (querybuilder.Definitions, error) {
	if group == nil {
		return base, nil
	}
	var definitions querybuilder.Definitions
	for _, rule := range group.Rules {
		key, ok := strings.CutPrefix(rule.Field, CRMCustomPropertyFilterPrefix)
		if !ok {
			continue
		}
		if !ValidCRMCustomPropertyKey(key) {
			return nil, &querybuilder.ValidationError{
				Message: fmt.Sprintf("invalid custom property filter field %q", rule.Field),
			}
		}
		if definitions == nil {
			definitions = make(querybuilder.Definitions, len(base)+len(group.Rules))
			for name, definition := range base {
				definitions[name] = definition
			}
		}
		definitions[rule.Field] = querybuilder.FieldDefinition{
			Expression: "(" + column + " ->> '" + key + "')",
			Type:       querybuilder.FieldTypeText,
		}
	}
	if definitions == nil {
		return base, nil
	}
	return definitions, nil
}
