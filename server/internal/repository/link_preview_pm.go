package repository

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/linkpreview"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func (q *previewLookup) task(t linkpreview.Target) *model.LinkPreview {
	var task model.PMTask
	query := q.scoped().Select("id, name, description, display_id, team_id, workflow_state_id").Where("archived = false AND team_id IS NOT NULL")
	if t.ID != "" {
		query = query.Where("id = ?", t.ID)
	} else {
		parts := strings.Split(t.Task, "-")
		number, err := strconv.Atoi(parts[len(parts)-1])
		if err != nil || number < 1 {
			return nil
		}
		if len(parts) == 2 && parts[0] != q.workspace.WorkspaceKey {
			var count int64
			if err := q.scoped().Model(&model.WorkspaceKeyHistory{}).Where("old_key = ?", parts[0]).Count(&count).Error; err != nil {
				q.err = err
				return nil
			}
			if count == 0 {
				return nil
			}
		}
		query = query.Where("display_id = ?", number)
	}
	if err := query.First(&task).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			q.err = err
		}
		return nil
	}
	team := q.team(task.TeamID)
	if team == "" {
		return nil
	}
	var owners []string
	if err := q.db.Table("pm_task_owners o").
		Joins("JOIN workspace_members wm ON wm.user_id = o.user_id AND wm.workspace_id = ?", q.workspace.ID).
		Joins("LEFT JOIN users u ON u.id = wm.user_id").Where("o.task_id = ?", task.ID).
		Order("wm.display_name, wm.id").Pluck("COALESCE(NULLIF(wm.display_name, ''), NULLIF(u.full_name, ''), '')", &owners).Error; err != nil {
		q.err = err
		return nil
	}
	assignee := compactNames(owners)
	if len(owners) == 0 {
		assignee = "Unassigned"
	}
	card := previewCard("task", task.Name, stringValue(task.Description), previewField("Team", team), previewField("Status", q.name("pm_workflow_states", task.WorkflowStateID)), previewField("Assignee", assignee))
	card.Reference = fmt.Sprintf("%s-%d", q.workspace.WorkspaceKey, task.DisplayID)
	if t.Run != "" {
		var run model.AgentRun
		if q.get(&run, t.Run, "id, target_type, target_id, task_id") && ((run.TargetType == "task" && run.TargetID == task.ID) || stringValue(run.TaskID) == task.ID) {
			if preview := q.run(t.Run, 0); preview != nil {
				return preview
			}
			fallback := linkpreview.TypeOnly("run")
			return &fallback
		}
	}
	return card
}

func (q *previewLookup) epic(id string) *model.LinkPreview {
	var epic model.PMEpic
	if !q.get(&epic, id, "id, name, description, team_id, epic_state_id, owner_member_id", "archived = false AND team_id IS NOT NULL") {
		return nil
	}
	team := q.team(epic.TeamID)
	if team == "" {
		return nil
	}
	return previewCard("epic", epic.Name, stringValue(epic.Description), previewField("Team", team), previewField("Status", q.name("pm_epic_workflow_states", stringValue(epic.EpicStateID))), previewField("Owner", q.member(epic.OwnerMemberID, false)))
}

func (q *previewLookup) sprint(id string) *model.LinkPreview {
	var sprint model.PMSprint
	if !q.get(&sprint, id, "id, name, description, team_id, start_date, end_date", "archived = false AND team_id IS NOT NULL") {
		return nil
	}
	team := q.team(sprint.TeamID)
	if team == "" {
		return nil
	}
	dates := q.date(sprint.StartDate, false)
	if sprint.StartDate != nil && sprint.EndDate != nil && sprint.StartDate.Year() == sprint.EndDate.Year() {
		dates = sprint.StartDate.Format("Jan 2")
	}
	if end := q.date(sprint.EndDate, false); end != "" {
		if dates != "" {
			dates += " – "
		}
		dates += end
	}
	status := computeSprintStatus(sprint.StartDate, sprint.EndDate, time.Now().UTC())
	return previewCard("sprint", sprint.Name, stringValue(sprint.Description), previewField("Team", team), previewField("Status", previewLabel(status)), previewField("Dates", dates))
}

func (q *previewLookup) objective(id string) *model.LinkPreview {
	var objective model.PMObjective
	if !q.get(&objective, id, "id, name, description, state", "archived = false") {
		return nil
	}
	teams := q.teams("pm_objective_teams", "objective_id", id)
	if teams == "" {
		return nil
	}
	// Use the same stored KR progress and completed-task ratio as objective stats,
	// but scope all related rows and omit archived linked epics.
	var kr struct {
		Count    int
		Progress float64
	}
	if err := q.db.Table("pm_key_results k").Joins("JOIN pm_objectives o ON o.id = k.objective_id").Select("COUNT(*) AS count, COALESCE(AVG(k.progress),0) AS progress").Where("o.workspace_id = ? AND k.objective_id = ?", q.workspace.ID, id).Scan(&kr).Error; err != nil {
		q.err = err
		return nil
	}
	progress := ""
	if kr.Count > 0 {
		progress = fmt.Sprintf("%.0f%%", kr.Progress)
	} else {
		var stats struct {
			Epics int
			Total int
			Done  int
		}
		err := q.db.Table("pm_epics e").Joins("JOIN pm_epic_objectives eo ON eo.epic_id = e.id").
			Joins("LEFT JOIN pm_tasks t ON t.epic_id = e.id AND t.workspace_id = e.workspace_id AND t.archived = false").
			Joins("LEFT JOIN pm_workflows w ON w.id = t.workflow_id AND w.workspace_id = e.workspace_id").
			Joins("LEFT JOIN pm_workflow_states s ON s.id = t.workflow_state_id AND s.workflow_id = w.id").
			Where("e.workspace_id = ? AND eo.objective_id = ? AND e.archived = false", q.workspace.ID, id).
			Select("COUNT(DISTINCT e.id) AS epics, COUNT(t.id) AS total, COUNT(CASE WHEN s.state_type = 'done' THEN 1 END) AS done").Scan(&stats).Error
		if err != nil {
			q.err = err
			return nil
		}
		if stats.Epics > 0 {
			pct := float64(0)
			if stats.Total > 0 {
				pct = float64(stats.Done) / float64(stats.Total) * 100
			}
			progress = fmt.Sprintf("%.0f%%", pct)
		}
	}
	return previewCard("objective", objective.Name, stringValue(objective.Description), previewField("Teams", teams), previewField("Status", previewLabel(objective.State)), previewField("Progress", progress))
}
