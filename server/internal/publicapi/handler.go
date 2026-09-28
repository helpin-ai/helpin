package publicapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

const (
	// BasePath is where the versioned API is mounted.
	BasePath = "/public/v1"

	maxBodyBytes    = 1 << 20
	requestDeadline = 30 * time.Second
)

// Backend is the public MCP service boundary the REST adapter delegates to.
type Backend interface {
	AuthenticateBearer(ctx context.Context, bearer string) (*model.MCPPrincipal, error)
	ExecuteTool(ctx context.Context, principal *model.MCPPrincipal, name string, arguments json.RawMessage) (*service.MCPToolResult, error)
}

// Limiter admits authenticated requests; expensive covers writes and search.
type Limiter interface {
	Allow(ctx context.Context, scope, subject string, expensive bool) bool
}

// Handler serves the public REST API.
type Handler struct {
	backend Backend
	limiter Limiter
	tools   map[string]service.MCPToolDefinition
	router  chi.Router
	spec    []byte
}

// NewHandler builds the REST adapter. limiter may be nil to disable admission checks.
func NewHandler(backend Backend, limiter Limiter, serverURL string) (*Handler, error) {
	catalog := service.PublicToolCatalog()
	tools := make(map[string]service.MCPToolDefinition, len(catalog))
	for _, tool := range catalog {
		tools[tool.Name] = tool
	}
	if err := validateRoutes(tools); err != nil {
		return nil, err
	}
	spec, err := BuildSpecJSON(serverURL)
	if err != nil {
		return nil, err
	}
	h := &Handler{backend: backend, limiter: limiter, tools: tools, spec: spec}
	r := chi.NewRouter()
	r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "No such endpoint. See "+BasePath+"/openapi.json.")
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "This endpoint does not support that HTTP method.")
	})
	r.Get("/openapi.json", h.serveSpec)
	r.Group(func(r chi.Router) {
		r.Use(h.authenticate)
		for _, route := range Routes {
			r.Method(route.Method, route.Path, h.operation(route))
		}
	})
	h.router = r
	return h, nil
}

// ServeHTTP dispatches requests under BasePath.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.StripPrefix(BasePath, h.router).ServeHTTP(w, r)
}

func (h *Handler) serveSpec(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=300")
	_, _ = w.Write(h.spec)
}

type principalKey struct{}

func (h *Handler) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bearer := bearerToken(r.Header.Get("Authorization"))
		if bearer == "" {
			w.Header().Set("WWW-Authenticate", `Bearer realm="helpin"`)
			writeError(w, http.StatusUnauthorized, "unauthorized", "Send an Authorization: Bearer <token> header with a Helpin service token.")
			return
		}
		principal, err := h.backend.AuthenticateBearer(r.Context(), bearer)
		if err != nil {
			if errors.Is(err, service.ErrMCPDisabled) {
				writeError(w, http.StatusForbidden, "api_disabled", "The Helpin API is disabled for this workspace or deployment.")
				return
			}
			if errors.Is(err, service.ErrMCPUnauthorized) {
				w.Header().Set("WWW-Authenticate", `Bearer realm="helpin", error="invalid_token"`)
				writeError(w, http.StatusUnauthorized, "unauthorized", "The token is invalid, expired, or revoked.")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal_error", "Authentication could not be completed.")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalKey{}, principal)))
	})
}

func (h *Handler) operation(route Route) http.Handler {
	tool := h.tools[route.Tool]
	schema := newToolSchema(tool.InputSchema)
	status := route.Status
	if status == 0 {
		status = http.StatusOK
		if strings.HasPrefix(route.Tool, "create_") {
			status = http.StatusCreated
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, _ := r.Context().Value(principalKey{}).(*model.MCPPrincipal)
		if principal == nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication is required.")
			return
		}
		if h.limiter != nil {
			expensive := tool.Mutating || tool.Name == "search_workspace"
			if !h.limiter.Allow(r.Context(), "publicapi", rateSubject(principal), expensive) {
				w.Header().Set("Retry-After", "60")
				writeError(w, http.StatusTooManyRequests, "rate_limited", "Rate limit exceeded. Retry after 60 seconds.")
				return
			}
		}
		args, err := buildArguments(r, route, schema, tool.Mutating)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_arguments", err.Error())
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), requestDeadline)
		defer cancel()
		result, err := h.backend.ExecuteTool(ctx, principal, route.Tool, args)
		if err != nil {
			code, httpStatus, message := classify(err)
			if httpStatus == http.StatusTooManyRequests {
				w.Header().Set("Retry-After", "60")
			}
			writeError(w, httpStatus, code, message)
			return
		}
		writeJSON(w, status, envelope{Data: result.Data, Summary: result.Summary, Links: result.Links})
	})
}

