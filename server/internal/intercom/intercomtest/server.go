// Package intercomtest serves a fake Intercom API built from fixtures,
// following the shapes and pagination of the real one, for import tests.
package intercomtest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/helpin-ai/helpin/server/internal/intercom"
)

// Token is the access token the fake accepts.
const Token = "test-intercom-token"

// Server is a fake Intercom API.
type Server struct {
	*httptest.Server
	mu            sync.Mutex
	Collections   []intercom.Collection
	Articles      []intercom.Article
	Conversations []intercom.Conversation
	Contacts      map[string]intercom.Contact
	// RateLimitNext answers the next N requests with 429.
	RateLimitNext int
	Requests      []string
}

// New starts a fake with the given data.
func New() *Server {
	s := &Server{Contacts: map[string]intercom.Contact{}}
	s.Server = httptest.NewServer(http.HandlerFunc(s.handle))
	return s
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Requests = append(s.Requests, r.Method+" "+r.URL.Path)
	if r.Header.Get("Authorization") != "Bearer "+Token {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"type": "error.list", "errors": []map[string]string{{"code": "unauthorized", "message": "Access Token Invalid"}}})
		return
	}
	if r.Header.Get("Intercom-Version") == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"errors": []map[string]string{{"message": "missing version"}}})
		return
	}
	if s.RateLimitNext > 0 {
		s.RateLimitNext--
		w.Header().Set("Retry-After", "1")
		writeJSON(w, http.StatusTooManyRequests, map[string]any{"errors": []map[string]string{{"message": "rate limited"}}})
		return
	}
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/me":
		writeJSON(w, http.StatusOK, map[string]any{"type": "admin", "id": "991", "name": "Ada Admin", "email": "ada@acme.test", "app": map[string]any{"name": "Acme", "id_code": "abc123"}})
	case r.Method == http.MethodGet && r.URL.Path == "/help_center/collections":
		writePage(w, r, s.Collections)
	case r.Method == http.MethodGet && r.URL.Path == "/articles":
		writePage(w, r, s.Articles)
	case r.Method == http.MethodPost && r.URL.Path == "/conversations/search":
		s.search(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/conversations/"):
		id := strings.TrimPrefix(r.URL.Path, "/conversations/")
		for _, conversation := range s.Conversations {
			if conversation.ID == id {
				writeJSON(w, http.StatusOK, conversation)
				return
			}
		}
		writeJSON(w, http.StatusNotFound, map[string]any{"errors": []map[string]string{{"message": "not found"}}})
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/contacts/"):
		contact, ok := s.Contacts[strings.TrimPrefix(r.URL.Path, "/contacts/")]
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]any{"errors": []map[string]string{{"message": "not found"}}})
			return
		}
		writeJSON(w, http.StatusOK, contact)
	default:
		writeJSON(w, http.StatusNotFound, map[string]any{"errors": []map[string]string{{"message": "unknown route"}}})
	}
}

func writePage[T any](w http.ResponseWriter, r *http.Request, items []T) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 50
	}
	start := min((page-1)*perPage, len(items))
	end := min(start+perPage, len(items))
	totalPages := (len(items) + perPage - 1) / perPage
	writeJSON(w, http.StatusOK, map[string]any{
		"type": "list", "data": items[start:end], "total_count": len(items),
		"pages": map[string]any{"page": page, "per_page": perPage, "total_pages": totalPages},
	})
}

func (s *Server) search(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Query struct {
			Value int64 `json:"value"`
		} `json:"query"`
		Pagination struct {
			PerPage       int    `json:"per_page"`
			StartingAfter string `json:"starting_after"`
		} `json:"pagination"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"errors": []map[string]string{{"message": "bad body"}}})
		return
	}
	matching := make([]intercom.Conversation, 0, len(s.Conversations))
	for _, conversation := range s.Conversations {
		if conversation.CreatedAt > body.Query.Value {
			matching = append(matching, conversation)
		}
	}
	slices.SortStableFunc(matching, func(a, b intercom.Conversation) int { return int(a.CreatedAt - b.CreatedAt) })
	start := 0
	if body.Pagination.StartingAfter != "" {
		start, _ = strconv.Atoi(body.Pagination.StartingAfter)
	}
	perPage := max(body.Pagination.PerPage, 1)
	end := min(start+perPage, len(matching))
	refs := make([]intercom.ConversationRef, 0, end-start)
	for _, conversation := range matching[start:end] {
		refs = append(refs, intercom.ConversationRef{ID: conversation.ID, State: conversation.State, CreatedAt: conversation.CreatedAt, UpdatedAt: conversation.UpdatedAt})
	}
	pages := map[string]any{"type": "pages", "per_page": perPage, "total_pages": (len(matching) + perPage - 1) / perPage}
	if end < len(matching) {
		pages["next"] = map[string]any{"page": end/perPage + 1, "starting_after": strconv.Itoa(end)}
	}
	writeJSON(w, http.StatusOK, map[string]any{"type": "conversation.list", "conversations": refs, "total_count": len(matching), "pages": pages})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
