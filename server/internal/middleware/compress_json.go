package middleware

import (
	"net/http"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

// CompressJSON compresses JSON responses when the client advertises a
// supported content encoding. It is intended for large collection routes.
func CompressJSON(next http.Handler) http.Handler {
	return chimiddleware.Compress(5, "application/json")(next)
}