type envelope struct {
	Data    any               `json:"data"`
	Summary string            `json:"summary,omitempty"`
	Links   map[string]string `json:"links,omitempty"`
}

type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorBody{Error: errorDetail{Code: code, Message: message}})
}

// classify maps a service error to a stable code, HTTP status, and client-safe message.
func classify(err error) (code string, status int, message string) {
	var toolErr *service.MCPToolError
	if errors.As(err, &toolErr) {
		code = strings.ToLower(toolErr.Code)
		status = http.StatusUnprocessableEntity
		switch {
		case errors.Is(err, service.ErrMCPRateLimited):
			status = http.StatusTooManyRequests
		case strings.HasSuffix(code, "_not_found"):
			status = http.StatusNotFound
		case strings.HasSuffix(code, "_busy"), strings.HasPrefix(code, "document_"), strings.HasPrefix(code, "doc_"):
			status = http.StatusConflict
		}
		return code, status, toolErr.Message
	}
	switch {
	case errors.Is(err, service.ErrMCPUnauthorized):
		return "unauthorized", http.StatusUnauthorized, "The token is invalid, expired, or revoked."
	case errors.Is(err, service.ErrMCPDisabled):
		return "api_disabled", http.StatusForbidden, "The Helpin API is disabled for this workspace."
	case errors.Is(err, service.ErrMCPForbidden):
		return "forbidden", http.StatusForbidden, "The token's scopes, role, module access, or workspace policy do not allow this operation."
	case errors.Is(err, service.ErrMCPNotFound):
		return "not_found", http.StatusNotFound, "The requested record was not found in the workspace."
	case errors.Is(err, service.ErrMCPConflict):
		return "idempotency_conflict", http.StatusConflict, "That Idempotency-Key was already used for a different request."
	case errors.Is(err, service.ErrMCPInvalidArguments):
		return "invalid_arguments", http.StatusBadRequest, "The request does not match the endpoint's schema. Check required fields and value types."
	case errors.Is(err, service.ErrMCPRateLimited):
		return "rate_limited", http.StatusTooManyRequests, "A safety limit was reached. Retry later."
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout", http.StatusGatewayTimeout, "The operation exceeded the 30 second deadline."
	default:
		return "internal_error", http.StatusInternalServerError, "The request could not be completed."
	}
}

func rateSubject(principal *model.MCPPrincipal) string {
	actor := "user:" + principal.UserID
	if principal.ServicePrincipalID != "" {
		actor = "service:" + principal.ServicePrincipalID
	}
	return principal.WorkspaceID + ":" + actor
}

func bearerToken(header string) string {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return parts[1]
}

// toolSchema is the subset of a tool's JSON Schema needed to type request input.
type toolSchema struct {
	types map[string]propType
}

type propType struct {
	kind     string
	itemKind string
}

func newToolSchema(schema map[string]any) toolSchema {
	out := toolSchema{types: map[string]propType{}}
	properties, _ := schema["properties"].(map[string]any)
	for name, raw := range properties {
		out.types[name] = schemaType(raw)
	}
	return out
}

func schemaType(raw any) propType {
	property, _ := raw.(map[string]any)
	pt := propType{kind: primaryType(property["type"])}
	if pt.kind == "array" {
		if items, ok := property["items"].(map[string]any); ok {
			pt.itemKind = primaryType(items["type"])
		}
	}
	return pt
}

func primaryType(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []string:
		for _, item := range typed {
			if item != "null" {
				return item
			}
		}
	case []any:
		for _, item := range typed {
			if text, ok := item.(string); ok && text != "null" {
				return text
			}
		}
	}
	return "string"
}

