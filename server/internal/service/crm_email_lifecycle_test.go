package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/crmemail"
	appcrypto "github.com/helpin-ai/helpin/server/internal/crypto"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/oauth"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/sync"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupCRMEmailLifecycleTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:crm_email_lifecycle_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	statements := []string{
		`PRAGMA foreign_keys = ON`,
		`CREATE TABLE IF NOT EXISTS workspaces (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			slug TEXT NOT NULL,
			owner_id TEXT NOT NULL,
			organization_id TEXT,
			description TEXT,
			logo_url TEXT,
			timezone TEXT NOT NULL DEFAULT 'UTC',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS crm_contacts (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL,
			display_id TEXT NOT NULL,
			first_name TEXT NOT NULL,
			last_name TEXT,
			email TEXT,
			phone TEXT,
			job_title TEXT,
			description TEXT,
			labels BLOB NOT NULL DEFAULT (CAST('{}' AS BLOB)),
			primary_location TEXT,
			country_code TEXT,
			country_name TEXT,
			linkedin_url TEXT,
			facebook_url TEXT,
			instagram_url TEXT,
			angellist_url TEXT,
			x_url TEXT,
			lifecycle_stage TEXT NOT NULL DEFAULT 'subscriber',
			lead_status TEXT NOT NULL DEFAULT 'new',
			owner_member_id TEXT,
			avatar_url TEXT,
			source TEXT,
			custom_properties BLOB NOT NULL DEFAULT (CAST('{}' AS BLOB)),
			email_status TEXT NOT NULL DEFAULT 'valid',
			email_status_reason TEXT,
			email_status_updated_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_crm_contacts_workspace_email_lower_unique
			ON crm_contacts(workspace_id, lower(email))
			WHERE email IS NOT NULL`,
		`CREATE TABLE IF NOT EXISTS crm_email_accounts (
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
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_crm_email_accounts_workspace_provider_normalized_email_unique
			ON crm_email_accounts(workspace_id, provider, normalized_email_address)
			WHERE normalized_email_address IS NOT NULL`,
		`CREATE TABLE IF NOT EXISTS crm_email_threads (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL,
			email_account_id TEXT NOT NULL REFERENCES crm_email_accounts(id) ON DELETE CASCADE,
			thread_external_id TEXT NOT NULL,
			subject TEXT NOT NULL,
			last_message_at DATETIME NOT NULL,
			message_count INTEGER NOT NULL DEFAULT 0,
			contact_ids BLOB NOT NULL DEFAULT (CAST('[]' AS BLOB)),
			deal_id TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS crm_email_messages (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL,
			email_account_id TEXT NOT NULL REFERENCES crm_email_accounts(id) ON DELETE CASCADE,
			thread_id TEXT REFERENCES crm_email_threads(id) ON DELETE SET NULL,
			message_external_id TEXT,
			rfc_message_id TEXT,
			in_reply_to TEXT,
			references_header TEXT,
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
		`CREATE TABLE IF NOT EXISTS crm_email_message_contacts (
			message_id TEXT NOT NULL,
			contact_id TEXT NOT NULL,
			participant_role TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (message_id, contact_id, participant_role)
		)`,
		`CREATE TABLE IF NOT EXISTS crm_calendar_events (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			email_account_id TEXT NOT NULL REFERENCES crm_email_accounts(id) ON DELETE CASCADE,
			external_event_id TEXT,
			title TEXT NOT NULL,
			description TEXT,
			start_time DATETIME NOT NULL,
			end_time DATETIME NOT NULL,
			location TEXT,
			attendees BLOB NOT NULL DEFAULT (CAST('[]' AS BLOB)),
			contact_ids BLOB NOT NULL DEFAULT (CAST('[]' AS BLOB)),
			deal_id TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("apply schema statement %q: %v", stmt, err)
		}
	}

	return db
}

