package repository

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestCustomerIOOutboxSQLiteRoundTrip(t *testing.T) {
	db := openCustomerIOOutboxTestDB(t)
	now := time.Date(2026, 8, 10, 12, 30, 0, 0, time.UTC)
	workspaceID := uuid.NewString()
	original := model.CustomerIOOutbox{
		ID:                uuid.NewString(),
		SemanticKey:       "workspace.created:" + workspaceID,
		WorkspaceID:       &workspaceID,
		EventName:         "workspace_created",
		OccurredAt:        now,
		Attributes:        json.RawMessage(`{"workspace_name":"Acme"}`),
		RecipientSnapshot: json.RawMessage(`{"user_id":"user-1","role":"owner"}`),
		Status:            model.CustomerIOOutboxStatusPending,
		Attempts:          0,
		NextAttemptAt:     now.Add(time.Minute),
	}

	if err := db.Create(&original).Error; err != nil {
		t.Fatalf("create outbox row: %v", err)
	}

	var restored model.CustomerIOOutbox
	if err := db.First(&restored, "id = ?", original.ID).Error; err != nil {
		t.Fatalf("load outbox row: %v", err)
	}
	if string(restored.Attributes) != string(original.Attributes) {
		t.Errorf("attributes = %s, want %s", restored.Attributes, original.Attributes)
	}
	if string(restored.RecipientSnapshot) != string(original.RecipientSnapshot) {
		t.Errorf("recipient snapshot = %s, want %s", restored.RecipientSnapshot, original.RecipientSnapshot)
	}
	if restored.Status != model.CustomerIOOutboxStatusPending {
		t.Errorf("status = %q, want %q", restored.Status, model.CustomerIOOutboxStatusPending)
	}
	if restored.WorkspaceID == nil || *restored.WorkspaceID != workspaceID {
		t.Errorf("workspace ID = %v, want %q", restored.WorkspaceID, workspaceID)
	}
}

func TestCustomerIOOutboxSemanticKeyIsUnique(t *testing.T) {
	db := openCustomerIOOutboxTestDB(t)
	row := model.CustomerIOOutbox{
		ID:                uuid.NewString(),
		SemanticKey:       "membership.created:membership-1",
		EventName:         "membership_created",
		OccurredAt:        time.Now().UTC(),
		Attributes:        json.RawMessage(`{}`),
		RecipientSnapshot: json.RawMessage(`{"user_id":"user-1","role":"member"}`),
		Status:            model.CustomerIOOutboxStatusPending,
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("create first outbox row: %v", err)
	}

	row.ID = uuid.NewString()
	if err := db.Create(&row).Error; err == nil {
		t.Fatal("expected duplicate semantic key to be rejected")
	}
}

func openCustomerIOOutboxTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&customerIOOutboxSQLiteSchema{}); err != nil {
		t.Fatalf("migrate outbox table: %v", err)
	}
	return db
}

// customerIOOutboxSQLiteSchema omits PostgreSQL-only types and UUID defaults while
// retaining the model's columns and uniqueness contract for SQLite tests.
type customerIOOutboxSQLiteSchema struct {
	ID                string                       `gorm:"primaryKey"`
	SemanticKey       string                       `gorm:"not null;uniqueIndex"`
	WorkspaceID       *string                      `gorm:"index"`
	EventName         string                       `gorm:"not null"`
	OccurredAt        time.Time                    `gorm:"not null"`
	Attributes        json.RawMessage              `gorm:"not null;default:'{}'"`
	RecipientSnapshot json.RawMessage              `gorm:"not null;default:'{}'"`
	Status            model.CustomerIOOutboxStatus `gorm:"not null;default:pending;index"`
	Attempts          int                          `gorm:"not null;default:0"`
	NextAttemptAt     time.Time                    `gorm:"not null;index"`
	ClaimToken        *string
	ClaimedAt         *time.Time
	LeaseExpiresAt    *time.Time `gorm:"index"`
	LastError         *string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (customerIOOutboxSQLiteSchema) TableName() string { return "customer_io_outbox" }
