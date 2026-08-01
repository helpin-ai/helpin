package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func builtInAgentTemplates() []model.AgentTemplate {
	releaseNotesDescription := "Creates release notes from GitHub releases, related tasks, and linked docs."
	releaseNotesStarterFlows := model.JSONBlob(mustJSONValue([]map[string]any{
		{
			"key":               "github_release_notes",
			"label":             "Start on GitHub releases",
			"description":       "Runs when GitHub publishes a matching release and writes release notes to Docs.",
			"trigger_type":      model.TriggerGitHubReleasePub,
			"default_enabled":   true,
			"config_schema_key": "release_notes_github",
			"output_type":       "docs_document",
		},
	}))
	competitiveIntelDescription := "Tracks recent competitor changelog updates and creates a recurring tracking report task."
	competitiveIntelSystemPrompt := `You are a competitors changelog tracking agent for {{target_company}}.

Configured report:
- target_company: {{target_company}}
- target_domain: {{target_domain}}
- competitors: {{competitors}}
- lookback_days: {{lookback_days}}
- destination_team_id: {{destination_team_id}}
- destination_state_id: {{destination_state_id}}
- schedule_preset: {{schedule_preset}}

Treat these configured values as already resolved and authoritative. Do not plan or perform discovery of configuration variables, workspace context, teams, stages, cadence, or lookback settings.

Use the configured competitor list when it is not empty. If no competitors are configured, discover competitors with web search and cite sources.

For each competitor, first use web_search_exa (or the runtime's built-in web search when available) to find official changelog, release notes, product updates, blog, docs, or roadmap pages. Then use fetch_url on exact source URLs to verify page content and dates. If search is thin, use crawl_url on the competitor's official website or docs host with changelog/update keywords before marking no_public_changelog. A missing search provider is a tool limitation, not evidence that a competitor has no public changelog.

Create exactly one competitors changelog tracking report task with create_task. Pass destination_team_id directly as team_id. Pass destination_state_id directly as state_id only when it is configured; otherwise let the team default stage apply.

Raw configuration:
{{raw_configuration_json}}`
	competitiveIntelStarterFlows := model.JSONBlob(mustJSONValue([]map[string]any{
		{
			"key":               "competitors_changelog_scheduled",
			"label":             "Run competitors changelog report on a schedule",
			"description":       "Runs on the selected cadence, researches recent competitor changelog updates, and creates one report task.",
			"trigger_type":      model.TriggerCron,
			"default_enabled":   true,
			"config_schema_key": "competitive_intel_cron",
			"output_type":       "task",
			"fields": []map[string]any{
				{
					"key":         "target_company",
					"label":       "Target company",
					"type":        "text",
					"required":    true,
					"placeholder": "Usermaven",
				},
				{
					"key":         "target_domain",
					"label":       "Target domain",
					"type":        "text",
					"required":    false,
					"placeholder": "usermaven.com",
				},
				{
					"key":         "competitors",
					"label":       "Known competitors",
					"type":        "string_list",
					"required":    false,
					"placeholder": "jasper.ai",
					"help_text":   "Optional. If empty, the agent discovers competitors during each run.",
				},
				{
					"key":      "schedule_preset",
					"label":    "Run cadence",
					"type":     "select",
					"required": true,
					"default":  "weekly",
					"options": []map[string]any{
						{"value": "daily", "label": "Daily"},
						{"value": "weekly", "label": "Weekly"},
					},
				},
				{
					"key":      "lookback_days",
					"label":    "Lookback window",
					"type":     "number",
					"required": true,
					"default":  7,
					"min":      1,
					"max":      30,
				},
				{
					"key":      "destination_team_id",
					"label":    "Task team",
					"type":     "team_select",
					"required": true,
				},
				{
					"key":        "destination_state_id",
					"label":      "Task stage",
					"type":       "workflow_state_select",
					"required":   false,
					"depends_on": "destination_team_id",
					"help_text":  "Optional. Defaults to the team's default stage.",
				},
			},
		},
	}))
	dependencyAuditorDescription := "Scans repository dependency manifests and creates deduplicated update tasks for outdated direct dependencies."
	dependencyAuditorSystemPrompt := `You are an autonomous dependency auditor for the selected repository.

Configured audit:
- ecosystems: {{ecosystems}}
- include_indirect: {{include_indirect}}
- destination_team_id: {{destination_team_id}}
- destination_state_id: {{destination_state_id}}
- max_tasks: {{max_tasks}}
- schedule_preset: {{schedule_preset}}

Treat these configured values as already resolved and authoritative. Do not plan or perform discovery of configuration variables, workspace context, teams, stages, cadence, or repository selection.

Scan only the configured ecosystems. Ignore indirect or transitive dependencies unless include_indirect is true. Do not modify files.

Use fetch_url for exact public registry and advisory API GET requests. Do not invoke curl or wget through run_command.

Repositories may contain more than one language ecosystem. Scan every selected ecosystem in the same run and deduplicate tasks within each ecosystem identity.

Create at most one task per outdated direct dependency with create_task, up to max_tasks. Pass destination_team_id directly as team_id. Pass destination_state_id directly as state_id only when it is configured; otherwise let the team default stage apply.

Raw configuration:
{{raw_configuration_json}}`
	dependencyAuditorStarterFlows := model.JSONBlob(mustJSONValue([]map[string]any{
		{
			"key":               "dependency_audit_cron",
			"label":             "Run dependency audit on a schedule",
			"description":       "Runs on the selected cadence against a repository and creates one task per verified outdated direct dependency.",
			"trigger_type":      model.TriggerCron,
			"default_enabled":   true,
			"config_schema_key": "engineering_dependency_auditor_cron",
			"output_type":       "task",
			"fields": []map[string]any{
				{
					"key":      "repository_id",
					"label":    "Repository",
					"type":     "repository_select",
					"required": true,
				},
				{
					"key":      "ecosystems",
					"label":    "Ecosystems",
					"type":     "multi_select",
					"required": true,
					"default":  []string{"go", "rust", "python", "node", "java"},
					"options": []map[string]any{
						{"value": "go", "label": "Go"},
						{"value": "rust", "label": "Rust"},
						{"value": "python", "label": "Python"},
						{"value": "node", "label": "Node / JavaScript"},
						{"value": "java", "label": "Java / JVM"},
					},
				},
				{
					"key":      "include_indirect",
					"label":    "Include indirect/transitive dependencies",
					"type":     "boolean",
					"required": false,
					"default":  false,
				},
				{
					"key":      "schedule_preset",
					"label":    "Run cadence",
					"type":     "select",
					"required": true,
					"default":  "weekly",
					"options": []map[string]any{
						{"value": "daily", "label": "Daily"},
						{"value": "weekly", "label": "Weekly"},
					},
				},
				{
					"key":      "max_tasks",
					"label":    "Maximum tasks per run",
					"type":     "number",
					"required": true,
					"default":  20,
					"min":      1,
					"max":      100,
				},
				{
					"key":      "destination_team_id",
					"label":    "Task team",
					"type":     "team_select",
					"required": true,
				},
				{
					"key":        "destination_state_id",
					"label":      "Task stage",
					"type":       "workflow_state_select",
					"required":   false,
					"depends_on": "destination_team_id",
					"help_text":  "Optional. Defaults to the team's default stage.",
				},
			},
		},
	}))
	securityTriageDescription := "Runs security scanners, triages findings in repository context, and creates actionable remediation tasks."
	securityTriageSystemPrompt := `You are Sentinel, an autonomous security triage agent for the selected repository.

Configured security triage:
- scanners: {{scanners}}
- severity_threshold: {{severity_threshold}}
- include_low_info: {{include_low_info}}
- destination_team_id: {{destination_team_id}}
- destination_state_id: {{destination_state_id}}
- max_tasks: {{max_tasks}}
- schedule_preset: {{schedule_preset}}

Treat these configured values as already resolved and authoritative. Do not plan or perform discovery of configuration variables, workspace context, teams, stages, cadence, or repository selection.

Run only the configured scanners through the dedicated scanner tools: scan_semgrep, scan_trivy, and scan_gitleaks. Do not run scanner CLIs through run_command. Do not modify files.

Use scanner pagination and filtering instead of rerunning a scanner when output is compacted. Start scanner analysis with summary_only: true for counts/groups only, then request detail_level: "index" with category, rule_ids, package_names, vulnerability_ids, paths, page, and page_size for compact finding rows. Use detail_level: "full" only for narrow follow-up inspection.

Triage normalized scanner findings against repository code and configuration. Suppress false positives and non-actionable findings. Create tasks only for applicable findings at or above severity_threshold. If include_low_info is false, do not create tasks for low or informational findings.

Before creating tasks, ensure a shared workspace label named security exists with ensure_task_label. Then call list_tasks with the returned security label_id, open_only: true, detail_level: "compact", and limit: 100. Do not request full descriptions/comments for the first duplicate lookup. Do not filter existing-task lookup by destination_state_id; duplicates must be detected across every open workflow state. Use these open security tasks for duplicate detection. Parse Sentinel markers such as <!-- sentinel:root_cause=... finding_ids=[...] --> from compact task excerpts yourself; do not expect structured marker fields. If an open matching task already exists, do not create a duplicate; add a comment with add_task_comment only when the current scan adds materially new evidence such as new CVEs, affected paths, fixed versions, scanner evidence, or advisory URLs.

Group related findings by fix unit, such as one vulnerable package/manifest upgrade, one secret exposure root cause, one scanner rule/sink remediation, or one misconfiguration remediation. Multiple CVEs may share one task only when the same package/manifest update fixes them together. Put a single-line <!-- sentinel:root_cause=... finding_ids=[...] --> marker at the top of every created task description, capped to roughly 300 characters. Put scan-update markers at the start of comments. Create at most max_tasks new remediation tasks with create_task. Pass destination_team_id directly as team_id. Pass destination_state_id directly as state_id only when it is configured; otherwise let the team default stage apply. Attach the security label to every created task with label_ids.

Raw configuration:
{{raw_configuration_json}}`
	securityTriageStarterFlows := model.JSONBlob(mustJSONValue([]map[string]any{
		{
			"key":               "engineering_security_triage_cron",
			"label":             "Run Sentinel on a schedule",
			"description":       "Runs security scanners against a repository, triages findings, and creates remediation tasks for applicable medium-or-higher issues.",
			"trigger_type":      model.TriggerCron,
			"default_enabled":   true,
			"config_schema_key": "engineering_security_triage_cron",
			"output_type":       "task",
			"fields": []map[string]any{
				{
					"key":      "repository_id",
					"label":    "Repository",
					"type":     "repository_select",
					"required": true,
				},
				{
					"key":      "scanners",
					"label":    "Scanners",
					"type":     "multi_select",
					"required": true,
					"default":  []string{"semgrep", "trivy", "gitleaks"},
					"options": []map[string]any{
						{"value": "semgrep", "label": "Semgrep"},
						{"value": "trivy", "label": "Trivy"},
						{"value": "gitleaks", "label": "Gitleaks"},
					},
				},
				{
					"key":      "severity_threshold",
					"label":    "Minimum severity",
					"type":     "select",
					"required": true,
					"default":  "medium",
					"options": []map[string]any{
						{"value": "critical", "label": "Critical"},
						{"value": "high", "label": "High"},
						{"value": "medium", "label": "Medium"},
					},
				},
				{
					"key":      "include_low_info",
					"label":    "Include low and informational findings",
					"type":     "boolean",
					"required": false,
					"default":  false,
				},
				{
					"key":      "schedule_preset",
					"label":    "Run cadence",
					"type":     "select",
					"required": true,
					"default":  "weekly",
					"options": []map[string]any{
						{"value": "daily", "label": "Daily"},
						{"value": "weekly", "label": "Weekly"},
					},
				},
				{
					"key":      "max_tasks",
					"label":    "Maximum tasks per run",
					"type":     "number",
					"required": true,
					"default":  20,
					"min":      1,
					"max":      100,
				},
				{
					"key":      "destination_team_id",
					"label":    "Task team",
					"type":     "team_select",
					"required": true,
				},
				{
					"key":        "destination_state_id",
					"label":      "Task stage",
					"type":       "workflow_state_select",
					"required":   false,
					"depends_on": "destination_team_id",
					"help_text":  "Optional. Defaults to the team's default stage.",
				},
			},
		},
	}))
	return []model.AgentTemplate{
		{
			Key:         model.AgentTemplateTypeReleaseNotes,
			Name:        "Release Notes Writer",
			Description: &releaseNotesDescription,
			RuntimeKind: model.AgentTemplateRuntimeKindNativeSDK,
			DefaultRole: "Release Notes Writer",
			Skills: model.AgentSkillRefs{
				{Key: "release_notes_writing"},
			},
			AllowedTools: model.JSONBlob(mustJSONStringSlice([]string{
				"get_release_context",
				"get_task_context",
				"find_tasks_for_git_changes",
				"read_document",
				"get_document_blocks",
				"search_documents",
				"list_collections",
				"create_document",
				"link_document_to_object",
			})),
			AllowedCommands:       model.JSONBlob(mustJSONStringSlice(nil)),
			AllowedTargets:        model.JSONBlob(mustJSONStringSlice([]string{"repository"})),
			RequiredContext:       model.JSONBlob(mustJSONStringSlice([]string{"event.github.release.tag_name", "event.github.release.repo_full_name"})),
			StarterFlows:          releaseNotesStarterFlows,
			ApprovalMode:          "never",
			DefaultInvocationMode: model.InvocationModeAutonomous,
			IsEnabled:             true,
		},
		{
			Key:          model.AgentTemplateTypeCompetitiveIntel,
			Name:         "Competitors Changelog Tracking Report",
			Description:  &competitiveIntelDescription,
			SystemPrompt: &competitiveIntelSystemPrompt,
			RuntimeKind:  model.AgentTemplateRuntimeKindNativeSDK,
			DefaultRole:  "Competitors Changelog Analyst",
			Skills: model.AgentSkillRefs{
				{Key: model.AgentTemplateTypeCompetitiveIntel},
			},
			AllowedTools: model.JSONBlob(mustJSONStringSlice([]string{
				"update_plan",
				"web_search_exa",
				"fetch_url",
				"crawl_url",
				"list_workspace_teams",
				"list_team_workflows_with_stages",
				"create_task",
			})),
			AllowedCommands:       model.JSONBlob(mustJSONStringSlice(nil)),
			AllowedTargets:        model.JSONBlob(mustJSONStringSlice([]string{"workspace"})),
			RequiredContext:       model.JSONBlob(mustJSONStringSlice(nil)),
			StarterFlows:          competitiveIntelStarterFlows,
			ApprovalMode:          "never",
			DefaultInvocationMode: model.InvocationModeAutonomous,
			IsEnabled:             true,
		},
		{
			Key:          model.AgentTemplateTypeDependencyAuditor,
			Name:         "Dependency Auditor",
			Description:  &dependencyAuditorDescription,
			SystemPrompt: &dependencyAuditorSystemPrompt,
			RuntimeKind:  model.AgentTemplateRuntimeKindNativeSDK,
			DefaultRole:  "Dependency Auditor",
			Skills: model.AgentSkillRefs{
				{Key: "dependency_audit"},
			},
			AllowedTools: model.JSONBlob(mustJSONStringSlice([]string{
				"update_plan",
				"list_directory",
				"read_file",
				"read_files",
				"read_file_range",
				"search_files",
				"ripgrep",
				"grep",
				"run_command",
				"web_search_exa",
				"fetch_url",
				"create_task",
			})),
			AllowedCommands: model.JSONBlob(mustJSONStringSlice([]string{
				"go",
				"cargo",
				"python",
				"python3",
				"pip",
				"uv",
				"poetry",
				"node",
				"npm",
				"pnpm",
				"yarn",
				"bun",
				"npx",
				"mvn",
				"gradle",
				"./gradlew",
				"java",
				"rg",
				"grep",
				"find",
				"cat",
				"ls",
				"head",
				"tail",
				"pwd",
			})),
			AllowedTargets:        model.JSONBlob(mustJSONStringSlice([]string{"repository"})),
			RequiredContext:       model.JSONBlob(mustJSONStringSlice(nil)),
			StarterFlows:          dependencyAuditorStarterFlows,
			ApprovalMode:          "never",
			DefaultInvocationMode: model.InvocationModeAutonomous,
			IsEnabled:             true,
		},
		{
			Key:          model.AgentTemplateTypeSecurityTriage,
			Name:         "Sentinel",
			Description:  &securityTriageDescription,
			SystemPrompt: &securityTriageSystemPrompt,
			RuntimeKind:  model.AgentTemplateRuntimeKindNativeSDK,
			DefaultRole:  "Security Triage Analyst",
			Skills: model.AgentSkillRefs{
				{Key: "security_triage"},
			},
			AllowedTools: model.JSONBlob(mustJSONStringSlice([]string{
				"update_plan",
				"list_directory",
				"read_file",
				"read_files",
				"read_file_range",
				"search_files",
				"ripgrep",
				"grep",
				"run_command",
				"scan_semgrep",
				"scan_trivy",
				"scan_gitleaks",
				"web_search_exa",
				"ensure_task_label",
				"list_tasks",
				"create_task",
				"add_task_comment",
			})),
			AllowedCommands: model.JSONBlob(mustJSONStringSlice([]string{
				"git",
				"rg",
				"grep",
				"find",
				"cat",
				"ls",
				"head",
				"tail",
				"pwd",
			})),
			AllowedTargets:        model.JSONBlob(mustJSONStringSlice([]string{"repository"})),
			RequiredContext:       model.JSONBlob(mustJSONStringSlice(nil)),
			StarterFlows:          securityTriageStarterFlows,
			ApprovalMode:          "never",
			DefaultInvocationMode: model.InvocationModeAutonomous,
			IsEnabled:             true,
		},
	}
}