type fakeGmailOAuthProvider struct {
	tokenPair *oauth.TokenPair
}

func (f *fakeGmailOAuthProvider) GenerateAuthURL(state string) string {
	return "https://example.test/oauth?state=" + state
}

func (f *fakeGmailOAuthProvider) ExchangeCode(ctx context.Context, code string) (*oauth.TokenPair, error) {
	return f.tokenPair, nil
}

type fakeMailboxClient struct {
	profile *sync.GmailProfile
}

func (f *fakeMailboxClient) GetMailboxProfile(ctx context.Context, accessToken string) (*sync.GmailProfile, error) {
	return f.profile, nil
}

func (f *fakeMailboxClient) GetValidToken(ctx context.Context, account *model.CRMEmailAccount) (string, error) {
	return "token", nil
}

func (f *fakeMailboxClient) SendMessage(ctx context.Context, accessToken, from string, to []string, cc []string, subject, bodyHTML string) (*sync.GmailSendResult, error) {
	return &sync.GmailSendResult{ID: "sent-1", ThreadID: "thread-1"}, nil
}

type fakeEmailSyncWorkflowRunner struct {
	started   []string
	canceled  []string
	requested []string
}

func (f *fakeEmailSyncWorkflowRunner) RequestAccountSync(ctx context.Context, accountID, mode string) error {
	f.requested = append(f.requested, accountID+":"+mode)
	return nil
}

func (f *fakeEmailSyncWorkflowRunner) StartAccountSync(ctx context.Context, accountID string) error {
	f.started = append(f.started, accountID)
	return nil
}

func (f *fakeEmailSyncWorkflowRunner) CancelAccountSync(ctx context.Context, accountID string) error {
	f.canceled = append(f.canceled, accountID)
	return nil
}

