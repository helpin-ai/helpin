package handler

import (
	"errors"
	"net/http"

	agentruntime "github.com/helpin-ai/agent-runtime-go"

	"github.com/helpin-ai/helpin/server/internal/service"
)

type AgentRuntimeHostHandler struct {
	host *service.AgentRuntimeHostService
}

func NewAgentRuntimeHostHandler(host *service.AgentRuntimeHostService) *AgentRuntimeHostHandler {
	return &AgentRuntimeHostHandler{host: host}
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