func (s *AgentService) EnsureSystemTemplates(ctx context.Context) error {
	if s == nil || s.agentTemplateRepo == nil {
		return nil
	}
	for _, desired := range builtInAgentTemplates() {
		if err := s.ensureSystemTemplate(ctx, desired); err != nil {
			return err
		}
	}
	return nil
}

func (s *AgentService) ensureSystemTemplate(ctx context.Context, desired model.AgentTemplate) error {
	existing, err := s.agentTemplateRepo.GetByKey(ctx, nil, desired.Key)
	if err != nil {
		return err
	}
	if existing == nil {
		template := desired
		template.WorkspaceID = nil
		return s.agentTemplateRepo.Create(ctx, &template)
	}

	changed := false
	if strings.TrimSpace(existing.Name) != strings.TrimSpace(desired.Name) {
		existing.Name = desired.Name
		changed = true
	}
	if stringOrDefault(existing.Description, "") != stringOrDefault(desired.Description, "") {
		existing.Description = trimPtr(desired.Description)
		changed = true
	}
	if strings.TrimSpace(existing.RuntimeKind) != strings.TrimSpace(desired.RuntimeKind) {
		existing.RuntimeKind = desired.RuntimeKind
		changed = true
	}
	if strings.TrimSpace(existing.DefaultRole) != strings.TrimSpace(desired.DefaultRole) {
		existing.DefaultRole = desired.DefaultRole
		changed = true
	}
	if string(normalizeExecutionConfigJSON(existing.ExecutionConfig)) != string(normalizeExecutionConfigJSON(desired.ExecutionConfig)) {
		existing.ExecutionConfig = normalizeExecutionConfigJSON(desired.ExecutionConfig)
		changed = true
	}
	if stringOrDefault(existing.SystemPrompt, "") != stringOrDefault(desired.SystemPrompt, "") {
		existing.SystemPrompt = trimPtr(desired.SystemPrompt)
		changed = true
	}
	if stringOrDefault(existing.PlanningNotes, "") != stringOrDefault(desired.PlanningNotes, "") {
		existing.PlanningNotes = trimPtr(desired.PlanningNotes)
		changed = true
	}
	if !equalAgentSkillRefs(existing.Skills, desired.Skills) {
		existing.Skills = desired.Skills.Normalize()
		changed = true
	}
	if string(normalizeAllowedToolsJSON(json.RawMessage(existing.AllowedTools))) != string(normalizeAllowedToolsJSON(json.RawMessage(desired.AllowedTools))) {
		existing.AllowedTools = model.JSONBlob(normalizeAllowedToolsJSON(json.RawMessage(desired.AllowedTools)))
		changed = true
	}
	if string(normalizeJSONSlice(json.RawMessage(existing.AllowedCommands))) != string(normalizeJSONSlice(json.RawMessage(desired.AllowedCommands))) {
		existing.AllowedCommands = model.JSONBlob(normalizeJSONSlice(json.RawMessage(desired.AllowedCommands)))
		changed = true
	}
	if string(normalizeJSONSlice(json.RawMessage(existing.AllowedTargets))) != string(normalizeJSONSlice(json.RawMessage(desired.AllowedTargets))) {
		existing.AllowedTargets = model.JSONBlob(normalizeJSONSlice(json.RawMessage(desired.AllowedTargets)))
		changed = true
	}
	if string(normalizeJSONSlice(json.RawMessage(existing.RequiredContext))) != string(normalizeJSONSlice(json.RawMessage(desired.RequiredContext))) {
		existing.RequiredContext = model.JSONBlob(normalizeJSONSlice(json.RawMessage(desired.RequiredContext)))
		changed = true
	}
	if string(normalizeJSONValue(existing.StarterFlows)) != string(normalizeJSONValue(desired.StarterFlows)) {
		existing.StarterFlows = model.JSONBlob(normalizeJSONValue(desired.StarterFlows))
		changed = true
	}
	if strings.TrimSpace(existing.ApprovalMode) != strings.TrimSpace(desired.ApprovalMode) {
		existing.ApprovalMode = desired.ApprovalMode
		changed = true
	}
	if strings.TrimSpace(existing.DefaultInvocationMode) != strings.TrimSpace(desired.DefaultInvocationMode) {
		existing.DefaultInvocationMode = desired.DefaultInvocationMode
		changed = true
	}
	if normalizeTokenBudget(existing.MonthlyTokenBudget) != normalizeTokenBudget(desired.MonthlyTokenBudget) {
		existing.MonthlyTokenBudget = normalizeTokenBudget(desired.MonthlyTokenBudget)
		changed = true
	}
	if existing.IsEnabled != desired.IsEnabled {
		existing.IsEnabled = desired.IsEnabled
		changed = true
	}
	if changed {
		return s.agentTemplateRepo.Update(ctx, existing)
	}
	return nil
}

