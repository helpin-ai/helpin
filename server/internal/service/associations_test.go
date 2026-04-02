package service

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestCreateTaskRelationshipMapsReciprocalActions(t *testing.T) {
	db := newAssociationsTestDB(t)
	svc := newAssociationsServiceForTest(db)

	seedAssociationTask(t, db, "task-a", "ws-1", 101, "Task A", false)
	seedAssociationTask(t, db, "task-b", "ws-1", 102, "Task B", false)

	link, err := svc.CreateTaskRelationship(context.Background(), "ws-1", "task-a", "user-1", model.CreateTaskRelationshipRequest{
		RelationshipType: model.TaskRelationshipActionIsBlockedBy,
		OtherTaskID:     "task-b",
	})
	if err != nil {
		t.Fatalf("CreateTaskRelationship returned error: %v", err)
	}

	if link.LinkType != model.PMTaskLinkTypeBlocks {
		t.Fatalf("expected blocks link type, got %q", link.LinkType)
	}
	if link.SourceTaskID != "task-b" || link.TargetTaskID != "task-a" {
		t.Fatalf("expected task-b to block task-a, got %s -> %s", link.SourceTaskID, link.TargetTaskID)
	}
}

func TestCreateTaskRelationshipRejectsCycles(t *testing.T) {
	db := newAssociationsTestDB(t)
	svc := newAssociationsServiceForTest(db)

	seedAssociationTask(t, db, "task-a", "ws-1", 101, "Task A", false)
	seedAssociationTask(t, db, "task-b", "ws-1", 102, "Task B", false)
	seedTaskLink(t, db, "link-1", "ws-1", "task-b", "task-a", model.PMTaskLinkTypeBlocks)

	_, err := svc.CreateTaskRelationship(context.Background(), "ws-1", "task-a", "user-1", model.CreateTaskRelationshipRequest{
		RelationshipType: model.TaskRelationshipActionBlocks,
		OtherTaskID:     "task-b",
	})
	if err == nil {
		t.Fatalf("expected cycle error, got nil")
	}
}

func TestListGroupedTaskAssociationsIncludesRelationshipsLegacySupportAndDocs(t *testing.T) {
	db := newAssociationsTestDB(t)
	svc := newAssociationsServiceForTest(db)

	seedAssociationTask(t, db, "task-a", "ws-1", 101, "Task A", false)
	seedAssociationTask(t, db, "task-b", "ws-1", 102, "Task B", true)
	seedTaskLink(t, db, "link-1", "ws-1", "task-b", "task-a", model.PMTaskLinkTypeBlocks)
	seedSupportConversation(t, db, "ticket-1", "ws-1", 11, "Customer asks for Task A", "task-a")
	seedDocsDocument(t, db, "doc-1", "ws-1", "Spec Doc")
	seedDocsLink(t, db, "doc-link-1", "ws-1", "doc-1", model.LinkedObjectTask, "task-a")

	grouped, err := svc.ListGrouped(context.Background(), "ws-1", model.CRMObjectTask, "task-a")
	if err != nil {
		t.Fatalf("ListGrouped returned error: %v", err)
	}

	if len(grouped.TaskRelationships.BlockedBy) != 1 {
		t.Fatalf("expected one blocked_by relationship, got %d", len(grouped.TaskRelationships.BlockedBy))
	}
	if grouped.TaskRelationships.BlockedBy[0].IsActive {
		t.Fatalf("expected completed blocker to be inactive in grouped response")
	}
	if len(grouped.SupportConversations) != 1 {
		t.Fatalf("expected one support conversation association, got %d", len(grouped.SupportConversations))
	}
	if len(grouped.Docs) != 1 {
		t.Fatalf("expected one docs association, got %d", len(grouped.Docs))
	}
}

func newAssociationsServiceForTest(db *gorm.DB) *AssociationsService {
	return NewAssociationsService(
		repository.NewCRMAssociationRepository(db),
		repository.NewPMTaskLinkRepository(db),
		repository.NewPMTaskRepository(db),
		repository.NewSupportConversationRepository(db),
		repository.NewDocsLinkRepository(db),
		repository.NewDocsDocumentRepository(db),
	)
}

func newAssociationsTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := "file:" + uuid.NewString() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	registerAssociationsTestUUIDCallback(t, db)

	statements := []string{
		`CREATE TABLE pm_tasks (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			display_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			workflow_state_id TEXT NOT NULL,
			completed BOOLEAN NOT NULL DEFAULT false,
			archived BOOLEAN NOT NULL DEFAULT false,
			blocked BOOLEAN NOT NULL DEFAULT false,
			plan_document_id TEXT,
			blocker TEXT,
			recurring_template_id TEXT,
			recurring_run_id TEXT,
			recurring_occurrence_number INTEGER,
			slice_type TEXT,
			implementation_brief TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_task_links (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			source_task_id TEXT NOT NULL,
			target_task_id TEXT NOT NULL,
			link_type TEXT NOT NULL,
			created_by TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE crm_associations (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			from_object_type TEXT NOT NULL,
			from_object_id TEXT NOT NULL,
			to_object_type TEXT NOT NULL,
			to_object_id TEXT NOT NULL,
			association_label TEXT,
			created_at DATETIME
		)`,
		`CREATE TABLE support_conversations (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			display_id INTEGER NOT NULL,
			subject TEXT NOT NULL,
			status TEXT NOT NULL,
			priority TEXT NOT NULL,
			linked_task_id TEXT,
			source TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE docs_documents (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			title TEXT NOT NULL,
			status TEXT NOT NULL,
			deleted_at DATETIME
		)`,
		`CREATE TABLE docs_links (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			document_id TEXT NOT NULL,
			linked_object_type TEXT NOT NULL,
			linked_object_id TEXT NOT NULL,
			link_context TEXT NOT NULL,
			created_by TEXT NOT NULL,
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

func registerAssociationsTestUUIDCallback(t *testing.T, db *gorm.DB) {
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

func seedAssociationTask(t *testing.T, db *gorm.DB, id, workspaceID string, displayID int, name string, completed bool) {
	t.Helper()
	if err := db.Exec(
		`INSERT INTO pm_tasks (id, workspace_id, display_id, name, workflow_state_id, completed, archived, blocked, created_at, updated_at)
		 VALUES (?, ?, ?, ?, 'state-1', ?, false, false, ?, ?)`,
		id, workspaceID, displayID, name, completed, time.Now().UTC(), time.Now().UTC(),
	).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}
}

func seedTaskLink(t *testing.T, db *gorm.DB, id, workspaceID, sourceTaskID, targetTaskID, linkType string) {
	t.Helper()
	if err := db.Exec(
		`INSERT INTO pm_task_links (id, workspace_id, source_task_id, target_task_id, link_type, created_by, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, 'user-1', ?, ?)`,
		id, workspaceID, sourceTaskID, targetTaskID, linkType, time.Now().UTC(), time.Now().UTC(),
	).Error; err != nil {
		t.Fatalf("seed task link: %v", err)
	}
}

func seedSupportConversation(t *testing.T, db *gorm.DB, id, workspaceID string, displayID int, subject, linkedTaskID string) {
	t.Helper()
	if err := db.Exec(
		`INSERT INTO support_conversations (id, workspace_id, display_id, subject, status, priority, linked_task_id, source, created_at, updated_at)
		 VALUES (?, ?, ?, ?, 'open', 'medium', ?, 'internal', ?, ?)`,
		id, workspaceID, displayID, subject, linkedTaskID, time.Now().UTC(), time.Now().UTC(),
	).Error; err != nil {
		t.Fatalf("seed support conversation: %v", err)
	}
}

func seedDocsDocument(t *testing.T, db *gorm.DB, id, workspaceID, title string) {
	t.Helper()
	if err := db.Exec(
		`INSERT INTO docs_documents (id, workspace_id, title, status, deleted_at)
		 VALUES (?, ?, ?, 'published', NULL)`,
		id, workspaceID, title,
	).Error; err != nil {
		t.Fatalf("seed docs document: %v", err)
	}
}

func seedDocsLink(t *testing.T, db *gorm.DB, id, workspaceID, documentID, linkedObjectType, linkedObjectID string) {
	t.Helper()
	if err := db.Exec(
		`INSERT INTO docs_links (id, workspace_id, document_id, linked_object_type, linked_object_id, link_context, created_by, created_at)
		 VALUES (?, ?, ?, ?, ?, 'attached', 'user-1', ?)`,
		id, workspaceID, documentID, linkedObjectType, linkedObjectID, time.Now().UTC(),
	).Error; err != nil {
		t.Fatalf("seed docs link: %v", err)
	}
}
