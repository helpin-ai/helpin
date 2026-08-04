package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

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

func queryStringValues(r *http.Request, key string) []string {
	rawValues := r.URL.Query()[key]
	if len(rawValues) == 0 {
		return nil
	}
	values := make([]string, 0, len(rawValues))
	for _, raw := range rawValues {
		for _, part := range strings.Split(raw, ",") {
			value := strings.TrimSpace(part)
			if value == "" {
				continue
			}
			values = append(values, value)
		}
	}
	return values
}

// queryStringPtrWithFallback returns the first non-empty query param from the given keys.
func queryStringPtrWithFallback(r *http.Request, keys ...string) *string {
	for _, key := range keys {
		if value := r.URL.Query().Get(key); value != "" {
			return &value
		}
	}
	return nil
}

func queryFilterGroup(r *http.Request, key string) (*model.QueryFilterGroup, error) {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return nil, nil
	}

	var group model.QueryFilterGroup
	if err := json.Unmarshal([]byte(raw), &group); err != nil {
		return nil, err
	}
	if len(group.Rules) == 0 {
		return nil, nil
	}
	if group.Logic == "" {
		group.Logic = model.QueryFilterLogicAnd
	}
	return &group, nil
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

func queryBoolDefault(r *http.Request, key string, fallback bool) (bool, error) {
	value := r.URL.Query().Get(key)
	if value == "" {
		return fallback, nil
	}
	return strconv.ParseBool(value)
}

func queryPagination(r *http.Request) model.PMPagination {
	return model.PMPagination{
		Page:    queryInt(r, "page", 1),
		PerPage: queryInt(r, "per_page", 50),
	}
}

func queryAgentRunPagination(r *http.Request) model.PMPagination {
	pagination := queryPagination(r)
	if pagination.Page <= 0 {
		pagination.Page = 1
	}
	if pagination.PerPage <= 0 {
		pagination.PerPage = 50
	}
	if pagination.PerPage > 500 {
		pagination.PerPage = 500
	}
	return pagination
}
