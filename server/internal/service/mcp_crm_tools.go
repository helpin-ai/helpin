package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
)

const (
	_mcpCRMIncludeLimit       = 25
	_mcpCRMActivityBodyMaxLen = 4000
)

// SetCRMRecords wires the company reads, activity reads, and batch upsert
// used by the CRM MCP tools.
func (s *MCPService) SetCRMRecords(companies *CRMCompanyService, activities *CRMActivityService, upserter *CRMRecordUpsertService) {
	s.crmCompanies = companies
	s.crmActivities = activities
	s.crmUpserter = upserter
}

func crmMCPToolDefinitions() []MCPToolDefinition {
	crmRead := func(definition MCPToolDefinition) MCPToolDefinition {
		definition.Toolset, definition.Scope = MCPToolsetCRM, MCPScopeCRMRead
		definition.Permission, definition.Module = authorization.PermCRMRead, model.ModuleCRM
		return definition
	}
	getSchema := func(idField string, includes ...string) map[string]any {
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				idField: map[string]any{"type": "string", "minLength": 1},
				"include": map[string]any{
					"type": "array", "uniqueItems": true,
					"items":       map[string]any{"type": "string", "enum": includes},
					"description": "Related records to return, newest first, up to 25 of each.",
				},
			},
			"required":             []string{idField},
			"additionalProperties": false,
		}
	}
	return []MCPToolDefinition{
		crmRead(MCPToolDefinition{
			Name: "get_crm_company", Title: "Get CRM company",
			Description: "Get one CRM company with its custom properties. Optionally include its logged activities (notes, calls, meetings, emails), contacts, and deals.",
			InputSchema: getSchema("company_id", "activities", "contacts", "deals"),
		}),
		crmRead(MCPToolDefinition{
			Name: "get_crm_contact", Title: "Get CRM contact",
			Description: "Get one CRM contact with its custom properties. Optionally include its logged activities.",
			InputSchema: getSchema("contact_id", "activities"),
		}),
		crmRead(MCPToolDefinition{
			Name: "get_crm_deal", Title: "Get CRM deal",
			Description: "Get one CRM deal. Optionally include its logged activities.",
			InputSchema: getSchema("deal_id", "activities"),
		}),
		{
			Name: "upsert_crm_records", Title: "Upsert CRM records",
			Description: "Create or update up to 100 CRM companies or contacts in one call, matched on a stable key so reruns never duplicate. " +
				"Companies match on domain (normalized), external_id, or id; contacts on email or id. " +
				"Use fill_only to fill empty values without overwriting existing ones. " +
				"Each record may carry a note, logged once as a note activity. Returns a result per record; one bad record does not stop the batch. " +
				"Company fields: " + strings.Join(CRMUpsertFieldNames(model.CRMUpsertObjectCompany), ", ") + ". " +
				"Contact fields: " + strings.Join(CRMUpsertFieldNames(model.CRMUpsertObjectContact), ", ") + ".",
			InputSchema: withMCPIdempotencyKey(crmUpsertMCPSchema()),
			Toolset:     MCPToolsetCRM, Scope: MCPScopeCRMWrite, Permission: authorization.PermCRMEdit, Module: model.ModuleCRM,
			Mutating: true, IdempotentHint: true,
		},
	}
}

func crmUpsertMCPSchema() map[string]any {
	record := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"ref": map[string]any{"type": "string", "maxLength": 200, "description": "Your identifier for this record, echoed in its result."},
			"id":  map[string]any{"type": "string", "description": "Existing record ID. Required when match_on is id."},
			"fields": map[string]any{
				"type":                 "object",
				"additionalProperties": map[string]any{"type": []string{"string", "number"}},
				"description":          "Native fields by name. Omit a field to leave it unchanged.",
			},
			"custom_properties": map[string]any{
				"type":        "object",
				"description": "snake_case keys with string, number, boolean, or string-list values, merged into existing properties. null removes a key. Filter on them with custom_properties.<key> in list tools.",
			},
			"note": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"subject": map[string]any{"type": "string", "maxLength": 500},
					"body":    map[string]any{"type": "string", "minLength": 1, "maxLength": 20000},
				},
				"required":             []string{"body"},
				"additionalProperties": false,
				"description":          "Evidence to log as a note activity. An identical note already logged by this tool is not repeated.",
			},
		},
		"additionalProperties": false,
	}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"object_type": map[string]any{"type": "string", "enum": []string{model.CRMUpsertObjectCompany, model.CRMUpsertObjectContact}},
			"match_on": map[string]any{
				"type":        "string",
				"enum":        []string{model.CRMUpsertMatchDomain, model.CRMUpsertMatchExternalID, model.CRMUpsertMatchEmail, model.CRMUpsertMatchID},
				"description": "Key used to find existing records. Defaults to domain for companies and email for contacts.",
			},
			"mode": map[string]any{
				"type":    "string",
				"enum":    []string{model.CRMUpsertModeUpsert, model.CRMUpsertModeCreateOnly, model.CRMUpsertModeUpdateOnly},
				"default": model.CRMUpsertModeUpsert,
			},
			"fill_only": map[string]any{"type": "boolean", "default": false, "description": "Only fill empty fields and missing custom properties; never overwrite."},
			"records":   map[string]any{"type": "array", "items": record, "minItems": 1, "maxItems": _crmUpsertMaxRecords},
		},
		"required":             []string{"object_type", "records"},
		"additionalProperties": false,
	}
}

