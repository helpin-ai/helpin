package handler

import (
	"encoding/json"
	"net/http"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		json.NewEncoder(w).Encode(data)
	}
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, model.APIError{Error: message})
}

func writeErrorCode(w http.ResponseWriter, status int, message, code string) {
	writeJSON(w, status, model.APIError{Error: message, Code: code})
}

// decodeJSON decodes a JSON request body into the given target.
func decodeJSON(r *http.Request, target interface{}) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(target)
}

// NoStoreOnWrites emits Cache-Control: no-store on non-idempotent requests
// (POST, PUT, PATCH, DELETE). Idempotent methods pass through so their own
// cache policies apply. Belt-and-braces protection against downstream proxies
// that might otherwise cache a mutation response.
func NoStoreOnWrites(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			w.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}