func TestCRMEmailService_DeleteAccountDisconnectsAndPreservesData(t *testing.T) {
	db := setupCRMEmailLifecycleTestDB(t)
	emailRepo := repository.NewCRMEmailRepository(db)
	contactRepo := repository.NewCRMContactRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	runner := &fakeEmailSyncWorkflowRunner{}

	encryptionKey := []byte("12345678901234567890123456789012")
	encAccess, err := appcrypto.EncryptString("access-token", encryptionKey)
	if err != nil {
		t.Fatalf("encrypt access token: %v", err)
	}
	encRefresh, err := appcrypto.EncryptString("refresh-token", encryptionKey)
	if err != nil {
		t.Fatalf("encrypt refresh token: %v", err)
	}

	ctx := context.Background()
	mustExecCRMEmailLifecycle(t, db, `INSERT INTO workspaces (id, name, slug, owner_id, timezone) VALUES (?, ?, ?, ?, ?)`, "ws-1", "Workspace", "workspace", "owner-1", "UTC")
	account := &model.CRMEmailAccount{
		ID:                     "acct-1",
		WorkspaceID:            "ws-1",
		MemberID:               "member-1",
		Provider:               model.CRMEmailProviderGmail,
		EmailAddress:           "owner@example.com",
		NormalizedEmailAddress: testStringPtr("owner@example.com"),
		AccessTokenEncrypted:   &encAccess,
		RefreshTokenEncrypted:  &encRefresh,
		SyncState:              model.JSONB{"status": model.CRMEmailAccountStatusConnected, "last_history_id": "hist-1"},
		LastHistoryID:          testStringPtr("hist-1"),
		IsActive:               true,
		Status:                 model.CRMEmailAccountStatusConnected,
	}
	if err := emailRepo.CreateAccount(ctx, account); err != nil {
		t.Fatalf("create account: %v", err)
	}
	mustExecCRMEmailLifecycle(t, db, `INSERT INTO crm_email_threads (id, workspace_id, email_account_id, thread_external_id, subject, last_message_at, message_count, contact_ids) VALUES (?, ?, ?, ?, ?, ?, ?, CAST(? AS BLOB))`,
		"thread-1", "ws-1", "acct-1", "ext-thread-1", "Subject", time.Now(), 1, `[]`)

	svc := &CRMEmailService{
		emailRepo:     emailRepo,
		contactRepo:   contactRepo,
		workspaceRepo: workspaceRepo,
		encryptionKey: encryptionKey,
		resolver:      crmemail.NewResolver(contactRepo),
		syncRunner:    runner,
	}

	if err := svc.DeleteAccount(ctx, "ws-1", "acct-1", "member-1", false); err != nil {
		t.Fatalf("DeleteAccount: %v", err)
	}

	stored, err := emailRepo.GetAccountByID(ctx, "acct-1")
	if err != nil {
		t.Fatalf("reload account: %v", err)
	}
	if stored == nil {
		t.Fatal("expected account row to be preserved")
	}
	if stored.IsActive {
		t.Fatal("expected account to be inactive after disconnect")
	}
	if stored.Status != model.CRMEmailAccountStatusDisconnected {
		t.Fatalf("status = %q, want disconnected", stored.Status)
	}
	if stored.AccessTokenEncrypted != nil || stored.RefreshTokenEncrypted != nil {
		t.Fatal("expected stored OAuth tokens to be cleared")
	}
	if stored.LastHistoryID == nil || *stored.LastHistoryID != "hist-1" {
		t.Fatalf("last_history_id = %v, want hist-1", stored.LastHistoryID)
	}
	if stored.DisconnectedAt == nil {
		t.Fatal("expected disconnected_at to be set")
	}
	if stored.SyncState["phase"] != crmemail.SyncPhaseDisconnected {
		t.Fatalf("sync_state.phase = %v, want %q", stored.SyncState["phase"], crmemail.SyncPhaseDisconnected)
	}
	if len(runner.canceled) != 1 || runner.canceled[0] != "acct-1" {
		t.Fatalf("canceled workflows = %v, want acct-1", runner.canceled)
	}

	var threadCount int64
	if err := db.Table("crm_email_threads").Where("email_account_id = ?", "acct-1").Count(&threadCount).Error; err != nil {
		t.Fatalf("count threads: %v", err)
	}
	if threadCount != 1 {
		t.Fatalf("thread count = %d, want 1", threadCount)
	}
}

func TestCRMEmailService_SyncAccountIsWorkspaceScopedAndOwnerAuthorized(t *testing.T) {
	db := setupCRMEmailLifecycleTestDB(t)
	emailRepo := repository.NewCRMEmailRepository(db)
	runner := &fakeEmailSyncWorkflowRunner{}
	ctx := context.Background()
	mustExecCRMEmailLifecycle(t, db, `INSERT INTO workspaces (id, name, slug, owner_id, timezone) VALUES (?, ?, ?, ?, ?)`, "ws-1", "Workspace", "workspace", "owner-1", "UTC")
	account := &model.CRMEmailAccount{
		ID: "acct-1", WorkspaceID: "ws-1", MemberID: "member-1", Provider: model.CRMEmailProviderGmail,
		EmailAddress: "owner@example.com", IsActive: true, Status: model.CRMEmailAccountStatusConnected,
		SyncState: crmemail.MarkConnectedIdle(nil, "hist-1"), LastHistoryID: testStringPtr("hist-1"),
	}
	if err := emailRepo.CreateAccount(ctx, account); err != nil {
		t.Fatalf("create account: %v", err)
	}
	svc := &CRMEmailService{emailRepo: emailRepo, syncRunner: runner}

	if _, err := svc.SyncAccount(ctx, "ws-other", account.ID, account.MemberID, model.CRMEmailSyncModeIncremental, false); err == nil {
		t.Fatal("expected cross-workspace sync request to be rejected")
	}
	if _, err := svc.SyncAccount(ctx, account.WorkspaceID, account.ID, "member-other", model.CRMEmailSyncModeIncremental, false); err == nil {
		t.Fatal("expected non-owner sync request to be rejected")
	}
	result, err := svc.SyncAccount(ctx, account.WorkspaceID, account.ID, account.MemberID, model.CRMEmailSyncModeHistorical, false)
	if err != nil {
		t.Fatalf("SyncAccount: %v", err)
	}
	if result.SyncState["phase"] != "queued" || len(runner.requested) != 1 || runner.requested[0] != "acct-1:historical" {
		t.Fatalf("result/requests = %+v/%v, want queued historical sync", result.SyncState, runner.requested)
	}
}

