//go:build ee

package demo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	coreservice "github.com/helpin-ai/helpin/server/internal/service"
)

// Names of the demo tools on the public MCP server.
const (
	ToolCreate = "create_demo"
	ToolGet    = "get_demo"
)

// MCPTools returns the demo tools. They are offered only in workspaces that
// may ask for demos; everywhere else they are neither listed nor callable.
func (s *Service) MCPTools() []coreservice.MCPToolExtension {
	companyProperties := map[string]any{
		"company_id": map[string]any{"type": "string", "minLength": 1, "maxLength": 64, "description": "ID of the company in this workspace's CRM."},
		"domain":     map[string]any{"type": "string", "minLength": 3, "maxLength": 253, "description": "The company's domain, such as acme.com. Used when company_id is not given."},
	}
	createProperties := map[string]any{
		"help_center_url": map[string]any{"type": "string", "minLength": 4, "maxLength": 2048, "description": "Address of the company's public Intercom help center. Defaults to the company's help_center_url property."},
	}
	getProperties := map[string]any{
		"demo_id": map[string]any{"type": "string", "minLength": 1, "maxLength": 64, "description": "ID returned by create_demo."},
	}
	for key, value := range companyProperties {
		createProperties[key] = value
		getProperties[key] = value
	}
	object := func(properties map[string]any) map[string]any {
		return map[string]any{"type": "object", "properties": properties, "additionalProperties": false}
	}
	return []coreservice.MCPToolExtension{
		{
			Definition: coreservice.MCPToolDefinition{
				Name:  ToolCreate,
				Title: "Create demo",
				Description: "Start building a demo workspace for a company in this workspace's CRM: a copy of the company's public Intercom help center " +
					"with its branding, published in demo mode, with the chat widget and support agent turned on. Returns at once with a demo ID; " +
					"the build takes a few minutes, so poll get_demo until status is ready or failed. Calling it again for the same company returns " +
					"the existing demo, or continues one that failed. When it finishes, the company's demo_status, demo_workspace_id and " +
					"demo_workspace_url properties are set and a note is added to the company.",
				InputSchema: object(createProperties),
				Toolset:     coreservice.MCPToolsetCRM, Scope: coreservice.MCPScopeCRMWrite,
				// A demo creates a workspace, so it takes a workspace admin.
				Permission: authorization.PermSettingsManage, Module: model.ModuleCRM, Mutating: true,
			},
			Available: s.AllowsWorkspace,
			Execute:   s.executeCreateDemo,
		},
		{
			Definition: coreservice.MCPToolDefinition{
				Name:  ToolGet,
				Title: "Get demo",
				Description: "Get the status and report of a demo build by demo_id, company_id or domain. status is queued, building, ready or failed; " +
					"step names the stage it is on. Once ready, demo_url is the demo help center and report lists what was imported and " +
					"anything that needs a look by hand.",
				InputSchema: object(getProperties),
				Toolset:     coreservice.MCPToolsetCRM, Scope: coreservice.MCPScopeCRMRead,
				Permission: authorization.PermCRMRead, Module: model.ModuleCRM,
			},
			Available: s.AllowsWorkspace,
			Execute:   s.executeGetDemo,
		},
	}
}

type demoToolArguments struct {
	DemoID        string `json:"demo_id"`
	CompanyID     string `json:"company_id"`
	Domain        string `json:"domain"`
	HelpCenterURL string `json:"help_center_url"`
}

func (s *Service) executeCreateDemo(ctx context.Context, call coreservice.MCPToolCall, arguments json.RawMessage) (*coreservice.MCPToolResult, error) {
	var input demoToolArguments
	if err := json.Unmarshal(arguments, &input); err != nil {
		return nil, coreservice.ErrMCPInvalidArguments
	}
	view, created, err := s.Create(ctx, call.WorkspaceID, call.UserID, CreateInput{
		CompanyID: input.CompanyID, Domain: input.Domain, HelpCenterURL: input.HelpCenterURL,
	})
	if err != nil {
		return nil, demoToolError(err)
	}
	summary := fmt.Sprintf("A demo for %s already exists (status: %s).", view.CompanyName, view.Status)
	if created {
		summary = fmt.Sprintf("Building a demo for %s. Poll get_demo with demo_id %s until status is ready or failed.", view.CompanyName, view.ID)
	}
	return demoToolResult(summary, view), nil
}

func (s *Service) executeGetDemo(ctx context.Context, call coreservice.MCPToolCall, arguments json.RawMessage) (*coreservice.MCPToolResult, error) {
	var input demoToolArguments
	if err := json.Unmarshal(arguments, &input); err != nil {
		return nil, coreservice.ErrMCPInvalidArguments
	}
	view, err := s.Get(ctx, call.WorkspaceID, input.DemoID, CreateInput{CompanyID: input.CompanyID, Domain: input.Domain})
	if err != nil {
		return nil, demoToolError(err)
	}
	summary := fmt.Sprintf("The demo for %s is %s (step: %s).", view.CompanyName, view.Status, view.Step)
	switch view.Status {
	case "ready":
		summary = fmt.Sprintf("The demo for %s is ready.", view.CompanyName)
	case "failed":
		summary = fmt.Sprintf("The demo for %s failed: %s", view.CompanyName, view.Error)
	}
	return demoToolResult(summary, view), nil
}

func demoToolResult(summary string, view *View) *coreservice.MCPToolResult {
	result := &coreservice.MCPToolResult{Summary: summary, Data: view}
	if view.DemoURL != "" {
		result.Links = map[string]string{"demo": view.DemoURL}
	}
	return result
}

// demoToolError turns a service error into what the tool caller is told.
func demoToolError(err error) error {
	var input *InputError
	switch {
	case errors.As(err, &input):
		return &coreservice.MCPToolError{Code: "invalid_request", Message: input.Message, Err: coreservice.ErrMCPInvalidArguments}
	case errors.Is(err, ErrUnavailable):
		return coreservice.ErrMCPForbidden
	}
	return err
}
