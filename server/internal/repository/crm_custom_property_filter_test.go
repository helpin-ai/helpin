package repository

import (
	"strings"
	"testing"
)

func TestCRMCustomPropertyExpressionsHaveNoBindPlaceholders(t *testing.T) {
	for _, dialect := range []string{"postgres", "sqlite"} {
		for name, expression := range map[string]string{
			"text":     crmCustomPropertyTextExpression("crm_companies.custom_properties", dialect, "employees_est"),
			"numeric":  crmCustomPropertyNumericExpression("crm_companies.custom_properties", dialect, "employees_est"),
			"elements": crmCustomPropertyElementSource("crm_companies.custom_properties", dialect, "employees_est"),
		} {
			if strings.Contains(expression, "?") {
				t.Errorf("%s %s expression contains a literal ?, which GORM binds as a parameter: %s", dialect, name, expression)
			}
		}
	}
}

func TestValidCRMCustomPropertyKey(t *testing.T) {
	for key, want := range map[string]bool{
		"fit": true, "employees_est": true, "a1": true,
		"": false, "Fit": false, "1st": false, "fit'--": false, "fit.key": false, strings.Repeat("a", 65): false,
	} {
		if got := ValidCRMCustomPropertyKey(key); got != want {
			t.Errorf("ValidCRMCustomPropertyKey(%q) = %v, want %v", key, got, want)
		}
	}
}
