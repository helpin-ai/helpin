package temporalapp

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/crmemail"
	"github.com/helpin-ai/helpin/server/internal/crmsignal"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/sync"
)

type fakeHistorySyncClient struct {
	listMessagesFn      func(ctx context.Context, accessToken, query string, maxResults int, pageToken string) ([]sync.GmailMessage, string, error)
	getMessageDetailFn  func(ctx context.Context, accessToken, messageID string) (*sync.GmailMessage, error)
	getMailboxProfileFn func(ctx context.Context, accessToken string) (*sync.GmailProfile, error)
	listHistoryFn       func(ctx context.Context, accessToken, startHistoryID string) (*sync.GmailHistoryResult, error)
}

type fakeSignalStarter struct {
	messageIDs []string
	payloads   [][]model.SignalSourcePayload
}

func (f *fakeSignalStarter) StartEmailSignalDetection(ctx context.Context, messageID string, payloads []model.SignalSourcePayload) error {
	f.messageIDs = append(f.messageIDs, messageID)
	f.payloads = append(f.payloads, payloads)
	return nil
}

func (f *fakeHistorySyncClient) GetValidToken(ctx context.Context, account *model.CRMEmailAccount) (string, error) {
	return "token", nil
}

func (f *fakeHistorySyncClient) ListMessages(ctx context.Context, accessToken, query string, maxResults int, pageToken string) ([]sync.GmailMessage, string, error) {
	if f.listMessagesFn == nil {
		return nil, "", nil
	}
	return f.listMessagesFn(ctx, accessToken, query, maxResults, pageToken)
}

func (f *fakeHistorySyncClient) GetMessageDetail(ctx context.Context, accessToken, messageID string) (*sync.GmailMessage, error) {
	if f.getMessageDetailFn == nil {
		return nil, fmt.Errorf("unexpected GetMessageDetail call")
	}
	return f.getMessageDetailFn(ctx, accessToken, messageID)
}

func (f *fakeHistorySyncClient) GetMailboxProfile(ctx context.Context, accessToken string) (*sync.GmailProfile, error) {
	if f.getMailboxProfileFn == nil {
		return &sync.GmailProfile{EmailAddress: "owner@example.com", HistoryID: "hist-default"}, nil
	}
	return f.getMailboxProfileFn(ctx, accessToken)
}

func (f *fakeHistorySyncClient) ListHistory(ctx context.Context, accessToken, startHistoryID string) (*sync.GmailHistoryResult, error) {
	if f.listHistoryFn == nil {
		return &sync.GmailHistoryResult{}, nil
	}
	return f.listHistoryFn(ctx, accessToken, startHistoryID)
}

func setupEmailSyncActivitiesTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:email_sync_activities_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	statements := []string{
		`CREATE TABLE crm_contacts (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL,
			display_id TEXT NOT NULL,
			first_name TEXT NOT NULL,
			last_name TEXT,
			email TEXT,
			phone TEXT,
			job_title TEXT,
			lifecycle_stage TEXT NOT NULL DEFAULT 'subscriber',
			lead_status TEXT NOT NULL DEFAULT 'new',
			owner_member_id TEXT,
			avatar_url TEXT,
			source TEXT,
			custom_properties BLOB NOT NULL DEFAULT (CAST('{}' AS BLOB)),
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE UNIQUE INDEX idx_crm_contacts_workspace_email_lower_unique
			ON crm_contacts(workspace_id, lower(email))
			WHERE email IS NOT NULL`,
		`CREATE TABLE crm_email_accounts (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			member_id TEXT NOT NULL,
			provider TEXT NOT NULL DEFAULT 'gmail',
			email_address TEXT NOT NULL,
			normalized_email_address TEXT,
			access_token_encrypted TEXT,
			refresh_token_encrypted TEXT,
			sync_state BLOB NOT NULL DEFAULT (CAST('{}' AS BLOB)),
			last_history_id TEXT,
			last_synced_at DATETIME,
			is_active BOOLEAN NOT NULL DEFAULT 1,
			status TEXT NOT NULL DEFAULT 'connected',
			disconnected_at DATETIME,
			oauth_state TEXT,
			token_expires_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE crm_email_threads (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL,
			email_account_id TEXT NOT NULL,
			thread_external_id TEXT NOT NULL,
			subject TEXT NOT NULL,
			last_message_at DATETIME NOT NULL,
			message_count INTEGER NOT NULL DEFAULT 0,
			contact_ids BLOB NOT NULL DEFAULT (CAST('[]' AS BLOB)),
			deal_id TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE crm_email_messages (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL,
			email_account_id TEXT NOT NULL,
			thread_id TEXT,
			message_external_id TEXT,
			from_address TEXT NOT NULL,
			from_name TEXT,
			to_addresses BLOB NOT NULL DEFAULT (CAST('[]' AS BLOB)),
			cc_addresses BLOB NOT NULL DEFAULT (CAST('[]' AS BLOB)),
			subject TEXT,
			body_text TEXT,
			body_html TEXT,
			direction TEXT NOT NULL DEFAULT 'inbound',
			sent_at DATETIME NOT NULL,
			contact_id TEXT,
			deal_id TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE crm_email_message_contacts (
			message_id TEXT NOT NULL,
			contact_id TEXT NOT NULL,
			participant_role TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (message_id, contact_id, participant_role)
		)`,
	}
	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("apply schema statement %q: %v", stmt, err)
		}
	}

	return db
}