func TestCRMEmailService_PurgeAccountDataRequiresAdmin(t *testing.T) {
	db := setupCRMEmailLifecycleTestDB(t)
	emailRepo := repository.NewCRMEmailRepository(db)
	contactRepo := repository.NewCRMContactRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	runner := &fakeEmailSyncWorkflowRunner{}

	ctx := context.Background()
	mustExecCRMEmailLifecycle(t, db, `INSERT INTO workspaces (id, name, slug, owner_id, timezone) VALUES (?, ?, ?, ?, ?)`, "ws-1", "Workspace", "workspace", "owner-1", "UTC")
	account := &model.CRMEmailAccount{
		ID:                     "acct-1",
		WorkspaceID:            "ws-1",
		MemberID:               "member-1",
		Provider:               model.CRMEmailProviderGmail,
		EmailAddress:           "owner@example.com",
		NormalizedEmailAddress: testStringPtr("owner@example.com"),
		SyncState:              model.JSONB{"status": model.CRMEmailAccountStatusConnected},
		IsActive:               true,
		Status:                 model.CRMEmailAccountStatusConnected,
	}
	if err := emailRepo.CreateAccount(ctx, account); err != nil {
		t.Fatalf("create account: %v", err)
	}
	mustExecCRMEmailLifecycle(t, db, `INSERT INTO crm_email_threads (id, workspace_id, email_account_id, thread_external_id, subject, last_message_at, message_count, contact_ids) VALUES (?, ?, ?, ?, ?, ?, ?, CAST(? AS BLOB))`,
		"thread-1", "ws-1", "acct-1", "ext-thread-1", "Subject", time.Now(), 1, `[]`)

	svc := &CRMEmailService{
		emailRepo:     emailRepo,
		contactRepo:   contactRepo,
		workspaceRepo: workspaceRepo,
		resolver:      crmemail.NewResolver(contactRepo),
		syncRunner:    runner,
	}

	if err := svc.PurgeAccountData(ctx, "ws-1", "acct-1", false); err == nil {
		t.Fatal("expected purge to require admin")
	}
	if err := svc.PurgeAccountData(ctx, "ws-1", "acct-1", true); err != nil {
		t.Fatalf("PurgeAccountData: %v", err)
	}

	if len(runner.canceled) != 1 || runner.canceled[0] != "acct-1" {
		t.Fatalf("canceled workflows = %v, want acct-1", runner.canceled)
	}
	stored, err := emailRepo.GetAccountByID(ctx, "acct-1")
	if err != nil {
		t.Fatalf("reload account: %v", err)
	}
	if stored != nil {
		t.Fatal("expected account row to be deleted after purge")
	}

	var threadCount int64
	if err := db.Table("crm_email_threads").Where("email_account_id = ?", "acct-1").Count(&threadCount).Error; err != nil {
		t.Fatalf("count threads: %v", err)
	}
	if threadCount != 0 {
		t.Fatalf("thread count = %d, want 0 after purge", threadCount)
	}
}

