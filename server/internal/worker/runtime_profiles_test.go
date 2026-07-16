package worker

import (
	"slices"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestDocumentationRuntimeProfileIncludesOrganizationTools(t *testing.T) {
	profile := GetRuntimeProfile(model.AgentPresetDocumentationAgent)
	for _, tool := range []string{"list_spaces", "create_space", "create_collection", "update_space", "update_collection", "move_document", "link_document_to_object"} {
		if !slices.Contains(profile.AllowedTools, tool) {
			t.Fatalf("documentation runtime profile is missing %q", tool)
		}
	}
}