func TestEmailSyncActivities_StoreMessageUsesSentLabelAndAssociatesAllParticipants(t *testing.T) {
	db := setupEmailSyncActivitiesTestDB(t)
	emailRepo := repository.NewCRMEmailRepository(db)
	contactRepo := repository.NewCRMContactRepository(db)
	activities := NewEmailSyncActivities(nil, emailRepo, contactRepo, nil, nil, nil, nil)
	ctx := context.Background()

	account := &model.CRMEmailAccount{
		ID:           "acct-1",
		WorkspaceID:  "ws-1",
		MemberID:     "member-1",
		Provider:     "gmail",
		EmailAddress: "owner@example.com",
		IsActive:     true,
	}
	if err := emailRepo.CreateAccount(ctx, account); err != nil {
		t.Fatalf("seed account: %v", err)
	}

	settings := model.DefaultEmailSyncSettings()
	settings.RecordCreationMode = "always"

	msg := &sync.GmailMessage{
		ID:       "gmail-msg-1",
		ThreadID: "gmail-thread-1",
		Subject:  "Hello",
		From:     "owner@example.com",
		To:       []string{"buyer@example.com"},
		CC:       []string{"other@example.com"},
		Date:     time.Now(),
		LabelIDs: []string{"SENT"},
	}

	result, err := activities.storeMessage(ctx, account, msg, &settings)
	if err != nil {
		t.Fatalf("storeMessage: %v", err)
	}
	if !result.Stored || result.AssociationsWritten != 2 {
		t.Fatalf("store result = %+v, want stored with two associations", result)
	}

	var stored model.CRMEmailMessage
	if err := db.Table("crm_email_messages").Where("message_external_id = ?", "gmail-msg-1").First(&stored).Error; err != nil {
		t.Fatalf("load stored message: %v", err)
	}
	if stored.Direction != model.CRMEmailDirectionOutbound {
		t.Fatalf("direction = %q, want outbound", stored.Direction)
	}
	if stored.ContactID != nil {
		t.Fatalf("contact_id = %v, want nil for multi-contact message", *stored.ContactID)
	}

	var assocCount int64
	if err := db.Table("crm_email_message_contacts").Where("message_id = ?", stored.ID).Count(&assocCount).Error; err != nil {
		t.Fatalf("count associations: %v", err)
	}
	if assocCount != 2 {
		t.Fatalf("association count = %d, want 2", assocCount)
	}

	var thread model.CRMEmailThread
	if err := db.Table("crm_email_threads").Where("thread_external_id = ?", "gmail-thread-1").First(&thread).Error; err != nil {
		t.Fatalf("load thread: %v", err)
	}
	var threadContactIDs []string
	if err := json.Unmarshal(thread.ContactIDs, &threadContactIDs); err != nil {
		t.Fatalf("unmarshal thread contact_ids: %v", err)
	}
	if len(threadContactIDs) != 2 {
		t.Fatalf("thread contact_ids = %v, want 2", threadContactIDs)
	}
}

