package service

import (
	"context"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestDealAutomationRequiresCompletedSignalActivation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:crm-deal-activation?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	statements := []string{
		`CREATE TABLE crm_signal_rule_configs (id TEXT PRIMARY KEY, workspace_id TEXT, rule_key TEXT, version INTEGER, cadence TEXT, enabled BOOLEAN, shadow_mode BOOLEAN, activation_eligible BOOLEAN, thresholds BLOB, business_weight REAL, half_life_days REAL, created_at DATETIME, updated_at DATETIME)`,
		`CREATE TABLE crm_signal_routing_policies (id TEXT PRIMARY KEY, workspace_id TEXT, version INTEGER, enabled BOOLEAN, minimum_priority REAL, required_trust TEXT, route_to_owner BOOLEAN, destination_team_id TEXT, channels BLOB, created_by_member_id TEXT, created_at DATETIME)`,
		`CREATE TABLE crm_signal_deliveries (id TEXT PRIMARY KEY, workspace_id TEXT, signal_id TEXT, policy_id TEXT, policy_version INTEGER, channel TEXT, recipient_member_id TEXT, destination_team_id TEXT, status TEXT, delivered_at DATETIME, created_at DATETIME)`,
		`INSERT INTO crm_signal_rule_configs (id, workspace_id, rule_key, version, enabled, shadow_mode, activation_eligible) VALUES ('rule-1', 'ws-1', 'known_contact_returned', 3, 1, 0, 1)`,
		`INSERT INTO crm_signal_routing_policies (id, workspace_id, version, enabled, minimum_priority, required_trust, route_to_owner, channels, created_by_member_id) VALUES ('policy-1', 'ws-1', 1, 1, 20, 'verified', 1, '["feed"]', 'member-1')`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("schema/seed: %v", err)
		}
	}
	repo := repository.NewCRMSignalRepository(db)
	svc := &DealAutomationService{signalRepo: repo}
	ruleKey, version := model.CRMSignalRuleKnownContactReturned, 3
	signal := model.CRMBuyerSignal{ID: "signal-1", WorkspaceID: "ws-1", RuleKey: &ruleKey, RuleVersion: &version, EvidenceIdentityTrust: model.IdentityTrustVerified}

	if svc.allSignalsActivationEligible(context.Background(), "ws-1", []model.CRMBuyerSignal{signal}) {
		t.Fatal("live rule without a completed routing delivery must not execute directly")
	}
	if err := db.Exec(`INSERT INTO crm_signal_deliveries (id, workspace_id, signal_id, policy_id, policy_version, channel, status) VALUES ('delivery-1', 'ws-1', 'signal-1', 'policy-1', 1, 'feed', 'routed')`).Error; err != nil {
		t.Fatalf("seed delivery: %v", err)
	}
	if !svc.allSignalsActivationEligible(context.Background(), "ws-1", []model.CRMBuyerSignal{signal}) {
		t.Fatal("verified exact rule version with active-policy delivery should be eligible")
	}
	signal.EvidenceIdentityTrust = model.IdentityTrustProbabilistic
	if svc.allSignalsActivationEligible(context.Background(), "ws-1", []model.CRMBuyerSignal{signal}) {
		t.Fatal("lower-trust evidence must not execute directly")
	}
}
