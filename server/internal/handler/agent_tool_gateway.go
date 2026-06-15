package handler

import (
	"net/http"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

type AgentToolGatewayHandler struct {
	gateway *service.AgentToolGateway
}

func NewAgentToolGatewayHandler(gateway *service.AgentToolGateway) *AgentToolGatewayHandler {
	return &AgentToolGatewayHandler{gateway: gateway}
}

func (h *AgentToolGatewayHandler) ListTools(w http.ResponseWriter, r *http.Request) {
	token := bearerToken(r)
	resp, err := h.gateway.ListTools(r.Context(), token)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *AgentToolGatewayHandler) CallTool(w http.ResponseWriter, r *http.Request) {
	token := bearerToken(r)
	var req model.AgentRunToolCallRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.gateway.CallTool(r.Context(), token, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func bearerToken(r *http.Request) string {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(header), "bearer ") {
		return strings.TrimSpace(header[len("bearer "):])
	}
	return ""
}
