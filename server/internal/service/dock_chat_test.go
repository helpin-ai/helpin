package service

import (
	"errors"
	"strings"
	"testing"
)

func TestComposeDockChatTurn(t *testing.T) {
	t.Run("without page context returns content unchanged", func(t *testing.T) {
		if got := composeDockChatTurn("hello", nil); got != "hello" {
			t.Errorf("composeDockChatTurn() = %q, want %q", got, "hello")
		}
	})

	t.Run("with page context appends block", func(t *testing.T) {
		got := composeDockChatTurn("hello", map[string]interface{}{"entity_type": "task", "entity_id": "t1"})
		if !strings.HasPrefix(got, "hello\n\n<page_context>") || !strings.HasSuffix(got, "</page_context>") {
			t.Errorf("composeDockChatTurn() = %q, want page context block", got)
		}
		if !strings.Contains(got, `"entity_type":"task"`) {
			t.Errorf("composeDockChatTurn() missing entity data: %q", got)
		}
	})
}

func TestDockChatTitleFromContent(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{name: "short content unchanged", content: "list open tasks", want: "list open tasks"},
		{name: "first line only", content: "line one\nline two", want: "line one"},
		{
			name:    "long content truncated",
			content: strings.Repeat("a", 100),
			want:    strings.Repeat("a", 60) + "…",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := dockChatTitleFromContent(tt.content); got != tt.want {
				t.Errorf("dockChatTitleFromContent() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsDockChatRunExpiredError(t *testing.T) {
	if isChatRunExpiredError(nil) {
		t.Error("isChatRunExpiredError(nil) = true, want false")
	}
	if !isChatRunExpiredError(errors.New("resume run: run idle timeout expired")) {
		t.Error("isChatRunExpiredError() = false for idle timeout error, want true")
	}
	if isChatRunExpiredError(errors.New("run is not paused")) {
		t.Error("isChatRunExpiredError() = true for unrelated error, want false")
	}
}

func TestSameNormalizedToolSet(t *testing.T) {
	if !sameNormalizedToolSet([]string{" read_file ", "ripgrep", "read_file"}, []string{"ripgrep", "read_file"}) {
		t.Fatal("same tool set with whitespace, order, and duplicates was reported stale")
	}
	if sameNormalizedToolSet([]string{"list_repositories", "list_commits"}, []string{"list_repositories", "checkout_repository", "ripgrep", "read_file"}) {
		t.Fatal("old metadata-only repository tool set was reported current")
	}
	if sameNormalizedToolSet([]string{"read_file", "write_file"}, []string{"read_file"}) {
		t.Fatal("run retaining a revoked tool was reported current")
	}
}
