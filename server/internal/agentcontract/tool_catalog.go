package agentcontract

import (
	_ "embed"
	"encoding/json"
	"sort"

	"github.com/helpin-ai/helpin/server/internal/commandtools"
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
	// Command-backed product tools evolve with their typed metadata. Overlay
	// the frozen host catalog so new tools and pagination/schema improvements
	// are available without hand-editing the embedded JSON snapshot.
	byName := make(map[string]int, len(catalog.Tools))
	categorySet := make(map[string]bool, len(catalog.Categories))
	for index := range catalog.Tools {
		byName[catalog.Tools[index].Name] = index
		categorySet[catalog.Tools[index].Category] = true
	}
	for _, metadata := range commandtools.AllRuntimeToolMetadata() {
		inputSchema := normalizeCatalogSchema(metadata.InputSchema)
		entry := model.ToolCatalogEntry{
			Name: metadata.Alias, Description: metadata.Description, Category: metadata.Category,
			InputSchema: inputSchema, Presets: []string{},
		}
		if index, ok := byName[metadata.Alias]; ok {
			entry.Presets = catalog.Tools[index].Presets
			catalog.Tools[index] = entry
		} else {
			byName[metadata.Alias] = len(catalog.Tools)
			catalog.Tools = append(catalog.Tools, entry)
		}
		if metadata.Category != "" && !categorySet[metadata.Category] {
			categorySet[metadata.Category] = true
			catalog.Categories = append(catalog.Categories, metadata.Category)
		}
	}
	sort.Slice(catalog.Tools, func(i, j int) bool { return catalog.Tools[i].Name < catalog.Tools[j].Name })
	return catalog
}

func normalizeCatalogSchema(schema map[string]any) map[string]any {
	encoded, err := json.Marshal(schema)
	if err != nil {
		panic("invalid command tool schema: " + err.Error())
	}
	var normalized map[string]any
	if err := json.Unmarshal(encoded, &normalized); err != nil {
		panic("invalid command tool schema: " + err.Error())
	}
	return normalized
}
