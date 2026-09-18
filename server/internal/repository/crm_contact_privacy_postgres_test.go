//go:build integration

package repository

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/dbmigrate"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Use the actual ledger, including inbox projection and snapshot triggers.
// GORM-only fixtures do not exercise the production deletion transaction.
func contactPrivacyDB(t *testing.T) *gorm.DB {
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

func TestContactAnonymizationPostgres(t *testing.T) {
	db := contactPrivacyDB(t)
	ctx := context.Background()
	ws, otherWS, contact, other, conv, legacy, protected, foreign, session, message, note, hidden, activity := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	seedContactPrivacyWorkspace(t, db, otherWS)
	seedContactPrivacyWorkspace(t, db, ws)
	privacyExec(t, db, `INSERT INTO crm_contacts (id,workspace_id,display_id,first_name,email) VALUES (?,?,1,'Alice','alice@example.com'), (?,?,2,'Other',NULL)`, contact, ws, other, ws)
	privacyExec(t, db, `INSERT INTO crm_identity_links (workspace_id,anonymous_id,contact_id,external_user_id,identity_method,identity_trust) VALUES (?,'browser-alice',?,'alice-user','hmac','verified')`, ws, contact)
	privacyExec(t, db, `INSERT INTO support_conversations (id,workspace_id,display_id,subject,status,customer_name,customer_email,anonymous_id,crm_contact_id,created_at) VALUES (?,?,1,'Alice question','resolved','Alice','alice@example.com','browser-alice',?,'2026-01-01'), (?, ?,2,'Legacy','open','Alice','ALICE@example.com',NULL,NULL,'2026-01-01'), (?, ?,3,'Other contact','open','Other','alice@example.com',NULL,?,'2026-01-01'), (?, ?,1,'Other workspace','open','Other','alice@example.com','browser-alice',NULL,'2026-01-01')`, conv, ws, contact, legacy, ws, protected, ws, other, foreign, otherWS)
	privacyExec(t, db, `INSERT INTO support_widget_sessions (id,workspace_id,conversation_id,anonymous_id,session_token,customer_email,ip_address,expires_at) VALUES (?, ?, ?, 'browser-alice','old-token','alice@example.com','192.0.2.1',now()+interval '1 hour')`, session, ws, conv)
	privacyExec(t, db, `INSERT INTO support_messages (id,workspace_id,conversation_id,sender_type,sender_display_name,content,metadata,is_internal,deleted_at) VALUES (?, ?, ?, 'customer','Alice','I am Alice, alice@example.com','{"customer_email":"alice@example.com","delivery_to_email":"alice@example.com","rating":5}',false,NULL), (?, ?, ?, 'user','Teammate','Comment about Alice','{}',true,NULL), (?, ?, ?, 'customer','Alice','Hidden Alice comment','{}',false,now())`, message, ws, conv, note, ws, conv, hidden, ws, conv)
	privacyExec(t, db, `INSERT INTO support_email_logs (workspace_id,conversation_id,direction,from_email,to_email,raw_body,html_body) VALUES (?,?,'inbound','alice@example.com','support@example.com','My name is Alice','<p>My name is Alice</p>')`, ws, conv)
	privacyExec(t, db, `INSERT INTO support_email_webhook_events (workspace_id,conversation_id,event_type,raw_payload) VALUES (?,?,'inbound','{"From":"alice@example.com","FromFull":{"Name":"Alice","Email":"alice@example.com"},"TextBody":"My name is Alice"}')`, ws, conv)
	privacyExec(t, db, `INSERT INTO support_events (workspace_id,conversation_id,widget_session_id,anonymous_id,event_type,metadata,occurred_at) VALUES (?, ?, ?, 'browser-alice','csat','{"name":"Alice","rating":5}',now())`, ws, conv, session)
	privacyExec(t, db, `INSERT INTO crm_activities (id,workspace_id,contact_id,body,metadata,occurred_at) VALUES (?, ?, ?, 'Keep comment about Alice','{"old_email":"alice@example.com","comment":"Retained"}',now())`, activity, ws, contact)
	privacyExec(t, db, `INSERT INTO support_messages (workspace_id,conversation_id,sender_type,content,metadata) SELECT ?,?,'customer','Retained history','{"original_sender_email":"alice@example.com","forwarded_by_name":"Alice"}'::jsonb FROM generate_series(1,101)`, ws, conv)
	privacyExec(t, db, `INSERT INTO support_widget_sessions (workspace_id,conversation_id,anonymous_id,session_token,customer_email,expires_at) VALUES (?,?,'browser-alice','other-contact-token','other@example.com',now()+interval '1 hour')`, ws, protected)

	repo := NewCRMContactRepository(db)
	if ids, err := repo.DeleteAnonymizingSupport(ctx, otherWS, contact); err != nil || len(ids) != 0 {
		t.Fatalf("wrong workspace: %v %v", ids, err)
	}
	ids, err := repo.DeleteAnonymizingSupport(ctx, ws, contact)
	if err != nil || len(ids) != 2 {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			t.Logf("database context: %s; changes: %s", pgErr.Where, pgErr.Detail)
		}
		t.Fatalf("delete: %v %v", ids, err)
	}
	privacyCheck(t, db, `SELECT anonymized_at IS NOT NULL AND customer_email IS NULL AND customer_name='Deleted customer' AND crm_contact_id IS NULL AND anonymous_id IS NULL AND subject='Alice question' AND status='resolved' AND created_at='2026-01-01' FROM support_conversations WHERE id=?`, conv)
	privacyCheck(t, db, `SELECT view_search_document='alice question deleted customer' FROM support_conversations WHERE id=?`, conv)
	privacyCheck(t, db, `SELECT count(*) > 0 AND bool_and(NOT jsonb_exists(old_values,'customer_email') AND NOT jsonb_exists(old_values,'customer_name') AND NOT jsonb_exists(old_values,'view_search_document') AND NOT jsonb_exists(old_values,'search_vector') AND NOT jsonb_exists(new_values,'customer_email')) FROM support_inbox_conversation_changes WHERE conversation_id=?`, conv)

	privacyCheck(t, db, `SELECT content='I am Alice, alice@example.com' AND sender_display_name IS NULL AND metadata='{"rating":5}'::jsonb FROM support_messages WHERE id=?`, message)
	privacyCheck(t, db, `SELECT content='Comment about Alice' AND sender_display_name='Teammate' AND is_internal FROM support_messages WHERE id=?`, note)
	privacyCheck(t, db, `SELECT content='Hidden Alice comment' AND sender_display_name IS NULL AND deleted_at IS NOT NULL FROM support_messages WHERE id=?`, hidden)
	privacyCheck(t, db, `SELECT session_token <> 'old-token' AND revoked_at IS NOT NULL AND customer_email IS NULL AND ip_address IS NULL FROM support_widget_sessions WHERE id=?`, session)
	privacyCheck(t, db, `SELECT from_email='' AND to_email='' AND raw_body='My name is Alice' AND html_body='<p>My name is Alice</p>' FROM support_email_logs WHERE conversation_id=?`, conv)
	privacyCheck(t, db, `SELECT raw_payload='{"TextBody":"My name is Alice"}'::jsonb FROM support_email_webhook_events WHERE conversation_id=?`, conv)
	privacyCheck(t, db, `SELECT body='Keep comment about Alice' AND contact_id IS NULL AND metadata='{"comment":"Retained"}'::jsonb FROM crm_activities WHERE id=?`, activity)
	privacyCheck(t, db, `SELECT anonymous_id IS NULL AND metadata='{"rating":5}'::jsonb FROM support_events WHERE conversation_id=?`, conv)
	privacyCheck(t, db, `SELECT contact_id IS NULL AND external_user_id IS NULL AND anonymous_id <> 'browser-alice' FROM crm_identity_links WHERE workspace_id=?`, ws)
	privacyCheck(t, db, `SELECT count(*)=2 FROM support_conversations WHERE id IN (?, ?) AND anonymized_at IS NULL AND customer_email='alice@example.com'`, protected, foreign)
	privacyCheck(t, db, `SELECT count(*)=101 FROM support_messages WHERE conversation_id=? AND content='Retained history' AND metadata='{}'::jsonb`, conv)
	privacyCheck(t, db, `SELECT session_token='other-contact-token' AND revoked_at IS NULL AND customer_email='other@example.com' FROM support_widget_sessions WHERE conversation_id=?`, protected)

	if ids, err := repo.DeleteAnonymizingSupport(ctx, ws, contact); err != nil || len(ids) != 0 {
		t.Fatalf("retry: %v %v", ids, err)
	}
	if err := repo.Update(ctx, &model.CRMContact{ID: contact, WorkspaceID: ws, FirstName: "Alice"}); err == nil {
		t.Fatal("stale contact recreated")
	}
	privacyCheck(t, db, `SELECT count(*)=0 FROM crm_contacts WHERE id=?`, contact)
	for _, sql := range []string{`UPDATE support_conversations SET customer_email='restored@example.com' WHERE id=?`, `UPDATE support_conversations SET anonymized_at=NULL WHERE id=?`, `INSERT INTO support_messages (workspace_id,conversation_id,sender_type,content) SELECT workspace_id,id,'customer','late' FROM support_conversations WHERE id=?`} {
		if err := db.Exec(sql, conv).Error; err == nil {
			t.Fatalf("stale write accepted: %s", sql)
		}
	}
	if err := db.Exec(`UPDATE support_widget_sessions SET customer_email='restored@example.com' WHERE id=?`, session).Error; err == nil {
		t.Fatal("session restored")
	}
	privacyExec(t, db, `UPDATE support_conversations SET team_last_seen_at=now() WHERE id=?`, conv)
	if err := NewSupportConversationRepository(db).Delete(ctx, ws, conv); err != nil {
		t.Fatalf("delete anonymized conversation: %v", err)
	}
	privacyCheck(t, db, `SELECT count(*)=0 FROM support_conversations WHERE id=?`, conv)
}

