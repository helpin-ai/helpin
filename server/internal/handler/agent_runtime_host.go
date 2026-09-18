package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	agentruntime "github.com/helpin-ai/agent-runtime-go"

	"github.com/helpin-ai/helpin/server/internal/service"
)

type AgentRuntimeHostHandler struct {
	host       *service.AgentRuntimeHostService
	projection *service.AgentRuntimeProjectionService
}

func (h *AgentRuntimeHostHandler) UploadBrowserAsset(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.host == nil {
		writeError(w, http.StatusServiceUnavailable, "agent runtime host unavailable")
		return
	}
	const maxBrowserArtifactBytes = 100 * 1024 * 1024
	r.Body = http.MaxBytesReader(w, r.Body, maxBrowserArtifactBytes+1024*1024)
	if err := r.ParseMultipartForm(8 * 1024 * 1024); err != nil {
		writeError(w, http.StatusBadRequest, "invalid browser artifact upload")
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()
	if header.Size <= 0 || header.Size > maxBrowserArtifactBytes {
		writeError(w, http.StatusBadRequest, "browser artifact must be between 1 byte and 100 MB")
		return
	}
	sniff := make([]byte, 512)
	n, readErr := io.ReadFull(file, sniff)
	if readErr != nil && !errors.Is(readErr, io.EOF) && !errors.Is(readErr, io.ErrUnexpectedEOF) {
		writeError(w, http.StatusBadRequest, "unable to read browser artifact")
		return
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		writeError(w, http.StatusBadRequest, "unable to read browser artifact")
		return
	}
	contentType := http.DetectContentType(sniff[:n])
	if r.FormValue("artifact_type") == "analysis_output" && strings.HasPrefix(contentType, "text/plain") {
		switch strings.ToLower(filepath.Ext(header.Filename)) {
		case ".csv":
			contentType = "text/csv"
		case ".json":
			contentType = "application/json"
		}
	}
	asset, err := h.host.UploadBrowserAsset(r.Context(), service.AgentRuntimeBrowserAssetUpload{
		AppID: r.FormValue("app_id"), RuntimeRunID: r.FormValue("run_id"), ArtifactType: r.FormValue("artifact_type"), Metadata: json.RawMessage(r.FormValue("metadata")),
		FileName: header.Filename, ContentType: contentType, Size: header.Size, Body: file,
	})
	if err != nil {
		writeAgentRuntimeHostError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, asset)
}

func (h *AgentRuntimeHostHandler) BrowserArtifactContentURL(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.host == nil {
		writeError(w, http.StatusServiceUnavailable, "agent runtime host unavailable")
		return
	}
	content, err := h.host.BrowserArtifactContentURL(r.Context(), r.URL.Query().Get("workspace_id"), chi.URLParam(r, "id"))
	if err != nil {
		writeAgentRuntimeHostError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, content)
}

func NewAgentRuntimeHostHandler(host *service.AgentRuntimeHostService) *AgentRuntimeHostHandler {
	return &AgentRuntimeHostHandler{host: host}
}

func (h *AgentRuntimeHostHandler) SetProjectionService(projection *service.AgentRuntimeProjectionService) *AgentRuntimeHostHandler {
	if h != nil {
		h.projection = projection
	}
	return h
}

func (h *AgentRuntimeHostHandler) ApplyEvent(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.projection == nil {
		writeError(w, http.StatusServiceUnavailable, "agent runtime projection unavailable")
		return
	}
	var event service.AgentRuntimeEventEnvelope
	if err := decodeJSON(r, &event); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.projection.ApplyEvent(r.Context(), event); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}

func (h *AgentRuntimeHostHandler) ResolveTargetContext(w http.ResponseWriter, r *http.Request) {
	var req agentruntime.TargetContextRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.host.ResolveTargetContext(r.Context(), req)
	if err != nil {
		writeAgentRuntimeHostError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *AgentRuntimeHostHandler) ResolveRepositorySpec(w http.ResponseWriter, r *http.Request) {
	var req agentruntime.PrepareWorkspaceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.host.ResolveRepositorySpec(r.Context(), req)
	if err != nil {
		writeAgentRuntimeHostError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// ListProviderTools serves Helpin's internal MCP tool catalog.
func (h *AgentRuntimeHostHandler) ListProviderTools(w http.ResponseWriter, _ *http.Request) {
	catalog, err := h.host.ListProviderTools()
	if err != nil {
		writeAgentRuntimeHostError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, catalog)
}

// CallProviderTool executes one Helpin internal MCP tool.
func (h *AgentRuntimeHostHandler) CallProviderTool(w http.ResponseWriter, r *http.Request) {
	var req agentruntime.ProviderToolCallRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.host.CallProviderTool(r.Context(), req)
	if err != nil {
		writeAgentRuntimeHostError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *AgentRuntimeHostHandler) ResolveSkillByID(w http.ResponseWriter, r *http.Request) {
	var req service.AgentRuntimeSkillLookupRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.host.ResolveSkillByID(r.Context(), req)
	if err != nil {
		writeAgentRuntimeHostError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *AgentRuntimeHostHandler) ResolveActiveSkillByKey(w http.ResponseWriter, r *http.Request) {
	var req service.AgentRuntimeSkillLookupRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.host.ResolveActiveSkillByKey(r.Context(), req)
	if err != nil {
		writeAgentRuntimeHostError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *AgentRuntimeHostHandler) GetSkillPackageObject(w http.ResponseWriter, r *http.Request) {
	objectKey := strings.TrimSpace(chi.URLParam(r, "*"))
	if decoded, err := url.PathUnescape(objectKey); err == nil {
		objectKey = decoded
	}
	payload, err := h.host.GetSkillPackageObject(r.Context(), objectKey)
	if err != nil {
		writeAgentRuntimeHostError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(payload)
}

func writeAgentRuntimeHostError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrAgentRuntimeHostBadRequest):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, service.ErrAgentRuntimeHostForbidden):
		writeError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, service.ErrAgentRuntimeHostNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}
