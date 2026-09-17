package service

import (
	"context"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/pmtriage"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

// AnalyzeDraft offers accessible existing work before a task is created.
func (s *PMTriageService) AnalyzeDraft(ctx context.Context, workspaceID string, req model.PMTriageDraftRequest) (*model.PMTriageView, error) {
	actor := authorization.GetActor(ctx)
	if actor == nil || actor.UserID == "" || actor.WorkspaceID != workspaceID || !authorization.NewRBACEngine().Can(actor.Role, authorization.PermPMEdit) {
		return nil, &model.ErrForbidden{Message: "PM edit permission required"}
	}
	view := &model.PMTriageView{Status: "disabled", SourceKind: "task_draft", SourceID: req.DraftID, Teams: []pmtriage.Option{}, Labels: []pmtriage.Option{}, Candidates: []model.PMTriageCandidateView{}}
	if !s.enabled(workspaceID) {
		return view, nil
	}
	if req.TeamID != nil {
		if err := requireTeamMembershipForCreate(ctx, req.TeamID); err != nil {
			return nil, err
		}
	}
	text := strings.TrimSpace(req.Name) + "\n" + tiptap.RichTextToMarkdown(req.Description)
	source := &pmTriageSource{input: pmtriage.Input{SourceKind: "task_draft", SourceID: req.DraftID, Text: text, Teams: []pmtriage.Option{}, Labels: []pmtriage.Option{}, Candidates: []pmtriage.Candidate{}}, hash: pmTriageHash(text), teamID: req.TeamID}
	return s.analyzeSource(ctx, actor, source, view)
}
