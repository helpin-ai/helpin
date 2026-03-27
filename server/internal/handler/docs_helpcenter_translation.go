package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func (h *DocsHandler) GetHelpcenterLocales(w http.ResponseWriter, r *http.Request) {
	wsID := middleware.GetWorkspaceID(r.Context())
	cfg, err := h.translationSvc.GetLocales(r.Context(), wsID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (h *DocsHandler) UpdateHelpcenterLocales(w http.ResponseWriter, r *http.Request) {
	wsID := middleware.GetWorkspaceID(r.Context())
	var req model.UpdateDocsHelpcenterLocalesRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	cfg, err := h.translationSvc.UpdateLocales(r.Context(), wsID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (h *DocsHandler) ListArticleTranslations(w http.ResponseWriter, r *http.Request) {
	docID := chi.URLParam(r, "docId")
	translations, err := h.translationSvc.ListArticleTranslations(r.Context(), docID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, translations)
}

func (h *DocsHandler) ListSpaceTranslations(w http.ResponseWriter, r *http.Request) {
	spaceID := chi.URLParam(r, "spaceId")
	translations, err := h.translationSvc.ListSpaceTranslations(r.Context(), spaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, translations)
}

func (h *DocsHandler) UpsertSpaceTranslation(w http.ResponseWriter, r *http.Request) {
	spaceID := chi.URLParam(r, "spaceId")
	var req model.UpsertDocsHelpcenterSpaceTranslationRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	translation, err := h.translationSvc.UpsertSpaceTranslation(r.Context(), spaceID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, translation)
}

func (h *DocsHandler) PublishSpaceTranslation(w http.ResponseWriter, r *http.Request) {
	spaceID := chi.URLParam(r, "spaceId")
	locale := chi.URLParam(r, "locale")
	var body struct {
		Slug *string `json:"slug"`
	}
	_ = decodeJSON(r, &body)
	translation, err := h.translationSvc.PublishSpaceTranslation(r.Context(), spaceID, locale, body.Slug)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, translation)
}

func (h *DocsHandler) UnpublishSpaceTranslation(w http.ResponseWriter, r *http.Request) {
	spaceID := chi.URLParam(r, "spaceId")
	locale := chi.URLParam(r, "locale")
	translation, err := h.translationSvc.UnpublishSpaceTranslation(r.Context(), spaceID, locale)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, translation)
}

func (h *DocsHandler) MarkSpaceTranslationReviewed(w http.ResponseWriter, r *http.Request) {
	spaceID := chi.URLParam(r, "spaceId")
	locale := chi.URLParam(r, "locale")
	translation, err := h.translationSvc.MarkSpaceTranslationReviewed(r.Context(), spaceID, locale)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, translation)
}

func (h *DocsHandler) ListCollectionTranslations(w http.ResponseWriter, r *http.Request) {
	collectionID := chi.URLParam(r, "collectionId")
	translations, err := h.translationSvc.ListCollectionTranslations(r.Context(), collectionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, translations)
}

func (h *DocsHandler) UpsertCollectionTranslation(w http.ResponseWriter, r *http.Request) {
	collectionID := chi.URLParam(r, "collectionId")
	var req model.UpsertDocsHelpcenterCollectionTranslationRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	translation, err := h.translationSvc.UpsertCollectionTranslation(r.Context(), collectionID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, translation)
}

func (h *DocsHandler) PublishCollectionTranslation(w http.ResponseWriter, r *http.Request) {
	collectionID := chi.URLParam(r, "collectionId")
	locale := chi.URLParam(r, "locale")
	var body struct {
		Slug *string `json:"slug"`
	}
	_ = decodeJSON(r, &body)
	translation, err := h.translationSvc.PublishCollectionTranslation(r.Context(), collectionID, locale, body.Slug)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, translation)
}

func (h *DocsHandler) UnpublishCollectionTranslation(w http.ResponseWriter, r *http.Request) {
	collectionID := chi.URLParam(r, "collectionId")
	locale := chi.URLParam(r, "locale")
	translation, err := h.translationSvc.UnpublishCollectionTranslation(r.Context(), collectionID, locale)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, translation)
}

func (h *DocsHandler) MarkCollectionTranslationReviewed(w http.ResponseWriter, r *http.Request) {
	collectionID := chi.URLParam(r, "collectionId")
	locale := chi.URLParam(r, "locale")
	translation, err := h.translationSvc.MarkCollectionTranslationReviewed(r.Context(), collectionID, locale)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, translation)
}

func (h *DocsHandler) UpsertArticleTranslation(w http.ResponseWriter, r *http.Request) {
	docID := chi.URLParam(r, "docId")
	var req model.UpsertDocsHelpcenterArticleTranslationRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	translation, err := h.translationSvc.UpsertArticleTranslation(r.Context(), docID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, translation)
}

func (h *DocsHandler) PublishArticleTranslation(w http.ResponseWriter, r *http.Request) {
	docID := chi.URLParam(r, "docId")
	locale := chi.URLParam(r, "locale")
	var body struct {
		Slug *string `json:"slug"`
	}
	_ = decodeJSON(r, &body)
	translation, err := h.translationSvc.PublishArticleTranslation(r.Context(), docID, locale, body.Slug)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, translation)
}

func (h *DocsHandler) UpdateArticleTranslationSlug(w http.ResponseWriter, r *http.Request) {
	wsID := middleware.GetWorkspaceID(r.Context())
	docID := chi.URLParam(r, "docId")
	locale := chi.URLParam(r, "locale")
	var body struct {
		Slug string `json:"slug"`
	}
	if err := decodeJSON(r, &body); err != nil || body.Slug == "" {
		writeError(w, http.StatusBadRequest, "slug is required")
		return
	}
	if err := h.translationSvc.UpdateArticleTranslationSlug(r.Context(), wsID, docID, locale, body.Slug); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "translation slug updated"})
}

func (h *DocsHandler) UnpublishArticleTranslation(w http.ResponseWriter, r *http.Request) {
	docID := chi.URLParam(r, "docId")
	locale := chi.URLParam(r, "locale")
	translation, err := h.translationSvc.UnpublishArticleTranslation(r.Context(), docID, locale)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, translation)
}

func (h *DocsHandler) MarkArticleTranslationReviewed(w http.ResponseWriter, r *http.Request) {
	docID := chi.URLParam(r, "docId")
	locale := chi.URLParam(r, "locale")
	translation, err := h.translationSvc.MarkArticleTranslationReviewed(r.Context(), docID, locale)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, translation)
}

func (h *DocsHandler) GenerateArticleTranslationDraft(w http.ResponseWriter, r *http.Request) {
	docID := chi.URLParam(r, "docId")
	locale := chi.URLParam(r, "locale")
	translation, err := h.translationSvc.GenerateArticleTranslationDraft(r.Context(), docID, locale)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, translation)
}

// GenerateSpaceTranslation generates an AI translation for a space's metadata.
func (h *DocsHandler) GenerateSpaceTranslation(w http.ResponseWriter, r *http.Request) {
	spaceID := chi.URLParam(r, "spaceId")
	locale := chi.URLParam(r, "locale")
	translation, err := h.translationSvc.GenerateSpaceTranslation(r.Context(), spaceID, locale)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, translation)
}

// GenerateCollectionTranslation generates an AI translation for a collection's metadata.
func (h *DocsHandler) GenerateCollectionTranslation(w http.ResponseWriter, r *http.Request) {
	collectionID := chi.URLParam(r, "collectionId")
	locale := chi.URLParam(r, "locale")
	translation, err := h.translationSvc.GenerateCollectionTranslation(r.Context(), collectionID, locale)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, translation)
}
