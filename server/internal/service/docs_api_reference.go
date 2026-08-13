package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const _maxOpenAPISpecBytes = 5 << 20

var (
	// ErrDocsAPIReferenceNotFound indicates that the requested reference does not exist.
	ErrDocsAPIReferenceNotFound = errors.New("API reference not found")
	// ErrDocsAPIReferenceExternalSpace indicates that API references require a public space.
	ErrDocsAPIReferenceExternalSpace = errors.New("API references can only be added to external spaces")
	// ErrDocsAPIReferenceInvalidSpec indicates that an OpenAPI document failed validation.
	ErrDocsAPIReferenceInvalidSpec = errors.New("invalid OpenAPI specification")
	// ErrDocsAPIReferenceInvalidSource indicates unsupported or incomplete source configuration.
	ErrDocsAPIReferenceInvalidSource = errors.New("invalid API reference source")
)

// DocsAPIReferenceService manages validated and publishable OpenAPI snapshots.
type DocsAPIReferenceService struct {
	referenceRepo *repository.DocsAPIReferenceRepository
	spaceRepo     *repository.DocsSpaceRepository
	httpClient    *http.Client
}

// NewDocsAPIReferenceService creates a DocsAPIReferenceService.
func NewDocsAPIReferenceService(
	referenceRepo *repository.DocsAPIReferenceRepository,
	spaceRepo *repository.DocsSpaceRepository,
) *DocsAPIReferenceService {
	return &DocsAPIReferenceService{
		referenceRepo: referenceRepo,
		spaceRepo:     spaceRepo,
		httpClient:    newDocsOpenAPIHTTPClient(),
	}
}

// SetHTTPClient replaces the source client, primarily for deterministic tests.
func (s *DocsAPIReferenceService) SetHTTPClient(client *http.Client) {
	if client != nil {
		s.httpClient = client
	}
}

// Create validates a source and creates the first draft revision.
func (s *DocsAPIReferenceService) Create(
	ctx context.Context,
	workspaceID, spaceID, userID string,
	req model.CreateDocsAPIReferenceRequest,
) (*model.DocsAPIReferenceResponse, error) {
	space, err := s.requireExternalSpace(ctx, workspaceID, spaceID)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("%w: name is required", ErrDocsAPIReferenceInvalidSource)
	}
	sourceType := strings.TrimSpace(strings.ToLower(req.SourceType))
	if sourceType == "" {
		if strings.TrimSpace(req.SourceURL) != "" {
			sourceType = model.DocsAPIReferenceSourceURL
		} else {
			sourceType = model.DocsAPIReferenceSourceUpload
		}
	}
	sourceText, sourceURL, err := s.resolveSource(ctx, sourceType, req.SourceURL, req.SpecificationText)
	if err != nil {
		return nil, err
	}
	revision, err := buildDocsAPIReferenceRevision(sourceText, workspaceID, userID)
	if err != nil {
		return nil, err
	}
	position, err := s.referenceRepo.NextPosition(ctx, space.ID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	reference := &model.DocsAPIReference{
		WorkspaceID:  workspaceID,
		SpaceID:      space.ID,
		Name:         name,
		Slug:         normalizedDocsAPIReferenceSlug(req.Slug, name),
		SourceType:   sourceType,
		SourceURL:    sourceURL,
		SyncEnabled:  req.SyncEnabled && sourceType == model.DocsAPIReferenceSourceURL,
		SyncStatus:   model.DocsAPIReferenceSyncReady,
		LastSyncedAt: &now,
		Position:     position,
		CreatedBy:    userID,
	}
	created, err := s.referenceRepo.Create(ctx, reference, revision)
	if err != nil {
		return nil, err
	}
	return s.expand(ctx, created, true)
}

// List returns references in a space without embedding full specifications.
func (s *DocsAPIReferenceService) List(
	ctx context.Context,
	workspaceID, spaceID string,
) ([]model.DocsAPIReferenceResponse, error) {
	if _, err := s.requireSpace(ctx, workspaceID, spaceID); err != nil {
		return nil, err
	}
	references, err := s.referenceRepo.ListBySpace(ctx, workspaceID, spaceID)
	if err != nil {
		return nil, err
	}
	result := make([]model.DocsAPIReferenceResponse, 0, len(references))
	for i := range references {
		expanded, err := s.expand(ctx, &references[i], false)
		if err != nil {
			return nil, err
		}
		result = append(result, *expanded)
	}
	return result, nil
}

