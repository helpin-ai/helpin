package repository

import (
	"github.com/helpin-ai/helpin/server/internal/linkpreview"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func (q *previewLookup) agent(id string) *model.LinkPreview {
	var agent model.Agent
	if !q.get(&agent, id, "id, name, role, status, team_id") {
		return nil
	}
	teams := q.teams("agent_team_access", "agent_id", agent.ID)
	if teams == "" {
		teams = q.team(agent.TeamID)
	}
	return previewCard("agent", agent.Name, "", previewField("Teams", teams), previewField("Role", previewLabel(agent.Role)), previewField("Status", previewLabel(agent.Status)))
}

func (q *previewLookup) flow(id string) *model.LinkPreview {
	var rule model.AutomationRule
	if !q.get(&rule, id, "id, name, description, team_id, enabled, trigger_type") {
		return nil
	}
	status := "Disabled"
	if rule.Enabled {
		status = "Enabled"
	}
	return previewCard("flow", rule.Name, stringValue(rule.Description), previewField("Team", q.team(rule.TeamID)), previewField("Status", status), previewField("Trigger", previewLabel(rule.TriggerType)))
}

func (q *previewLookup) run(id string, depth int) *model.LinkPreview {
	var run model.AgentRun
	if !q.get(&run, id, "id, agent_id, target_type, target_id, task_id, conversation_id, dock_chat_id, status, started_at") {
		return nil
	}
	if run.TargetType == "support_preview" {
		return nil
	}
	q.modules = append(q.modules, model.ModuleAgents)
	if run.DockChatID != nil && q.chat(*run.DockChatID, false) == nil {
		return nil
	}
	kind := run.TargetType
	if kind == "support_conversation" {
		kind = "conversation"
	}
	if kind == "dock_chat" {
		kind = "chat"
	}
	targetID := run.TargetID
	if targetID == "" && run.TaskID != nil {
		kind, targetID = "task", *run.TaskID
	}
	if targetID == "" && run.ConversationID != nil {
		kind, targetID = "conversation", *run.ConversationID
	}
	switch kind {
	case "", "workspace", "task", "epic", "sprint", "objective", "document", "space", "conversation", "mailbox", "deal", "contact", "company", "meeting", "playbook", "agent", "flow", "chat":
	default:
		return nil
	}
	if kind != "" && targetID == "" {
		return nil
	}
	team := ""
	if targetID != "" {
		// Unknown target kinds fail closed instead of exposing a private target
		// indirectly. Workspace/agent targets have no additional record content.
		if kind == "workspace" {
			if targetID != q.workspace.ID {
				return nil
			}
		} else {
			if kind == "run" || kind == "agent_run" {
				return nil
			}
			target := q.record(linkpreview.Target{Kind: kind, ID: targetID}, depth+1)
			if target == nil {
				return nil
			}
			for _, f := range target.Fields {
				if f.Label == "Team" || f.Label == "Teams" {
					team = f.Value
					break
				}
			}
		}
	}
	agent := q.agent(run.AgentID)
	if agent == nil {
		return nil
	}
	if team == "" {
		for _, f := range agent.Fields {
			if f.Label == "Teams" {
				team = f.Value
				break
			}
		}
	}
	return previewCard("run", agent.Title+" · Agent run", "", previewField("Team", team), previewField("Status", previewLabel(run.Status)), previewField("Started", q.date(run.StartedAt, true)))
}

func (q *previewLookup) chat(id string, public bool) *model.LinkPreview {
	var chat model.DockChat
	conditions := []string{"archived_at IS NULL"}
	if !public {
		conditions = append(conditions, "visibility = 'workspace'")
	}
	if !q.get(&chat, id, "id, title, module_id, updated_at", conditions...) {
		return nil
	}
	fields := []model.LinkPreviewField{}
	if !public && chat.ModuleID != nil {
		q.modules = append(q.modules, *chat.ModuleID)
		label := map[model.ModuleID]string{model.ModulePM: "Projects", model.ModuleCRM: "Sales", model.ModuleDocs: "Docs", model.ModuleSupport: "Support", model.ModuleAutomation: "Automation", model.ModuleAgents: "Agents"}[*chat.ModuleID]
		fields = append(fields, previewField("Module", label))
	}
	fields = append(fields, previewField("Updated", chat.UpdatedAt.Format("Jan 2, 2006")))
	return previewCard("chat", chat.Title, "", fields...)
}
