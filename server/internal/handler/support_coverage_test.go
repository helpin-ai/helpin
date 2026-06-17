package handler

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestSupportCoverageHandlerAllowRegenerateDebouncesPerUserGap(t *testing.T) {
	h := NewSupportCoverageHandler(nil, nil, nil)

	if !h.allowRegenerate("user-1", "gap-1", 30*time.Second) {
		t.Fatal("first regenerate should be allowed")
	}
	if h.allowRegenerate("user-1", "gap-1", 30*time.Second) {
		t.Fatal("second regenerate inside debounce window should be rejected")
	}
	if !h.allowRegenerate("user-1", "gap-2", 30*time.Second) {
		t.Fatal("different gap should be allowed")
	}
	if !h.allowRegenerate("user-2", "gap-1", 30*time.Second) {
		t.Fatal("different user should be allowed")
	}
}

func TestSupportCoverageGapFilterFromRequestParsesPagination(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/support/coverage/gaps?status=open&gap_kind=content&page=3&per_page=40&show_raw=true", nil)

	filter := supportCoverageGapFilterFromRequest(req)

	if filter.Status != "open" {
		t.Fatalf("status=%q, want open", filter.Status)
	}
	if filter.GapKind != "content" {
		t.Fatalf("gap_kind=%q, want content", filter.GapKind)
	}
	if filter.Page != 3 {
		t.Fatalf("page=%d, want 3", filter.Page)
	}
	if filter.PerPage != 40 {
		t.Fatalf("per_page=%d, want 40", filter.PerPage)
	}
	if !filter.ShowRaw {
		t.Fatal("show_raw should be true")
	}
}
