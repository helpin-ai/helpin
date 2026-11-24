package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/repository"
)

type testMCPFileStore string

func (s testMCPFileStore) ReadObjectChunk(_ context.Context, key string, offset int64, limit int) ([]byte, error) {
	if key != "private-plan" {
		return nil, errors.New("unexpected storage key")
	}
	return []byte(s[offset:min(len(s), int(offset)+limit)]), nil
}

func TestMCPTaskAttachmentDiscoveryAndRead(t *testing.T) {
	ctx := context.Background()
	svc, p, actor, _, _ := setupMCPPMToolTest()
	db := newTestDB(t)
	plan := strings.Repeat("a", 24*1024-1) + "✅ done"
	svc.SetTaskAttachments(repository.NewPMAttachmentRepository(db), testMCPFileStore(plan))
	mustExec(t, db, "INSERT INTO pm_comments (id,entity_type,entity_id,author_id,body) VALUES ('comment','task','task-1','u','Plan')")
	for _, f := range []struct {
		id, ws, kind, entity string
		uploaded             bool
	}{
		{"plan", "workspace-1", "comment", "comment", true}, {"direct", "workspace-1", "task", "task-1", true},
		{"other", "workspace-1", "task", "task-2", true}, {"foreign", "workspace-2", "task", "task-1", true}, {"pending", "workspace-1", "task", "task-1", false},
	} {
		mustExec(t, db, "INSERT INTO pm_attachments(id,workspace_id,entity_type,entity_id,file_name,storage_key,is_uploaded) VALUES(?,?,?,?,?,?,?)", f.id, f.ws, f.kind, f.entity, "plan.md", "private-plan", f.uploaded)
	}
	call := func(name string, args map[string]any) (*MCPToolResult, error) {
		t.Helper()
		args["task_id"] = "task-1"
		raw, err := json.Marshal(args)
		if err != nil {
			t.Fatal(err)
		}
		return svc.executeSpecialMCPTool(ctx, p, actor, name, raw)
	}
	listed, err := call("list_task_attachments", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	items := listed.Data.(map[string]any)["attachments"].([]map[string]any)
	if len(items) != 2 || items[0]["attachment_id"] != "direct" || items[1]["attachment_id"] != "plan" {
		t.Fatalf("listed: %v", items)
	}
	var content string
	for offset := 0; ; {
		chunk, err := call("read_task_attachment", map[string]any{"attachment_id": "plan", "offset": offset})
		if err != nil {
			t.Fatal(err)
		}
		data := chunk.Data.(map[string]any)
		if data["encoding"] != "utf-8" {
			t.Fatal(data["encoding"])
		}
		content += data["content"].(string)
		next, ok := data["next_offset"].(int)
		if !ok {
			break
		}
		if next <= offset {
			t.Fatal("offset did not advance")
		}
		offset = next
	}
	if content != plan {
		t.Fatal("plan changed across chunk boundary")
	}
	for _, id := range []string{"other", "foreign", "pending"} {
		if _, err := call("read_task_attachment", map[string]any{"attachment_id": id}); !errors.Is(err, ErrMCPNotFound) {
			t.Fatalf("read %s: %v", id, err)
		}
	}
}
