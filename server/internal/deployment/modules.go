package deployment

import (
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// ParseModules validates the operator's deployment surface. It does not change
// workspace roles or remove shared customer, document or agent services.
func ParseModules(raw string) ([]model.ModuleID, error) {
	if strings.TrimSpace(raw) == "" {
		raw = DefaultModules
	}
	seen := map[model.ModuleID]bool{}
	var result []model.ModuleID
	for _, part := range strings.Split(raw, ",") {
		module := model.ModuleID(strings.TrimSpace(part))
		if !model.IsValidWorkspaceModule(module) {
			return nil, fmt.Errorf("unknown HELPIN_ENABLED_MODULES entry %q", part)
		}
		if !seen[module] {
			result = append(result, module)
			seen[module] = true
		}
	}
	if seen[model.ModuleSupport] && !seen[model.ModuleDocs] {
		return nil, fmt.Errorf("support requires docs for its help center")
	}
	return result, nil
}

// APIModule classifies product endpoints, including the existing PM agent API.
// Internal callbacks and shared account settings have no deployment-module gate.
func APIModule(path string) model.ModuleID {
	parts := strings.Split(strings.Trim(strings.TrimPrefix(path, "/api/"), "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	sub := ""
	if len(parts) > 1 {
		sub = parts[1]
	}
	switch parts[0] {
	case "pm":
		switch sub {
		case "agents", "agent-runs", "agent-presets", "agent-runtime", "agent-tools", "agent-skills", "agent-preset-versions", "agent-model-providers", "tool-catalog", "coding-sessions", "content-sources":
			return model.ModuleAgents
		}
		return model.ModulePM
	case "automation":
		switch sub {
		case "agents", "agent-fleet", "runs":
			return model.ModuleAgents
		}
		if sub == "library" && len(parts) > 2 && (parts[2] == "tools" || parts[2] == "skills") {
			return model.ModuleAgents
		}
		return model.ModuleAutomation
	case "agent-chats", "command-bar", "dock":
		return model.ModuleAgents
	case "support":
		return model.ModuleSupport
	case "docs":
		return model.ModuleDocs
	case "crm":
		return model.ModuleCRM
	}
	return ""
}

// SharedCustomerAPI retains the customer records used by Support's details rail
// when the CRM product surface is disabled. Existing resource permissions apply.
func SharedCustomerAPI(path string) bool {
	for _, prefix := range []string{"/api/crm/contacts", "/api/crm/companies", "/api/crm/associations", "/api/crm/search", "/api/crm/custom-fields"} {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}
