package querybuilder

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// FieldType classifies which operators a query-builder field supports.
type FieldType string

const (
	// FieldTypeNumber supports finite numeric comparisons and inclusive ranges.
	FieldTypeNumber FieldType = "number"
	// FieldTypeText supports text comparison operators.
	FieldTypeText FieldType = "text"
	// FieldTypeEnum supports exact-match style operators.
	FieldTypeEnum FieldType = "enum"
	// FieldTypeID supports exact-match identifier operators.
	FieldTypeID FieldType = "id"
	// FieldTypeDate supports date and range operators.
	FieldTypeDate FieldType = "date"
)

// FieldDefinition describes how one query-builder field maps to SQL.
type FieldDefinition struct {
	Column     string
	Expression string
	Type       FieldType
	Operators  []model.QueryFilterOperator
}

// ExpressionSQL returns the SQL expression for this field.
func (f FieldDefinition) ExpressionSQL() string {
	if strings.TrimSpace(f.Expression) != "" {
		return f.Expression
	}
	return f.Column
}

// Definitions maps public filter field keys to SQL-backed field definitions.
type Definitions map[string]FieldDefinition

// ValidationError identifies malformed query-builder input from clients.
type ValidationError struct {
	Message string
}

// Error implements error.
func (e *ValidationError) Error() string {
	return e.Message
}

// ApplyGORM applies a validated query-filter group to a GORM query.
func ApplyGORM(query *gorm.DB, group *model.QueryFilterGroup, definitions Definitions) (*gorm.DB, error) {
	if group == nil || len(group.Rules) == 0 {
		return query, nil
	}

	logic := normalizedLogic(group.Logic)
	fragments := make([]string, 0, len(group.Rules))
	args := make([]any, 0, len(group.Rules)*2)

	for _, rule := range group.Rules {
		definition, ok := definitions[rule.Field]
		if !ok {
			return nil, &ValidationError{Message: fmt.Sprintf("unsupported filter field %q", rule.Field)}
		}
		if !isOperatorAllowed(definition, rule.Operator) {
			return nil, &ValidationError{Message: fmt.Sprintf("operator %q is not allowed for field %q", rule.Operator, rule.Field)}
		}

		fragment, fragmentArgs, err := buildClause(definition, rule)
		if err != nil {
			return nil, err
		}

		fragments = append(fragments, "("+fragment+")")
		args = append(args, fragmentArgs...)
	}

	if len(fragments) == 0 {
		return query, nil
	}

	return query.Where(strings.Join(fragments, " "+logic+" "), args...), nil
}

func normalizedLogic(logic model.QueryFilterLogic) string {
	switch logic {
	case model.QueryFilterLogicOr:
		return "OR"
	default:
		return "AND"
	}
}

func isOperatorAllowed(definition FieldDefinition, operator model.QueryFilterOperator) bool {
	allowed := definition.Operators
	if len(allowed) == 0 {
		allowed = defaultOperators(definition.Type)
	}
	for _, candidate := range allowed {
		if candidate == operator {
			return true
		}
	}
	return false
}

func defaultOperators(fieldType FieldType) []model.QueryFilterOperator {
	switch fieldType {
	case FieldTypeNumber:
		return []model.QueryFilterOperator{model.QueryFilterOpIs, model.QueryFilterOpIsNot, model.QueryFilterOpGT, model.QueryFilterOpGTE, model.QueryFilterOpLT, model.QueryFilterOpLTE, model.QueryFilterOpBetween, model.QueryFilterOpIsEmpty, model.QueryFilterOpIsNotEmpty}
	case FieldTypeText:
		return []model.QueryFilterOperator{
			model.QueryFilterOpIs,
			model.QueryFilterOpIsNot,
			model.QueryFilterOpContains,
			model.QueryFilterOpNotContains,
			model.QueryFilterOpStartsWith,
			model.QueryFilterOpEndsWith,
			model.QueryFilterOpIsEmpty,
			model.QueryFilterOpIsNotEmpty,
		}
	case FieldTypeDate:
		return []model.QueryFilterOperator{
			model.QueryFilterOpOn,
			model.QueryFilterOpBefore,
			model.QueryFilterOpAfter,
			model.QueryFilterOpOnOrBefore,
			model.QueryFilterOpOnOrAfter,
			model.QueryFilterOpBetween,
			model.QueryFilterOpIsEmpty,
			model.QueryFilterOpIsNotEmpty,
		}
	default:
		return []model.QueryFilterOperator{
			model.QueryFilterOpIs,
			model.QueryFilterOpIsNot,
			model.QueryFilterOpIsEmpty,
			model.QueryFilterOpIsNotEmpty,
		}
	}
}

