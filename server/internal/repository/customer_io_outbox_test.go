package repository

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestCustomerIOLifecycleOutboxEnqueue(t *testing.T) {
	db := openCustomerIOOutboxTestDB(t)
	repo := NewCustomerIOLifecycleOutboxRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 8, 10, 12, 30, 0, 0, time.UTC)
	first := &model.CustomerIOOutbox{
		ID:                uuid.NewString(),
		SemanticKey:       "workspace.created:workspace-1",
		EventName:         "workspace_created",
		OccurredAt:        now,
		Attributes:        json.RawMessage(`{"workspace_name":"Original"}`),
		RecipientSnapshot: json.RawMessage(`[]`),
		Status:            model.CustomerIOOutboxStatusPending,
		NextAttemptAt:     now,
	}

	persisted, err := repo.Enqueue(ctx, first)
	if err != nil {
		t.Fatalf("enqueue first row: %v", err)
	}
	duplicate := *first
	duplicate.ID = uuid.NewString()
	duplicate.Attributes = json.RawMessage(`{"workspace_name":"Duplicate"}`)
	got, err := repo.Enqueue(ctx, &duplicate)
	if err != nil {
		t.Fatalf("enqueue duplicate row: %v", err)
	}
	if got.ID != persisted.ID {
		t.Errorf("duplicate ID = %q, want original %q", got.ID, persisted.ID)
	}
	if string(got.Attributes) != string(first.Attributes) {
		t.Errorf("duplicate attributes = %s, want original %s", got.Attributes, first.Attributes)
	}

	var count int64
	if err := db.Model(&model.CustomerIOOutbox{}).Count(&count).Error; err != nil {
		t.Fatalf("count outbox rows: %v", err)
	}
	if count != 1 {
		t.Errorf("outbox row count = %d, want 1", count)
	}
}

func TestCustomerIOLifecycleOutboxClaimDueExcludesLiveClaims(t *testing.T) {
	db := openCustomerIOOutboxTestDB(t)
	repo := NewCustomerIOLifecycleOutboxRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 8, 10, 12, 30, 0, 0, time.UTC)
	enqueueCustomerIOOutboxTestRow(t, db, "event:live-claim", now)

	first, err := repo.ClaimDue(ctx, now, 5*time.Minute, 10)
	if err != nil {
		t.Fatalf("claim first batch: %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("first claim count = %d, want 1", len(first))
	}
	second, err := repo.ClaimDue(ctx, now.Add(time.Minute), 5*time.Minute, 10)
	if err != nil {
		t.Fatalf("claim second batch: %v", err)
	}
	if len(second) != 0 {
		t.Errorf("second claim count = %d, want 0", len(second))
	}
}

func TestCustomerIOLifecycleOutboxClaimDueReclaimsExpiredLease(t *testing.T) {
	db := openCustomerIOOutboxTestDB(t)
	repo := NewCustomerIOLifecycleOutboxRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 8, 10, 12, 30, 0, 0, time.UTC)
	enqueueCustomerIOOutboxTestRow(t, db, "event:expired-claim", now)

	first, err := repo.ClaimDue(ctx, now, 5*time.Minute, 1)
	if err != nil {
		t.Fatalf("claim first lease: %v", err)
	}
	second, err := repo.ClaimDue(ctx, now.Add(5*time.Minute), 5*time.Minute, 1)
	if err != nil {
		t.Fatalf("reclaim expired lease: %v", err)
	}
	if len(second) != 1 {
		t.Fatalf("reclaimed count = %d, want 1", len(second))
	}
	if first[0].ClaimToken == nil || second[0].ClaimToken == nil ||
		*first[0].ClaimToken == *second[0].ClaimToken {
		t.Errorf("claim tokens = %v then %v, want distinct tokens", first[0].ClaimToken, second[0].ClaimToken)
	}
}

