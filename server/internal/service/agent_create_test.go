package service

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestCreateAgentDefaultsToCodeBuilderPreset(t *testing.T) {
	db := newAgentServiceTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	activitySvc := NewPMActivityService(repository.NewPMActivityRepository(db))
	svc := &AgentService{
		agentRepo:   agentRepo,
		activitySvc: activitySvc,
		wsPublisher: nil,
	}

	created, err := svc.CreateAgent(context.Background(), modelCreateAgentRequest(nil), "user-1")
	if err != nil {
		t.Fatalf("CreateAgent returned error: %v", err)
	}

	if created.PresetKey != model.AgentPresetCodeBuilder {
		t.Fatalf("expected code builder preset, got %q", created.PresetKey)
	}
	if created.Role != "Code Builder" {
		t.Fatalf("expected default role Code Builder, got %q", created.Role)
	}
	if created.RuntimeKind != "opencode" {
		t.Fatalf("expected default runtime opencode, got %q", created.RuntimeKind)
	}
}

func TestCreatePlannerPersistsDefaultSystemPrompt(t *testing.T) {
	db := newAgentServiceTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	activitySvc := NewPMActivityService(repository.NewPMActivityRepository(db))
	svc := &AgentService{
		agentRepo:   agentRepo,
		activitySvc: activitySvc,
		wsPublisher: nil,
	}

	presetKey := model.AgentPresetEpicPlanner
	created, err := svc.CreateAgent(context.Background(), modelCreateAgentRequest(&presetKey), "user-1")
	if err != nil {
		t.Fatalf("CreateAgent returned error: %v", err)
	}

	if created.SystemPrompt == nil || *created.SystemPrompt == "" {
		t.Fatal("expected planner default system prompt to be persisted")
	}
	if created.PlanningNotes != nil {
		t.Fatalf("expected planner planning notes to be empty after create, got %+v", created.PlanningNotes)
	}
	if created.PresetKey != model.AgentPresetEpicPlanner {
		t.Fatalf("expected epic planner preset, got %q", created.PresetKey)
	}
}