func TestEmailSyncActivities_InternalExclusionHonorsCC(t *testing.T) {
	db := setupEmailSyncActivitiesTestDB(t)
	emailRepo := repository.NewCRMEmailRepository(db)
	contactRepo := repository.NewCRMContactRepository(db)
	activities := NewEmailSyncActivities(nil, emailRepo, contactRepo, nil, nil, nil, nil)
	ctx := context.Background()

	account := &model.CRMEmailAccount{
		ID:           "acct-1",
		WorkspaceID:  "ws-1",
		MemberID:     "member-1",
		Provider:     "gmail",
		EmailAddress: "owner@example.com",
		IsActive:     true,
	}
	if err := emailRepo.CreateAccount(ctx, account); err != nil {
		t.Fatalf("seed account: %v", err)
	}

	settings := model.DefaultEmailSyncSettings()
	settings.InternalExclusion = "exclude"
	settings.RecordCreationMode = "always"

	msg := &sync.GmailMessage{
		ID:       "gmail-msg-2",
		ThreadID: "gmail-thread-2",
		Subject:  "Internal-ish",
		From:     "owner@example.com",
		To:       []string{"coworker@example.com"},
		CC:       []string{"partner@external.com"},
		Date:     time.Now(),
		LabelIDs: []string{"SENT"},
	}

	result, err := activities.storeMessage(ctx, account, msg, &settings)
	if err != nil {
		t.Fatalf("storeMessage: %v", err)
	}
	if !result.Stored {
		t.Fatalf("store result = %+v, want stored message", result)
	}

	var count int64
	if err := db.Table("crm_email_messages").Where("message_external_id = ?", "gmail-msg-2").Count(&count).Error; err != nil {
		t.Fatalf("count stored messages: %v", err)
	}
	if count != 1 {
		t.Fatalf("stored message count = %d, want 1 (external cc should bypass internal exclusion)", count)
	}
}

func TestEmailSyncActivities_StoreMessageEnqueuesSignalDetectionForEligibleMessage(t *testing.T) {
	db := setupEmailSyncActivitiesTestDB(t)
	emailRepo := repository.NewCRMEmailRepository(db)
	contactRepo := repository.NewCRMContactRepository(db)
	activities := NewEmailSyncActivities(nil, emailRepo, contactRepo, nil, nil, nil, nil)
	starter := &fakeSignalStarter{}
	activities.signalIngestion = crmsignal.NewIngestionService(emailRepo, starter)
	ctx := context.Background()

	account := &model.CRMEmailAccount{
		ID:           "acct-1",
		WorkspaceID:  "ws-1",
		MemberID:     "member-1",
		Provider:     "gmail",
		EmailAddress: "owner@example.com",
		IsActive:     true,
	}
	if err := emailRepo.CreateAccount(ctx, account); err != nil {
		t.Fatalf("seed account: %v", err)
	}

	settings := model.DefaultEmailSyncSettings()
	settings.RecordCreationMode = "always"

	msg := &sync.GmailMessage{
		ID:       "gmail-msg-signal-1",
		ThreadID: "gmail-thread-signal-1",
		Subject:  "Pricing question",
		From:     "buyer@example.com",
		To:       []string{"owner@example.com"},
		BodyText: "Can you send pricing for the enterprise plan?",
		Date:     time.Now(),
	}

	result, err := activities.storeMessage(ctx, account, msg, &settings)
	if err != nil {
		t.Fatalf("storeMessage: %v", err)
	}
	if !result.Stored {
		t.Fatalf("store result = %+v, want stored message", result)
	}
	if len(starter.messageIDs) != 1 {
		t.Fatalf("started workflows = %v, want 1", starter.messageIDs)
	}
	if len(starter.payloads) != 1 || len(starter.payloads[0]) != 1 {
		t.Fatalf("payloads = %#v, want one payload", starter.payloads)
	}
	payload := starter.payloads[0][0]
	if payload.SourceID == "" || payload.Body == "" {
		t.Fatalf("payload = %+v, want source id and body", payload)
	}
}

