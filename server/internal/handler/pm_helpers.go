package handler

import (
	"net/http"
	"strconv"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func getWorkspaceID(r *http.Request) string {
	workspaceID := middleware.GetWorkspaceID(r.Context())
	if workspaceID != "" {
		return workspaceID
	}
	workspaceID = r.Header.Get("X-Workspace-ID")
	if workspaceID != "" {
		return workspaceID
	}
	return r.URL.Query().Get("workspace_id")
}

func queryStringPtr(r *http.Request, key string) *string {
	value := r.URL.Query().Get(key)
	if value == "" {
		return nil
	}
	return &value
}

func queryBoolPtr(r *http.Request, key string) (*bool, error) {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseBool(raw)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func queryInt(r *http.Request, key string, defaultValue int) int {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return defaultValue
	}
	return value
}

func queryPagination(r *http.Request) model.PMPagination {
	return model.PMPagination{
		Page:    queryInt(r, "page", 1),
		PerPage: queryInt(r, "per_page", 50),
	}
}
