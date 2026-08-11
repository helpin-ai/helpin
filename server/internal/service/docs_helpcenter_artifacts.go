package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"path"
	"strings"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/storage"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

const privateArtifactReferencePrefix = "helpin://artifacts/"

type publicationArtifactRepository interface {
	GetByIDAndWorkspace(ctx context.Context, workspaceID, artifactID string) (*model.AgentRunArtifact, error)
}

type publicationArtifactStore interface {
	GetObject(ctx context.Context, key string) ([]byte, error)
	PutObject(ctx context.Context, key, contentType string, size int64, body io.Reader, publicRead bool) error
	PublicURL(key string) string
}

// HasActivePublicationArtifactReferences reports whether renderable publication
// content still points at private agent artifacts. References retained inside
// publishedFrom metadata are intentionally ignored because they preserve source
// equality and are never rendered publicly.
func HasActivePublicationArtifactReferences(raw json.RawMessage) bool {
	if len(raw) == 0 || !bytes.Contains(raw, []byte(privateArtifactReferencePrefix)) {
		return false
	}
	var root tiptap.Node
	if err := json.Unmarshal(raw, &root); err != nil {
		return true
	}
	return publicationNodeHasUnresolvedArtifactReference(&root)
}

// MaterializeDocsPublicationArtifacts copies private artifact-backed media to
// permanent public help-center storage and rewrites the publication snapshot.
func MaterializeDocsPublicationArtifacts(
	ctx context.Context,
	artifactRepo *repository.AgentRunArtifactRepository,
	artifactStore *storage.S3Client,
	workspaceID string,
	documentID string,
	raw json.RawMessage,
) (json.RawMessage, error) {
	return materializePublicationArtifactReferences(ctx, artifactRepo, artifactStore, workspaceID, documentID, raw)
}

func materializePublicationArtifactReferences(
	ctx context.Context,
	artifactRepo publicationArtifactRepository,
	artifactStore publicationArtifactStore,
	workspaceID string,
	documentID string,
	raw json.RawMessage,
) (json.RawMessage, error) {
	if len(raw) == 0 || !bytes.Contains(raw, []byte(privateArtifactReferencePrefix)) {
		return raw, nil
	}

	var root tiptap.Node
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, fmt.Errorf("decode published artifact references: %w", err)
	}
	if err := materializePublicationArtifactNode(ctx, artifactRepo, artifactStore, workspaceID, documentID, &root); err != nil {
		return nil, err
	}

	materialized, err := json.Marshal(root)
	if err != nil {
		return nil, fmt.Errorf("encode published artifact references: %w", err)
	}
	if publicationNodeHasUnresolvedArtifactReference(&root) {
		return nil, fmt.Errorf("published content contains an unresolved private artifact reference")
	}
	return json.RawMessage(materialized), nil
}

func materializePublicationArtifactNode(
	ctx context.Context,
	artifactRepo publicationArtifactRepository,
	artifactStore publicationArtifactStore,
	workspaceID string,
	documentID string,
	node *tiptap.Node,
) error {
	var original tiptap.Node
	originalPayload, _ := json.Marshal(node)
	_ = json.Unmarshal(originalPayload, &original)
	changed := false

	if node.Attrs != nil {
		for _, attrName := range []string{"src", "darkSrc", "href", "poster"} {
			rawReference, ok := node.Attrs[attrName].(string)
			if !ok || !strings.HasPrefix(rawReference, privateArtifactReferencePrefix) {
				continue
			}
			artifactID, ok := publicationArtifactID(rawReference)
			if !ok {
				return fmt.Errorf("published content contains an invalid private artifact reference")
			}
			publicURL, err := materializePublicationArtifact(ctx, artifactRepo, artifactStore, workspaceID, documentID, artifactID)
			if err != nil {
				return err
			}
			node.Attrs[attrName] = publicURL
			changed = true
		}
		if changed {
			if _, exists := node.Attrs["publishedFrom"]; !exists {
				node.Attrs["publishedFrom"] = original
			}
		}
	}

	for i := range node.Content {
		if err := materializePublicationArtifactNode(ctx, artifactRepo, artifactStore, workspaceID, documentID, &node.Content[i]); err != nil {
			return err
		}
	}
	return nil
}

