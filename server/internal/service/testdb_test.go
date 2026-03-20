package service

import (
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// newTestDB creates an in-memory SQLite database with all core tables needed
// for service-level testing. Each call returns a fresh, isolated DB.
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:testdb-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	tables := []string{
		`CREATE TABLE users (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			email TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			full_name TEXT NOT NULL,
			avatar_url TEXT,
			default_workspace_id TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE organizations (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			name TEXT NOT NULL,
			slug TEXT NOT NULL UNIQUE,
			owner_id TEXT NOT NULL,
			logo_url TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE organization_members (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			organization_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'member',
			created_at DATETIME,
			updated_at DATETIME,
			UNIQUE(organization_id, user_id)
		)`,
		`CREATE TABLE workspaces (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			name TEXT NOT NULL,
			slug TEXT NOT NULL UNIQUE,
			owner_id TEXT NOT NULL,
			organization_id TEXT,
			description TEXT,
			logo_url TEXT,
			timezone TEXT NOT NULL DEFAULT 'UTC',
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE workspace_members (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL,
			user_id TEXT,
			email TEXT NOT NULL,
			display_name TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'member',
			status TEXT NOT NULL DEFAULT 'active',
			invited_by TEXT,
			invited_at DATETIME,
			accepted_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE workspace_teams (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL,
			name TEXT NOT NULL,
			handle TEXT,
			description TEXT,
			manager_id TEXT,
			team_type TEXT NOT NULL DEFAULT 'engineering',
			default_story_type TEXT NOT NULL DEFAULT 'feature',
			docs_publisher_enabled BOOLEAN NOT NULL DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE team_workspace_memberships (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			team_id TEXT NOT NULL,
			workspace_member_id TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'member',
			created_at DATETIME,
			updated_at DATETIME,
			UNIQUE(team_id, workspace_member_id)
		)`,
		`CREATE TABLE workspace_settings (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL UNIQUE,
			quarter_start_date TEXT,
			sprint_duration_weeks INTEGER NOT NULL DEFAULT 2,
			notifications_enabled BOOLEAN NOT NULL DEFAULT 1,
			auto_calculate_bonuses BOOLEAN NOT NULL DEFAULT 0,
			team_weight INTEGER NOT NULL DEFAULT 50,
			planning_methodology TEXT NOT NULL DEFAULT 'structured_v1',
			planning_web_search_enabled BOOLEAN NOT NULL DEFAULT 0,
			planning_web_search_provider TEXT NOT NULL DEFAULT 'brave',
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE workspace_invitations (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL,
			workspace_member_id TEXT,
			email TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'member',
			token TEXT NOT NULL UNIQUE,
			invited_by TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			expires_at DATETIME NOT NULL,
			accepted_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE invitation_team_preassignments (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			invitation_id TEXT NOT NULL,
			team_id TEXT NOT NULL,
			created_at DATETIME
		)`,
		`CREATE TABLE pm_workflows (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			team_id TEXT,
			default_state_id TEXT,
			auto_assign_owner BOOLEAN NOT NULL DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_workflow_states (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workflow_id TEXT NOT NULL,
			name TEXT NOT NULL,
			state_type TEXT NOT NULL,
			position INTEGER NOT NULL DEFAULT 0,
			color TEXT,
			description TEXT,
			w_ip_limit INTEGER,
			is_default BOOLEAN NOT NULL DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_epic_workflow_states (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL,
			name TEXT NOT NULL,
			state_type TEXT NOT NULL,
			position INTEGER NOT NULL DEFAULT 0,
			color TEXT,
			is_default BOOLEAN NOT NULL DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_labels (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL,
			team_id TEXT,
			name TEXT NOT NULL,
			description TEXT,
			color TEXT,
			archived BOOLEAN NOT NULL DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_epics (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			external_id TEXT,
			epic_state_id TEXT,
			owner_id TEXT,
			owner_member_id TEXT,
			team_id TEXT,
			planned_start_date TEXT,
			deadline TEXT,
			started BOOLEAN NOT NULL DEFAULT 0,
			started_at DATETIME,
			completed BOOLEAN NOT NULL DEFAULT 0,
			completed_at DATETIME,
			position INTEGER NOT NULL DEFAULT 0,
			color TEXT,
			health TEXT NOT NULL DEFAULT 'no_health',
			health_comment TEXT,
			archived BOOLEAN NOT NULL DEFAULT 0,
			orchestrator_agent_id TEXT,
			spec_document_id TEXT,
			planning_repository_id TEXT,
			planning_state TEXT NOT NULL DEFAULT 'not_started',
			spec_clarifications BLOB NOT NULL DEFAULT (CAST('[]' AS BLOB)),
			spec_clarified_at DATETIME,
			spec_clarified_by TEXT,
			approved_spec_version_id TEXT,
			active_planning_session_id TEXT,
			active_flow_run_id TEXT,
			last_planning_run_id TEXT,
			created_by TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_epic_labels (
			epic_id TEXT NOT NULL,
			label_id TEXT NOT NULL,
			created_at DATETIME,
			PRIMARY KEY (epic_id, label_id)
		)`,
		`CREATE TABLE pm_epic_objectives (
			epic_id TEXT NOT NULL,
			objective_id TEXT NOT NULL,
			created_at DATETIME,
			PRIMARY KEY (epic_id, objective_id)
		)`,
		`CREATE TABLE pm_sprints (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			external_id TEXT,
			start_date DATETIME,
			end_date DATETIME,
			team_id TEXT,
			archived BOOLEAN NOT NULL DEFAULT 0,
			created_by TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_sprint_labels (
			sprint_id TEXT NOT NULL,
			label_id TEXT NOT NULL,
			created_at DATETIME,
			PRIMARY KEY (sprint_id, label_id)
		)`,
		`CREATE TABLE pm_stories (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL,
			display_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			story_type TEXT NOT NULL DEFAULT 'feature',
			workflow_id TEXT NOT NULL,
			workflow_state_id TEXT NOT NULL,
			epic_id TEXT,
			sprint_id TEXT,
			team_id TEXT,
			owner_id TEXT,
			owner_member_id TEXT,
			requester_id TEXT,
			requester_member_id TEXT,
			estimate INTEGER,
			priority TEXT NOT NULL DEFAULT 'none',
			severity TEXT NOT NULL DEFAULT 'none',
			deadline TEXT,
			position INTEGER NOT NULL DEFAULT 0,
			started BOOLEAN NOT NULL DEFAULT 0,
			started_at DATETIME,
			completed BOOLEAN NOT NULL DEFAULT 0,
			completed_at DATETIME,
			moved_at DATETIME,
			blocked BOOLEAN NOT NULL DEFAULT 0,
			blocker TEXT,
			archived BOOLEAN NOT NULL DEFAULT 0,
			assigned_agent_id TEXT,
			template_id TEXT,
			external_id TEXT,
			slice_type TEXT,
			implementation_brief TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_story_owners (
			story_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			created_at DATETIME,
			PRIMARY KEY (story_id, user_id)
		)`,
		`CREATE TABLE pm_story_labels (
			story_id TEXT NOT NULL,
			label_id TEXT NOT NULL,
			created_at DATETIME,
			PRIMARY KEY (story_id, label_id)
		)`,
		`CREATE TABLE pm_story_followers (
			story_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			created_at DATETIME,
			PRIMARY KEY (story_id, user_id)
		)`,
		`CREATE TABLE pm_story_links (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL,
			source_story_id TEXT NOT NULL,
			target_story_id TEXT NOT NULL,
			link_type TEXT NOT NULL,
			created_by TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_activity_log (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL,
			entity_type TEXT NOT NULL,
			entity_id TEXT NOT NULL,
			actor_id TEXT,
			action TEXT NOT NULL,
			field_name TEXT,
			old_value TEXT,
			new_value TEXT,
			metadata TEXT,
			created_at DATETIME
		)`,
		`CREATE TABLE pm_comments (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT,
			entity_type TEXT NOT NULL,
			entity_id TEXT NOT NULL,
			author_id TEXT NOT NULL,
			body TEXT NOT NULL,
			parent_id TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_comment_reactions (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			comment_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			emoji TEXT NOT NULL,
			created_at DATETIME
		)`,
		`CREATE TABLE pm_attachments (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL,
			entity_type TEXT NOT NULL DEFAULT 'story',
			entity_id TEXT NOT NULL,
			file_name TEXT NOT NULL,
			file_size INTEGER NOT NULL DEFAULT 0,
			content_type TEXT NOT NULL DEFAULT '',
			storage_key TEXT NOT NULL,
			url TEXT,
			is_uploaded BOOLEAN NOT NULL DEFAULT 0,
			uploaded_by_id TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_objectives (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			external_id TEXT,
			objective_type TEXT NOT NULL DEFAULT 'tactical',
			state TEXT NOT NULL DEFAULT 'not_started',
			planned_start_date TEXT,
			deadline TEXT,
			health TEXT NOT NULL DEFAULT 'no_health',
			health_comment TEXT,
			position INTEGER NOT NULL DEFAULT 0,
			archived BOOLEAN NOT NULL DEFAULT 0,
			created_by TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_key_results (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			objective_id TEXT NOT NULL,
			name TEXT NOT NULL,
			result_type TEXT NOT NULL DEFAULT 'boolean',
			initial_value REAL NOT NULL DEFAULT 0,
			current_value REAL NOT NULL DEFAULT 0,
			target_value REAL NOT NULL DEFAULT 100,
			progress REAL NOT NULL DEFAULT 0,
			note TEXT,
			note_updated_by TEXT,
			note_updated_at DATETIME,
			position INTEGER NOT NULL DEFAULT 0,
			updated_by TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_objective_teams (
			objective_id TEXT NOT NULL,
			team_id TEXT NOT NULL,
			created_at DATETIME,
			PRIMARY KEY (objective_id, team_id)
		)`,
		`CREATE TABLE pm_objective_owners (
			objective_id TEXT NOT NULL,
			workspace_member_id TEXT NOT NULL,
			created_at DATETIME,
			PRIMARY KEY (objective_id, workspace_member_id)
		)`,
		`CREATE TABLE pm_objective_labels (
			objective_id TEXT NOT NULL,
			label_id TEXT NOT NULL,
			created_at DATETIME,
			PRIMARY KEY (objective_id, label_id)
		)`,
		`CREATE TABLE pm_checklist_items (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			story_id TEXT NOT NULL,
			text TEXT NOT NULL,
			completed BOOLEAN NOT NULL DEFAULT 0,
			position INTEGER NOT NULL DEFAULT 0,
			assignee_id TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_external_links (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			story_id TEXT NOT NULL,
			url TEXT NOT NULL,
			title TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_views (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			view_type TEXT NOT NULL DEFAULT 'board',
			filters TEXT,
			sort_by TEXT,
			group_by TEXT,
			columns TEXT,
			is_default BOOLEAN NOT NULL DEFAULT 0,
			team_id TEXT,
			created_by TEXT,
			shared BOOLEAN NOT NULL DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_automations (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL,
			automation_type TEXT NOT NULL,
			enabled BOOLEAN NOT NULL DEFAULT 0,
			team_id TEXT,
			config_state_id TEXT,
			config_int INTEGER,
			config_int2 INTEGER,
			config_int3 INTEGER,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		// Notification tables (reuse from notification tests)
		`CREATE TABLE notifications (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL,
			recipient_id TEXT NOT NULL,
			actor_id TEXT,
			entity_type TEXT NOT NULL,
			entity_id TEXT NOT NULL,
			event_type TEXT NOT NULL,
			title TEXT NOT NULL,
			body TEXT,
			metadata TEXT,
			latest_event_category TEXT NOT NULL,
			actor_snapshot TEXT,
			entity_snapshot TEXT,
			parent_entity_snapshot TEXT,
			event_count INTEGER NOT NULL,
			last_event_at DATETIME,
			status TEXT NOT NULL,
			snoozed_until DATETIME,
			read_at DATETIME,
			archived_at DATETIME,
			priority TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME,
			UNIQUE(recipient_id, entity_type, entity_id, workspace_id)
		)`,
		`CREATE TABLE notification_events (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			notification_id TEXT NOT NULL,
			actor_id TEXT,
			event_type TEXT NOT NULL,
			title TEXT NOT NULL,
			metadata TEXT,
			category TEXT NOT NULL,
			actor_snapshot TEXT,
			priority TEXT NOT NULL,
			created_at DATETIME
		)`,
		`CREATE TABLE notification_deliveries (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			notification_event_id TEXT NOT NULL,
			channel TEXT NOT NULL,
			status TEXT NOT NULL,
			delivered_at DATETIME,
			error TEXT,
			external_message_id TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE notification_preferences (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			user_id TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			team_id TEXT,
			mute_workspace BOOLEAN NOT NULL DEFAULT 0,
			do_not_disturb BOOLEAN NOT NULL DEFAULT 0,
			dnd_until DATETIME,
			email_enabled BOOLEAN NOT NULL DEFAULT 1,
			email_digest_frequency TEXT,
			email_digest_time TEXT,
			email_digest_day INTEGER,
			timezone TEXT,
			channel_preferences TEXT,
			badge_mode TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE user_notification_settings (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			user_id TEXT NOT NULL,
			email_enabled BOOLEAN NOT NULL DEFAULT 1,
			email_digest_frequency TEXT NOT NULL DEFAULT 'none',
			email_digest_time TEXT NOT NULL DEFAULT '09:00',
			email_digest_day INTEGER NOT NULL DEFAULT 1,
			do_not_disturb BOOLEAN NOT NULL DEFAULT 0,
			dnd_until DATETIME,
			badge_mode TEXT NOT NULL DEFAULT 'all',
			timezone TEXT NOT NULL DEFAULT 'UTC',
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE entity_followers (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			user_id TEXT NOT NULL,
			entity_type TEXT NOT NULL,
			entity_id TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			reason TEXT,
			created_at DATETIME
		)`,
		`CREATE TABLE git_repositories (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL,
			git_integration_id TEXT NOT NULL,
			repo_full_name TEXT NOT NULL,
			repo_name TEXT NOT NULL,
			repo_owner TEXT NOT NULL,
			default_branch TEXT NOT NULL DEFAULT 'main',
			is_active BOOLEAN NOT NULL DEFAULT 1,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE support_conversations (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL,
			display_id INTEGER NOT NULL,
			subject TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'open',
			priority TEXT NOT NULL DEFAULT 'medium',
			channel TEXT NOT NULL DEFAULT 'widget',
			customer_name TEXT,
			customer_email TEXT,
			customer_phone TEXT,
			opened_by_user_id TEXT,
			assigned_agent_id TEXT,
			linked_story_id TEXT,
			source TEXT NOT NULL DEFAULT 'internal',
			anonymous_id TEXT,
			crm_contact_id TEXT,
			resolved_at DATETIME,
			closed_at DATETIME,
			team_last_seen_at DATETIME,
			contact_last_seen_at DATETIME,
			ai_state TEXT,
			ai_resolved_at DATETIME,
			ai_escalated_at DATETIME,
			ai_resolution_type TEXT,
			ai_turn_count INTEGER NOT NULL DEFAULT 0,
			customer_requested_human_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE support_messages (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL,
			conversation_id TEXT NOT NULL,
			ticket_id TEXT,
			sender_type TEXT NOT NULL,
			message_type TEXT NOT NULL DEFAULT 'reply',
			sender_user_id TEXT,
			sender_agent_id TEXT,
			sender_display_name TEXT,
			sender_avatar_url TEXT,
			content TEXT NOT NULL,
			is_internal BOOLEAN NOT NULL DEFAULT 0,
			metadata TEXT DEFAULT '{}',
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE support_canned_responses (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL,
			short_code TEXT NOT NULL,
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			created_by_id TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE support_widget_installations (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL,
			widget_key TEXT NOT NULL,
			secret_key TEXT NOT NULL,
			settings TEXT NOT NULL DEFAULT '{}',
			active BOOLEAN NOT NULL DEFAULT 1,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE support_widget_sessions (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL,
			conversation_id TEXT,
			ticket_id TEXT,
			session_token TEXT NOT NULL,
			anonymous_id TEXT NOT NULL DEFAULT '',
			is_anonymous BOOLEAN NOT NULL DEFAULT 1,
			customer_name TEXT,
			customer_email TEXT,
			user_agent TEXT,
			last_page_url TEXT,
			revoked_at DATETIME,
			expires_at DATETIME NOT NULL,
			created_at DATETIME
		)`,
	}

	for _, stmt := range tables {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create table: %v\nSQL: %s", err, stmt[:80])
		}
	}

	return db
}

// seedUser inserts a user with a pre-hashed password into the test DB.
func seedUser(t *testing.T, db *gorm.DB, id, email, fullName, passwordHash string) {
	t.Helper()
	now := time.Now()
	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		id, email, passwordHash, fullName, now, now)
}

// seedWorkspace inserts a workspace into the test DB.
func seedWorkspace(t *testing.T, db *gorm.DB, id, name, slug, ownerID string) {
	t.Helper()
	now := time.Now()
	mustExec(t, db, `INSERT INTO workspaces (id, name, slug, owner_id, timezone, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, name, slug, ownerID, "UTC", now, now)
}

// seedWorkspaceMember inserts a workspace member into the test DB.
func seedWorkspaceMember(t *testing.T, db *gorm.DB, id, wsID, userID, email, displayName, role string) {
	t.Helper()
	now := time.Now()
	mustExec(t, db, `INSERT INTO workspace_members (id, workspace_id, user_id, email, display_name, role, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, wsID, userID, email, displayName, role, "active", now, now)
}

// seedWorkflow inserts a workflow with a default state into the test DB.
func seedWorkflow(t *testing.T, db *gorm.DB, workflowID, wsID, defaultStateID string) {
	t.Helper()
	now := time.Now()
	mustExec(t, db, `INSERT INTO pm_workflows (id, workspace_id, name, default_state_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		workflowID, wsID, "Default Workflow", defaultStateID, now, now)
	mustExec(t, db, `INSERT INTO pm_workflow_states (id, workflow_id, name, state_type, position, is_default, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		defaultStateID, workflowID, "To Do", "unstarted", 0, true, now, now)
}

// seedSettings inserts workspace settings into the test DB.
func seedSettings(t *testing.T, db *gorm.DB, id, wsID string) {
	t.Helper()
	now := time.Now()
	mustExec(t, db, `INSERT INTO workspace_settings (id, workspace_id, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		id, wsID, now, now)
}

// mustExec runs a SQL statement and fails the test on error.
func mustExec(t *testing.T, db *gorm.DB, query string, args ...any) {
	t.Helper()
	if err := db.Exec(query, args...).Error; err != nil {
		t.Fatalf("exec %q: %v", query[:min(len(query), 60)], err)
	}
}

// seedTemporaryAttachment inserts a temporary (unconfirmed) attachment into the test DB.
func seedTemporaryAttachment(t *testing.T, db *gorm.DB, id, workspaceID, entityID, uploadedByID string) {
	t.Helper()
	now := time.Now()
	mustExec(t, db, `INSERT INTO pm_attachments (id, workspace_id, entity_type, entity_id, file_name, file_size, content_type, storage_key, is_uploaded, uploaded_by_id, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, workspaceID, "temporary", entityID, "test-file.png", 1024, "image/png", "uploads/"+id+".png", true, uploadedByID, now)
}

// min is provided by the builtin (Go 1.21+) or pm_import.go
