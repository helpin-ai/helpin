package publicapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/service"
)

// SpecVersion is the version of the public API contract.
const SpecVersion = "1.0.0"

// DefaultServerURL is the hosted Helpin Cloud API base URL.
const DefaultServerURL = "https://api.helpin.ai" + BasePath

const apiDescription = `The Helpin API lets you read and change your workspace's tasks, planning, docs, CRM, and support data, and run Helpin agents, from your own code.

## Authentication

Create an **automation account** in Helpin under **Settings → MCP access → Service accounts** (a workspace manager must first allow automation accounts in the Permissions tab), choose its scopes, toolsets, and whether it is read-only, then create a token and copy it (it starts with ` + "`hmp_`" + ` and is shown once). Send it on every request:

` + "```" + `
Authorization: Bearer hmp_...
` + "```" + `

A token is bound to one workspace. Each operation lists the scope it needs; a token without that scope, an admin-restricted toolset, or a read-only token receives ` + "`403 forbidden`" + `. Workspace roles, product-module access, and workspace policy are enforced on every call, exactly as they are for the Helpin MCP server.

## Idempotency

Every operation that changes data accepts an ` + "`Idempotency-Key`" + ` header (8–128 characters). Retrying with the same key and body replays the original result instead of repeating the change; reusing a key with a different body returns ` + "`409 idempotency_conflict`" + `. If you omit the header a random key is generated, which means retries are not deduplicated.

## Responses and errors

Successful responses are ` + "`{ \"data\": …, \"summary\": \"…\", \"links\": {…} }`" + `. Errors are ` + "`{ \"error\": { \"code\": \"…\", \"message\": \"…\" } }`" + ` with a stable, lowercase ` + "`code`" + `.

## Pagination and limits

List endpoints accept ` + "`limit`" + ` and ` + "`offset`" + ` where the underlying resource supports them. Requests are rate limited per token; exceeding the limit returns ` + "`429`" + ` with a ` + "`Retry-After`" + ` header. Request bodies are limited to 1 MiB and each call has a 30 second deadline.`

// BuildSpecJSON renders the OpenAPI 3.1 document for serverURL as JSON.
func BuildSpecJSON(serverURL string) ([]byte, error) {
	spec, err := BuildSpec(serverURL)
	if err != nil {
		return nil, err
	}
	encoded, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("publicapi: encode spec: %w", err)
	}
	return append(encoded, '\n'), nil
}

