package service

import (
	"encoding/json"
	"net/mail"
	"regexp"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const (
	forwardedEmailDefaultMinConfidence = 80
	forwardedEmailMaxScanBytes         = 30 * 1024
)

var (
	forwardedEmailMarkerPatterns = []string{
		"forwarded message",
		"begin forwarded message",
		"original message",
	}
	forwardedEmailFromLineRE = regexp.MustCompile(`(?i)^[\s>]*from:\s*(.+?)\s*$`)
)

type forwardedEmailDetectionInput struct {
	ForwarderEmail  string
	ForwarderName   string
	RecipientEmails []string
	ReplyDomain     string
	RouteDomain     string
	Text            string
	MinConfidence   int
}

type forwardedEmailDetectionResult struct {
	Applied          bool
	OriginalEmail    string
	OriginalName     string
	ForwardedByEmail string
	ForwardedByName  string
	Confidence       int
	ConfidenceLevel  string
	Source           string
	Reason           string
}

type forwardedEmailCandidate struct {
	email string
	name  string
	line  int
	raw   string
}

func detectForwardedEmailAttribution(input forwardedEmailDetectionInput) forwardedEmailDetectionResult {
	minConfidence := input.MinConfidence
	if minConfidence <= 0 {
		minConfidence = forwardedEmailDefaultMinConfidence
	}
	text := capForwardedEmailScanText(input.Text)
	if strings.TrimSpace(text) == "" {
		return forwardedEmailDetectionResult{Reason: "empty_body"}
	}

	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	markerIndex, marker := firstForwardedEmailMarker(lines)
	if markerIndex < 0 {
		return forwardedEmailDetectionResult{Reason: "missing_forwarded_marker"}
	}

	candidates := forwardedEmailCandidatesAfterMarker(lines, markerIndex)
	if len(candidates) == 0 {
		return forwardedEmailDetectionResult{Reason: "missing_original_from"}
	}
	if hasConflictingForwardedEmailCandidates(candidates) {
		return forwardedEmailDetectionResult{Reason: "conflicting_original_from", Confidence: 40}
	}

	candidate := candidates[0]
	result := forwardedEmailDetectionResult{
		OriginalEmail:    candidate.email,
		OriginalName:     candidate.name,
		ForwardedByEmail: strings.ToLower(strings.TrimSpace(input.ForwarderEmail)),
		ForwardedByName:  strings.TrimSpace(input.ForwarderName),
		Source:           marker,
		Confidence:       40,
	}
	if candidate.line-markerIndex <= 4 {
		result.Confidence += 30
	} else {
		result.Confidence += 15
	}
	if forwardedHeaderHasContext(lines, markerIndex, candidate.line) {
		result.Confidence += 15
	}
	if strings.TrimSpace(candidate.name) != "" {
		result.Confidence += 10
	}
	if forwardedOriginalDomainLooksPlausible(candidate.email, input) {
		result.Confidence += 10
	}
	if forwardedOriginalSenderBlocked(candidate.email, input) {
		result.Confidence -= 50
		result.Reason = "blocked_original_sender"
	}
	if result.Confidence < 0 {
		result.Confidence = 0
	}
	result.ConfidenceLevel = forwardedConfidenceLevel(result.Confidence)
	if result.Reason == "" && result.Confidence >= minConfidence {
		result.Applied = true
	} else if result.Reason == "" {
		result.Reason = "below_confidence_threshold"
	}
	return result
}

func capForwardedEmailScanText(text string) string {
	if len(text) <= forwardedEmailMaxScanBytes {
		return text
	}
	return text[:forwardedEmailMaxScanBytes]
}

func firstForwardedEmailMarker(lines []string) (int, string) {
	for i, line := range lines {
		normalized := strings.ToLower(strings.TrimSpace(strings.TrimLeft(line, "> ")))
		for _, marker := range forwardedEmailMarkerPatterns {
			if strings.Contains(normalized, marker) {
				return i, marker
			}
		}
	}
	return -1, ""
}

func forwardedEmailCandidatesAfterMarker(lines []string, markerIndex int) []forwardedEmailCandidate {
	end := markerIndex + 16
	if end > len(lines) {
		end = len(lines)
	}
	var candidates []forwardedEmailCandidate
	for i := markerIndex + 1; i < end; i++ {
		match := forwardedEmailFromLineRE.FindStringSubmatch(lines[i])
		if len(match) != 2 {
			continue
		}
		addr, err := mail.ParseAddress(strings.TrimSpace(match[1]))
		if err != nil || strings.TrimSpace(addr.Address) == "" {
			continue
		}
		candidates = append(candidates, forwardedEmailCandidate{
			email: strings.ToLower(strings.TrimSpace(addr.Address)),
			name:  strings.TrimSpace(addr.Name),
			line:  i,
			raw:   strings.TrimSpace(match[1]),
		})
	}
	return candidates
}

func hasConflictingForwardedEmailCandidates(candidates []forwardedEmailCandidate) bool {
	if len(candidates) <= 1 {
		return false
	}
	first := candidates[0].email
	for _, candidate := range candidates[1:] {
		if !strings.EqualFold(first, candidate.email) {
			return true
		}
	}
	return false
}

func forwardedHeaderHasContext(lines []string, markerIndex, fromIndex int) bool {
	end := fromIndex + 8
	if end > len(lines) {
		end = len(lines)
	}
	for i := markerIndex + 1; i < end; i++ {
		normalized := strings.ToLower(strings.TrimSpace(strings.TrimLeft(lines[i], "> ")))
		if strings.HasPrefix(normalized, "date:") || strings.HasPrefix(normalized, "subject:") || strings.HasPrefix(normalized, "to:") {
			return true
		}
	}
	return false
}

func forwardedOriginalDomainLooksPlausible(email string, input forwardedEmailDetectionInput) bool {
	domain := emailDomain(email)
	if domain == "" {
		return false
	}
	if strings.EqualFold(domain, emailDomain(input.ForwarderEmail)) {
		return false
	}
	return !forwardedDomainMatches(domain, input.ReplyDomain) && !forwardedDomainMatches(domain, input.RouteDomain)
}

func forwardedOriginalSenderBlocked(email string, input forwardedEmailDetectionInput) bool {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return true
	}
	if strings.EqualFold(email, strings.TrimSpace(input.ForwarderEmail)) {
		return true
	}
	for _, recipient := range input.RecipientEmails {
		if strings.EqualFold(email, strings.TrimSpace(recipient)) {
			return true
		}
	}
	domain := emailDomain(email)
	return forwardedDomainMatches(domain, input.ReplyDomain) || forwardedDomainMatches(domain, input.RouteDomain)
}

