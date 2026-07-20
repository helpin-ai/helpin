package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// publishAIConversationalReply publishes a non-answer AI message. Social turns
// keep the conversation in AI handling without consuming an answer turn;
// confirmations retain the resolved state applied by the caller.
func (s *SupportAIService) publishAIConversationalReply(
	ctx context.Context,
	workspaceID string,
	conversationID string,
	agentID string,
	content string,
	modelName string,
	tokensUsed int,
	confidence float64,
	route string,
	replyKind string,
	customerEmail *string,
	customerPhone *string,
) (*model.SupportMessage, error) {
	metadata := AIMessageMetadata{
		AIAutoReply:  true,
		AIConfidence: confidence,
		AIModel:      modelName,
		AITokensUsed: tokensUsed,
		AIAgentID:    agentID,
		AIReplyKind:  replyKind,
		AIPreRoute:   route,
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal conversational AI metadata: %w", err)
	}

	aiMsg := &model.SupportMessage{
		WorkspaceID:       workspaceID,
		ConversationID:    conversationID,
		SenderType:        "ai",
		SenderAgentID:     &agentID,
		SenderDisplayName: strPtr(helpinAIDisplayName),
		Content:           stripConversationPII(strings.TrimSpace(content), customerEmail, customerPhone),
		MessageType:       "reply",
		Metadata:          string(metadataJSON),
	}
	if s.linkPreviewService != nil {
		s.linkPreviewService.EnrichMessage(ctx, aiMsg)
	}
	if err := s.messageRepo.Create(ctx, aiMsg); err != nil {
		return nil, fmt.Errorf("create conversational AI message: %w", err)
	}
	if replyKind != supportReplyKindConfirm && s.conversationRepo != nil {
		pending := "pending"
		_ = s.conversationRepo.UpdateFields(ctx, workspaceID, conversationID, map[string]any{
			"ai_state":          &pending,
			"assigned_agent_id": &agentID,
			"flow_state":        model.SupportConversationFlowStateAIHandling,
		})
	}
	s.wsPublisher.Publish(websocket.SupportMessageEvent(workspaceID, aiMsg, "ai:"+agentID))
	return aiMsg, nil
}

// hasImmediateAIAnswerToConfirm reports whether the most recent public
// conversational turn was an answer from the configured AI agent. Historical
// answers and clarifying questions cannot resolve a newer, unanswered request.
func hasImmediateAIAnswerToConfirm(history []model.SupportMessage, agentID string) bool {
	for i := len(history) - 1; i >= 0; i-- {
		msg := history[i]
		if msg.MessageType == "system" || msg.IsInternal || strings.TrimSpace(msg.Content) == "" {
			continue
		}
		return isAIReplyFromAgent(msg, agentID) && inferAIReplyKind(msg) == supportReplyKindAnswer
	}
	return false
}

// countSubstantiveAITurns is retained for conversation analytics and tests. It
// does not route messages; the LLM pre-router owns language classification.
func countSubstantiveAITurns(history []model.SupportMessage, agentID string) int {
	count := 0
	for _, msg := range history {
		if !isAIReplyFromAgent(msg, agentID) {
			continue
		}
		if inferAIReplyKind(msg) == supportReplyKindGreeting {
			continue
		}
		count++
	}
	return count
}

// ---------------------------------------------------------------------------
// Hardened confirmation detection
// ---------------------------------------------------------------------------

// confirmationNegationTokens block confirmation detection when present as
// whole words: "not helpful at all" or "thanks but it still doesn't work"
// must never be treated as a resolution confirmation. The bare token "t"
// catches all tokenized negative contractions ("doesn t", "can t", "won t").
var confirmationNegationTokens = map[string]struct{}{
	"not": {}, "no": {}, "never": {}, "still": {}, "but": {}, "however": {},
	"unhelpful": {}, "nothing": {}, "unfortunately": {}, "cannot": {},
	"dont": {}, "doesnt": {}, "didnt": {}, "isnt": {}, "wasnt": {},
	"wont": {}, "cant": {}, "t": {}, "issue": {}, "problem": {}, "error": {},
	"broken": {}, "wrong": {}, "fail": {}, "failed": {}, "failing": {},
	"missing": {}, "unable": {},
}

// confirmationAllowedTokens makes confirmation recognition a whole-utterance
// grammar. A positive phrase surrounded by a new request must not resolve the
// conversation merely because it is short or lacks a question mark.
var confirmationAllowedTokens = map[string]struct{}{
	"a": {}, "all": {}, "awesome": {}, "cheers": {}, "cool": {}, "excellent": {},
	"for": {}, "good": {}, "got": {}, "great": {}, "help": {}, "helped": {},
	"helpful": {}, "i": {}, "info": {}, "information": {}, "is": {}, "it": {},
	"lot": {}, "makes": {}, "me": {}, "much": {}, "needed": {}, "now": {},
	"oh": {}, "ok": {}, "okay": {}, "perfect": {}, "resolved": {}, "s": {},
	"sense": {}, "so": {}, "solved": {}, "thanks": {}, "thank": {}, "that": {},
	"the": {}, "this": {}, "very": {}, "what": {}, "worked": {}, "works": {},
	"yep": {}, "yes": {}, "you": {}, "your": {},
}

// confirmationPhrases are matched as whole tokenized phrases.
var confirmationPhrases = []string{
	"thanks", "thank you", "that helped", "got it", "perfect",
	"that works", "that worked", "awesome", "great", "resolved", "solved",
	"that's what i needed", "all good", "helpful", "works now", "makes sense",
}

const confirmationMaxWords = 12

// isConfirmationMessage checks if a customer message is a resolution
// confirmation: whole-word phrase matching plus negation, question, and
// length guards. Its predecessor substring-matched ("great", "helpful"),
// which classified "not helpful at all" as a confirmed resolution.
func isConfirmationMessage(content string) bool {
	lower := strings.ToLower(strings.TrimSpace(content))
	if lower == "" {
		return false
	}
	if strings.Contains(lower, "?") {
		return false // questions are never confirmations
	}
	words := tokenizeWords(lower)
	if len(words) == 0 || len(words) > confirmationMaxWords {
		return false // long messages carry new content
	}
	for _, w := range words {
		if _, blocked := confirmationNegationTokens[w]; blocked {
			return false
		}
		if _, allowed := confirmationAllowedTokens[w]; !allowed {
			return false
		}
	}
	padded := " " + strings.Join(words, " ") + " "
	for _, phrase := range confirmationPhrases {
		normalized := strings.Join(tokenizeWords(phrase), " ")
		if strings.Contains(padded, " "+normalized+" ") {
			return true
		}
	}
	return false
}
