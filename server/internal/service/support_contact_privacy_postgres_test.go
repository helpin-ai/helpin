//go:build integration

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/dbmigrate"
	"github.com/helpin-ai/helpin/server/internal/email"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Use the actual ledger, including inbox projection and snapshot triggers.
// GORM-only fixtures do not exercise the production deletion transaction.
func privacyServiceDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("CONTACT_PRIVACY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set CONTACT_PRIVACY_TEST_DATABASE_URL to disposable PostgreSQL with vector and CREATEDB privileges")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	adminSQL, err := admin.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := adminSQL.Close(); err != nil {
			t.Error(err)
		}
	})
	database := fmt.Sprintf("contact_privacy_%d", time.Now().UnixNano())
	privacyExec(t, admin, "CREATE DATABASE "+database)
	t.Cleanup(func() {
		if err := admin.Exec("DROP DATABASE " + database + " WITH (FORCE)").Error; err != nil {
			t.Error(err)
		}
	})
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	parsed.Path = "/" + database
	db, err := gorm.Open(postgres.Open(parsed.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Error(err)
		}
	})
	privacyExec(t, db, "CREATE EXTENSION vector; CREATE EXTENSION pgcrypto")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := dbmigrate.Up(ctx, sqlDB); err != nil {
		t.Fatal(err)
	}
	if err := dbmigrate.Up(ctx, sqlDB); err != nil {
		t.Fatal(err)
	}

	return db
}
func seedContactPrivacyWorkspace(t *testing.T, db *gorm.DB, workspaceID string) {
	t.Helper()
	owner := uuid.NewString()
	privacyExec(t, db, `INSERT INTO users (id,email,full_name,password_hash) VALUES (?,?,'Test owner','test-only')`, owner, owner+"@example.invalid")
	privacyExec(t, db, `INSERT INTO workspaces (id,name,slug,workspace_key,owner_id) VALUES (?,'Test workspace',?,'TEST',?)`, workspaceID, workspaceID, owner)
}

func privacyExec(t *testing.T, db *gorm.DB, sql string, args ...any) {
	t.Helper()
	if err := db.Exec(sql, args...).Error; err != nil {
		t.Fatal(err)
	}
}
func privacyCheck(t *testing.T, db *gorm.DB, sql string, args ...any) {
	t.Helper()
	var ok bool
	if err := db.Raw(sql, args...).Scan(&ok).Error; err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatalf("assertion failed: %s", sql)
	}
}

func privacyInbox(db *gorm.DB) *SupportInboxService {
	return NewSupportInboxService(repository.NewSupportConversationRepository(db), repository.NewSupportMailboxRepository(db), repository.NewSupportMessageRepository(db), nil, nil,
		repository.NewSupportInboxInstallationRepository(db), repository.NewSupportInboxSessionRepository(db), nil, nil, nil, repository.NewCRMContactRepository(db), nil, nil, nil, nil)
}

func TestContactAnonymizationWidgetAdmissionPostgres(t *testing.T) {
	db := privacyServiceDB(t)
	ctx := context.Background()
	ws, contact := uuid.NewString(), uuid.NewString()
	seedContactPrivacyWorkspace(t, db, ws)
	privacyExec(t, db, `INSERT INTO crm_contacts (id,workspace_id,display_id,first_name,email) VALUES (?,?,1,'Alice','alice@example.com')`, contact, ws)
	privacyExec(t, db, `INSERT INTO crm_identity_links (workspace_id,anonymous_id,contact_id,identity_method,identity_trust) VALUES (?,'alice-browser',?,'anonymous','untrusted')`, ws, contact)
	inbox := privacyInbox(db)
	session := &model.SupportWidgetSession{WorkspaceID: ws, AnonymousID: "alice-browser", SessionToken: "alice-token", CustomerName: strPtr("Alice"), CustomerEmail: strPtr("alice@example.com"), ExpiresAt: time.Now().Add(time.Hour)}
	if err := inbox.sessionRepo.Create(ctx, session); err != nil {
		t.Fatal(err)
	}
	// Model a request which already loaded the session before deletion committed.
	if _, err := inbox.contactRepo.DeleteAnonymizingSupport(ctx, ws, contact); err != nil {
		t.Fatal(err)
	}
	if _, _, err := inbox.createWidgetConversation(ctx, session, "Question", "Retain message"); err == nil {
		t.Fatal("stale session was admitted")
	}
	privacyCheck(t, db, `SELECT count(*)=0 FROM crm_contacts WHERE workspace_id=?`, ws)
	privacyCheck(t, db, `SELECT count(*)=0 FROM support_conversations WHERE workspace_id=?`, ws)
}