// Get returns a reference and both current revision snapshots.
func (s *DocsAPIReferenceService) Get(
	ctx context.Context,
	workspaceID, id string,
) (*model.DocsAPIReferenceResponse, error) {
	reference, err := s.requireReference(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	return s.expand(ctx, reference, true)
}

// Update changes reference metadata and optionally creates a new uploaded draft.
func (s *DocsAPIReferenceService) Update(
	ctx context.Context,
	workspaceID, id, userID string,
	req model.UpdateDocsAPIReferenceRequest,
) (*model.DocsAPIReferenceResponse, error) {
	reference, err := s.requireReference(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	updates := map[string]any{}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, fmt.Errorf("%w: name is required", ErrDocsAPIReferenceInvalidSource)
		}
		updates["name"] = name
	}
	if req.Slug != nil {
		updates["slug"] = normalizedDocsAPIReferenceSlug(*req.Slug, reference.Name)
	}
	if req.SyncEnabled != nil {
		updates["sync_enabled"] = *req.SyncEnabled && reference.SourceType == model.DocsAPIReferenceSourceURL
	}
	if req.SourceURL != nil {
		if reference.SourceType != model.DocsAPIReferenceSourceURL {
			return nil, fmt.Errorf("%w: uploaded references do not have a source URL", ErrDocsAPIReferenceInvalidSource)
		}
		normalized, err := validateDocsOpenAPIURL(*req.SourceURL)
		if err != nil {
			return nil, err
		}
		updates["source_url"] = normalized.String()
	}
	updated, err := s.referenceRepo.Update(ctx, id, updates)
	if err != nil {
		return nil, err
	}
	if req.SpecificationText != nil {
		revision, buildErr := buildDocsAPIReferenceRevision(*req.SpecificationText, workspaceID, userID)
		if buildErr != nil {
			return nil, buildErr
		}
		if err := s.referenceRepo.CreateDraft(ctx, id, revision, time.Now().UTC()); err != nil {
			return nil, err
		}
		updated, err = s.referenceRepo.GetByID(ctx, id)
		if err != nil {
			return nil, err
		}
	}
	return s.expand(ctx, updated, true)
}