func TestCustomerIOLifecycleOutboxClaimDueAtomicallyIncrementsAttempts(t *testing.T) {
	db := openCustomerIOOutboxTestDB(t)
	repo := NewCustomerIOLifecycleOutboxRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 8, 10, 12, 30, 0, 0, time.UTC)
	row := enqueueCustomerIOOutboxTestRow(t, db, "event:attempts", now)

	first, err := repo.ClaimDue(ctx, now, time.Minute, 1)
	if err != nil {
		t.Fatalf("claim first attempt: %v", err)
	}
	if first[0].Attempts != 1 {
		t.Errorf("first claim attempts = %d, want 1", first[0].Attempts)
	}
	second, err := repo.ClaimDue(ctx, now.Add(time.Minute), time.Minute, 1)
	if err != nil {
		t.Fatalf("claim second attempt: %v", err)
	}
	if second[0].Attempts != 2 {
		t.Errorf("second claim attempts = %d, want 2", second[0].Attempts)
	}
	var persisted model.CustomerIOOutbox
	if err := db.First(&persisted, "id = ?", row.ID).Error; err != nil {
		t.Fatalf("load claimed row: %v", err)
	}
	if persisted.Attempts != 2 {
		t.Errorf("persisted attempts = %d, want 2", persisted.Attempts)
	}
}

func TestCustomerIOLifecycleOutboxClaimDueConcurrentSQLiteHasSingleOwner(t *testing.T) {
	db := openCustomerIOOutboxTestDB(t)
	now := time.Date(2026, 8, 10, 12, 30, 0, 0, time.UTC)
	enqueueCustomerIOOutboxTestRow(t, db, "event:concurrent-claim", now)
	repositories := []*CustomerIOLifecycleOutboxRepository{
		NewCustomerIOLifecycleOutboxRepository(db),
		NewCustomerIOLifecycleOutboxRepository(db),
	}

	type claimResult struct {
		rows []model.CustomerIOOutbox
		err  error
	}
	start := make(chan struct{})
	results := make(chan claimResult, len(repositories))
	var wg sync.WaitGroup
	for _, repo := range repositories {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			rows, err := repo.ClaimDue(context.Background(), now, 5*time.Minute, 1)
			results <- claimResult{rows: rows, err: err}
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	var claims []model.CustomerIOOutbox
	for result := range results {
		if result.err != nil {
			t.Fatalf("concurrent claim: %v", result.err)
		}
		claims = append(claims, result.rows...)
	}
	if len(claims) != 1 {
		t.Fatalf("live claim owners = %d, want 1", len(claims))
	}
	if claims[0].ClaimToken == nil {
		t.Fatal("live claim has no token")
	}
}

func TestCustomerIOLifecycleOutboxClaimDuePostgresUsesSkipLocked(t *testing.T) {
	expression := customerIOClaimLockingClause("postgres")
	locking, ok := expression.(clause.Locking)
	if !ok {
		t.Fatalf("PostgreSQL claim clause type = %T, want clause.Locking", expression)
	}
	if locking.Strength != "UPDATE" || locking.Options != "SKIP LOCKED" {
		t.Errorf("PostgreSQL locking clause = %#v, want FOR UPDATE SKIP LOCKED", locking)
	}
	if got := customerIOClaimLockingClause("sqlite"); got != nil {
		t.Errorf("SQLite locking clause = %#v, want nil", got)
	}
}

func TestCustomerIOLifecycleOutboxMarkDeliveredFencesClaimTokens(t *testing.T) {
	db := openCustomerIOOutboxTestDB(t)
	repo := NewCustomerIOLifecycleOutboxRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 8, 10, 12, 30, 0, 0, time.UTC)
	row := enqueueCustomerIOOutboxTestRow(t, db, "event:delivered", now)
	claimed, err := repo.ClaimDue(ctx, now, 5*time.Minute, 1)
	if err != nil {
		t.Fatalf("claim row: %v", err)
	}
	token := *claimed[0].ClaimToken

	updated, err := repo.MarkDelivered(ctx, row.ID, "stale-token")
	if err != nil {
		t.Fatalf("mark delivered with stale token: %v", err)
	}
	if updated {
		t.Error("stale token unexpectedly updated row")
	}
	assertCustomerIOOutboxStatus(t, db, row.ID, model.CustomerIOOutboxStatusProcessing)

	updated, err = repo.MarkDelivered(ctx, row.ID, token)
	if err != nil {
		t.Fatalf("mark delivered with current token: %v", err)
	}
	if !updated {
		t.Error("current token did not update row")
	}
	var delivered model.CustomerIOOutbox
	if err := db.First(&delivered, "id = ?", row.ID).Error; err != nil {
		t.Fatalf("load delivered row: %v", err)
	}
	if delivered.Status != model.CustomerIOOutboxStatusDelivered {
		t.Errorf("status = %q, want %q", delivered.Status, model.CustomerIOOutboxStatusDelivered)
	}
	if delivered.ClaimToken != nil || delivered.ClaimedAt != nil || delivered.LeaseExpiresAt != nil {
		t.Errorf("delivered claim fields = (%v, %v, %v), want nil", delivered.ClaimToken, delivered.ClaimedAt, delivered.LeaseExpiresAt)
	}
}