func equalAgentSkillRefs(a, b model.AgentSkillRefs) bool {
	left := a.Normalize()
	right := b.Normalize()
	if len(left) != len(right) {
		return false
	}
	for idx := range left {
		if stringOrDefault(left[idx].SkillID, "") != stringOrDefault(right[idx].SkillID, "") {
			return false
		}
		if strings.TrimSpace(left[idx].Key) != strings.TrimSpace(right[idx].Key) {
			return false
		}
		if stringOrDefault(left[idx].VersionKey, "") != stringOrDefault(right[idx].VersionKey, "") {
			return false
		}
		if string(normalizeExecutionConfigJSON(left[idx].Config)) != string(normalizeExecutionConfigJSON(right[idx].Config)) {
			return false
		}
	}
	return true
}

func mustJSONValue(value any) json.RawMessage {
	payload, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage("null")
	}
	return payload
}

func normalizeJSONValue(raw model.JSONBlob) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return json.RawMessage("[]")
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return json.RawMessage("[]")
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage("[]")
	}
	return payload
}

func (s *AgentService) ListAgentTemplates(ctx context.Context, workspaceID string) ([]model.AgentTemplate, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if err := s.EnsureSystemTemplates(ctx); err != nil {
		return nil, err
	}
	if s.agentTemplateRepo == nil {
		return []model.AgentTemplate{}, nil
	}
	return s.agentTemplateRepo.ListVisible(ctx, workspaceID)
}