func forwardedDomainMatches(domain, configured string) bool {
	domain = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(domain)), "@")
	configured = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(configured)), "@")
	return domain != "" && configured != "" && (domain == configured || strings.HasSuffix(domain, "."+configured))
}

func emailDomain(email string) string {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(email)), "@")
	if len(parts) != 2 {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func forwardedConfidenceLevel(score int) string {
	if score >= forwardedEmailDefaultMinConfidence {
		return "high"
	}
	if score >= 50 {
		return "medium"
	}
	return "low"
}

func forwardedAttributionMetadata(result forwardedEmailDetectionResult) map[string]any {
	if !result.Applied {
		return nil
	}
	return map[string]any{
		"forwarded_by_email":                  result.ForwardedByEmail,
		"forwarded_by_name":                   result.ForwardedByName,
		"original_sender_email":               result.OriginalEmail,
		"original_sender_name":                result.OriginalName,
		"sender_attribution_source":           "forwarded_body",
		"sender_attribution_confidence":       result.Confidence,
		"sender_attribution_confidence_level": result.ConfidenceLevel,
		"sender_attribution_marker":           result.Source,
	}
}

func mergeForwardedAttributionMetadata(base string, result forwardedEmailDetectionResult) string {
	metadata := map[string]any{}
	if strings.TrimSpace(base) != "" {
		_ = json.Unmarshal([]byte(base), &metadata)
	}
	for key, value := range forwardedAttributionMetadata(result) {
		if str, ok := value.(string); ok && strings.TrimSpace(str) == "" {
			continue
		}
		metadata[key] = value
	}
	if len(metadata) == 0 {
		return "{}"
	}
	data, err := json.Marshal(metadata)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func forwardedAttributionFromMetadata(metadata map[string]any) *model.SupportForwardedAttribution {
	if len(metadata) == 0 {
		return nil
	}
	originalEmail, _ := metadata["original_sender_email"].(string)
	forwardedByEmail, _ := metadata["forwarded_by_email"].(string)
	if strings.TrimSpace(originalEmail) == "" || strings.TrimSpace(forwardedByEmail) == "" {
		return nil
	}
	originalName, _ := metadata["original_sender_name"].(string)
	forwardedByName, _ := metadata["forwarded_by_name"].(string)
	source, _ := metadata["sender_attribution_source"].(string)
	level, _ := metadata["sender_attribution_confidence_level"].(string)
	confidence := intFromMetadata(metadata["sender_attribution_confidence"])
	return &model.SupportForwardedAttribution{
		OriginalSenderEmail: strings.TrimSpace(originalEmail),
		OriginalSenderName:  strings.TrimSpace(originalName),
		ForwardedByEmail:    strings.TrimSpace(forwardedByEmail),
		ForwardedByName:     strings.TrimSpace(forwardedByName),
		Confidence:          confidence,
		ConfidenceLevel:     strings.TrimSpace(level),
		Source:              strings.TrimSpace(source),
	}
}

func intFromMetadata(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}
