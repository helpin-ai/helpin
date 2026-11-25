package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	agentruntime "github.com/helpin-ai/agent-runtime-go"

	"github.com/helpin-ai/helpin/server/internal/agentcontract"
	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestTaskReadToolsIncludeAttachments(t *testing.T) {
	env := newPMCommandTestEnv(t)
	seedPMCommandTask(t, env.db, "task-a", "ws-1", "team-a", "wf-a", "state-a", "", "", 105)
	seedPMCommandTask(t, env.db, "task-b", "ws-1", "team-a", "wf-a", "state-a", "", "", 106)
	mustExec(t, env.db, "INSERT INTO pm_comments(id, entity_type, entity_id, author_id, body) VALUES ('comment-1', 'task', 'task-a', 'actor-1', 'See plan')")
	mustExec(t, env.db, `INSERT INTO pm_attachments(id,workspace_id,entity_type,entity_id,file_name,file_size,content_type,storage_key,uploaded_by_id,uploaded_by_agent_id,is_uploaded,created_at)
		VALUES ('file-1','ws-1','comment','comment-1','plan.md',7,'text/markdown','private-plan','actor-1','agent-1',true,CURRENT_TIMESTAMP)`)
	env.service.SetReleaseFactsProvider(NewReleaseFactsService(nil, nil, repository.NewPMTaskRepository(env.db), nil, nil, nil, nil, nil, repository.NewWorkspaceRepository(env.db), nil))
	ctx := context.Background()
	meta := env.meta("workspace", "ws-1")
	for _, tc := range []struct{ name, args string }{
		{"pm.get_task", `{"task_id":"task-a"}`},
		{"release.get_task_context", `{"task_ids":["task-a","task-b"]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := env.service.Execute(ctx, meta, tc.name, json.RawMessage(tc.args))
			if err != nil {
				t.Fatal(err)
			}
			var values []map[string]any
			if tc.name == "pm.get_task" {
				var detail map[string]any
				if err := json.Unmarshal(out, &detail); err != nil {
					t.Fatal(err)
				}
				values = append(values, detail)
			} else {
				var batch struct {
					Tasks []map[string]any `json:"tasks"`
				}
				if err := json.Unmarshal(out, &batch); err != nil {
					t.Fatal(err)
				}
				values = batch.Tasks
				if len(values) != 2 {
					t.Fatalf("got %d tasks", len(values))
				}
			}
			for _, task := range values {
				files, ok := task["attachments"].([]any)
				if !ok {
					t.Fatalf("missing attachment array for %v", task["task_id"])
				}
				if task["task_id"] == "task-b" {
					if len(files) != 0 {
						t.Fatal("unrelated attachments", files)
					}
					continue
				}
				if len(files) != 1 {
					t.Fatal(files)
				}
				f := files[0].(map[string]any)
				if f["attachment_id"] != "file-1" || f["file_name"] != "plan.md" || f["content_type"] != "text/markdown" || f["file_size"] != float64(7) || f["uploaded_by_id"] != "actor-1" || f["uploaded_by_agent_id"] != "agent-1" || f["created_at"] == nil || f["entity_id"] != "comment-1" || f["storage_key"] != nil {
					t.Fatalf("unexpected attachment metadata: %v", f)
				}
			}
		})
	}
}

func TestTaskAttachmentsIncludeEditorFilesFromDescription(t *testing.T) {
	env := newPMCommandTestEnv(t)
	seedPMCommandTask(t, env.db, "task-a", "ws-1", "team-a", "wf-a", "state-a", "", "", 105)
	const plan = "4a4be826-be3e-4fc7-9e1d-28bc6c6cea70"
	const image = "bf77a7ab-e50b-4657-ab98-f843c1635c3e"
	const other = "99381d4c-a69a-4dd6-89df-c32cc8009e2b"
	description := fmt.Sprintf(`<a href="https://helpin.test/api/pm/attachments/%s/content">plan.md</a><img data-attachment-id="%s"><img src="/api/pm/attachments/%s/content">`, plan, image, other)
	mustExec(t, env.db, "UPDATE pm_tasks SET description=? WHERE id='task-a'", description)
	for _, f := range []struct{ id, kind, entity string }{{plan, "editor_upload", "ws-1"}, {image, "editor_upload", "ws-1"}, {other, "task", "task-b"}} {
		mustExec(t, env.db, `INSERT INTO pm_attachments(id,workspace_id,entity_type,entity_id,file_name,storage_key,is_uploaded) VALUES(?, 'ws-1',?,?, 'file','private-plan',true)`, f.id, f.kind, f.entity)
	}
	env.service.SetTaskFileStore(testMCPFileStore("# Existing plan"))
	ctx := context.Background()
	meta := env.meta("task", "task-a")
	listed, err := env.service.executeListTaskAttachments(ctx, meta, json.RawMessage(`{"task_id":"task-a"}`))
	if err != nil {
		t.Fatal(err)
	}
	var result struct{ Attachments []model.TaskAttachment }
	if err := json.Unmarshal(listed, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Attachments) != 2 || result.Attachments[0].AttachmentID != plan || result.Attachments[1].AttachmentID != image {
		t.Fatalf("unexpected inline files: %v", result.Attachments)
	}
	for _, file := range result.Attachments {
		if file.TaskID != "task-a" {
			t.Fatalf("incorrect task for inline file: %v", file)
		}
	}
	body, err := env.service.executeReadTaskAttachment(ctx, meta, mustJSON(map[string]string{"task_id": "task-a", "attachment_id": plan}))
	if err != nil || !strings.Contains(string(body), "# Existing plan") {
		t.Fatalf("read inline Markdown: %s, %v", body, err)
	}
	if _, err := env.service.executeReadTaskAttachment(ctx, meta, mustJSON(map[string]string{"task_id": "task-a", "attachment_id": other})); err == nil {
		t.Fatal("expected the file owned by a different task to be excluded")
	}
}

func TestTaskAttachmentToolsReachAgents(t *testing.T) {
	tools := agentcontract.ListToolCatalog().Tools
	for _, name := range []string{"list_task_attachments", "read_task_attachment", "analyze_task_attachment"} {
		if !slices.ContainsFunc(tools, func(t model.ToolCatalogEntry) bool { return t.Name == name }) {
			t.Errorf("runtime catalog missing %s", name)
		}
		for surface, preset := range map[string][]string{"Dock": askAgentPresetTools(), "sub-agent": commandAgentPresetTools()} {
			if !slices.Contains(preset, name) {
				t.Errorf("%s missing %s", surface, name)
			}
		}
	}
}

// Exercise the model-facing runtime provider used by Dock and delegated agents.
func TestTaskAttachmentsRuntimeProvider(t *testing.T) {
	env := newPMCommandTestEnv(t)
	mustExec(t, env.db, "UPDATE workspaces SET workspace_key = 'TST' WHERE id = 'ws-1'")
	seedPMCommandTask(t, env.db, "task-b", "ws-1", "team-b", "wf-b", "state-b", "", "", 123)
	authz := authorization.NewAuthzService(env.db, authorization.NewGORMMemberRepository(env.db), repository.NewWorkspaceModuleGrantRepository(env.db))
	env.service.SetAuthorizationService(authz)
	agentDB := setupAgentScopeTestDB(t)
	seedAgentScopeAgent(t, agentDB, model.Agent{ID: "ask-agent", WorkspaceID: "ws-1", Name: "Ask Agent", Status: "idle", RuntimeKind: "native_sdk", Skills: model.AgentSkillRefs{}, ExecutionConfig: model.JSONBlob(`{}`), AllowedTools: mustJSON(askAgentPresetTools()), AllowedCommands: mustJSON([]string{}), AllowedTargets: mustJSON([]string{"workspace", "task"}), ApprovalMode: "risk_based", DefaultInvocationMode: "interactive", TeamIDs: []string{"team-b"}})
	host := NewAgentRuntimeHostService("helpin", nil, nil, nil, nil, nil, nil, nil, nil, nil, env.service, nil).
		SetAgentRepository(repository.NewAgentRepository(agentDB)).SetAuthorizationService(authz)
	req := agentruntime.ProviderToolCallRequest{
		Meta:     agentruntime.CommandExecutionContext{AppID: "helpin", WorkspaceID: "ws-1", AgentID: "ask-agent", ExternalActorID: "actor-1", TargetType: "workspace", TargetID: "ws-1"},
		ToolName: "read_task_attachment",
	}
	analyses := 0
	reply := llm.ChatResponse{Content: "Upload timed out.", FinishReason: "stop"}
	env.service.SetAttachmentMediaAnalyzer(dockMediaCompletionFunc(func(_ context.Context, request llm.ChatRequest) (*llm.ChatResponse, error) {
		analyses++
		parts := request.Messages[0].ContentParts
		if len(parts) != 2 || parts[1].ImageURL == nil || parts[1].ImageURL.URL != "https://storage.example/design.png" {
			t.Fatalf("expected the signed task image, got %+v", parts)
		}
		return &reply, nil
	}))

	for i, tc := range []struct{ name, mime, content string }{
		{"plan.md", "text/markdown", "# Plan"}, {"notes.txt", "text/plain", "Read this"},
		{"data.csv", "text/csv", "id,name"}, {"config.json", "application/json", `{"ok":true}`},
		{"app.log", "application/octet-stream", "INFO done"}, {"design.png", "image/png", "\x89PNG\x00"},
	} {
		id := fmt.Sprintf("file-%d", i)
		mustExec(t, env.db, `INSERT INTO pm_attachments(id,workspace_id,entity_type,entity_id,file_name,content_type,file_size,storage_key,is_uploaded) VALUES(?, 'ws-1','task','task-b',?,?,?,'private-plan',true)`, id, tc.name, tc.mime, len(tc.content))
		env.service.SetTaskFileStore(testMCPFileStore(tc.content))
		req.Input = mustJSON(map[string]any{"task_id": "TST-123", "attachment_id": id})
		result, err := host.CallProviderTool(context.Background(), req)
		if err != nil || result.IsError {
			t.Fatalf("read %s: %v, %v", tc.name, result, err)
		}
		var file struct {
			Content, Encoding string
			URL               string `json:"download_url"`
		}
		if err := json.Unmarshal(result.StructuredContent, &file); err != nil {
			t.Fatal(err)
		}
		want := tc.content
		if tc.mime == "image/png" {
			want = base64.StdEncoding.EncodeToString([]byte(tc.content))
			if file.Encoding != "base64" {
				t.Fatal(file.Encoding)
			}
		} else if file.Encoding != "utf-8" {
			t.Fatal(file.Encoding)
		}
		if file.Content != want || file.URL != "https://storage.example/"+tc.name {
			t.Fatalf("%s: %#v", tc.name, file)
		}
	}

	env.service.SetReleaseFactsProvider(NewReleaseFactsService(nil, nil, env.service.taskRepo, nil, nil, nil, nil, nil, repository.NewWorkspaceRepository(env.db), nil))
	req.ToolName = "get_task_context"
	req.Input = mustJSON(map[string]any{"task_keys": []string{"TST-123"}})
	result, err := host.CallProviderTool(context.Background(), req)
	if err != nil || result.IsError {
		t.Fatalf("get task context: %v, %v", result, err)
	}
	var resultContext model.GetTaskContextResult
	if err := json.Unmarshal(result.StructuredContent, &resultContext); err != nil {
		t.Fatal(err)
	}
	if len(resultContext.Tasks) != 1 || len(resultContext.Tasks[0].Attachments) != 6 {
		t.Fatalf("missing attachments in runtime context: %v", resultContext.Tasks)
	}

	if analyses != 0 {
		t.Fatal("ordinary attachment reads invoked the model")
	}
	req.ToolName = "analyze_task_attachment"
	req.Input = mustJSON(map[string]string{"task_id": "TST-123", "attachment_id": "file-5", "question": "What failed?"})
	result, err = host.CallProviderTool(context.Background(), req)
	if err != nil || result.IsError || !strings.Contains(string(result.StructuredContent), "Upload timed out.") {
		t.Fatalf("analyze: %v, %v", result, err)
	}

	reply = llm.ChatResponse{Content: "The upload timed", FinishReason: "length"}
	result, err = host.CallProviderTool(context.Background(), req)
	if err != nil || !result.IsError || len(result.StructuredContent) != 0 {
		t.Fatalf("expected an error without a partial answer: %v, %v", result, err)
	}

	for _, tc := range []struct{ name, actor, targetType, targetID string }{
		{"member outside task team", "user-2", "workspace", "ws-1"},
		{"different task target", "actor-1", "task", "another-task"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req.Meta.ExternalActorID, req.Meta.TargetType, req.Meta.TargetID = tc.actor, tc.targetType, tc.targetID
			for _, name := range []string{"get_task", "get_task_context", "list_task_attachments", "read_task_attachment", "analyze_task_attachment"} {
				req.ToolName = name
				args := map[string]any{"task_id": "task-b"}
				if name == "get_task_context" {
					args = map[string]any{"task_ids": []string{"task-b"}}
				}
				if name == "read_task_attachment" || name == "analyze_task_attachment" {
					args["attachment_id"] = "file-5"
				}
				if name == "analyze_task_attachment" {
					args["question"] = "What failed?"
				}
				req.Input = mustJSON(args)
				resp, err := host.CallProviderTool(context.Background(), req)
				if err != nil || !resp.IsError || len(resp.StructuredContent) != 0 {
					t.Fatalf("expected denied %s: %v, %v", name, resp, err)
				}
			}
		})
	}
	if analyses != 2 {
		t.Fatalf("got %d model calls, want two authorized attempts", analyses)
	}
}
