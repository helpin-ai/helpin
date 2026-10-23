package service

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// supportSharedLinkPattern finds absolute links in reply bodies, whether the
// body is Markdown, HTML, or plain text.
var supportSharedLinkPattern = regexp.MustCompile(`https?://[^\s"'<>()\[\]]+`)

// maxSupportSharedArticleLookups bounds article lookups for one conversation.
const maxSupportSharedArticleLookups = 25

// supportSharedArticleLink is an article link found in a team reply.
type supportSharedArticleLink struct {
	PublicID string
	URL      string
	Message  model.SupportMessage
}

// ListConversationSharedItems returns the help-center articles and files the
// team sent to the customer in a conversation, newest first. Internal notes,
// customer messages, and system events are excluded.
func (s *SupportInboxService) ListConversationSharedItems(ctx context.Context, workspaceID, conversationID string) ([]model.SupportSharedItem, error) {
	conversation, err := s.loadConversationAccessible(ctx, workspaceID, conversationID)
	if err != nil {
		return nil, err
	}
	if conversation == nil {
		return nil, fmt.Errorf("conversation not found")
	}

	messages, err := s.messageRepo.ListByConversation(ctx, workspaceID, conversationID, false)
	if err != nil {
		return nil, err
	}
	replies := teamRepliesForSharedItems(messages)
	items := make([]model.SupportSharedItem, 0)
	if len(replies) == 0 {
		return items, nil
	}

	items = append(items, s.sharedArticles(ctx, workspaceID, replies)...)
	files, err := s.sharedFiles(ctx, replies)
	if err != nil {
		return nil, err
	}
	items = append(items, files...)
	sort.SliceStable(items, func(i, j int) bool { return items[i].SharedAt.After(items[j].SharedAt) })
	return items, nil
}

// teamRepliesForSharedItems keeps customer-visible replies written by
// teammates, agents, or AI, newest first.
func teamRepliesForSharedItems(messages []model.SupportMessage) []model.SupportMessage {
	replies := make([]model.SupportMessage, 0, len(messages))
	for _, message := range messages {
		if message.IsInternal || message.SenderType == "customer" {
			continue
		}
		if message.MessageType != "" && message.MessageType != "reply" {
			continue
		}
		replies = append(replies, message)
	}
	sort.SliceStable(replies, func(i, j int) bool { return replies[i].CreatedAt.After(replies[j].CreatedAt) })
	return replies
}

// sharedArticleLinks returns the latest link to each distinct article public
// ID, scanning replies newest first.
func sharedArticleLinks(replies []model.SupportMessage) []supportSharedArticleLink {
	seen := make(map[string]struct{})
	var links []supportSharedArticleLink
	for _, reply := range replies {
		for _, raw := range supportSharedLinkPattern.FindAllString(reply.Content, -1) {
			link := strings.TrimRight(raw, ".,;:!?*_`")
			publicID, ok := articlePublicIDFromLink(link)
			if !ok {
				continue
			}
			if _, dup := seen[publicID]; dup {
				continue
			}
			seen[publicID] = struct{}{}
			links = append(links, supportSharedArticleLink{PublicID: publicID, URL: link, Message: reply})
		}
	}
	return links
}

// articlePublicIDFromLink reads the article public ID from the last path
// segment of a help-center article URL (`{slug}-{publicID}`).
func articlePublicIDFromLink(link string) (string, bool) {
	parsed, err := url.Parse(link)
	if err != nil {
		return "", false
	}
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	last := segments[len(segments)-1]
	if last == "" {
		return "", false
	}
	_, publicID, ok := parseDocsHelpcenterArticleKey(last)
	return publicID, ok
}

// sharedArticles resolves article links against the workspace's published
// help-center articles. Links that do not resolve are not reported.
func (s *SupportInboxService) sharedArticles(ctx context.Context, workspaceID string, replies []model.SupportMessage) []model.SupportSharedItem {
	if s.docsHelpcenterRepo == nil {
		return nil
	}
	links := sharedArticleLinks(replies)
	if len(links) > maxSupportSharedArticleLookups {
		links = links[:maxSupportSharedArticleLookups]
	}
	items := make([]model.SupportSharedItem, 0, len(links))
	for _, link := range links {
		doc, _, _, err := s.docsHelpcenterRepo.GetPublicArticleByPublicID(ctx, workspaceID, link.PublicID)
		if err != nil {
			slog.WarnContext(ctx, "resolve shared help-center article", "error", err, "workspace_id", workspaceID, "public_id", link.PublicID)
			continue
		}
		if doc == nil {
			continue
		}
		items = append(items, model.SupportSharedItem{
			Kind:      model.SupportSharedItemKindDoc,
			ID:        doc.ID,
			Title:     doc.Title,
			URL:       link.URL,
			MessageID: link.Message.ID,
			SharedAt:  link.Message.CreatedAt,
		})
	}
	return items
}

// sharedFiles lists files attached to team replies, excluding inline email
// images.
func (s *SupportInboxService) sharedFiles(ctx context.Context, replies []model.SupportMessage) ([]model.SupportSharedItem, error) {
	if s.attachmentService == nil {
		return nil, nil
	}
	ids := make([]string, len(replies))
	for i, reply := range replies {
		ids[i] = reply.ID
	}
	byMessage, err := s.attachmentService.PayloadsByMessage(ctx, ids)
	if err != nil {
		return nil, err
	}
	var items []model.SupportSharedItem
	for _, reply := range replies {
		for _, attachment := range byMessage[reply.ID] {
			if attachment.ContentID != "" {
				continue
			}
			items = append(items, model.SupportSharedItem{
				Kind:      model.SupportSharedItemKindFile,
				ID:        attachment.ID,
				Title:     attachment.FileName,
				URL:       attachment.URL,
				FileType:  attachment.FileType,
				FileSize:  attachment.FileSize,
				MessageID: reply.ID,
				SharedAt:  reply.CreatedAt,
			})
		}
	}
	return items, nil
}
