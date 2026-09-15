package model

// QueryFilterLogic identifies how multiple query-filter rules are combined.
type QueryFilterLogic string

const (
	// QueryFilterLogicAnd requires every rule to match.
	QueryFilterLogicAnd QueryFilterLogic = "and"
	// QueryFilterLogicOr requires at least one rule to match.
	QueryFilterLogicOr QueryFilterLogic = "or"
)

// QueryFilterOperator identifies the comparison applied to a filter field.
type QueryFilterOperator string

const (
	QueryFilterOpGT          QueryFilterOperator = "gt"
	QueryFilterOpGTE         QueryFilterOperator = "gte"
	QueryFilterOpLT          QueryFilterOperator = "lt"
	QueryFilterOpLTE         QueryFilterOperator = "lte"
	QueryFilterOpIs          QueryFilterOperator = "is"
	QueryFilterOpIsNot       QueryFilterOperator = "is_not"
	QueryFilterOpContains    QueryFilterOperator = "contains"
	QueryFilterOpNotContains QueryFilterOperator = "not_contains"
	QueryFilterOpStartsWith  QueryFilterOperator = "starts_with"
	QueryFilterOpEndsWith    QueryFilterOperator = "ends_with"
	QueryFilterOpIsEmpty     QueryFilterOperator = "is_empty"
	QueryFilterOpIsNotEmpty  QueryFilterOperator = "is_not_empty"
	QueryFilterOpOn          QueryFilterOperator = "on"
	QueryFilterOpBefore      QueryFilterOperator = "before"
	QueryFilterOpAfter       QueryFilterOperator = "after"
	QueryFilterOpOnOrBefore  QueryFilterOperator = "on_or_before"
	QueryFilterOpOnOrAfter   QueryFilterOperator = "on_or_after"
	QueryFilterOpBetween     QueryFilterOperator = "between"
)

// QueryFilterRule represents one field/operator/value rule in a query builder.
type QueryFilterRule struct {
	Field    string              `json:"field"`
	Operator QueryFilterOperator `json:"operator"`
	Value    *string             `json:"value,omitempty"`
	Values   []string            `json:"values,omitempty"`
}

// QueryFilterGroup represents a list of query-filter rules joined by one logic mode.
type QueryFilterGroup struct {
	Logic QueryFilterLogic  `json:"logic"`
	Rules []QueryFilterRule `json:"rules"`
}
