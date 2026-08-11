package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

type fakePublicationArtifactRepository struct {
	artifacts map[string]*model.AgentRunArtifact
}

func (r *fakePublicationArtifactRepository) GetByIDAndWorkspace(_ context.Context, workspaceID, artifactID string) (*model.AgentRunArtifact, error) {
	artifact := r.artifacts[artifactID]
	if artifact == nil || artifact.WorkspaceID != workspaceID {
		return nil, nil
	}
	return artifact, nil
}

type fakePublicationArtifactStore struct {
	objects    map[string][]byte
	puts       map[string][]byte
	publicRead map[string]bool
}

func (s *fakePublicationArtifactStore) GetObject(_ context.Context, key string) ([]byte, error) {
	return s.objects[key], nil
}

func (s *fakePublicationArtifactStore) PutObject(_ context.Context, key, _ string, _ int64, body io.Reader, publicRead bool) error {
	content, _ := io.ReadAll(body)
	s.puts[key] = content
	s.publicRead[key] = publicRead
	return nil
}

func (s *fakePublicationArtifactStore) PublicURL(key string) string {
	return "https://assets.example.com/" + key
}

func TestMaterializePublicationArtifactReferences(t *testing.T) {
	const (
		workspaceID = "a4db06cf-89fc-45d5-bf5a-8cd208e9695d"
		documentID  = "42050872-8a25-438e-8e6f-9d7614d84417"
		artifactID  = "2f3cd104-c31f-4102-a95d-338248870d88"
	)
	objectKey := "agent-runs/ws/run/browser/" + artifactID + ".png"
	repo := &fakePublicationArtifactRepository{artifacts: map[string]*model.AgentRunArtifact{
		artifactID: {
			ID:          artifactID,
			WorkspaceID: workspaceID,
			StorageMode: "object",
			ObjectKey:   &objectKey,
			Metadata:    json.RawMessage(`{"content_type":"image/png"}`),
		},
	}}
	store := &fakePublicationArtifactStore{
		objects:    map[string][]byte{objectKey: []byte("png-content")},
		puts:       map[string][]byte{},
		publicRead: map[string]bool{},
	}
	raw := json.RawMessage(`{"type":"doc","content":[{"type":"resizableImage","attrs":{"src":"helpin://artifacts/` + artifactID + `","artifactId":"` + artifactID + `","alt":"Trust Center"}}]}`)

	got, err := materializePublicationArtifactReferences(context.Background(), repo, store, workspaceID, documentID, raw)
	if err != nil {
		t.Fatalf("materialize publication artifacts: %v", err)
	}
	rendered, err := tiptap.RenderHTML(got)
	if err != nil {
		t.Fatalf("render materialized content: %v", err)
	}
	if strings.Contains(rendered, privateArtifactReferencePrefix) {
		t.Fatalf("rendered content still contains private reference: %s", rendered)
	}
	publicKey := "helpcenter/" + workspaceID + "/articles/" + documentID + "/artifacts/" + artifactID + ".png"
	if !bytes.Equal(store.puts[publicKey], []byte("png-content")) {
		t.Fatalf("published object = %q, want png-content", store.puts[publicKey])
	}
	if !store.publicRead[publicKey] {
		t.Fatal("published artifact must be public")
	}
	if !strings.Contains(string(got), "https://assets.example.com/"+publicKey) {
		t.Fatalf("materialized content does not contain public URL: %s", got)
	}
	if !publicationContentEqual(raw, got) {
		t.Fatalf("materialized publication must compare equal to its source\nsource: %s\npublished: %s", raw, got)
	}
}

func TestMaterializePublicationArtifactReferencesRejectsUnresolvedReference(t *testing.T) {
	raw := json.RawMessage(`{"type":"doc","content":[{"type":"htmlBlock","attrs":{"html":"<img src=\"helpin://artifacts/2f3cd104-c31f-4102-a95d-338248870d88\">"}}]}`)

	_, err := materializePublicationArtifactReferences(context.Background(), nil, nil, "ws-1", "doc-1", raw)
	if err == nil || !strings.Contains(err.Error(), "unresolved private artifact reference") {
		t.Fatalf("error = %v, want unresolved private artifact reference", err)
	}
}

func TestMaterializePublicationArtifactReferencesRejectsMissingArtifact(t *testing.T) {
	const artifactID = "2f3cd104-c31f-4102-a95d-338248870d88"
	raw := json.RawMessage(`{"type":"doc","content":[{"type":"resizableImage","attrs":{"src":"helpin://artifacts/` + artifactID + `"}}]}`)
	repo := &fakePublicationArtifactRepository{artifacts: map[string]*model.AgentRunArtifact{}}
	store := &fakePublicationArtifactStore{objects: map[string][]byte{}, puts: map[string][]byte{}, publicRead: map[string]bool{}}

	_, err := materializePublicationArtifactReferences(context.Background(), repo, store, "ws-1", "doc-1", raw)
	if err == nil || !strings.Contains(err.Error(), "was not found") {
		t.Fatalf("error = %v, want artifact not found", err)
	}
}
