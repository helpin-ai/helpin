package model

import "testing"

func TestAIUsageModelsUseBillingTables(t *testing.T) {
	tests := []struct{ got, want string }{
		{got: (AIUsagePeriod{}).TableName(), want: "billing_ai_usage_periods"},
		{got: (AIUsageLedgerEntry{}).TableName(), want: "billing_ai_usage_ledger"},
		{got: (AIUsageReservation{}).TableName(), want: "billing_ai_usage_reservations"},
		{got: (AIUsageSettlement{}).TableName(), want: "billing_ai_usage_settlements"},
		{got: (AIUsageTaskEstimate{}).TableName(), want: "billing_ai_usage_task_estimates"},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("TableName() = %q, want %q", tt.got, tt.want)
		}
	}
}