func (s *MCPService) executeCRMMCPTool(
	ctx context.Context,
	principal *model.MCPPrincipal,
	name string,
	arguments json.RawMessage,
) (*MCPToolResult, bool, error) {
	switch name {
	case "get_crm_company":
		result, err := s.getMCPCRMCompany(ctx, principal, arguments)
		return result, true, err
	case "get_crm_contact":
		result, err := s.getMCPCRMContact(ctx, principal, arguments)
		return result, true, err
	case "get_crm_deal":
		result, err := s.getMCPCRMDeal(ctx, principal, arguments)
		return result, true, err
	case "upsert_crm_records":
		result, err := s.upsertMCPCRMRecords(ctx, principal, arguments)
		return result, true, err
	default:
		return nil, false, nil
	}
}

type mcpCRMGetInput struct {
	CompanyID string   `json:"company_id"`
	ContactID string   `json:"contact_id"`
	DealID    string   `json:"deal_id"`
	Include   []string `json:"include"`
}

func (s *MCPService) getMCPCRMCompany(ctx context.Context, principal *model.MCPPrincipal, arguments json.RawMessage) (*MCPToolResult, error) {
	var input mcpCRMGetInput
	if err := decodeMCPArguments(arguments, &input); err != nil {
		return nil, err
	}
	if s.crmCompanies == nil {
		return nil, ErrMCPForbidden
	}
	company, err := s.crmCompanies.GetByID(ctx, strings.TrimSpace(input.CompanyID))
	if err != nil || company.WorkspaceID != principal.WorkspaceID {
		return nil, ErrMCPNotFound
	}
	data, err := mcpCRMRecordData(company)
	if err != nil {
		return nil, err
	}
	page := model.PMPagination{Page: 1, PerPage: _mcpCRMIncludeLimit}
	for _, include := range input.Include {
		switch include {
		case "activities":
			data["activities"], err = s.mcpCRMActivities(ctx, principal.WorkspaceID, model.CRMActivityListFilters{CompanyID: &company.ID})
		case "contacts":
			var contacts []model.CRMContact
			contacts, _, err = s.crmCompanies.ListContacts(ctx, principal.WorkspaceID, company.ID, "", page)
			items := make([]map[string]any, 0, len(contacts))
			for index := range contacts {
				items = append(items, compactCRMContact(&contacts[index]))
			}
			data["contacts"] = items
		case "deals":
			var deals []model.CRMDeal
			deals, _, err = s.crmCompanies.ListDeals(ctx, principal.WorkspaceID, company.ID, "", page)
			items := make([]map[string]any, 0, len(deals))
			for index := range deals {
				items = append(items, compactCRMDeal(&deals[index]))
			}
			data["deals"] = items
		}
		if err != nil {
			return nil, err
		}
	}
	return &MCPToolResult{Summary: "CRM company " + company.Name + " loaded.", Data: data}, nil
}

