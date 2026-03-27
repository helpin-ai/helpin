package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/auth"
	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// DocsHandler handles HTTP requests for the Docs module.
type DocsHandler struct {
	spaceSvc       *service.DocsSpaceService
	collectionSvc  *service.DocsCollectionService
	documentSvc    *service.DocsDocumentService
	contentSvc     *service.DocsContentService
	versionSvc     *service.DocsVersionService
	linkSvc        *service.DocsLinkService
	helpcenterSvc  *service.DocsHelpcenterService
	translationSvc *service.DocsHelpcenterTranslationService
	searchSvc      *service.DocsSearchService
	importService  *service.DocsImportService
	embeddingSvc   *service.DocsEmbeddingService
	agentService   *service.AgentService
	jwtManager     *auth.JWTManager
}

// NewDocsHandler creates a new DocsHandler.
func NewDocsHandler(
	spaceSvc *service.DocsSpaceService,
	collectionSvc *service.DocsCollectionService,
	documentSvc *service.DocsDocumentService,
	contentSvc *service.DocsContentService,
	versionSvc *service.DocsVersionService,
	linkSvc *service.DocsLinkService,
	helpcenterSvc *service.DocsHelpcenterService,
	translationSvc *service.DocsHelpcenterTranslationService,
	searchSvc *service.DocsSearchService,
	importService *service.DocsImportService,
	embeddingSvc *service.DocsEmbeddingService,
	agentService *service.AgentService,
	jwtManager *auth.JWTManager,
) *DocsHandler {
	return &DocsHandler{
		spaceSvc:       spaceSvc,
		collectionSvc:  collectionSvc,
		documentSvc:    documentSvc,
		contentSvc:     contentSvc,
		versionSvc:     versionSvc,
		linkSvc:        linkSvc,
		helpcenterSvc:  helpcenterSvc,
		translationSvc: translationSvc,
		searchSvc:      searchSvc,
		importService:  importService,
		embeddingSvc:   embeddingSvc,
		agentService:   agentService,
		jwtManager:     jwtManager,
	}
}

// ─── Spaces ─────────────────────────────────────────────────────────────────

func (h *DocsHandler) ListSpaces(w http.ResponseWriter, r *http.Request) {
	wsID := middleware.GetWorkspaceID(r.Context())
	actor := authorization.GetActor(r.Context())
	spaces, err := h.spaceSvc.List(r.Context(), wsID, actor)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, spaces)
}

func (h *DocsHandler) CreateSpace(w http.ResponseWriter, r *http.Request) {
	wsID := middleware.GetWorkspaceID(r.Context())
	userID := middleware.GetUserID(r.Context())
	var req model.CreateDocsSpaceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	space, err := h.spaceSvc.Create(r.Context(), wsID, req, userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, space)
}

func (h *DocsHandler) GetSpace(w http.ResponseWriter, r *http.Request) {
	actor := authorization.GetActor(r.Context())
	space, err := h.spaceSvc.Get(r.Context(), chi.URLParam(r, "spaceId"), actor)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if space == nil {
		writeError(w, http.StatusNotFound, "space not found")
		return
	}
	writeJSON(w, http.StatusOK, space)
}

func (h *DocsHandler) UpdateSpace(w http.ResponseWriter, r *http.Request) {
	var req model.UpdateDocsSpaceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	space, err := h.spaceSvc.Update(r.Context(), chi.URLParam(r, "spaceId"), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, space)
}

