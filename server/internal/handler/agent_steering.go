package handler

import (
	"errors"
	"github.com/go-chi/chi/v5"
	sdk "github.com/helpin-ai/agent-runtime-go"
	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
	"net/http"
)

func handleRunSteering(w http.ResponseWriter, r *http.Request, agents *service.AgentService, runID string) {
	workspaceID := getWorkspaceID(r)
	if r.Method == http.MethodGet {
		commands, err := agents.RunSteering(r.Context(), workspaceID, runID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Unable to load steering for this run")
			return
		}
		writeJSON(w, http.StatusOK, commands)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 40<<10)
	var req model.SteerAgentRunRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid steering request")
		return
	}
	command, err := agents.SteerRun(r.Context(), workspaceID, runID, middleware.GetUserID(r.Context()), req)
	if err != nil {
		status := http.StatusBadGateway
		var upstream *sdk.HTTPStatusError
		if errors.As(err, &upstream) && upstream.StatusCode >= 400 && upstream.StatusCode < 500 {
			status = upstream.StatusCode
		}
		writeError(w, status, "Instruction was not accepted. Refresh the run and check that it supports steering before retrying.")
		return
	}
	writeJSON(w, http.StatusAccepted, command)
}
func (h *AgentHandler) RunSteering(w http.ResponseWriter, r *http.Request) {
	handleRunSteering(w, r, h.agentService, chi.URLParam(r, "id"))
}
func (h *DockChatHandler) RunSteering(w http.ResponseWriter, r *http.Request) {
	run, ok := h.resolveDockRun(w, r)
	if !ok {
		return
	}
	handleRunSteering(w, r, h.agentService, run.ID)
}

// ChatRunSteering pins the command to the run the user saw. A successor must
// never receive an instruction intended for the previous run.
func (h *DockChatHandler) ChatRunSteering(w http.ResponseWriter, r *http.Request) {
	run, ok := h.resolveOwnedChatRun(w, r)
	if !ok {
		return
	}
	if run.ID != chi.URLParam(r, "runID") {
		writeError(w, http.StatusConflict, "The chat run changed. Reopen the chat before steering.")
		return
	}
	handleRunSteering(w, r, h.agentService, run.ID)
}
