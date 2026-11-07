package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	sdk "github.com/helpin-ai/agent-runtime-go"
	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/service"
)

func TestChatSteeringPinsRunAndChecksOwnership(t *testing.T) {
	for _, tc := range []struct {
		name, workspace, actor, run string
		status                      int
	}{
		{"owner", "ws-1", "owner", "chat-run", 202},
		{"foreign workspace", "ws-2", "owner", "chat-run", 404},
		{"shared reader", "ws-1", "other", "chat-run", 404},
		{"stale run", "ws-1", "owner", "old-run", 409},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, db := newDockEventsTestHandler(t)
			for _, sql := range []string{
				`ALTER TABLE agent_runs ADD COLUMN external_runtime TEXT`,
				`ALTER TABLE agent_runs ADD COLUMN external_runtime_id TEXT`,
				`UPDATE agent_runs SET external_runtime='agent-runtime', external_runtime_id='runtime-chat', status='running', pause_reason='none' WHERE id='chat-run'`,
				`UPDATE dock_chats SET visibility='workspace' WHERE id='chat-1'`,
			} {
				if err := db.Exec(sql).Error; err != nil {
					t.Fatal(err)
				}
			}
			calls := 0
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Path != "/v1/runs/runtime-chat/steering" {
					t.Errorf("wrong runtime run: %s", r.URL.Path)
				}
				var command sdk.SteerRunRequest
				if err := json.NewDecoder(r.Body).Decode(&command); err != nil {
					t.Error(err)
				}
				if command.ExternalActorID != "owner" || command.CommandID != "command-1" {
					t.Errorf("wrong attribution: %+v", command)
				}
				if err := json.NewEncoder(w).Encode(sdk.RunSteeringCommand{SteerRunRequest: command, Revision: 1, State: "accepted"}); err != nil {
					t.Error(err)
				}
			}))
			defer upstream.Close()
			client, err := service.NewAgentRuntimeClient(upstream.URL, "helpin", "token", upstream.Client())
			if err != nil {
				t.Fatal(err)
			}
			h.agentService.SetAgentRuntimeClient(client)
			r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"command_id":"command-1","expected_revision":0,"content":"Research only"}`))
			route := chi.NewRouteContext()
			route.URLParams.Add("chatID", "chat-1")
			route.URLParams.Add("runID", tc.run)
			ctx := context.WithValue(r.Context(), chi.RouteCtxKey, route)
			ctx = middleware.WithWorkspaceID(ctx, tc.workspace)
			ctx = middleware.WithUserID(ctx, tc.actor)
			response := httptest.NewRecorder()
			h.ChatRunSteering(response, r.WithContext(ctx))
			if response.Code != tc.status {
				t.Fatalf("status=%d want=%d body=%s", response.Code, tc.status, response.Body.String())
			}
			wantCalls := 0
			if tc.status == 202 {
				wantCalls = 1
			}
			if calls != wantCalls {
				t.Errorf("upstream calls=%d want=%d", calls, wantCalls)
			}
		})
	}
}
