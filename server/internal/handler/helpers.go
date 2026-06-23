package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
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

func writeBillingAwareError(w http.ResponseWriter, fallbackStatus int, err error) {
	if err == nil {
		return
	}
	message := err.Error()
	var entitlementErr *service.EntitlementError
	if errors.As(err, &entitlementErr) {
		writeError(w, http.StatusPaymentRequired, message)
		return
	}
	normalized := strings.ToLower(message)
	if strings.Contains(normalized, "requires the growth plan") ||
		strings.Contains(normalized, "ai usage exhausted") ||
		strings.Contains(normalized, "workspace is locked") ||
		strings.Contains(normalized, "extra ai usage is not available") {
		writeError(w, http.StatusPaymentRequired, message)
		return
	}
	writeError(w, fallbackStatus, message)
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
