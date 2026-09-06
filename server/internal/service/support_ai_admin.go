package service

// Admin/teammate features: reply preview, draft rewrite, task drafts, and
// the retained generateResponse helper they share.

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// PreviewSupportReply runs the support AI planner + retrieval + answer pipeline without side effects.
func (s *SupportAIService) PreviewSupportReply(
	ctx context.Context,
	workspaceID, agentID string,
	req model.SupportAIPreviewRequest,
) (*model.SupportAIPreviewResponse, error) {
	if s == nil {
		return nil, fmt.Errorf("support AI service not initialized")
	}
	if strings.TrimSpace(workspaceID) == "" {
		return nil, fmt.Errorf("%w: workspace_id is required", ErrSupportPreviewInvalidInput)
	}
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return nil, fmt.Errorf("%w: agent_id is required", ErrSupportPreviewInvalidInput)
	}
	customerMessage := strings.TrimSpace(req.Message)
	if customerMessage == "" {
		return nil, fmt.Errorf("%w: message is required", ErrSupportPreviewInvalidInput)
	}
	if s.agentRepo == nil {
		return nil, fmt.Errorf("agent repository is not configured")
	}
	agent, err := s.agentRepo.GetByID(ctx, workspaceID, agentID)
	if err != nil {
		return nil, fmt.Errorf("get agent: %w", err)
	}
	if agent == nil {
		return nil, fmt.Errorf("%w: %s", ErrSupportPreviewAgentNotFound, agentID)
	}

	history, conversationSource, err := s.resolvePreviewHistory(ctx, workspaceID, req)
	if err != nil {
		return nil, err
	}

	settings := model.DefaultSupportInboxSettings()
	if s.installationRepo != nil {
		if loaded, err := s.loadSettings(ctx, workspaceID); err == nil && loaded != nil {
			settings = *loaded
		}
	}

	includeAnswer := true
	if req.IncludeAnswer != nil {
		includeAnswer = *req.IncludeAnswer
	}

	return s.previewSupportReply(
		ctx,
		workspaceID,
		agent,
		history,
		customerMessage,
		includeAnswer,
		normalizePreviewMaxResults(req.MaxResults),
		settings.AIConfidenceThreshold,
		conversationSource,
		settings.WelcomeMessage,
	)
}

// RewriteSupportDraft rewrites a human-authored support draft for a specific conversation.
func (s *SupportAIService) RewriteSupportDraft(
	ctx context.Context,
	workspaceID, conversationID string,
	req model.SupportAIRewriteDraftRequest,
) (*model.SupportAIRewriteDraftResponse, error) {
	if s == nil {
		return nil, fmt.Errorf("support AI service not initialized")
	}
	if s.llmProvider == nil {
		return nil, fmt.Errorf("support chat LLM provider is not configured")
	}
	if strings.TrimSpace(workspaceID) == "" {
		return nil, fmt.Errorf("%w: workspace_id is required", ErrSupportRewriteInvalidInput)
	}
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return nil, fmt.Errorf("%w: conversation_id is required", ErrSupportRewriteInvalidInput)
	}

	content := strings.TrimSpace(req.Content)
	if content == "" {
		return nil, fmt.Errorf("%w: content is required", ErrSupportRewriteInvalidInput)
	}

	operation := normalizeSupportRewriteOperation(req.Operation)
	if operation == "" {
		return nil, fmt.Errorf("%w: unsupported operation %q", ErrSupportRewriteInvalidInput, strings.TrimSpace(req.Operation))
	}

	history, err := s.loadRewriteHistory(ctx, workspaceID, conversationID)
	if err != nil {
		return nil, err
	}

	return s.rewriteSupportDraftWithHistory(ctx, workspaceID, history, req)
}

// RewriteSupportDraftWithoutConversation rewrites a support draft before a conversation exists.
func (s *SupportAIService) RewriteSupportDraftWithoutConversation(
	ctx context.Context,
	workspaceID string,
	req model.SupportAIRewriteDraftRequest,
) (*model.SupportAIRewriteDraftResponse, error) {
	return s.rewriteDraftWithHistory(ctx, workspaceID, nil, "support reply", BillingFeatureSupportReplyRewrite, req)
}

// RewriteDraftForSurface applies the shared conversation-composer rewrite contract
// without requiring Support conversation context.
func (s *SupportAIService) RewriteDraftForSurface(
	ctx context.Context,
	workspaceID, surface, featureKey string,
	req model.SupportAIRewriteDraftRequest,
) (*model.SupportAIRewriteDraftResponse, error) {
	return s.rewriteDraftWithHistory(ctx, workspaceID, nil, surface, featureKey, req)
}

