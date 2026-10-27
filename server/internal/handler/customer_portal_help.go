package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/service"
)

// HelpArticles lists the help articles for signed-in customers.
func (h *CustomerPortalHandler) HelpArticles(w http.ResponseWriter, r *http.Request) {
	access := h.authorized(w, r)
	if access == nil {
		return
	}
	articles, err := h.portal.HelpArticles(r.Context(), &access.Workspace, r.URL.Query().Get("locale"))
	if err != nil {
		writePortalError(w, r, access.Workspace.ID, err, "unable to load help articles")
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	writeJSON(w, http.StatusOK, map[string]any{"articles": articles})
}

// HelpArticle returns one help article for signed-in customers.
func (h *CustomerPortalHandler) HelpArticle(w http.ResponseWriter, r *http.Request) {
	access := h.authorized(w, r)
	if access == nil {
		return
	}
	article, err := h.portal.HelpArticle(r.Context(), &access.Workspace, chi.URLParam(r, "articleId"), r.URL.Query().Get("locale"))
	if errors.Is(err, service.ErrPortalHelpArticleNotFound) {
		writeError(w, http.StatusNotFound, "article not found")
		return
	}
	if err != nil {
		writePortalError(w, r, access.Workspace.ID, err, "unable to load the article")
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	writeJSON(w, http.StatusOK, article)
}

// HelpSearch searches the help articles for signed-in customers.
func (h *CustomerPortalHandler) HelpSearch(w http.ResponseWriter, r *http.Request) {
	access := h.authorized(w, r)
	if access == nil {
		return
	}
	articles, err := h.portal.SearchHelpArticles(r.Context(), &access.Workspace, r.URL.Query().Get("q"), r.URL.Query().Get("locale"))
	if errors.Is(err, context.Canceled) {
		return
	}
	if err != nil {
		writePortalError(w, r, access.Workspace.ID, err, "unable to search help articles")
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	writeJSON(w, http.StatusOK, map[string]any{"articles": articles})
}

// signedIn reports whether the request carries a valid portal session. A
// failed check counts as signed out.
func (h *CustomerPortalHandler) signedIn(r *http.Request, ws *service.PortalWorkspace) bool {
	secret := portalSessionCookie(r)
	if secret == "" {
		return false
	}
	access, err := h.portal.Authenticate(r.Context(), ws, secret)
	return err == nil && access != nil
}
