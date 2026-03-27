package docsi18n

import (
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

func PrepareArticleTranslationPlan(source ArticleSource, opts PrepareOptions) (*ArticleTranslationPlan, error) {
	if strings.TrimSpace(source.Title) == "" {
		return nil, fmt.Errorf("source title is required")
	}
	if strings.TrimSpace(source.SourceLocale) == "" || strings.TrimSpace(source.TargetLocale) == "" {
		return nil, fmt.Errorf("source and target locales are required")
	}
	if err := ValidateDocument(&source.Content); err != nil {
		return nil, fmt.Errorf("validate source content: %w", err)
	}

	registry := DefaultRegistry()
	protected := newProtectedTermSet(opts.ProtectedTerms)
	plan := &ArticleTranslationPlan{
		Request: TranslationRequest{
			SourceLocale:   source.SourceLocale,
			TargetLocale:   source.TargetLocale,
			ProtectedTerms: protected.Terms(),
		},
		source:   source,
		registry: registry,
	}

	ctx := &ExtractContext{
		registry:  registry,
		protected: protected,
	}

	plan.addMetaSegment(metaFieldTitle, strings.TrimSpace(source.Title), protected)
	if source.Excerpt != nil && strings.TrimSpace(*source.Excerpt) != "" {
		plan.addMetaSegment(metaFieldExcerpt, strings.TrimSpace(*source.Excerpt), protected)
	}
	if source.SEOTitle != nil && strings.TrimSpace(*source.SEOTitle) != "" {
		plan.addMetaSegment(metaFieldSEOTitle, strings.TrimSpace(*source.SEOTitle), protected)
	}
	if source.SEODescription != nil && strings.TrimSpace(*source.SEODescription) != "" {
		plan.addMetaSegment(metaFieldSEODescription, strings.TrimSpace(*source.SEODescription), protected)
	}

	if err := ctx.extractNode(&source.Content, NodePath{}); err != nil {
		return nil, err
	}

	plan.Request.Segments = append(plan.Request.Segments, ctx.segments...)
	plan.nodeRefs = append(plan.nodeRefs, ctx.nodeRefs...)
	return plan, nil
}

func (p *ArticleTranslationPlan) addMetaSegment(field metaField, text string, protected protectedTermSet) {
	protectedText, protection := protected.Protect(text)
	ref := metaSegmentRef{
		ID:         fmt.Sprintf("meta:%s", field),
		Field:      field,
		Protection: protection,
	}
	p.metaRefs = append(p.metaRefs, ref)
	p.Request.Segments = append(p.Request.Segments, Segment{
		ID:          ref.ID,
		FieldName:   string(field),
		Text:        protectedText,
		ContextHint: "article metadata",
	})
}

func (c *ExtractContext) extractNode(node *tiptap.Node, path NodePath) error {
	handler, err := c.registry.Handler(node.Type)
	if err != nil {
		return err
	}
	return handler.Extract(node, path, c)
}

func (c *ExtractContext) visitChildren(node *tiptap.Node, path NodePath) error {
	for i := range node.Content {
		if err := c.extractNode(&node.Content[i], path.Child(i)); err != nil {
			return err
		}
	}
	return nil
}

func (c *ExtractContext) addNodeSegment(path NodePath, nodeType, fieldName, text, contextHint string) {
	protectedText, protection := c.protected.Protect(text)
	id := fmt.Sprintf("doc/%d", c.nextDocIndex)
	c.nextDocIndex++
	c.segments = append(c.segments, Segment{
		ID:          id,
		NodePath:    path.String(),
		NodeType:    nodeType,
		FieldName:   fieldName,
		Text:        protectedText,
		ContextHint: contextHint,
		GroupID:     firstPathGroup(path),
	})
	c.nodeRefs = append(c.nodeRefs, nodeSegmentRef{
		ID:         id,
		Path:       path,
		NodeType:   nodeType,
		FieldName:  fieldName,
		Protection: protection,
	})
}

func firstPathGroup(path NodePath) string {
	if len(path) == 0 {
		return "root"
	}
	return fmt.Sprintf("block/%d", path[0])
}
