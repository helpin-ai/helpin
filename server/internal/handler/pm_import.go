package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

const shortcutImportMaxBytes = 50 << 20

type PMImportHandler struct {
	importService *service.PMImportService
}

func NewPMImportHandler(importService *service.PMImportService) *PMImportHandler {
	return &PMImportHandler{importService: importService}
}

func (h *PMImportHandler) PreviewShortcut(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "id")
	userID := middleware.GetUserID(r.Context())
	data, _, err := readShortcutImportFile(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	resp, err := h.importService.PreviewShortcut(r.Context(), workspaceID, userID, data)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *PMImportHandler) ExecuteShortcut(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "id")
	userID := middleware.GetUserID(r.Context())

	data, fileName, err := readShortcutImportFile(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	req := model.ShortcutImportExecuteRequest{
		UserMappings: map[string]string{},
		Options: model.ShortcutImportOptions{
			ImportArchived:  true,
			ImportCompleted: true,
		},
	}
	if raw := r.FormValue("user_mappings"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &req.UserMappings); err != nil {
			writeError(w, http.StatusBadRequest, "invalid user_mappings")
			return
		}
	}
	if raw := r.FormValue("workflow_state_mappings"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &req.WorkflowStateMappings); err != nil {
			writeError(w, http.StatusBadRequest, "invalid workflow_state_mappings")
			return
		}
	}
	if raw := r.FormValue("options"); raw != "" {
		var parsed map[string]bool
		if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
			writeError(w, http.StatusBadRequest, "invalid options")
			return
		}
		if value, ok := parsed["import_archived"]; ok {
			req.Options.ImportArchived = value
		}
		if value, ok := parsed["import_completed"]; ok {
			req.Options.ImportCompleted = value
		}
	}

	resp, err := h.importService.ExecuteShortcut(r.Context(), workspaceID, userID, fileName, data, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, resp)
}

func (h *PMImportHandler) ShortcutStatus(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "id")
	importID := chi.URLParam(r, "importId")
	userID := middleware.GetUserID(r.Context())

	resp, err := h.importService.GetShortcutStatus(r.Context(), workspaceID, userID, importID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func readShortcutImportFile(r *http.Request) ([]byte, string, error) {
	if err := r.ParseMultipartForm(shortcutImportMaxBytes); err != nil {
		return nil, "", err
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		return nil, "", err
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, shortcutImportMaxBytes+1))
	if err != nil {
		return nil, "", err
	}
	if int64(len(data)) > shortcutImportMaxBytes {
		return nil, "", errFileTooLarge
	}
	return data, header.Filename, nil
}

var errFileTooLarge = httpError("file exceeds 50MB limit")

type httpError string

func (e httpError) Error() string { return string(e) }
