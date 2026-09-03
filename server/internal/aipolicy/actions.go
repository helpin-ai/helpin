package aipolicy

import "time"

const (
	ActionSupportCoverageAnalyze     = "support.coverage.analyze.v1"
	ActionSupportCoverageRefine      = "support.coverage.refine.v1"
	ActionSupportCoverageEmbed       = "support.coverage.embed.v1"
	ActionSupportCoverageAssignTopic = "support.coverage.assign_topic.v1"
	ActionDocsEmbed                  = "docs.embedding.index.v1"
	ActionSupportKnowledgeEmbed      = "support.knowledge.embed.v1"
	ActionHelpcenterSearchEmbed      = "support.helpcenter.search_embed.v1"
	ActionCuratedGuidanceEmbed       = "support.curated_guidance.embed.v1"
	ActionPlatformRerank             = "platform.rerank.v1"
	ActionAskMediaEnrichment         = "agents.ask.media_enrichment.v1"
)

type featureSeed struct {
	key        string
	label      string
	category   Category
	origin     string
	floor      int
	chargeable bool
	modality   Modality
}

func defaultActions() []Action {
	actions := []Action{
		coverageChatAction(ActionSupportCoverageAnalyze, "Coverage gap analysis"),
		coverageChatAction(ActionSupportCoverageRefine, "Coverage recommendation refinement"),
		coverageEmbeddingAction(),
		coverageChatAction(ActionSupportCoverageAssignTopic, "Coverage topic assignment"),
		embeddingAction(ActionDocsEmbed, "Docs semantic embedding", CategoryDocsAI, "docs"),
		embeddingAction(ActionSupportKnowledgeEmbed, "Support knowledge embedding", CategorySupportAI, "support"),
		embeddingAction(ActionHelpcenterSearchEmbed, "Help-center search embedding", CategorySupportAI, "helpcenter"),
		embeddingAction(ActionCuratedGuidanceEmbed, "Curated guidance embedding", CategorySupportAI, "support"),
		{
			Key: ActionAskMediaEnrichment, PolicyVersion: "v1", FeatureKey: "ask_chat",
			Label: "Ask media enrichment", Category: CategoryAgents, Origin: "ask",
			Modality: ModalityChat, DefaultProvider: "openrouter", DefaultModel: "google/gemini-3.7-flash",
			AllowedModels: defaultChatModels(), Timeout: 2 * time.Minute,
			MaxInputTokens: 200000, MaxOutputTokens: 700, MaxReasoningTokens: 32000,
			RetryClass: RetryTransient, Autonomy: AutonomyAnalyze,
			DataClass: DataClassWorkspaceData, FloorUnits: 40, Chargeable: true,
		},
		{
			Key: ActionPlatformRerank, PolicyVersion: "v1", FeatureKey: "ai_rerank",
			Label: "AI search reranking", Category: CategorySupportAI, Origin: "search",
			Modality: ModalityRerank, DefaultProvider: "cohere", DefaultModel: "rerank-v3.5",
			AllowedModels: map[string][]string{"cohere": {"rerank-v3.5"}},
			Timeout:       15 * time.Second, MaxInputTokens: 32000, RetryClass: RetryTransient,
			Autonomy: AutonomyAnalyze, DataClass: DataClassWorkspaceData, Chargeable: false,
		},
	}
	seeds := []featureSeed{
		{"ai_routing", "AI triage and routing", CategorySupportAI, "support", 2, true, ModalityChat},
		{"crm_signal_detection", "CRM signal detection", CategoryCRMAI, "crm", 3, true, ModalityChat},
		{"support_reply_rewrite", "Support reply rewrite", CategorySupportAI, "support", 4, true, ModalityChat},
		{"pm_comment_rewrite", "Project comment rewrite", CategoryProjectAI, "project", 4, true, ModalityChat},
		{"crm_email_rewrite", "CRM email rewrite", CategoryCRMAI, "crm", 4, true, ModalityChat},
		{"deal_automation_inference", "Deal automation inference", CategoryCRMAI, "crm", 5, true, ModalityChat},
		{"meeting_intelligence", "Meeting intelligence", CategoryCRMAI, "crm", 8, true, ModalityChat},
		{"crm_summary", "CRM summary", CategoryCRMAI, "crm", 6, true, ModalityChat},
		{"task_standing_brief", "Task standing brief", CategoryProjectAI, "project", 6, true, ModalityChat},
		{"support_ai_reply", "Support reply draft", CategorySupportAI, "support", 8, true, ModalityChat},
		{"support_task_draft", "Support task draft", CategorySupportAI, "support", 8, true, ModalityChat},
		{"docs_generation", "Document generation", CategoryDocsAI, "docs", 15, true, ModalityChat},
		{"docs_article_translation", "Help article translation", CategoryDocsAI, "docs", 15, true, ModalityChat},
		{"docs_article_generation", "Help article generation", CategoryDocsAI, "docs", 20, true, ModalityChat},
		{"docs_import_conversion", "Help article import formatting", CategoryDocsAI, "docs", 15, true, ModalityChat},
		{"helpcenter_answer_generation", "Help-center answer generation", CategoryDocsAI, "docs", 2, true, ModalityChat},
		{"crm_action", "CRM / deal action", CategoryCRMAI, "crm", 5, true, ModalityChat},
		{"built_in_light_agent_run", "Built-in agent run", CategoryAgents, "agent", 40, true, ModalityExternalAgent},
		{"ask_chat", "Ask Chat", CategoryAgents, "agent", 40, true, ModalityExternalAgent},
		{"planning_run", "Planning run", CategoryAgents, "agent", 80, true, ModalityExternalAgent},
		{"scribe_run", "Scribe task planning run", CategoryAgents, "agent", 50, true, ModalityExternalAgent},
		{"mira_run", "Mira marketing run", CategoryAgents, "agent", 50, true, ModalityExternalAgent},
		{"quill_run", "Quill documentation run", CategoryAgents, "agent", 60, true, ModalityExternalAgent},
		{"custom_agent_run", "Custom agent run", CategoryAgents, "agent", 60, true, ModalityExternalAgent},
		{"atlas_run", "Atlas epic planning run", CategoryAgents, "agent", 80, true, ModalityExternalAgent},
		{"coding_run", "Coding / review run", CategoryAgents, "agent", 100, true, ModalityExternalAgent},
		{"forge_run", "Forge coding run", CategoryAgents, "agent", 100, true, ModalityExternalAgent},
		{"lens_run", "Lens review run", CategoryAgents, "agent", 100, true, ModalityExternalAgent},
		{"custom_coding_review_run", "Custom coding/review run", CategoryAgents, "agent", 100, true, ModalityExternalAgent},
		{"custom_agent_draft", "Custom agent draft", CategorySetup, "setup", 0, false, ModalityChat},
		{"automation_setup", "Automation setup", CategorySetup, "setup", 0, false, ModalityChat},
		{"flow_setup", "Flow setup", CategorySetup, "setup", 0, false, ModalityChat},
		{"agent_prompt_improvement", "Agent prompt improvement", CategorySetup, "setup", 0, false, ModalityChat},
		{"data_import_setup", "Data import setup", CategorySetup, "setup", 0, false, ModalityChat},
		{"company_product_context_generation", "Company/product context generation", CategorySetup, "setup", 0, false, ModalityChat},
		{"dock_chat_title_generation", "Dock chat title", CategoryAgents, "agent", 0, false, ModalityChat},
	}
	for _, seed := range seeds {
		actions = append(actions, seededAction(seed))
	}
	return actions
}