func (s *AgentService) GetAgentTemplate(ctx context.Context, workspaceID, templateID string) (*model.AgentTemplate, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if strings.TrimSpace(templateID) == "" {
		return nil, fmt.Errorf("template_id is required")
	}
	if err := s.EnsureSystemTemplates(ctx); err != nil {
		return nil, err
	}
	if s.agentTemplateRepo == nil {
		return nil, fmt.Errorf("agent template repository is not configured")
	}
	template, err := s.agentTemplateRepo.GetByID(ctx, workspaceID, templateID)
	if err != nil {
		return nil, err
	}
	if template == nil {
		return nil, fmt.Errorf("agent template not found")
	}
	return template, nil
}

func (s *AgentService) CreateAgentFromTemplate(ctx context.Context, workspaceID, templateID string, req model.CreateAgentFromTemplateRequest, actorID string) (*model.CreateAgentFromTemplateResponse, error) {
	template, err := s.GetAgentTemplate(ctx, workspaceID, templateID)
	if err != nil {
		return nil, err
	}

	createReq := materializeCreateAgentRequestFromTemplate(workspaceID, template, req)
	agent, err := s.createCustomAgent(ctx, createReq, actorID, template)
	if err != nil {
		return nil, err
	}

	var flow *model.AutomationRule
	if req.CreateFlow {
		flow, err = s.createStarterFlowForTemplate(ctx, workspaceID, template, agent, req)
		if err != nil {
			if deleteErr := s.agentRepo.Delete(ctx, workspaceID, agent.ID); deleteErr != nil {
				slog.WarnContext(ctx, "failed to clean up template-created agent after starter flow error",
					"workspace_id", workspaceID,
					"agent_id", agent.ID,
					"template_id", template.ID,
					"error", deleteErr,
				)
			}
			return nil, err
		}
	}

	return &model.CreateAgentFromTemplateResponse{
		Agent: agent,
		Flow:  flow,
	}, nil
}

func materializeCreateAgentRequestFromTemplate(workspaceID string, template *model.AgentTemplate, req model.CreateAgentFromTemplateRequest) model.CreateAgentRequest {
	name := strings.TrimSpace(stringOrDefault(req.Name, ""))
	if name == "" && template != nil {
		name = strings.TrimSpace(template.Name)
	}
	role := ""
	runtimeKind := ""
	systemPrompt := (*string)(nil)
	planningNotes := (*string)(nil)
	monthlyTokenBudget := (*int)(nil)
	allowedTools := json.RawMessage(mustJSONStringSlice(nil))
	allowedCommands := json.RawMessage(mustJSONStringSlice(nil))
	allowedTargets := json.RawMessage(mustJSONStringSlice([]string{"task"}))
	approvalMode := "never"
	defaultInvocationMode := model.InvocationModeAutonomous
	skills := model.AgentSkillRefs{}
	executionConfig := normalizeExecutionConfigJSON(nil)

	if template != nil {
		role = strings.TrimSpace(template.DefaultRole)
		runtimeKind = strings.TrimSpace(template.RuntimeKind)
		systemPrompt = trimPtr(template.SystemPrompt)
		planningNotes = trimPtr(template.PlanningNotes)
		monthlyTokenBudget = normalizeTokenBudget(template.MonthlyTokenBudget)
		allowedTools = json.RawMessage(normalizeAllowedToolsJSON(json.RawMessage(template.AllowedTools)))
		allowedCommands = json.RawMessage(normalizeJSONSlice(json.RawMessage(template.AllowedCommands)))
		allowedTargets = json.RawMessage(normalizeJSONSlice(json.RawMessage(template.AllowedTargets)))
		approvalMode = strings.TrimSpace(template.ApprovalMode)
		if approvalMode == "" {
			approvalMode = "never"
		}
		if strings.TrimSpace(template.DefaultInvocationMode) != "" {
			defaultInvocationMode = strings.TrimSpace(template.DefaultInvocationMode)
		}
		skills = template.Skills.Normalize()
		executionConfig = normalizeExecutionConfigJSON(template.ExecutionConfig)
	}
	if role == "" {
		role = "Custom Agent"
	}
	if runtimeKind == "" {
		runtimeKind = "opencode"
	}

	createReq := model.CreateAgentRequest{
		WorkspaceID:           workspaceID,
		Name:                  name,
		Role:                  role,
		RuntimeKind:           &runtimeKind,
		Skills:                skills,
		TriggerMode:           strPtr("manual"),
		ExecutionConfig:       json.RawMessage(executionConfig),
		SystemPrompt:          systemPrompt,
		PlanningNotes:         planningNotes,
		MonthlyTokenBudget:    monthlyTokenBudget,
		TeamID:                trimPtr(req.TeamID),
		AllowedTools:          allowedTools,
		AllowedCommands:       allowedCommands,
		AllowedTargets:        allowedTargets,
		ApprovalMode:          &approvalMode,
		DefaultInvocationMode: &defaultInvocationMode,
	}
	if req.Overrides != nil {
		if req.Overrides.IconKey != nil {
			createReq.IconKey = trimPtr(req.Overrides.IconKey)
		}
		if req.Overrides.Role != nil {
			roleValue := strings.TrimSpace(*req.Overrides.Role)
			if roleValue != "" {
				createReq.Role = roleValue
			}
		}
		if req.Overrides.RuntimeKind != nil {
			createReq.RuntimeKind = trimPtr(req.Overrides.RuntimeKind)
		}
		if req.Overrides.Skills != nil {
			normalized := req.Overrides.Skills.Normalize()
			createReq.Skills = normalized
		}
		if req.Overrides.Provider != nil {
			createReq.Provider = trimPtr(req.Overrides.Provider)
		}
		if req.Overrides.Model != nil {
			createReq.Model = trimPtr(req.Overrides.Model)
		}
		if req.Overrides.MonthlyTokenBudget != nil {
			createReq.MonthlyTokenBudget = normalizeTokenBudget(req.Overrides.MonthlyTokenBudget)
		}
		if req.Overrides.ExecutionConfig != nil {
			createReq.ExecutionConfig = json.RawMessage(normalizeExecutionConfigJSON(req.Overrides.ExecutionConfig))
		}
		if req.Overrides.SystemPrompt != nil {
			createReq.SystemPrompt = trimPtr(req.Overrides.SystemPrompt)
		}
		if req.Overrides.PlanningNotes != nil {
			createReq.PlanningNotes = trimPtr(req.Overrides.PlanningNotes)
		}
		if req.Overrides.AllowedTools != nil {
			createReq.AllowedTools = json.RawMessage(normalizeAllowedToolsJSON(json.RawMessage(req.Overrides.AllowedTools)))
		}
		if req.Overrides.AllowedCommands != nil {
			createReq.AllowedCommands = json.RawMessage(normalizeJSONSlice(json.RawMessage(req.Overrides.AllowedCommands)))
		}
		if req.Overrides.AllowedTargets != nil {
			createReq.AllowedTargets = json.RawMessage(normalizeJSONSlice(json.RawMessage(req.Overrides.AllowedTargets)))
		}
		if req.Overrides.ApprovalMode != nil {
			createReq.ApprovalMode = trimPtr(req.Overrides.ApprovalMode)
		}
		if req.Overrides.MaxConcurrentRuns != nil {
			createReq.MaxConcurrentRuns = req.Overrides.MaxConcurrentRuns
		}
		if req.Overrides.DefaultInvocationMode != nil {
			createReq.DefaultInvocationMode = trimPtr(req.Overrides.DefaultInvocationMode)
		}
	}
	if template != nil && strings.TrimSpace(template.Key) == model.AgentTemplateTypeCompetitiveIntel && req.CreateFlow {
		if input, err := competitiveIntelInputFromTemplateFlow(req.Flow); err == nil {
			createReq.SystemPrompt = renderCompetitiveIntelSystemPrompt(createReq.SystemPrompt, input)
		}
	}
	if template != nil && strings.TrimSpace(template.Key) == model.AgentTemplateTypeDependencyAuditor && req.CreateFlow {
		if input, err := dependencyAuditorInputFromTemplateFlow(req.Flow); err == nil {
			createReq.SystemPrompt = renderDependencyAuditorSystemPrompt(createReq.SystemPrompt, input)
		}
	}
	if template != nil && strings.TrimSpace(template.Key) == model.AgentTemplateTypeSecurityTriage && req.CreateFlow {
		if input, err := securityTriageInputFromTemplateFlow(req.Flow); err == nil {
			createReq.SystemPrompt = renderSecurityTriageSystemPrompt(createReq.SystemPrompt, input)
		}
	}
	return createReq
}

