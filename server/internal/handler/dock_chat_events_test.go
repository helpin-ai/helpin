package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
)

func TestDockEventListsSnapshotOption(t *testing.T) {
	for _, route := range []string{"chat", "run"} {
		t.Run(route, func(t *testing.T) {
			for _, tc := range []struct {
				name         string
				query        string
				wantSnapshot bool
				wantEvents   int
			}{
				{name: "default", wantSnapshot: true, wantEvents: 2},
				{name: "explicit inclusion", query: "include_snapshot=true", wantSnapshot: true, wantEvents: 2},
				{name: "opt out", query: "include_snapshot=false", wantEvents: 2},
				{name: "empty default", query: "after=2", wantSnapshot: true},
				{name: "empty opt out", query: "after=2&include_snapshot=false"},
			} {
				t.Run(tc.name, func(t *testing.T) {
					h, db := newDockEventsTestHandler(t)
					snapshotReads := 0
					if err := db.Callback().Query().Before("gorm:query").Register("count_snapshot_reads", func(tx *gorm.DB) {
						if tx.Statement.Table == "coding_session_state_snapshots" {
							snapshotReads++
						}
					}); err != nil {
						t.Fatal(err)
					}
					req := newDockEventsRequest(t, route, tc.query, "ws-1", "owner")
					rr := httptest.NewRecorder()
					serveDockEvents(t, h, route, rr, req)
					if rr.Code != http.StatusOK {
						t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
					}
					var response model.CodingSessionEventListResponse
					if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
						t.Fatal(err)
					}
					if got := response.StreamStateSnapshot != nil; got != tc.wantSnapshot {
						t.Errorf("snapshot included = %v, want %v", got, tc.wantSnapshot)
					}
					wantReads := 0
					if tc.wantSnapshot {
						wantReads = 1
					}
					if snapshotReads != wantReads {
						t.Errorf("snapshot reads = %d, want %d", snapshotReads, wantReads)
					}
					if len(response.Events) != tc.wantEvents || response.NextSequenceNo != 2 {
						t.Errorf("events = %d, cursor = %d, want %d/2", len(response.Events), response.NextSequenceNo, tc.wantEvents)
					}
				})
			}
		})
	}
}

func TestDockEventListsSnapshotOptOutPreservesAccess(t *testing.T) {
	for _, route := range []string{"chat", "run"} {
		t.Run(route, func(t *testing.T) {
			for _, tc := range []struct {
				name        string
				workspaceID string
				actorID     string
			}{
				{name: "other actor", workspaceID: "ws-1", actorID: "other"},
				{name: "other workspace", workspaceID: "ws-2", actorID: "owner"},
			} {
				t.Run(tc.name, func(t *testing.T) {
					h, db := newDockEventsTestHandler(t)
					if err := db.Callback().Query().Before("gorm:query").Register("reject_event_reads", func(tx *gorm.DB) {
						switch tx.Statement.Table {
						case "coding_session_state_snapshots", "agent_run_messages", "agent_run_artifacts":
							t.Errorf("unauthorized request read %s", tx.Statement.Table)
						}
					}); err != nil {
						t.Fatal(err)
					}
					req := newDockEventsRequest(t, route, "include_snapshot=false", tc.workspaceID, tc.actorID)
					rr := httptest.NewRecorder()
					serveDockEvents(t, h, route, rr, req)
					if rr.Code != http.StatusNotFound {
						t.Errorf("status = %d, want 404, body=%s", rr.Code, rr.Body.String())
					}
				})
			}
		})
	}
}

