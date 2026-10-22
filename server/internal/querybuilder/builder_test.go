package querybuilder

import (
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestBuildClauseEmptyChecksOnIDFieldsAvoidEmptyStringComparison(t *testing.T) {
	definition := FieldDefinition{Column: "owner_member_id", Type: FieldTypeID}
	for operator, want := range map[model.QueryFilterOperator]string{
		model.QueryFilterOpIsEmpty:    "owner_member_id IS NULL",
		model.QueryFilterOpIsNotEmpty: "owner_member_id IS NOT NULL",
	} {
		clause, args, err := buildClause(definition, model.QueryFilterRule{Field: "owner", Operator: operator})
		if err != nil || clause != want || len(args) != 0 {
			t.Errorf("%s = %q %v %v, want %q", operator, clause, args, err, want)
		}
	}
}

func TestBuildClauseAnyOfAndNoneOf(t *testing.T) {
	text := FieldDefinition{Column: "industry", Type: FieldTypeText}
	clause, args, err := buildClause(text, model.QueryFilterRule{Field: "industry", Operator: model.QueryFilterOpIsAnyOf, Values: []string{" Software ", "", "RETAIL"}})
	if err != nil || clause != "LOWER(industry) IN ?" {
		t.Fatalf("any of = %q %v", clause, err)
	}
	if values := args[0].([]string); strings.Join(values, ",") != "software,retail" {
		t.Fatalf("any of values = %v, want trimmed, lowercased, blanks dropped", values)
	}

	listed := FieldDefinition{Expression: "(props ->> 'tools')", Type: FieldTypeText, ElementSource: "json_each(props)"}
	clause, args, err = buildClause(listed, model.QueryFilterRule{Field: "tools", Operator: model.QueryFilterOpIsNoneOf, Values: []string{"zendesk"}})
	want := "NOT COALESCE(((LOWER((props ->> 'tools')) IN ? OR EXISTS (SELECT 1 FROM json_each(props) WHERE LOWER(value) IN ?))), FALSE)"
	if err != nil || clause != want || len(args) != 2 {
		t.Fatalf("none of = %q (%d args) %v\nwant %q", clause, len(args), err, want)
	}

	id := FieldDefinition{Column: "owner_member_id", Type: FieldTypeID}
	if clause, _, _ := buildClause(id, model.QueryFilterRule{Field: "owner", Operator: model.QueryFilterOpIsAnyOf, Values: []string{"A"}}); clause != "owner_member_id IN ?" {
		t.Fatalf("id any of = %q, want exact (case-sensitive) match", clause)
	}
}

func TestNumericExpressionEnablesComparisonOperators(t *testing.T) {
	definition := FieldDefinition{Expression: "(props ->> 'n')", Type: FieldTypeText, NumericExpression: "NUM(n)"}
	value := "5"
	clause, args, err := buildClause(definition, model.QueryFilterRule{Field: "n", Operator: model.QueryFilterOpGT, Value: &value})
	if err != nil || clause != "NUM(n) > ?" || args[0] != 5.0 {
		t.Fatalf("gt = %q %v %v", clause, args, err)
	}
	if !isOperatorAllowed(definition, model.QueryFilterOpBetween) {
		t.Fatal("between should be allowed with a numeric expression")
	}
	if isOperatorAllowed(FieldDefinition{Column: "name", Type: FieldTypeText}, model.QueryFilterOpGT) {
		t.Fatal("gt must stay disallowed on plain text fields")
	}
}