// Sync fetches a URL source and creates a new draft only when its content changed.
func (s *DocsAPIReferenceService) Sync(
	ctx context.Context,
	workspaceID, id, userID string,
) (*model.DocsAPIReferenceResponse, error) {
	reference, err := s.requireReference(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if reference.SourceType != model.DocsAPIReferenceSourceURL || reference.SourceURL == nil {
		return nil, fmt.Errorf("%w: only URL sources can be synchronized", ErrDocsAPIReferenceInvalidSource)
	}
	sourceText, err := s.fetchSource(ctx, *reference.SourceURL)
	if err != nil {
		_ = s.referenceRepo.RecordSyncFailure(ctx, id, err.Error())
		return nil, err
	}
	revision, err := buildDocsAPIReferenceRevision(sourceText, workspaceID, userID)
	if err != nil {
		_ = s.referenceRepo.RecordSyncFailure(ctx, id, err.Error())
		return nil, err
	}
	current, err := s.referenceRepo.GetRevision(ctx, docsAPIReferenceStringValue(reference.DraftRevisionID))
	if err != nil {
		return nil, err
	}
	if current == nil || current.SourceHash != revision.SourceHash {
		if err := s.referenceRepo.CreateDraft(ctx, id, revision, time.Now().UTC()); err != nil {
			return nil, err
		}
	} else {
		_, err = s.referenceRepo.Update(ctx, id, map[string]any{
			"sync_status":     model.DocsAPIReferenceSyncReady,
			"last_sync_error": nil,
			"last_synced_at":  time.Now().UTC(),
		})
		if err != nil {
			return nil, err
		}
	}
	return s.Get(ctx, workspaceID, id)
}

// Publish promotes the current draft revision.
func (s *DocsAPIReferenceService) Publish(
	ctx context.Context,
	workspaceID, id string,
) (*model.DocsAPIReferenceResponse, error) {
	reference, err := s.requireReference(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if reference.DraftRevisionID == nil {
		return nil, fmt.Errorf("%w: no draft revision exists", ErrDocsAPIReferenceInvalidSpec)
	}
	updated, err := s.referenceRepo.Publish(ctx, id, *reference.DraftRevisionID, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	return s.expand(ctx, updated, true)
}

// Unpublish removes a reference from the public help center.
func (s *DocsAPIReferenceService) Unpublish(
	ctx context.Context,
	workspaceID, id string,
) (*model.DocsAPIReferenceResponse, error) {
	if _, err := s.requireReference(ctx, workspaceID, id); err != nil {
		return nil, err
	}
	updated, err := s.referenceRepo.Unpublish(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.expand(ctx, updated, true)
}

// Delete removes an API reference from its space.
func (s *DocsAPIReferenceService) Delete(ctx context.Context, workspaceID, id string) error {
	if _, err := s.requireReference(ctx, workspaceID, id); err != nil {
		return err
	}
	return s.referenceRepo.Delete(ctx, id)
}

// ListPublic returns published reference summaries for a public space.
func (s *DocsAPIReferenceService) ListPublic(
	ctx context.Context,
	workspaceID, spaceID string,
) ([]model.PublicDocsAPIReferenceSummary, error) {
	space, err := s.requireExternalSpace(ctx, workspaceID, spaceID)
	if errors.Is(err, ErrDocsAPIReferenceExternalSpace) {
		return nil, ErrDocsSpaceNotFound
	}
	if err != nil {
		return nil, err
	}
	references, err := s.referenceRepo.ListPublishedBySpace(ctx, workspaceID, space.ID)
	if err != nil {
		return nil, err
	}
	result := make([]model.PublicDocsAPIReferenceSummary, 0, len(references))
	for i := range references {
		revision, err := s.referenceRepo.GetRevision(ctx, docsAPIReferenceStringValue(references[i].PublishedRevisionID))
		if err != nil {
			return nil, err
		}
		if revision == nil {
			continue
		}
		result = append(result, publicDocsAPIReferenceSummary(&references[i], revision))
	}
	return result, nil
}

// GetPublic returns a published OpenAPI snapshot by public space and reference slugs.
func (s *DocsAPIReferenceService) GetPublic(
	ctx context.Context,
	workspaceID, spaceID, referenceSlug string,
) (*model.PublicDocsAPIReferenceResponse, error) {
	space, err := s.requireExternalSpace(ctx, workspaceID, spaceID)
	if errors.Is(err, ErrDocsSpaceNotFound) ||
		errors.Is(err, ErrDocsCrossWorkspace) ||
		errors.Is(err, ErrDocsAPIReferenceExternalSpace) {
		return nil, ErrDocsAPIReferenceNotFound
	}
	if err != nil {
		return nil, err
	}
	reference, err := s.referenceRepo.GetPublishedBySlug(ctx, workspaceID, space.ID, strings.TrimSpace(referenceSlug))
	if err != nil {
		return nil, err
	}
	if reference == nil {
		return nil, ErrDocsAPIReferenceNotFound
	}
	revision, err := s.referenceRepo.GetRevision(ctx, docsAPIReferenceStringValue(reference.PublishedRevisionID))
	if err != nil {
		return nil, err
	}
	if revision == nil {
		return nil, ErrDocsAPIReferenceNotFound
	}
	return &model.PublicDocsAPIReferenceResponse{
		PublicDocsAPIReferenceSummary: publicDocsAPIReferenceSummary(reference, revision),
		OpenAPIVersion:                revision.OpenAPIVersion,
		Specification:                 revision.Specification,
		PublishedAt:                   reference.PublishedAt,
	}, nil
}

func (s *DocsAPIReferenceService) requireSpace(
	ctx context.Context,
	workspaceID, spaceID string,
) (*model.DocsSpace, error) {
	space, err := s.spaceRepo.GetByID(ctx, spaceID)
	if err != nil {
		return nil, err
	}
	if space == nil {
		return nil, ErrDocsSpaceNotFound
	}
	if space.WorkspaceID != workspaceID {
		return nil, ErrDocsCrossWorkspace
	}
	return space, nil
}

func (s *DocsAPIReferenceService) requireExternalSpace(
	ctx context.Context,
	workspaceID, spaceID string,
) (*model.DocsSpace, error) {
	space, err := s.requireSpace(ctx, workspaceID, spaceID)
	if err != nil {
		return nil, err
	}
	if space.Type != model.SpaceTypeExternalCapable {
		return nil, ErrDocsAPIReferenceExternalSpace
	}
	return space, nil
}

func (s *DocsAPIReferenceService) requireReference(
	ctx context.Context,
	workspaceID, id string,
) (*model.DocsAPIReference, error) {
	reference, err := s.referenceRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if reference == nil {
		return nil, ErrDocsAPIReferenceNotFound
	}
	if reference.WorkspaceID != workspaceID {
		return nil, ErrDocsCrossWorkspace
	}
	return reference, nil
}

func (s *DocsAPIReferenceService) resolveSource(
	ctx context.Context,
	sourceType, sourceURL, specificationText string,
) (string, *string, error) {
	switch sourceType {
	case model.DocsAPIReferenceSourceURL:
		normalizedURL, err := validateDocsOpenAPIURL(sourceURL)
		if err != nil {
			return "", nil, err
		}
		content, err := s.fetchSource(ctx, normalizedURL.String())
		if err != nil {
			return "", nil, err
		}
		value := normalizedURL.String()
		return content, &value, nil
	case model.DocsAPIReferenceSourceUpload:
		if strings.TrimSpace(specificationText) == "" {
			return "", nil, fmt.Errorf("%w: an OpenAPI file is required", ErrDocsAPIReferenceInvalidSource)
		}
		if len(specificationText) > _maxOpenAPISpecBytes {
			return "", nil, fmt.Errorf("%w: specification exceeds 5 MB", ErrDocsAPIReferenceInvalidSpec)
		}
		return specificationText, nil, nil
	default:
		return "", nil, fmt.Errorf("%w: source_type must be url or upload", ErrDocsAPIReferenceInvalidSource)
	}
}

func (s *DocsAPIReferenceService) fetchSource(ctx context.Context, rawURL string) (string, error) {
	normalizedURL, err := validateDocsOpenAPIURL(rawURL)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, normalizedURL.String(), nil)
	if err != nil {
		return "", fmt.Errorf("%w: invalid source URL", ErrDocsAPIReferenceInvalidSource)
	}
	req.Header.Set("Accept", "application/json, application/yaml, text/yaml, text/plain")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch OpenAPI source: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("fetch OpenAPI source: server returned %s", resp.Status)
	}
	limited := io.LimitReader(resp.Body, _maxOpenAPISpecBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return "", fmt.Errorf("read OpenAPI source: %w", err)
	}
	if len(body) > _maxOpenAPISpecBytes {
		return "", fmt.Errorf("%w: specification exceeds 5 MB", ErrDocsAPIReferenceInvalidSpec)
	}
	return string(body), nil
}

func (s *DocsAPIReferenceService) expand(
	ctx context.Context,
	reference *model.DocsAPIReference,
	includeSpecification bool,
) (*model.DocsAPIReferenceResponse, error) {
	response := &model.DocsAPIReferenceResponse{DocsAPIReference: *reference}
	var err error
	response.DraftRevision, err = s.referenceRepo.GetRevision(ctx, docsAPIReferenceStringValue(reference.DraftRevisionID))
	if err != nil {
		return nil, err
	}
	response.PublishedRevision, err = s.referenceRepo.GetRevision(ctx, docsAPIReferenceStringValue(reference.PublishedRevisionID))
	if err != nil {
		return nil, err
	}
	if !includeSpecification {
		if response.DraftRevision != nil {
			response.DraftRevision.Specification = nil
		}
		if response.PublishedRevision != nil {
			response.PublishedRevision.Specification = nil
		}
	}
	return response, nil
}

func buildDocsAPIReferenceRevision(
	sourceText, workspaceID, userID string,
) (*model.DocsAPIReferenceRevision, error) {
	normalized, document, err := normalizeDocsOpenAPISpec(sourceText)
	if err != nil {
		return nil, err
	}
	openAPIVersion, _ := document["openapi"].(string)
	info, _ := document["info"].(map[string]any)
	apiVersion, _ := info["version"].(string)
	operationCount, schemaCount, warnings := inspectDocsOpenAPISpec(document)
	sum := sha256.Sum256(normalized)
	return &model.DocsAPIReferenceRevision{
		WorkspaceID:    workspaceID,
		SourceHash:     hex.EncodeToString(sum[:]),
		OpenAPIVersion: openAPIVersion,
		APIVersion:     strings.TrimSpace(apiVersion),
		Specification:  normalized,
		Warnings:       model.DocsStringArray(warnings),
		OperationCount: operationCount,
		SchemaCount:    schemaCount,
		CreatedBy:      userID,
	}, nil
}

func normalizeDocsOpenAPISpec(sourceText string) (json.RawMessage, map[string]any, error) {
	if strings.TrimSpace(sourceText) == "" {
		return nil, nil, fmt.Errorf("%w: specification is empty", ErrDocsAPIReferenceInvalidSpec)
	}
	var document map[string]any
	trimmedSource := strings.TrimSpace(sourceText)
	if strings.HasPrefix(trimmedSource, "{") || strings.HasPrefix(trimmedSource, "[") {
		decoder := json.NewDecoder(bytes.NewBufferString(sourceText))
		decoder.UseNumber()
		if err := decoder.Decode(&document); err != nil {
			return nil, nil, fmt.Errorf("%w: expected valid JSON", ErrDocsAPIReferenceInvalidSpec)
		}
		var trailing any
		if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
			return nil, nil, fmt.Errorf("%w: JSON contains trailing content", ErrDocsAPIReferenceInvalidSpec)
		}
	} else {
		var yamlDocument any
		if yamlErr := yaml.Unmarshal([]byte(sourceText), &yamlDocument); yamlErr != nil {
			return nil, nil, fmt.Errorf("%w: expected JSON or YAML", ErrDocsAPIReferenceInvalidSpec)
		}
		normalizedYAML, normalizeErr := json.Marshal(yamlDocument)
		if normalizeErr != nil {
			return nil, nil, fmt.Errorf("%w: YAML contains unsupported values", ErrDocsAPIReferenceInvalidSpec)
		}
		if err := json.Unmarshal(normalizedYAML, &document); err != nil {
			return nil, nil, fmt.Errorf("%w: YAML root must be an object", ErrDocsAPIReferenceInvalidSpec)
		}
	}
	openAPIVersion, _ := document["openapi"].(string)
	if !strings.HasPrefix(openAPIVersion, "3.0.") && !strings.HasPrefix(openAPIVersion, "3.1.") {
		return nil, nil, fmt.Errorf("%w: only OpenAPI 3.0 and 3.1 are supported", ErrDocsAPIReferenceInvalidSpec)
	}
	info, ok := document["info"].(map[string]any)
	if !ok {
		return nil, nil, fmt.Errorf("%w: info is required", ErrDocsAPIReferenceInvalidSpec)
	}
	title, _ := info["title"].(string)
	if strings.TrimSpace(title) == "" {
		return nil, nil, fmt.Errorf("%w: info.title is required", ErrDocsAPIReferenceInvalidSpec)
	}
	if _, ok := document["paths"].(map[string]any); !ok {
		return nil, nil, fmt.Errorf("%w: paths must be an object", ErrDocsAPIReferenceInvalidSpec)
	}
	if externalRef := findExternalDocsOpenAPIRef(document); externalRef != "" {
		return nil, nil, fmt.Errorf("%w: external $ref values are not supported (%s)", ErrDocsAPIReferenceInvalidSpec, externalRef)
	}
	normalized, err := json.Marshal(document)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: cannot normalize specification", ErrDocsAPIReferenceInvalidSpec)
	}
	return normalized, document, nil
}

