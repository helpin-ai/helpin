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
// keys are rejected as client validation errors. dialect selects the JSON
// list functions that let is_any_of match elements of list values. Keys in
// dateKeys are defined as date fields and get date operators instead.
func withCRMCustomPropertyFields(
	base querybuilder.Definitions,
	column string,
	dialect string,
	group *model.QueryFilterGroup,
	dateKeys map[string]bool,
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
		if customPropertyRuleIsDate(rule.Operator, dateKeys[key]) {
			definitions[rule.Field] = querybuilder.FieldDefinition{Expression: customFieldDateExpression(column, dialect, key), Type: querybuilder.FieldTypeDate}
			continue
		}
		definitions[rule.Field] = querybuilder.FieldDefinition{
			Expression:        crmCustomPropertyTextExpression(column, dialect, key),
			Type:              querybuilder.FieldTypeText,
			ElementSource:     crmCustomPropertyElementSource(column, dialect, key),
			NumericExpression: crmCustomPropertyNumericExpression(column, dialect, key),
		}
	}
	if definitions == nil {
		return base, nil
	}
	return definitions, nil
}

// crmCustomPropertyElementSource returns a set-returning source whose value
// column holds the elements of a list-valued custom property, or no rows when
// the property is not a list.
func crmCustomPropertyElementSource(column, dialect, key string) string {
	if dialect == "postgres" {
		value := column + " -> '" + key + "'"
		return "jsonb_array_elements_text(CASE WHEN jsonb_typeof(" + value + ") = 'array' THEN " + value + " ELSE '[]'::jsonb END)"
	}
	path := "'$." + key + "'"
	return "json_each(CASE WHEN json_type(" + column + ", " + path + ") = 'array' THEN json_extract(" + column + ", " + path + ") ELSE '[]' END)"
}

// crmCustomPropertyTextExpression reads a custom property as text. SQLite
// returns JSON booleans from ->> as 1 and 0, so they are spelled out to match
// PostgreSQL's 'true' and 'false'.
func crmCustomPropertyTextExpression(column, dialect, key string) string {
	if dialect == "postgres" {
		return "(" + column + " ->> '" + key + "')"
	}
	path := "'$." + key + "'"
	return "(CASE json_type(" + column + ", " + path + ") WHEN 'true' THEN 'true' WHEN 'false' THEN 'false' ELSE " + column + " ->> '" + key + "' END)"
}

// crmCustomPropertyNumericExpression reads a custom property as a number:
// JSON numbers, and strings holding only a decimal number such as "40". Any
// other value yields NULL. The CASE guard matters: a bare cast would fail the
// whole query on one non-numeric record. Expressions must not contain a
// literal "?", which GORM would treat as a bind placeholder.
func crmCustomPropertyNumericExpression(column, dialect, key string) string {
	if dialect == "postgres" {
		value := column + " -> '" + key + "'"
		text := "TRIM(" + column + " ->> '" + key + "')"
		return "(CASE WHEN jsonb_typeof(" + value + ") = 'number' THEN (" + column + " ->> '" + key + "')::numeric" +
			" WHEN jsonb_typeof(" + value + ") = 'string' AND " + text + " ~ '^-{0,1}[0-9]+(\\.[0-9]+){0,1}$' THEN " + text + "::numeric END)"
	}
	path := "'$." + key + "'"
	text := "TRIM(json_extract(" + column + ", " + path + "))"
	// A minus sign is only allowed in front, so ranges like "11-50" stay NULL.
	unsigned := "LTRIM(" + text + ", '-')"
	return "(CASE WHEN json_type(" + column + ", " + path + ") IN ('integer', 'real') THEN json_extract(" + column + ", " + path + ")" +
		" WHEN json_type(" + column + ", " + path + ") = 'text' AND " + unsigned + " <> '' AND " + unsigned + " NOT GLOB '*[^0-9.]*'" +
		" THEN CAST(" + text + " AS REAL) END)"
}

// customPropertyRuleIsDate reports whether a rule compares a custom property
// as a date: date-only operators always do, so saved filters keep working
// after a field is archived or retyped; between and empty checks do for
// keys defined as dates. Other operators compare text, as before.
func customPropertyRuleIsDate(operator model.QueryFilterOperator, definedAsDate bool) bool {
	switch operator {
	case model.QueryFilterOpOn, model.QueryFilterOpBefore, model.QueryFilterOpAfter,
		model.QueryFilterOpOnOrBefore, model.QueryFilterOpOnOrAfter:
		return true
	case model.QueryFilterOpBetween, model.QueryFilterOpIsEmpty, model.QueryFilterOpIsNotEmpty:
		return definedAsDate
	}
	return false
}

// customFieldDateExpression reads a valid YYYY-MM-DD value as midnight UTC,
// comparable with the date filters' bounds; any other value, including
// impossible dates such as 2026-02-30, yields NULL instead of failing the
// query. SQLite compares text, so the value is spelled the way it binds times.
func customFieldDateExpression(column, dialect, key string) string {
	if dialect == "postgres" {
		text := "(" + column + " ->> '" + key + "')"
		// Built from parts so out-of-range days never reach a failing cast.
		date := "(make_date(substr(" + text + ", 1, 4)::int, substr(" + text + ", 6, 2)::int, 1) + (substr(" + text + ", 9, 2)::int - 1))"
		return "(CASE WHEN " + text + " ~ '^[1-9][0-9]{3}-(0[1-9]|1[0-2])-(0[1-9]|[12][0-9]|3[01])$'" +
			" THEN CASE WHEN to_char(" + date + ", 'YYYY-MM-DD') = " + text +
			" THEN " + date + "::timestamp AT TIME ZONE 'UTC' END END)"
	}
	value := "json_extract(" + column + ", '$." + key + "')"
	// date() normalizes impossible days, so the round trip must match.
	return "(CASE WHEN json_type(" + column + ", '$." + key + "') = 'text' AND date(" + value + ") = " + value +
		" THEN " + value + " || ' 00:00:00+00:00' END)"
}