// BuildSpec derives the OpenAPI document from Routes and the tool catalog.
func BuildSpec(serverURL string) (map[string]any, error) {
	if serverURL == "" {
		serverURL = DefaultServerURL
	}
	tools := map[string]service.MCPToolDefinition{}
	for _, tool := range service.PublicToolCatalog() {
		tools[tool.Name] = tool
	}
	if err := validateRoutes(tools); err != nil {
		return nil, err
	}

	paths := map[string]any{}
	for _, route := range Routes {
		tool := tools[route.Tool]
		operation, err := buildOperation(route, tool)
		if err != nil {
			return nil, err
		}
		item, _ := paths[route.Path].(map[string]any)
		if item == nil {
			item = map[string]any{}
			paths[route.Path] = item
		}
		item[strings.ToLower(route.Method)] = operation
	}

	tags := make([]any, 0, len(Tags))
	for _, tag := range Tags {
		tags = append(tags, map[string]any{"name": tag.Name, "description": tag.Description})
	}

	errorResponse := func(description string) map[string]any {
		return map[string]any{
			"description": description,
			"content": map[string]any{"application/json": map[string]any{
				"schema": map[string]any{"$ref": "#/components/schemas/Error"},
			}},
		}
	}
	return map[string]any{
		"openapi": "3.1.0",
		"info": map[string]any{
			"title":       "Helpin API",
			"version":     SpecVersion,
			"description": apiDescription,
		},
		"servers":  []any{map[string]any{"url": serverURL, "description": "Helpin Cloud. For self-hosted installations use your own API host followed by " + BasePath + "."}},
		"security": []any{map[string]any{"bearerAuth": []any{}}},
		"tags":     tags,
		"paths":    paths,
		"components": map[string]any{
			"securitySchemes": map[string]any{
				"bearerAuth": map[string]any{
					"type":         "http",
					"scheme":       "bearer",
					"bearerFormat": "Helpin service token (hmp_…)",
					"description":  "A workspace-bound automation-account token created in Settings → MCP access → Service accounts.",
				},
			},
			"parameters": map[string]any{
				"IdempotencyKey": map[string]any{
					"name": "Idempotency-Key", "in": "header", "required": false,
					"description": "Stable key (8–128 characters) that makes retries of this request safe.",
					"schema":      map[string]any{"type": "string", "minLength": 8, "maxLength": 128},
				},
			},
			"schemas": map[string]any{
				"Error": map[string]any{
					"type":     "object",
					"required": []any{"error"},
					"properties": map[string]any{"error": map[string]any{
						"type":     "object",
						"required": []any{"code", "message"},
						"properties": map[string]any{
							"code":    map[string]any{"type": "string", "description": "Stable machine-readable error code, for example forbidden or not_found."},
							"message": map[string]any{"type": "string"},
						},
					}},
				},
			},
			"responses": map[string]any{
				"BadRequest":    errorResponse("The request does not match the endpoint's schema (`invalid_arguments`)."),
				"Unauthorized":  errorResponse("The token is missing, invalid, expired, or revoked (`unauthorized`)."),
				"Forbidden":     errorResponse("The token's scopes, role, module access, read-only mode, or workspace policy do not allow this operation (`forbidden`, `api_disabled`)."),
				"NotFound":      errorResponse("The record does not exist in the workspace (`not_found`)."),
				"Conflict":      errorResponse("The request conflicts with current state, or the idempotency key was reused with a different body (`idempotency_conflict`, or a resource-specific code)."),
				"Unprocessable": errorResponse("The request is well formed but cannot be applied; the `code` says why."),
				"RateLimited": map[string]any{
					"description": "Rate limit exceeded (`rate_limited`).",
					"headers":     map[string]any{"Retry-After": map[string]any{"description": "Seconds to wait before retrying.", "schema": map[string]any{"type": "integer"}}},
					"content": map[string]any{"application/json": map[string]any{
						"schema": map[string]any{"$ref": "#/components/schemas/Error"},
					}},
				},
			},
		},
	}, nil
}

