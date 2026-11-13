package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/linkpreview"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// LinkPreviewRepository reads only the fields permitted in anonymous previews.
// Every entity and relation lookup is scoped to a verified workspace.
type LinkPreviewRepository struct{ db *gorm.DB }

// NewLinkPreviewRepository creates the metadata-specific repository.
func NewLinkPreviewRepository(db *gorm.DB) *LinkPreviewRepository {
	return &LinkPreviewRepository{db: db}
}

// GetWorkspace loads only the context needed to resolve public preview fields.
func (r *LinkPreviewRepository) GetWorkspace(ctx context.Context, slug string) (*model.Workspace, error) {
	var workspace model.Workspace
	err := r.db.WithContext(ctx).Select("id, slug, workspace_key, timezone, link_previews_enabled").Where("slug = ?", slug).First(&workspace).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &workspace, nil
}

// GetRecord returns an eligible projection, or nil for a missing/restricted item.
func (r *LinkPreviewRepository) GetRecord(ctx context.Context, ws *model.Workspace, target linkpreview.Target) (*model.LinkPreviewRecord, error) {
	q := &previewLookup{db: r.db.WithContext(ctx), workspace: ws}
	card := q.record(target, 0)
	if q.err != nil {
		return nil, fmt.Errorf("read link preview: %w", q.err)
	}
	if card == nil {
		return nil, nil
	}
	return &model.LinkPreviewRecord{Preview: *card, Modules: q.modules}, nil
}

type previewLookup struct {
	db        *gorm.DB
	workspace *model.Workspace
	err       error
	modules   []model.ModuleID
}

func (q *previewLookup) record(t linkpreview.Target, depth int) *model.LinkPreview {
	if depth > 2 {
		return nil
	}
	q.modules = append(q.modules, linkpreview.ModuleForKind(t.Kind))
	switch t.Kind {
	case "task":
		return q.task(t)
	case "epic":
		return q.epic(t.ID)
	case "sprint":
		return q.sprint(t.ID)
	case "objective":
		return q.objective(t.ID)
	case "document":
		return q.document(t.ID)
	case "space":
		return q.space(t)
	case "collection":
		return q.collection(t)
	case "conversation":
		return q.conversation(t.ID)
	case "mailbox":
		return q.mailbox(t.ID)
	case "deal":
		return q.deal(t.ID)
	case "contact":
		return q.contact(t.ID)
	case "company":
		return q.company(t.ID)
	case "meeting":
		return q.meeting(t.ID)
	case "playbook":
		return q.playbook(t.ID)
	case "agent":
		return q.agent(t.ID)
	case "flow":
		return q.flow(t.ID)
	case "run":
		return q.run(t.ID, depth)
	case "chat":
		return q.chat(t.ID, false)
	}
	return nil
}

func (q *previewLookup) scoped() *gorm.DB { return q.db.Where("workspace_id = ?", q.workspace.ID) }

func (q *previewLookup) get(out any, id, columns string, conditions ...string) bool {
	query := q.scoped().Select(columns).Where("id = ?", id)
	for _, condition := range conditions {
		query = query.Where(condition)
	}
	err := query.First(out).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false
	}
	if err != nil {
		q.err = err
		return false
	}
	return true
}

// Table and column arguments below are constants supplied by repository code.
func (q *previewLookup) name(table, id string) string {
	if id == "" {
		return ""
	}
	var value string
	query := q.scoped().Table(table).Where("id = ?", id).Select("name")
	switch table {
	case "pm_workflow_states":
		query = q.db.Table("pm_workflow_states s").Joins("JOIN pm_workflows w ON w.id = s.workflow_id").Where("w.workspace_id = ? AND s.id = ?", q.workspace.ID, id).Select("s.name")
	case "crm_pipeline_stages":
		query = q.db.Table("crm_pipeline_stages s").Joins("JOIN crm_pipelines p ON p.id = s.pipeline_id").Where("p.workspace_id = ? AND s.id = ?", q.workspace.ID, id).Select("s.name")
	}
	err := query.Limit(1).Scan(&value).Error
	if err != nil {
		q.err = err
	}
	return value
}

func (q *previewLookup) member(id *string, userID bool) string {
	if id == nil || *id == "" {
		return "Unassigned"
	}
	key := "wm.id"
	if userID {
		key = "wm.user_id"
	}
	var value string
	err := q.db.Table("workspace_members wm").
		Joins("LEFT JOIN users u ON u.id = wm.user_id").
		Where("wm.workspace_id = ? AND "+key+" = ?", q.workspace.ID, *id).
		Select("COALESCE(NULLIF(wm.display_name, ''), NULLIF(u.full_name, ''), '')").Limit(1).Scan(&value).Error
	if err != nil {
		q.err = err
	}
	return value
}

func (q *previewLookup) team(id *string) string { return q.name("workspace_teams", stringValue(id)) }

func (q *previewLookup) teams(join, column, id string) string {
	var names []string
	err := q.db.Table("workspace_teams t").
		Joins("JOIN "+join+" j ON j.team_id = t.id").
		Where("t.workspace_id = ? AND j."+column+" = ?", q.workspace.ID, id).
		Order("t.name, t.id").Pluck("t.name", &names).Error
	if err != nil {
		q.err = err
	}
	return compactNames(names)
}

func previewCard(kind, title, description string, fields ...model.LinkPreviewField) *model.LinkPreview {
	card := linkpreview.TypeOnly(kind)
	if strings.TrimSpace(title) != "" {
		card.Title = title
	}
	card.Description = description
	for _, f := range fields {
		if strings.TrimSpace(f.Value) != "" {
			card.Fields = append(card.Fields, f)
		}
	}
	return &card
}

func previewField(label, value string) model.LinkPreviewField {
	return model.LinkPreviewField{Label: label, Value: value}
}
func compactNames(names []string) string {
	var values []string
	seen := map[string]bool{}
	for _, n := range names {
		if n = strings.TrimSpace(n); n != "" && !seen[n] {
			values = append(values, n)
			seen[n] = true
		}
	}
	if len(values) == 0 {
		return ""
	}
	if len(values) == 1 {
		return values[0]
	}
	return fmt.Sprintf("%s +%d", values[0], len(values)-1)
}

func (q *previewLookup) date(v *time.Time, clock bool) string {
	if v == nil {
		return ""
	}
	if !clock {
		return v.Format("Jan 2, 2006")
	}
	loc, err := time.LoadLocation(q.workspace.Timezone)
	if err != nil {
		loc = time.UTC
	}
	return v.In(loc).Format("Jan 2, 2006 · 15:04 MST")
}

func previewLabel(value string) string {
	labels := map[string]string{"unstarted": "Not started", "started": "In progress", "done": "Done", "in_progress": "In progress", "waiting_on_customer": "Waiting on customer", "waiting_on_team": "Waiting on team", "google_meet": "Google Meet", "microsoft_teams": "Microsoft Teams", "task.state_entered": "Task enters a status", "story.state_entered": "Task enters a status", "agent_run.approved": "Agent run approved", "cron": "Schedule", "qa": "QA"}
	if label, ok := labels[value]; ok {
		return label
	}
	words := strings.Fields(strings.NewReplacer("_", " ", ".", " ", "-", " ").Replace(value))
	for i, w := range words {
		r := []rune(w)
		if len(r) > 0 && r[0] >= 'a' && r[0] <= 'z' {
			r[0] -= 'a' - 'A'
		}
		words[i] = string(r)
	}
	return strings.Join(words, " ")
}
