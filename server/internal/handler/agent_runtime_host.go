package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
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
	r.Body = http.MaxBytesReader(w, r.Body, 10*1024*1024+1024*1024)
	if err := r.ParseMultipartForm(10 * 1024 * 1024); err != nil {
		writeError(w, http.StatusBadRequest, "invalid screenshot upload")
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
	payload, err := io.ReadAll(io.LimitReader(file, 10*1024*1024+1))
	if err != nil || len(payload) == 0 || len(payload) > 10*1024*1024 {
		writeError(w, http.StatusBadRequest, "screenshot must be between 1 byte and 10 MB")
		return
	}
	contentType := http.DetectContentType(payload)
	asset, err := h.host.UploadBrowserAsset(r.Context(), service.AgentRuntimeBrowserAssetUpload{
		AppID: r.FormValue("app_id"), RuntimeRunID: r.FormValue("run_id"), ArtifactType: r.FormValue("artifact_type"), Metadata: json.RawMessage(r.FormValue("metadata")),
		FileName: header.Filename, ContentType: contentType, Size: int64(len(payload)), Body: bytes.NewReader(payload),
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

func (h *AgentRuntimeHostHandler) ExecuteCommand(w http.ResponseWriter, r *http.Request) {
	var req agentruntime.CommandExecutionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.host.ExecuteCommand(r.Context(), req)
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
