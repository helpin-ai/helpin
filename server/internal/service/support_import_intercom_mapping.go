package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/intercom"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// importedMessageMetadata marks messages copied from another help desk, so
// live-only behavior such as webhooks skips them.
const importedMessageMetadata = `{"imported_from":"intercom"}`

// intercomConversationImporter maps one page of Intercom conversations,
// caching contacts, teammates and tags across the page.
type intercomConversationImporter struct {
	svc      *SupportImportService
	client   *intercom.Client
	job      model.SupportImportJob
	contacts map[string]*intercom.Contact
	users    map[string]string // lower-case email -> Helpin user ID
	tags     map[string]string // lower-case name -> tag ID
}

func newIntercomConversationImporter(svc *SupportImportService, client *intercom.Client, job model.SupportImportJob) *intercomConversationImporter {
	return &intercomConversationImporter{svc: svc, client: client, job: job, contacts: map[string]*intercom.Contact{}}
}

func (m *intercomConversationImporter) loadTeammates(ctx context.Context) error {
	if m.users != nil {
		return nil
	}
	m.users = map[string]string{}
	if m.svc.inbox.workspaceRepo == nil {
		return nil
	}
	members, err := m.svc.inbox.workspaceRepo.ListAssignableMembers(ctx, m.job.WorkspaceID)
	if err != nil {
		return err
	}
	for _, member := range members {
		if member.UserID != nil && member.Email != "" {
			m.users[strings.ToLower(member.Email)] = *member.UserID
		}
	}
	return nil
}

// tagID returns the workspace tag with this name, creating it when missing.
func (m *intercomConversationImporter) tagID(ctx context.Context, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || m.svc.inbox.tagRepo == nil {
		return "", nil
	}
	if m.tags == nil {
		m.tags = map[string]string{}
		tags, err := m.svc.inbox.tagRepo.ListByWorkspace(ctx, m.job.WorkspaceID)
		if err != nil {
			return "", err
		}
		for _, tag := range tags {
			m.tags[strings.ToLower(tag.Name)] = tag.ID
		}
	}
	if id, ok := m.tags[strings.ToLower(name)]; ok {
		return id, nil
	}
	tag := &model.SupportTag{ID: uuid.NewString(), WorkspaceID: m.job.WorkspaceID, Name: truncateWebhookText(name, 60)}
	if err := m.svc.inbox.tagRepo.Create(ctx, tag); err != nil {
		return "", fmt.Errorf("create tag %q: %w", name, err)
	}
	m.tags[strings.ToLower(name)] = tag.ID
	return tag.ID, nil
}