func inspectDocsOpenAPISpec(document map[string]any) (int, int, []string) {
	methods := map[string]bool{
		"get": true, "post": true, "put": true, "patch": true,
		"delete": true, "options": true, "head": true, "trace": true,
	}
	paths, _ := document["paths"].(map[string]any)
	operationCount := 0
	missingOperationIDs := 0
	operationIDs := map[string]bool{}
	duplicateOperationIDs := map[string]bool{}
	for _, rawPath := range paths {
		pathItem, _ := rawPath.(map[string]any)
		for method, rawOperation := range pathItem {
			if !methods[strings.ToLower(method)] {
				continue
			}
			operation, _ := rawOperation.(map[string]any)
			if operation == nil {
				continue
			}
			operationCount++
			operationID, _ := operation["operationId"].(string)
			operationID = strings.TrimSpace(operationID)
			if operationID == "" {
				missingOperationIDs++
			} else if operationIDs[operationID] {
				duplicateOperationIDs[operationID] = true
			} else {
				operationIDs[operationID] = true
			}
		}
	}
	schemaCount := 0
	if components, ok := document["components"].(map[string]any); ok {
		if schemas, ok := components["schemas"].(map[string]any); ok {
			schemaCount = len(schemas)
		}
	}
	var warnings []string
	if operationCount == 0 {
		warnings = append(warnings, "The specification does not contain any operations.")
	}
	if missingOperationIDs > 0 {
		warnings = append(warnings, fmt.Sprintf("%d operations do not define operationId.", missingOperationIDs))
	}
	if len(duplicateOperationIDs) > 0 {
		warnings = append(warnings, fmt.Sprintf("%d operationId values are duplicated.", len(duplicateOperationIDs)))
	}
	return operationCount, schemaCount, warnings
}

