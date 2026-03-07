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
func BuildSystemPrompt(agent *model.Agent, story *model.PMStory, epic *model.PMEpic, ticket *model.SupportTicket, config *WorkflowConfig) string {
	var parts []string

	// Agent's own system prompt.
	if agent.SystemPrompt != nil && *agent.SystemPrompt != "" {
		parts = append(parts, *agent.SystemPrompt)
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
		parts = append(parts, `Respond with valid JSON in this shape: {"summary":"...","proposed_stories":[{"name":"...","description":"...","story_type":"feature|bug|chore","estimate":1,"assign_agent_id":"optional-agent-id"}]}.`)
	}
	if ticket != nil {
		parts = append(parts, "\n## Current Support Ticket")
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
		parts = append(parts, "- Propose implementation-ready stories, not vague project phases.")
		parts = append(parts, "- Avoid duplicating or overlapping existing stories.")
		parts = append(parts, "- Return JSON only, with no markdown fences.")
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
	ticket *model.SupportTicket,
	ticketMessages []model.SupportMessage,
	checklist []model.PMChecklistItem,
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
		parts = append(parts, fmt.Sprintf("Please decompose epic: **%s**", epic.Name))
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
		parts = append(parts, "\nPropose a concise epic summary and a set of concrete, implementation-ready stories.")
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
