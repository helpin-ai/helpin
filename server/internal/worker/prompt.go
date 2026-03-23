package worker

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// BuildSystemPrompt assembles the system prompt from agent config, target context, and WORKFLOW.md.
func BuildSystemPrompt(agent *model.Agent, story *model.PMStory, epic *model.PMEpic, ticket *model.SupportConversation, planningStage, planningMethodology string, config *WorkflowConfig) string {
	var parts []string

	if agent != nil && agent.SystemPrompt != nil && strings.TrimSpace(*agent.SystemPrompt) != "" {
		parts = append(parts, strings.TrimSpace(*agent.SystemPrompt))
	} else if epic != nil && strings.TrimSpace(planningStage) != "" {
		// Legacy staged planning compatibility. Direct planner runs should keep instructions on the agent itself.
		parts = append(parts, planningIdentity(agent, planningStage, planningMethodology))
		parts = append(parts, planningPackSections(agent, planningStage, planningMethodology)...)
	} else {
		parts = append(parts, fmt.Sprintf("You are %s, an AI coding agent. You write clean, correct code and follow existing project conventions.", agent.Name))
	}

	// Story context.
	if story != nil {
		parts = append(parts, "\n## Current Task")
		parts = append(parts, fmt.Sprintf("**Story**: %s", story.Name))
		if story.Description != nil && *story.Description != "" {
			parts = append(parts, fmt.Sprintf("**Description**: %s", *story.Description))
		}
	}
	if epic != nil {
		parts = append(parts, "\n## Current Epic")
		parts = append(parts, fmt.Sprintf("**Epic**: %s", epic.Name))
		if epic.Description != nil && *epic.Description != "" {
			parts = append(parts, fmt.Sprintf("**Description**: %s", *epic.Description))
		}
		switch strings.TrimSpace(planningStage) {
		case model.PlanningStageDraftSpec:
			parts = append(parts, `Respond with valid JSON in this shape: {"title":"...","summary":"...","spec_markdown":"# ...","risks":["..."],"assumptions":["..."],"open_questions":["..."],"sources":[{"title":"...","url":"https://...","note":"why this matters","published_at":"optional"}]}.`)
		case model.PlanningStagePlanStories:
			parts = append(parts, `Respond with valid JSON in this shape: {"summary":"...","spec_version_id":"optional","proposed_stories":[{"ref":"story_1","name":"...","description":"...","story_type":"feature|bug|chore","estimate":1,"priority":"none|low|medium|high|urgent","acceptance_criteria":["..."],"dependency_refs":["story_0"],"source_refs":[{"type":"spec_section","title":"..."}],"assign_agent_id":"optional-agent-id","slice_type":"vertical|enabler|spike","implementation_brief":{"approach":"...","files_to_modify":[{"path":"server/internal/model/foo.go","action":"create|modify|delete","description":"..."}],"test_strategy":"...","vertical_layers":["model","repository","service"],"depends_on_files":["server/internal/model/bar.go"]}}],"vertical_coverage":[{"behavior":"...","story_refs":["story_1"],"full_slice":true}],"open_questions":["..."],"risks":["..."]}.`)
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
	parts = append(parts, "- Use the provided tools to read, write, and search files.")
	if story != nil {
		parts = append(parts, "- Run tests after making changes when possible.")
		parts = append(parts, "- Commit and push your changes when the task is complete.")
	}
	if epic != nil {
		switch strings.TrimSpace(planningStage) {
		case model.PlanningStageDraftSpec:
			parts = append(parts, "- Produce a structured product spec draft, not implementation tasks.")
			parts = append(parts, "- The spec markdown must be well organized with headings and scenario-style acceptance language.")
			parts = append(parts, "- Return assumptions separately from open questions so a human can resolve them before approval.")
			parts = append(parts, "- If the web_search tool is available and you use it, return sources in the JSON sources field.")
			parts = append(parts, "- Do not embed a Research Sources section inside spec_markdown; the system will append a normalized citations section.")
			parts = append(parts, "- Return JSON only, with no markdown fences.")
		case model.PlanningStagePlanStories:
			parts = append(parts, "- Propose implementation-ready stories grounded in the approved spec and the current codebase.")
			parts = append(parts, "- Use stable story refs so dependencies can be mapped deterministically.")
			parts = append(parts, "- Prefer vertical, user-visible slices. Only introduce enabler stories when a vertical slice would be misleading or unsafe.")
			parts = append(parts, "- Keep story names flat and outcome-oriented. Do not use phase prefixes or sequencing labels in titles.")
			parts = append(parts, "- Align stories with the existing module boundaries, naming patterns, and architecture when the code context is clear.")
			parts = append(parts, "- If the approved spec conflicts with the current implementation or the code context is ambiguous, surface that as risks or open questions.")
			parts = append(parts, "- Avoid duplicating or overlapping existing stories.")
			parts = append(parts, "- Return JSON only, with no markdown fences.")
		}
	}
	if ticket != nil {
		parts = append(parts, "- Customer-visible replies must be drafted for human approval before they are sent.")
	}
	parts = append(parts, "- Leave Helpin artifacts and summaries in a state a human can review.")

	return strings.Join(parts, "\n")
}

// BuildUserPrompt creates the initial user message for the run.
func BuildUserPrompt(
	story *model.PMStory,
	epic *model.PMEpic,
	epicStories []model.PMStory,
	ticket *model.SupportConversation,
	ticketMessages []model.SupportMessage,
	checklist []model.PMChecklistItem,
	planningStage string,
	initialInstructions string,
) string {
	var parts []string

	if story != nil {
		parts = append(parts, fmt.Sprintf("Please work on the story: **%s**", story.Name))
		if story.Description != nil && *story.Description != "" {
			parts = append(parts, "\nDescription:\n"+*story.Description)
		}
	}
	if epic != nil {
		switch strings.TrimSpace(planningStage) {
		case model.PlanningStageDraftSpec:
			parts = append(parts, fmt.Sprintf("Please draft or refresh the canonical product spec for epic: **%s**", epic.Name))
		case model.PlanningStagePlanStories:
			parts = append(parts, fmt.Sprintf("Please create a dependency-aware story plan for epic: **%s**", epic.Name))
		default:
			parts = append(parts, fmt.Sprintf("Please work on epic: **%s**", epic.Name))
		}
		if epic.Description != nil && *epic.Description != "" {
			parts = append(parts, "\nDescription:\n"+*epic.Description)
		}
		if len(epicStories) > 0 {
			parts = append(parts, "\nExisting stories already linked to this epic:")
			for _, story := range epicStories {
				storyType := story.StoryType
				if storyType == "" {
					storyType = "feature"
				}
				parts = append(parts, fmt.Sprintf("- %s (type=%s)", story.Name, storyType))
			}
		}
	}
	if ticket != nil {
		parts = append(parts, fmt.Sprintf("Please work on support ticket: **%s**", ticket.Subject))
		if ticket.CustomerEmail != nil && *ticket.CustomerEmail != "" {
			parts = append(parts, "Customer email: "+*ticket.CustomerEmail)
		}
		if len(ticketMessages) > 0 {
			parts = append(parts, "\nConversation so far:")
			for _, message := range ticketMessages {
				scope := "public"
				if message.IsInternal {
					scope = "internal"
				}
				parts = append(parts, fmt.Sprintf("- [%s/%s] %s", message.SenderType, scope, message.Content))
			}
		}
	}

	if len(checklist) > 0 {
		parts = append(parts, "\nChecklist items:")
		for _, item := range checklist {
			status := "[ ]"
			if item.Completed {
				status = "[x]"
			}
			parts = append(parts, fmt.Sprintf("- %s %s", status, item.Text))
		}
	}

	if strings.TrimSpace(initialInstructions) != "" {
		parts = append(parts, "\nAdditional instructions:\n"+strings.TrimSpace(initialInstructions))
	}

	if ticket != nil {
		parts = append(parts, "\nPlease triage the issue, update the ticket status if needed, and draft a reply for human approval.")
	} else if epic != nil {
		switch strings.TrimSpace(planningStage) {
		case model.PlanningStageDraftSpec:
			parts = append(parts, "\nCreate a structured product spec draft that a human can edit and approve in Docs. Separate assumptions from open questions so the human owner can clarify the draft before approval.")
		case model.PlanningStagePlanStories:
			parts = append(parts, "\nUse the approved spec and the current codebase context to produce a reviewable story plan with acceptance criteria, dependencies, risks, and open questions.")
		}
	} else {
		parts = append(parts, "\nPlease complete this task. Start by reading the relevant files to understand the codebase, then implement the changes.")
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