func findExternalDocsOpenAPIRef(value any) string {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if key == "$ref" {
				if ref, ok := child.(string); ok && !strings.HasPrefix(strings.TrimSpace(ref), "#/") {
					return ref
				}
			}
			if ref := findExternalDocsOpenAPIRef(child); ref != "" {
				return ref
			}
		}
	case []any:
		for _, child := range typed {
			if ref := findExternalDocsOpenAPIRef(child); ref != "" {
				return ref
			}
		}
	}
	return ""
}

func validateDocsOpenAPIURL(rawURL string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" {
		return nil, fmt.Errorf("%w: source URL must be a valid HTTPS URL", ErrDocsAPIReferenceInvalidSource)
	}
	if parsed.User != nil {
		return nil, fmt.Errorf("%w: source URL must not contain credentials", ErrDocsAPIReferenceInvalidSource)
	}
	if parsed.Port() != "" && parsed.Port() != "443" {
		return nil, fmt.Errorf("%w: source URL must use the standard HTTPS port", ErrDocsAPIReferenceInvalidSource)
	}
	if ip := net.ParseIP(parsed.Hostname()); ip != nil && !isPublicDocsOpenAPIIP(ip) {
		return nil, fmt.Errorf("%w: source URL cannot target a private network", ErrDocsAPIReferenceInvalidSource)
	}
	parsed.Fragment = ""
	return parsed, nil
}