func buildOperation(route Route, tool service.MCPToolDefinition) (map[string]any, error) {
	schema, err := normalizeSchema(tool.InputSchema)
	if err != nil {
		return nil, fmt.Errorf("publicapi: tool %q schema: %w", tool.Name, err)
	}
	properties, _ := schema["properties"].(map[string]any)
	required := stringSet(schema["required"])
	pathNames := pathParams(route.Path)
	isPath := map[string]bool{}
	for _, name := range pathNames {
		isPath[name] = true
	}

	var parameters []any
	for _, name := range pathNames {
		parameter := map[string]any{"name": name, "in": "path", "required": true, "schema": paramSchema(properties[name])}
		if description := description(properties[name]); description != "" {
			parameter["description"] = description
		}
		parameters = append(parameters, parameter)
	}

	names := sortedKeys(properties)
	body := map[string]any{}
	bodyRequired := []any{}
	for _, name := range names {
		if isPath[name] || name == "idempotency_key" {
			continue
		}
		if route.Method == http.MethodGet {
			parameter := map[string]any{"name": name, "in": "query", "required": required[name], "schema": paramSchema(properties[name])}
			if description := description(properties[name]); description != "" {
				parameter["description"] = description
			}
			if prop, _ := properties[name].(map[string]any); prop["type"] == "array" {
				parameter["style"] = "form"
				parameter["explode"] = false
			}
			parameters = append(parameters, parameter)
			continue
		}
		body[name] = properties[name]
		if required[name] {
			bodyRequired = append(bodyRequired, name)
		}
	}
	if tool.Mutating {
		parameters = append(parameters, map[string]any{"$ref": "#/components/parameters/IdempotencyKey"})
	}

	status := route.Status
	if status == 0 {
		status = http.StatusOK
		if strings.HasPrefix(route.Tool, "create_") {
			status = http.StatusCreated
		}
	}
	responses := map[string]any{
		fmt.Sprint(status): map[string]any{
			"description": "Success.",
			"content": map[string]any{"application/json": map[string]any{"schema": map[string]any{
				"type":     "object",
				"required": []any{"data"},
				"properties": map[string]any{
					"data":    map[string]any{"description": "The result. Its shape depends on the resource; fields match what the Helpin app shows."},
					"summary": map[string]any{"type": "string", "description": "A short human-readable description of the outcome."},
					"links":   map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}, "description": "Deep links into the Helpin app."},
				},
			}}},
		},
		"400": map[string]any{"$ref": "#/components/responses/BadRequest"},
		"401": map[string]any{"$ref": "#/components/responses/Unauthorized"},
		"403": map[string]any{"$ref": "#/components/responses/Forbidden"},
		"429": map[string]any{"$ref": "#/components/responses/RateLimited"},
	}
	if len(pathNames) > 0 {
		responses["404"] = map[string]any{"$ref": "#/components/responses/NotFound"}
	}
	if tool.Mutating {
		responses["409"] = map[string]any{"$ref": "#/components/responses/Conflict"}
		responses["422"] = map[string]any{"$ref": "#/components/responses/Unprocessable"}
	}

	descriptionText := tool.Description + "\n\n**Required scope:** `" + tool.Scope + "`."
	if tool.Mutating {
		descriptionText += " Write operations are unavailable to read-only tokens."
	}
	operation := map[string]any{
		"operationId":       camelCase(tool.Name),
		"summary":           route.Summary,
		"description":       descriptionText,
		"tags":              []any{route.Tag},
		"parameters":        parameters,
		"responses":         responses,
		"x-helpin-scope":    tool.Scope,
		"x-helpin-toolset":  tool.Toolset,
		"x-helpin-mcp-tool": tool.Name,
	}
	if len(parameters) == 0 {
		delete(operation, "parameters")
	}
	if route.Method != http.MethodGet && len(body) > 0 {
		requestSchema := map[string]any{"type": "object", "properties": body, "additionalProperties": false}
		if len(bodyRequired) > 0 {
			requestSchema["required"] = bodyRequired
		}
		operation["requestBody"] = map[string]any{
			"required": len(bodyRequired) > 0,
			"content":  map[string]any{"application/json": map[string]any{"schema": requestSchema}},
		}
	}
	return operation, nil
}

// paramSchema strips the description (carried on the parameter) from a property schema.
func paramSchema(raw any) map[string]any {
	property, _ := raw.(map[string]any)
	out := make(map[string]any, len(property))
	for key, value := range property {
		if key != "description" {
			out[key] = value
		}
	}
	return out
}

func description(raw any) string {
	property, _ := raw.(map[string]any)
	text, _ := property["description"].(string)
	return text
}

func normalizeSchema(schema map[string]any) (map[string]any, error) {
	encoded, err := json.Marshal(schema)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(encoded, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func stringSet(raw any) map[string]bool {
	out := map[string]bool{}
	if values, ok := raw.([]any); ok {
		for _, value := range values {
			if text, ok := value.(string); ok {
				out[text] = true
			}
		}
	}
	return out
}

func sortedKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func camelCase(snake string) string {
	parts := strings.Split(snake, "_")
	for i := 1; i < len(parts); i++ {
		if parts[i] != "" {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}