func TestCustomerIOLifecycleOutboxScheduleRetryFencesClaimTokens(t *testing.T) {
	db := openCustomerIOOutboxTestDB(t)
	repo := NewCustomerIOLifecycleOutboxRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 8, 10, 12, 30, 0, 0, time.UTC)
	nextAttemptAt := now.Add(10 * time.Minute)
	row := enqueueCustomerIOOutboxTestRow(t, db, "event:retry", now)
	claimed, err := repo.ClaimDue(ctx, now, 5*time.Minute, 1)
	if err != nil {
		t.Fatalf("claim row: %v", err)
	}
	token := *claimed[0].ClaimToken

	updated, err := repo.ScheduleRetry(ctx, row.ID, "stale-token", nextAttemptAt, "stale")
	if err != nil {
		t.Fatalf("schedule retry with stale token: %v", err)
	}
	if updated {
		t.Error("stale token unexpectedly scheduled retry")
	}
	assertCustomerIOOutboxStatus(t, db, row.ID, model.CustomerIOOutboxStatusProcessing)

	updated, err = repo.ScheduleRetry(ctx, row.ID, token, nextAttemptAt, "temporary outage")
	if err != nil {
		t.Fatalf("schedule retry with current token: %v", err)
	}
	if !updated {
		t.Error("current token did not schedule retry")
	}
	var retried model.CustomerIOOutbox
	if err := db.First(&retried, "id = ?", row.ID).Error; err != nil {
		t.Fatalf("load retried row: %v", err)
	}
	if retried.Status != model.CustomerIOOutboxStatusPending || retried.Attempts != 1 {
		t.Errorf("retry state = (%q, attempts %d), want (pending, attempts 1)", retried.Status, retried.Attempts)
	}
	if !retried.NextAttemptAt.Equal(nextAttemptAt) {
		t.Errorf("next attempt = %v, want %v", retried.NextAttemptAt, nextAttemptAt)
	}
	if retried.LastError == nil || *retried.LastError != "temporary outage" {
		t.Errorf("last error = %v, want temporary outage", retried.LastError)
	}
	if retried.ClaimToken != nil || retried.ClaimedAt != nil || retried.LeaseExpiresAt != nil {
		t.Errorf("retry claim fields = (%v, %v, %v), want nil", retried.ClaimToken, retried.ClaimedAt, retried.LeaseExpiresAt)
	}
}