func TestContactAnonymizationWidgetRollbackPostgres(t *testing.T) {
	db := privacyServiceDB(t)
	for _, table := range []string{"support_widget_sessions", "support_messages"} {
		t.Run(table, func(t *testing.T) {
			ctx := context.Background()
			ws := uuid.NewString()
			seedContactPrivacyWorkspace(t, db, ws)
			inbox := privacyInbox(db)
			session := &model.SupportWidgetSession{WorkspaceID: ws, AnonymousID: uuid.NewString(), SessionToken: uuid.NewString(), CustomerEmail: strPtr("new@example.com"), ExpiresAt: time.Now().Add(time.Hour)}
			if err := inbox.sessionRepo.Create(ctx, session); err != nil {
				t.Fatal(err)
			}
			privacyExec(t, db, `CREATE OR REPLACE FUNCTION reject_privacy_test_write() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected admission failure'; END $$`)
			privacyExec(t, db, "CREATE TRIGGER reject_privacy_test_write BEFORE INSERT OR UPDATE ON "+table+" FOR EACH ROW EXECUTE FUNCTION reject_privacy_test_write()")
			t.Cleanup(func() { privacyExec(t, db, "DROP TRIGGER reject_privacy_test_write ON "+table) })
			if _, _, err := inbox.createWidgetConversation(ctx, session, "Question", "Keep text"); err == nil {
				t.Fatal("injected failure was ignored")
			}
			privacyCheck(t, db, `SELECT count(*)=0 FROM crm_contacts WHERE workspace_id=?`, ws)
			privacyCheck(t, db, `SELECT count(*)=0 FROM support_conversations WHERE workspace_id=?`, ws)
			privacyCheck(t, db, `SELECT conversation_id IS NULL FROM support_widget_sessions WHERE id=?`, session.ID)
		})
	}
}

func privacyEmail(t *testing.T, db *gorm.DB, ws string) *EmailFallbackService {
	t.Helper()
	inbox := privacyInbox(db)
	settings := model.DefaultSupportInboxSettings()
	settings.EmailFallbackEnabled = true
	raw, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	if err := inbox.installationRepo.Create(context.Background(), &model.SupportWidgetInstallation{WorkspaceID: ws, WidgetKey: uuid.NewString(), SecretKey: "test-only", Settings: string(raw), Active: true}); err != nil {
		t.Fatal(err)
	}
	return NewEmailFallbackService(nil, websocket.NewHub(), nil, email.NewClient("test-only", "support@example.com"), inbox.messageRepo, inbox.conversationRepo, repository.NewSupportEmailLogRepository(db), repository.NewSupportEmailWebhookEventRepository(db), inbox.installationRepo, inbox.sessionRepo, repository.NewWorkspaceRepository(db), "replies.example.com", "https://example.com", "test")
}