func TestEmailSyncActivities_IncrementalSyncUsesHistoryCursor(t *testing.T) {
	db := setupEmailSyncActivitiesTestDB(t)
	emailRepo := repository.NewCRMEmailRepository(db)
	contactRepo := repository.NewCRMContactRepository(db)

	account := &model.CRMEmailAccount{
		ID:                     "acct-1",
		WorkspaceID:            "ws-1",
		MemberID:               "member-1",
		Provider:               "gmail",
		EmailAddress:           "owner@example.com",
		NormalizedEmailAddress: stringPtr("owner@example.com"),
		LastHistoryID:          stringPtr("hist-1"),
		IsActive:               true,
		Status:                 model.CRMEmailAccountStatusConnected,
		SyncState:              model.JSONB{"status": model.CRMEmailAccountStatusConnected, "last_history_id": "hist-1"},
	}
	if err := emailRepo.CreateAccount(context.Background(), account); err != nil {
		t.Fatalf("seed account: %v", err)
	}

	activities := &EmailSyncActivities{
		gmailClient: &fakeHistorySyncClient{
			listHistoryFn: func(ctx context.Context, accessToken, startHistoryID string) (*sync.GmailHistoryResult, error) {
				if startHistoryID != "hist-1" {
					t.Fatalf("startHistoryID = %q, want hist-1", startHistoryID)
				}
				return &sync.GmailHistoryResult{
					MessageIDs:      []string{"gmail-msg-3"},
					LatestHistoryID: "hist-2",
				}, nil
			},
			getMessageDetailFn: func(ctx context.Context, accessToken, messageID string) (*sync.GmailMessage, error) {
				return &sync.GmailMessage{
					ID:       messageID,
					ThreadID: "gmail-thread-3",
					Subject:  "Incremental",
					From:     "buyer@example.com",
					To:       []string{"owner@example.com"},
					Date:     time.Now(),
				}, nil
			},
		},
		emailRepo: emailRepo,
		resolver:  crmemail.NewResolver(contactRepo),
	}

	result, err := activities.IncrementalSyncActivity(context.Background(), "acct-1")
	if err != nil {
		t.Fatalf("IncrementalSyncActivity: %v", err)
	}
	if result.NewHistoryID != "hist-2" {
		t.Fatalf("result.NewHistoryID = %q, want hist-2", result.NewHistoryID)
	}

	stored, err := emailRepo.GetAccountByID(context.Background(), "acct-1")
	if err != nil {
		t.Fatalf("reload account: %v", err)
	}
	if stored.LastHistoryID == nil || *stored.LastHistoryID != "hist-2" {
		t.Fatalf("last_history_id = %v, want hist-2", stored.LastHistoryID)
	}
	if stored.LastSyncedAt == nil {
		t.Fatal("expected last_synced_at to be updated")
	}
	if stored.SyncState["phase"] != crmemail.SyncPhaseIdle {
		t.Fatalf("sync_state.phase = %v, want %q", stored.SyncState["phase"], crmemail.SyncPhaseIdle)
	}
	if stored.SyncState["status"] != model.CRMEmailAccountStatusConnected {
		t.Fatalf("sync_state.status = %v, want connected", stored.SyncState["status"])
	}
	lastCycle, ok := stored.SyncState["last_cycle"].(map[string]interface{})
	if !ok {
		t.Fatalf("sync_state.last_cycle = %T, want map", stored.SyncState["last_cycle"])
	}
	if got := int(lastCycle["messages_seen"].(float64)); got != 1 {
		t.Fatalf("last_cycle.messages_seen = %d, want 1", got)
	}
	if got := int(lastCycle["messages_stored"].(float64)); got != 1 {
		t.Fatalf("last_cycle.messages_stored = %d, want 1", got)
	}

	var messageCount int64
	if err := db.Table("crm_email_messages").Where("email_account_id = ?", "acct-1").Count(&messageCount).Error; err != nil {
		t.Fatalf("count messages: %v", err)
	}
	if messageCount != 1 {
		t.Fatalf("message count = %d, want 1", messageCount)
	}
}