func TestCRMEmailService_CompleteOAuthReusesDisconnectedMailbox(t *testing.T) {
	db := setupCRMEmailLifecycleTestDB(t)
	emailRepo := repository.NewCRMEmailRepository(db)
	contactRepo := repository.NewCRMContactRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	runner := &fakeEmailSyncWorkflowRunner{}
	ctx := context.Background()

	mustExecCRMEmailLifecycle(t, db, `INSERT INTO workspaces (id, name, slug, owner_id, timezone) VALUES (?, ?, ?, ?, ?)`, "ws-1", "Workspace", "workspace", "owner-1", "UTC")

	existing := &model.CRMEmailAccount{
		ID:                     "acct-existing",
		WorkspaceID:            "ws-1",
		MemberID:               "member-old",
		Provider:               model.CRMEmailProviderGmail,
		EmailAddress:           "owner@example.com",
		NormalizedEmailAddress: testStringPtr("owner@example.com"),
		SyncState:              model.JSONB{"status": model.CRMEmailAccountStatusDisconnected, "last_history_id": "hist-existing"},
		LastHistoryID:          testStringPtr("hist-existing"),
		LastSyncedAt:           testTimePtr(time.Now().Add(-2 * time.Hour)),
		IsActive:               false,
		Status:                 model.CRMEmailAccountStatusDisconnected,
		DisconnectedAt:         testTimePtr(time.Now().Add(-time.Hour)),
	}
	if err := emailRepo.CreateAccount(ctx, existing); err != nil {
		t.Fatalf("create existing account: %v", err)
	}

	pendingState := "oauth-state-1"
	pending := &model.CRMEmailAccount{
		ID:           "acct-pending",
		WorkspaceID:  "ws-1",
		MemberID:     "member-new",
		Provider:     model.CRMEmailProviderGmail,
		EmailAddress: "pending@oauth.local",
		IsActive:     false,
		Status:       model.CRMEmailAccountStatusPendingOAuth,
		OAuthState:   &pendingState,
		SyncState:    model.JSONB{"status": model.CRMEmailAccountStatusPendingOAuth},
	}
	if err := emailRepo.CreateAccount(ctx, pending); err != nil {
		t.Fatalf("create pending account: %v", err)
	}

	svc := &CRMEmailService{
		emailRepo:     emailRepo,
		contactRepo:   contactRepo,
		workspaceRepo: workspaceRepo,
		oauthClient: &fakeGmailOAuthProvider{
			tokenPair: &oauth.TokenPair{
				AccessToken:  "new-access-token",
				RefreshToken: "new-refresh-token",
				ExpiresAt:    time.Now().Add(time.Hour),
			},
		},
		encryptionKey: []byte("12345678901234567890123456789012"),
		gmailSync: &fakeMailboxClient{
			profile: &sync.GmailProfile{
				EmailAddress: "Owner@Example.com",
				HistoryID:    "hist-fresh",
			},
		},
		resolver:   crmemail.NewResolver(contactRepo),
		syncRunner: runner,
	}

	slug, err := svc.CompleteOAuth(ctx, pendingState, "code-1")
	if err != nil {
		t.Fatalf("CompleteOAuth: %v", err)
	}
	if slug != "workspace" {
		t.Fatalf("slug = %q, want workspace", slug)
	}

	stored, err := emailRepo.GetAccountByID(ctx, "acct-existing")
	if err != nil {
		t.Fatalf("reload existing account: %v", err)
	}
	if stored == nil {
		t.Fatal("expected existing mailbox to remain")
	}
	if stored.MemberID != "member-new" {
		t.Fatalf("member_id = %q, want member-new", stored.MemberID)
	}
	if !stored.IsActive || stored.Status != model.CRMEmailAccountStatusConnected {
		t.Fatalf("account active/status = %v/%q, want true/connected", stored.IsActive, stored.Status)
	}
	if stored.DisconnectedAt != nil {
		t.Fatal("expected disconnected_at to be cleared on reconnect")
	}
	if stored.LastHistoryID == nil || *stored.LastHistoryID != "hist-existing" {
		t.Fatalf("last_history_id = %v, want preserved hist-existing", stored.LastHistoryID)
	}
	if stored.NormalizedEmailAddress == nil || *stored.NormalizedEmailAddress != "owner@example.com" {
		t.Fatalf("normalized_email_address = %v, want owner@example.com", stored.NormalizedEmailAddress)
	}
	if stored.SyncState["status"] != model.CRMEmailAccountStatusConnected {
		t.Fatalf("sync_state.status = %v, want connected", stored.SyncState["status"])
	}
	if stored.SyncState["phase"] != crmemail.SyncPhaseIdle {
		t.Fatalf("sync_state.phase = %v, want %q", stored.SyncState["phase"], crmemail.SyncPhaseIdle)
	}
	if stored.SyncState["last_error"] != nil {
		t.Fatalf("sync_state.last_error = %v, want nil", stored.SyncState["last_error"])
	}
	if stored.AccessTokenEncrypted == nil || stored.RefreshTokenEncrypted == nil {
		t.Fatal("expected fresh encrypted tokens to be stored")
	}

	pendingStored, err := emailRepo.GetAccountByID(ctx, "acct-pending")
	if err != nil {
		t.Fatalf("reload pending account: %v", err)
	}
	if pendingStored != nil {
		t.Fatal("expected pending mailbox row to be deleted after merge")
	}
	if len(runner.started) != 1 || runner.started[0] != "acct-existing" {
		t.Fatalf("started workflows = %v, want acct-existing", runner.started)
	}
}

