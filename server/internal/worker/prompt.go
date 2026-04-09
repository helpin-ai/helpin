package worker

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

// BuildSystemPrompt assembles the system prompt from agent config, target context, and WORKFLOW.md.
func BuildSystemPrompt(agent *model.Agent, story *model.PMTask, epic *model.PMEpic, ticket *model.SupportConversation, planningStage, planningMethodology string, config *WorkflowConfig) string {
	var parts []string
	resolvedProfile := ResolveAgentProfile(agent)
	toolSet := make(map[string]bool, len(resolvedProfile.Tools))
	for _, toolName := range resolvedProfile.Tools {
		toolSet[toolName] = true
	}
	hasRepoAccess := hasRepoTools(resolvedProfile.Tools)
	hasFileMutationTools := toolSet["write_file"] || toolSet["edit_file"] || toolSet["apply_patch"]

	if agent != nil && agent.SystemPrompt != nil && strings.TrimSpace(*agent.SystemPrompt) != "" {
		parts = append(parts, strings.TrimSpace(*agent.SystemPrompt))
	} else {
		parts = append(parts, fmt.Sprintf("You are %s, an AI coding agent. You write clean, correct code and follow existing project conventions.", agent.Name))
	}

	// Story context.
	if story != nil {
		parts = append(parts, "\n## Current Task")
		parts = append(parts, fmt.Sprintf("**Story**: %s", story.Name))
		if story.Description != nil {
			if description := tiptap.RichTextToMarkdown(*story.Description); description != "" {
				parts = append(parts, "**Description**:\n"+description)
			}
		}
	}
	if epic != nil {
		parts = append(parts, "\n## Current Epic")
		parts = append(parts, fmt.Sprintf("**Epic**: %s", epic.Name))
		if epic.Description != nil {
			if description := tiptap.RichTextToMarkdown(*epic.Description); description != "" {
				parts = append(parts, "**Description**:\n"+description)
			}
		}
	}
	if ticket != nil {
		parts = append(parts, "\n## Current Support Conversation")
		parts = append(parts, fmt.Sprintf("**Subject**: %s", ticket.Subject))
		if ticket.CustomerName != nil && *ticket.CustomerName != "" {
			parts = append(parts, fmt.Sprintf("**Customer**: %s", *ticket.CustomerName))
		}
	}

	var skills []string
	if len(agent.Skills) > 0 {
		_ = json.Unmarshal(agent.Skills, &skills)
	}
	if len(skills) > 0 {
		parts = append(parts, "\n## Enabled Skills")
		parts = append(parts, "- "+strings.Join(skills, "\n- "))
	}

	// WORKFLOW.md extra prompt.
	if config != nil && config.ExtraPrompt != "" {
		parts = append(parts, "\n## Repository Guidelines")
		parts = append(parts, config.ExtraPrompt)
	}

	parts = append(parts, "\n## Rules")
	parts = append(parts, "- Work within the cloned repository only.")
	switch {
	case hasRepoAccess && hasFileMutationTools:
		parts = append(parts, "- Use the provided tools to read, write, and search files.")
	case hasRepoAccess:
		parts = append(parts, "- Use the provided tools to inspect the repository and search for relevant context. Keep repository interactions read-only.")
	}
	if (story != nil || epic != nil) && hasRepoAccess {
		parts = append(parts, "- Start by locating the relevant code with list_directory, ripgrep, search_files, or list_symbols before reading large files.")
		parts = append(parts, "- read_file now returns a bounded window by default; use offset_line to continue and use read_file_range for targeted spans.")
		if hasFileMutationTools {
			parts = append(parts, "- Prefer edit_file for focused in-place changes and apply_patch for coordinated multi-file edits.")
			parts = append(parts, "- Use write_file for new files or full rewrites only after you have read the current file state.")
			parts = append(parts, "- If an edit tool reports that a file changed or was not read first, re-read the file and retry with fresh context.")
		} else {
			parts = append(parts, "- This run is planning-only and read-only. Do not change code, create files, or alter git state.")
		}
		parts = append(parts, "- When available, keep a short working execution checklist with update_plan instead of repeating plan status in prose. Do not use update_plan as a substitute for publish_prd_draft, publish_task_plan, or publish_task_plan_doc.")
	}
	if story != nil && strings.TrimSpace(planningStage) != model.PlanningStageStoryPlanDoc {
		parts = append(parts, "- Run tests after making changes when possible.")
		parts = append(parts, "- Commit and push your changes when the task is complete.")
	}
	if story != nil && strings.TrimSpace(planningStage) == model.PlanningStageStoryPlanDoc {
		parts = append(parts, "- This is a planning-doc run, not an implementation run.")
		parts = append(parts, "- Draft or refine the canonical task planning document in chat first, then request approval.")
		parts = append(parts, "- After approval, stop. The platform will persist and link the approved task planning document.")
	}
	if ticket != nil {
		parts = append(parts, "- Customer-visible replies must be drafted for human approval before they are sent.")
	}
	parts = append(parts, "- Leave Helpin artifacts and summaries in a state a human can review.")

	return strings.Join(parts, "\n")
}

