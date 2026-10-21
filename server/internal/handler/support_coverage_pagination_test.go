package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCoverageInsightHandlersReturnFullTotalsForLaterPages(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:coverage_handler_%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE coverage_topics (id TEXT PRIMARY KEY, workspace_id TEXT, status TEXT, conversation_count INTEGER, customer_count INTEGER, updated_at DATETIME)`,
		`CREATE TABLE coverage_unreviewed_signals (id TEXT PRIMARY KEY, workspace_id TEXT, status TEXT, observed_at DATETIME)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 26; i++ {
		id := fmt.Sprintf("item-%02d", i)
		if err := db.Exec(`INSERT INTO coverage_topics VALUES (?, 'ws-1', 'open', 1, 1, ?)`, id, time.Now().UTC()).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Exec(`INSERT INTO coverage_unreviewed_signals VALUES (?, 'ws-1', 'unreviewed', ?)`, id, time.Now().UTC()).Error; err != nil {
			t.Fatal(err)
		}
	}
	h := NewSupportCoverageHandler(nil, nil, nil, nil).SetCoverageV2Service(service.NewSupportCoverageV2Service(repository.NewCoverageV2Repository(db)))
	for _, test := range []struct {
		name    string
		handler http.HandlerFunc
	}{
		{"topics", h.ListTopicsV2}, {"signals", h.ListSignalsV2},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/?limit=25&offset=25", nil)
			r = r.WithContext(middleware.WithWorkspaceID(r.Context(), "ws-1"))
			w := httptest.NewRecorder()
			test.handler(w, r)
			if w.Code != http.StatusOK {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
			var page struct {
				Total int
				Items []json.RawMessage
			}
			if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil {
				t.Fatal(err)
			}
			if page.Total != 26 || len(page.Items) != 1 {
				t.Fatalf("page total %d, items %d", page.Total, len(page.Items))
			}
		})
	}
}

func TestCoverageConversationFilterRejectsInvalidID(t *testing.T) {
	h := NewSupportCoverageHandler(nil, nil, nil, nil)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/?conversation_id=invalid", nil)
	h.ListGaps(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", w.Code)
	}
}