func TestCRMEmailService_GetAccountDiagnostics(t *testing.T) {
	db := setupCRMEmailLifecycleTestDB(t)
	emailRepo := repository.NewCRMEmailRepository(db)
	contactRepo := repository.NewCRMContactRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	ctx := context.Background()

	mustExecCRMEmailLifecycle(t, db, `INSERT INTO workspaces (id, name, slug, owner_id, timezone) VALUES (?, ?, ?, ?, ?)`, "ws-1", "Workspace", "workspace", "owner-1", "UTC")
	now := time.Now().UTC()
	account := &model.CRMEmailAccount{
		ID:                     "acct-1",
		WorkspaceID:            "ws-1",
		MemberID:               "member-1",
		Provider:               model.CRMEmailProviderGmail,
		EmailAddress:           "owner@example.com",
		NormalizedEmailAddress: testStringPtr("owner@example.com"),
		LastHistoryID:          testStringPtr("hist-1"),
		LastSyncedAt:           testTimePtr(now.Add(-time.Minute)),
		IsActive:               true,
		Status:                 model.CRMEmailAccountStatusConnected,
		SyncState: model.JSONB{
			"status":               model.CRMEmailAccountStatusError,
			"phase":                crmemail.SyncPhaseError,
			"last_history_id":      "hist-1",
			"last_failure_at":      now.Format(time.RFC3339Nano),
			"consecutive_failures": 2,
			"last_error": map[string]interface{}{
				"operation": "list_history",
				"code":      "sync_error",
				"message":   "history too old",
			},
			"last_cycle": map[string]interface{}{
				"mode":                 "incremental",
				"started_at":           now.Add(-2 * time.Minute).Format(time.RFC3339Nano),
				"completed_at":         now.Add(-time.Minute).Format(time.RFC3339Nano),
				"messages_seen":        4,
				"messages_stored":      2,
				"duplicates_skipped":   1,
				"filtered_skipped":     0,
				"internal_skipped":     1,
				"contacts_created":     1,
				"associations_written": 2,
				"threads_touched":      1,
				"recovery_triggered":   true,
			},
		},
	}
	if err := emailRepo.CreateAccount(ctx, account); err != nil {
		t.Fatalf("create account: %v", err)
	}
	mustExecCRMEmailLifecycle(t, db, `INSERT INTO crm_email_threads (id, workspace_id, email_account_id, thread_external_id, subject, last_message_at, message_count, contact_ids) VALUES (?, ?, ?, ?, ?, ?, ?, CAST(? AS BLOB))`,
		"thread-1", "ws-1", "acct-1", "ext-thread-1", "Subject", now, 1, `[]`)
	mustExecCRMEmailLifecycle(t, db, `INSERT INTO crm_email_messages (id, workspace_id, email_account_id, thread_id, message_external_id, from_address, to_addresses, cc_addresses, subject, direction, sent_at) VALUES (?, ?, ?, ?, ?, ?, CAST(? AS BLOB), CAST(? AS BLOB), ?, ?, ?)`,
		"msg-1", "ws-1", "acct-1", "thread-1", "external-1", "buyer@example.com", `["owner@example.com"]`, `[]`, "Subject", model.CRMEmailDirectionInbound, now)
	mustExecCRMEmailLifecycle(t, db, `INSERT INTO crm_calendar_events (id, workspace_id, email_account_id, external_event_id, title, start_time, end_time, attendees, contact_ids) VALUES (?, ?, ?, ?, ?, ?, ?, CAST(? AS BLOB), CAST(? AS BLOB))`,
		"evt-1", "ws-1", "acct-1", "event-1", "Meeting", now, now.Add(time.Hour), `[]`, `[]`)

	svc := &CRMEmailService{
		emailRepo:     emailRepo,
		contactRepo:   contactRepo,
		workspaceRepo: workspaceRepo,
		resolver:      crmemail.NewResolver(contactRepo),
	}

	diagnostics, err := svc.GetAccountDiagnostics(ctx, "ws-1", "acct-1", "member-1", true)
	if err != nil {
		t.Fatalf("GetAccountDiagnostics: %v", err)
	}
	if diagnostics.Counts.Threads != 1 || diagnostics.Counts.Messages != 1 || diagnostics.Counts.CalendarEvents != 1 {
		t.Fatalf("counts = %+v, want 1/1/1", diagnostics.Counts)
	}
	if diagnostics.AssociationHealth.MessagesMissingAssociations != 1 {
		t.Fatalf("messages_missing_associations = %d, want 1", diagnostics.AssociationHealth.MessagesMissingAssociations)
	}
	if diagnostics.Sync.ConsecutiveFailures != 2 {
		t.Fatalf("consecutive_failures = %d, want 2", diagnostics.Sync.ConsecutiveFailures)
	}
	if diagnostics.Sync.LastError == nil || diagnostics.Sync.LastError.Operation != "list_history" {
		t.Fatalf("last_error = %+v, want list_history", diagnostics.Sync.LastError)
	}
	if diagnostics.Sync.LastCycle == nil || diagnostics.Sync.LastCycle.MessagesSeen != 4 || !diagnostics.Sync.LastCycle.RecoveryTriggered {
		t.Fatalf("last_cycle = %+v, want populated recovery stats", diagnostics.Sync.LastCycle)
	}
}

