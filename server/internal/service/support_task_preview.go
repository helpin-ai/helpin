package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

// PreviewTaskFromConversation prepares an editable draft without creating work.
// It uses the existing metered draft generator and public conversation evidence.
func (s *SupportInboxService) PreviewTaskFromConversation(ctx context.Context, workspaceID, conversationID string, teamIDs ...string) (*model.SupportTaskDraftPreview, error) {
	conversation, err := s.loadConversationAccessible(ctx, workspaceID, conversationID)
	if err != nil {
		return nil, err
	}
	if conversation == nil || conversation.AnonymizedAt != nil {
		return nil, ErrPMTriageSourceUnavailable
	}
	// Draft generation includes attachment context; matching and reviewed saves only need public text.
	messages, err := s.ListConversationMessages(ctx, workspaceID, conversationID, false)
	if err != nil {
		return nil, err
	}
	teamContext := ""
	teamID := ""
	if len(teamIDs) > 0 {
		teamID = strings.TrimSpace(teamIDs[0])
	}
	if teamID != "" {
		if err := requireTeamAccess(ctx, &teamID); err != nil {
			return nil, err
		}
		if s.workspaceRepo == nil {
			return nil, fmt.Errorf("team settings unavailable")
		}
		team, err := s.workspaceRepo.GetTeamByID(ctx, workspaceID, teamID)
		if err != nil {
			return nil, err
		}
		if team == nil {
			return nil, fmt.Errorf("team not found in workspace")
		}
		raw, err := json.Marshal(map[string]string{"name": team.Name, "type": team.TeamType, "description": derefString(team.Description), "default_task_type": team.DefaultTaskType})
		if err != nil {
			return nil, err
		}
		teamContext = string(raw)
	}
	draft, err := s.generateTaskDraftFromConversation(ctx, workspaceID, conversation, messages, teamContext)
	if err != nil {
		return nil, err
	}
	_, hash := supportPMTriageEvidence(conversation, messages)
	return &model.SupportTaskDraftPreview{Name: draft.Title, Description: derefString(supportTaskDescriptionToRichText(draft.Description)), TaskType: draft.TaskType, Priority: draft.Priority, SourceHash: hash}, nil
}

func supportPMTriageEvidence(conversation *model.SupportConversation, messages []model.SupportMessage) (string, string) {
	var text, revision strings.Builder
	text.WriteString(conversation.Subject)
	revision.WriteString(conversation.UpdatedAt.String())
	for _, message := range messages {
		if message.IsInternal || message.DeletedAt.Valid || (message.MessageType != "" && message.MessageType != "reply") {
			continue
		}
		text.WriteString("\n" + message.SenderType + ": " + tiptap.RichTextToMarkdown(message.Content))
		revision.WriteString(message.ID + message.UpdatedAt.String())
	}
	return text.String(), pmTriageHash(text.String() + "\n" + revision.String())
}