func buildClause(definition FieldDefinition, rule model.QueryFilterRule) (string, []any, error) {
	expression := definition.ExpressionSQL()
	switch definition.Type {
	case FieldTypeNumber:
		return buildNumberClause(expression, rule)
	case FieldTypeText:
		return buildTextClause(expression, rule)
	case FieldTypeDate:
		return buildDateClause(expression, rule)
	case FieldTypeEnum, FieldTypeID:
		return buildExactClause(expression, rule)
	default:
		return "", nil, &ValidationError{Message: fmt.Sprintf("unsupported field type %q", definition.Type)}
	}
}

func buildTextClause(expression string, rule model.QueryFilterRule) (string, []any, error) {
	switch rule.Operator {
	case model.QueryFilterOpIs:
		value, err := singleRuleValue(rule, "requires a value")
		if err != nil {
			return "", nil, err
		}
		return "LOWER(" + expression + ") = ?", []any{strings.ToLower(value)}, nil
	case model.QueryFilterOpIsNot:
		value, err := singleRuleValue(rule, "requires a value")
		if err != nil {
			return "", nil, err
		}
		return "LOWER(" + expression + ") <> ?", []any{strings.ToLower(value)}, nil
	case model.QueryFilterOpContains:
		value, err := singleRuleValue(rule, "requires a value")
		if err != nil {
			return "", nil, err
		}
		return "LOWER(" + expression + ") LIKE ?", []any{"%" + strings.ToLower(value) + "%"}, nil
	case model.QueryFilterOpNotContains:
		value, err := singleRuleValue(rule, "requires a value")
		if err != nil {
			return "", nil, err
		}
		return "LOWER(" + expression + ") NOT LIKE ?", []any{"%" + strings.ToLower(value) + "%"}, nil
	case model.QueryFilterOpStartsWith:
		value, err := singleRuleValue(rule, "requires a value")
		if err != nil {
			return "", nil, err
		}
		return "LOWER(" + expression + ") LIKE ?", []any{strings.ToLower(value) + "%"}, nil
	case model.QueryFilterOpEndsWith:
		value, err := singleRuleValue(rule, "requires a value")
		if err != nil {
			return "", nil, err
		}
		return "LOWER(" + expression + ") LIKE ?", []any{"%" + strings.ToLower(value)}, nil
	case model.QueryFilterOpIsEmpty:
		return "(" + expression + " IS NULL OR " + expression + " = '')", nil, nil
	case model.QueryFilterOpIsNotEmpty:
		return "(" + expression + " IS NOT NULL AND " + expression + " <> '')", nil, nil
	default:
		return "", nil, &ValidationError{Message: fmt.Sprintf("unsupported text operator %q", rule.Operator)}
	}
}

func buildExactClause(expression string, rule model.QueryFilterRule) (string, []any, error) {
	switch rule.Operator {
	case model.QueryFilterOpIs:
		value, err := singleRuleValue(rule, "requires a value")
		if err != nil {
			return "", nil, err
		}
		return expression + " = ?", []any{value}, nil
	case model.QueryFilterOpIsNot:
		value, err := singleRuleValue(rule, "requires a value")
		if err != nil {
			return "", nil, err
		}
		return expression + " <> ?", []any{value}, nil
	case model.QueryFilterOpIsEmpty:
		return "(" + expression + " IS NULL OR " + expression + " = '')", nil, nil
	case model.QueryFilterOpIsNotEmpty:
		return "(" + expression + " IS NOT NULL AND " + expression + " <> '')", nil, nil
	default:
		return "", nil, &ValidationError{Message: fmt.Sprintf("unsupported exact-match operator %q", rule.Operator)}
	}
}