func TestCustomerIOLifecycleOutboxMarkFailedFencesClaimTokens(t *testing.T) {
	db := openCustomerIOOutboxTestDB(t)
	repo := NewCustomerIOLifecycleOutboxRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 8, 10, 12, 30, 0, 0, time.UTC)
	row := enqueueCustomerIOOutboxTestRow(t, db, "event:failed", now)
	claimed, err := repo.ClaimDue(ctx, now, 5*time.Minute, 1)
	if err != nil {
		t.Fatalf("claim row: %v", err)
	}
	token := *claimed[0].ClaimToken

	updated, err := repo.MarkFailed(ctx, row.ID, "stale-token", "stale")
	if err != nil {
		t.Fatalf("mark failed with stale token: %v", err)
	}
	if updated {
		t.Error("stale token unexpectedly marked row failed")
	}
	assertCustomerIOOutboxStatus(t, db, row.ID, model.CustomerIOOutboxStatusProcessing)

	updated, err = repo.MarkFailed(ctx, row.ID, token, "bad request")
	if err != nil {
		t.Fatalf("mark failed with current token: %v", err)
	}
	if !updated {
		t.Error("current token did not mark row failed")
	}
	var failed model.CustomerIOOutbox
	if err := db.First(&failed, "id = ?", row.ID).Error; err != nil {
		t.Fatalf("load failed row: %v", err)
	}
	if failed.Status != model.CustomerIOOutboxStatusFailed || failed.Attempts != 1 {
		t.Errorf("failed state = (%q, attempts %d), want (failed, attempts 1)", failed.Status, failed.Attempts)
	}
	if failed.LastError == nil || *failed.LastError != "bad request" {
		t.Errorf("last error = %v, want bad request", failed.LastError)
	}
	if failed.ClaimToken != nil || failed.ClaimedAt != nil || failed.LeaseExpiresAt != nil {
		t.Errorf("failed claim fields = (%v, %v, %v), want nil", failed.ClaimToken, failed.ClaimedAt, failed.LeaseExpiresAt)
	}
}

func TestCustomerIOLifecycleOutboxErrorTruncation(t *testing.T) {
	db := openCustomerIOOutboxTestDB(t)
	repo := NewCustomerIOLifecycleOutboxRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 8, 10, 12, 30, 0, 0, time.UTC)
	first := enqueueCustomerIOOutboxTestRow(t, db, "event:retry-truncation", now)
	second := enqueueCustomerIOOutboxTestRow(t, db, "event:failed-truncation", now)
	claimed, err := repo.ClaimDue(ctx, now, 5*time.Minute, 2)
	if err != nil {
		t.Fatalf("claim rows: %v", err)
	}
	tokens := make(map[string]string, len(claimed))
	for _, row := range claimed {
		tokens[row.ID] = *row.ClaimToken
	}
	longError := strings.Repeat("x", 3*1024)

	if _, err := repo.ScheduleRetry(ctx, first.ID, tokens[first.ID], now.Add(time.Minute), longError); err != nil {
		t.Fatalf("schedule retry: %v", err)
	}
	if _, err := repo.MarkFailed(ctx, second.ID, tokens[second.ID], longError); err != nil {
		t.Fatalf("mark failed: %v", err)
	}
	for _, id := range []string{first.ID, second.ID} {
		var row model.CustomerIOOutbox
		if err := db.First(&row, "id = ?", id).Error; err != nil {
			t.Fatalf("load outbox row: %v", err)
		}
		if row.LastError == nil || len(*row.LastError) != 2*1024 {
			t.Errorf("last error length = %d, want %d", len(customerIOTestStringValue(row.LastError)), 2*1024)
		}
	}
}

func TestCustomerIOLifecycleOutboxErrorTruncationPreservesUTF8(t *testing.T) {
	lastError := strings.Repeat("x", 2047) + "€"

	got := truncateCustomerIOLastError(lastError)

	if len(got) > 2*1024 {
		t.Errorf("last error length = %d, want at most %d", len(got), 2*1024)
	}
	if !utf8.ValidString(got) {
		t.Error("truncated last error is not valid UTF-8")
	}
}

