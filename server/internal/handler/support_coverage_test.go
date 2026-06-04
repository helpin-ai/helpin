package handler

import (
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
