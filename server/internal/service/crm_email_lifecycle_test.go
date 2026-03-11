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
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			display_id TEXT NOT NULL,
			first_name TEXT NOT NULL,
			last_name TEXT,
			email TEXT,
			custom_properties BLOB NOT NULL DEFAULT (CAST('{}' AS BLOB)),
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
			id TEXT PRIMARY KEY,
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
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			email_account_id TEXT NOT NULL REFERENCES crm_email_accounts(id) ON DELETE CASCADE,
			thread_id TEXT REFERENCES crm_email_threads(id) ON DELETE SET NULL,
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
	started  []string
	canceled []string
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

	if err := svc.DeleteAccount(ctx, "acct-1", "member-1", false); err != nil {
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

	if err := svc.PurgeAccountData(ctx, "acct-1", false); err == nil {
		t.Fatal("expected purge to require admin")
	}
	if err := svc.PurgeAccountData(ctx, "acct-1", true); err != nil {
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
