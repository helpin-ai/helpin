package service

import (
	"context"
	"encoding/json"
	"testing"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// addBonusExtraTables creates the tables required by RewardBonusService and
// RewardScoringRepository that are not part of the shared newTestDB helper.
func addBonusExtraTables(t *testing.T, db *gorm.DB) {
	t.Helper()

	tables := []string{
		`CREATE TABLE IF NOT EXISTS reward_bonus_calculations (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL,
			quarter_id TEXT NOT NULL,
			employee_id TEXT NOT NULL,
			final_amount REAL NOT NULL DEFAULT 0,
			bonus_tier TEXT,
			team_tqi INTEGER,
			individual_iqi INTEGER,
			final_score INTEGER,
			base_salary REAL NOT NULL DEFAULT 0,
			is_override BOOLEAN NOT NULL DEFAULT 0,
			override_reason TEXT,
			override_applied_by TEXT,
			override_applied_at DATETIME,
			calculation_details TEXT,
			locked_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME,
			UNIQUE(workspace_id, quarter_id, employee_id)
		)`,
		`CREATE TABLE IF NOT EXISTS reward_finance_settings (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL,
			quarter_id TEXT NOT NULL,
			mrr_start REAL NOT NULL DEFAULT 0,
			mrr_end REAL NOT NULL DEFAULT 0,
			bonus_pool_percentage INTEGER NOT NULL DEFAULT 0,
			max_bonus_pool REAL,
			team_weight INTEGER NOT NULL DEFAULT 50,
			bonus_tiers TEXT,
			total_pool REAL NOT NULL DEFAULT 0,
			total_paid REAL NOT NULL DEFAULT 0,
			pool_utilization REAL NOT NULL DEFAULT 0,
			budget_factor REAL NOT NULL DEFAULT 0,
			total_basic_salary REAL NOT NULL DEFAULT 0,
			locked_at DATETIME,
			locked_by TEXT,
			created_at DATETIME,
			updated_at DATETIME,
			UNIQUE(workspace_id, quarter_id)
		)`,
		`CREATE TABLE IF NOT EXISTS reward_audit_log (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL,
			quarter_id TEXT NOT NULL,
			action TEXT NOT NULL,
			employee_id TEXT,
			performed_by TEXT NOT NULL,
			performed_by_name TEXT,
			performed_by_role TEXT,
			old_value TEXT,
			new_value TEXT,
			justification TEXT,
			affected_count INTEGER NOT NULL DEFAULT 0,
			performed_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS reward_individual_checks (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			sprint_id TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			employee_id TEXT NOT NULL,
			scored_by TEXT,
			criteria_id TEXT NOT NULL,
			answer BOOLEAN NOT NULL DEFAULT 0,
			notes TEXT,
			created_at DATETIME,
			updated_at DATETIME,
			UNIQUE(sprint_id, employee_id, criteria_id)
		)`,
	}

	for _, stmt := range tables {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create bonus table: %v", err)
		}
	}
}

// newBonusTestEnv creates a fully-wired test environment with the DB, repos,
// service, and a pre-seeded workspace + member.
type bonusTestEnv struct {
	db          *gorm.DB
	svc         *RewardBonusService
	workspaceID string
	memberID    string
	quarterID   string
}

func newBonusTestEnv(t *testing.T) *bonusTestEnv {
	t.Helper()

	db := newTestDB(t)
	addBonusExtraTables(t, db)

	bonusRepo := repository.NewRewardBonusRepository(db)
	scoringRepo := repository.NewRewardScoringRepository(db)
	svc := NewRewardBonusService(bonusRepo, scoringRepo)

	wsID := "ws-bonus-001"
	ownerID := "user-bonus-001"
	memberID := "member-bonus-001"
	quarterID := "quarter-bonus-001"

	seedUser(t, db, ownerID, "bonus-owner@test.com", "Bonus Owner", "hashed")
	seedWorkspace(t, db, wsID, "Bonus Test Workspace", "bonus-test", ownerID)
	seedWorkspaceMember(t, db, memberID, wsID, ownerID, "bonus-owner@test.com", "Bonus Owner", "owner")

	return &bonusTestEnv{
		db:          db,
		svc:         svc,
		workspaceID: wsID,
		memberID:    memberID,
		quarterID:   quarterID,
	}
}

// ---------------------------------------------------------------------------
// SaveCalculations
// ---------------------------------------------------------------------------

