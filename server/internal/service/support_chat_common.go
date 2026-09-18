package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode"

	"github.com/helpin-ai/helpin/server/internal/aiusage"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
)

const supportGreetingOperation = aiusage.AIUsageOperationSupportGreeting

// classifySupportCommonMessage is the entry point for bounded conversational
// shortcuts. An empty route retains the normal support agent and its context.
// FAQs require verified answers; acknowledgments never imply issue resolution.
func classifySupportCommonMessage(conv *model.SupportConversation, source *model.SupportMessage, history []model.SupportMessage) string {
	if conv == nil || source == nil || conv.AIActiveRunID != nil || conv.AIControlVersion != 0 || conv.AIResumedAt != nil || conv.AITurnCount != 0 || conv.LinkedTaskID != nil || conv.Status != model.SupportConversationStatusOpen || model.SupportAIConversationBlocked(conv) || supportConversationHumanOwned(conv) {
		return ""
	}
	if model.SupportAIReplyChannel(conv, source) != "chat" {
		return ""
	}
	if len(source.Content) > 128 || source.ID == "" || source.WorkspaceID != conv.WorkspaceID || source.ConversationID != conv.ID || source.IsInternal || source.SenderType != "customer" || source.MessageType != "reply" || source.DeletedAt.Valid || len(source.Attachments) != 0 || derefString(conv.LastPublicMessageID) != source.ID {
		return ""
	}
	// Trim punctuation only at the edges: "hi, payment failed" never matches.
	text := strings.TrimFunc(strings.ToLower(strings.TrimSpace(source.Content)), func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsPunct(r)
	})
	text = strings.Join(strings.Fields(text), " ")
	switch text {
	case "hi", "hello", "hey", "hi there", "hello there", "hey there", "good morning", "good afternoon", "good evening", "hallo", "guten tag", "bonjour", "salut", "hola", "olá", "ciao", "مرحبا", "你好", "こんにちは":
	default:
		return ""
	}
	replies := 0
	for _, message := range history {
		if message.MessageType != "reply" || message.DeletedAt.Valid {
			continue
		}
		replies++
		if message.ID != source.ID || message.WorkspaceID != conv.WorkspaceID || message.ConversationID != conv.ID || message.Content != source.Content || message.IsInternal || len(message.Attachments) != 0 {
			return ""
		}
	}
	if replies == 1 {
		return supportGreetingOperation
	}
	return ""
}

func (s *SupportChatService) replyToInitialGreeting(ctx context.Context, conv *model.SupportConversation, source *model.SupportMessage, agent *model.Agent, processing *model.AIMessageProcessing, settings model.SupportInboxSettings) (bool, error) {
	started := time.Now()
	callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	response, err := completeAI(callCtx, s.supportAIService.llmProvider, AICompletionRequest{
		WorkspaceID: conv.WorkspaceID, FeatureKey: BillingFeatureSupportAIReply,
		OperationKey:    supportGreetingOperation,
		IdempotencyKey:  aiUsageIdempotencyKey(conv.WorkspaceID, BillingFeatureSupportAIReply, supportGreetingOperation, source.ID),
		Metadata:        map[string]interface{}{"conversation_id": conv.ID, "source_message_id": source.ID},
		RequireComplete: true,
		ValidateResponse: func(response *llm.ChatResponse) error {
			_, err := supportGreetingContent(response)
			return err
		},
		Chat: llm.ChatRequest{
			SystemPrompt: "You are a customer support assistant replying to the first greeting in a new conversation. Briefly greet the customer in the same language and ask how you can help. Use at most two short sentences. Do not claim to have performed work, mention products or policies, request personal information, or include links. Return JSON with a single content string.",
			Messages:     []llm.Message{{Role: "user", Content: source.Content}},
			MaxTokens:    256, JSONMode: true,
			JSONSchema: map[string]any{"type": "object", "properties": map[string]any{"content": map[string]any{"type": "string"}}, "required": []string{"content"}, "additionalProperties": false},
			Reasoning:  &llm.ReasoningConfig{Effort: "low"},
		},
	})
	if err != nil {
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		slog.WarnContext(ctx, "support greeting falling back to agent", "workspace_id", conv.WorkspaceID, "conversation_id", conv.ID, "latency_ms", time.Since(started).Milliseconds(), "error", err)
		return false, nil
	}
	content, err := supportGreetingContent(response)
	if err != nil {
		return false, err
	}
	metadata, err := json.Marshal(AIMessageMetadata{
		AIAutoReply: shouldCreatePublicSupportAIReply(settings), AIAgentID: agent.ID,
		AIReplyKind: supportReplyKindGreeting, AIModel: response.Model,
		AITokensUsed: response.TokensUsed.InputTokens + response.TokensUsed.OutputTokens,
		AIConfidence: 1, AIPreRoute: supportGreetingOperation,
		AIStageLatencyMS: map[string]int64{supportGreetingOperation: time.Since(started).Milliseconds()},
	})
	if err != nil {
		return false, fmt.Errorf("encode greeting metadata: %w", err)
	}
	reply := &model.SupportMessage{
		WorkspaceID: conv.WorkspaceID, ConversationID: conv.ID,
		SenderType: "ai", SenderAgentID: &agent.ID, SenderDisplayName: strPtr(helpinAIDisplayName),
		Content: content, MessageType: "reply", Metadata: string(metadata),
		IsInternal: !shouldCreatePublicSupportAIReply(settings),
	}
	created, err := s.processingRepo.CreateInitialGreetingReply(ctx, processing.ID, reply, *source)
	if err != nil {
		return false, fmt.Errorf("publish initial greeting: %w", err)
	}
	if !created {
		// A new message, takeover, policy change or settled turn invalidated
		// this reply. New customer messages have their own processing records.
		return true, s.processingRepo.MarkCompleted(ctx, processing.ID, nil, 0)
	}
	publishSupportAIMessageStream(s.supportAIService.wsPublisher, conv.WorkspaceID, reply, "ai:"+agent.ID)
	slog.InfoContext(ctx, "support greeting handled without agent run", "workspace_id", conv.WorkspaceID, "conversation_id", conv.ID, "provider", response.Provider, "model", response.Model, "latency_ms", time.Since(started).Milliseconds())
	return true, nil
}

func supportGreetingContent(response *llm.ChatResponse) (string, error) {
	if response == nil {
		return "", fmt.Errorf("greeting response is missing")
	}
	var reply struct {
		Content string `json:"content"`
	}
	if err := llm.UnmarshalResponse(response.Content, &reply); err != nil {
		return "", fmt.Errorf("invalid greeting response")
	}
	content := strings.TrimSpace(reply.Content)
	if content == "" || len([]rune(content)) > 400 {
		return "", fmt.Errorf("greeting response must contain between 1 and 400 characters")
	}
	return content, nil
}