func (h *DocsHandler) DeleteSpace(w http.ResponseWriter, r *http.Request) {
	if err := h.spaceSvc.Delete(r.Context(), chi.URLParam(r, "spaceId")); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *DocsHandler) RestoreSpace(w http.ResponseWriter, r *http.Request) {
	space, err := h.spaceSvc.Restore(r.Context(), chi.URLParam(r, "spaceId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, space)
}

// ─── Collections ────────────────────────────────────────────────────────────

func (h *DocsHandler) ListCollections(w http.ResponseWriter, r *http.Request) {
	colls, err := h.collectionSvc.List(r.Context(), chi.URLParam(r, "spaceId"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, colls)
}

func (h *DocsHandler) CreateCollection(w http.ResponseWriter, r *http.Request) {
	wsID := middleware.GetWorkspaceID(r.Context())
	userID := middleware.GetUserID(r.Context())
	spaceID := chi.URLParam(r, "spaceId")
	var req model.CreateDocsCollectionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	coll, err := h.collectionSvc.Create(r.Context(), wsID, spaceID, req, userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, coll)
}

func (h *DocsHandler) UpdateCollection(w http.ResponseWriter, r *http.Request) {
	var req model.UpdateDocsCollectionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	coll, err := h.collectionSvc.Update(r.Context(), chi.URLParam(r, "collectionId"), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, coll)
}

func (h *DocsHandler) DeleteCollection(w http.ResponseWriter, r *http.Request) {
	if err := h.collectionSvc.Delete(r.Context(), chi.URLParam(r, "collectionId")); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *DocsHandler) RestoreCollection(w http.ResponseWriter, r *http.Request) {
	coll, err := h.collectionSvc.Restore(r.Context(), chi.URLParam(r, "collectionId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, coll)
}

// ─── Documents ──────────────────────────────────────────────────────────────

func (h *DocsHandler) ListDocuments(w http.ResponseWriter, r *http.Request) {
	wsID := middleware.GetWorkspaceID(r.Context())
	userID := middleware.GetUserID(r.Context())
	actor := authorization.GetActor(r.Context())
	q := r.URL.Query()
	spaceID := ptrIfSet(q.Get("space_id"))
	collectionID := ptrIfSet(q.Get("collection_id"))
	status := ptrIfSet(q.Get("status"))
	teamID := ptrIfSet(q.Get("team_id"))
	includeArchived := q.Get("include_archived") == "true"

	role := ""
	if actor != nil {
		role = actor.Role
	}

	statusVal := ""
	if status != nil {
		statusVal = *status
	}
	slog.Info("[DEBUG] ListDocuments called",
		"workspace_id", wsID,
		"status_filter", statusVal,
		"include_archived", includeArchived,
		"space_id", q.Get("space_id"),
		"raw_query", r.URL.RawQuery,
	)

	docs, err := h.documentSvc.List(r.Context(), wsID, spaceID, collectionID, status, teamID, userID, role, includeArchived)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	slog.Info("[DEBUG] ListDocuments result",
		"status_filter", statusVal,
		"count", len(docs),
	)

	writeJSON(w, http.StatusOK, docs)
}

func (h *DocsHandler) CreateDocument(w http.ResponseWriter, r *http.Request) {
	wsID := middleware.GetWorkspaceID(r.Context())
	userID := middleware.GetUserID(r.Context())
	actor := authorization.GetActor(r.Context())
	var req model.CreateDocsDocumentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	// Default owner to the creator's membership ID.
	if req.OwnerID == nil && actor != nil && actor.WorkspaceMemberID != "" {
		req.OwnerID = &actor.WorkspaceMemberID
	}
	doc, err := h.documentSvc.Create(r.Context(), wsID, req, userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, doc)
}

func (h *DocsHandler) GetDocument(w http.ResponseWriter, r *http.Request) {
	docID := chi.URLParam(r, "docId")
	doc, err := h.documentSvc.Get(r.Context(), docID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if doc == nil {
		writeError(w, http.StatusNotFound, "document not found")
		return
	}
	// Enrich with help center slug if available.
	if art, _ := h.helpcenterSvc.GetArticle(r.Context(), docID); art != nil && art.Slug != "" {
		doc.HCSlug = art.Slug
	}
	writeJSON(w, http.StatusOK, doc)
}

func (h *DocsHandler) UpdateDocument(w http.ResponseWriter, r *http.Request) {
	docID := chi.URLParam(r, "docId")
	var req model.UpdateDocsDocumentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	doc, err := h.documentSvc.Update(r.Context(), docID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	h.queueEmbeddingSync(r.Context(), docID)
	writeJSON(w, http.StatusOK, doc)
}

func (h *DocsHandler) DeleteDocument(w http.ResponseWriter, r *http.Request) {
	docID := chi.URLParam(r, "docId")
	doc, _ := h.documentSvc.Get(r.Context(), docID)
	if err := h.documentSvc.Delete(r.Context(), docID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if doc != nil && h.embeddingSvc != nil {
		_ = h.embeddingSvc.QueueSpaceSync(r.Context(), doc.WorkspaceID, doc.SpaceID)
	}
	if h.agentService != nil {
		_ = h.agentService.ClearEpicSpecForDocument(r.Context(), docID)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *DocsHandler) RestoreDocument(w http.ResponseWriter, r *http.Request) {
	doc, err := h.documentSvc.Restore(r.Context(), chi.URLParam(r, "docId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, doc)
}

func (h *DocsHandler) UnpublishDocument(w http.ResponseWriter, r *http.Request) {
	docID := chi.URLParam(r, "docId")

	// If externally published, unpublish first.
	art, _ := h.helpcenterSvc.GetArticle(r.Context(), docID)
	if art != nil && art.PublicPublishedAt != nil {
		_ = h.helpcenterSvc.UnpublishExternally(r.Context(), docID)
	}

	doc, err := h.documentSvc.Unpublish(r.Context(), docID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	h.queueEmbeddingSync(r.Context(), docID)
	writeJSON(w, http.StatusOK, doc)
}

func (h *DocsHandler) ArchiveDocument(w http.ResponseWriter, r *http.Request) {
	docID := chi.URLParam(r, "docId")

	// If externally published, unpublish first.
	art, _ := h.helpcenterSvc.GetArticle(r.Context(), docID)
	if art != nil && art.PublicPublishedAt != nil {
		_ = h.helpcenterSvc.UnpublishExternally(r.Context(), docID)
	}

	doc, err := h.documentSvc.Archive(r.Context(), docID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	h.queueEmbeddingSync(r.Context(), docID)
	writeJSON(w, http.StatusOK, doc)
}

func (h *DocsHandler) UnarchiveDocument(w http.ResponseWriter, r *http.Request) {
	doc, err := h.documentSvc.Unarchive(r.Context(), chi.URLParam(r, "docId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, doc)
}

func (h *DocsHandler) MoveDocument(w http.ResponseWriter, r *http.Request) {
	docID := chi.URLParam(r, "docId")
	var req model.MoveDocsDocumentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	doc, err := h.documentSvc.Move(r.Context(), docID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// If moved to a non-external space, unpublish externally.
	space, _ := h.spaceSvc.GetUnfiltered(r.Context(), doc.SpaceID)
	if space != nil && space.Type != model.SpaceTypeExternalCapable {
		_ = h.helpcenterSvc.UnpublishExternally(r.Context(), docID)
	}
	h.queueEmbeddingSync(r.Context(), docID)

	writeJSON(w, http.StatusOK, doc)
}

// ─── Content ────────────────────────────────────────────────────────────────

func (h *DocsHandler) GetContent(w http.ResponseWriter, r *http.Request) {
	content, err := h.contentSvc.Get(r.Context(), chi.URLParam(r, "docId"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if content == nil {
		writeJSON(w, http.StatusOK, nil)
		return
	}
	writeJSON(w, http.StatusOK, content)
}

// ReorderSpaces reorders spaces within a section.
func (h *DocsHandler) ReorderSpaces(w http.ResponseWriter, r *http.Request) {
	wsID := r.URL.Query().Get("workspace_id")
	var req model.ReorderDocsSpacesRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.spaceSvc.ReorderSpaces(r.Context(), wsID, req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "order updated"})
}

// ReorderCollections reorders collections within a space.
func (h *DocsHandler) ReorderCollections(w http.ResponseWriter, r *http.Request) {
	spaceID := chi.URLParam(r, "spaceId")
	var req model.ReorderDocsCollectionsRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.collectionSvc.ReorderCollections(r.Context(), spaceID, req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "order updated"})
}

// ReorderDocuments reorders documents within a bucket.
func (h *DocsHandler) ReorderDocuments(w http.ResponseWriter, r *http.Request) {
	spaceID := chi.URLParam(r, "spaceId")
	var req model.ReorderDocsDocumentsRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.documentSvc.ReorderDocuments(r.Context(), spaceID, req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "order updated"})
}

// GeneratePreviewToken creates a short-lived JWT for previewing a document in the help center app.
func (h *DocsHandler) GeneratePreviewToken(w http.ResponseWriter, r *http.Request) {
	wsID := r.URL.Query().Get("workspace_id")
	docID := chi.URLParam(r, "docId")

	doc, err := h.documentSvc.Get(r.Context(), docID)
	if err != nil || doc == nil || doc.WorkspaceID != wsID {
		writeError(w, http.StatusNotFound, "document not found")
		return
	}

	cfg, err := h.helpcenterSvc.GetConfig(r.Context(), wsID)
	if err != nil || cfg == nil {
		writeError(w, http.StatusNotFound, "help center not configured")
		return
	}

	token, err := h.jwtManager.GeneratePreviewToken(docID, wsID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate preview token")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"token":     token,
		"subdomain": cfg.Subdomain,
	})
}

// PublicPreviewArticle returns a preview of a document for the help center app.
// Requires a valid preview JWT token as query parameter.
func (h *DocsHandler) PublicPreviewArticle(w http.ResponseWriter, r *http.Request) {
	cfg := h.resolveSubdomain(w, r)
	if cfg == nil {
		return
	}

	docID := chi.URLParam(r, "docId")
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		writeError(w, http.StatusUnauthorized, "preview token required")
		return
	}

	claims, err := h.jwtManager.ValidatePreviewToken(tokenStr)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid or expired preview token")
		return
	}

	if claims.DocID != docID || claims.WorkspaceID != cfg.WorkspaceID {
		writeError(w, http.StatusForbidden, "token does not match document")
		return
	}

	resp, err := h.helpcenterSvc.PreviewArticleHTML(r.Context(), cfg.WorkspaceID, docID)
	if err != nil {
		writeError(w, http.StatusNotFound, "document not found")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *DocsHandler) SaveContent(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	docID := chi.URLParam(r, "docId")

	// Reject content saves on locked documents.
	doc, err := h.documentSvc.Get(r.Context(), docID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if doc != nil && doc.IsLocked {
		writeError(w, http.StatusForbidden, "document is locked and cannot be modified")
		return
	}

	var req model.SaveDocsContentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	content, err := h.contentSvc.Save(r.Context(), docID, req.Content)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Check for periodic auto-snapshot (non-blocking).
	go h.versionSvc.MaybeAutoSnapshot(r.Context(), docID, userID)
	h.queueEmbeddingSync(r.Context(), docID)

	writeJSON(w, http.StatusOK, content)
}

// SaveMarkdownContent accepts raw Markdown from AI agents or external tools and
// stores it as a JSON envelope that the frontend auto-converts on first load.
func (h *DocsHandler) SaveMarkdownContent(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	docID := chi.URLParam(r, "docId")

	// Reject content saves on locked documents.
	doc, err := h.documentSvc.Get(r.Context(), docID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if doc != nil && doc.IsLocked {
		writeError(w, http.StatusForbidden, "document is locked and cannot be modified")
		return
	}

	var req model.SaveDocsMarkdownRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Markdown == "" {
		writeError(w, http.StatusBadRequest, "markdown field is required")
		return
	}

	// Wrap markdown in a JSON envelope the frontend Tiptap editor will detect and convert.
	envelope := map[string]string{"_markdown_source": req.Markdown}
	raw, _ := json.Marshal(envelope)

	content, err := h.contentSvc.Save(r.Context(), docID, raw)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	go h.versionSvc.MaybeAutoSnapshot(r.Context(), docID, userID)
	h.queueEmbeddingSync(r.Context(), docID)

	writeJSON(w, http.StatusOK, content)
}

// ─── Versions ───────────────────────────────────────────────────────────────

func (h *DocsHandler) ListVersions(w http.ResponseWriter, r *http.Request) {
	versions, err := h.versionSvc.List(r.Context(), chi.URLParam(r, "docId"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, versions)
}

func (h *DocsHandler) CreateVersion(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	var req model.CreateDocsVersionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	version, err := h.versionSvc.CreateSnapshot(r.Context(), chi.URLParam(r, "docId"), userID, req.SnapshotLabel)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, version)
}

func (h *DocsHandler) GetVersion(w http.ResponseWriter, r *http.Request) {
	versionID := chi.URLParam(r, "versionId")
	version, err := h.versionSvc.Get(r.Context(), versionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if version == nil {
		writeError(w, http.StatusNotFound, "version not found")
		return
	}
	writeJSON(w, http.StatusOK, version)
}

func (h *DocsHandler) UpdateVersionLabel(w http.ResponseWriter, r *http.Request) {
	versionID := chi.URLParam(r, "versionId")
	var req model.UpdateDocsVersionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	version, err := h.versionSvc.UpdateLabel(r.Context(), versionID, req.SnapshotLabel)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, version)
}

func (h *DocsHandler) RevertVersion(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	docID := chi.URLParam(r, "docId")
	versionID := chi.URLParam(r, "versionId")
	content, err := h.versionSvc.Revert(r.Context(), docID, versionID, userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, content)
}

// ─── Publish (internal status) ──────────────────────────────────────────────

func (h *DocsHandler) PublishDocument(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	docID := chi.URLParam(r, "docId")

	// Accept optional slug for external publish.
	var body struct {
		Slug string `json:"slug"`
	}
	_ = decodeJSON(r, &body)

	doc, err := h.documentSvc.Publish(r.Context(), docID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Snapshot on publish.
	_, _ = h.versionSvc.SnapshotOnPublish(r.Context(), docID, userID)

	// Auto-publish externally if document is in an external-capable space.
	pubSpace, _ := h.spaceSvc.GetUnfiltered(r.Context(), doc.SpaceID)
	if pubSpace != nil && pubSpace.Type == model.SpaceTypeExternalCapable {
		if err := h.helpcenterSvc.PublishExternally(r.Context(), docID, body.Slug); err != nil {
			slog.Warn("PublishExternally failed", "doc_id", docID, "error", err)
		}
	}
	h.queueEmbeddingSync(r.Context(), docID)

	// Re-fetch to include updated helpcenter article data.
	doc, _ = h.documentSvc.Get(r.Context(), docID)
	if art, _ := h.helpcenterSvc.GetArticle(r.Context(), docID); art != nil && art.Slug != "" {
		doc.HCSlug = art.Slug
	}
	writeJSON(w, http.StatusOK, doc)
}

// ─── External Publish/Unpublish ─────────────────────────────────────────────

func (h *DocsHandler) PublishExternally(w http.ResponseWriter, r *http.Request) {
	docID := chi.URLParam(r, "docId")
	var body struct {
		Slug string `json:"slug"`
	}
	_ = decodeJSON(r, &body)

	if err := h.helpcenterSvc.PublishExternally(r.Context(), docID, body.Slug); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	h.queueEmbeddingSync(r.Context(), docID)
	writeJSON(w, http.StatusOK, map[string]string{"status": "published"})
}

func (h *DocsHandler) UnpublishExternally(w http.ResponseWriter, r *http.Request) {
	docID := chi.URLParam(r, "docId")
	if err := h.helpcenterSvc.UnpublishExternally(r.Context(), docID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	h.queueEmbeddingSync(r.Context(), docID)
	writeJSON(w, http.StatusOK, map[string]string{"status": "unpublished"})
}

func (h *DocsHandler) queueEmbeddingSync(ctx context.Context, docID string) {
	if h == nil || h.embeddingSvc == nil || strings.TrimSpace(docID) == "" {
		return
	}
	if err := h.embeddingSvc.QueueDocumentSync(ctx, docID); err != nil {
		slog.Warn("queue docs embedding sync failed", "doc_id", docID, "error", err)
	}
}

// ─── Links ──────────────────────────────────────────────────────────────────

func (h *DocsHandler) ListLinks(w http.ResponseWriter, r *http.Request) {
	links, err := h.linkSvc.ListByDocument(r.Context(), chi.URLParam(r, "docId"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, links)
}

func (h *DocsHandler) CreateLink(w http.ResponseWriter, r *http.Request) {
	wsID := middleware.GetWorkspaceID(r.Context())
	userID := middleware.GetUserID(r.Context())
	docID := chi.URLParam(r, "docId")
	var req model.CreateDocsLinkRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	link, err := h.linkSvc.Create(r.Context(), wsID, docID, req, userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, link)
}

func (h *DocsHandler) DeleteLink(w http.ResponseWriter, r *http.Request) {
	if err := h.linkSvc.Delete(r.Context(), chi.URLParam(r, "linkId")); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *DocsHandler) ListLinkedDocs(w http.ResponseWriter, r *http.Request) {
	wsID := middleware.GetWorkspaceID(r.Context())
	objectType := chi.URLParam(r, "objectType")
	objectID := chi.URLParam(r, "objectId")
	links, err := h.linkSvc.ListByObject(r.Context(), wsID, objectType, objectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, links)
}

// ─── Search ─────────────────────────────────────────────────────────────────

func (h *DocsHandler) Search(w http.ResponseWriter, r *http.Request) {
	wsID := middleware.GetWorkspaceID(r.Context())
	actor := authorization.GetActor(r.Context())
	q := r.URL.Query()
	query := q.Get("q")
	status := ptrIfSet(q.Get("status"))
	limit := 50
	if l := q.Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil {
			limit = n
		}
	}

	// Get accessible space IDs for the actor (nil = no filtering for admin/owner)
	spaceIDs, err := h.spaceSvc.AccessibleSpaceIDs(r.Context(), wsID, actor)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	results, err := h.searchSvc.Search(r.Context(), wsID, query, spaceIDs, status, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, results)
}

// ─── Help Center Config ─────────────────────────────────────────────────────

func (h *DocsHandler) GetHelpcenterConfig(w http.ResponseWriter, r *http.Request) {
	wsID := middleware.GetWorkspaceID(r.Context())
	cfg, err := h.helpcenterSvc.GetConfig(r.Context(), wsID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (h *DocsHandler) UpdateHelpcenterConfig(w http.ResponseWriter, r *http.Request) {
	wsID := middleware.GetWorkspaceID(r.Context())
	var req model.UpdateDocsHelpcenterConfigRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	cfg, err := h.helpcenterSvc.UpsertConfig(r.Context(), wsID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

// UploadHelpcenterAsset handles POST /docs/helpcenter/upload?type={logo|logo_dark|favicon}.
func (h *DocsHandler) UploadHelpcenterAsset(w http.ResponseWriter, r *http.Request) {
	wsID := middleware.GetWorkspaceID(r.Context())

	assetType := r.URL.Query().Get("type")
	if assetType != "logo" && assetType != "logo_dark" && assetType != "favicon" {
		writeError(w, http.StatusBadRequest, "type must be 'logo', 'logo_dark', or 'favicon'")
		return
	}

	maxSize := int64(2 << 20) // 2 MB
	if err := r.ParseMultipartForm(maxSize); err != nil {
		writeError(w, http.StatusBadRequest, "file too large (max 2MB)")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing file")
		return
	}
	defer file.Close()

	ct := header.Header.Get("Content-Type")
	allowed := map[string]bool{
		"image/png": true, "image/jpeg": true, "image/webp": true,
		"image/svg+xml": true, "image/x-icon": true, "image/vnd.microsoft.icon": true,
	}
	if !allowed[ct] {
		writeError(w, http.StatusBadRequest, "only PNG, JPEG, WebP, SVG, and ICO images are allowed")
		return
	}

	publicURL, err := h.helpcenterSvc.UploadAsset(r.Context(), wsID, assetType, ct, header.Size, file)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"url": publicURL})
}

// ImportExternalImage handles POST /docs/images/import.
func (h *DocsHandler) ImportExternalImage(w http.ResponseWriter, r *http.Request) {
	wsID := middleware.GetWorkspaceID(r.Context())

	var req model.ImportDocsExternalImageRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	publicURL, err := h.importService.ImportExternalImage(r.Context(), wsID, strings.TrimSpace(req.ImageURL))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, model.ImportDocsExternalImageResponse{URL: publicURL})
}

// ─── Article Feedback ───────────────────────────────────────────────────────

func (h *DocsHandler) SubmitArticleFeedback(w http.ResponseWriter, r *http.Request) {
	docID := chi.URLParam(r, "docId")
	var req model.DocsArticleFeedbackRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.helpcenterSvc.SubmitFeedback(r.Context(), docID, req); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ─── Redirect Management ────────────────────────────────────────────────────

// ListRedirects handles GET /api/docs/redirects.
func (h *DocsHandler) ListRedirects(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.URL.Query().Get("workspace_id")
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	filter := model.DocsRedirectFilter{
		Search: r.URL.Query().Get("search"),
		Type:   r.URL.Query().Get("type"),
	}
	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			filter.Page = v
		}
	}
	if pp := r.URL.Query().Get("per_page"); pp != "" {
		if v, err := strconv.Atoi(pp); err == nil {
			filter.PerPage = v
		}
	}
	items, total, err := h.helpcenterSvc.ListRedirects(r.Context(), workspaceID, filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.DocsRedirectListResponse{Items: items, Total: total})
}

// CreateRedirect handles POST /api/docs/redirects.
func (h *DocsHandler) CreateRedirect(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.URL.Query().Get("workspace_id")
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	var req model.CreateDocsRedirectRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	redirect, err := h.helpcenterSvc.CreateRedirect(r.Context(), workspaceID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, redirect)
}

// DeleteRedirect handles DELETE /api/docs/redirects/{id}.
// UpdateRedirect updates a redirect's fields.
func (h *DocsHandler) UpdateRedirect(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req model.UpdateDocsRedirectRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	redirect, err := h.helpcenterSvc.UpdateRedirect(r.Context(), id, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, redirect)
}

func (h *DocsHandler) DeleteRedirect(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.helpcenterSvc.DeleteRedirect(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "redirect deleted"})
}

// ─── Public Help Center routes ──────────────────────────────────────────────

// resolveSubdomain resolves a subdomain to its help center config.
// Returns nil config with a 404 written if not found or not published.
func (h *DocsHandler) resolveSubdomain(w http.ResponseWriter, r *http.Request) *model.DocsHelpcenterConfig {
	subdomain := chi.URLParam(r, "subdomain")
	cfg, err := h.helpcenterSvc.ResolveConfig(r.Context(), subdomain)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return nil
	}
	if cfg == nil {
		writeError(w, http.StatusNotFound, "help center not found")
		return nil
	}
	return cfg
}

// VerifyDomain checks if a domain is registered for on_demand_tls (Caddy).
func (h *DocsHandler) VerifyDomain(w http.ResponseWriter, r *http.Request) {
	domain := r.URL.Query().Get("domain")
	if domain == "" {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	// Allow our own domains always.
	if domain == "helpcenter.helpin.ai" || domain == "helpcenter-stage.helpin.ai" {
		w.WriteHeader(http.StatusOK)
		return
	}
	// Check if domain is registered as a custom_domain in our DB.
	cfg, err := h.helpcenterSvc.GetConfigByCustomDomain(r.Context(), domain)
	if err != nil || cfg == nil {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *DocsHandler) PublicGetConfig(w http.ResponseWriter, r *http.Request) {
	cfg := h.resolveSubdomain(w, r)
	if cfg == nil {
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (h *DocsHandler) PublicGetSpaces(w http.ResponseWriter, r *http.Request) {
	cfg := h.resolveSubdomain(w, r)
	if cfg == nil {
		return
	}
	spaces, err := h.helpcenterSvc.ListPublicSpaces(r.Context(), cfg.WorkspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, spaces)
}

func (h *DocsHandler) PublicGetSpaceNavigation(w http.ResponseWriter, r *http.Request) {
	cfg := h.resolveSubdomain(w, r)
	if cfg == nil {
		return
	}
	spaceSlug := chi.URLParam(r, "spaceSlug")
	nav, err := h.helpcenterSvc.GetSpaceNavigation(r.Context(), cfg.WorkspaceID, spaceSlug)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, nav)
}

func (h *DocsHandler) PublicGetSpaceArticle(w http.ResponseWriter, r *http.Request) {
	cfg := h.resolveSubdomain(w, r)
	if cfg == nil {
		return
	}
	spaceSlug := chi.URLParam(r, "spaceSlug")
	articleSlug := chi.URLParam(r, "articleSlug")

	article, err := h.helpcenterSvc.GetPublicArticle(r.Context(), cfg.WorkspaceID, spaceSlug, articleSlug)
	if err != nil {
		writeError(w, http.StatusNotFound, "article not found")
		return
	}
	writeJSON(w, http.StatusOK, article)
}

// PublicGetCollectionPage returns a collection and its published articles for the public help center.
func (h *DocsHandler) PublicGetCollectionPage(w http.ResponseWriter, r *http.Request) {
	cfg := h.resolveSubdomain(w, r)
	if cfg == nil {
		return
	}
	collectionSlug := chi.URLParam(r, "collectionSlug")
	coll, articles, err := h.helpcenterSvc.GetPublicCollection(r.Context(), cfg.WorkspaceID, collectionSlug)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if coll == nil {
		writeError(w, http.StatusNotFound, "collection not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"collection": coll,
		"articles":   articles,
	})
}

// PublicGetCanonicalArticle returns a public article by canonical collection/article slug path.
func (h *DocsHandler) PublicGetCanonicalArticle(w http.ResponseWriter, r *http.Request) {
	cfg := h.resolveSubdomain(w, r)
	if cfg == nil {
		return
	}
	collectionSlug := chi.URLParam(r, "collectionSlug")
	articleSlug := chi.URLParam(r, "articleSlug")
	article, err := h.helpcenterSvc.GetPublicArticleByCanonicalPath(r.Context(), cfg.WorkspaceID, collectionSlug, articleSlug)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if article == nil {
		writeError(w, http.StatusNotFound, "article not found")
		return
	}
	writeJSON(w, http.StatusOK, article)
}

// PublicResolvePath resolves a legacy or imported URL path to a redirect target.
func (h *DocsHandler) PublicResolvePath(w http.ResponseWriter, r *http.Request) {
	cfg := h.resolveSubdomain(w, r)
	if cfg == nil {
		return
	}
	// Extract the catch-all path after /resolve/.
	path := chi.URLParam(r, "*")
	if path == "" {
		writeError(w, http.StatusNotFound, "path required")
		return
	}
	redirect, err := h.helpcenterSvc.ResolvePublicPath(r.Context(), cfg.WorkspaceID, path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if redirect == nil {
		writeError(w, http.StatusNotFound, "no redirect found")
		return
	}
	target := "/" + redirect.TargetCollectionSlug
	if redirect.TargetArticleSlug != nil && *redirect.TargetArticleSlug != "" {
		target += "/" + *redirect.TargetArticleSlug
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"redirect": true,
		"target":   target,
		"status":   301,
	})
}

func (h *DocsHandler) PublicSearchArticles(w http.ResponseWriter, r *http.Request) {
	cfg := h.resolveSubdomain(w, r)
	if cfg == nil {
		return
	}
	query := r.URL.Query().Get("q")
	spaceSlug := r.URL.Query().Get("space")
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil {
			limit = n
		}
	}

	// Resolve space slug to ID if provided.
	spaceID := ""
	if spaceSlug != "" {
		space, err := h.spaceSvc.GetBySlug(r.Context(), cfg.WorkspaceID, spaceSlug)
		if err == nil && space != nil {
			spaceID = space.ID
		}
	}

	results, err := h.searchSvc.PublicSearch(r.Context(), cfg.WorkspaceID, query, spaceID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, results)
}

func (h *DocsHandler) PublicSubmitFeedback(w http.ResponseWriter, r *http.Request) {
	cfg := h.resolveSubdomain(w, r)
	if cfg == nil {
		return
	}
	articleSlug := chi.URLParam(r, "articleSlug")
	spaceSlug := chi.URLParam(r, "spaceSlug")

	// Resolve article.
	article, err := h.helpcenterSvc.GetPublicArticle(r.Context(), cfg.WorkspaceID, spaceSlug, articleSlug)
	if err != nil || article == nil {
		writeError(w, http.StatusNotFound, "article not found")
		return
	}

	var req model.DocsArticleFeedbackRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.helpcenterSvc.SubmitFeedback(r.Context(), article.ID, req); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ─── Share Toggle ────────────────────────────────────────────────────────────

func (h *DocsHandler) ToggleDocShare(w http.ResponseWriter, r *http.Request) {
	docID := chi.URLParam(r, "docId")
	var req model.ToggleDocShareRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	doc, err := h.documentSvc.ToggleShare(r.Context(), docID, req.IsPubliclyShared)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, doc)
}

func (h *DocsHandler) ToggleDocLock(w http.ResponseWriter, r *http.Request) {
	docID := chi.URLParam(r, "docId")
	userID := middleware.GetUserID(r.Context())
	actor := authorization.GetActor(r.Context())
	var req model.ToggleDocLockRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	role := ""
	if actor != nil {
		role = actor.Role
	}
	doc, err := h.documentSvc.ToggleLock(r.Context(), docID, req.IsLocked, userID, role)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, doc)
}

// ─── Public Shared Document (no JWT) ────────────────────────────────────────

func (h *DocsHandler) PublicGetSharedDoc(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "shareToken")
	doc, err := h.documentSvc.GetByShareToken(r.Context(), token)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if doc == nil {
		writeError(w, http.StatusNotFound, "document not found or sharing disabled")
		return
	}

	content, _ := h.contentSvc.Get(r.Context(), doc.ID)

	writeJSON(w, http.StatusOK, model.PublicDocResponse{
		Document: doc,
		Content:  content,
	})
}

// ─── Helpers ────────────────────────────────────────────────────────────────

func ptrIfSet(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
