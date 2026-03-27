package docsi18n

import (
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

type ArticleSource struct {
	SourceLocale   string
	TargetLocale   string
	Title          string
	Excerpt        *string
	SEOTitle       *string
	SEODescription *string
	Content        tiptap.Node
}

type ArticleDraft struct {
	Title          string
	Excerpt        *string
	SEOTitle       *string
	SEODescription *string
	Content        tiptap.Node
}

type PrepareOptions struct {
	ProtectedTerms []string
}

type Segment struct {
	ID          string `json:"id"`
	NodePath    string `json:"node_path,omitempty"`
	NodeType    string `json:"node_type,omitempty"`
	FieldName   string `json:"field_name"`
	Text        string `json:"text"`
	ContextHint string `json:"context_hint,omitempty"`
	GroupID     string `json:"group_id,omitempty"`
}

type TranslationRequest struct {
	SourceLocale   string    `json:"source_locale"`
	TargetLocale   string    `json:"target_locale"`
	ProtectedTerms []string  `json:"protected_terms,omitempty"`
	Segments       []Segment `json:"segments"`
}

type TranslatedSegment struct {
	ID             string `json:"id"`
	TranslatedText string `json:"translated_text"`
}

type TranslationResponse struct {
	Segments []TranslatedSegment `json:"segments"`
}

type NodePath []int

func (p NodePath) Child(index int) NodePath {
	next := make(NodePath, len(p)+1)
	copy(next, p)
	next[len(p)] = index
	return next
}

func (p NodePath) String() string {
	if len(p) == 0 {
		return ""
	}
	parts := make([]string, len(p))
	for i, v := range p {
		parts[i] = fmt.Sprintf("%d", v)
	}
	return strings.Join(parts, ".")
}

type ArticleTranslationPlan struct {
	Request  TranslationRequest
	source   ArticleSource
	registry *Registry
	metaRefs []metaSegmentRef
	nodeRefs []nodeSegmentRef
}

type segmentProtection struct {
	placeholderToTerm map[string]string
	placeholderCounts map[string]int
}

type metaField string

const (
	metaFieldTitle          metaField = "title"
	metaFieldExcerpt        metaField = "excerpt"
	metaFieldSEOTitle       metaField = "seo_title"
	metaFieldSEODescription metaField = "seo_description"
)

type metaSegmentRef struct {
	ID         string
	Field      metaField
	Protection segmentProtection
}

type nodeSegmentRef struct {
	ID         string
	Path       NodePath
	NodeType   string
	FieldName  string
	Protection segmentProtection
}

type ExtractContext struct {
	registry     *Registry
	segments     []Segment
	nodeRefs     []nodeSegmentRef
	nextDocIndex int
	protected    protectedTermSet
}

type ReinsertContext struct {
	translatedByID map[string]string
}