func (m *intercomConversationImporter) contact(ctx context.Context, id string) (*intercom.Contact, error) {
	if contact, ok := m.contacts[id]; ok {
		return contact, nil
	}
	contact, err := m.client.GetContact(ctx, id)
	if errors.Is(err, intercom.ErrNotFound) {
		m.contacts[id] = nil
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	m.contacts[id] = contact
	return contact, nil
}

func unixTime(seconds int64) time.Time { return time.Unix(seconds, 0).UTC() }

// intercomSender maps an Intercom author to a Helpin sender.
func (m *intercomConversationImporter) intercomSender(author intercom.Author, customerName *string) (senderType string, userID *string, name *string) {
	display := strings.TrimSpace(author.Name)
	switch author.Type {
	case "admin", "team":
		if id, ok := m.users[strings.ToLower(strings.TrimSpace(author.Email))]; ok && author.Email != "" {
			userID = &id
		}
		if display == "" {
			display = "Teammate"
		}
		return "user", userID, &display
	case "bot":
		if display == "" {
			display = "Intercom bot"
		}
		return "ai", nil, &display
	default:
		if display == "" && customerName != nil {
			display = *customerName
		}
		if display == "" {
			return "customer", nil, nil
		}
		return "customer", nil, &display
	}
}

// importConversation copies one Intercom conversation with its public replies
// and internal notes. Assignments, state changes and attachments are not
// copied.
func (m *intercomConversationImporter) importConversation(ctx context.Context, externalID string) error {
	source, err := m.client.GetConversation(ctx, externalID)
	if err != nil {
		return err
	}
	if err := m.loadTeammates(ctx); err != nil {
		return err
	}
	var customerName, customerEmail *string
	if len(source.Contacts.Contacts) > 0 {
		contact, err := m.contact(ctx, source.Contacts.Contacts[0].ID)
		if err != nil {
			return err
		}
		if contact != nil {
			customerName, customerEmail = trimmedPtr(contact.Name), trimmedPtr(contact.Email)
		}
	}
	if customerEmail == nil && isIntercomCustomer(source.Source.Author.Type) {
		customerEmail, customerName = trimmedPtr(&source.Source.Author.Email), firstNonEmptyPtr(customerName, trimmedPtr(&source.Source.Author.Name))
	}

	conversation := &model.SupportConversation{
		ID: uuid.NewString(), WorkspaceID: m.job.WorkspaceID, MailboxID: m.job.MailboxID,
		Subject: intercomSubject(source), Status: model.SupportConversationStatusOpen, Priority: "medium",
		Channel: "widget", Source: model.SupportImportSourceIntercom, CustomerName: customerName, CustomerEmail: customerEmail,
		CreatedAt: unixTime(source.CreatedAt), UpdatedAt: unixTime(max(source.UpdatedAt, source.CreatedAt)),
		HumanTakeover: boolPtr(true),
	}
	if source.Source.Type == "email" {
		conversation.Channel = "email"
	}
	if source.Priority == "priority" {
		conversation.Priority = "high"
	}

	messages := make([]model.SupportMessage, 0, len(source.Parts.Parts)+1)
	add := func(at int64, author intercom.Author, body string, internal bool) {
		body = strings.TrimSpace(body)
		if body == "" {
			return
		}
		senderType, userID, name := m.intercomSender(author, customerName)
		message := model.SupportMessage{
			ID: uuid.NewString(), WorkspaceID: m.job.WorkspaceID, ConversationID: conversation.ID,
			SenderType: senderType, SenderUserID: userID, SenderDisplayName: name, Content: body,
			MessageType: "reply", IsInternal: internal, Metadata: importedMessageMetadata, CreatedAt: unixTime(at), UpdatedAt: unixTime(at),
		}
		if internal {
			message.MessageType = "note"
		}
		messages = append(messages, message)
		conversation.ApplyMessageProjection(message)
	}
	add(source.CreatedAt, source.Source.Author, source.Source.Body, false)
	for _, part := range source.Parts.Parts {
		if part.Body == nil {
			continue
		}
		switch part.PartType {
		case "note":
			add(part.CreatedAt, part.Author, *part.Body, true)
		case "comment", "assignment", "open", "close", "snoozed", "away_mode_assignment", "default_assignment":
			// Replies sent together with an action still reached the customer.
			add(part.CreatedAt, part.Author, *part.Body, false)
		}
	}
	if len(messages) == 0 {
		return errors.New("the conversation has no messages to import")
	}
	if source.State == "closed" {
		closedAt := conversation.UpdatedAt
		if source.Statistics != nil && source.Statistics.LastCloseAt != nil && *source.Statistics.LastCloseAt > 0 {
			closedAt = unixTime(*source.Statistics.LastCloseAt)
		}
		conversation.Status = model.SupportConversationStatusResolved
		conversation.ResolvedAt, conversation.ClosedAt = &closedAt, &closedAt
		resolvedByHuman := model.SupportConversationFlowStateResolvedByHuman
		conversation.FlowState = &resolvedByHuman
		conversation.RecomputeHumanAttention()
	}
	// The team has seen the history; only new activity is unread.
	last := messages[len(messages)-1].CreatedAt
	conversation.TeamLastSeenAt = &last

	var tagIDs []string
	for _, tag := range source.Tags.Tags {
		id, err := m.tagID(ctx, tag.Name)
		if err != nil {
			return err
		}
		if id != "" {
			tagIDs = append(tagIDs, id)
		}
	}
	jobID := m.job.ID
	record := model.SupportImportRecord{WorkspaceID: m.job.WorkspaceID, Source: model.SupportImportSourceIntercom, ExternalID: externalID,
		ConversationID: conversation.ID, JobID: &jobID, CreatedAt: m.svc.now()}
	return m.svc.repo.ImportConversation(ctx, record, conversation, messages, tagIDs)
}

func isIntercomCustomer(authorType string) bool {
	return authorType == "user" || authorType == "lead" || authorType == "contact"
}

func intercomSubject(source *intercom.Conversation) string {
	for _, candidate := range []string{derefString(source.Title), source.Source.Subject, source.Source.Body} {
		if subject := strings.Join(strings.Fields(candidate), " "); subject != "" {
			return truncateWebhookText(subject, 120)
		}
	}
	return "Intercom conversation"
}

func trimmedPtr(value *string) *string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	return &trimmed
}

func firstNonEmptyPtr(values ...*string) *string {
	for _, value := range values {
		if value != nil && *value != "" {
			return value
		}
	}
	return nil
}