// BuildUserPrompt creates the initial user message for the run.
func BuildUserPrompt(
	agent *model.Agent,
	story *model.PMTask,
	epic *model.PMEpic,
	epicStories []model.PMTask,
	ticket *model.SupportConversation,
	ticketMessages []model.SupportMessage,
	checklist []model.PMChecklistItem,
	artifactContext *ArtifactContext,
	planningStage string,
	initialInstructions string,
) string {
	var sections []string
	var contextParts []string

	if agent != nil && agent.SystemPrompt != nil && strings.TrimSpace(*agent.SystemPrompt) != "" {
		sections = append(sections, strings.TrimSpace(*agent.SystemPrompt))
	}

	if story != nil {
		if strings.TrimSpace(planningStage) == model.PlanningStageStoryPlanDoc {
			contextParts = append(contextParts, fmt.Sprintf("Please draft or refine the canonical task planning document for task: **%s**", story.Name))
		} else {
			contextParts = append(contextParts, fmt.Sprintf("Please work on the story: **%s**", story.Name))
		}
		if story.Description != nil {
			if description := tiptap.RichTextToMarkdown(*story.Description); description != "" {
				contextParts = append(contextParts, "\nDescription:\n"+description)
			}
		}
	}
	if epic != nil {
		contextParts = append(contextParts, fmt.Sprintf("Please work on epic: **%s**", epic.Name))
		if epic.Description != nil {
			if description := tiptap.RichTextToMarkdown(*epic.Description); description != "" {
				contextParts = append(contextParts, "\nDescription:\n"+description)
			}
		}
		if len(epicStories) > 0 {
			contextParts = append(contextParts, "\nExisting stories already linked to this epic:")
			for _, story := range epicStories {
				storyType := story.TaskType
				if storyType == "" {
					storyType = "feature"
				}
				contextParts = append(contextParts, fmt.Sprintf("- %s (type=%s)", story.Name, storyType))
			}
		}
	}
	if ticket != nil {
		contextParts = append(contextParts, fmt.Sprintf("Please work on support ticket: **%s**", ticket.Subject))
		if ticket.CustomerEmail != nil && *ticket.CustomerEmail != "" {
			contextParts = append(contextParts, "Customer email: "+*ticket.CustomerEmail)
		}
		if len(ticketMessages) > 0 {
			contextParts = append(contextParts, "\nConversation so far:")
			for _, message := range ticketMessages {
				scope := "public"
				if message.IsInternal {
					scope = "internal"
				}
				contextParts = append(contextParts, fmt.Sprintf("- [%s/%s] %s", message.SenderType, scope, message.Content))
			}
		}
	}

	if len(checklist) > 0 {
		contextParts = append(contextParts, "\nChecklist items:")
		for _, item := range checklist {
			status := "[ ]"
			if item.Completed {
				status = "[x]"
			}
			contextParts = append(contextParts, fmt.Sprintf("- %s %s", status, item.Text))
		}
	}

	if artifactSection := formatArtifactContext(artifactContext); artifactSection != "" {
		contextParts = append(contextParts, artifactSection)
	}

	if strings.TrimSpace(initialInstructions) != "" {
		contextParts = append(contextParts, "\nAdditional instructions:\n"+strings.TrimSpace(initialInstructions))
	}

	if ticket != nil {
		contextParts = append(contextParts, "\nPlease triage the issue, update the ticket status if needed, and draft a reply for human approval.")
	}

	if len(contextParts) > 0 {
		sections = append(sections, "Context:\n"+strings.Join(contextParts, "\n"))
	}

	return strings.Join(sections, "\n\n")
}