func TestCRMEmailService_RebuildAccountAssociations(t *testing.T) {
	db := setupCRMEmailLifecycleTestDB(t)
	emailRepo := repository.NewCRMEmailRepository(db)
	contactRepo := repository.NewCRMContactRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	ctx := context.Background()

	mustExecCRMEmailLifecycle(t, db, `INSERT INTO workspaces (id, name, slug, owner_id, timezone) VALUES (?, ?, ?, ?, ?)`, "ws-1", "Workspace", "workspace", "owner-1", "UTC")
	account := &model.CRMEmailAccount{
		ID:                     "acct-1",
		WorkspaceID:            "ws-1",
		MemberID:               "member-1",
		Provider:               model.CRMEmailProviderGmail,
		EmailAddress:           "owner@example.com",
		NormalizedEmailAddress: testStringPtr("owner@example.com"),
		IsActive:               true,
		Status:                 model.CRMEmailAccountStatusConnected,
		SyncState:              crmemail.MarkConnectedIdle(nil, ""),
	}
	if err := emailRepo.CreateAccount(ctx, account); err != nil {
		t.Fatalf("create account: %v", err)
	}

	mustExecCRMEmailLifecycle(t, db, `INSERT INTO crm_contacts (id, workspace_id, display_id, first_name, email, custom_properties) VALUES (?, ?, ?, ?, ?, CAST(? AS BLOB))`,
		"contact-1", "ws-1", "C-1", "Buyer", "buyer@example.com", `{}`)

	now := time.Now().UTC()
	mustExecCRMEmailLifecycle(t, db, `INSERT INTO crm_email_threads (id, workspace_id, email_account_id, thread_external_id, subject, last_message_at, message_count, contact_ids) VALUES (?, ?, ?, ?, ?, ?, ?, CAST(? AS BLOB))`,
		"thread-1", "ws-1", "acct-1", "ext-thread-1", "Subject", now, 1, `[]`)
	mustExecCRMEmailLifecycle(t, db, `INSERT INTO crm_email_messages (id, workspace_id, email_account_id, thread_id, message_external_id, from_address, to_addresses, cc_addresses, subject, direction, sent_at) VALUES (?, ?, ?, ?, ?, ?, CAST(? AS BLOB), CAST(? AS BLOB), ?, ?, ?)`,
		"msg-1", "ws-1", "acct-1", "thread-1", "external-1", "buyer@example.com", `["owner@example.com"]`, `[]`, "Subject", model.CRMEmailDirectionInbound, now)

	svc := &CRMEmailService{
		emailRepo:     emailRepo,
		contactRepo:   contactRepo,
		workspaceRepo: workspaceRepo,
		resolver:      crmemail.NewResolver(contactRepo),
	}

	result, err := svc.RebuildAccountAssociations(ctx, "ws-1", "acct-1", true)
	if err != nil {
		t.Fatalf("RebuildAccountAssociations: %v", err)
	}
	if result.MessagesScanned != 1 || result.MessagesRepaired != 1 {
		t.Fatalf("result = %+v, want one scanned and repaired message", result)
	}
	if result.AssociationsWritten != 1 {
		t.Fatalf("associations_written = %d, want 1", result.AssociationsWritten)
	}
	if result.ThreadsRefreshed != 1 {
		t.Fatalf("threads_refreshed = %d, want 1", result.ThreadsRefreshed)
	}

	var assocCount int64
	if err := db.Table("crm_email_message_contacts").Where("message_id = ?", "msg-1").Count(&assocCount).Error; err != nil {
		t.Fatalf("count associations: %v", err)
	}
	if assocCount != 1 {
		t.Fatalf("association count = %d, want 1", assocCount)
	}

	var storedMessage model.CRMEmailMessage
	if err := db.Table("crm_email_messages").Where("id = ?", "msg-1").First(&storedMessage).Error; err != nil {
		t.Fatalf("load message: %v", err)
	}
	if storedMessage.ContactID == nil || *storedMessage.ContactID != "contact-1" {
		t.Fatalf("contact_id = %v, want contact-1", storedMessage.ContactID)
	}

	var storedThread model.CRMEmailThread
	if err := db.Table("crm_email_threads").Where("id = ?", "thread-1").First(&storedThread).Error; err != nil {
		t.Fatalf("load thread: %v", err)
	}
	if string(storedThread.ContactIDs) != `["contact-1"]` {
		t.Fatalf("thread contact_ids = %s, want [\"contact-1\"]", string(storedThread.ContactIDs))
	}
}

func mustExecCRMEmailLifecycle(t *testing.T, db *gorm.DB, stmt string, args ...interface{}) {
	t.Helper()
	if err := db.Exec(stmt, args...).Error; err != nil {
		t.Fatalf("exec %q: %v", stmt, err)
	}
}

func testStringPtr(value string) *string {
	return &value
}

func testTimePtr(value time.Time) *time.Time {
	return &value
}