func buildDateClause(expression string, rule model.QueryFilterRule) (string, []any, error) {
	switch rule.Operator {
	case model.QueryFilterOpOn:
		start, end, err := dateWindow(rule)
		if err != nil {
			return "", nil, err
		}
		return expression + " >= ? AND " + expression + " < ?", []any{start, end}, nil
	case model.QueryFilterOpBefore:
		start, _, err := dateWindow(rule)
		if err != nil {
			return "", nil, err
		}
		return expression + " < ?", []any{start}, nil
	case model.QueryFilterOpAfter:
		_, end, err := dateWindow(rule)
		if err != nil {
			return "", nil, err
		}
		return expression + " >= ?", []any{end}, nil
	case model.QueryFilterOpOnOrBefore:
		_, end, err := dateWindow(rule)
		if err != nil {
			return "", nil, err
		}
		return expression + " < ?", []any{end}, nil
	case model.QueryFilterOpOnOrAfter:
		start, _, err := dateWindow(rule)
		if err != nil {
			return "", nil, err
		}
		return expression + " >= ?", []any{start}, nil
	case model.QueryFilterOpBetween:
		if len(rule.Values) != 2 {
			return "", nil, &ValidationError{Message: fmt.Sprintf("operator %q requires exactly two date values", rule.Operator)}
		}
		start, _, err := parseDateValue(rule.Values[0])
		if err != nil {
			return "", nil, err
		}
		_, end, err := parseDateValue(rule.Values[1])
		if err != nil {
			return "", nil, err
		}
		if end.Before(start) {
			start, end = end, start
		}
		return expression + " >= ? AND " + expression + " < ?", []any{start, end.Add(24 * time.Hour)}, nil
	case model.QueryFilterOpIsEmpty:
		return expression + " IS NULL", nil, nil
	case model.QueryFilterOpIsNotEmpty:
		return expression + " IS NOT NULL", nil, nil
	default:
		return "", nil, &ValidationError{Message: fmt.Sprintf("unsupported date operator %q", rule.Operator)}
	}
}

func singleRuleValue(rule model.QueryFilterRule, suffix string) (string, error) {
	if rule.Value == nil || strings.TrimSpace(*rule.Value) == "" {
		return "", &ValidationError{Message: fmt.Sprintf("field %q %s", rule.Field, suffix)}
	}
	return strings.TrimSpace(*rule.Value), nil
}

func dateWindow(rule model.QueryFilterRule) (time.Time, time.Time, error) {
	value, err := singleRuleValue(rule, "requires a date value")
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	return parseDateValue(value)
}

func parseDateValue(raw string) (time.Time, time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, time.Time{}, &ValidationError{Message: "date value is required"}
	}

	if t, err := time.Parse("2006-01-02", raw); err == nil {
		start := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
		return start, start.Add(24 * time.Hour), nil
	}

	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		start := time.Date(t.UTC().Year(), t.UTC().Month(), t.UTC().Day(), 0, 0, 0, 0, time.UTC)
		return start, start.Add(24 * time.Hour), nil
	}

	return time.Time{}, time.Time{}, &ValidationError{Message: fmt.Sprintf("invalid date value %q", raw)}
}

func buildNumberClause(expression string, rule model.QueryFilterRule) (string, []any, error) {
	if rule.Operator == model.QueryFilterOpIsEmpty {
		return expression + " IS NULL", nil, nil
	}
	if rule.Operator == model.QueryFilterOpIsNotEmpty {
		return expression + " IS NOT NULL", nil, nil
	}
	parse := func(raw string) (float64, error) {
		value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
			return 0, &ValidationError{Message: "numeric filter requires a finite number"}
		}
		return value, nil
	}
	if rule.Operator == model.QueryFilterOpBetween {
		if len(rule.Values) != 2 {
			return "", nil, &ValidationError{Message: "numeric range requires two values"}
		}
		low, err := parse(rule.Values[0])
		if err != nil {
			return "", nil, err
		}
		high, err := parse(rule.Values[1])
		if err != nil {
			return "", nil, err
		}
		if low > high {
			return "", nil, &ValidationError{Message: "range minimum must not exceed maximum"}
		}
		return expression + " BETWEEN ? AND ?", []any{low, high}, nil
	}
	if rule.Value == nil {
		return "", nil, &ValidationError{Message: "numeric filter requires a value"}
	}
	value, err := parse(*rule.Value)
	if err != nil {
		return "", nil, err
	}
	operators := map[model.QueryFilterOperator]string{model.QueryFilterOpIs: "=", model.QueryFilterOpIsNot: "<>", model.QueryFilterOpGT: ">", model.QueryFilterOpGTE: ">=", model.QueryFilterOpLT: "<", model.QueryFilterOpLTE: "<="}
	operator, ok := operators[rule.Operator]
	if !ok {
		return "", nil, &ValidationError{Message: "unsupported numeric operator"}
	}
	return expression + " " + operator + " ?", []any{value}, nil
}
