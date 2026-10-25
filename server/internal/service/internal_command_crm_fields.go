package service

import (
	"context"
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
