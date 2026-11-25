package service

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/llm"
)

func TestTaskAttachmentDocumentReads(t *testing.T) {
	var docx bytes.Buffer
	zw := zip.NewWriter(&docx)
	doc, err := zw.Create("word/document.xml")
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Repeat("a", 24*1024-1) + "✅ Friday"
	if _, err := doc.Write([]byte("<document><p><t>" + want + "</t></p></document>")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	pdf, err := os.ReadFile("testdata/task_attachment_text.pdf")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, mime, data, text string }{
		{"brief.pdf", "application/pdf", string(pdf), "Delivery date is Friday."},
		{"spec.docx", "application/octet-stream", docx.String(), want},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := newPMCommandTestEnv(t)
			seedPMCommandTask(t, env.db, "task-a", "ws-1", "team-a", "wf-a", "state-a", "", "", 105)
			mustExec(t, env.db, `INSERT INTO pm_attachments(id,workspace_id,entity_type,entity_id,file_name,content_type,file_size,storage_key,is_uploaded) VALUES('file','ws-1','task','task-a',?,?,?,'private-plan',true)`, tc.name, tc.mime, len(tc.data))
			env.service.SetTaskFileStore(testMCPFileStore(tc.data))
			var text strings.Builder
			offset := 0
			for {
				out, err := env.service.Execute(context.Background(), env.meta("task", "task-a"), "pm.read_task_attachment", mustJSON(map[string]any{"attachment_id": "file", "offset": offset}))
				if err != nil {
					t.Fatal(err)
				}
				var page struct {
					Content, Encoding, Representation string
					NextOffset                        int `json:"next_offset"`
				}
				if err := json.Unmarshal(out, &page); err != nil {
					t.Fatal(err)
				}
				if page.Encoding != "utf-8" || page.Representation != "extracted_text" || len(page.Content) > 24*1024 {
					t.Fatalf("unexpected document response: encoding=%q representation=%q length=%d", page.Encoding, page.Representation, len(page.Content))
				}
				text.WriteString(page.Content)
				if page.NextOffset == 0 {
					break
				}
				if page.NextOffset <= offset {
					t.Fatal("pagination did not advance")
				}
				offset = page.NextOffset
			}
			if text.String() != tc.text {
				t.Fatalf("extracted text differs: got %d bytes, want %d", text.Len(), len(tc.text))
			}
		})
	}
}

func TestTaskAttachmentDocumentFallback(t *testing.T) {
	pdf, err := os.ReadFile("testdata/task_attachment_text.pdf")
	if err != nil {
		t.Fatal(err)
	}
	scan := strings.Replace(string(pdf), "Delivery date is Friday.", strings.Repeat(" ", len("Delivery date is Friday.")), 1)
	for _, tc := range []struct {
		name, data, notice string
		size               int
	}{
		{"scanned", scan, "analyze_task_attachment", len(scan)},
		{"unreadable", "broken document", "unsuccessful", 15},
		{"oversized", "original bytes", "8 MiB", maxTaskDocumentBytes + 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := newPMCommandTestEnv(t)
			seedPMCommandTask(t, env.db, "task-a", "ws-1", "team-a", "wf-a", "state-a", "", "", 105)
			mustExec(t, env.db, `INSERT INTO pm_attachments(id,workspace_id,entity_type,entity_id,file_name,content_type,file_size,storage_key,is_uploaded) VALUES('file','ws-1','task','task-a','brief.pdf','application/pdf',?,'private-plan',true)`, tc.size)
			env.service.SetTaskFileStore(testMCPFileStore(tc.data))
			read := func(raw bool) taskAttachmentReadResult {
				t.Helper()
				out, err := env.service.Execute(context.Background(), env.meta("task", "task-a"), "pm.read_task_attachment", mustJSON(map[string]any{"attachment_id": "file", "raw": raw}))
				if err != nil {
					t.Fatal(err)
				}
				var result taskAttachmentReadResult
				if err := json.Unmarshal(out, &result); err != nil {
					t.Fatal(err)
				}
				return result
			}
			result := read(false)
			if !strings.Contains(result.Notice, tc.notice) || result.DownloadURL != "https://storage.example/brief.pdf" || result.Content != "" {
				t.Fatalf("unexpected document fallback: %+v", result)
			}
			original := read(true)
			data := []byte(original.Content)
			if original.Encoding == "base64" {
				data, err = base64.StdEncoding.DecodeString(original.Content)
				if err != nil {
					t.Fatal(err)
				}
			}
			if string(data) != tc.data || original.Representation != "" {
				t.Fatal("raw read did not preserve original bytes")
			}
		})
	}
}

func TestTaskAttachmentVisualAnalysis(t *testing.T) {
	env := newPMCommandTestEnv(t)
	seedPMCommandTask(t, env.db, "task-a", "ws-1", "team-a", "wf-a", "state-a", "", "", 105)
	seedPMCommandTask(t, env.db, "task-b", "ws-1", "team-b", "wf-b", "state-b", "", "", 106)
	env.service.SetTaskFileStore(testMCPFileStore(""))
	for _, tc := range []struct{ name, mime, part string }{{"error.png", "image/png", "image_url"}, {"demo.mp4", "video/mp4", "video_url"}, {"scan.pdf", "application/pdf", "file"}} {
		t.Run(tc.name, func(t *testing.T) {
			mustExec(t, env.db, `INSERT INTO pm_attachments(id,workspace_id,entity_type,entity_id,file_name,content_type,file_size,storage_key,is_uploaded) VALUES(?,'ws-1','task','task-a',?,?,128,'private-plan',true)`, tc.name, tc.name, tc.mime)
			calls := 0
			env.service.SetAttachmentMediaAnalyzer(dockMediaCompletionFunc(func(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
				calls++
				parts := req.Messages[0].ContentParts
				if len(parts) != 3 || !strings.Contains(parts[0].Text, "What failed?") || parts[2].Type != tc.part {
					t.Fatalf("unexpected provider request: %+v", parts)
				}
				return &llm.ChatResponse{Content: "Upload timed out."}, nil
			}))
			args := mustJSON(map[string]string{"task_id": "task-a", "attachment_id": tc.name, "question": "What failed?"})
			if _, err := env.service.Execute(context.Background(), env.meta("task", "task-b"), "pm.analyze_task_attachment", args); err == nil || calls != 0 {
				t.Fatal("expected denial before model call")
			}
			out, err := env.service.Execute(context.Background(), env.meta("workspace", "ws-1"), "pm.analyze_task_attachment", args)
			if err != nil {
				t.Fatal(err)
			}
			var result struct {
				Analysis     string
				AttachmentID string `json:"attachment_id"`
			}
			if err := json.Unmarshal(out, &result); err != nil {
				t.Fatal(err)
			}
			if result.Analysis != "Upload timed out." || result.AttachmentID != tc.name || calls != 1 {
				t.Fatalf("unexpected analysis: %s", out)
			}
		})
	}
}