func newDockEventsTestHandler(t *testing.T) (*DockChatHandler, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:dock_events_%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE agent_runs (id TEXT PRIMARY KEY, workspace_id TEXT, agent_id TEXT, dock_chat_id TEXT,
			triggered_by_user_id TEXT, status TEXT, pause_reason TEXT, runtime_kind TEXT, approval_state TEXT,
			output_summary BLOB, created_at DATETIME, updated_at DATETIME)`,
		`CREATE TABLE dock_chats ( flow_builder TEXT,
 coverage_gap_id TEXT, initial_context TEXT,
execution_enabled boolean NOT NULL DEFAULT false,id TEXT PRIMARY KEY, workspace_id TEXT, user_id TEXT, visibility TEXT, active_run_id TEXT)`,
		`CREATE TABLE agent_run_messages (id TEXT PRIMARY KEY, workspace_id TEXT, run_id TEXT, role TEXT,
			content TEXT, sequence_no INTEGER, created_at DATETIME)`,
		`CREATE TABLE agent_run_artifacts (id TEXT PRIMARY KEY, workspace_id TEXT, run_id TEXT, sequence_no INTEGER, created_at DATETIME)`,
		`CREATE TABLE coding_session_state_snapshots (id TEXT PRIMARY KEY, workspace_id TEXT, run_id TEXT,
			schema_version TEXT, snapshot_payload BLOB)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create schema: %v", err)
		}
	}
	for _, runID := range []string{"chat-run", "run-1"} {
		if err := db.Exec(`INSERT INTO agent_runs (id, workspace_id, triggered_by_user_id, status, pause_reason,
			runtime_kind, output_summary, updated_at) VALUES (?, 'ws-1', 'owner', 'paused', 'user_message', 'native_sdk', ?, ?)`,
			runID, []byte(`{}`), time.Now().UTC()).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Exec(`INSERT INTO agent_run_messages VALUES (?, 'ws-1', ?, 'assistant', 'Hello', 1, ?)`,
			"message-"+runID, runID, time.Now().UTC().Add(-time.Minute)).Error; err != nil {
			t.Fatal(err)
		}
		payload, err := model.EncodeCodingSessionStreamSnapshot(&model.CodingSessionStreamSnapshot{
			LiveTurnSegments: []model.CodingSessionLiveTurnSegment{{SegmentID: "segment-1", Kind: "text"}},
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Exec(`INSERT INTO coding_session_state_snapshots VALUES (?, 'ws-1', ?, ?, ?)`,
			"snapshot-"+runID, runID, model.CodingSessionStateSnapshotSchemaVersionV1, []byte(payload)).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Exec(`INSERT INTO dock_chats (id, workspace_id, user_id, visibility, active_run_id) VALUES ('chat-1', 'ws-1', 'owner', 'private', 'chat-run')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`UPDATE agent_runs SET dock_chat_id = 'chat-1' WHERE id = 'chat-run'`).Error; err != nil {
		t.Fatal(err)
	}
	runRepo := repository.NewAgentRunRepository(db)
	messageRepo := repository.NewAgentRunMessageRepository(db)
	agentService := service.NewAgentService(nil, nil, runRepo, messageRepo,
		repository.NewAgentRunArtifactRepository(db), nil, repository.NewCodingSessionStateSnapshotRepository(db),
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	chatService := service.NewDockChatService(repository.NewDockChatRepository(db), runRepo, messageRepo, nil, agentService, nil, nil)
	return NewDockChatHandler(chatService, agentService), db
}

func newDockEventsRequest(t *testing.T, route, query, workspaceID, actorID string) *http.Request {
	t.Helper()
	path := "/api/dock/runs/run-1/events"
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("runID", "run-1")
	if route == "chat" {
		path = "/api/dock/chats/chat-1/run/events"
		routeCtx.URLParams.Add("chatID", "chat-1")
	}
	req := httptest.NewRequest(http.MethodGet, path+"?"+query, nil)
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx)
	ctx = middleware.WithWorkspaceID(ctx, workspaceID)
	ctx = middleware.WithUserID(ctx, actorID)
	return req.WithContext(ctx)
}

func serveDockEvents(t *testing.T, h *DockChatHandler, route string, rr *httptest.ResponseRecorder, req *http.Request) {
	t.Helper()
	if route == "chat" {
		h.ListChatRunEvents(rr, req)
		return
	}
	h.ListRunEvents(rr, req)
}
