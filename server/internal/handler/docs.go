package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// DocsHandler handles HTTP requests for the Docs module.
type DocsHandler struct {
	spaceSvc      *service.DocsSpaceService
	collectionSvc *service.DocsCollectionService
	documentSvc   *service.DocsDocumentService
	contentSvc    *service.DocsContentService
	versionSvc    *service.DocsVersionService
	linkSvc       *service.DocsLinkService
	helpcenterSvc *service.DocsHelpcenterService
	searchSvc     *service.DocsSearchService
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
	searchSvc *service.DocsSearchService,
) *DocsHandler {
	return &DocsHandler{
		spaceSvc:      spaceSvc,
		collectionSvc: collectionSvc,
		documentSvc:   documentSvc,
		contentSvc:    contentSvc,
		versionSvc:    versionSvc,
		linkSvc:       linkSvc,
		helpcenterSvc: helpcenterSvc,
		searchSvc:     searchSvc,
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
	docType := ptrIfSet(q.Get("doc_type"))
	status := ptrIfSet(q.Get("status"))
	teamID := ptrIfSet(q.Get("team_id"))

	role := ""
	if actor != nil {
		role = actor.Role
	}
	docs, err := h.documentSvc.List(r.Context(), wsID, spaceID, collectionID, docType, status, teamID, userID, role)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
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
	doc, err := h.documentSvc.Get(r.Context(), chi.URLParam(r, "docId"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if doc == nil {
		writeError(w, http.StatusNotFound, "document not found")
		return
	}
	writeJSON(w, http.StatusOK, doc)
}

func (h *DocsHandler) UpdateDocument(w http.ResponseWriter, r *http.Request) {
	var req model.UpdateDocsDocumentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	doc, err := h.documentSvc.Update(r.Context(), chi.URLParam(r, "docId"), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, doc)
}

func (h *DocsHandler) DeleteDocument(w http.ResponseWriter, r *http.Request) {
	if err := h.documentSvc.Delete(r.Context(), chi.URLParam(r, "docId")); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
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

	// If help center article moved to non-external space, unpublish externally.
	if doc.DocType == model.DocTypeHelpCenterArticle {
		space, _ := h.spaceSvc.GetUnfiltered(r.Context(), doc.SpaceID)
		if space != nil && space.Type != model.SpaceTypeExternalCapable {
			_ = h.helpcenterSvc.UnpublishExternally(r.Context(), docID)
		}
	}

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

func (h *DocsHandler) SaveContent(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	docID := chi.URLParam(r, "docId")
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

	writeJSON(w, http.StatusOK, content)
}

// SaveMarkdownContent accepts raw Markdown from AI agents or external tools and
// stores it as a JSON envelope that the frontend auto-converts on first load.
func (h *DocsHandler) SaveMarkdownContent(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	docID := chi.URLParam(r, "docId")
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

	doc, err := h.documentSvc.Publish(r.Context(), docID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Snapshot on publish.
	_, _ = h.versionSvc.SnapshotOnPublish(r.Context(), docID, userID)

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
	writeJSON(w, http.StatusOK, map[string]string{"status": "published"})
}

func (h *DocsHandler) UnpublishExternally(w http.ResponseWriter, r *http.Request) {
	if err := h.helpcenterSvc.UnpublishExternally(r.Context(), chi.URLParam(r, "docId")); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "unpublished"})
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
	docType := ptrIfSet(q.Get("doc_type"))
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

	results, err := h.searchSvc.Search(r.Context(), wsID, query, spaceIDs, docType, status, limit)
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

// ─── Public Help Center routes ──────────────────────────────────────────────

func (h *DocsHandler) PublicGetArticle(w http.ResponseWriter, r *http.Request) {
	subdomain := chi.URLParam(r, "subdomain")
	slug := chi.URLParam(r, "slug")

	cfg, err := h.helpcenterSvc.GetConfig(r.Context(), "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Lookup config by subdomain.
	if cfg == nil || cfg.Subdomain != subdomain {
		// Try subdomain-based lookup via direct query.
		cfg = nil
	}

	// Resolve the workspace from subdomain — need to use hcRepo directly.
	// For public routes, we pass through with a special handler.
	_ = cfg

	// Resolve slug to document ID.
	docID, isAlias, err := h.helpcenterSvc.ResolveSlug(r.Context(), "", slug)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = isAlias

	if docID == "" {
		writeError(w, http.StatusNotFound, "article not found")
		return
	}

	// Verify article is publicly published.
	art, err := h.helpcenterSvc.GetArticle(r.Context(), docID)
	if err != nil || art == nil || art.PublicPublishedAt == nil {
		writeError(w, http.StatusNotFound, "article not found")
		return
	}

	doc, err := h.documentSvc.Get(r.Context(), docID)
	if err != nil || doc == nil || doc.Status != model.DocStatusPublished {
		writeError(w, http.StatusNotFound, "article not found")
		return
	}

	content, _ := h.contentSvc.Get(r.Context(), docID)

	// Increment view count asynchronously.
	go func() { _ = h.helpcenterSvc.IncrementViewCount(r.Context(), docID) }()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"document": doc,
		"content":  content,
		"article":  art,
	})
}

func (h *DocsHandler) PublicSearchArticles(w http.ResponseWriter, r *http.Request) {
	subdomain := chi.URLParam(r, "subdomain")
	query := r.URL.Query().Get("q")
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil {
			limit = n
		}
	}

	// Resolve workspace from subdomain — for now, search requires knowing workspaceID.
	// In production, the public route middleware will resolve this.
	_ = subdomain

	results, err := h.searchSvc.PublicSearch(r.Context(), "", query, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, results)
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
