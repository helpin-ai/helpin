package service

import (
	"encoding/json"
	"regexp"
	"strings"
	"unicode"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// ---------------------------------------------------------------------------
// Escalation signal constants
// ---------------------------------------------------------------------------

const (
	// Escalation reasons for analytics (AgentHandoff.Reason).
	escalationReasonRepetitionLoop     = "repetition_loop_detected"
	escalationReasonFrustration        = "frustration_detected"
	escalationReasonConsecutiveLowConf = "consecutive_low_confidence"
	escalationReasonDecliningSatisfy   = "declining_satisfaction"
	escalationReasonSameIssueStalled   = "same_issue_stalled_limit_reached"

	// Repetition detection: Jaccard bigram similarity threshold.
	repetitionJaccardThreshold = 0.55
	// Number of prior similar messages required before escalation (3rd ask triggers).
	repetitionCountThreshold = 2
	// Minimum words in a message to use bigram comparison; below this use exact match.
	repetitionMinBigramWords = 3

	// Consecutive low-confidence: how many marginal AI replies trigger escalation.
	consecutiveLowConfCount = 2
	// Confidence within this margin above the threshold is considered "marginal".
	consecutiveLowConfMargin = 0.10

	// Frustration weighted score threshold to trigger escalation.
	frustrationScoreThreshold = 0.6

	// Declining satisfaction: number of recent AI turns to evaluate.
	satisfactionDeclineWindow = 3
	// Minimum confidence drop across the window to trigger escalation.
	satisfactionDeclineMinDrop = 0.20
)

// ---------------------------------------------------------------------------
// EscalationSignal
// ---------------------------------------------------------------------------

// EscalationSignal represents a detected escalation trigger with its reason and severity.
type EscalationSignal struct {
	Reason string  // snake_case reason for AgentHandoff
	Score  float64 // 0.0–1.0 severity
}

// ---------------------------------------------------------------------------
// Pre-LLM orchestrator
// ---------------------------------------------------------------------------

// evaluatePreLLMEscalation checks conversation history for escalation signals
// that do not require the LLM response. Returns the highest-priority signal,
// or nil if no escalation is warranted. Priority: repetition > frustration > consecutive low confidence.
func evaluatePreLLMEscalation(
	currentMessage string,
	history []model.SupportMessage,
	confidenceThreshold float64,
) *EscalationSignal {
	if signal := detectRepetitionLoop(currentMessage, history); signal != nil {
		return signal
	}
	if signal := detectFrustration(currentMessage); signal != nil {
		return signal
	}
	if signal := detectConsecutiveLowConfidence(history, confidenceThreshold); signal != nil {
		return signal
	}
	return nil
}

type supportIssueHistoryStats struct {
	IssueKey            string
	AIReplyCount        int
	StalledAttemptCount int
	ClarifyCount        int
	LowConfidenceCount  int
	LastReplyKind       string
	LastConfidence      float64
	LastProgressState   string
}

// ---------------------------------------------------------------------------
// Repetition / loop detection
// ---------------------------------------------------------------------------

// detectRepetitionLoop checks if the customer is repeating a question that the
// AI has already answered, indicating the AI's answer was unhelpful.
func detectRepetitionLoop(currentMessage string, history []model.SupportMessage) *EscalationSignal {
	current := strings.TrimSpace(currentMessage)
	if current == "" {
		return nil
	}
	currentLower := strings.ToLower(current)

	// Collect customer messages that were followed by an AI reply.
	var answeredCustomerMessages []string
	for i, msg := range history {
		if msg.SenderType != "customer" {
			continue
		}
		// Check if the next message in history is an AI reply.
		if i+1 < len(history) && history[i+1].SenderType == "ai" {
			answeredCustomerMessages = append(answeredCustomerMessages, msg.Content)
		}
	}

	if len(answeredCustomerMessages) == 0 {
		return nil
	}

	currentWords := tokenizeWords(currentLower)
	useExact := len(currentWords) < repetitionMinBigramWords
	var currentBigrams map[string]struct{}
	if !useExact {
		currentBigrams = wordBigrams(currentLower)
	}

	similarCount := 0
	for _, prior := range answeredCustomerMessages {
		priorLower := strings.ToLower(strings.TrimSpace(prior))
		if useExact || len(tokenizeWords(priorLower)) < repetitionMinBigramWords {
			if currentLower == priorLower {
				similarCount++
			}
			continue
		}
		priorBigrams := wordBigrams(priorLower)
		if jaccardSimilarity(currentBigrams, priorBigrams) >= repetitionJaccardThreshold {
			similarCount++
		}
	}

	if similarCount >= repetitionCountThreshold {
		score := float64(similarCount) / float64(len(answeredCustomerMessages))
		if score > 1.0 {
			score = 1.0
		}
		return &EscalationSignal{
			Reason: escalationReasonRepetitionLoop,
			Score:  score,
		}
	}
	return nil
}

func countAgentAITurns(history []model.SupportMessage, agentID string) int {
	count := 0
	for _, msg := range history {
		if isAIReplyFromAgent(msg, agentID) {
			count++
		}
	}
	return count
}

func countMaxFollowupAITurns(history []model.SupportMessage, agentID string) int {
	count := 0
	for _, msg := range history {
		if !isAIReplyFromAgent(msg, agentID) {
			continue
		}
		if inferAIReplyKind(msg) != supportReplyKindAnswer {
			continue
		}
		count++
	}
	return count
}

func collectSupportIssueHistoryStats(history []model.SupportMessage, agentID, issueKey string, confidenceThreshold float64) supportIssueHistoryStats {
	stats := supportIssueHistoryStats{IssueKey: strings.TrimSpace(issueKey)}
	if stats.IssueKey == "" {
		return stats
	}

	for _, msg := range history {
		if !isAIReplyFromAgent(msg, agentID) {
			continue
		}
		meta, ok := parseAIMessageMetadata(msg)
		if !ok || strings.TrimSpace(meta.AIIssueKey) != stats.IssueKey {
			continue
		}
		stats.AIReplyCount++
		stats.LastReplyKind = inferAIReplyKind(msg)
		stats.LastConfidence = meta.AIConfidence
		stats.LastProgressState = strings.TrimSpace(meta.AIProgressState)
		if stats.LastReplyKind == supportReplyKindClarify {
			stats.ClarifyCount++
		}
		if meta.AIConfidence > 0 && meta.AIConfidence < confidenceThreshold {
			stats.LowConfidenceCount++
		}
		switch strings.TrimSpace(meta.AIProgressState) {
		case supportStateStalled:
			stats.StalledAttemptCount++
		}
	}

	return stats
}

func isAIReplyFromAgent(msg model.SupportMessage, agentID string) bool {
	if msg.SenderType != "ai" {
		return false
	}
	if agentID == "" {
		return true
	}
	if msg.SenderAgentID == nil {
		return false
	}
	return strings.TrimSpace(*msg.SenderAgentID) == agentID
}

func parseAIMessageMetadata(msg model.SupportMessage) (AIMessageMetadata, bool) {
	if strings.TrimSpace(msg.Metadata) == "" {
		return AIMessageMetadata{}, false
	}
	var meta AIMessageMetadata
	if err := json.Unmarshal([]byte(msg.Metadata), &meta); err != nil {
		return AIMessageMetadata{}, false
	}
	if !meta.AIAutoReply {
		return AIMessageMetadata{}, false
	}
	return meta, true
}

func detectStuckOnSameIssue(
	currentPlan SupportQueryPlanContract,
	currentMessage model.SupportMessage,
	stats supportIssueHistoryStats,
	confidenceThreshold float64,
	maxStalledAttempts int,
) *EscalationSignal {
	if maxStalledAttempts <= 0 || stats.IssueKey == "" || stats.AIReplyCount == 0 {
		return nil
	}
	if currentPlan.ProgressSignal == supportProgressNewIssue || currentPlan.ProgressSignal == supportProgressSameNewInfo {
		return nil
	}

	stalled := false
	switch {
	case currentPlan.Decision == supportDecisionClarify && stats.ClarifyCount > 0:
		stalled = true
	case currentPlan.ProgressSignal == supportProgressSameRepeat:
		stalled = true
	case currentPlan.ProgressSignal == supportProgressSameUnclear && stats.LastReplyKind == supportReplyKindClarify:
		stalled = true
	case stats.LastReplyKind == supportReplyKindAnswer && stats.LastConfidence > 0 && stats.LastConfidence < confidenceThreshold:
		stalled = true
	case stats.LowConfidenceCount > 0 && currentPlan.ProgressSignal != supportProgressSameNewInfo:
		stalled = true
	case isSameIssueDissatisfaction(currentMessage.Content):
		stalled = true
	}
	if !stalled {
		return nil
	}

	if stats.StalledAttemptCount+1 < maxStalledAttempts {
		return nil
	}
	return &EscalationSignal{
		Reason: escalationReasonSameIssueStalled,
		Score:  float64(stats.StalledAttemptCount + 1),
	}
}

func determineAIProgressState(
	currentPlan SupportQueryPlanContract,
	replyKind string,
	confidence float64,
	confidenceThreshold float64,
	stats supportIssueHistoryStats,
) string {
	switch currentPlan.ProgressSignal {
	case supportProgressNewIssue, supportProgressSameNewInfo:
		return supportStateProgressing
	}

	switch replyKind {
	case supportReplyKindClarify:
		if stats.ClarifyCount > 0 || currentPlan.ProgressSignal == supportProgressSameRepeat {
			return supportStateStalled
		}
		return supportStateProgressing
	case supportReplyKindAnswer:
		if confidence > 0 && confidence < confidenceThreshold {
			return supportStateStalled
		}
		if currentPlan.ProgressSignal == supportProgressSameRepeat {
			return supportStateStalled
		}
		return supportStateProgressing
	default:
		return supportStateProgressing
	}
}

func inferAIReplyKind(msg model.SupportMessage) string {
	var meta AIMessageMetadata
	if strings.TrimSpace(msg.Metadata) != "" {
		if err := json.Unmarshal([]byte(msg.Metadata), &meta); err == nil && strings.TrimSpace(meta.AIReplyKind) != "" {
			return strings.TrimSpace(meta.AIReplyKind)
		}
	}

	content := strings.ToLower(strings.TrimSpace(msg.Content))
	switch {
	case isLikelyGreetingReply(content):
		return supportReplyKindGreeting
	case isLikelyClarificationReply(content):
		return supportReplyKindClarify
	default:
		return supportReplyKindAnswer
	}
}

func isGreetingMessage(content string) bool {
	switch strings.Join(tokenizeWords(content), " ") {
	case "hi", "hi there", "hello", "hello there", "hey", "hey there", "good morning", "good afternoon", "good evening":
		return true
	default:
		return false
	}
}

func isLikelyGreetingReply(content string) bool {
	if !strings.HasSuffix(content, "?") {
		return false
	}
	prefixes := []string{
		"how can i help",
		"how can i assist",
		"what can i help",
		"what can i assist",
		"hello how can i help",
		"hi how can i help",
		"hey how can i help",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(content, prefix) {
			return true
		}
	}
	return false
}

func isLikelyClarificationReply(content string) bool {
	if !strings.HasSuffix(content, "?") {
		return false
	}
	patterns := []string{
		"what product or page are you referring to",
		"are you referring to",
		"do you mean",
		"could you clarify",
		"can you clarify",
		"which product",
		"which page",
		"what do you mean by",
		"what exactly do you mean",
		"what part are you asking about",
	}
	for _, pattern := range patterns {
		if strings.Contains(content, pattern) {
			return true
		}
	}
	return false
}

// wordBigrams returns a set of consecutive word pairs from lowercased text,
// after stripping punctuation. Used for fuzzy message similarity comparison.
func wordBigrams(text string) map[string]struct{} {
	words := tokenizeWords(text)
	bigrams := make(map[string]struct{}, len(words))
	for i := 0; i+1 < len(words); i++ {
		bigrams[words[i]+" "+words[i+1]] = struct{}{}
	}
	return bigrams
}

// tokenizeWords splits text into lowercase alphanumeric words.
func tokenizeWords(text string) []string {
	lower := strings.ToLower(text)
	// Strip non-alphanumeric except spaces.
	cleaned := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' {
			return r
		}
		return ' '
	}, lower)
	return strings.Fields(cleaned)
}

