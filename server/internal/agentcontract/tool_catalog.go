package agentcontract

import (
	_ "embed"
	"encoding/json"

	"github.com/helpin-ai/helpin/server/internal/model"
)

//go:embed tool_catalog.json
var toolCatalogJSON []byte

//go:embed browser_tool_catalog.json
var browserToolCatalogJSON []byte

// ListToolCatalog returns the frozen Helpin host tool catalog.
//
// Execution moved to Agent Runtime, but Helpin still owns this metadata for
// agent configuration and UI display.
func ListToolCatalog() model.ToolCatalogResponse {
	var catalog model.ToolCatalogResponse
	if err := json.Unmarshal(toolCatalogJSON, &catalog); err != nil {
		panic("invalid embedded agent tool catalog: " + err.Error())
	}
	var browserCatalog model.ToolCatalogResponse
	if err := json.Unmarshal(browserToolCatalogJSON, &browserCatalog); err != nil {
		panic("invalid embedded browser tool catalog: " + err.Error())
	}
	catalog.Tools = append(catalog.Tools, browserCatalog.Tools...)
	catalog.Categories = append(catalog.Categories, browserCatalog.Categories...)
	return catalog
}
