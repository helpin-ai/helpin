//go:build ee

package demo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	eerepository "github.com/helpin-ai/helpin/server/ee/repository"
	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	coreservice "github.com/helpin-ai/helpin/server/internal/service"
)

// Names of the demo tools on the public MCP server.
const (
	ToolCreate  = "create_demo"
	ToolGet     = "get_demo"
	ToolRefresh = "refresh_demo"
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
		"website_url":     map[string]any{"type": "string", "minLength": 4, "maxLength": 2048, "description": "The company's website (https), crawled so the AI can answer pricing and product questions. Defaults to the company's website property, then https://<domain>."},
		"verify_questions": map[string]any{
			"type": "array", "maxItems": demoMaxVerifyQuestions, "description": "Extra questions to ask the demo's AI after the build. They are reported but do not decide the status.",
			"items": map[string]any{"type": "string", "minLength": 3, "maxLength": 500},
		},
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
					"with its branding, published in demo mode, the company website crawled for the AI, and the chat widget and support agent turned on. " +
					"Before it finishes, the build waits for indexing and asks the AI a few check questions. Returns at once with a demo ID; " +
					"the build takes 10 to 60 minutes and waits in the queue while other builds run, so poll get_demo until status is ready, " +
					"ready_with_issues or failed. Calling it again for the same company returns the existing demo, or continues one that failed. " +
					"When it finishes, the company's demo_status, demo_workspace_id, demo_workspace_url and demo_website_url properties are set " +
					"and a note is added to the company.",
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
				Description: "Get the status and report of a demo build by demo_id, company_id or domain. status is queued, building, ready, " +
					"ready_with_issues or failed; step names the stage it is on (index_wait while the AI indexes). Once finished, demo_url is the " +
					"demo help center and report lists what was imported, the website and help articles indexed, each check question's outcome " +
					"(verify), issues explaining ready_with_issues, and anything that needs a look by hand.",
				InputSchema: object(getProperties),
				Toolset:     coreservice.MCPToolsetCRM, Scope: coreservice.MCPScopeCRMRead,
				Permission: authorization.PermCRMRead, Module: model.ModuleCRM,
			},
			Available: s.AllowsWorkspace,
			Execute:   s.executeGetDemo,
		},
		{
			Definition: coreservice.MCPToolDefinition{
				Name:  ToolRefresh,
				Title: "Refresh demo",
				Description: "Run a finished demo again from its website step without importing the help center again: link the company " +
					"website, rename the chat widget and limit it to the demo help center, reindex the help center for the AI, and ask the check " +
					"questions again. Use it after reindexing or answer-check fixes. Poll get_demo until status is ready or ready_with_issues.",
				InputSchema: object(getProperties),
				Toolset:     coreservice.MCPToolsetCRM, Scope: coreservice.MCPScopeCRMWrite,
				Permission: authorization.PermSettingsManage, Module: model.ModuleCRM, Mutating: true,
			},
			Available: s.AllowsWorkspace,
			Execute:   s.executeRefreshDemo,
		},
	}
}

type demoToolArguments struct {
	DemoID          string   `json:"demo_id"`
	CompanyID       string   `json:"company_id"`
	Domain          string   `json:"domain"`
	HelpCenterURL   string   `json:"help_center_url"`
	WebsiteURL      string   `json:"website_url"`
	VerifyQuestions []string `json:"verify_questions"`
}

func (s *Service) executeCreateDemo(ctx context.Context, call coreservice.MCPToolCall, arguments json.RawMessage) (*coreservice.MCPToolResult, error) {
	var input demoToolArguments
	if err := json.Unmarshal(arguments, &input); err != nil {
		return nil, coreservice.ErrMCPInvalidArguments
	}
	view, created, err := s.Create(ctx, call.WorkspaceID, call.UserID, CreateInput{
		CompanyID: input.CompanyID, Domain: input.Domain, HelpCenterURL: input.HelpCenterURL,
		WebsiteURL: input.WebsiteURL, VerifyQuestions: input.VerifyQuestions,
	})
	if err != nil {
		return nil, demoToolError(err)
	}
	summary := fmt.Sprintf("A demo for %s already exists (status: %s).", view.CompanyName, view.Status)
	if created {
		summary = fmt.Sprintf("Building a demo for %s. Poll get_demo with demo_id %s until status is ready, ready_with_issues or failed.", view.CompanyName, view.ID)
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
	case eerepository.DemoStatusReady:
		summary = fmt.Sprintf("The demo for %s is ready.", view.CompanyName)
	case eerepository.DemoStatusReadyWithIssues:
		summary = fmt.Sprintf("The demo for %s is ready with issues: %s.", view.CompanyName, strings.Join(view.Report.Issues, ", "))
	case eerepository.DemoStatusFailed:
		summary = fmt.Sprintf("The demo for %s failed: %s", view.CompanyName, view.Error)
	}
	return demoToolResult(summary, view), nil
}

func (s *Service) executeRefreshDemo(ctx context.Context, call coreservice.MCPToolCall, arguments json.RawMessage) (*coreservice.MCPToolResult, error) {
	var input demoToolArguments
	if err := json.Unmarshal(arguments, &input); err != nil {
		return nil, coreservice.ErrMCPInvalidArguments
	}
	view, err := s.Refresh(ctx, call.WorkspaceID, input.DemoID, CreateInput{CompanyID: input.CompanyID, Domain: input.Domain})
	if err != nil {
		return nil, demoToolError(err)
	}
	summary := fmt.Sprintf("Refreshing the demo for %s. Poll get_demo with demo_id %s until status is ready or ready_with_issues.", view.CompanyName, view.ID)
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