func TestSaveCalculations_Single(t *testing.T) {
	env := newBonusTestEnv(t)
	ctx := context.Background()

	tier := "gold"
	tqi := 85
	iqi := 90
	score := 88

	calcs := []model.RewardBonusCalculation{
		{
			WorkspaceID:   env.workspaceID,
			QuarterID:     env.quarterID,
			EmployeeID:    env.memberID,
			FinalAmount:   5000.50,
			BonusTier:     &tier,
			TeamTQI:       &tqi,
			IndividualIQI: &iqi,
			FinalScore:    &score,
			BaseSalary:    60000,
		},
	}

	if err := env.svc.SaveCalculations(ctx, calcs); err != nil {
		t.Fatalf("SaveCalculations: %v", err)
	}

	// Verify via GetCalculations
	got, err := env.svc.GetCalculations(ctx, env.workspaceID, env.quarterID)
	if err != nil {
		t.Fatalf("GetCalculations: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 calculation, got %d", len(got))
	}
	if got[0].FinalAmount != 5000.50 {
		t.Errorf("FinalAmount = %v, want 5000.50", got[0].FinalAmount)
	}
	if got[0].BaseSalary != 60000 {
		t.Errorf("BaseSalary = %v, want 60000", got[0].BaseSalary)
	}
	if got[0].BonusTier == nil || *got[0].BonusTier != "gold" {
		t.Errorf("BonusTier = %v, want gold", got[0].BonusTier)
	}
	if got[0].EmployeeID != env.memberID {
		t.Errorf("EmployeeID = %v, want %v", got[0].EmployeeID, env.memberID)
	}
}

func TestSaveCalculations_Multiple(t *testing.T) {
	env := newBonusTestEnv(t)
	ctx := context.Background()

	// Add a second member
	member2ID := "member-bonus-002"
	seedUser(t, env.db, "user-bonus-002", "bonus-emp2@test.com", "Employee Two", "hashed")
	seedWorkspaceMember(t, env.db, member2ID, env.workspaceID, "user-bonus-002", "bonus-emp2@test.com", "Employee Two", "member")

	calcs := []model.RewardBonusCalculation{
		{
			WorkspaceID: env.workspaceID,
			QuarterID:   env.quarterID,
			EmployeeID:  env.memberID,
			FinalAmount: 3000,
			BaseSalary:  50000,
		},
		{
			WorkspaceID: env.workspaceID,
			QuarterID:   env.quarterID,
			EmployeeID:  member2ID,
			FinalAmount: 4000,
			BaseSalary:  55000,
		},
	}

	if err := env.svc.SaveCalculations(ctx, calcs); err != nil {
		t.Fatalf("SaveCalculations: %v", err)
	}

	got, err := env.svc.GetCalculations(ctx, env.workspaceID, env.quarterID)
	if err != nil {
		t.Fatalf("GetCalculations: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 calculations, got %d", len(got))
	}
}

func TestSaveCalculations_Upsert(t *testing.T) {
	env := newBonusTestEnv(t)
	ctx := context.Background()

	// First save
	calcs := []model.RewardBonusCalculation{
		{
			WorkspaceID: env.workspaceID,
			QuarterID:   env.quarterID,
			EmployeeID:  env.memberID,
			FinalAmount: 1000,
			BaseSalary:  40000,
		},
	}
	if err := env.svc.SaveCalculations(ctx, calcs); err != nil {
		t.Fatalf("first SaveCalculations: %v", err)
	}

	// Update the same record
	calcs[0].FinalAmount = 2500
	calcs[0].BaseSalary = 45000
	if err := env.svc.SaveCalculations(ctx, calcs); err != nil {
		t.Fatalf("second SaveCalculations: %v", err)
	}

	got, err := env.svc.GetCalculations(ctx, env.workspaceID, env.quarterID)
	if err != nil {
		t.Fatalf("GetCalculations: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 calculation after upsert, got %d", len(got))
	}
	if got[0].FinalAmount != 2500 {
		t.Errorf("FinalAmount after upsert = %v, want 2500", got[0].FinalAmount)
	}
	if got[0].BaseSalary != 45000 {
		t.Errorf("BaseSalary after upsert = %v, want 45000", got[0].BaseSalary)
	}
}

