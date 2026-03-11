package service

import (
	"context"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestCreateAgentDerivesDefaultsFromAgentClass(t *testing.T) {
	db := newAgentServiceTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	activitySvc := NewPMActivityService(repository.NewPMActivityRepository(db))
	svc := &AgentService{
		agentRepo:   agentRepo,
		activitySvc: activitySvc,
		wsPublisher: nil,
	}

	created, err := svc.CreateAgent(context.Background(), modelCreateAgentRequest("engineer"), "user-1")
	if err != nil {
		t.Fatalf("CreateAgent returned error: %v", err)
	}

	if created.AgentKind != "llm" {
		t.Fatalf("expected llm agent kind, got %q", created.AgentKind)
	}
	if created.Role != "Engineer" {
		t.Fatalf("expected default role Engineer, got %q", created.Role)
	}
	if created.RuntimeKind != "opencode" {
		t.Fatalf("expected default runtime opencode, got %q", created.RuntimeKind)
	}
	if created.CapabilityProfile != "engineer" {
		t.Fatalf("expected engineer capability profile, got %q", created.CapabilityProfile)
	}
}

func TestCreateHumanAgentDerivesHumanDefaults(t *testing.T) {
	db := newAgentServiceTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	activitySvc := NewPMActivityService(repository.NewPMActivityRepository(db))
	svc := &AgentService{
		agentRepo:   agentRepo,
		activitySvc: activitySvc,
		wsPublisher: nil,
	}

	created, err := svc.CreateAgent(context.Background(), modelCreateAgentRequest("human"), "user-1")
	if err != nil {
		t.Fatalf("CreateAgent returned error: %v", err)
	}

	if created.AgentKind != "human" {
		t.Fatalf("expected human agent kind, got %q", created.AgentKind)
	}
	if created.Role != "Human" {
		t.Fatalf("expected default role Human, got %q", created.Role)
	}
	if created.Model != nil || created.SystemPrompt != nil || created.PlanningNotes != nil {
		t.Fatalf("expected human agent to have no llm-specific fields, got %+v", created)
	}
}

func modelCreateAgentRequest(agentClass string) model.CreateAgentRequest {
	return model.CreateAgentRequest{
		WorkspaceID: "ws-test",
		Name:        "Example",
		AgentClass:  &agentClass,
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
			name TEXT NOT NULL,
			agent_kind TEXT NOT NULL,
			agent_class TEXT NOT NULL,
			role TEXT,
			status TEXT NOT NULL,
			user_id TEXT,
			runtime_kind TEXT NOT NULL,
			capability_profile TEXT NOT NULL,
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
