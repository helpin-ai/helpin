package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

// Links between Help Center articles are resolved when a page is read, not
// only when it is published.
//
// Authors link documents with "helpin://documents/<id>". Publishing still
// rewrites those links in the snapshot (see materializePublicationDocumentLinks)
// so every consumer of the stored JSON sees a safe public href, and it keeps
// the author's text node in attrs.publishedFrom so draft/snapshot comparison
// ignores the rewrite. That publish-time href is only a fallback: every public
// HTML render (resolvePublicDocumentLinks) goes back to the original link in
// publishedFrom, or to an unrewritten helpin:// href such as in translation
// snapshots, and resolves it against the target's current live state. A
// target published, renamed or unpublished after the source article therefore
// shows up correctly without republishing the source; the public cache is
// invalidated per workspace whenever an article's public state changes.

// internalDocumentLinkPrefix is the in-app link form for Helpin documents,
// written by the editor's document mentions and by MCP markdown links.
const internalDocumentLinkPrefix = "helpin://documents/"

// publicationLinkPathLookup returns the public Help Center path for each
// document ID that is a live article. IDs missing from the result are not
// linkable publicly.
type publicationLinkPathLookup func(ctx context.Context, documentIDs []string) (map[string]string, error)

// publicLinkTargetRepository is the data the link resolver reads.
type publicLinkTargetRepository interface {
	GetConfig(ctx context.Context, workspaceID string) (*model.DocsHelpcenterConfig, error)
	ListLiveArticleLinkTargets(ctx context.Context, workspaceID, locale string, documentIDs []string) ([]repository.DocsHelpcenterLinkTarget, error)
}

// materializePublicationDocumentLinks rewrites in-app document links in a
// public snapshot. Links to live articles become public article paths; links
// to anything else lose the link and keep their text, so readers never reach
// an in-app URL they cannot open. The source node is kept in publishedFrom.
func materializePublicationDocumentLinks(ctx context.Context, raw json.RawMessage, lookup publicationLinkPathLookup) (json.RawMessage, error) {
	if len(raw) == 0 || !bytes.Contains(raw, []byte(internalDocumentLinkPrefix)) {
		return raw, nil
	}
	var root tiptap.Node
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, fmt.Errorf("decode published document links: %w", err)
	}
	paths, err := lookup(ctx, collectDocumentLinkIDs(&root, false))
	if err != nil {
		return nil, err
	}
	rewriteDocumentLinkNode(&root, paths, false)
	encoded, err := json.Marshal(root)
	if err != nil {
		return nil, fmt.Errorf("encode published document links: %w", err)
	}
	return json.RawMessage(encoded), nil
}

// resolvePublicDocumentLinks re-resolves document links in stored public
// content against the targets' current live state. It reads the author's
// original link from publishedFrom when present, so links rewritten or
// dropped at publish time are recomputed on every render.
func resolvePublicDocumentLinks(ctx context.Context, raw json.RawMessage, lookup publicationLinkPathLookup) (json.RawMessage, error) {
	if len(raw) == 0 || !bytes.Contains(raw, []byte(internalDocumentLinkPrefix)) {
		return raw, nil
	}
	var root tiptap.Node
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, fmt.Errorf("decode public document links: %w", err)
	}
	paths, err := lookup(ctx, collectDocumentLinkIDs(&root, true))
	if err != nil {
		return nil, err
	}
	rewriteDocumentLinkNode(&root, paths, true)
	encoded, err := json.Marshal(root)
	if err != nil {
		return nil, fmt.Errorf("encode public document links: %w", err)
	}
	return json.RawMessage(encoded), nil
}

// renderPublicArticleHTML resolves document links for the reader's locale and
// renders redacted public HTML. A failed link lookup degrades to plain text
// for internal links instead of failing the page.
func renderPublicArticleHTML(ctx context.Context, repo publicLinkTargetRepository, workspaceID, locale string, raw json.RawMessage) (string, error) {
	return tiptap.RenderHTML(RedactPublicDocsJSON(resolvePublicDocumentLinksForRender(ctx, repo, workspaceID, locale, raw)))
}

func resolvePublicDocumentLinksForRender(ctx context.Context, repo publicLinkTargetRepository, workspaceID, locale string, raw json.RawMessage) json.RawMessage {
	resolved, err := resolvePublicDocumentLinks(ctx, raw, liveArticleLinkPathLookup(repo, workspaceID, locale))
	if err == nil {
		return resolved
	}
	slog.WarnContext(ctx, "hc: resolve article links failed; rendering them as text", "workspace_id", workspaceID, "error", err)
	resolved, err = resolvePublicDocumentLinks(ctx, raw, func(context.Context, []string) (map[string]string, error) {
		return nil, nil
	})
	if err != nil {
		return raw
	}
	return resolved
}

