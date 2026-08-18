package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestWidgetSafeSupportMessagesRemovesLinkSecurity(t *testing.T) {
	messages := []model.SupportMessage{{
		Metadata: `{"link_previews":[{"url":"http://example.com"}],"link_security":[{"status":"malicious"}],"other":true}`,
	}}

	got := widgetSafeSupportMessages(messages)
	if strings.Contains(got[0].Metadata, "link_security") {
		t.Fatalf("widget metadata leaked link security: %s", got[0].Metadata)
	}
	if !strings.Contains(got[0].Metadata, "link_previews") || !strings.Contains(got[0].Metadata, `"other":true`) {
		t.Fatalf("widget metadata lost safe fields: %s", got[0].Metadata)
	}
	if strings.Contains(messages[0].Metadata, `"other":false`) {
		t.Fatal("input messages were mutated")
	}
}

func TestSupportInboxWidgetHandlerValidation(t *testing.T) {
	h := NewSupportInboxWidgetHandler(nil)

	t.Run("GetConfig requires widget_key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/widget/support/config", nil)
		rec := httptest.NewRecorder()

		h.GetConfig(rec, req)

		assertAPIError(t, rec, http.StatusBadRequest, "widget_key is required")
	})

	t.Run("CreateSession rejects invalid json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/widget/support/session", strings.NewReader("{"))
		rec := httptest.NewRecorder()

		h.CreateSession(rec, req)

		assertAPIError(t, rec, http.StatusBadRequest, "invalid request body")
	})

	t.Run("CreateSession requires widget_key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/widget/support/session", bytes.NewBufferString(`{}`))
		rec := httptest.NewRecorder()

		h.CreateSession(rec, req)

		assertAPIError(t, rec, http.StatusBadRequest, "widget_key is required")
	})

	t.Run("RevokeSession requires session_token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/widget/session/revoke", bytes.NewBufferString(`{}`))
		rec := httptest.NewRecorder()

		h.RevokeSession(rec, req)

		assertAPIError(t, rec, http.StatusBadRequest, "session_token is required")
	})

	t.Run("SendMessage requires token and content", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/widget/support/messages", bytes.NewBufferString(`{"session_token":"token"}`))
		rec := httptest.NewRecorder()

		h.SendMessage(rec, req)

		assertAPIError(t, rec, http.StatusBadRequest, "session_token and content are required")
	})

	t.Run("TypingIndicator requires session_token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/widget/typing", bytes.NewBufferString(`{"is_typing":true}`))
		rec := httptest.NewRecorder()

		h.TypingIndicator(rec, req)

		assertAPIError(t, rec, http.StatusBadRequest, "session_token is required")
	})

	t.Run("GetMessages requires session_token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/widget/support/messages", nil)
		rec := httptest.NewRecorder()

		h.GetMessages(rec, req)

		assertAPIError(t, rec, http.StatusBadRequest, "session_token is required")
	})

	t.Run("SendTranscript requires conversation_id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/widget/support/conversations/transcript", bytes.NewBufferString(`{"session_token":"token"}`))
		rec := httptest.NewRecorder()

		h.SendTranscript(rec, req)

		assertAPIError(t, rec, http.StatusBadRequest, "conversation_id is required")
	})

	t.Run("SendTranscript requires session_token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/widget/support/conversations/conv-1/transcript", bytes.NewBufferString(`{}`))
		req = withURLParam(req, "conversationId", "conv-1")
		rec := httptest.NewRecorder()

		h.SendTranscript(rec, req)

		assertAPIError(t, rec, http.StatusBadRequest, "session_token is required")
	})

	t.Run("Identify rejects invalid json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/widget/identify", strings.NewReader("{"))
		rec := httptest.NewRecorder()

		h.Identify(rec, req)

		assertAPIError(t, rec, http.StatusBadRequest, "invalid request body")
	})

	t.Run("Identify requires api_key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/widget/identify", bytes.NewBufferString(`{"anonymous_id":"anon-1"}`))
		rec := httptest.NewRecorder()

		h.Identify(rec, req)

		assertAPIError(t, rec, http.StatusBadRequest, "api_key is required")
	})

	t.Run("Identify requires anonymous_id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/widget/identify", bytes.NewBufferString(`{"api_key":"wk_123"}`))
		rec := httptest.NewRecorder()

		h.Identify(rec, req)

		assertAPIError(t, rec, http.StatusBadRequest, "anonymous_id is required")
	})

	t.Run("Identify allows empty email", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/widget/identify", bytes.NewBufferString(`{"api_key":"wk_123","anonymous_id":"anon-1","name":"Visitor"}`))
		rec := httptest.NewRecorder()

		h.Identify(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}

		var resp map[string]bool
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if !resp["success"] {
			t.Fatalf("response = %#v, want success=true", resp)
		}
	})

	t.Run("GetHelpCollections requires widget_key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/widget/support/help/spaces/docs/collections", nil)
		rec := httptest.NewRecorder()

		h.GetHelpCollections(rec, req)

		assertAPIError(t, rec, http.StatusBadRequest, "widget_key is required")
	})

	t.Run("GetHelpCollections requires spaceSlug", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/widget/support/help/spaces//collections?widget_key=wk_123", nil)
		req = withURLParam(req, "spaceSlug", "")
		rec := httptest.NewRecorder()

		h.GetHelpCollections(rec, req)

		assertAPIError(t, rec, http.StatusBadRequest, "spaceSlug is required")
	})

	t.Run("GetHelpArticles requires widget_key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/widget/support/help/collections/getting-started/articles", nil)
		rec := httptest.NewRecorder()

		h.GetHelpArticles(rec, req)

		assertAPIError(t, rec, http.StatusBadRequest, "widget_key is required")
	})

	t.Run("GetHelpArticles requires collectionSlug", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/widget/support/help/collections//articles?widget_key=wk_123", nil)
		req = withURLParam(req, "collectionSlug", "")
		rec := httptest.NewRecorder()

		h.GetHelpArticles(rec, req)

		assertAPIError(t, rec, http.StatusBadRequest, "collectionSlug is required")
	})

	t.Run("GetHelpArticle requires widget_key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/widget/support/help/articles/install", nil)
		rec := httptest.NewRecorder()

		h.GetHelpArticle(rec, req)

		assertAPIError(t, rec, http.StatusBadRequest, "widget_key is required")
	})

	t.Run("SearchHelpArticles requires widget_key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/widget/support/help/search?q=reset", nil)
		rec := httptest.NewRecorder()

		h.SearchHelpArticles(rec, req)

		assertAPIError(t, rec, http.StatusBadRequest, "widget_key is required")
	})

	t.Run("SearchHelpArticles requires query", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/widget/support/help/search?widget_key=wk_123", nil)
		rec := httptest.NewRecorder()

		h.SearchHelpArticles(rec, req)

		assertAPIError(t, rec, http.StatusBadRequest, "q is required")
	})

	t.Run("GetHelpArticle requires articleKey", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/widget/support/help/articles/?widget_key=wk_123", nil)
		req = withURLParam(req, "articleKey", "")
		rec := httptest.NewRecorder()

		h.GetHelpArticle(rec, req)

		assertAPIError(t, rec, http.StatusBadRequest, "articleKey is required")
	})
}

func assertAPIError(t *testing.T, rec *httptest.ResponseRecorder, status int, message string) {
	t.Helper()

	if rec.Code != status {
		t.Fatalf("status = %d, want %d", rec.Code, status)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content-type = %q, want application/json", got)
	}
	if body := rec.Body.String(); !strings.Contains(body, `"`+"error"+`":"`+message+`"`) {
		t.Fatalf("body = %q, want error %q", body, message)
	}
}

func withURLParam(req *http.Request, key, value string) *http.Request {
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add(key, value)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
}
