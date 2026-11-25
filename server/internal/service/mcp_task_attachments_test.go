package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
)

type testMCPFileStore string

func (s testMCPFileStore) ReadObjectChunk(_ context.Context, key string, offset int64, limit int) ([]byte, error) {
	if key != "private-plan" {
		return nil, errors.New("unexpected storage key")
	}
	if offset > int64(len(s)) {
		return nil, errors.New("offset outside file")
	}
	return []byte(s[offset:min(len(s), int(offset)+limit)]), nil
}

func (s testMCPFileStore) GeneratePresignedGetURL(key, name string) (string, error) {
	if key != "private-plan" {
		return "", errors.New("unexpected storage key")
	}
	return "https://storage.example/" + name, nil
}

func TestMCPTaskAttachmentDiscoveryAndRead(t *testing.T) {
	ctx := context.Background()
	env := newPMCommandTestEnv(t)
	db := env.db
	const taskID = "b1cc90e6-8465-48bb-a907-d66f2b36bab7"
	seedPMCommandTask(t, db, taskID, "ws-1", "team-a", "wf-a", "state-a", "", "", 42)
	svc := &MCPService{commands: env.service}
	p := &model.MCPPrincipal{WorkspaceID: env.workspaceID, UserID: env.actorID}
	actor := &authorization.Actor{WorkspaceID: env.workspaceID, UserID: env.actorID, Role: model.RoleAdmin}
	ctx = authorization.WithActor(ctx, actor)
	defs := map[string]MCPToolDefinition{}
	for _, def := range svc.buildToolCatalog() {
		defs[def.Name] = def
	}
	plan := strings.Repeat("a", 24*1024-1) + "✅ done"
	env.service.SetTaskFileStore(testMCPFileStore(plan))
	mustExec(t, db, "INSERT INTO pm_comments (id,entity_type,entity_id,author_id,body) VALUES ('comment','task',?,'u','Plan')", taskID)
	for _, f := range []struct {
		id, ws, kind, entity string
		uploaded             bool
	}{
		{"plan", "ws-1", "comment", "comment", true}, {"direct", "ws-1", "task", taskID, true},
		{"other", "ws-1", "task", "task-2", true}, {"foreign", "ws-2", "task", taskID, true}, {"pending", "ws-1", "task", taskID, false},
	} {
		mustExec(t, db, "INSERT INTO pm_attachments(id,workspace_id,entity_type,entity_id,file_name,storage_key,is_uploaded) VALUES(?,?,?,?,?,?,?)", f.id, f.ws, f.kind, f.entity, "plan.md", "private-plan", f.uploaded)
	}
	call := func(name string, args map[string]any) (*MCPToolResult, error) {
		t.Helper()
		args["task_id"] = taskID
		raw, err := json.Marshal(args)
		if err != nil {
			t.Fatal(err)
		}
		return svc.executeAuthorizedMCPTool(ctx, p, actor, defs[name], raw)
	}
	listed, err := call("list_task_attachments", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	items := listed.Data.(map[string]any)["attachments"].([]any)
	if len(items) != 2 || items[0].(map[string]any)["attachment_id"] != "direct" || items[1].(map[string]any)["attachment_id"] != "plan" {
		t.Fatalf("listed: %v", items)
	}
	var content string
	for offset := 0; ; {
		chunk, err := call("read_task_attachment", map[string]any{"attachment_id": "plan", "offset": offset})
		if err != nil {
			t.Fatal(err)
		}
		data := chunk.Data.(map[string]any)
		if data["download_url"] != "https://storage.example/plan.md" {
			t.Fatal(data["download_url"])
		}
		if data["encoding"] != "utf-8" {
			t.Fatal(data["encoding"])
		}
		content += data["content"].(string)
		next, ok := data["next_offset"].(float64)
		if !ok {
			break
		}
		if int(next) <= offset {
			t.Fatal("offset did not advance")
		}
		offset = int(next)
	}
	if content != plan {
		t.Fatal("plan changed across chunk boundary")
	}
	for _, id := range []string{"other", "foreign", "pending"} {
		if _, err := call("read_task_attachment", map[string]any{"attachment_id": id}); mcpToolErrorCode(err) != MCPErrorCodeNotFound {
			t.Fatalf("read %s: %v", id, err)
		}
	}
	mustExec(t, db, "UPDATE pm_attachments SET file_name='design.png', content_type='image/png' WHERE id='direct'")
	calls := 0
	env.service.SetAttachmentMediaAnalyzer(dockMediaCompletionFunc(func(_ context.Context, request llm.ChatRequest) (*llm.ChatResponse, error) {
		calls++
		return &llm.ChatResponse{Content: "The upload failed."}, nil
	}))
	visual, err := call("analyze_task_attachment", map[string]any{"attachment_id": "direct", "question": "What happened?"})
	if err != nil || visual.Data.(map[string]any)["analysis"] != "The upload failed." {
		t.Fatalf("MCP visual analysis: %v, %v", visual, err)
	}
	for _, id := range []string{"other", "foreign", "pending"} {
		if _, err := call("analyze_task_attachment", map[string]any{"attachment_id": id, "question": "What happened?"}); mcpToolErrorCode(err) != MCPErrorCodeNotFound {
			t.Fatalf("analyze %s: %v", id, err)
		}
	}
	if calls != 1 {
		t.Fatalf("expected one authorized model call, got %d", calls)
	}
	// The normal task read includes all files even when discovery needs another page.
	for i := 0; i < 49; i++ {
		mustExec(t, db, "INSERT INTO pm_attachments(id,workspace_id,entity_type,entity_id,file_name,storage_key,is_uploaded) VALUES(?, 'ws-1','task',?, 'log.txt','log',true)", fmt.Sprintf("log-%02d", i), taskID)
	}
	task, err := call("get_task", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if got := len(task.Data.(*agentTaskDetail).Attachments); got != 51 {
		t.Fatalf("get_task returned %d of 51 files", got)
	}
	page, err := call("list_task_attachments", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	first := page.Data.(map[string]any)
	if len(first["attachments"].([]any)) != 50 || first["next_offset"] != float64(50) {
		t.Fatal("expected a full first page and a continuation")
	}
	page, err = call("list_task_attachments", map[string]any{"offset": first["next_offset"]})
	if err != nil {
		t.Fatal(err)
	}
	last := page.Data.(map[string]any)
	if len(last["attachments"].([]any)) != 1 || last["next_offset"] != nil {
		t.Fatal("expected the remaining attachment on the last page")
	}

}