// jaccardSimilarity computes |A ∩ B| / |A ∪ B| for two string sets.
func jaccardSimilarity(a, b map[string]struct{}) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 0
	}
	intersection := 0
	for k := range a {
		if _, ok := b[k]; ok {
			intersection++
		}
	}
	union := len(a) + len(b) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

// ---------------------------------------------------------------------------
// Frustration detection
// ---------------------------------------------------------------------------

// frustrationPattern holds a phrase and its weight toward the frustration score.
type frustrationPattern struct {
	phrase string
	weight float64
}

// frustrationPatterns are checked against the lowercased customer message.
var frustrationPatterns = []frustrationPattern{
	// High weight — strong frustration signals
	{"this is ridiculous", 0.4},
	{"waste of time", 0.4},
	{"not helpful at all", 0.4},
	{"useless", 0.4},
	{"doesn't help", 0.4},
	{"does not help", 0.4},
	{"completely unhelpful", 0.4},
	{"absolutely useless", 0.4},

	// Medium weight — clear dissatisfaction
	{"frustrated", 0.25},
	{"fed up", 0.25},
	{"unacceptable", 0.25},
	{"terrible", 0.25},
	{"worst", 0.25},
	{"still not working", 0.25},
	{"still broken", 0.25},
	{"not working", 0.25},
	{"this is a joke", 0.25},

	// Low weight — mild irritation signals
	{"disappointed", 0.15},
	{"annoying", 0.15},
	{"already told you", 0.15},
	{"i already said", 0.15},
	{"as i mentioned", 0.15},
	{"for the third time", 0.15},
	{"how many times", 0.15},
	{"i keep asking", 0.15},
	{"you already asked me", 0.15},
	{"same thing again", 0.15},
}