func TestContactAnonymizationRollbackPostgres(t *testing.T) {
	db := contactPrivacyDB(t)
	ws, contact, conv := uuid.NewString(), uuid.NewString(), uuid.NewString()
	seedContactPrivacyWorkspace(t, db, ws)
	privacyExec(t, db, `INSERT INTO crm_contacts (id,workspace_id,display_id,first_name) VALUES (?,?,1,'Alice')`, contact, ws)
	privacyExec(t, db, `INSERT INTO support_conversations (id,workspace_id,display_id,subject,crm_contact_id,customer_name) VALUES (?, ?, 1,'Keep',?,'Alice')`, conv, ws, contact)
	privacyExec(t, db, `INSERT INTO support_widget_sessions (workspace_id,conversation_id,anonymous_id,session_token,expires_at) VALUES (?,?,'alice','still-valid',now()+interval '1 hour')`, ws, conv)
	privacyExec(t, db, `CREATE FUNCTION reject_contact_delete() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected delete failure'; END; $$; CREATE TRIGGER reject_delete BEFORE DELETE ON crm_contacts FOR EACH ROW EXECUTE FUNCTION reject_contact_delete()`)
	if _, err := NewCRMContactRepository(db).DeleteAnonymizingSupport(context.Background(), ws, contact); err == nil {
		t.Fatal("expected rollback")
	}
	privacyCheck(t, db, `SELECT anonymized_at IS NULL AND customer_name='Alice' FROM support_conversations WHERE id=?`, conv)
	privacyCheck(t, db, `SELECT session_token='still-valid' AND revoked_at IS NULL FROM support_widget_sessions WHERE conversation_id=?`, conv)
}

