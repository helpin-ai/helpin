package authorization

import (
	"net/http"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/deployment"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// SetDeploymentModules installs validated startup policy. A workspace owner may
// grant access within this surface, but cannot enable a deployment-disabled module.
func (s *AuthzService) SetDeploymentModules(modules []model.ModuleID) {
	s.deploymentModules = make(map[model.ModuleID]bool, len(modules))
	for _, module := range modules {
		s.deploymentModules[module] = true
	}
}

func (s *AuthzService) deploymentEnabled(module model.ModuleID) bool {
	return module == "" || s.deploymentModules == nil || s.deploymentModules[module]
}

func (s *AuthzService) visibleModules(allowed map[model.ModuleID]struct{}) []model.ModuleID {
	if s.deploymentModules != nil {
		if _, ok := allowed[model.ModuleAutomation]; ok && s.deploymentEnabled(model.ModuleAgents) {
			allowed[model.ModuleAgents] = struct{}{}
		}
		for module := range allowed {
			if !s.deploymentEnabled(module) {
				delete(allowed, module)
			}
		}
	}
	return orderedModules(allowed)
}

func (s *AuthzService) requestModule(path string) model.ModuleID {
	module := deployment.APIModule(path)
	// Attachment content is shared by Docs and PM; the handler still checks
	// access to the owning resource. Do not open the rest of the PM API.
	if module == model.ModulePM && !s.deploymentEnabled(model.ModulePM) && (path == "/api/pm/attachments" || strings.HasPrefix(path, "/api/pm/attachments/")) {
		return model.ModuleDocs
	}
	if module == model.ModuleCRM && !s.deploymentEnabled(model.ModuleCRM) && deployment.SharedCustomerAPI(path) {
		return model.ModuleSupport
	}
	return module
}

// RequireDeploymentAccess rejects direct navigation/API access to disabled
// surfaces. It runs inside authentication and does not replace resource RBAC.
func RequireDeploymentAccess(s *AuthzService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if s != nil && !s.deploymentEnabled(s.requestModule(r.URL.Path)) {
				http.Error(w, "module is disabled for this installation", http.StatusNotFound)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireProductModule keeps agents independently accessible on legacy route
// prefixes without also granting access to flow builders or other modules.
func RequireProductModule(s *AuthzService, module model.ModuleID) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			selected := module
			if s != nil && s.deploymentModules != nil {
				candidate := s.requestModule(r.URL.Path)
				if candidate != "" {
					selected = candidate
				}
			}
			RequireModuleAccess(s, selected)(next).ServeHTTP(w, r)
		})
	}
}
