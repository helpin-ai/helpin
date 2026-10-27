//go:build integration

package dbmigrate

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"
)

// Retiring customer-facing portal AI converts settings and queued work and
// hands waiting requests to the team, and is safe to run twice.
func TestRetirePortalCustomerAIPostgres(t *testing.T) {
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
	schema := fmt.Sprintf("portal_human_owned_%d", time.Now().UnixNano())
	if _, err := db.ExecContext(ctx, "CREATE SCHEMA "+schema+"; SET search_path TO "+schema); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := db.ExecContext(ctx, "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Error(err)
		}
	}()
	if _, err := db.ExecContext(ctx, `
CREATE TABLE support_widget_installations (id text PRIMARY KEY, settings jsonb);
CREATE TABLE support_portal_ai_dispatches (source_message_id text PRIMARY KEY, mode_at_enqueue text NOT NULL, status text NOT NULL, updated_at timestamptz,
    CONSTRAINT chk_support_portal_ai_dispatch_mode CHECK (mode_at_enqueue IN ('internal_note', 'ai_first')));
CREATE TABLE support_conversations (id text PRIMARY KEY, channel text, status text, flow_state text, assigned_user_id text, opened_by_user_id text,
    human_takeover boolean, assigned_agent_id text, ai_paused_at timestamptz, needs_human_reply boolean, updated_at timestamptz);
INSERT INTO support_widget_installations VALUES
    ('customer-replies', '{"portal_ai_mode":"ai_first","ai_response_mode":"ai_first"}'),
    ('suggestions', '{"portal_ai_mode":"internal_note"}'),
    ('off', '{"portal_ai_mode":"off"}');
INSERT INTO support_portal_ai_dispatches VALUES
    ('pending', 'ai_first', 'pending', now()), ('done', 'ai_first', 'published', now()), ('note', 'internal_note', 'pending', now());
INSERT INTO support_conversations VALUES
    ('waiting', 'portal', 'open', 'ai_handling', NULL, NULL, false, 'agent', NULL, false, now()),
    ('owned', 'portal', 'open', 'ai_handling', 'teammate', NULL, false, 'agent', NULL, false, now()),
    ('resolved', 'portal', 'resolved', 'ai_handling', NULL, NULL, false, 'agent', NULL, false, now()),
    ('chat', 'widget', 'open', 'ai_handling', NULL, NULL, false, 'agent', NULL, false, now());`); err != nil {
		t.Fatal(err)
	}
	data, err := migrationFiles.ReadFile("sql/202610270001_support_portal_human_owned.sql")
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := db.ExecContext(ctx, string(data)); err != nil {
			t.Fatal(err)
		}
	}
	modes := map[string]string{}
	rows, err := db.QueryContext(ctx, `SELECT id, settings->>'portal_ai_mode' || '/' || COALESCE(settings->>'ai_response_mode', '') FROM support_widget_installations`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id, mode string
		if err := rows.Scan(&id, &mode); err != nil {
			t.Fatal(err)
		}
		modes[id] = mode
	}
	rows.Close()
	if modes["customer-replies"] != "internal_note/ai_first" || modes["suggestions"] != "internal_note/" || modes["off"] != "off/" {
		t.Fatalf("settings = %v (chat AI must be untouched)", modes)
	}
	var pending, published string
	if err := db.QueryRowContext(ctx, `SELECT (SELECT mode_at_enqueue FROM support_portal_ai_dispatches WHERE source_message_id='pending'), (SELECT mode_at_enqueue FROM support_portal_ai_dispatches WHERE source_message_id='done')`).Scan(&pending, &published); err != nil {
		t.Fatal(err)
	}
	if pending != "internal_note" || published != "ai_first" {
		t.Fatalf("dispatch modes pending=%s published=%s", pending, published)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO support_portal_ai_dispatches VALUES ('new', 'ai_first', 'pending', now())`); err == nil {
		t.Fatal("a new customer-facing portal dispatch was accepted")
	}
	states := map[string]string{}
	rows, err = db.QueryContext(ctx, `SELECT id, flow_state || '/' || human_takeover::text || '/' || COALESCE(assigned_agent_id, '-') FROM support_conversations`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id, state string
		if err := rows.Scan(&id, &state); err != nil {
			t.Fatal(err)
		}
		states[id] = state
	}
	rows.Close()
	want := map[string]string{
		"waiting":  "waiting_for_human/true/-",
		"owned":    "assigned_to_human/true/-",
		"resolved": "ai_handling/false/agent",
		"chat":     "ai_handling/false/agent",
	}
	for id, state := range want {
		if states[id] != state {
			t.Errorf("%s = %s, want %s", id, states[id], state)
		}
	}
}
