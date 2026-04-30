package temporalapp

import (
	"context"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

func (a *AgentRunActivities) ensureEpicSpecDocument(ctx context.Context, state *resolvedRunState, actorID string) (*model.DocsDocument, error) {
	if state.epic == nil {
		return nil, fmt.Errorf("epic context is required")
	}
	if state.epic.SpecDocumentID != nil && strings.TrimSpace(*state.epic.SpecDocumentID) != "" {
		doc, err := a.docsDocRepo.GetByID(ctx, *state.epic.SpecDocumentID)
		if err != nil {
			return nil, err
		}
		if doc != nil {
			return doc, nil
		}
	}

	space, err := a.docsSpaceRepo.GetBySlug(ctx, state.run.WorkspaceID, productSpecsSpaceSlug)
	if err != nil {
		return nil, err
	}
	if space == nil {
		space, err = a.docsSpaceRepo.Create(ctx, &model.DocsSpace{
			WorkspaceID: state.run.WorkspaceID,
			Name:        productSpecsSpaceName,
			Slug:        productSpecsSpaceSlug,
			Visibility:  model.SpaceVisibilityWorkspaceWide,
			Type:        model.SpaceTypeInternal,
			IsSystem:    true,
			Position:    2,
			CreatedBy:   actorID,
		})
		if err != nil {
			return nil, fmt.Errorf("create product specs space: %w", err)
		}
	}

	teamID := state.epic.TeamID
	if teamID == nil {
		teamID = strPtr(actorID)
	}

	doc, err := a.docsDocRepo.Create(ctx, &model.DocsDocument{
		WorkspaceID: state.run.WorkspaceID,
		SpaceID:     space.ID,
		Title:       strings.TrimSpace(state.epic.Name) + " Product Spec",
		Status:      model.DocStatusDraft,
		Visibility:  model.SpaceVisibilityWorkspaceWide,
		OwnerID:     strPtr(actorID),
		TeamID:      teamID,
		TemplateKey: strPtr("product_spec"),
		Tags:        model.DocsStringArray{"product-spec", "epic"},
		CreatedBy:   actorID,
	})
	if err != nil {
		return nil, err
	}

	state.epic.SpecDocumentID = &doc.ID
	if err := a.epicRepo.Update(ctx, state.epic); err != nil {
		return nil, err
	}
	if err := a.ensureEpicSpecLink(ctx, state.run.WorkspaceID, doc.ID, state.epic.ID, actorID); err != nil {
		return nil, err
	}

	return doc, nil
}

func (a *AgentRunActivities) ensureTaskPlanDocument(ctx context.Context, state *resolvedRunState, actorID string) (*model.DocsDocument, error) {
	if state.task == nil {
		return nil, fmt.Errorf("task not found")
	}
	if state.task.PlanDocumentID != nil && strings.TrimSpace(*state.task.PlanDocumentID) != "" {
		doc, err := a.docsDocRepo.GetByID(ctx, *state.task.PlanDocumentID)
		if err != nil {
			return nil, err
		}
		if doc != nil {
			if err := a.ensureTaskPlanLink(ctx, state.run.WorkspaceID, doc.ID, state.task.ID, actorID); err != nil {
				return nil, err
			}
			return doc, nil
		}
	}

	space, err := a.docsSpaceRepo.GetBySlug(ctx, state.run.WorkspaceID, productSpecsSpaceSlug)
	if err != nil {
		return nil, err
	}
	if space == nil {
		space, err = a.docsSpaceRepo.Create(ctx, &model.DocsSpace{
			WorkspaceID: state.run.WorkspaceID,
			Name:        productSpecsSpaceName,
			Slug:        productSpecsSpaceSlug,
			Visibility:  model.SpaceVisibilityWorkspaceWide,
			Type:        model.SpaceTypeInternal,
			IsSystem:    true,
			Position:    2,
			CreatedBy:   actorID,
		})
		if err != nil {
			return nil, fmt.Errorf("create product specs space: %w", err)
		}
	}

	teamID := state.task.TeamID
	if teamID == nil && state.epic != nil {
		teamID = state.epic.TeamID
	}
	if teamID == nil {
		teamID = strPtr(actorID)
	}

	doc, err := a.docsDocRepo.Create(ctx, &model.DocsDocument{
		WorkspaceID: state.run.WorkspaceID,
		SpaceID:     space.ID,
		Title:       strings.TrimSpace(state.task.Name) + " Plan",
		Status:      model.DocStatusDraft,
		Visibility:  model.SpaceVisibilityWorkspaceWide,
		OwnerID:     strPtr(actorID),
		TeamID:      teamID,
		TemplateKey: strPtr("task_plan"),
		Tags:        model.DocsStringArray{"task-plan", "task"},
		CreatedBy:   actorID,
	})
	if err != nil {
		return nil, err
	}

	state.task.PlanDocumentID = &doc.ID
	if err := a.taskRepo.Update(ctx, state.task); err != nil {
		return nil, err
	}
	if err := a.ensureTaskPlanLink(ctx, state.run.WorkspaceID, doc.ID, state.task.ID, actorID); err != nil {
		return nil, err
	}
	return doc, nil
}

func (a *AgentRunActivities) ensureEpicSpecLink(ctx context.Context, workspaceID, documentID, epicID, actorID string) error {
	links, err := a.docsLinkRepo.ListByObject(ctx, workspaceID, model.LinkedObjectEpic, epicID)
	if err != nil {
		return err
	}
	for _, link := range links {
		if link.DocumentID == documentID {
			return nil
		}
	}
	_, err = a.docsLinkRepo.Create(ctx, &model.DocsLink{
		WorkspaceID:      workspaceID,
		DocumentID:       documentID,
		LinkedObjectType: model.LinkedObjectEpic,
		LinkedObjectID:   epicID,
		LinkContext:      model.LinkContextCreatedFrom,
		CreatedBy:        actorID,
	})
	return err
}

func (a *AgentRunActivities) ensureTaskPlanLink(ctx context.Context, workspaceID, documentID, taskID, actorID string) error {
	links, err := a.docsLinkRepo.ListByObject(ctx, workspaceID, model.LinkedObjectTask, taskID)
	if err != nil {
		return err
	}
	for _, link := range links {
		if link.DocumentID == documentID {
			return nil
		}
	}
	_, err = a.docsLinkRepo.Create(ctx, &model.DocsLink{
		WorkspaceID:      workspaceID,
		DocumentID:       documentID,
		LinkedObjectType: model.LinkedObjectTask,
		LinkedObjectID:   taskID,
		LinkContext:      model.LinkContextCreatedFrom,
		CreatedBy:        actorID,
	})
	return err
}

func (a *AgentRunActivities) renderLinkedDocsContext(ctx context.Context, workspaceID, epicID, excludeDocumentID string) (string, error) {
	return a.renderObjectLinkedDocsContext(ctx, workspaceID, model.LinkedObjectEpic, epicID, excludeDocumentID)
}

func (a *AgentRunActivities) renderObjectLinkedDocsContext(ctx context.Context, workspaceID, objectType, objectID, excludeDocumentID string) (string, error) {
	if a.docsLinkRepo == nil || a.docsDocRepo == nil || a.docsContentRepo == nil {
		return "", nil
	}
	links, err := a.docsLinkRepo.ListByObject(ctx, workspaceID, objectType, objectID)
	if err != nil {
		return "", err
	}
	if len(links) == 0 {
		return "", nil
	}

	seen := make(map[string]bool, len(links))
	entries := make([]string, 0, len(links))
	for _, link := range links {
		if link.DocumentID == excludeDocumentID || seen[link.DocumentID] {
			continue
		}
		seen[link.DocumentID] = true

		doc, err := a.docsDocRepo.GetByID(ctx, link.DocumentID)
		if err != nil {
			return "", err
		}
		if doc == nil {
			continue
		}

		content, err := a.docsContentRepo.GetByDocumentID(ctx, doc.ID)
		if err != nil {
			return "", err
		}
		body := "(no content yet)"
		if markdown := docsContentMarkdown(content); markdown != "" {
			body = truncatePlanningText(markdown, 3000)
		}

		entries = append(entries, fmt.Sprintf("- %s [%s]\n%s", doc.Title, doc.ID, body))
		if len(entries) >= 5 {
			break
		}
	}

	return strings.Join(entries, "\n\n"), nil
}

func docsContentMarkdown(content *model.DocsContent) string {
	if content == nil {
		return ""
	}
	if markdown := tiptap.RichTextToMarkdown(string(content.Content)); markdown != "" {
		return markdown
	}
	return strings.TrimSpace(content.ContentText)
}

func (a *AgentRunActivities) renderTaskCommentsContext(ctx context.Context, taskID string) (string, error) {
	if strings.TrimSpace(taskID) == "" || a.commentRepo == nil {
		return "", nil
	}
	comments, err := a.commentRepo.List(ctx, "task", taskID)
	if err != nil {
		return "", err
	}
	if len(comments) == 0 {
		return "", nil
	}

	entries := make([]string, 0, len(comments))
	for idx, entry := range comments {
		authorName := strings.TrimSpace(entry.Author.FullName)
		if authorName == "" {
			authorName = entry.Comment.AuthorID
		}
		line := fmt.Sprintf("- %s: %s", authorName, truncatePlanningText(entry.Comment.Body, 320))
		if len(entry.Replies) > 0 {
			replyLines := make([]string, 0, len(entry.Replies))
			for _, reply := range entry.Replies {
				replyAuthor := strings.TrimSpace(reply.Author.FullName)
				if replyAuthor == "" {
					replyAuthor = reply.Comment.AuthorID
				}
				replyLines = append(replyLines, fmt.Sprintf("  - %s: %s", replyAuthor, truncatePlanningText(reply.Comment.Body, 220)))
			}
			line += "\nReplies:\n" + strings.Join(replyLines, "\n")
		}
		entries = append(entries, line)
		if idx >= 5 {
			break
		}
	}
	return strings.Join(entries, "\n\n"), nil
}

func (a *AgentRunActivities) renderLinkedTicketsContext(ctx context.Context, state *resolvedRunState) (string, error) {
	if len(state.epicTasks) == 0 {
		return "", nil
	}

	taskIDs := make([]string, 0, len(state.epicTasks))
	taskNames := make(map[string]string, len(state.epicTasks))
	for _, task := range state.epicTasks {
		taskIDs = append(taskIDs, task.ID)
		taskNames[task.ID] = task.Name
	}

	tickets, err := a.conversationRepo.ListByLinkedStoryIDs(ctx, state.run.WorkspaceID, taskIDs)
	if err != nil {
		return "", err
	}
	if len(tickets) == 0 {
		return "", nil
	}

	entries := make([]string, 0, len(tickets))
	for idx, ticket := range tickets {
		taskName := ""
		if ticket.LinkedTaskID != nil {
			taskName = taskNames[*ticket.LinkedTaskID]
		}

		header := fmt.Sprintf("- Ticket #%d: %s [status=%s priority=%s]", ticket.DisplayID, ticket.Subject, ticket.Status, ticket.Priority)
		if taskName != "" {
			header += fmt.Sprintf(" linked_task=%q", taskName)
		}
		if ticket.CustomerEmail != nil && strings.TrimSpace(*ticket.CustomerEmail) != "" {
			header += fmt.Sprintf(" customer=%s", *ticket.CustomerEmail)
		}

		entry := header
		messages, err := a.messageRepo.ListByConversation(ctx, state.run.WorkspaceID, ticket.ID, true)
		if err != nil {
			return "", err
		}
		if len(messages) > 0 {
			start := len(messages) - 2
			if start < 0 {
				start = 0
			}
			lines := make([]string, 0, len(messages)-start)
			for _, message := range messages[start:] {
				scope := "public"
				if message.IsInternal {
					scope = "internal"
				}
				lines = append(lines, fmt.Sprintf("  - [%s/%s] %s", message.SenderType, scope, truncatePlanningText(message.Content, 280)))
			}
			entry += "\nRecent messages:\n" + strings.Join(lines, "\n")
		}

		entries = append(entries, entry)
		if idx >= 4 {
			break
		}
	}

	return strings.Join(entries, "\n\n"), nil
}