func (s *MCPService) getMCPCRMContact(ctx context.Context, principal *model.MCPPrincipal, arguments json.RawMessage) (*MCPToolResult, error) {
	var input mcpCRMGetInput
	if err := decodeMCPArguments(arguments, &input); err != nil {
		return nil, err
	}
	contact, err := s.crmContacts.GetByID(ctx, strings.TrimSpace(input.ContactID))
	if err != nil || contact.WorkspaceID != principal.WorkspaceID {
		return nil, ErrMCPNotFound
	}
	data, err := mcpCRMRecordData(contact)
	if err != nil {
		return nil, err
	}
	if containsString(input.Include, "activities") {
		if data["activities"], err = s.mcpCRMActivities(ctx, principal.WorkspaceID, model.CRMActivityListFilters{ContactID: &contact.ID}); err != nil {
			return nil, err
		}
	}
	return &MCPToolResult{Summary: "CRM contact loaded.", Data: data}, nil
}

func (s *MCPService) getMCPCRMDeal(ctx context.Context, principal *model.MCPPrincipal, arguments json.RawMessage) (*MCPToolResult, error) {
	var input mcpCRMGetInput
	if err := decodeMCPArguments(arguments, &input); err != nil {
		return nil, err
	}
	deal, err := s.crmDeals.GetByID(ctx, strings.TrimSpace(input.DealID))
	if err != nil || deal.WorkspaceID != principal.WorkspaceID {
		return nil, ErrMCPNotFound
	}
	data, err := mcpCRMRecordData(deal)
	if err != nil {
		return nil, err
	}
	if containsString(input.Include, "activities") {
		if data["activities"], err = s.mcpCRMActivities(ctx, principal.WorkspaceID, model.CRMActivityListFilters{DealID: &deal.ID}); err != nil {
			return nil, err
		}
	}
	return &MCPToolResult{Summary: "CRM deal " + deal.Name + " loaded.", Data: data}, nil
}

// mcpCRMActivities returns the newest logged activities for one record, with
// long bodies truncated to keep tool results bounded.
func (s *MCPService) mcpCRMActivities(ctx context.Context, workspaceID string, filters model.CRMActivityListFilters) ([]map[string]any, error) {
	if s.crmActivities == nil {
		return nil, ErrMCPForbidden
	}
	activities, _, err := s.crmActivities.List(ctx, workspaceID, filters, model.PMPagination{Page: 1, PerPage: _mcpCRMIncludeLimit})
	if err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(activities))
	for _, activity := range activities {
		item := map[string]any{
			"id": activity.ID, "activity_type": activity.ActivityType,
			"subject": activity.Subject, "occurred_at": activity.OccurredAt,
		}
		if activity.Body != nil {
			body := []rune(*activity.Body)
			if len(body) > _mcpCRMActivityBodyMaxLen {
				item["body"] = string(body[:_mcpCRMActivityBodyMaxLen])
				item["body_truncated"] = true
			} else {
				item["body"] = *activity.Body
			}
		}
		items = append(items, item)
	}
	return items, nil
}

// mcpCRMRecordData keeps the record's JSON shape so related records can be
// added alongside its fields without changing existing keys.
func mcpCRMRecordData(record any) (map[string]any, error) {
	data := map[string]any{}
	if err := remarshalCRMUpsert(record, &data); err != nil {
		return nil, err
	}
	return data, nil
}

func (s *MCPService) upsertMCPCRMRecords(ctx context.Context, principal *model.MCPPrincipal, arguments json.RawMessage) (*MCPToolResult, error) {
	if s.crmUpserter == nil {
		return nil, ErrMCPForbidden
	}
	var req model.CRMRecordUpsertRequest
	if err := decodeMCPArguments(arguments, &req); err != nil {
		return nil, err
	}
	req.WorkspaceID, req.ActorUserID = principal.WorkspaceID, principal.UserID
	response, err := s.crmUpserter.Upsert(ctx, req)
	if err != nil {
		if message, ok := CRMUpsertInputMessage(err); ok {
			return nil, newMCPToolError(MCPErrorCodeInvalidInput, mcpSentence(message))
		}
		return nil, fmt.Errorf("upsert CRM records: %w", err)
	}
	summary := make([]string, 0, len(response.Counts))
	for _, action := range []string{
		model.CRMUpsertActionCreated, model.CRMUpsertActionUpdated, model.CRMUpsertActionUnchanged,
		model.CRMUpsertActionSkipped, model.CRMUpsertActionError,
	} {
		if count := response.Counts[action]; count > 0 {
			summary = append(summary, strconv.Itoa(count)+" "+action)
		}
	}
	return &MCPToolResult{
		Summary: "Processed " + strconv.Itoa(len(response.Results)) + " " + response.ObjectType + " records: " + strings.Join(summary, ", ") + ".",
		Data:    response,
	}, nil
}