// multiExclamation matches 3+ consecutive exclamation marks.
var multiExclamation = regexp.MustCompile(`!{3,}`)

// detectFrustration detects customer frustration signals in the current message.
// Returns a signal with severity score, or nil if frustration is below threshold.
func detectFrustration(content string) *EscalationSignal {
	lower := strings.ToLower(strings.TrimSpace(content))
	if lower == "" {
		return nil
	}

	var score float64

	// Check keyword patterns (take the max weight per match to avoid double-counting substrings).
	for _, fp := range frustrationPatterns {
		if strings.Contains(lower, fp.phrase) {
			score += fp.weight
		}
	}

	// Punctuation signal: multiple exclamation marks.
	if multiExclamation.MatchString(content) {
		score += 0.2
	}

	// Punctuation signal: 3+ consecutive ALL-CAPS words (minimum 2 chars each).
	if hasConsecutiveCapsWords(content, 3) {
		score += 0.2
	}

	if score >= frustrationScoreThreshold {
		if score > 1.0 {
			score = 1.0
		}
		return &EscalationSignal{
			Reason: escalationReasonFrustration,
			Score:  score,
		}
	}
	return nil
}

// hasConsecutiveCapsWords checks if the text contains n or more consecutive
// words that are entirely uppercase and at least 2 characters long.
func hasConsecutiveCapsWords(text string, n int) bool {
	words := strings.Fields(text)
	consecutive := 0
	for _, w := range words {
		// Strip punctuation for the check.
		cleaned := strings.TrimFunc(w, func(r rune) bool {
			return !unicode.IsLetter(r)
		})
		if len(cleaned) >= 2 && cleaned == strings.ToUpper(cleaned) && cleaned != strings.ToLower(cleaned) {
			consecutive++
			if consecutive >= n {
				return true
			}
		} else {
			consecutive = 0
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Consecutive low-confidence detection
// ---------------------------------------------------------------------------

// detectConsecutiveLowConfidence checks if recent AI replies had marginal
// confidence, indicating the AI is struggling with this conversation topic.
func detectConsecutiveLowConfidence(history []model.SupportMessage, confidenceThreshold float64) *EscalationSignal {
	marginalThreshold := confidenceThreshold + consecutiveLowConfMargin
	consecutiveCount := 0

	// Walk history backward to find consecutive marginal AI replies.
	for i := len(history) - 1; i >= 0; i-- {
		msg := history[i]
		if msg.SenderType == "customer" {
			// Customer message between AI replies — keep counting through it
			// since we want consecutive AI replies regardless of interleaved customer msgs.
			continue
		}
		if msg.SenderType != "ai" {
			continue
		}
		// Parse AI metadata to get confidence.
		conf, ok := parseAIConfidence(msg.Metadata)
		if !ok {
			// No parseable metadata — skip (e.g., system message from AI).
			continue
		}
		if conf < marginalThreshold {
			consecutiveCount++
		} else {
			// Streak broken by a high-confidence reply.
			break
		}
	}

	if consecutiveCount >= consecutiveLowConfCount {
		return &EscalationSignal{
			Reason: escalationReasonConsecutiveLowConf,
			Score:  float64(consecutiveCount) / float64(consecutiveCount+1),
		}
	}
	return nil
}

// parseAIConfidence extracts the ai_confidence value from a message's metadata JSON.
func parseAIConfidence(metadata string) (float64, bool) {
	if strings.TrimSpace(metadata) == "" {
		return 0, false
	}
	meta, ok := parseAIMessageMetadata(model.SupportMessage{Metadata: metadata})
	if !ok {
		return 0, false
	}
	return meta.AIConfidence, true
}

// ---------------------------------------------------------------------------
// Post-answer: declining satisfaction
// ---------------------------------------------------------------------------

// evaluatePostAnswerEscalation checks if the conversation shows declining
// confidence after the AI has generated a response.
func evaluatePostAnswerEscalation(
	history []model.SupportMessage,
	currentConfidence float64,
) *EscalationSignal {
	// Collect recent AI confidence values from history.
	var confidences []float64
	for i := len(history) - 1; i >= 0 && len(confidences) < satisfactionDeclineWindow; i-- {
		msg := history[i]
		if msg.SenderType != "ai" {
			continue
		}
		conf, ok := parseAIConfidence(msg.Metadata)
		if !ok {
			continue
		}
		confidences = append(confidences, conf)
	}

	// Reverse so oldest is first.
	for i, j := 0, len(confidences)-1; i < j; i, j = i+1, j-1 {
		confidences[i], confidences[j] = confidences[j], confidences[i]
	}

	// Append current response confidence.
	confidences = append(confidences, currentConfidence)

	// Need at least 3 data points for a meaningful trend.
	if len(confidences) < 3 {
		return nil
	}

	first := confidences[0]
	last := confidences[len(confidences)-1]
	drop := first - last

	if drop >= satisfactionDeclineMinDrop {
		return &EscalationSignal{
			Reason: escalationReasonDecliningSatisfy,
			Score:  drop,
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Hard escalation rules (moved from support_ai.go)
// ---------------------------------------------------------------------------

// checkHardEscalation checks if a message matches hard escalation rules.
func checkHardEscalation(content string) string {
	lower := strings.ToLower(strings.TrimSpace(content))

	// Customer explicitly asks for a human.
	humanPatterns := []string{
		"talk to someone", "real person", "human agent", "talk to a human",
		"speak to someone", "real agent", "live agent", "connect me",
	}
	for _, pattern := range humanPatterns {
		if strings.Contains(lower, pattern) {
			return "customer_requested_human"
		}
	}

	return ""
}

func isSameIssueDissatisfaction(content string) bool {
	lower := strings.ToLower(strings.TrimSpace(content))
	if lower == "" {
		return false
	}
	patterns := []string{
		"still not working",
		"that did not help",
		"that didn't help",
		"same issue",
		"you already said that",
		"you already asked that",
		"this is the same problem",
	}
	for _, pattern := range patterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}
