package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestDocsEntityReferenceResolverUnavailableFallback(t *testing.T) {
	resolver := NewDocsEntityReferenceResolverService(nil, nil, nil, nil, nil, nil, nil, nil)
	resolved, err := resolver.Resolve(context.Background(), "ws-1", model.ResolveDocsEntityRefsRequest{
		Refs: []model.DocsEntityRefRequest{{
			EntityType: "conversation",
			EntityID:   "conv-1",
			Label:      "Refund question",
			DisplayID:  42,
		}},
	})
	if err != nil {
		t.Fatalf("resolve refs: %v", err)
	}
	if len(resolved.Refs) != 1 {
		t.Fatalf("expected one ref, got %d", len(resolved.Refs))
	}
	ref := resolved.Refs[0]
	if ref.EntityType != "support_conversation" {
		t.Fatalf("expected normalized conversation type, got %q", ref.EntityType)
	}
	if ref.Status != docsEntityRefStatusUnavailable || ref.Access != docsEntityRefAccessUnavailable {
		t.Fatalf("expected unavailable ref, got status=%q access=%q", ref.Status, ref.Access)
	}
	if ref.Title != "Refund question" {
		t.Fatalf("expected snapshot label fallback, got %q", ref.Title)
	}
}

func TestDocsEntityReferenceResolverRedactsSnapshotLabelWithoutPermission(t *testing.T) {
	resolver := NewDocsEntityReferenceResolverService(nil, nil, nil, nil, nil, nil, nil, nil)
	ctx := authorization.WithActor(context.Background(), &authorization.Actor{
		UserID:      "user-1",
		WorkspaceID: "ws-1",
		Role:        "no_access_role",
	})
	resolved, err := resolver.Resolve(ctx, "ws-1", model.ResolveDocsEntityRefsRequest{
		Refs: []model.DocsEntityRefRequest{{
			EntityType: "contact",
			EntityID:   "contact-1",
			Label:      "Private Buyer",
			DisplayID:  "CON-1",
		}},
	})
	if err != nil {
		t.Fatalf("resolve refs: %v", err)
	}
	if len(resolved.Refs) != 1 {
		t.Fatalf("expected one ref, got %d", len(resolved.Refs))
	}
	ref := resolved.Refs[0]
	if ref.Access != docsEntityRefAccessRedacted {
		t.Fatalf("expected redacted access, got %q", ref.Access)
	}
	if ref.Title == "Private Buyer" || ref.DisplayID != nil {
		t.Fatalf("redacted ref leaked snapshot data: %+v", ref)
	}
	if ref.Title != "Restricted reference" {
		t.Fatalf("expected restricted title, got %q", ref.Title)
	}
}

func TestInlineEntityMentionRefsFindsNestedMentions(t *testing.T) {
	raw := json.RawMessage(`{
		"type":"paragraph",
		"content":[
			{"type":"text","text":"See "},
			{"type":"entityMention","attrs":{"entityType":"task","entityId":"task-1","label":"TASK-1 Fix login"}},
			{"type":"text","text":" and "},
			{"type":"entityMention","attrs":{"entityType":"contact","entityId":"contact-1","label":"Ada Lovelace"}}
		]
	}`)
	refs := inlineEntityMentionRefs(raw)
	if len(refs) != 2 {
		t.Fatalf("expected 2 refs, got %d", len(refs))
	}
	if refs[0].entityType != "task" || refs[0].entityID != "task-1" || refs[0].label != "TASK-1 Fix login" {
		t.Fatalf("unexpected first ref: %+v", refs[0])
	}
	if refs[1].entityType != "contact" || refs[1].entityID != "contact-1" || refs[1].label != "Ada Lovelace" {
		t.Fatalf("unexpected second ref: %+v", refs[1])
	}
}