func TestEnsureSystemProductPlannerAgentRefreshesLegacyPrompt(t *testing.T) {
	db := newAgentServiceTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	svc := &AgentService{agentRepo: agentRepo}

	now := time.Now().UTC()
	legacyPrompt := "1. `prd_draft`\n2. `awaiting_prd_approval`\n3. `persist_prd`\n4. `story_plan`\n5. `awaiting_story_approval`\n6. `create_stories`"
	if err := db.Exec(`INSERT INTO agents (
		id, workspace_id, is_system, name, preset_key, role, status, runtime_kind,
		skills, trigger_mode, system_prompt, approval_mode, max_concurrent_runs, default_invocation_mode, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"agent-system", "ws-test", true, defaultSystemProductPlannerName, model.AgentPresetEpicPlanner, "Planner", "idle", "native_sdk",
		"[]", "manual", legacyPrompt, "never", 1, model.InvocationModeInteractive, now, now,
	).Error; err != nil {
		t.Fatalf("insert system agent: %v", err)
	}

	updated, err := svc.ensureSystemProductPlannerAgent(context.Background(), "ws-test", "user-1")
	if err != nil {
		t.Fatalf("ensureSystemProductPlannerAgent returned error: %v", err)
	}
	if updated.SystemPrompt == nil || *updated.SystemPrompt == "" {
		t.Fatal("expected refreshed system prompt")
	}
	if strings.Contains(*updated.SystemPrompt, "awaiting_prd_approval") || strings.Contains(*updated.SystemPrompt, "awaiting_story_approval") {
		t.Fatalf("expected refreshed prompt to remove legacy approval phases, got %q", *updated.SystemPrompt)
	}
	if !strings.Contains(*updated.SystemPrompt, "There is no hidden planner phase machine deciding the next step for you.") {
		t.Fatalf("expected refreshed prompt to include inline approval guidance, got %q", *updated.SystemPrompt)
	}
}

func TestCreateAgentRejectsUnknownPreset(t *testing.T) {
	db := newAgentServiceTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	activitySvc := NewPMActivityService(repository.NewPMActivityRepository(db))
	svc := &AgentService{
		agentRepo:   agentRepo,
		activitySvc: activitySvc,
		wsPublisher: nil,
	}

	badPreset := "human"
	if _, err := svc.CreateAgent(context.Background(), modelCreateAgentRequest(&badPreset), "user-1"); err == nil {
		t.Fatal("expected unknown preset to be rejected")
	}
}

func TestDeleteAgentRejectsSystemAgent(t *testing.T) {
	db := newAgentServiceTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	activitySvc := NewPMActivityService(repository.NewPMActivityRepository(db))
	svc := &AgentService{
		agentRepo:   agentRepo,
		activitySvc: activitySvc,
		wsPublisher: nil,
	}

	now := "2026-03-21T00:00:00Z"
	if err := db.Exec(`INSERT INTO agents (
		id, workspace_id, is_system, name, preset_key, role, status, runtime_kind,
		skills, trigger_mode, allowed_tools, allowed_commands, allowed_targets, approval_mode,
		max_concurrent_runs, default_invocation_mode, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"agent-system", "ws-test", true, defaultSystemProductPlannerName, model.AgentPresetEpicPlanner, "Epic Planner", "idle", "native_sdk",
		"[]", "manual", "[]", "[]", "[]", "never", 1, model.InvocationModeInteractive, now, now,
	).Error; err != nil {
		t.Fatalf("insert system agent: %v", err)
	}

	if err := svc.DeleteAgent(context.Background(), "ws-test", "agent-system", "user-1"); err == nil {
		t.Fatal("expected delete to reject system agent")
	}
}

func modelCreateAgentRequest(presetKey *string) model.CreateAgentRequest {
	return model.CreateAgentRequest{
		WorkspaceID: "ws-test",
		Name:        "Example",
		PresetKey:   presetKey,
	}
}

func newAgentServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := "file:" + uuid.NewString() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	registerAgentTestUUIDCallback(t, db)

	statements := []string{
		`CREATE TABLE agents (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			is_system BOOLEAN NOT NULL DEFAULT 0,
			name TEXT NOT NULL,
			preset_key TEXT,
			role TEXT,
			status TEXT NOT NULL,
			runtime_kind TEXT NOT NULL,
			skills TEXT NOT NULL DEFAULT '[]',
			trigger_mode TEXT NOT NULL,
			provider TEXT,
			model TEXT,
			system_prompt TEXT,
			planning_notes TEXT,
			tools TEXT NOT NULL DEFAULT '[]',
			monthly_token_budget INTEGER,
			tokens_used_this_month INTEGER NOT NULL DEFAULT 0,
			active_story_id TEXT,
			team_id TEXT,
			allowed_tools TEXT NOT NULL DEFAULT '[]',
			allowed_commands TEXT NOT NULL DEFAULT '[]',
			allowed_targets TEXT NOT NULL DEFAULT '[]',
			schedule TEXT,
			target_selector TEXT,
			trigger_events TEXT NOT NULL DEFAULT '[]',
			approval_mode TEXT NOT NULL DEFAULT 'never',
			max_concurrent_runs INTEGER NOT NULL DEFAULT 1,
			default_invocation_mode TEXT NOT NULL DEFAULT 'autonomous',
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_activity_log (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			entity_type TEXT NOT NULL,
			entity_id TEXT NOT NULL,
			actor_id TEXT,
			action TEXT NOT NULL,
			field_name TEXT,
			old_value TEXT,
			new_value TEXT,
			metadata TEXT,
			created_at DATETIME
		)`,
	}

	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("exec schema: %v", err)
		}
	}

	return db
}

func registerAgentTestUUIDCallback(t *testing.T, db *gorm.DB) {
	t.Helper()
	err := db.Callback().Create().Before("gorm:before_create").Register("test:assign_uuid", func(tx *gorm.DB) {
		if tx.Statement.Schema == nil {
			return
		}
		field := tx.Statement.Schema.LookUpField("ID")
		if field == nil || field.FieldType.Kind() != reflect.String {
			return
		}

		assignID := func(value reflect.Value) {
			if _, zero := field.ValueOf(tx.Statement.Context, value); !zero {
				return
			}
			_ = field.Set(tx.Statement.Context, value, uuid.NewString())
		}

		switch tx.Statement.ReflectValue.Kind() {
		case reflect.Slice, reflect.Array:
			for i := 0; i < tx.Statement.ReflectValue.Len(); i++ {
				assignID(tx.Statement.ReflectValue.Index(i))
			}
		default:
			assignID(tx.Statement.ReflectValue)
		}
	})
	if err != nil {
		t.Fatalf("register uuid callback: %v", err)
	}
}
