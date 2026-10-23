//go:build integration

package dbmigrate

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRestoreHistoricalEmailParticipantsPostgres(t *testing.T) {
	dsn := os.Getenv("AI_PROFILES_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated PostgreSQL fixture required")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	schema := fmt.Sprintf("email_participant_repair_%d", time.Now().UnixNano())
	if _, err := db.ExecContext(ctx, "CREATE SCHEMA "+schema+"; SET search_path TO "+schema); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := db.ExecContext(ctx, "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Error(err)
		}
	}()
	_, err = db.ExecContext(ctx, `
CREATE TABLE support_conversations (id text PRIMARY KEY, workspace_id text, customer_email text, email_cc text[], email_thread_participants text[], anonymized_at timestamptz, updated_at timestamptz);
CREATE TABLE support_messages (id text PRIMARY KEY DEFAULT gen_random_uuid()::text, workspace_id text, conversation_id text, origin_conversation_id text, sender_type text, sender_user_id text, sender_display_name text, message_type text, system_event_type text, via_channel text, is_internal boolean, metadata jsonb, content text, created_at timestamptz DEFAULT now(), updated_at timestamptz DEFAULT now(), deleted_at timestamptz);
CREATE TABLE support_email_logs (workspace_id text, conversation_id text, message_ids text[], direction text, from_email text, to_email text, cc_emails text[], bcc_emails text[], recipient_address text);
CREATE TABLE workspace_members (workspace_id text, email text);
CREATE TABLE support_email_routes (workspace_id text, inbound_address text, source_address text);
CREATE TABLE support_email_senders (workspace_id text, email text, forwarding_address text);
INSERT INTO support_conversations VALUES ('c','w','buyer@example.com',ARRAY['existing@example.com'],ARRAY['removed@example.com'],NULL,now());
INSERT INTO support_conversations VALUES ('other','other-w','other@example.com',NULL,NULL,NULL,now());
INSERT INTO workspace_members VALUES ('w','staff@example.com');
INSERT INTO support_email_routes VALUES ('w','inbox@support.example.com','support@example.com');
INSERT INTO support_messages (id,workspace_id,conversation_id,sender_type,sender_user_id,sender_display_name,message_type,via_channel,is_internal,metadata,content,created_at) VALUES
 ('external','w','c','customer',NULL,'Alex','reply','email',true,'{"email_unknown_sender":true,"email_sender":"alex@example.com","email_ai_request":false}','Original reply','2026-09-01'),
 ('staff','w','c','customer',NULL,'Staff','reply','email',true,'{"email_unknown_sender":true,"email_sender":"staff@example.com"}','Private forward','2026-09-01'),
 ('note','w','c','user','staff-id','Staff','reply',NULL,true,'{}','Private note','2026-09-01'),
 ('deleted','w','c','customer',NULL,'Deleted','reply','email',true,'{"email_unknown_sender":true,"email_sender":"deleted@example.com"}','Deleted reply','2026-09-01'),
 ('unverified','w','c','customer',NULL,'Missing log','reply','email',true,'{"email_unknown_sender":true,"email_sender":"missing@example.com"}','No inbound log','2026-09-01');
UPDATE support_messages SET deleted_at=now() WHERE id='deleted';
INSERT INTO support_email_logs VALUES
 ('w','c',ARRAY['external'],'inbound','alex@example.com','Support <support@example.com>, Buyer <buyer@example.com>, Pat <pat@example.com>',ARRAY['existing@example.com','removed@example.com','copy@example.com'],ARRAY['hidden@example.com'],'inbox@support.example.com'),
 ('w','c',ARRAY['staff'],'inbound','staff@example.com','inbox@support.example.com',NULL,NULL,'inbox@support.example.com'),
 ('w','c',ARRAY['deleted'],'inbound','deleted@example.com','inbox@support.example.com',NULL,NULL,'inbox@support.example.com');`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `ALTER TABLE support_conversations ADD COLUMN merge_revision integer DEFAULT 0, ADD COLUMN merged_into_id text;`); err != nil {
		t.Fatal(err)
	}
	data, err := migrationFiles.ReadFile("sql/202610230004_restore_external_email_replies.sql")
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := db.ExecContext(ctx, string(data)); err != nil {
			t.Fatal(err)
		}
	}
	var internal, replayBlocked bool
	var content, created string
	if err := db.QueryRowContext(ctx, `SELECT is_internal,content,created_at::date::text, metadata->>'email_ai_suppression_reason'='historical_participant_repair' FROM support_messages WHERE id='external'`).Scan(&internal, &content, &created, &replayBlocked); err != nil {
		t.Fatal(err)
	}
	if internal || content != "Original reply" || created != "2026-09-01" || !replayBlocked {
		t.Fatalf("wrong repaired message: %v %q %s %v", internal, content, created, replayBlocked)
	}
	var cc string
	if err := db.QueryRowContext(ctx, `SELECT array_to_string(email_cc,',') FROM support_conversations WHERE id='c'`).Scan(&cc); err != nil {
		t.Fatal(err)
	}
	if cc != "alex@example.com,copy@example.com,existing@example.com,pat@example.com" {
		t.Fatalf("CC=%s", cc)
	}
	var preserved, events int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM support_messages WHERE id IN ('staff','note','deleted','unverified') AND is_internal`).Scan(&preserved); err != nil {
		t.Fatal(err)
	}
	if preserved != 4 {
		t.Fatalf("changed private/ineligible messages: %d", preserved)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM support_messages WHERE system_event_type='email_recipients_updated'`).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if events != 1 {
		t.Fatalf("events=%d", events)
	}
	var event string
	if err := db.QueryRowContext(ctx, `SELECT content FROM support_messages WHERE system_event_type='email_recipients_updated'`).Scan(&event); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(event, "Alex (alex@example.com) was added to CC.") || strings.Contains(event, "hidden@example.com") {
		t.Fatalf("incorrect event: %s", event)
	}
}
