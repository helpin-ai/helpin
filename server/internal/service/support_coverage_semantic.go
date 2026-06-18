package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const (
	coverageEmbeddingProviderName    = "openai"
	coverageGapEmbeddingVersion      = "coverage-gap-canonical-v1"
	coverageFindingEmbeddingVersion  = "coverage-finding-canonical-v1"
	coverageEmbeddingDimensions      = 1536
	coverageDefaultEmbeddingModel    = "text-embedding-3-small"
	coverageEvidenceSourceKeyPrefix  = "coverage_analysis:"
	coverageEventEvidenceKeyPrefix   = "support_event:"
	coverageSemanticAttachThreshold  = 0.90
	coverageSemanticSuggestThreshold = 0.78
	coverageSameRunClusterThreshold  = 0.88
)

func coverageGapEmbeddingText(item model.SupportCoverageGapListItem) string {
	return coverageJoinEmbeddingParts(
		item.CustomerNeedText,
		item.CanonicalTitle,
		item.TopicTitle,
		item.Title,
	)
}

func coverageFindingEmbeddingText(analysis model.SupportCoverageConversationAnalysis) string {
	return coverageJoinEmbeddingParts(
		analysis.CustomerNeed,
		analysis.CanonicalTitle,
	)
}

func coverageJoinEmbeddingParts(parts ...string) string {
	seen := map[string]bool{}
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		part = coverageLimitRunes(strings.Join(strings.Fields(part), " "), 400)
		if part == "" {
			continue
		}
		key := strings.ToLower(part)
		if seen[key] {
			continue
		}
		seen[key] = true
		values = append(values, part)
	}
	return strings.Join(values, "\n")
}

func coverageEmbeddingTextHash(text string) string {
	normalized := coverageNormalizeEmbeddingText(text)
	if normalized == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}

func coverageNormalizeEmbeddingText(text string) string {
	return strings.ToLower(strings.Join(strings.Fields(text), " "))
}

func coverageCosineSimilarity(a, b []float32) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float64
	for idx := range a {
		av := float64(a[idx])
		bv := float64(b[idx])
		dot += av * bv
		normA += av * av
		normB += bv * bv
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	score := dot / (math.Sqrt(normA) * math.Sqrt(normB))
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}

func coverageDeterministicClusterKey(workspaceID string, text string) string {
	hash := coverageEmbeddingTextHash(workspaceID + "\n" + text)
	if hash == "" {
		return ""
	}
	return "semantic:" + hash
}

func coverageEvidenceSourceKey(analysisID string) string {
	analysisID = strings.TrimSpace(analysisID)
	if analysisID == "" {
		return ""
	}
	return coverageEvidenceSourceKeyPrefix + analysisID
}

func coverageEventEvidenceSourceKey(eventID string) string {
	eventID = strings.TrimSpace(eventID)
	if eventID == "" {
		return ""
	}
	return coverageEventEvidenceKeyPrefix + eventID
}

func coverageEmbeddingModel(configured string) string {
	configured = strings.TrimSpace(configured)
	if configured != "" {
		return configured
	}
	return coverageDefaultEmbeddingModel
}

func coverageLimitRunes(value string, maxRunes int) string {
	if maxRunes <= 0 || value == "" {
		return value
	}
	count := 0
	for idx := range value {
		if count == maxRunes {
			return strings.TrimSpace(value[:idx])
		}
		count++
	}
	return value
}

func coverageVectorLiteral(vector []float32) string {
	if len(vector) == 0 {
		return "[]"
	}
	var b strings.Builder
	b.Grow(len(vector) * 8)
	b.WriteByte('[')
	for idx, value := range vector {
		if idx > 0 {
			b.WriteByte(',')
		}
		b.WriteString(fmt.Sprintf("%g", value))
	}
	b.WriteByte(']')
	return b.String()
}

func coverageParseVectorLiteral(value string) []float32 {
	value = strings.TrimSpace(value)
	if value == "" || value == "[]" {
		return nil
	}
	value = strings.TrimPrefix(strings.TrimSuffix(value, "]"), "[")
	parts := strings.Split(value, ",")
	vector := make([]float32, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		var parsed float32
		if _, err := fmt.Sscanf(part, "%f", &parsed); err != nil {
			return nil
		}
		vector = append(vector, parsed)
	}
	return vector
}

func coverageFirstRuneStart(value string, idx int) int {
	if idx >= len(value) {
		return len(value)
	}
	for idx < len(value) && !utf8.RuneStart(value[idx]) {
		idx++
	}
	return idx
}

func coverageIsBlank(value string) bool {
	for _, r := range value {
		if !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}
