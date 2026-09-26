package router

import (
	"github.com/go-chi/chi/v5"
	"net/http"
)

// EditionRoutes registers optional edition endpoints at the host's existing
// authorization boundaries. A community router leaves it nil.
type EditionRoutes interface {
	RegisterPublic(chi.Router)
	RegisterAuthenticated(chi.Router)
	RegisterWorkspace(chi.Router)
	RequireActiveWorkspace(http.Handler) http.Handler
}