// buildArguments merges path parameters, the query string or JSON body, and the
// idempotency key into one tool argument object.
func buildArguments(r *http.Request, route Route, schema toolSchema, mutating bool) (json.RawMessage, error) {
	args := map[string]any{}
	if r.Method == http.MethodGet {
		query := r.URL.Query()
		names := make([]string, 0, len(query))
		for name := range query {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			pt, ok := schema.types[name]
			if !ok || name == "idempotency_key" || isPathParam(route.Path, name) {
				return nil, fmt.Errorf("unknown query parameter %q", name)
			}
			value, err := coerceQuery(pt, query[name])
			if err != nil {
				return nil, fmt.Errorf("query parameter %q: %w", name, err)
			}
			args[name] = value
		}
	} else if r.Body != nil && r.Body != http.NoBody {
		body, err := io.ReadAll(http.MaxBytesReader(nil, r.Body, maxBodyBytes))
		if err != nil {
			return nil, errors.New("request body is too large or unreadable")
		}
		if len(strings.TrimSpace(string(body))) > 0 {
			if err := json.Unmarshal(body, &args); err != nil {
				return nil, errors.New("request body must be a JSON object")
			}
		}
	}
	if _, present := args["idempotency_key"]; present {
		return nil, errors.New("send the idempotency key in the Idempotency-Key header, not the body")
	}
	for _, name := range pathParams(route.Path) {
		value := chi.URLParam(r, name)
		if value == "" {
			return nil, fmt.Errorf("path parameter %q is required", name)
		}
		if _, clash := args[name]; clash {
			return nil, fmt.Errorf("%q is set by the URL and must not appear in the body or query", name)
		}
		decoded, err := url.PathUnescape(value)
		if err != nil {
			return nil, fmt.Errorf("path parameter %q is invalid", name)
		}
		args[name] = decoded
	}
	if mutating {
		key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if key == "" {
			key = "req-" + uuid.NewString()
		}
		args["idempotency_key"] = key
	}
	encoded, err := json.Marshal(args)
	if err != nil {
		return nil, errors.New("request could not be encoded")
	}
	return encoded, nil
}

func coerceQuery(pt propType, values []string) (any, error) {
	switch pt.kind {
	case "array":
		var parts []string
		for _, value := range values {
			for _, part := range strings.Split(value, ",") {
				if part = strings.TrimSpace(part); part != "" {
					parts = append(parts, part)
				}
			}
		}
		out := make([]any, 0, len(parts))
		for _, part := range parts {
			item, err := coerceScalar(pt.itemKind, part)
			if err != nil {
				return nil, err
			}
			out = append(out, item)
		}
		return out, nil
	case "object":
		return nil, errors.New("objects are not supported in the query string")
	default:
		if len(values) != 1 {
			return nil, errors.New("provide a single value")
		}
		return coerceScalar(pt.kind, values[0])
	}
}

func coerceScalar(kind, value string) (any, error) {
	switch kind {
	case "integer":
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return nil, errors.New("expected an integer")
		}
		return n, nil
	case "number":
		n, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, errors.New("expected a number")
		}
		return n, nil
	case "boolean":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return nil, errors.New("expected true or false")
		}
		return b, nil
	default:
		return value, nil
	}
}

func pathParams(path string) []string {
	var names []string
	for _, segment := range strings.Split(path, "/") {
		if strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}") {
			names = append(names, segment[1:len(segment)-1])
		}
	}
	return names
}

func isPathParam(path, name string) bool {
	for _, param := range pathParams(path) {
		if param == name {
			return true
		}
	}
	return false
}

func validateRoutes(tools map[string]service.MCPToolDefinition) error {
	seen := map[string]bool{}
	tags := map[string]bool{}
	for _, tag := range Tags {
		tags[tag.Name] = true
	}
	for _, route := range Routes {
		id := route.Method + " " + route.Path
		if seen[id] {
			return fmt.Errorf("publicapi: duplicate route %s", id)
		}
		seen[id] = true
		tool, ok := tools[route.Tool]
		if !ok {
			return fmt.Errorf("publicapi: route %s uses unknown tool %q", id, route.Tool)
		}
		if !tags[route.Tag] {
			return fmt.Errorf("publicapi: route %s uses unknown tag %q", id, route.Tag)
		}
		if tool.Mutating == (route.Method == http.MethodGet) {
			return fmt.Errorf("publicapi: route %s method does not match tool %q mutability", id, route.Tool)
		}
		properties, _ := tool.InputSchema["properties"].(map[string]any)
		for _, param := range pathParams(route.Path) {
			if _, ok := properties[param]; !ok {
				return fmt.Errorf("publicapi: route %s path parameter %q is not an argument of %q", id, param, route.Tool)
			}
		}
	}
	return nil
}
