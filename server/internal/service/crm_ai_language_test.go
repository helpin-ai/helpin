package service

import "testing"

func TestValidateEnglishCRMNarrativeRejectsChineseOutput(t *testing.T) {
	if err := validateEnglishCRMNarrative("该公司正在评估企业计划，并希望本月作出决定。"); err == nil {
		t.Fatal("expected Chinese CRM narrative to be rejected")
	}
	if err := validateEnglishCRMNarrative("Acme is evaluating the enterprise plan with 李明 as the main stakeholder."); err != nil {
		t.Fatalf("expected English narrative containing a name to pass: %v", err)
	}
}