func BuildExecutionSupplementPrompt(run *model.AgentRun, runFacts map[string]string, artifactContext *ArtifactContext) string {
	var parts []string

	if run != nil && run.InvocationMode == model.InvocationModeInteractive {
		parts = append(parts, "This is an interactive transcript that may resume after a human reply.")
		parts = append(parts, "If the latest human message answers a question, gives feedback, or requests changes, continue the work from that reply.")
		parts = append(parts, "Do not treat a human reply as the end of the run by default. Either continue the task, ask another focused question with request_user_input, request a product review checkpoint with request_review_checkpoint, or reach a durable final outcome.")
		parts = append(parts, "If you need more information from the human, do not end the turn with prose questions or an open-questions list. Call request_user_input with the blocking questions and stop so the session stays interactive.")
		parts = append(parts, "If an approval is denied or the human requests changes, continue from that feedback. If you are blocked afterward, ask the next blocking questions with request_user_input instead of finishing the run.")
	}

	if factsSection := formatRunFacts(runFacts); factsSection != "" {
		parts = append(parts, "Treat the durable run facts below as the authoritative identifiers and persisted context for this run.")
		parts = append(parts, factsSection)
	}

	if artifactSection := formatArtifactContext(artifactContext); artifactSection != "" {
		parts = append(parts, "Use the latest persisted artifacts below as the current source of truth when they conflict with older transcript content.")
		parts = append(parts, strings.TrimSpace(artifactSection))
	}

	return strings.Join(parts, "\n\n")
}

func formatRunFacts(runFacts map[string]string) string {
	if len(runFacts) == 0 {
		return ""
	}

	keys := make([]string, 0, len(runFacts))
	for key, value := range runFacts {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return ""
	}
	sort.Strings(keys)

	var parts []string
	parts = append(parts, "Durable run facts:")
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("- %s=%s", key, strings.TrimSpace(runFacts[key])))
	}
	return strings.Join(parts, "\n")
}

func formatArtifactContext(ctx *ArtifactContext) string {
	if ctx == nil || len(ctx.Entries) == 0 {
		return ""
	}

	var parts []string
	parts = append(parts, "\nCurrent persisted artifacts:")
	for _, entry := range ctx.Entries {
		content := strings.TrimSpace(entry.Content)
		if content == "" {
			continue
		}

		header := strings.TrimSpace(entry.Label)
		if header == "" {
			header = "Artifact"
		}
		var qualifiers []string
		if source := strings.TrimSpace(entry.Source); source != "" {
			qualifiers = append(qualifiers, "source="+source)
		}
		if status := strings.TrimSpace(entry.Status); status != "" {
			qualifiers = append(qualifiers, "status="+status)
		}
		if format := strings.TrimSpace(entry.Format); format != "" {
			qualifiers = append(qualifiers, "format="+format)
		}
		if len(qualifiers) > 0 {
			header += " [" + strings.Join(qualifiers, ", ") + "]"
		}
		parts = append(parts, header+":")
		parts = append(parts, content)
	}
	if len(parts) == 1 {
		return ""
	}
	return strings.Join(parts, "\n")
}

// ParseWorkflowConfig reads a WORKFLOW.md file from the workspace root.
// Returns nil if the file doesn't exist.
func ParseWorkflowConfig(workDir string) *WorkflowConfig {
	path := filepath.Join(workDir, "WORKFLOW.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	content := string(data)
	config := DefaultWorkflowConfig()

	// Check for YAML front matter.
	if strings.HasPrefix(content, "---") {
		parts := strings.SplitN(content[3:], "---", 2)
		if len(parts) == 2 {
			var frontMatter struct {
				MaxIterations   int      `yaml:"max_iterations"`
				TimeoutMinutes  int      `yaml:"timeout_minutes"`
				AllowedCommands []string `yaml:"allowed_commands"`
				HandoffState    string   `yaml:"handoff_state"`
			}
			if err := yaml.Unmarshal([]byte(parts[0]), &frontMatter); err == nil {
				if frontMatter.MaxIterations > 0 {
					config.MaxIterations = frontMatter.MaxIterations
				}
				if frontMatter.TimeoutMinutes > 0 {
					config.TimeoutMinutes = frontMatter.TimeoutMinutes
				}
				if len(frontMatter.AllowedCommands) > 0 {
					config.AllowedCommands = frontMatter.AllowedCommands
				}
				if frontMatter.HandoffState != "" {
					config.HandoffState = frontMatter.HandoffState
				}
			}
			config.ExtraPrompt = strings.TrimSpace(parts[1])
		}
	} else {
		config.ExtraPrompt = strings.TrimSpace(content)
	}

	return config
}