func (s *SupportAIService) rewriteSupportDraftWithHistory(
	ctx context.Context,
	workspaceID string,
	history []model.SupportMessage,
	req model.SupportAIRewriteDraftRequest,
) (*model.SupportAIRewriteDraftResponse, error) {
	return s.rewriteDraftWithHistory(ctx, workspaceID, history, "support reply", BillingFeatureSupportReplyRewrite, req)
}

func (s *SupportAIService) rewriteDraftWithHistory(
	ctx context.Context,
	workspaceID string,
	history []model.SupportMessage,
	surface, featureKey string,
	req model.SupportAIRewriteDraftRequest,
) (*model.SupportAIRewriteDraftResponse, error) {
	if s == nil {
		return nil, fmt.Errorf("support AI service not initialized")
	}
	if s.llmProvider == nil {
		return nil, fmt.Errorf("support chat LLM provider is not configured")
	}
	if strings.TrimSpace(workspaceID) == "" {
		return nil, fmt.Errorf("%w: workspace_id is required", ErrSupportRewriteInvalidInput)
	}

	content := strings.TrimSpace(req.Content)
	if content == "" {
		return nil, fmt.Errorf("%w: content is required", ErrSupportRewriteInvalidInput)
	}

	operation := normalizeSupportRewriteOperation(req.Operation)
	if operation == "" {
		return nil, fmt.Errorf("%w: unsupported operation %q", ErrSupportRewriteInvalidInput, strings.TrimSpace(req.Operation))
	}

	// Composer actions must not inherit the provider's five-minute timeout.
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	resp, err := completeAI(ctx, s.llmProvider, AICompletionRequest{
		WorkspaceID:    workspaceID,
		FeatureKey:     featureKey,
		IdempotencyKey: aiUsageIdempotencyKey(workspaceID, featureKey, operation, aiUsageStableHash(content)),
		Metadata: map[string]interface{}{
			"operation": operation,
		},
		RequireComplete:    true,
		RetryInvalidOutput: true,
		ValidateResponse: func(response *llm.ChatResponse) error {
			if _, ok := parseSupportRewriteResponse(response.Content); !ok {
				return fmt.Errorf("rewrite response must contain non-empty content")
			}
			return nil
		},
		Chat: llm.ChatRequest{
			SystemPrompt: buildDraftRewriteSystemPrompt(surface, operation),
			Messages:     buildSupportRewriteMessages(history, content),
			Temperature:  0.2,
			MaxTokens:    900,
			JSONMode:     true,
			Reasoning:    &llm.ReasoningConfig{Effort: "low"},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("rewrite support draft: %w", err)
	}

	rewritten, ok := parseSupportRewriteResponse(resp.Content)
	if !ok || strings.TrimSpace(rewritten) == "" {
		return nil, fmt.Errorf("rewrite support draft: empty response")
	}

	return &model.SupportAIRewriteDraftResponse{
		Content:   rewritten,
		Operation: operation,
		Provider:  resp.Provider,
		Model:     resp.Model,
	}, nil
}

// GenerateTaskDraftFromConversation turns a support conversation into a structured PM task draft.
func (s *SupportAIService) GenerateTaskDraftFromConversation(
	ctx context.Context,
	workspaceID string,
	conversation *model.SupportConversation,
	history []model.SupportMessage,
) (*supportConversationTaskDraft, error) {
	if s == nil {
		return nil, fmt.Errorf("support AI service not initialized")
	}
	if conversation == nil {
		return nil, fmt.Errorf("conversation is required")
	}
	if s.llmProvider == nil {
		return nil, fmt.Errorf("support chat LLM provider is not configured")
	}

	sanitized := sanitizeConversationHistory(history, "")
	var agent *model.Agent
	if conversation.AssignedAgentID != nil && strings.TrimSpace(*conversation.AssignedAgentID) != "" && s.agentRepo != nil {
		loaded, err := s.agentRepo.GetByID(ctx, workspaceID, strings.TrimSpace(*conversation.AssignedAgentID))
		if err == nil {
			agent = loaded
		}
	}
	providerName, modelName := resolveSupportLLMConfig(agent)

	messages := buildConversationMessages(sanitized)
	messages = append(messages, llm.Message{
		Role: "user",
		Content: fmt.Sprintf(
			"Create one internal PM task draft for this support conversation.\n\nConversation ID: %s\nConversation Number: %d\nSubject: %s\nCustomer Name: %s\nCustomer Email: %s\nCurrent Status: %s\nCurrent Priority: %s\n\nReturn every required task-draft field with a concrete value.",
			conversation.ID,
			conversation.DisplayID,
			strings.TrimSpace(conversation.Subject),
			strings.TrimSpace(derefString(conversation.CustomerName)),
			strings.TrimSpace(derefString(conversation.CustomerEmail)),
			strings.TrimSpace(conversation.Status),
			strings.TrimSpace(conversation.Priority),
		),
	})

	messagesJSON, err := json.Marshal(messages)
	if err != nil {
		return nil, fmt.Errorf("marshal support task draft usage payload: %w", err)
	}

	resp, err := completeAI(ctx, s.llmProvider, AICompletionRequest{
		WorkspaceID:    workspaceID,
		FeatureKey:     BillingFeatureSupportTaskDraft,
		IdempotencyKey: aiUsagePayloadIdempotencyKey(messagesJSON, workspaceID, BillingFeatureSupportTaskDraft, conversation.ID),
		Metadata: map[string]interface{}{
			"conversation_id": conversation.ID,
		},
		PreferredRoute: &AICompletionRoute{
			Provider: providerName, Model: modelName, ServiceTier: defaultAICompletionServiceTier,
		},
		Chat: llm.ChatRequest{
			SystemPrompt:     supportTaskDraftSystemPrompt,
			Messages:         messages,
			Temperature:      0.2,
			MaxTokens:        1200,
			JSONMode:         true,
			JSONSchema:       supportTaskDraftJSONSchema(),
			JSONSchemaStrict: true,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("generate support task draft: %w", err)
	}

	var parsed struct {
		Title               string `json:"title"`
		Summary             string `json:"summary"`
		DescriptionMarkdown string `json:"description_markdown"`
		TaskType            string `json:"task_type"`
		Priority            string `json:"priority"`
	}
	if err := llm.UnmarshalResponse(resp.Content, &parsed); err != nil {
		return nil, fmt.Errorf("parse support task draft: %w", err)
	}

	title := strings.TrimSpace(parsed.Title)
	summary := strings.TrimSpace(parsed.Summary)
	description := strings.TrimSpace(parsed.DescriptionMarkdown)
	slog.InfoContext(ctx, "support task draft generated",
		"workspace_id", workspaceID,
		"conversation_id", conversation.ID,
		"backend", "structured_chat_completion",
		"provider", providerName,
		"model", modelName,
		"title_len", len(title),
		"summary_len", len(summary),
		"description_len", len(description),
		"input_tokens", resp.TokensUsed.InputTokens,
		"output_tokens", resp.TokensUsed.OutputTokens,
	)

	// When the parser finds JSON but our expected fields come back empty,
	// the model likely responded with a different schema (e.g. nested under
	// a "task" key or with different field names). Log a bounded preview of
	// the raw response so we can see what Claude actually produced.
	if title == "" && description == "" {
		preview := resp.Content
		const maxPreviewLen = 600
		if len(preview) > maxPreviewLen {
			preview = preview[:maxPreviewLen] + "...[truncated]"
		}
		slog.WarnContext(ctx, "support task draft llm response had empty title and description",
			"workspace_id", workspaceID,
			"conversation_id", conversation.ID,
			"provider", providerName,
			"model", modelName,
			"output_tokens", resp.TokensUsed.OutputTokens,
			"response_preview", preview,
		)
	}

	return &supportConversationTaskDraft{
		Title:       title,
		Summary:     summary,
		Description: description,
		TaskType:    strings.TrimSpace(parsed.TaskType),
		Priority:    strings.TrimSpace(parsed.Priority),
	}, nil
}

func (s *SupportAIService) resolvePreviewHistory(
	ctx context.Context,
	workspaceID string,
	req model.SupportAIPreviewRequest,
) ([]model.SupportMessage, string, error) {
	if len(req.History) > 0 {
		return sanitizeConversationHistory(previewHistoryToMessages(req.History), ""), "history", nil
	}

	if req.ConversationID != nil && strings.TrimSpace(*req.ConversationID) != "" {
		conversationID := strings.TrimSpace(*req.ConversationID)
		if s.conversationRepo != nil {
			conv, err := s.conversationRepo.GetByID(ctx, workspaceID, conversationID, "", model.RoleOwner)
			if err != nil {
				return nil, "", fmt.Errorf("get conversation: %w", err)
			}
			if conv == nil {
				return nil, "", fmt.Errorf("%w: %s", ErrSupportPreviewConversationNotFound, conversationID)
			}
		}
		if s.messageRepo == nil {
			return nil, "", fmt.Errorf("support message repository is not configured")
		}
		history, err := s.messageRepo.ListByConversation(ctx, workspaceID, conversationID, false)
		if err != nil {
			return nil, "", fmt.Errorf("list conversation history: %w", err)
		}
		return sanitizeConversationHistory(history, ""), "conversation", nil
	}

	return nil, "none", nil
}

func (s *SupportAIService) previewSupportReply(
	ctx context.Context,
	workspaceID string,
	agent *model.Agent,
	history []model.SupportMessage,
	customerMessage string,
	includeAnswer bool,
	maxResults int,
	confidenceThreshold float64,
	conversationSource string,
	welcomeMessage string,
) (*model.SupportAIPreviewResponse, error) {
	if s.llmProvider == nil {
		return nil, fmt.Errorf("support chat LLM provider is not configured")
	}
	previewDeadline := time.Now().Add(30 * time.Second)

	// The LLM planner was retired with the auto-reply pipeline; preview uses
	// the deterministic default plan (the runtime agent path does its own
	// routing via tools).
	_ = welcomeMessage
	queryPlan := defaultSupportQueryPlan(customerMessage)
	plannerTokens := 0
	fallbackUsed := true
	plannerError := ""

	response := &model.SupportAIPreviewResponse{
		ConversationSource:  conversationSource,
		ConfidenceThreshold: confidenceThreshold,
		FinalDecision:       queryPlan.Decision,
		FinalReason:         queryPlan.Reason,
		TotalTokensUsed:     plannerTokens,
		QueryPlan: model.SupportAIPreviewQueryPlan{
			Route:              queryPlan.Route,
			Decision:           queryPlan.Decision,
			Intent:             queryPlan.Intent,
			Subject:            queryPlan.Subject,
			Language:           queryPlan.Language,
			Risk:               queryPlan.Risk,
			RequiredEvidence:   cloneStringSlice(queryPlan.RequiredEvidence),
			EvidenceMode:       queryPlan.EvidenceMode,
			RegistryVersion:    queryPlan.RegistryVersion,
			ContextAction:      queryPlan.ContextAction,
			IssueKey:           queryPlan.IssueKey,
			IssueSummary:       queryPlan.IssueSummary,
			ProgressSignal:     queryPlan.ProgressSignal,
			StandaloneQuery:    queryPlan.StandaloneQuery,
			SearchQueries:      cloneStringSlice(queryPlan.SearchQueries),
			ClarifyingQuestion: queryPlan.ClarifyingQuestion,
			GreetingReply:      queryPlan.GreetingReply,
			Reason:             queryPlan.Reason,
			TokensUsed:         plannerTokens,
			FallbackUsed:       fallbackUsed,
			Error:              plannerError,
		},
		Retrieval: model.SupportAIPreviewRetrieval{
			QueryCount:      len(queryPlan.SearchQueries),
			EvidenceFound:   map[string][]string{},
			EvidenceMissing: []string{},
			Results:         []model.SupportAIPreviewSearchResult{},
		},
	}

	switch queryPlan.Decision {
	case supportDecisionClarify, supportDecisionHandoff, supportDecisionGreet:
		return response, nil
	}

	searchResults, retrievalErr := s.loadKnowledgeChunks(ctx, workspaceID, agent.ID, queryPlan.Language, queryPlan.SearchQueries)
	if retrievalErr != nil {
		response.Retrieval.Error = retrievalErr.Error()
		searchResults = nil
	}
	coverage := supportEvidenceCoverage{Found: map[string][]string{}, Missing: []string{}}
	response.Retrieval.EvidenceFound = coverage.Found
	response.Retrieval.EvidenceMissing = cloneStringSlice(coverage.Missing)
	if maxResults > 0 && len(searchResults) > maxResults {
		searchResults = searchResults[:maxResults]
	}
	response.Retrieval.ResultCount = len(searchResults)
	response.Retrieval.Results = previewSearchResults(searchResults)

	if !includeAnswer {
		return response, nil
	}

	providerName, modelName := resolveSupportLLMConfig(agent)
	contextResults := selectSupportEvidenceContext(queryPlan, coverage, searchResults, 8)
	generationCtx, cancelGeneration := context.WithDeadline(ctx, previewDeadline)
	answer, answerTokens, err := s.generateResponseWithPlan(generationCtx, agent, nil, history, buildKnowledgeContext(contextResults), model.SupportMessage{
		SenderType: "customer",
		Content:    customerMessage,
	}, providerName, modelName, queryPlan)
	cancelGeneration()
	if err != nil {
		return nil, fmt.Errorf("generate preview response: %w", err)
	}
	answer.SourceDocIDs = publicSourceDocIDs(answer.SourceDocIDs, searchResults)
	validation := validateSupportAnswer(queryPlan, coverage, searchResults, answer)
	if validation.Outcome != supportValidationPass {
		answer.CanAnswer = false
		answer.Confidence = 0
	}
	response.TotalTokensUsed += answerTokens

	groundedConfidence := evaluateConfidence(searchResults, answer, isGreetingMessage(customerMessage))
	response.Answer = &model.SupportAIPreviewAnswer{
		Content:            answer.Content,
		CanAnswer:          answer.CanAnswer,
		SourceDocIDs:       cloneStringSlice(answer.SourceDocIDs),
		LLMConfidence:      answer.Confidence,
		GroundedConfidence: groundedConfidence,
		TokensUsed:         answerTokens,
		Provider:           providerName,
		Model:              modelName,
		ValidationOutcome:  validation.Outcome,
		ValidationReasons:  cloneStringSlice(validation.Reasons),
		MaterialClaims:     validation.MaterialClaimCount,
		SupportedClaims:    validation.SupportedClaimCount,
	}

	if answer.CanAnswer && groundedConfidence >= confidenceThreshold {
		response.FinalDecision = supportDecisionAnswer
		response.FinalReason = queryPlan.Reason
		return response, nil
	}

	response.FinalDecision = supportDecisionHandoff
	response.FinalReason = "low_confidence"
	return response, nil
}

// generateResponse calls the LLM with knowledge context and conversation history.
func (s *SupportAIService) generateResponse(
	ctx context.Context,
	agent *model.Agent,
	conv *model.SupportConversation,
	history []model.SupportMessage,
	knowledgeContext string,
	customerMessage model.SupportMessage,
	providerName string,
	modelName string,
) (*AIResponseContract, int, error) {
	return s.generateResponseWithPlan(
		ctx,
		agent,
		conv,
		history,
		knowledgeContext,
		customerMessage,
		providerName,
		modelName,
		defaultSupportQueryPlan(supportMessagePromptText(customerMessage)),
	)
}

func (s *SupportAIService) generateResponseWithPlan(
	ctx context.Context,
	agent *model.Agent,
	conv *model.SupportConversation,
	history []model.SupportMessage,
	knowledgeContext string,
	customerMessage model.SupportMessage,
	providerName string,
	modelName string,
	plan SupportQueryPlanContract,
) (*AIResponseContract, int, error) {
	return s.generateResponseWithPlanRevision(ctx, agent, conv, history, knowledgeContext, customerMessage, providerName, modelName, plan, "")
}

func (s *SupportAIService) generateResponseWithPlanRevision(
	ctx context.Context,
	agent *model.Agent,
	conv *model.SupportConversation,
	history []model.SupportMessage,
	knowledgeContext string,
	customerMessage model.SupportMessage,
	providerName string,
	modelName string,
	plan SupportQueryPlanContract,
	revisionInstruction string,
) (*AIResponseContract, int, error) {
	if s == nil || s.llmProvider == nil {
		return nil, 0, fmt.Errorf("support chat LLM provider is not configured")
	}

	systemPrompt := buildAISystemPromptWithPlan(agent, knowledgeContext, plan)
	if strings.TrimSpace(revisionInstruction) != "" {
		systemPrompt += "\n\nREVISION REQUIRED:\n" + revisionInstruction
	}

	messages := make([]llm.Message, 0, len(history)+1)
	messages = append(messages, buildConversationMessages(history)...)
	messages = append(messages, llm.Message{
		Role:         "user",
		Content:      "<customer_message>\n" + supportMessagePromptText(customerMessage) + "\n</customer_message>",
		ContentParts: buildSupportCustomerContentParts(customerMessage),
	})

	workspaceID := ""
	conversationID := ""
	if conv != nil {
		workspaceID = conv.WorkspaceID
		conversationID = conv.ID
	} else if agent != nil {
		workspaceID = agent.WorkspaceID
	}
	messageID := customerMessage.ID
	resp, err := completeAI(ctx, s.llmProvider, AICompletionRequest{
		WorkspaceID:    workspaceID,
		FeatureKey:     BillingFeatureSupportAIReply,
		IdempotencyKey: aiUsageIdempotencyKey(workspaceID, BillingFeatureSupportAIReply, conversationID, messageID),
		Metadata: map[string]interface{}{
			"conversation_id": conversationID,
			"message_id":      messageID,
		},
		PreferredRoute: &AICompletionRoute{
			Provider: providerName, Model: modelName, ServiceTier: defaultAICompletionServiceTier,
		},
		Chat: llm.ChatRequest{
			SystemPrompt:     systemPrompt,
			Messages:         messages,
			Temperature:      0.3,
			MaxTokens:        1024,
			JSONMode:         true,
			JSONSchema:       supportAnswerJSONSchema(),
			JSONSchemaStrict: true,
		},
	})
	if err != nil {
		return nil, 0, err
	}

	contract, cleanedContent, ok := parseAIResponse(resp.Content)
	totalTokens := resp.TokensUsed.InputTokens + resp.TokensUsed.OutputTokens

	if !ok {
		slog.ErrorContext(ctx, "AI response JSON parse failed — refusing ungrounded reply",
			"provider", providerName,
			"model", modelName,
			"raw_content_prefix", truncateLog(resp.Content, 200),
		)
		return &AIResponseContract{
			Content:    cleanedContent,
			CanAnswer:  false,
			Confidence: 0,
		}, totalTokens, nil
	}
	if isTemplateLikeAIContent(contract.Content) {
		slog.ErrorContext(ctx, "AI response matched prompt placeholder — refusing templated reply",
			"provider", providerName,
			"model", modelName,
			"content_preview", truncateLog(contract.Content, 120),
		)
		return &AIResponseContract{
			CanAnswer:  false,
			Confidence: 0,
		}, totalTokens, nil
	}

	slog.InfoContext(ctx, "AI response parsed",
		"provider", providerName,
		"model", modelName,
		"can_answer", contract.CanAnswer,
		"confidence", contract.Confidence,
		"source_count", len(contract.SourceDocIDs),
		"claim_count", len(contract.Claims),
	)

	return &contract, totalTokens, nil
}

func sanitizeConversationHistory(history []model.SupportMessage, currentMessageID string) []model.SupportMessage {
	if len(history) == 0 {
		return nil
	}

	sanitized := make([]model.SupportMessage, 0, len(history))
	for _, msg := range history {
		if currentMessageID != "" && msg.ID == currentMessageID {
			continue
		}
		if msg.MessageType == "system" {
			continue
		}
		if strings.TrimSpace(msg.Content) == "" && len(msg.Attachments) == 0 {
			continue
		}
		sanitized = append(sanitized, msg)
	}
	return sanitized
}

func hasEscalationMessageInHistory(history []model.SupportMessage) bool {
	for _, msg := range history {
		if isEscalationSystemEvent(msg) {
			return true
		}
		if (msg.SenderType == "ai" || (msg.SenderType == "agent" && msg.MessageType == "system")) && containsHandoffLanguage(msg.Content) {
			return true
		}
	}
	return false
}

// systemEventForEscalationReason maps the reason argument passed to
// EscalateToHuman to the appropriate internal-only system event type.
// Customer-driven reasons surface as "customer_requested_human"; AI-driven
// reasons (low_confidence, stuck, etc.) surface as "ai_escalated".
func systemEventForEscalationReason(reason string) model.SupportSystemEventType {
	if reason == "customer_requested" || reason == "customer_requested_human" {
		return model.SystemEventCustomerRequestedHuman
	}
	return model.SystemEventAIEscalated
}

func isEscalationSystemEvent(msg model.SupportMessage) bool {
	if msg.SystemEventType == nil {
		return false
	}
	switch *msg.SystemEventType {
	case model.SystemEventAIEscalated,
		model.SystemEventCustomerRequestedHuman:
		return true
	}
	return false
}

func containsHandoffLanguage(content string) bool {
	lower := strings.ToLower(strings.TrimSpace(content))
	if lower == "" {
		return false
	}
	handoffPhrases := []string{
		"connect you with a team member",
		"connect you to a team member",
		"connect you with our team",
		"connect you to our team",
		"let me connect you",
		"hand you over",
		"handover to",
		"hand off to",
		"transfer you",
		"pass you to",
		"real person",
		"human agent",
	}
	for _, phrase := range handoffPhrases {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	return false
}

func buildConversationMessages(history []model.SupportMessage) []llm.Message {
	if len(history) == 0 {
		return nil
	}

	messages := make([]llm.Message, 0, len(history))
	for _, msg := range history {
		role := "user"
		switch msg.SenderType {
		case "agent", "user", "ai":
			role = "assistant"
		}
		messages = append(messages, llm.Message{Role: role, Content: supportMessagePromptText(msg)})
	}
	return messages
}

const supportTaskDraftSystemPrompt = `You convert support conversations into one internal PM task draft.

Return JSON with exactly these fields:
- title: concise issue-oriented task title
- summary: 1-2 sentence summary
- description_markdown: internal markdown task description
- task_type: one of "feature", "bug", "chore"
- priority: one of "none", "low", "medium", "high", "urgent"

Rules:
- Create exactly one task.
- Focus on the concrete work the team should do next.
- Do not write a customer reply.
- Make the title name the issue or request itself, not the action to take.
- Avoid titles that start with generic verbs like "Investigate", "Fix", "Handle", or "Follow up" unless unavoidable.
- Include useful reproduction context, observed impact, and the latest customer need when present.
- Keep title short, specific, and product-facing.
- Write description_markdown as an adaptive internal brief with:
  - ## Problem
  - ## Impact
  - ## Requested Outcome
- Add ## Reproduction only when concrete repro steps exist in the conversation.
- Add ## Customer Context or ## Internal Notes only when they add useful detail.
- Do not emit empty sections.
- Prefer "bug" when the conversation describes something broken, failing, or incorrect.
- Prefer "feature" for requests or missing capability.
- Prefer "chore" for operational follow-up, cleanup, or non-user-facing work.
- If priority is unclear, use "medium".`

func supportTaskDraftJSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title":                map[string]any{"type": "string"},
			"summary":              map[string]any{"type": "string"},
			"description_markdown": map[string]any{"type": "string"},
			"task_type": map[string]any{
				"type": "string",
				"enum": []string{"feature", "bug", "chore"},
			},
			"priority": map[string]any{
				"type": "string",
				"enum": []string{"none", "low", "medium", "high", "urgent"},
			},
		},
		"required": []string{
			"title",
			"summary",
			"description_markdown",
			"task_type",
			"priority",
		},
		"additionalProperties": false,
	}
}

func previewHistoryToMessages(history []model.SupportAIPreviewHistoryTurn) []model.SupportMessage {
	if len(history) == 0 {
		return nil
	}

	messages := make([]model.SupportMessage, 0, len(history))
	for _, turn := range history {
		messageType := strings.TrimSpace(turn.MessageType)
		if messageType == "" {
			messageType = "reply"
		}
		messages = append(messages, model.SupportMessage{
			SenderType:  strings.TrimSpace(turn.SenderType),
			MessageType: messageType,
			Content:     strings.TrimSpace(turn.Content),
		})
	}
	return messages
}

func normalizePreviewMaxResults(raw *int) int {
	if raw == nil {
		return 8
	}
	value := *raw
	if value <= 0 {
		return 8
	}
	if value > 12 {
		return 12
	}
	return value
}

func (s *SupportAIService) loadRewriteHistory(ctx context.Context, workspaceID, conversationID string) ([]model.SupportMessage, error) {
	if conversationID == "" {
		return nil, nil
	}
	if s.conversationRepo != nil {
		conv, err := s.conversationRepo.GetByID(ctx, workspaceID, conversationID, "", model.RoleOwner)
		if err != nil {
			return nil, fmt.Errorf("get conversation: %w", err)
		}
		if conv == nil {
			return nil, fmt.Errorf("%w: %s", ErrSupportRewriteConversationNotFound, conversationID)
		}
	}
	if s.messageRepo == nil {
		return nil, nil
	}
	history, err := s.messageRepo.ListByConversation(ctx, workspaceID, conversationID, false)
	if err != nil {
		return nil, fmt.Errorf("list conversation history: %w", err)
	}
	history = sanitizeConversationHistory(history, "")
	filtered := history[:0]
	for _, msg := range history {
		if msg.IsInternal {
			continue
		}
		filtered = append(filtered, msg)
	}
	return filtered, nil
}

func normalizeSupportRewriteOperation(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case supportRewriteExpand:
		return supportRewriteExpand
	case supportRewriteRephrase:
		return supportRewriteRephrase
	case supportRewriteFixGrammar:
		return supportRewriteFixGrammar
	case supportRewriteFriendly:
		return supportRewriteFriendly
	case supportRewriteFormal:
		return supportRewriteFormal
	default:
		return ""
	}
}

func buildSupportRewriteSystemPrompt(operation string) string {
	return buildDraftRewriteSystemPrompt("support reply", operation)
}

func buildDraftRewriteSystemPrompt(surface, operation string) string {
	var instruction string
	switch operation {
	case supportRewriteExpand:
		instruction = "Make the draft more complete and helpful. Add useful detail, but stay concise and avoid fluff."
	case supportRewriteRephrase:
		instruction = "Rewrite the draft for clarity and flow without materially changing its meaning or overall length."
	case supportRewriteFixGrammar:
		instruction = "Fix grammar, spelling, punctuation, and readability issues only. Preserve meaning, tone, and structure as much as possible."
	case supportRewriteFriendly:
		instruction = "Make the draft warmer, more empathetic, and more customer-friendly while staying professional."
	case supportRewriteFormal:
		instruction = "Make the draft more formal, polished, and professional while keeping it natural and helpful."
	default:
		instruction = "Improve the draft while preserving intent."
	}

	return strings.TrimSpace(`You rewrite ` + strings.TrimSpace(surface) + ` drafts for people.

Return a JSON object with a single "content" field containing only the rewritten draft text.
Do not mention AI, model choice, or that you edited the text.
Do not invent policies, refunds, timelines, or product facts that are not already supported by the draft or the conversation context.
Preserve valid HTML, markdown-style bullets, and links when present. Return the content in the same markup format as the draft.
Preserve the language of the original draft unless the draft itself mixes languages.
` + "\n\n" + instruction)
}

func buildSupportRewriteMessages(history []model.SupportMessage, draft string) []llm.Message {
	messages := make([]llm.Message, 0, len(history)+1)
	if len(history) > 0 {
		history = trimSupportRewriteHistory(history, 8)
		messages = append(messages, llm.Message{
			Role:    "user",
			Content: buildSupportRewriteHistoryPrompt(history),
		})
	}
	messages = append(messages, llm.Message{
		Role: "user",
		Content: "<draft_reply>\n" + strings.TrimSpace(draft) + "\n</draft_reply>\n\n" +
			"Rewrite the draft now and return valid JSON.",
	})
	return messages
}

func trimSupportRewriteHistory(history []model.SupportMessage, limit int) []model.SupportMessage {
	if limit <= 0 || len(history) <= limit {
		return history
	}
	return history[len(history)-limit:]
}

func buildSupportRewriteHistoryPrompt(history []model.SupportMessage) string {
	var b strings.Builder
	b.WriteString("Here is the recent conversation context. Use it only to preserve factual consistency.\n\n<conversation_history>\n")
	for _, msg := range history {
		role := strings.TrimSpace(msg.SenderType)
		if role == "" {
			role = "unknown"
		}
		b.WriteString("[")
		b.WriteString(role)
		b.WriteString("]\n")
		b.WriteString(strings.TrimSpace(supportMessagePromptText(msg)))
		b.WriteString("\n\n")
	}
	b.WriteString("</conversation_history>")
	return b.String()
}

func parseSupportRewriteResponse(raw string) (string, bool) {
	type contract struct {
		Content string `json:"content"`
	}

	var parsed contract
	if err := llm.UnmarshalResponse(raw, &parsed); err == nil {
		content := strings.TrimSpace(parsed.Content)
		if content != "" {
			return content, true
		}
	}

	return "", false
}

func previewSearchResults(results []KnowledgeSearchResult) []model.SupportAIPreviewSearchResult {
	if len(results) == 0 {
		return []model.SupportAIPreviewSearchResult{}
	}

	preview := make([]model.SupportAIPreviewSearchResult, 0, len(results))
	for _, result := range results {
		preview = append(preview, model.SupportAIPreviewSearchResult{
			ReferenceID:   result.ReferenceID,
			SourceType:    result.SourceType,
			Title:         result.Title,
			URL:           result.URL,
			ChunkIndex:    result.ChunkIndex,
			CombinedScore: result.CombinedScore,
			VectorScore:   result.VectorScore,
			LexicalScore:  result.LexicalScore,
			Snippet:       excerptText(result.Content, 220),
		})
	}
	return preview
}