func coverageChatAction(key, label string) Action {
	return Action{
		Key: key, PolicyVersion: "v1", FeatureKey: "coverage_gap_analysis", Label: label,
		Category: CategorySupportAI, Origin: "coverage", Modality: ModalityChat,
		DefaultProvider: "anthropic", DefaultModel: "claude-sonnet-4-6",
		AllowedModels: defaultChatModels(),
		Fallbacks:     []Route{{Provider: "openrouter", Model: "openai/gpt-5.6-luna"}},
		Timeout:       45 * time.Second, MaxInputTokens: 32000, MaxOutputTokens: 1800,
		MaxReasoningTokens: 0, RetryClass: RetryTransient, Autonomy: AutonomyAnalyze,
		DataClass: DataClassCustomerContent, FloorUnits: 2, Chargeable: true,
	}
}

func coverageEmbeddingAction() Action {
	action := embeddingAction(ActionSupportCoverageEmbed, "Coverage semantic embedding", CategorySupportAI, "coverage")
	action.FeatureKey = "coverage_gap_analysis"
	action.FloorUnits = 2
	action.Chargeable = true
	action.DataClass = DataClassCustomerContent
	return action
}

func embeddingAction(key, label string, category Category, origin string) Action {
	return Action{
		Key: key, PolicyVersion: "v1", FeatureKey: "semantic_embedding", Label: label,
		Category: category, Origin: origin, Modality: ModalityEmbedding,
		DefaultProvider: "openai", DefaultModel: "text-embedding-3-small",
		AllowedModels: map[string][]string{"openai": {"text-embedding-3-small"}},
		Timeout:       30 * time.Second, MaxInputTokens: 8191, RetryClass: RetryTransient,
		Autonomy: AutonomyAnalyze, DataClass: DataClassWorkspaceData, Chargeable: false,
	}
}

func seededAction(seed featureSeed) Action {
	provider, model := "anthropic", "claude-sonnet-4-6"
	allowed := defaultChatModels()
	maxOutput := 4096
	if seed.modality == ModalityExternalAgent {
		provider, model = "helpin-runtime", "policy-selected"
		allowed = map[string][]string{"helpin-runtime": {"policy-selected"}}
		maxOutput = 1
	}
	return Action{
		Key: "feature." + seed.key + ".v1", PolicyVersion: "v1", FeatureKey: seed.key,
		Label: seed.label, Category: seed.category, Origin: seed.origin, Modality: seed.modality,
		DefaultProvider: provider, DefaultModel: model, AllowedModels: allowed,
		Timeout: 5 * time.Minute, MaxInputTokens: 200000, MaxOutputTokens: maxOutput,
		MaxReasoningTokens: 100000, RetryClass: RetryTransient, Autonomy: AutonomyAssist,
		DataClass: DataClassWorkspaceData, FloorUnits: seed.floor, Chargeable: seed.chargeable,
	}
}

func defaultChatModels() map[string][]string {
	return map[string][]string{
		"anthropic":  {"claude-haiku-4-5", "claude-sonnet-4-6", "claude-sonnet-5", "claude-opus-4-8"},
		"openai":     {"gpt-5-mini", "gpt-5.5", "gpt-5.6-luna", "gpt-5.6-terra"},
		"openrouter": {"z-ai/glm-5.3-flash:exacto", "deepseek/deepseek-v4-flash-0731", "openai/gpt-5.6-luna", "openai/gpt-5.6-terra", "anthropic/claude-sonnet-5", "google/gemini-3.7-flash"},
	}
}
