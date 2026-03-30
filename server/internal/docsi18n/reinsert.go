package docsi18n

import (
	"fmt"
)

func (p *ArticleTranslationPlan) Apply(response TranslationResponse) (*ArticleDraft, error) {
	translatedByID, err := validateResponseSegments(p.Request.Segments, response)
	if err != nil {
		return nil, err
	}

	draft := &ArticleDraft{
		Title:          p.source.Title,
		Excerpt:        cloneStringPointer(p.source.Excerpt),
		SEOTitle:       cloneStringPointer(p.source.SEOTitle),
		SEODescription: cloneStringPointer(p.source.SEODescription),
		Content:        cloneNode(p.source.Content),
	}

	for _, ref := range p.metaRefs {
		value, err := restoreProtectedTerms(translatedByID[ref.ID], ref.Protection)
		if err != nil {
			return nil, fmt.Errorf("restore protected terms for %s: %w", ref.ID, err)
		}
		switch ref.Field {
		case metaFieldTitle:
			draft.Title = value
		case metaFieldExcerpt:
			draft.Excerpt = stringPtr(value)
		case metaFieldSEOTitle:
			draft.SEOTitle = stringPtr(value)
		case metaFieldSEODescription:
			draft.SEODescription = stringPtr(value)
		default:
			return nil, fmt.Errorf("unsupported meta field %s", ref.Field)
		}
	}

	for _, ref := range p.nodeRefs {
		node, err := nodeAtPath(&draft.Content, ref.Path)
		if err != nil {
			return nil, err
		}
		translatedText, err := restoreProtectedTerms(translatedByID[ref.ID], ref.Protection)
		if err != nil {
			return nil, fmt.Errorf("restore protected terms for %s: %w", ref.ID, err)
		}
		handler, err := p.registry.Handler(node.Type)
		if err != nil {
			return nil, err
		}
		if err := handler.Reinsert(node, ref, translatedText); err != nil {
			return nil, err
		}
	}

	if err := ValidateDocument(&draft.Content); err != nil {
		return nil, fmt.Errorf("validate translated content schema: %w", err)
	}
	if err := validatePreservation(&p.source.Content, &draft.Content, NodePath{}, p.registry); err != nil {
		return nil, err
	}
	return draft, nil
}

func validateResponseSegments(expected []Segment, response TranslationResponse) (map[string]string, error) {
	translatedByID := make(map[string]string, len(response.Segments))
	for _, segment := range response.Segments {
		if _, exists := translatedByID[segment.ID]; exists {
			return nil, fmt.Errorf("duplicate translated segment id %s", segment.ID)
		}
		translatedByID[segment.ID] = segment.TranslatedText
	}

	for _, expectedSegment := range expected {
		if _, ok := translatedByID[expectedSegment.ID]; !ok {
			return nil, fmt.Errorf("missing translated segment id %s", expectedSegment.ID)
		}
	}
	if len(translatedByID) != len(expected) {
		return nil, fmt.Errorf("translated segment count = %d, want %d", len(translatedByID), len(expected))
	}
	return translatedByID, nil
}