func newDocsOpenAPIHTTPClient() *http.Client {
	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
	resolver := net.DefaultResolver
	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, fmt.Errorf("invalid OpenAPI source address")
			}
			ips, err := resolver.LookupIP(ctx, "ip", host)
			if err != nil {
				return nil, err
			}
			for _, ip := range ips {
				if !isPublicDocsOpenAPIIP(ip) {
					return nil, fmt.Errorf("OpenAPI source resolves to a private network")
				}
			}
			if len(ips) == 0 {
				return nil, fmt.Errorf("OpenAPI source has no address")
			}
			return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
		},
		TLSHandshakeTimeout: 5 * time.Second,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   12 * time.Second,
	}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return fmt.Errorf("too many OpenAPI source redirects")
		}
		_, err := validateDocsOpenAPIURL(req.URL.String())
		return err
	}
	return client
}

func isPublicDocsOpenAPIIP(ip net.IP) bool {
	return !ip.IsLoopback() &&
		!ip.IsPrivate() &&
		!ip.IsUnspecified() &&
		!ip.IsLinkLocalUnicast() &&
		!ip.IsLinkLocalMulticast() &&
		!ip.IsMulticast()
}

func publicDocsAPIReferenceSummary(
	reference *model.DocsAPIReference,
	revision *model.DocsAPIReferenceRevision,
) model.PublicDocsAPIReferenceSummary {
	return model.PublicDocsAPIReferenceSummary{
		ID:             reference.ID,
		SpaceID:        reference.SpaceID,
		Name:           reference.Name,
		Slug:           reference.Slug,
		APIVersion:     revision.APIVersion,
		OperationCount: revision.OperationCount,
	}
}

func normalizedDocsAPIReferenceSlug(slug, fallback string) string {
	normalized := slugify(strings.TrimSpace(slug))
	if normalized == "" {
		normalized = slugify(strings.TrimSpace(fallback))
	}
	return normalized
}

func docsAPIReferenceStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