func TestCustomerIOLifecycleOutboxSQLiteRoundTrip(t *testing.T) {
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

func TestCustomerIOLifecycleOutboxSemanticKeyIsUnique(t *testing.T) {
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
	dsn := "file:" + t.Name() + "-" + uuid.NewString() + "?mode=memory&cache=shared&_busy_timeout=5000"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&customerIOOutboxSQLiteSchema{}); err != nil {
		t.Fatalf("migrate outbox table: %v", err)
	}
	return db
}

func enqueueCustomerIOOutboxTestRow(
	t *testing.T,
	db *gorm.DB,
	semanticKey string,
	nextAttemptAt time.Time,
) model.CustomerIOOutbox {
	t.Helper()
	row := model.CustomerIOOutbox{
		ID:                uuid.NewString(),
		SemanticKey:       semanticKey,
		EventName:         "test_event",
		OccurredAt:        nextAttemptAt,
		Attributes:        json.RawMessage(`{}`),
		RecipientSnapshot: json.RawMessage(`[]`),
		Status:            model.CustomerIOOutboxStatusPending,
		NextAttemptAt:     nextAttemptAt,
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("create outbox row: %v", err)
	}
	return row
}

func assertCustomerIOOutboxStatus(
	t *testing.T,
	db *gorm.DB,
	id string,
	want model.CustomerIOOutboxStatus,
) {
	t.Helper()
	var row model.CustomerIOOutbox
	if err := db.First(&row, "id = ?", id).Error; err != nil {
		t.Fatalf("load outbox row: %v", err)
	}
	if row.Status != want {
		t.Errorf("status = %q, want %q", row.Status, want)
	}
}

func customerIOTestStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
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

func TestProductAnalyticsOutboxEnqueueDeduplicatesSemanticKey(t *testing.T) {
	db := openCustomerIOOutboxTestDB(t)
	if err := db.Exec("CREATE TABLE product_analytics_outbox (id TEXT PRIMARY KEY, semantic_key TEXT NOT NULL UNIQUE, user_id TEXT, anonymous_id TEXT, workspace_id TEXT, event_name TEXT NOT NULL, source TEXT NOT NULL, occurred_at DATETIME NOT NULL, attributes TEXT NOT NULL, status TEXT NOT NULL, attempts INTEGER NOT NULL, next_attempt_at DATETIME NOT NULL, claim_token TEXT, claimed_at DATETIME, lease_expires_at DATETIME, last_error TEXT, created_at DATETIME, updated_at DATETIME)").Error; err != nil {
		t.Fatalf("create product analytics outbox: %v", err)
	}
	repo := NewProductAnalyticsOutboxRepository(db)
	input := ProductAnalyticsEventInput{
		SemanticKey: "task:task-1:created",
		UserID:      "user-1", AnonymousID: "anon-1", WorkspaceID: "workspace-1",
		EventName: "task_created", Source: "api",
		OccurredAt: time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC),
		Attributes: map[string]any{"task_id": "task-1"},
	}
	first, err := repo.Enqueue(context.Background(), input)
	if err != nil {
		t.Fatalf("enqueue first event: %v", err)
	}
	input.Attributes = map[string]any{"task_id": "duplicate"}
	second, err := repo.Enqueue(context.Background(), input)
	if err != nil {
		t.Fatalf("enqueue duplicate event: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("duplicate ID = %q, want %q", second.ID, first.ID)
	}
	var count int64
	if err := db.Model(&model.ProductAnalyticsOutbox{}).Count(&count).Error; err != nil {
		t.Fatalf("count product analytics events: %v", err)
	}
	if count != 1 {
		t.Fatalf("event count = %d, want 1", count)
	}
	if second.AnonymousID == nil || *second.AnonymousID != "anon-1" {
		t.Fatalf("anonymous ID = %#v", second.AnonymousID)
	}
}
