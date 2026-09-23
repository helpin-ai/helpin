package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

// internalDocumentLinkPrefix is the in-app link form for Helpin documents,
// written by the editor's document mentions and by MCP markdown links.
const internalDocumentLinkPrefix = "helpin://documents/"

// publicationArticlePathResolver returns the public Help Center path for a
// document, or ok=false when the document is not a live article.
type publicationArticlePathResolver func(ctx context.Context, documentID string) (path string, ok bool, err error)

// materializePublicationDocumentLinks rewrites in-app document links in a
// public snapshot. Links to live articles become public article paths; links
// to anything else lose the link and keep their text, so readers never reach
// an in-app URL they cannot open.
func materializePublicationDocumentLinks(ctx context.Context, raw json.RawMessage, resolve publicationArticlePathResolver) (json.RawMessage, error) {
	if len(raw) == 0 || !bytes.Contains(raw, []byte(internalDocumentLinkPrefix)) {
		return raw, nil
	}
	var root tiptap.Node
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, fmt.Errorf("decode published document links: %w", err)
	}
	if err := materializePublicationDocumentLinkNode(ctx, &root, resolve); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(root)
	if err != nil {
		return nil, fmt.Errorf("encode published document links: %w", err)
	}
	return json.RawMessage(encoded), nil
}

func materializePublicationDocumentLinkNode(ctx context.Context, node *tiptap.Node, resolve publicationArticlePathResolver) error {
	if nodeHasInternalDocumentLink(node) {
		// Keep the source node so unpublished-change detection compares the
		// draft with what the author wrote, not with the rewritten link.
		original := tiptap.Node{Type: node.Type, Text: node.Text, Marks: append([]tiptap.Mark(nil), node.Marks...)}
		marks := make([]tiptap.Mark, 0, len(node.Marks))
		for _, mark := range node.Marks {
			if mark.Type != "link" {
				marks = append(marks, mark)
				continue
			}
			href, _ := mark.Attrs["href"].(string)
			documentID, internal := internalDocumentLinkID(href)
			if !internal {
				marks = append(marks, mark)
				continue
			}
			path, ok, err := resolve(ctx, documentID)
			if err != nil {
				return err
			}
			if !ok {
				continue
			}
			attrs := make(map[string]any, len(mark.Attrs))
			for key, value := range mark.Attrs {
				attrs[key] = value
			}
			attrs["href"] = path
			delete(attrs, "target")
			mark.Attrs = attrs
			marks = append(marks, mark)
		}
		if len(marks) == 0 {
			marks = nil
		}
		node.Marks = marks
		if node.Attrs == nil {
			node.Attrs = map[string]any{}
		}
		node.Attrs["publishedFrom"] = original
	}
	for index := range node.Content {
		if err := materializePublicationDocumentLinkNode(ctx, &node.Content[index], resolve); err != nil {
			return err
		}
	}
	return nil
}

func nodeHasInternalDocumentLink(node *tiptap.Node) bool {
	for _, mark := range node.Marks {
		if mark.Type != "link" {
			continue
		}
		href, _ := mark.Attrs["href"].(string)
		if _, ok := internalDocumentLinkID(href); ok {
			return true
		}
	}
	return false
}

func internalDocumentLinkID(href string) (string, bool) {
	trimmed := strings.TrimSpace(href)
	if !strings.HasPrefix(trimmed, internalDocumentLinkPrefix) {
		return "", false
	}
	documentID := strings.TrimPrefix(trimmed, internalDocumentLinkPrefix)
	if cut := strings.IndexAny(documentID, "/?#"); cut >= 0 {
		documentID = documentID[:cut]
	}
	return documentID, documentID != ""
}

// publicationArticlePathResolver builds a resolver bound to one workspace and
// locale. Only documents from the same workspace that are live in the Help
// Center resolve to a public path.
func (s *DocsHelpcenterService) publicationArticlePathResolver(workspaceID, locale string, cfg *model.DocsHelpcenterConfig) publicationArticlePathResolver {
	return func(ctx context.Context, documentID string) (string, bool, error) {
		target, err := s.docRepo.GetByID(ctx, documentID)
		if err != nil {
			return "", false, err
		}
		if target == nil || target.WorkspaceID != workspaceID {
			return "", false, nil
		}
		article, err := s.hcRepo.GetArticle(ctx, documentID)
		if err != nil {
			return "", false, err
		}
		if article == nil || article.PublicPublishedAt == nil || strings.TrimSpace(article.PublicID) == "" {
			return "", false, nil
		}
		slug := article.Slug
		publication, err := s.publicationRepo.GetArticlePublication(ctx, documentID, locale)
		if err != nil {
			return "", false, err
		}
		if publication != nil && strings.TrimSpace(publication.Slug) != "" {
			slug = publication.Slug
		}
		return buildDocsHelpcenterArticleCanonicalPath(cfg, locale, slug, article.PublicID), true, nil
	}
}