func TestContactAnonymizationSharedIdentityPostgres(t *testing.T) {
	db := contactPrivacyDB(t)
	ws, deleted, other, explicit, legacy := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	seedContactPrivacyWorkspace(t, db, ws)
	privacyExec(t, db, `INSERT INTO crm_contacts (id,workspace_id,display_id,first_name,email) VALUES (?, ?, 'CON-1','Deleted','shared@example.com'), (?, ?, 'CON-2','Other','SHARED@example.com')`, deleted, ws, other, ws)
	privacyExec(t, db, `INSERT INTO crm_identity_links (workspace_id,contact_id,anonymous_id,identity_method,identity_trust) VALUES (?,?,'shared-browser','browser_claim','untrusted'), (?,?,'shared-browser','browser_claim','untrusted')`, ws, deleted, ws, other)
	privacyExec(t, db, `INSERT INTO support_conversations (id,workspace_id,display_id,subject,crm_contact_id,customer_email,anonymous_id) VALUES (?, ?,1,'Explicit',?,'shared@example.com','shared-browser'), (?, ?,2,'Unattributed',NULL,'shared@example.com','shared-browser')`, explicit, ws, deleted, legacy, ws)
	privacyExec(t, db, `INSERT INTO support_widget_sessions (workspace_id,anonymous_id,session_token,customer_email,expires_at) VALUES (?,'shared-browser','unattributed-token','shared@example.com',now()+interval '1 hour')`, ws)
	ids, err := NewCRMContactRepository(db).DeleteAnonymizingSupport(context.Background(), ws, deleted)
	if err != nil || len(ids) != 1 || ids[0] != explicit {
		t.Fatalf("ambiguous histories attributed: %v %v", ids, err)
	}
	privacyCheck(t, db, `SELECT anonymized_at IS NULL AND customer_email='shared@example.com' FROM support_conversations WHERE id=?`, legacy)
	privacyCheck(t, db, `SELECT revoked_at IS NULL AND customer_email='shared@example.com' FROM support_widget_sessions WHERE session_token='unattributed-token'`)
}