func TestEmailSyncActivities_IncrementalSyncRecoversExpiredHistoryCursor(t *testing.T) {
	db := setupEmailSyncActivitiesTestDB(t)
	emailRepo := repository.NewCRMEmailRepository(db)
	contactRepo := repository.NewCRMContactRepository(db)

	lastSyncedAt := time.Now().Add(-2 * time.Hour)
	account := &model.CRMEmailAccount{
		ID:                     "acct-1",
		WorkspaceID:            "ws-1",
		MemberID:               "member-1",
		Provider:               "gmail",
		EmailAddress:           "owner@example.com",
		NormalizedEmailAddress: stringPtr("owner@example.com"),
		LastHistoryID:          stringPtr("hist-stale"),
		LastSyncedAt:           &lastSyncedAt,
		IsActive:               true,
		Status:                 model.CRMEmailAccountStatusConnected,
		SyncState:              model.JSONB{"status": model.CRMEmailAccountStatusConnected, "last_history_id": "hist-stale"},
	}
	if err := emailRepo.CreateAccount(context.Background(), account); err != nil {
		t.Fatalf("seed account: %v", err)
	}

	listMessagesCalled := false
	activities := &EmailSyncActivities{
		gmailClient: &fakeHistorySyncClient{
			listHistoryFn: func(ctx context.Context, accessToken, startHistoryID string) (*sync.GmailHistoryResult, error) {
				return nil, &sync.GmailAPIError{StatusCode: 404, Body: "HistoryId is too old"}
			},
			listMessagesFn: func(ctx context.Context, accessToken, query string, maxResults int, pageToken string) ([]sync.GmailMessage, string, error) {
				listMessagesCalled = true
				return []sync.GmailMessage{
					{
						ID:       "gmail-msg-4",
						ThreadID: "gmail-thread-4",
						Subject:  "Recovered",
						From:     "buyer@example.com",
						To:       []string{"owner@example.com"},
						Date:     time.Now(),
					},
				}, "", nil
			},
			getMailboxProfileFn: func(ctx context.Context, accessToken string) (*sync.GmailProfile, error) {
				return &sync.GmailProfile{
					EmailAddress: "owner@example.com",
					HistoryID:    "hist-fresh",
				}, nil
			},
		},
		emailRepo: emailRepo,
		resolver:  crmemail.NewResolver(contactRepo),
	}

	result, err := activities.IncrementalSyncActivity(context.Background(), "acct-1")
	if err != nil {
		t.Fatalf("IncrementalSyncActivity: %v", err)
	}
	if !listMessagesCalled {
		t.Fatal("expected recovery backfill to call ListMessages")
	}
	if result.NewHistoryID != "hist-fresh" {
		t.Fatalf("result.NewHistoryID = %q, want hist-fresh", result.NewHistoryID)
	}

	stored, err := emailRepo.GetAccountByID(context.Background(), "acct-1")
	if err != nil {
		t.Fatalf("reload account: %v", err)
	}
	if stored.LastHistoryID == nil || *stored.LastHistoryID != "hist-fresh" {
		t.Fatalf("last_history_id = %v, want hist-fresh", stored.LastHistoryID)
	}
	if stored.SyncState["phase"] != crmemail.SyncPhaseIdle {
		t.Fatalf("sync_state.phase = %v, want %q", stored.SyncState["phase"], crmemail.SyncPhaseIdle)
	}
	lastCycle, ok := stored.SyncState["last_cycle"].(map[string]interface{})
	if !ok {
		t.Fatalf("sync_state.last_cycle = %T, want map", stored.SyncState["last_cycle"])
	}
	if recoveryTriggered, ok := lastCycle["recovery_triggered"].(bool); !ok || !recoveryTriggered {
		t.Fatalf("last_cycle.recovery_triggered = %v, want true", lastCycle["recovery_triggered"])
	}
}