func TestContactAnonymizationInboundRetryPostgres(t *testing.T) {
	db := privacyServiceDB(t)
	ctx := context.Background()
	ws, contact, conv := uuid.NewString(), uuid.NewString(), uuid.NewString()
	seedContactPrivacyWorkspace(t, db, ws)
	privacyExec(t, db, `INSERT INTO crm_contacts(id,workspace_id,display_id,first_name,email) VALUES (?,?,1,'Alice','alice@example.com')`, contact, ws)
	privacyExec(t, db, `INSERT INTO support_conversations(id,workspace_id,display_id,subject,customer_email,crm_contact_id) VALUES (?,?,1,'Keep subject','alice@example.com',?)`, conv, ws, contact)
	privacyExec(t, db, `INSERT INTO support_email_logs(workspace_id,conversation_id,direction,postmark_message_id,from_email) VALUES (?,?,'inbound','opaque-provider-receipt','alice@example.com')`, ws, conv)
	messageID := uuid.NewString()
	privacyExec(t, db, `INSERT INTO support_messages(id,workspace_id,conversation_id,sender_type,content) VALUES (?,?,?,'user','Retain reply')`, messageID, ws, conv)
	privacyExec(t, db, `INSERT INTO support_email_logs(workspace_id,conversation_id,direction,postmark_message_id,message_ids,to_email) VALUES (?,?,'outbound','outbound-receipt',ARRAY[?],'alice@example.com')`, ws, conv, messageID)
	svc := privacyEmail(t, db, ws)
	if _, err := repository.NewCRMContactRepository(db).DeleteAnonymizingSupport(ctx, ws, contact); err != nil {
		t.Fatal(err)
	}
	receipt, err := svc.emailLogRepo.GetByPostmarkMessageID(ctx, "opaque-provider-receipt")
	if err != nil || receipt == nil {
		t.Fatalf("receipt lost: %v", err)
	}
	// No route service is wired: a duplicate must stop before route resolution,
	// webhook recording, or dispatching AI work using retained message content.
	payload := model.PostmarkInboundPayload{MessageID: "opaque-provider-receipt", To: "route-test@replies.example.com", OriginalRecipient: "route-test@replies.example.com", FromFull: model.PostmarkAddress{Email: "alice@example.com", Name: "Alice"}, TextBody: "Keep message"}
	if err := svc.ProcessInboundEmail(ctx, payload, ""); err != nil {
		t.Fatal(err)
	}
	privacyCheck(t, db, `SELECT count(*)=0 FROM crm_contacts WHERE workspace_id=?`, ws)
	privacyCheck(t, db, `SELECT count(*)=1 FROM support_conversations WHERE workspace_id=?`, ws)
	for _, event := range []struct {
		name string
		run  func() error
	}{
		{"open", func() error {
			return svc.ProcessOpenEvent(ctx, model.PostmarkOpenPayload{MessageID: "outbound-receipt", FirstOpen: true}, "")
		}},
		{"delivery", func() error {
			return svc.ProcessDeliveryEvent(ctx, model.PostmarkDeliveryPayload{MessageID: "outbound-receipt"}, "")
		}},
		{"bounce", func() error {
			return svc.ProcessBounceEvent(ctx, model.PostmarkBouncePayload{MessageID: "outbound-receipt", Description: "alice@example.com"}, "")
		}},
		{"complaint", func() error {
			return svc.ProcessSpamComplaintEvent(ctx, model.PostmarkSpamComplaintPayload{MessageID: "outbound-receipt", Description: "alice@example.com"}, "")
		}},
	} {
		t.Run(event.name, func(t *testing.T) {
			if err := event.run(); err != nil {
				t.Fatal(err)
			}
		})
	}
	privacyCheck(t, db, `SELECT count(*)=0 FROM support_email_webhook_events`)
}