func TestContactAnonymizationWaitsForInFlightMessagePostgres(t *testing.T) {
	db := contactPrivacyDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ws, contact, conv, message := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	seedContactPrivacyWorkspace(t, db, ws)
	privacyExec(t, db, `INSERT INTO crm_contacts (id,workspace_id,display_id,first_name) VALUES (?,?,'CON-1','Alice')`, contact, ws)
	privacyExec(t, db, `INSERT INTO support_conversations (id,workspace_id,display_id,subject,crm_contact_id) VALUES (?, ?,1,'Question',?)`, conv, ws, contact)
	writer := db.WithContext(ctx).Begin()
	if writer.Error != nil {
		t.Fatal(writer.Error)
	}
	defer writer.Rollback()
	privacyExec(t, writer, `INSERT INTO support_messages (id,workspace_id,conversation_id,sender_type,sender_display_name,content,metadata,created_at) VALUES (?, ?, ?,'customer','Alice','Retain in-flight text','{"customer_email":"alice@example.com"}',now())`, message, ws, conv)
	deletion := db.WithContext(ctx).Begin()
	if deletion.Error != nil {
		t.Fatal(deletion.Error)
	}
	defer deletion.Rollback()
	var pid int
	if err := deletion.Raw("SELECT pg_backend_pid()").Scan(&pid).Error; err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	var group sync.WaitGroup
	group.Add(1)
	go func() {
		defer group.Done()
		_, err := NewCRMContactRepository(deletion).DeleteAnonymizingSupport(ctx, ws, contact)
		if err == nil {
			err = deletion.Commit().Error
		}
		done <- err
	}()
	defer func() { cancel(); writer.Rollback(); group.Wait() }()
	// Observe actual database blocking; a sleep alone would not prove the race.
	for {
		var waiting bool
		if err := db.WithContext(ctx).Raw("SELECT EXISTS(SELECT 1 FROM pg_locks WHERE pid=? AND NOT granted)", pid).Scan(&waiting).Error; err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		select {
		case err := <-done:
			t.Fatalf("deletion did not wait for writer: %v", err)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(10 * time.Millisecond):
		}
	}
	if err := writer.Commit().Error; err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	privacyCheck(t, db, `SELECT content='Retain in-flight text' AND sender_display_name IS NULL AND metadata='{}'::jsonb FROM support_messages WHERE id=?`, message)
	privacyCheck(t, db, `SELECT anonymized_at IS NOT NULL FROM support_conversations WHERE id=?`, conv)
}