// liveArticleLinkPathLookup resolves document IDs to canonical public paths
// with one config read and one batched publication query. Only documents from
// workspaceID that are live in the Help Center resolve. A target published in
// locale links to that locale; otherwise it links to the default locale.
// An empty locale means the workspace default locale.
func liveArticleLinkPathLookup(repo publicLinkTargetRepository, workspaceID, locale string) publicationLinkPathLookup {
	return func(ctx context.Context, documentIDs []string) (map[string]string, error) {
		if repo == nil || len(documentIDs) == 0 {
			return nil, nil
		}
		cfg, err := repo.GetConfig(ctx, workspaceID)
		if err != nil {
			return nil, err
		}
		requested := strings.TrimSpace(strings.ToLower(locale))
		if requested == "" {
			requested = defaultHelpcenterLocale(cfg)
		}
		targets, err := repo.ListLiveArticleLinkTargets(ctx, workspaceID, requested, documentIDs)
		if err != nil {
			return nil, err
		}
		chosen := make(map[string]repository.DocsHelpcenterLinkTarget, len(targets))
		for _, target := range targets {
			if strings.TrimSpace(target.Slug) == "" || strings.TrimSpace(target.PublicID) == "" {
				continue
			}
			current, seen := chosen[target.DocumentID]
			if !seen || (target.Locale == requested && current.Locale != requested) {
				chosen[target.DocumentID] = target
			}
		}
		paths := make(map[string]string, len(chosen))
		for documentID, target := range chosen {
			paths[documentID] = buildDocsHelpcenterArticleCanonicalPath(cfg, target.Locale, target.Slug, target.PublicID)
		}
		return paths, nil
	}
}

// collectDocumentLinkIDs returns the distinct document IDs linked from the
// tree. With useSource, links preserved in publishedFrom count as well.
func collectDocumentLinkIDs(root *tiptap.Node, useSource bool) []string {
	seen := map[string]struct{}{}
	var ids []string
	var visit func(node *tiptap.Node)
	visit = func(node *tiptap.Node) {
		marks := node.Marks
		if useSource {
			marks = documentLinkSourceMarks(node)
		}
		for _, mark := range marks {
			if mark.Type != "link" {
				continue
			}
			href, _ := mark.Attrs["href"].(string)
			documentID, ok := internalDocumentLinkID(href)
			if !ok {
				continue
			}
			if _, dup := seen[documentID]; !dup {
				seen[documentID] = struct{}{}
				ids = append(ids, documentID)
			}
		}
		for index := range node.Content {
			visit(&node.Content[index])
		}
	}
	visit(root)
	return ids
}

// rewriteDocumentLinkNode applies resolved paths to every document link.
// With useSource, the author's marks from publishedFrom replace whatever the
// publish step wrote; otherwise the source node is stored in publishedFrom.
func rewriteDocumentLinkNode(node *tiptap.Node, paths map[string]string, useSource bool) {
	sourceMarks := node.Marks
	if useSource {
		sourceMarks = documentLinkSourceMarks(node)
	}
	if marksHaveInternalDocumentLink(sourceMarks) {
		if !useSource {
			// Keep the source node so unpublished-change detection compares
			// the draft with what the author wrote, not with the rewritten link.
			original := tiptap.Node{Type: node.Type, Text: node.Text, Marks: append([]tiptap.Mark(nil), node.Marks...)}
			if node.Attrs == nil {
				node.Attrs = map[string]any{}
			}
			node.Attrs["publishedFrom"] = original
		}
		node.Marks = resolveDocumentLinkMarks(sourceMarks, paths)
	}
	for index := range node.Content {
		rewriteDocumentLinkNode(&node.Content[index], paths, useSource)
	}
}

func resolveDocumentLinkMarks(source []tiptap.Mark, paths map[string]string) []tiptap.Mark {
	marks := make([]tiptap.Mark, 0, len(source))
	for _, mark := range source {
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
		path, ok := paths[documentID]
		if !ok {
			continue
		}
		attrs := make(map[string]any, len(mark.Attrs))
		for key, value := range mark.Attrs {
			attrs[key] = value
		}
		attrs["href"] = path
		delete(attrs, "target")
		marks = append(marks, tiptap.Mark{Type: mark.Type, Attrs: attrs})
	}
	if len(marks) == 0 {
		return nil
	}
	return marks
}

// documentLinkSourceMarks returns the author's marks for a node: the marks
// preserved in publishedFrom when they carry a document link, else the
// node's own marks.
func documentLinkSourceMarks(node *tiptap.Node) []tiptap.Mark {
	if node.Attrs == nil {
		return node.Marks
	}
	source, ok := node.Attrs["publishedFrom"]
	if !ok {
		return node.Marks
	}
	var original tiptap.Node
	switch typed := source.(type) {
	case tiptap.Node:
		original = typed
	default:
		payload, err := json.Marshal(source)
		if err != nil || json.Unmarshal(payload, &original) != nil {
			return node.Marks
		}
	}
	if original.Type != node.Type || original.Text != node.Text || !marksHaveInternalDocumentLink(original.Marks) {
		return node.Marks
	}
	return original.Marks
}

func marksHaveInternalDocumentLink(marks []tiptap.Mark) bool {
	for _, mark := range marks {
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