func TestSaveCalculations_WithOverride(t *testing.T) {
	env := newBonusTestEnv(t)
	ctx := context.Background()

	reason := "Exceptional performance"
	appliedBy := "user-bonus-001"

	calcs := []model.RewardBonusCalculation{
		{
			WorkspaceID:       env.workspaceID,
			QuarterID:         env.quarterID,
			EmployeeID:        env.memberID,
			FinalAmount:       7500,
			BaseSalary:        60000,
			IsOverride:        true,
			OverrideReason:    &reason,
			OverrideAppliedBy: &appliedBy,
		},
	}

	if err := env.svc.SaveCalculations(ctx, calcs); err != nil {
		t.Fatalf("SaveCalculations: %v", err)
	}

	got, err := env.svc.GetCalculations(ctx, env.workspaceID, env.quarterID)
	if err != nil {
		t.Fatalf("GetCalculations: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1, got %d", len(got))
	}
	if !got[0].IsOverride {
		t.Error("IsOverride should be true")
	}
	if got[0].OverrideReason == nil || *got[0].OverrideReason != reason {
		t.Errorf("OverrideReason = %v, want %q", got[0].OverrideReason, reason)
	}
}

func TestSaveCalculations_WithCalculationDetails(t *testing.T) {
	env := newBonusTestEnv(t)
	ctx := context.Background()

	details := json.RawMessage(`{"breakdown":{"team":60,"individual":40},"notes":"Q1 calc"}`)

	calcs := []model.RewardBonusCalculation{
		{
			WorkspaceID:        env.workspaceID,
			QuarterID:          env.quarterID,
			EmployeeID:         env.memberID,
			FinalAmount:        3000,
			BaseSalary:         50000,
			CalculationDetails: details,
		},
	}

	if err := env.svc.SaveCalculations(ctx, calcs); err != nil {
		t.Fatalf("SaveCalculations: %v", err)
	}

	got, err := env.svc.GetCalculations(ctx, env.workspaceID, env.quarterID)
	if err != nil {
		t.Fatalf("GetCalculations: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1, got %d", len(got))
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(got[0].CalculationDetails, &parsed); err != nil {
		t.Fatalf("unmarshal CalculationDetails: %v", err)
	}
	if parsed["notes"] != "Q1 calc" {
		t.Errorf("CalculationDetails.notes = %v, want Q1 calc", parsed["notes"])
	}
}

func TestSaveCalculations_InvalidEmployee(t *testing.T) {
	env := newBonusTestEnv(t)
	ctx := context.Background()

	calcs := []model.RewardBonusCalculation{
		{
			WorkspaceID: env.workspaceID,
			QuarterID:   env.quarterID,
			EmployeeID:  "nonexistent-member",
			FinalAmount: 1000,
			BaseSalary:  40000,
		},
	}

	err := env.svc.SaveCalculations(ctx, calcs)
	if err == nil {
		t.Fatal("expected error for invalid employee, got nil")
	}
}

// ---------------------------------------------------------------------------
// GetCalculations
// ---------------------------------------------------------------------------

func TestGetCalculations_Empty(t *testing.T) {
	env := newBonusTestEnv(t)
	ctx := context.Background()

	got, err := env.svc.GetCalculations(ctx, env.workspaceID, env.quarterID)
	if err != nil {
		t.Fatalf("GetCalculations: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected 0 calculations, got %d", len(got))
	}
}

func TestGetCalculations_DifferentQuarters(t *testing.T) {
	env := newBonusTestEnv(t)
	ctx := context.Background()

	q1 := "quarter-q1"
	q2 := "quarter-q2"

	calcs1 := []model.RewardBonusCalculation{
		{
			WorkspaceID: env.workspaceID,
			QuarterID:   q1,
			EmployeeID:  env.memberID,
			FinalAmount: 1000,
			BaseSalary:  40000,
		},
	}
	calcs2 := []model.RewardBonusCalculation{
		{
			WorkspaceID: env.workspaceID,
			QuarterID:   q2,
			EmployeeID:  env.memberID,
			FinalAmount: 2000,
			BaseSalary:  45000,
		},
	}

	if err := env.svc.SaveCalculations(ctx, calcs1); err != nil {
		t.Fatalf("SaveCalculations Q1: %v", err)
	}
	if err := env.svc.SaveCalculations(ctx, calcs2); err != nil {
		t.Fatalf("SaveCalculations Q2: %v", err)
	}

	gotQ1, err := env.svc.GetCalculations(ctx, env.workspaceID, q1)
	if err != nil {
		t.Fatalf("GetCalculations Q1: %v", err)
	}
	if len(gotQ1) != 1 {
		t.Fatalf("Q1: expected 1, got %d", len(gotQ1))
	}
	if gotQ1[0].FinalAmount != 1000 {
		t.Errorf("Q1 FinalAmount = %v, want 1000", gotQ1[0].FinalAmount)
	}

	gotQ2, err := env.svc.GetCalculations(ctx, env.workspaceID, q2)
	if err != nil {
		t.Fatalf("GetCalculations Q2: %v", err)
	}
	if len(gotQ2) != 1 {
		t.Fatalf("Q2: expected 1, got %d", len(gotQ2))
	}
	if gotQ2[0].FinalAmount != 2000 {
		t.Errorf("Q2 FinalAmount = %v, want 2000", gotQ2[0].FinalAmount)
	}
}

func TestGetCalculations_DifferentWorkspaces(t *testing.T) {
	env := newBonusTestEnv(t)
	ctx := context.Background()

	// Create a second workspace with its own member
	ws2 := "ws-bonus-002"
	owner2 := "user-bonus-ws2"
	member2 := "member-bonus-ws2"
	seedUser(t, env.db, owner2, "ws2-owner@test.com", "WS2 Owner", "hashed")
	seedWorkspace(t, env.db, ws2, "Bonus WS2", "bonus-test-2", owner2)
	seedWorkspaceMember(t, env.db, member2, ws2, owner2, "ws2-owner@test.com", "WS2 Owner", "owner")

	calcs1 := []model.RewardBonusCalculation{
		{WorkspaceID: env.workspaceID, QuarterID: env.quarterID, EmployeeID: env.memberID, FinalAmount: 1000, BaseSalary: 40000},
	}
	calcs2 := []model.RewardBonusCalculation{
		{WorkspaceID: ws2, QuarterID: env.quarterID, EmployeeID: member2, FinalAmount: 9999, BaseSalary: 80000},
	}

	if err := env.svc.SaveCalculations(ctx, calcs1); err != nil {
		t.Fatalf("SaveCalculations ws1: %v", err)
	}
	if err := env.svc.SaveCalculations(ctx, calcs2); err != nil {
		t.Fatalf("SaveCalculations ws2: %v", err)
	}

	got1, _ := env.svc.GetCalculations(ctx, env.workspaceID, env.quarterID)
	got2, _ := env.svc.GetCalculations(ctx, ws2, env.quarterID)

	if len(got1) != 1 || got1[0].FinalAmount != 1000 {
		t.Errorf("ws1: unexpected result %v", got1)
	}
	if len(got2) != 1 || got2[0].FinalAmount != 9999 {
		t.Errorf("ws2: unexpected result %v", got2)
	}
}

// ---------------------------------------------------------------------------
// Lock / Unlock
// ---------------------------------------------------------------------------

func TestLockUnlock(t *testing.T) {
	env := newBonusTestEnv(t)
	ctx := context.Background()

	calcs := []model.RewardBonusCalculation{
		{WorkspaceID: env.workspaceID, QuarterID: env.quarterID, EmployeeID: env.memberID, FinalAmount: 1000, BaseSalary: 40000},
	}
	if err := env.svc.SaveCalculations(ctx, calcs); err != nil {
		t.Fatalf("SaveCalculations: %v", err)
	}

	// Initially unlocked
	got, _ := env.svc.GetCalculations(ctx, env.workspaceID, env.quarterID)
	if got[0].LockedAt != nil {
		t.Fatal("expected LockedAt to be nil initially")
	}

	// Lock
	if err := env.svc.Lock(ctx, env.workspaceID, env.quarterID); err != nil {
		t.Fatalf("Lock: %v", err)
	}

	got, _ = env.svc.GetCalculations(ctx, env.workspaceID, env.quarterID)
	if got[0].LockedAt == nil {
		t.Fatal("expected LockedAt to be set after Lock")
	}

	// Unlock
	if err := env.svc.Unlock(ctx, env.workspaceID, env.quarterID); err != nil {
		t.Fatalf("Unlock: %v", err)
	}

	got, _ = env.svc.GetCalculations(ctx, env.workspaceID, env.quarterID)
	if got[0].LockedAt != nil {
		t.Fatal("expected LockedAt to be nil after Unlock")
	}
}

func TestLock_MultipleCalculations(t *testing.T) {
	env := newBonusTestEnv(t)
	ctx := context.Background()

	member2ID := "member-bonus-lock2"
	seedUser(t, env.db, "user-bonus-lock2", "lock2@test.com", "Lock Two", "hashed")
	seedWorkspaceMember(t, env.db, member2ID, env.workspaceID, "user-bonus-lock2", "lock2@test.com", "Lock Two", "member")

	calcs := []model.RewardBonusCalculation{
		{WorkspaceID: env.workspaceID, QuarterID: env.quarterID, EmployeeID: env.memberID, FinalAmount: 1000, BaseSalary: 40000},
		{WorkspaceID: env.workspaceID, QuarterID: env.quarterID, EmployeeID: member2ID, FinalAmount: 2000, BaseSalary: 50000},
	}
	if err := env.svc.SaveCalculations(ctx, calcs); err != nil {
		t.Fatalf("SaveCalculations: %v", err)
	}

	if err := env.svc.Lock(ctx, env.workspaceID, env.quarterID); err != nil {
		t.Fatalf("Lock: %v", err)
	}

	got, _ := env.svc.GetCalculations(ctx, env.workspaceID, env.quarterID)
	for i, c := range got {
		if c.LockedAt == nil {
			t.Errorf("calculation %d: expected LockedAt to be set", i)
		}
	}
}

func TestLock_OnlyAffectsTargetQuarter(t *testing.T) {
	env := newBonusTestEnv(t)
	ctx := context.Background()

	q1 := "quarter-lock-q1"
	q2 := "quarter-lock-q2"

	calcsQ1 := []model.RewardBonusCalculation{
		{WorkspaceID: env.workspaceID, QuarterID: q1, EmployeeID: env.memberID, FinalAmount: 1000, BaseSalary: 40000},
	}
	calcsQ2 := []model.RewardBonusCalculation{
		{WorkspaceID: env.workspaceID, QuarterID: q2, EmployeeID: env.memberID, FinalAmount: 2000, BaseSalary: 50000},
	}

	if err := env.svc.SaveCalculations(ctx, calcsQ1); err != nil {
		t.Fatalf("SaveCalculations Q1: %v", err)
	}
	if err := env.svc.SaveCalculations(ctx, calcsQ2); err != nil {
		t.Fatalf("SaveCalculations Q2: %v", err)
	}

	// Lock only Q1
	if err := env.svc.Lock(ctx, env.workspaceID, q1); err != nil {
		t.Fatalf("Lock Q1: %v", err)
	}

	gotQ1, _ := env.svc.GetCalculations(ctx, env.workspaceID, q1)
	gotQ2, _ := env.svc.GetCalculations(ctx, env.workspaceID, q2)

	if gotQ1[0].LockedAt == nil {
		t.Error("Q1 should be locked")
	}
	if gotQ2[0].LockedAt != nil {
		t.Error("Q2 should NOT be locked")
	}
}

func TestLock_NoCalculations_NoError(t *testing.T) {
	env := newBonusTestEnv(t)
	ctx := context.Background()

	// Lock with no existing calculations should not error
	if err := env.svc.Lock(ctx, env.workspaceID, "nonexistent-quarter"); err != nil {
		t.Fatalf("Lock on empty set: %v", err)
	}
}

func TestUnlock_NoCalculations_NoError(t *testing.T) {
	env := newBonusTestEnv(t)
	ctx := context.Background()

	if err := env.svc.Unlock(ctx, env.workspaceID, "nonexistent-quarter"); err != nil {
		t.Fatalf("Unlock on empty set: %v", err)
	}
}

// ---------------------------------------------------------------------------
// GetFinance
// ---------------------------------------------------------------------------

func TestGetFinance_NotFound(t *testing.T) {
	env := newBonusTestEnv(t)
	ctx := context.Background()

	fs, err := env.svc.GetFinance(ctx, env.workspaceID, env.quarterID)
	if err != nil {
		t.Fatalf("GetFinance: %v", err)
	}
	if fs != nil {
		t.Fatalf("expected nil for non-existent finance settings, got %+v", fs)
	}
}

func TestGetFinance_AfterUpsert(t *testing.T) {
	env := newBonusTestEnv(t)
	ctx := context.Background()

	maxPool := 100000.0
	req := model.UpsertRewardFinanceRequest{
		WorkspaceID:         env.workspaceID,
		QuarterID:           env.quarterID,
		MRRStart:            50000,
		MRREnd:              75000,
		BonusPoolPercentage: 15,
		MaxBonusPool:        &maxPool,
		TeamWeight:          60,
		BonusTiers:          json.RawMessage(`[{"name":"gold","min":90},{"name":"silver","min":70}]`),
		TotalPool:           37500,
		TotalPaid:           30000,
		PoolUtilization:     0.80,
		BudgetFactor:        1.2,
		TotalBasicSalary:    200000,
	}

	if _, err := env.svc.UpsertFinance(ctx, req); err != nil {
		t.Fatalf("UpsertFinance: %v", err)
	}

	fs, err := env.svc.GetFinance(ctx, env.workspaceID, env.quarterID)
	if err != nil {
		t.Fatalf("GetFinance: %v", err)
	}
	if fs == nil {
		t.Fatal("expected non-nil finance settings")
	}
	if fs.MRRStart != 50000 {
		t.Errorf("MRRStart = %v, want 50000", fs.MRRStart)
	}
	if fs.MRREnd != 75000 {
		t.Errorf("MRREnd = %v, want 75000", fs.MRREnd)
	}
	if fs.BonusPoolPercentage != 15 {
		t.Errorf("BonusPoolPercentage = %v, want 15", fs.BonusPoolPercentage)
	}
	if fs.TeamWeight != 60 {
		t.Errorf("TeamWeight = %v, want 60", fs.TeamWeight)
	}
	if fs.TotalPool != 37500 {
		t.Errorf("TotalPool = %v, want 37500", fs.TotalPool)
	}
}

// ---------------------------------------------------------------------------
// UpsertFinance
// ---------------------------------------------------------------------------

func TestUpsertFinance_Create(t *testing.T) {
	env := newBonusTestEnv(t)
	ctx := context.Background()

	req := model.UpsertRewardFinanceRequest{
		WorkspaceID:         env.workspaceID,
		QuarterID:           env.quarterID,
		MRRStart:            10000,
		MRREnd:              20000,
		BonusPoolPercentage: 10,
		TeamWeight:          50,
		TotalPool:           5000,
		TotalBasicSalary:    100000,
	}

	fs, err := env.svc.UpsertFinance(ctx, req)
	if err != nil {
		t.Fatalf("UpsertFinance create: %v", err)
	}
	if fs == nil {
		t.Fatal("expected non-nil result")
	}
	if fs.ID == "" {
		t.Error("expected generated ID")
	}
	if fs.WorkspaceID != env.workspaceID {
		t.Errorf("WorkspaceID = %v, want %v", fs.WorkspaceID, env.workspaceID)
	}
	if fs.QuarterID != env.quarterID {
		t.Errorf("QuarterID = %v, want %v", fs.QuarterID, env.quarterID)
	}
	if fs.MRRStart != 10000 {
		t.Errorf("MRRStart = %v, want 10000", fs.MRRStart)
	}
	if fs.TotalBasicSalary != 100000 {
		t.Errorf("TotalBasicSalary = %v, want 100000", fs.TotalBasicSalary)
	}
}

func TestUpsertFinance_Update(t *testing.T) {
	env := newBonusTestEnv(t)
	ctx := context.Background()

	// Create
	req := model.UpsertRewardFinanceRequest{
		WorkspaceID:         env.workspaceID,
		QuarterID:           env.quarterID,
		MRRStart:            10000,
		MRREnd:              20000,
		BonusPoolPercentage: 10,
		TeamWeight:          50,
		TotalPool:           5000,
		TotalBasicSalary:    100000,
	}
	created, err := env.svc.UpsertFinance(ctx, req)
	if err != nil {
		t.Fatalf("UpsertFinance create: %v", err)
	}

	// Update the same workspace+quarter
	req.MRREnd = 30000
	req.BonusPoolPercentage = 20
	req.TotalPool = 10000
	req.TotalPaid = 8000
	req.PoolUtilization = 0.80

	updated, err := env.svc.UpsertFinance(ctx, req)
	if err != nil {
		t.Fatalf("UpsertFinance update: %v", err)
	}

	// Same record, same ID
	if updated.ID != created.ID {
		t.Errorf("expected same ID after upsert, got %v vs %v", updated.ID, created.ID)
	}
	if updated.MRREnd != 30000 {
		t.Errorf("MRREnd = %v, want 30000", updated.MRREnd)
	}
	if updated.BonusPoolPercentage != 20 {
		t.Errorf("BonusPoolPercentage = %v, want 20", updated.BonusPoolPercentage)
	}
	if updated.TotalPool != 10000 {
		t.Errorf("TotalPool = %v, want 10000", updated.TotalPool)
	}
	if updated.TotalPaid != 8000 {
		t.Errorf("TotalPaid = %v, want 8000", updated.TotalPaid)
	}
	if updated.PoolUtilization != 0.80 {
		t.Errorf("PoolUtilization = %v, want 0.80", updated.PoolUtilization)
	}
}

func TestUpsertFinance_WithMaxBonusPool(t *testing.T) {
	env := newBonusTestEnv(t)
	ctx := context.Background()

	maxPool := 50000.0
	req := model.UpsertRewardFinanceRequest{
		WorkspaceID:         env.workspaceID,
		QuarterID:           env.quarterID,
		MRRStart:            10000,
		MRREnd:              20000,
		BonusPoolPercentage: 10,
		MaxBonusPool:        &maxPool,
		TeamWeight:          50,
		TotalPool:           5000,
		TotalBasicSalary:    100000,
	}

	fs, err := env.svc.UpsertFinance(ctx, req)
	if err != nil {
		t.Fatalf("UpsertFinance: %v", err)
	}
	if fs.MaxBonusPool == nil || *fs.MaxBonusPool != 50000 {
		t.Errorf("MaxBonusPool = %v, want 50000", fs.MaxBonusPool)
	}
}

func TestUpsertFinance_WithBonusTiers(t *testing.T) {
	env := newBonusTestEnv(t)
	ctx := context.Background()

	tiers := json.RawMessage(`[{"name":"platinum","min":95,"multiplier":1.5},{"name":"gold","min":85,"multiplier":1.2}]`)
	req := model.UpsertRewardFinanceRequest{
		WorkspaceID:         env.workspaceID,
		QuarterID:           env.quarterID,
		MRRStart:            10000,
		MRREnd:              20000,
		BonusPoolPercentage: 10,
		TeamWeight:          50,
		BonusTiers:          tiers,
		TotalPool:           5000,
		TotalBasicSalary:    100000,
	}

	fs, err := env.svc.UpsertFinance(ctx, req)
	if err != nil {
		t.Fatalf("UpsertFinance: %v", err)
	}

	var parsed []map[string]interface{}
	if err := json.Unmarshal(fs.BonusTiers, &parsed); err != nil {
		t.Fatalf("unmarshal BonusTiers: %v", err)
	}
	if len(parsed) != 2 {
		t.Fatalf("expected 2 tiers, got %d", len(parsed))
	}
	if parsed[0]["name"] != "platinum" {
		t.Errorf("first tier name = %v, want platinum", parsed[0]["name"])
	}
}

func TestUpsertFinance_DifferentQuartersIndependent(t *testing.T) {
	env := newBonusTestEnv(t)
	ctx := context.Background()

	q1 := "quarter-fin-q1"
	q2 := "quarter-fin-q2"

	reqQ1 := model.UpsertRewardFinanceRequest{
		WorkspaceID: env.workspaceID, QuarterID: q1,
		MRRStart: 10000, MRREnd: 20000, BonusPoolPercentage: 10,
		TeamWeight: 50, TotalPool: 5000, TotalBasicSalary: 100000,
	}
	reqQ2 := model.UpsertRewardFinanceRequest{
		WorkspaceID: env.workspaceID, QuarterID: q2,
		MRRStart: 20000, MRREnd: 40000, BonusPoolPercentage: 15,
		TeamWeight: 60, TotalPool: 12000, TotalBasicSalary: 200000,
	}

	if _, err := env.svc.UpsertFinance(ctx, reqQ1); err != nil {
		t.Fatalf("UpsertFinance Q1: %v", err)
	}
	if _, err := env.svc.UpsertFinance(ctx, reqQ2); err != nil {
		t.Fatalf("UpsertFinance Q2: %v", err)
	}

	fsQ1, _ := env.svc.GetFinance(ctx, env.workspaceID, q1)
	fsQ2, _ := env.svc.GetFinance(ctx, env.workspaceID, q2)

	if fsQ1.TotalPool != 5000 {
		t.Errorf("Q1 TotalPool = %v, want 5000", fsQ1.TotalPool)
	}
	if fsQ2.TotalPool != 12000 {
		t.Errorf("Q2 TotalPool = %v, want 12000", fsQ2.TotalPool)
	}
	if fsQ1.ID == fsQ2.ID {
		t.Error("Q1 and Q2 should have different IDs")
	}
}

func TestUpsertFinance_BudgetFactor(t *testing.T) {
	env := newBonusTestEnv(t)
	ctx := context.Background()

	req := model.UpsertRewardFinanceRequest{
		WorkspaceID:         env.workspaceID,
		QuarterID:           env.quarterID,
		MRRStart:            10000,
		MRREnd:              20000,
		BonusPoolPercentage: 10,
		TeamWeight:          50,
		TotalPool:           5000,
		BudgetFactor:        1.35,
		TotalBasicSalary:    100000,
	}

	fs, err := env.svc.UpsertFinance(ctx, req)
	if err != nil {
		t.Fatalf("UpsertFinance: %v", err)
	}
	if fs.BudgetFactor != 1.35 {
		t.Errorf("BudgetFactor = %v, want 1.35", fs.BudgetFactor)
	}
}

// ---------------------------------------------------------------------------
// GetTeamSprintData
// ---------------------------------------------------------------------------

func TestGetTeamSprintData_Empty(t *testing.T) {
	env := newBonusTestEnv(t)
	ctx := context.Background()

	checks, err := env.svc.GetTeamSprintData(ctx, "sprint-nonexistent")
	if err != nil {
		t.Fatalf("GetTeamSprintData: %v", err)
	}
	if len(checks) != 0 {
		t.Fatalf("expected 0 checks, got %d", len(checks))
	}
}

func TestGetTeamSprintData_WithData(t *testing.T) {
	env := newBonusTestEnv(t)
	ctx := context.Background()

	sprintID := "sprint-bonus-001"

	// Seed individual checks directly in the DB
	mustExec(t, env.db,
		`INSERT INTO reward_individual_checks (id, sprint_id, workspace_id, employee_id, criteria_id, answer, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		"check-001", sprintID, env.workspaceID, env.memberID, "criteria-a", true)
	mustExec(t, env.db,
		`INSERT INTO reward_individual_checks (id, sprint_id, workspace_id, employee_id, criteria_id, answer, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		"check-002", sprintID, env.workspaceID, env.memberID, "criteria-b", false)

	checks, err := env.svc.GetTeamSprintData(ctx, sprintID)
	if err != nil {
		t.Fatalf("GetTeamSprintData: %v", err)
	}
	if len(checks) != 2 {
		t.Fatalf("expected 2 checks, got %d", len(checks))
	}

	// Ordered by employee_id, criteria_id
	if checks[0].CriteriaID != "criteria-a" {
		t.Errorf("first check criteria_id = %v, want criteria-a", checks[0].CriteriaID)
	}
	if checks[0].Answer != true {
		t.Error("first check answer should be true")
	}
	if checks[1].CriteriaID != "criteria-b" {
		t.Errorf("second check criteria_id = %v, want criteria-b", checks[1].CriteriaID)
	}
	if checks[1].Answer != false {
		t.Error("second check answer should be false")
	}
}

func TestGetTeamSprintData_OnlyReturnsSprint(t *testing.T) {
	env := newBonusTestEnv(t)
	ctx := context.Background()

	sprint1 := "sprint-filter-001"
	sprint2 := "sprint-filter-002"

	mustExec(t, env.db,
		`INSERT INTO reward_individual_checks (id, sprint_id, workspace_id, employee_id, criteria_id, answer, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		"check-f1", sprint1, env.workspaceID, env.memberID, "crit-1", true)
	mustExec(t, env.db,
		`INSERT INTO reward_individual_checks (id, sprint_id, workspace_id, employee_id, criteria_id, answer, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		"check-f2", sprint2, env.workspaceID, env.memberID, "crit-1", false)

	checks, err := env.svc.GetTeamSprintData(ctx, sprint1)
	if err != nil {
		t.Fatalf("GetTeamSprintData: %v", err)
	}
	if len(checks) != 1 {
		t.Fatalf("expected 1 check for sprint1, got %d", len(checks))
	}
	if checks[0].SprintID != sprint1 {
		t.Errorf("SprintID = %v, want %v", checks[0].SprintID, sprint1)
	}
}