func (s *AgentService) createStarterFlowForTemplate(ctx context.Context, workspaceID string, template *model.AgentTemplate, agent *model.Agent, req model.CreateAgentFromTemplateRequest) (*model.AutomationRule, error) {
	if s == nil || s.ruleEngine == nil {
		return nil, fmt.Errorf("automation rule engine is not configured")
	}
	switch strings.TrimSpace(template.Key) {
	case model.AgentTemplateTypeReleaseNotes:
		return s.createReleaseNotesStarterFlow(ctx, workspaceID, agent, req.Flow)
	case model.AgentTemplateTypeCompetitiveIntel:
		return s.createCompetitiveIntelStarterFlow(ctx, workspaceID, agent, req.Flow)
	case model.AgentTemplateTypeDependencyAuditor:
		return s.createDependencyAuditorStarterFlow(ctx, workspaceID, agent, req.Flow)
	case model.AgentTemplateTypeSecurityTriage:
		return s.createSecurityTriageStarterFlow(ctx, workspaceID, agent, req.Flow)
	default:
		return nil, fmt.Errorf("starter flow is not supported for template %q", strings.TrimSpace(template.Key))
	}
}

func (s *AgentService) createReleaseNotesStarterFlow(ctx context.Context, workspaceID string, agent *model.Agent, flow *model.CreateAgentFromTemplateFlow) (*model.AutomationRule, error) {
	if flow == nil {
		return nil, fmt.Errorf("flow configuration is required when create_flow is true")
	}
	repoFullName := strings.TrimSpace(flow.RepoFullName)
	repositoryID := strings.TrimSpace(flow.RepositoryID)
	if repoFullName == "" && repositoryID != "" {
		if s.gitService == nil {
			return nil, fmt.Errorf("git service is not configured")
		}
		repo, err := s.gitService.GetRepositoryByID(ctx, workspaceID, repositoryID)
		if err != nil {
			return nil, err
		}
		if repo == nil {
			return nil, fmt.Errorf("repository not found")
		}
		repoFullName = strings.TrimSpace(repo.FullName)
	}
	if repoFullName == "" {
		return nil, fmt.Errorf("flow.repo_full_name or flow.repository_id is required")
	}
	if strings.TrimSpace(flow.SpaceID) == "" {
		return nil, fmt.Errorf("flow.space_id is required")
	}

	releaseKinds := normalizedReleaseKinds(flow.ReleaseKinds)
	triggerConfig, err := json.Marshal(model.TriggerConfigGitHubReleasePublished{
		RepoFullName:      repoFullName,
		TagPattern:        strings.TrimSpace(flow.TagPattern),
		ReleaseKinds:      releaseKinds,
		IncludePrerelease: flow.IncludePrerelease,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal release trigger config: %w", err)
	}

	targetType := ""
	if repositoryID != "" {
		targetType = "repository"
	}
	actionConfig, err := json.Marshal(model.ActionConfigRunAgent{
		TargetType: targetType,
		TargetID:   repositoryID,
		AgentID:    agent.ID,
		Output: &model.ActionConfigRunAgentOutput{
			Type:         "docs_document",
			SpaceID:      strings.TrimSpace(flow.SpaceID),
			CollectionID: trimPtr(flow.CollectionID),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal release action config: %w", err)
	}

	name := fmt.Sprintf("%s for %s releases", strings.TrimSpace(agent.Name), repoFullName)
	description := fmt.Sprintf("Runs %s when GitHub publishes matching releases for %s and writes release notes to Docs.", strings.TrimSpace(agent.Name), repoFullName)
	rule, err := s.ruleEngine.CreateRule(ctx, workspaceID, model.CreateAutomationRuleRequest{
		WorkspaceID:   workspaceID,
		Name:          name,
		Description:   &description,
		TriggerType:   model.TriggerGitHubReleasePub,
		TriggerConfig: triggerConfig,
		ActionType:    model.ActionStartAgentRun,
		ActionConfig:  actionConfig,
	})
	if err != nil {
		return nil, err
	}
	return rule, nil
}

type competitiveIntelStarterFlowInput struct {
	TargetCompany      string   `json:"target_company"`
	TargetDomain       string   `json:"target_domain,omitempty"`
	Competitors        []string `json:"competitors,omitempty"`
	SchedulePreset     string   `json:"schedule_preset,omitempty"`
	LookbackDays       int      `json:"lookback_days,omitempty"`
	DestinationTeamID  string   `json:"destination_team_id"`
	DestinationStateID string   `json:"destination_state_id,omitempty"`
}

func (s *AgentService) createCompetitiveIntelStarterFlow(ctx context.Context, workspaceID string, agent *model.Agent, flow *model.CreateAgentFromTemplateFlow) (*model.AutomationRule, error) {
	input, err := competitiveIntelInputFromTemplateFlow(flow)
	if err != nil {
		return nil, err
	}

	triggerConfig, err := json.Marshal(model.TriggerConfigCron{
		Preset: input.SchedulePreset,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal competitive intel trigger config: %w", err)
	}

	actionConfig, err := json.Marshal(model.ActionConfigRunAgent{
		TargetType: "workspace",
		TargetID:   workspaceID,
		AgentID:    agent.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal competitive intel action config: %w", err)
	}

	name := fmt.Sprintf("%s %s digest for %s", strings.TrimSpace(agent.Name), input.SchedulePreset, input.TargetCompany)
	description := fmt.Sprintf("Runs %s on a %s schedule to research competitor updates for %s and create one marketing task.", strings.TrimSpace(agent.Name), input.SchedulePreset, input.TargetCompany)
	rule, err := s.ruleEngine.CreateRule(ctx, workspaceID, model.CreateAutomationRuleRequest{
		WorkspaceID:   workspaceID,
		Name:          name,
		Description:   &description,
		TriggerType:   model.TriggerCron,
		TriggerConfig: triggerConfig,
		ActionType:    model.ActionStartAgentRun,
		ActionConfig:  actionConfig,
	})
	if err != nil {
		return nil, err
	}
	return rule, nil
}

func competitiveIntelInputFromTemplateFlow(flow *model.CreateAgentFromTemplateFlow) (competitiveIntelStarterFlowInput, error) {
	if flow == nil {
		return competitiveIntelStarterFlowInput{}, fmt.Errorf("flow configuration is required when create_flow is true")
	}
	flowKey := strings.TrimSpace(flow.FlowKey)
	if flowKey != "" && flowKey != "competitors_changelog_scheduled" && flowKey != "competitive_intel_scheduled" {
		return competitiveIntelStarterFlowInput{}, fmt.Errorf("unsupported competitors changelog flow_key %q", flowKey)
	}
	if len(flow.FlowInput) == 0 || strings.TrimSpace(string(flow.FlowInput)) == "" || strings.TrimSpace(string(flow.FlowInput)) == "null" {
		return competitiveIntelStarterFlowInput{}, fmt.Errorf("flow.flow_input is required for competitors changelog starter flow")
	}

	var input competitiveIntelStarterFlowInput
	if err := json.Unmarshal(flow.FlowInput, &input); err != nil {
		return competitiveIntelStarterFlowInput{}, fmt.Errorf("parse competitive intel flow_input: %w", err)
	}
	input.TargetCompany = strings.TrimSpace(input.TargetCompany)
	input.TargetDomain = strings.TrimSpace(input.TargetDomain)
	input.SchedulePreset = strings.ToLower(strings.TrimSpace(input.SchedulePreset))
	input.DestinationTeamID = strings.TrimSpace(input.DestinationTeamID)
	input.DestinationStateID = strings.TrimSpace(input.DestinationStateID)
	input.Competitors = normalizeTemplateStringList(input.Competitors)
	if input.TargetCompany == "" {
		return competitiveIntelStarterFlowInput{}, fmt.Errorf("flow.flow_input.target_company is required")
	}
	if input.DestinationTeamID == "" {
		return competitiveIntelStarterFlowInput{}, fmt.Errorf("flow.flow_input.destination_team_id is required")
	}
	if input.SchedulePreset != "daily" && input.SchedulePreset != "weekly" {
		return competitiveIntelStarterFlowInput{}, fmt.Errorf("flow.flow_input.schedule_preset must be daily or weekly")
	}
	if input.LookbackDays < 1 || input.LookbackDays > 30 {
		return competitiveIntelStarterFlowInput{}, fmt.Errorf("flow.flow_input.lookback_days must be between 1 and 30")
	}
	return input, nil
}

func renderCompetitiveIntelSystemPrompt(base *string, input competitiveIntelStarterFlowInput) *string {
	prompt := strings.TrimSpace(derefString(base))
	if prompt == "" {
		fallback, err := competitiveIntelSystemPromptSection(input)
		if err != nil || strings.TrimSpace(fallback) == "" {
			return base
		}
		return &fallback
	}
	rendered, err := renderCompetitiveIntelPromptVariables(prompt, input)
	if err != nil || strings.TrimSpace(rendered) == "" {
		return base
	}
	return &rendered
}

func renderCompetitiveIntelPromptVariables(prompt string, input competitiveIntelStarterFlowInput) (string, error) {
	payload, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal competitive intel run configuration: %w", err)
	}
	values := competitiveIntelPromptValues(input, string(payload))
	replacerArgs := make([]string, 0, len(values)*2)
	for key, value := range values {
		replacerArgs = append(replacerArgs, "{{"+key+"}}", value)
	}
	return strings.TrimSpace(strings.NewReplacer(replacerArgs...).Replace(prompt)), nil
}

func competitiveIntelPromptValues(input competitiveIntelStarterFlowInput, rawJSON string) map[string]string {
	competitors := "none configured; discover competitors during this run"
	if len(input.Competitors) > 0 {
		competitors = strings.Join(input.Competitors, ", ")
	}
	destinationState := "not configured; use the team's default stage"
	if input.DestinationStateID != "" {
		destinationState = input.DestinationStateID
	}
	targetDomain := "not configured"
	if input.TargetDomain != "" {
		targetDomain = input.TargetDomain
	}
	return map[string]string{
		"target_company":         input.TargetCompany,
		"target_domain":          targetDomain,
		"competitors":            competitors,
		"lookback_days":          fmt.Sprintf("%d", input.LookbackDays),
		"destination_team_id":    input.DestinationTeamID,
		"destination_state_id":   destinationState,
		"schedule_preset":        input.SchedulePreset,
		"raw_configuration_json": "```json\n" + rawJSON + "\n```",
	}
}

func competitiveIntelSystemPromptSection(input competitiveIntelStarterFlowInput) (string, error) {
	payload, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal competitive intel run configuration: %w", err)
	}
	competitors := "none configured; discover competitors during this run"
	if len(input.Competitors) > 0 {
		competitors = strings.Join(input.Competitors, ", ")
	}
	destinationState := "not configured; use the team's default stage"
	if input.DestinationStateID != "" {
		destinationState = input.DestinationStateID
	}
	targetDomain := "not configured"
	if input.TargetDomain != "" {
		targetDomain = input.TargetDomain
	}

	context := fmt.Sprintf(`Competitive intelligence configuration:

These values were configured when this custom agent was created. Treat them as already resolved and authoritative. Do not plan or perform discovery of configuration variables, workspace context, teams, stages, cadence, or lookback settings.

- target_company: %s
- target_domain: %s
- competitors: %s
- lookback_days: %d
- destination_team_id: %s
- destination_state_id: %s
- schedule_preset: %s (informational; the automation rule already handled cadence)

Use the destination IDs directly when calling create_task. Only discover competitors if the configured competitors list is empty.
Use web_search_exa to find candidate official update sources, fetch_url to verify exact source pages, and crawl_url on official or docs hosts when search results are thin.

Raw configuration:

`, input.TargetCompany, targetDomain, competitors, input.LookbackDays, input.DestinationTeamID, destinationState, input.SchedulePreset)
	return strings.TrimSpace(context + "```json\n" + string(payload) + "\n```"), nil
}

type dependencyAuditorStarterFlowInput struct {
	Ecosystems         []string `json:"ecosystems"`
	IncludeIndirect    bool     `json:"include_indirect"`
	SchedulePreset     string   `json:"schedule_preset,omitempty"`
	DestinationTeamID  string   `json:"destination_team_id"`
	DestinationStateID string   `json:"destination_state_id,omitempty"`
	MaxTasks           int      `json:"max_tasks,omitempty"`
}

func (s *AgentService) createDependencyAuditorStarterFlow(ctx context.Context, workspaceID string, agent *model.Agent, flow *model.CreateAgentFromTemplateFlow) (*model.AutomationRule, error) {
	input, err := dependencyAuditorInputFromTemplateFlow(flow)
	if err != nil {
		return nil, err
	}

	repositoryID := strings.TrimSpace(flow.RepositoryID)
	if repositoryID == "" {
		return nil, fmt.Errorf("flow.repository_id is required for dependency auditor starter flow")
	}
	repoLabel := repositoryID
	if repoFullName := strings.TrimSpace(flow.RepoFullName); repoFullName != "" {
		repoLabel = repoFullName
	} else if s.gitService != nil {
		repo, err := s.gitService.GetRepositoryByID(ctx, workspaceID, repositoryID)
		if err != nil {
			return nil, err
		}
		if repo == nil {
			return nil, fmt.Errorf("repository not found")
		}
		if strings.TrimSpace(repo.FullName) != "" {
			repoLabel = strings.TrimSpace(repo.FullName)
		}
	}

	triggerConfig, err := json.Marshal(model.TriggerConfigCron{
		Preset: input.SchedulePreset,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal dependency auditor trigger config: %w", err)
	}

	actionConfig, err := json.Marshal(model.ActionConfigRunAgent{
		TargetType: "repository",
		TargetID:   repositoryID,
		AgentID:    agent.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal dependency auditor action config: %w", err)
	}

	name := fmt.Sprintf("%s %s audit for %s", strings.TrimSpace(agent.Name), input.SchedulePreset, repoLabel)
	description := fmt.Sprintf("Runs %s on a %s schedule to audit direct dependencies in %s and create verified update tasks.", strings.TrimSpace(agent.Name), input.SchedulePreset, repoLabel)
	rule, err := s.ruleEngine.CreateRule(ctx, workspaceID, model.CreateAutomationRuleRequest{
		WorkspaceID:   workspaceID,
		Name:          name,
		Description:   &description,
		TriggerType:   model.TriggerCron,
		TriggerConfig: triggerConfig,
		ActionType:    model.ActionStartAgentRun,
		ActionConfig:  actionConfig,
	})
	if err != nil {
		return nil, err
	}
	return rule, nil
}

func dependencyAuditorInputFromTemplateFlow(flow *model.CreateAgentFromTemplateFlow) (dependencyAuditorStarterFlowInput, error) {
	if flow == nil {
		return dependencyAuditorStarterFlowInput{}, fmt.Errorf("flow configuration is required when create_flow is true")
	}
	flowKey := strings.TrimSpace(flow.FlowKey)
	if flowKey != "" && flowKey != "dependency_audit_cron" {
		return dependencyAuditorStarterFlowInput{}, fmt.Errorf("unsupported dependency auditor flow_key %q", flowKey)
	}
	if len(flow.FlowInput) == 0 || strings.TrimSpace(string(flow.FlowInput)) == "" || strings.TrimSpace(string(flow.FlowInput)) == "null" {
		return dependencyAuditorStarterFlowInput{}, fmt.Errorf("flow.flow_input is required for dependency auditor starter flow")
	}

	var input dependencyAuditorStarterFlowInput
	if err := json.Unmarshal(flow.FlowInput, &input); err != nil {
		return dependencyAuditorStarterFlowInput{}, fmt.Errorf("parse dependency auditor flow_input: %w", err)
	}
	for _, ecosystem := range input.Ecosystems {
		normalized := strings.ToLower(strings.TrimSpace(ecosystem))
		if !isDependencyAuditorEcosystem(normalized) {
			return dependencyAuditorStarterFlowInput{}, fmt.Errorf("flow.flow_input.ecosystems must only include go, rust, python, node, or java")
		}
	}
	input.Ecosystems = normalizeDependencyAuditorEcosystems(input.Ecosystems)
	input.SchedulePreset = strings.ToLower(strings.TrimSpace(input.SchedulePreset))
	input.DestinationTeamID = strings.TrimSpace(input.DestinationTeamID)
	input.DestinationStateID = strings.TrimSpace(input.DestinationStateID)
	if len(input.Ecosystems) == 0 {
		return dependencyAuditorStarterFlowInput{}, fmt.Errorf("flow.flow_input.ecosystems requires at least one of go, rust, python, node, or java")
	}
	if input.DestinationTeamID == "" {
		return dependencyAuditorStarterFlowInput{}, fmt.Errorf("flow.flow_input.destination_team_id is required")
	}
	if input.SchedulePreset != "daily" && input.SchedulePreset != "weekly" {
		return dependencyAuditorStarterFlowInput{}, fmt.Errorf("flow.flow_input.schedule_preset must be daily or weekly")
	}
	if input.MaxTasks < 1 || input.MaxTasks > 100 {
		return dependencyAuditorStarterFlowInput{}, fmt.Errorf("flow.flow_input.max_tasks must be between 1 and 100")
	}
	return input, nil
}

func normalizeDependencyAuditorEcosystems(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		normalized := strings.ToLower(strings.TrimSpace(value))
		if !isDependencyAuditorEcosystem(normalized) {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out
}

func isDependencyAuditorEcosystem(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "go", "rust", "python", "node", "java":
		return true
	default:
		return false
	}
}

func renderDependencyAuditorSystemPrompt(base *string, input dependencyAuditorStarterFlowInput) *string {
	prompt := strings.TrimSpace(derefString(base))
	if prompt == "" {
		fallback, err := dependencyAuditorSystemPromptSection(input)
		if err != nil || strings.TrimSpace(fallback) == "" {
			return base
		}
		return &fallback
	}
	rendered, err := renderDependencyAuditorPromptVariables(prompt, input)
	if err != nil || strings.TrimSpace(rendered) == "" {
		return base
	}
	return &rendered
}

func renderDependencyAuditorPromptVariables(prompt string, input dependencyAuditorStarterFlowInput) (string, error) {
	payload, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal dependency auditor run configuration: %w", err)
	}
	values := dependencyAuditorPromptValues(input, string(payload))
	replacerArgs := make([]string, 0, len(values)*2)
	for key, value := range values {
		replacerArgs = append(replacerArgs, "{{"+key+"}}", value)
	}
	return strings.TrimSpace(strings.NewReplacer(replacerArgs...).Replace(prompt)), nil
}

func dependencyAuditorPromptValues(input dependencyAuditorStarterFlowInput, rawJSON string) map[string]string {
	destinationState := "not configured; use the team's default stage"
	if input.DestinationStateID != "" {
		destinationState = input.DestinationStateID
	}
	return map[string]string{
		"ecosystems":             strings.Join(input.Ecosystems, ", "),
		"include_indirect":       fmt.Sprintf("%t", input.IncludeIndirect),
		"destination_team_id":    input.DestinationTeamID,
		"destination_state_id":   destinationState,
		"schedule_preset":        input.SchedulePreset,
		"max_tasks":              fmt.Sprintf("%d", input.MaxTasks),
		"raw_configuration_json": "```json\n" + rawJSON + "\n```",
	}
}

func dependencyAuditorSystemPromptSection(input dependencyAuditorStarterFlowInput) (string, error) {
	payload, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal dependency auditor run configuration: %w", err)
	}
	destinationState := "not configured; use the team's default stage"
	if input.DestinationStateID != "" {
		destinationState = input.DestinationStateID
	}

	context := fmt.Sprintf(`Dependency auditor configuration:

These values were configured when this custom agent was created. Treat them as already resolved and authoritative. Do not plan or perform discovery of configuration variables, workspace context, teams, stages, cadence, or repository selection.

- ecosystems: %s
- include_indirect: %t
- destination_team_id: %s
- destination_state_id: %s
- max_tasks: %d
- schedule_preset: %s (informational; the automation rule already handled cadence)

Use the destination IDs directly when calling create_task. Repositories may contain more than one language ecosystem; scan every selected ecosystem in the same run and create at most max_tasks tasks.

Raw configuration:

`, strings.Join(input.Ecosystems, ", "), input.IncludeIndirect, input.DestinationTeamID, destinationState, input.MaxTasks, input.SchedulePreset)
	return strings.TrimSpace(context + "```json\n" + string(payload) + "\n```"), nil
}

type securityTriageStarterFlowInput struct {
	Scanners           []string `json:"scanners"`
	SeverityThreshold  string   `json:"severity_threshold,omitempty"`
	IncludeLowInfo     bool     `json:"include_low_info"`
	SchedulePreset     string   `json:"schedule_preset,omitempty"`
	DestinationTeamID  string   `json:"destination_team_id"`
	DestinationStateID string   `json:"destination_state_id,omitempty"`
	MaxTasks           int      `json:"max_tasks,omitempty"`
}

func (s *AgentService) createSecurityTriageStarterFlow(ctx context.Context, workspaceID string, agent *model.Agent, flow *model.CreateAgentFromTemplateFlow) (*model.AutomationRule, error) {
	input, err := securityTriageInputFromTemplateFlow(flow)
	if err != nil {
		return nil, err
	}

	repositoryID := strings.TrimSpace(flow.RepositoryID)
	if repositoryID == "" {
		return nil, fmt.Errorf("flow.repository_id is required for security triage starter flow")
	}
	repoLabel := repositoryID
	if repoFullName := strings.TrimSpace(flow.RepoFullName); repoFullName != "" {
		repoLabel = repoFullName
	} else if s.gitService != nil {
		repo, err := s.gitService.GetRepositoryByID(ctx, workspaceID, repositoryID)
		if err != nil {
			return nil, err
		}
		if repo == nil {
			return nil, fmt.Errorf("repository not found")
		}
		if strings.TrimSpace(repo.FullName) != "" {
			repoLabel = strings.TrimSpace(repo.FullName)
		}
	}

	triggerConfig, err := json.Marshal(model.TriggerConfigCron{
		Preset: input.SchedulePreset,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal security triage trigger config: %w", err)
	}

	actionConfig, err := json.Marshal(model.ActionConfigRunAgent{
		TargetType: "repository",
		TargetID:   repositoryID,
		AgentID:    agent.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal security triage action config: %w", err)
	}

	name := fmt.Sprintf("%s %s scan for %s", strings.TrimSpace(agent.Name), input.SchedulePreset, repoLabel)
	description := fmt.Sprintf("Runs %s on a %s schedule to triage security scanner findings in %s and create applicable remediation tasks.", strings.TrimSpace(agent.Name), input.SchedulePreset, repoLabel)
	rule, err := s.ruleEngine.CreateRule(ctx, workspaceID, model.CreateAutomationRuleRequest{
		WorkspaceID:   workspaceID,
		Name:          name,
		Description:   &description,
		TriggerType:   model.TriggerCron,
		TriggerConfig: triggerConfig,
		ActionType:    model.ActionStartAgentRun,
		ActionConfig:  actionConfig,
	})
	if err != nil {
		return nil, err
	}
	return rule, nil
}

func securityTriageInputFromTemplateFlow(flow *model.CreateAgentFromTemplateFlow) (securityTriageStarterFlowInput, error) {
	if flow == nil {
		return securityTriageStarterFlowInput{}, fmt.Errorf("flow configuration is required when create_flow is true")
	}
	flowKey := strings.TrimSpace(flow.FlowKey)
	if flowKey != "" && flowKey != "engineering_security_triage_cron" {
		return securityTriageStarterFlowInput{}, fmt.Errorf("unsupported security triage flow_key %q", flowKey)
	}
	if len(flow.FlowInput) == 0 || strings.TrimSpace(string(flow.FlowInput)) == "" || strings.TrimSpace(string(flow.FlowInput)) == "null" {
		return securityTriageStarterFlowInput{}, fmt.Errorf("flow.flow_input is required for security triage starter flow")
	}

	var input securityTriageStarterFlowInput
	if err := json.Unmarshal(flow.FlowInput, &input); err != nil {
		return securityTriageStarterFlowInput{}, fmt.Errorf("parse security triage flow_input: %w", err)
	}
	for _, scanner := range input.Scanners {
		normalized := strings.ToLower(strings.TrimSpace(scanner))
		if !isSecurityTriageScanner(normalized) {
			return securityTriageStarterFlowInput{}, fmt.Errorf("flow.flow_input.scanners must only include semgrep, trivy, or gitleaks")
		}
	}
	input.Scanners = normalizeSecurityTriageScanners(input.Scanners)
	input.SeverityThreshold = strings.ToLower(strings.TrimSpace(input.SeverityThreshold))
	input.SchedulePreset = strings.ToLower(strings.TrimSpace(input.SchedulePreset))
	input.DestinationTeamID = strings.TrimSpace(input.DestinationTeamID)
	input.DestinationStateID = strings.TrimSpace(input.DestinationStateID)
	if len(input.Scanners) == 0 {
		return securityTriageStarterFlowInput{}, fmt.Errorf("flow.flow_input.scanners requires at least one of semgrep, trivy, or gitleaks")
	}
	if !isSecurityTriageSeverity(input.SeverityThreshold) {
		return securityTriageStarterFlowInput{}, fmt.Errorf("flow.flow_input.severity_threshold must be critical, high, or medium")
	}
	if input.DestinationTeamID == "" {
		return securityTriageStarterFlowInput{}, fmt.Errorf("flow.flow_input.destination_team_id is required")
	}
	if input.SchedulePreset != "daily" && input.SchedulePreset != "weekly" {
		return securityTriageStarterFlowInput{}, fmt.Errorf("flow.flow_input.schedule_preset must be daily or weekly")
	}
	if input.MaxTasks < 1 || input.MaxTasks > 100 {
		return securityTriageStarterFlowInput{}, fmt.Errorf("flow.flow_input.max_tasks must be between 1 and 100")
	}
	return input, nil
}

func normalizeSecurityTriageScanners(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		normalized := strings.ToLower(strings.TrimSpace(value))
		if !isSecurityTriageScanner(normalized) {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out
}

func isSecurityTriageScanner(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "semgrep", "trivy", "gitleaks":
		return true
	default:
		return false
	}
}

func isSecurityTriageSeverity(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "critical", "high", "medium":
		return true
	default:
		return false
	}
}

func renderSecurityTriageSystemPrompt(base *string, input securityTriageStarterFlowInput) *string {
	prompt := strings.TrimSpace(derefString(base))
	if prompt == "" {
		fallback, err := securityTriageSystemPromptSection(input)
		if err != nil || strings.TrimSpace(fallback) == "" {
			return base
		}
		return &fallback
	}
	rendered, err := renderSecurityTriagePromptVariables(prompt, input)
	if err != nil || strings.TrimSpace(rendered) == "" {
		return base
	}
	return &rendered
}

func renderSecurityTriagePromptVariables(prompt string, input securityTriageStarterFlowInput) (string, error) {
	payload, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal security triage run configuration: %w", err)
	}
	values := securityTriagePromptValues(input, string(payload))
	replacerArgs := make([]string, 0, len(values)*2)
	for key, value := range values {
		replacerArgs = append(replacerArgs, "{{"+key+"}}", value)
	}
	return strings.TrimSpace(strings.NewReplacer(replacerArgs...).Replace(prompt)), nil
}

func securityTriagePromptValues(input securityTriageStarterFlowInput, rawJSON string) map[string]string {
	destinationState := "not configured; use the team's default stage"
	if input.DestinationStateID != "" {
		destinationState = input.DestinationStateID
	}
	return map[string]string{
		"scanners":               strings.Join(input.Scanners, ", "),
		"severity_threshold":     input.SeverityThreshold,
		"include_low_info":       fmt.Sprintf("%t", input.IncludeLowInfo),
		"destination_team_id":    input.DestinationTeamID,
		"destination_state_id":   destinationState,
		"schedule_preset":        input.SchedulePreset,
		"max_tasks":              fmt.Sprintf("%d", input.MaxTasks),
		"raw_configuration_json": "```json\n" + rawJSON + "\n```",
	}
}

func securityTriageSystemPromptSection(input securityTriageStarterFlowInput) (string, error) {
	payload, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal security triage run configuration: %w", err)
	}
	destinationState := "not configured; use the team's default stage"
	if input.DestinationStateID != "" {
		destinationState = input.DestinationStateID
	}

	context := fmt.Sprintf(`Sentinel security triage configuration:

These values were configured when this custom agent was created. Treat them as already resolved and authoritative. Do not plan or perform discovery of configuration variables, workspace context, teams, stages, cadence, or repository selection.

- scanners: %s
- severity_threshold: %s
- include_low_info: %t
- destination_team_id: %s
- destination_state_id: %s
- max_tasks: %d
- schedule_preset: %s (informational; the automation rule already handled cadence)

Before creating tasks, call ensure_task_label for a shared workspace label named "security". Then call list_tasks with the returned security label_id, open_only: true, detail_level: "compact", and limit: 100. Do not request full descriptions/comments for the first duplicate lookup. Do not filter existing-task lookup by destination_state_id; duplicates must be detected across every open workflow state. Use the existing open security tasks to avoid duplicates. Parse Sentinel markers such as <!-- sentinel:root_cause=... finding_ids=[...] --> from compact task excerpts yourself; do not expect structured marker fields. If a matching open task already exists, add a scan-update comment only when the current scan adds materially new evidence; otherwise leave it unchanged.

When creating a security task, put a single-line <!-- sentinel:root_cause=... finding_ids=[...] --> marker at the top of the task description, capped to roughly 300 characters. When adding a scan-update comment, put the scan-update marker at the start of the comment so compact excerpts preserve it.

Use scanner summary_only and pagination/filtering for large result sets. summary_only returns counts/groups only; detail_level: "index" returns compact finding rows; detail_level: "full" returns verbose scanner details for narrow follow-up only. Do not rerun a scanner only because the model-visible output was compacted; request the next page or a narrower category/package/CVE/path filter instead.

Use the destination IDs directly when calling create_task. Attach the security label ID with label_ids. Run only the configured scanners, triage findings in repository context, suppress false positives, group applicable findings by fix unit, and create at most max_tasks new tasks.

Raw configuration:

`, strings.Join(input.Scanners, ", "), input.SeverityThreshold, input.IncludeLowInfo, input.DestinationTeamID, destinationState, input.MaxTasks, input.SchedulePreset)
	return strings.TrimSpace(context + "```json\n" + string(payload) + "\n```"), nil
}

func normalizedReleaseKinds(values []string) []string {
	if len(values) == 0 {
		return []string{"minor"}
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		normalized := strings.ToLower(strings.TrimSpace(value))
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	if len(out) == 0 {
		return []string{"minor"}
	}
	return out
}

func normalizeTemplateStringList(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			continue
		}
		key := strings.ToLower(normalized)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, normalized)
	}
	return out
}