func waitForPrivacyLock(ctx context.Context, db *gorm.DB) error {
	for {
		var waiting bool
		if err := db.Raw(`SELECT EXISTS (SELECT 1 FROM pg_locks l JOIN pg_stat_activity a ON a.pid=l.pid WHERE a.datname=current_database() AND NOT l.granted)`).Scan(&waiting).Error; err != nil {
			return err
		}
		if waiting {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func TestContactAnonymizationEmailSendRacePostgres(t *testing.T) {
	db := privacyServiceDB(t)
	for _, deletionFirst := range []bool{true, false} {
		t.Run(fmt.Sprintf("deletion_first=%t", deletionFirst), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			ws, contact, conv := uuid.NewString(), uuid.NewString(), uuid.NewString()
			seedContactPrivacyWorkspace(t, db, ws)
			privacyExec(t, db, `INSERT INTO crm_contacts(id,workspace_id,display_id,first_name,email) VALUES (?,?,1,'Alice','alice@example.com')`, contact, ws)
			privacyExec(t, db, `INSERT INTO support_conversations(id,workspace_id,display_id,subject,customer_email,crm_contact_id,status) VALUES (?,?,1,'Keep subject','alice@example.com',?,'open')`, conv, ws, contact)
			svc := privacyEmail(t, db, ws)
			msg := &model.SupportMessage{WorkspaceID: ws, ConversationID: conv, SenderType: "user", Content: "Retain reply", MessageType: "reply", Metadata: "{}"}
			if err := svc.messageRepo.Create(ctx, msg); err != nil {
				t.Fatal(err)
			}
			contacts := repository.NewCRMContactRepository(db)
			var deletedDuringPrepare atomic.Bool
			if deletionFirst {
				// Delete after the worker loaded its first conversation snapshot,
				// before it acquires send admission. No sleeps or timing guesses.
				name := "privacy_delete_during_prepare"
				if err := db.Callback().Query().After("gorm:query").Register(name, func(tx *gorm.DB) {
					if tx.Statement.Table == "support_conversations" && deletedDuringPrepare.CompareAndSwap(false, true) {
						_, err := contacts.DeleteAnonymizingSupport(ctx, ws, contact)
						if err != nil {
							tx.AddError(err)
						}
					}
				}); err != nil {
					t.Fatal(err)
				}
				defer db.Callback().Query().Remove(name)
			}
			sent := 0
			deleted := make(chan error, 1)
			svc.emailClient.SetHTTPClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				sent++
				if !deletionFirst {
					go func() { _, err := contacts.DeleteAnonymizingSupport(ctx, ws, contact); deleted <- err }()
					if err := waitForPrivacyLock(ctx, db); err != nil {
						return nil, err
					}
					select {
					case err := <-deleted:
						return nil, fmt.Errorf("deletion finished during send: %v", err)
					default:
					}
				}
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"ErrorCode":0,"Message":"OK","MessageID":"test-sent-receipt"}`))}, nil
			})})
			if err := svc.fireEmailBatch(ctx, conv, []string{msg.ID}, emailFallbackFireOptions{}); err != nil {
				t.Fatal(err)
			}
			if deletionFirst {
				if sent != 0 {
					t.Fatalf("sent %d emails after deletion", sent)
				}
			} else {
				select {
				case err := <-deleted:
					if err != nil {
						t.Fatal(err)
					}
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				}
				if sent != 1 {
					t.Fatalf("sent %d emails, want 1", sent)
				}
				privacyCheck(t, db, `SELECT to_email='' AND postmark_message_id='test-sent-receipt' FROM support_email_logs WHERE conversation_id=?`, conv)
			}
			privacyCheck(t, db, `SELECT anonymized_at IS NOT NULL AND customer_email IS NULL FROM support_conversations WHERE id=?`, conv)
			privacyCheck(t, db, `SELECT content='Retain reply' FROM support_messages WHERE id=?`, msg.ID)
		})
	}
}

func TestContactAnonymizationWaitsForWidgetCreationPostgres(t *testing.T) {
	db := privacyServiceDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ws, contact := uuid.NewString(), uuid.NewString()
	seedContactPrivacyWorkspace(t, db, ws)
	privacyExec(t, db, `INSERT INTO crm_contacts(id,workspace_id,display_id,first_name,email) VALUES (?,?,1,'Alice','alice@example.com')`, contact, ws)
	privacyExec(t, db, `INSERT INTO crm_identity_links(workspace_id,anonymous_id,contact_id,identity_method,identity_trust) VALUES (?,'alice-browser',?,'anonymous','untrusted')`, ws, contact)
	inbox := privacyInbox(db)
	session := &model.SupportWidgetSession{WorkspaceID: ws, AnonymousID: "alice-browser", SessionToken: "alice-token", CustomerName: strPtr("Alice"), CustomerEmail: strPtr("alice@example.com"), ExpiresAt: time.Now().Add(time.Hour)}
	if err := inbox.sessionRepo.Create(ctx, session); err != nil {
		t.Fatal(err)
	}
	deleted := make(chan error, 1)
	var started atomic.Bool
	name := "privacy_delete_during_creation"
	if err := db.Callback().Create().After("gorm:create").Register(name, func(tx *gorm.DB) {
		if tx.Statement.Table == "support_messages" && started.CompareAndSwap(false, true) {
			go func() { _, err := inbox.contactRepo.DeleteAnonymizingSupport(ctx, ws, contact); deleted <- err }()
			if err := waitForPrivacyLock(ctx, db); err != nil {
				tx.AddError(err)
			}
		}
	}); err != nil {
		t.Fatal(err)
	}
	defer db.Callback().Create().Remove(name)
	_, msg, err := inbox.createWidgetConversation(ctx, session, "Question", "Retain first message")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-deleted:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	privacyCheck(t, db, `SELECT anonymized_at IS NOT NULL AND customer_email IS NULL FROM support_conversations WHERE id=?`, msg.ConversationID)
	privacyCheck(t, db, `SELECT content='Retain first message' AND sender_display_name IS NULL FROM support_messages WHERE id=?`, msg.ID)
	privacyCheck(t, db, `SELECT count(*)=0 FROM crm_contacts WHERE workspace_id=?`, ws)
}
