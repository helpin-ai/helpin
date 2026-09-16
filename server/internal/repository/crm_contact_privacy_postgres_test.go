//go:build integration

package repository

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func contactPrivacyDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("CONTACT_PRIVACY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set CONTACT_PRIVACY_TEST_DATABASE_URL to disposable PostgreSQL")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	schema := fmt.Sprintf("contact_privacy_%d", time.Now().UnixNano())
	privacyExec(t, db, "CREATE SCHEMA "+schema)
	t.Cleanup(func() {
		if err := db.Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Error(err)
		}
		if err := sqlDB.Close(); err != nil {
			t.Error(err)
		}
	})
	privacyExec(t, db, "SET search_path TO "+schema)
	if err := db.AutoMigrate(&model.CRMContact{}, &model.CRMIdentityLink{}, &model.CRMActivity{}, &model.SupportConversation{}, &model.SupportMessage{}, &model.SupportWidgetSession{}, &model.SupportEmailLog{}, &model.SupportEmailWebhookEvent{}, &model.SupportEvent{}, &model.SupportConversationTriage{}, &model.SupportConversationTriageEvent{}, &model.SupportAIFollowUp{}, &model.SupportAttachment{}); err != nil {
		t.Fatal(err)
	}
	privacyExec(t, db, "ALTER TABLE support_conversations DROP COLUMN anonymized_at")
	_, file, _, _ := runtime.Caller(0)
	migration, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../dbmigrate/sql/202609160003_support_contact_anonymization.sql"))
	if err != nil {
		t.Fatal(err)
	}
	privacyExec(t, db, string(migration))
	privacyExec(t, db, string(migration))
	return db
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
		t.Fatalf("delete: %v %v", ids, err)
	}
	privacyCheck(t, db, `SELECT anonymized_at IS NOT NULL AND customer_email IS NULL AND customer_name='Deleted customer' AND crm_contact_id IS NULL AND anonymous_id IS NULL AND subject='Alice question' AND status='resolved' AND created_at='2026-01-01' FROM support_conversations WHERE id=?`, conv)
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
}

func TestContactAnonymizationRollbackPostgres(t *testing.T) {
	db := contactPrivacyDB(t)
	ws, contact, conv := uuid.NewString(), uuid.NewString(), uuid.NewString()
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
