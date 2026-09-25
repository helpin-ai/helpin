//go:build !ee

package deployment

import (
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestCommunityHasNoHostedServiceDefaults(t *testing.T) {
	if DefaultReplyDomain != "" || DefaultRouteDomain != "" || DefaultWidgetOrigin != "" || DefaultSDKLoaderURL != "" {
		t.Fatal("Community must use operator-owned service addresses")
	}
}

func TestCommunityEnablesAllModulesByDefault(t *testing.T) {
	modules, err := ParseModules("")
	if err != nil {
		t.Fatalf("ParseModules(\"\") error = %v", err)
	}
	enabled := map[model.ModuleID]bool{}
	for _, module := range modules {
		enabled[module] = true
	}
	for _, module := range []model.ModuleID{
		model.ModuleSupport, model.ModuleDocs, model.ModuleAgents,
		model.ModulePM, model.ModuleCRM, model.ModuleAutomation,
	} {
		if !enabled[module] {
			t.Errorf("default Community modules omit %q", module)
		}
	}
}

func TestCommunityShowsSetupGuideByDefault(t *testing.T) {
	enabled, err := SetupGuidePolicy("")
	if err != nil || !enabled {
		t.Fatalf("SetupGuidePolicy(\"\") = %v, %v; want true", enabled, err)
	}
}