func publicationNodeHasUnresolvedArtifactReference(node *tiptap.Node) bool {
	for key, value := range node.Attrs {
		if key == "publishedFrom" {
			continue
		}
		if publicationValueHasArtifactReference(value) {
			return true
		}
	}
	for i := range node.Content {
		if publicationNodeHasUnresolvedArtifactReference(&node.Content[i]) {
			return true
		}
	}
	return false
}

func publicationValueHasArtifactReference(value any) bool {
	switch typed := value.(type) {
	case string:
		return strings.Contains(typed, privateArtifactReferencePrefix)
	case []any:
		for _, item := range typed {
			if publicationValueHasArtifactReference(item) {
				return true
			}
		}
	case map[string]any:
		for _, item := range typed {
			if publicationValueHasArtifactReference(item) {
				return true
			}
		}
	}
	return false
}

func publicationArtifactID(reference string) (string, bool) {
	artifactID := strings.TrimPrefix(strings.TrimSpace(reference), privateArtifactReferencePrefix)
	if artifactID == "" || strings.ContainsAny(artifactID, "/?#") {
		return "", false
	}
	if _, err := uuid.Parse(artifactID); err != nil {
		return "", false
	}
	return artifactID, true
}

func materializePublicationArtifact(
	ctx context.Context,
	artifactRepo publicationArtifactRepository,
	artifactStore publicationArtifactStore,
	workspaceID string,
	documentID string,
	artifactID string,
) (string, error) {
	if artifactRepo == nil || artifactStore == nil {
		return "", fmt.Errorf("private artifact publishing is not configured")
	}
	artifact, err := artifactRepo.GetByIDAndWorkspace(ctx, workspaceID, artifactID)
	if err != nil {
		return "", fmt.Errorf("load publication artifact %s: %w", artifactID, err)
	}
	if artifact == nil {
		return "", fmt.Errorf("publication artifact %s was not found", artifactID)
	}
	if artifact.StorageMode != "object" || artifact.ObjectKey == nil || strings.TrimSpace(*artifact.ObjectKey) == "" {
		return "", fmt.Errorf("publication artifact %s is not stored as an object", artifactID)
	}

	objectKey := strings.TrimSpace(*artifact.ObjectKey)
	content, err := artifactStore.GetObject(ctx, objectKey)
	if err != nil {
		return "", fmt.Errorf("read publication artifact %s: %w", artifactID, err)
	}
	contentType := publicationArtifactContentType(artifact, objectKey)
	extension := path.Ext(objectKey)
	if extension == "" {
		extension = publicationArtifactExtension(contentType)
	}
	publicKey := fmt.Sprintf("helpcenter/%s/articles/%s/artifacts/%s%s", workspaceID, documentID, artifactID, extension)
	if err := artifactStore.PutObject(ctx, publicKey, contentType, int64(len(content)), bytes.NewReader(content), true); err != nil {
		return "", fmt.Errorf("publish artifact %s: %w", artifactID, err)
	}
	publicURL := strings.TrimSpace(artifactStore.PublicURL(publicKey))
	if publicURL == "" {
		return "", fmt.Errorf("public URL is not configured for publication artifact %s", artifactID)
	}
	return publicURL, nil
}

func publicationArtifactContentType(artifact *model.AgentRunArtifact, objectKey string) string {
	var metadata struct {
		ContentType string `json:"content_type"`
	}
	if err := json.Unmarshal(artifact.Metadata, &metadata); err == nil && strings.TrimSpace(metadata.ContentType) != "" {
		return strings.TrimSpace(metadata.ContentType)
	}
	if contentType := mime.TypeByExtension(path.Ext(objectKey)); contentType != "" {
		return contentType
	}
	return "application/octet-stream"
}

func publicationArtifactExtension(contentType string) string {
	switch strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0])) {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/svg+xml":
		return ".svg"
	case "video/mp4":
		return ".mp4"
	case "video/webm":
		return ".webm"
	default:
		return ""
	}
}
