package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// SetCRMFieldCatalog wires crm.describe_fields.
func (s *InternalCommandService) SetCRMFieldCatalog(catalog *CRMFieldCatalogService) {
	if s == nil {
		return
	}
	s.crmFieldCatalog = catalog
}

func (s *InternalCommandService) executeCRMDescribeFields(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if s.crmFieldCatalog == nil {
		return nil, fmt.Errorf("CRM field catalog is not configured")
	}
	var req struct {
		ObjectType string `json:"object_type"`
		Compact    bool   `json:"compact"`
		Key        string `json:"key"`
		ValueQuery string `json:"value_query"`
		Limit      int    `json:"limit"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("parse describe fields input: %w", err)
	}
	objectType := strings.TrimSpace(req.ObjectType)
	if strings.TrimSpace(req.Key) == "" {
		if strings.TrimSpace(req.ValueQuery) != "" {
			return nil, errCommandInput("value_query requires key")
		}
		catalog, err := s.crmFieldCatalog.Catalog(ctx, meta.WorkspaceID, objectType)
		if err != nil {
			return nil, crmFieldCatalogError(err)
		}
		if req.Compact {
			return compactCRMFieldCatalog(meta.WorkspaceID, catalog), nil
		}
		return mustJSON(catalog), nil
	}
	values, err := s.crmFieldCatalog.PropertyValues(ctx, meta.WorkspaceID, model.CRMFieldCatalogRequest{
		ObjectType: objectType, Key: req.Key, ValueQuery: req.ValueQuery, Limit: req.Limit,
	})
	if err != nil {
		return nil, crmFieldCatalogError(err)
	}
	return mustJSON(values), nil
}

func crmFieldCatalogError(err error) error {
	var input *CRMFieldCatalogInputError
	if errors.As(err, &input) {
		return errCommandInput("%s", input.Message)
	}
	return err
}

// Compact discovery is opt-in so existing MCP and UI consumers keep their catalog.
func compactCRMFieldCatalog(workspaceID string, catalog *model.CRMFieldCatalog) json.RawMessage {
	properties := make([]map[string]any, 0, len(catalog.CustomProperties))
	for _, field := range catalog.CustomProperties {
		entry := map[string]any{"key": field.Key, "field": field.Field, "value_type": field.ValueType, "operators": field.Operators}
		if field.Definition != nil {
			entry["definition"] = field.Definition
		}
		properties = append(properties, entry)
	}
	schema := map[string]any{"workspace_id": workspaceID, "object_type": catalog.ObjectType, "fields": catalog.Fields, "custom_properties": properties, "truncated": catalog.Truncated}
	schema["schema_revision"] = fmt.Sprintf("%x", sha256.Sum256(mustJSON(schema)))
	schema["usage"] = "Reuse within this workspace and connection. Refresh after field changes or unknown-key errors. Request a specific key to inspect values; compact mode omits record samples."
	return mustJSON(schema)
}
