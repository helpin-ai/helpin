package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// Portal copy errors returned to customers.
var (
	ErrPortalCCDisabled = errors.New("copying people on requests is turned off")
	ErrPortalCCDenied   = errors.New("only the requester can change who is copied")
	ErrPortalCCInvalid  = errors.New("invalid copy list")
)

// PortalScopeCopied lists requests that copy the signed-in customer.
const PortalScopeCopied = "copied"

// PortalCCLimit bounds the addresses a requester copies on one request.
const PortalCCLimit = 10

// Copy notices carry text the requester wrote, so they are capped: one per
// address and request a day, three per address and twenty per requester a
// day, and a workspace cap an hour.
const (
	portalCCNoticeInterval      = 24 * time.Hour
	portalCCNoticesPerAddress   = 3
	portalCCNoticesPerRequester = 20
	portalCCNoticesPerWorkspace = 100
)

func invalidPortalCC(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrPortalCCInvalid, fmt.Sprintf(format, args...))
}

// CopiedRequests lists the requests that copy the signed-in customer.
func (s *CustomerPortalService) CopiedRequests(ctx context.Context, access *PortalAccess, status string) ([]model.SupportPortalRequest, error) {
	if access.Workspace.Settings.PortalCCMode == "" {
		return []model.SupportPortalRequest{}, nil
	}
	requests, err := s.repo.ListCopiedRequests(ctx, access.Workspace.ID, access.Identity.ID, access.Identity.Email, status)
	if err != nil {
		return nil, err
	}
	s.attachReplyExpectedBy(ctx, &access.Workspace, requests)
	for i := range requests {
		// Read state belongs to the requester.
		requests[i].Unread = false
	}
	return requests, nil
}

// PortalCCOptions tells the portal whether and whom the customer may copy.
type PortalCCOptions struct {
	Mode       string                       `json:"mode"`
	Colleagues []repository.PortalColleague `json:"colleagues"`
	Domains    []string                     `json:"domains"`
}

// CCOptions returns the copy setting and, in colleagues mode, the people
// and domains the customer may copy.
func (s *CustomerPortalService) CCOptions(ctx context.Context, access *PortalAccess) (*PortalCCOptions, error) {
	options := &PortalCCOptions{Mode: access.Workspace.Settings.PortalCCMode, Colleagues: []repository.PortalColleague{}, Domains: []string{}}
	if options.Mode == "" || access.Session.CRMContactID == nil {
		return options, nil
	}
	colleagues, domains, err := s.repo.PortalColleagues(ctx, access.Workspace.ID, *access.Session.CRMContactID)
	if err != nil {
		return nil, err
	}
	options.Colleagues = colleagues
	if domains != nil {
		options.Domains = domains
	}
	return options, nil
}

// normalizeCC validates a requester's copy list: addresses, the limit, the
// workspace's copy mode, and never the requester. previous holds addresses
// already copied, which stay allowed even when the mode would not add them
// now (a teammate may have copied them).
func (s *CustomerPortalService) normalizeCC(ctx context.Context, access *PortalAccess, requesterEmail string, emails, previous []string) ([]string, error) {
	mode := access.Workspace.Settings.PortalCCMode
	if mode == "" {
		if len(emails) == 0 {
			return nil, nil
		}
		return nil, ErrPortalCCDisabled
	}
	seen := map[string]bool{strings.ToLower(strings.TrimSpace(requesterEmail)): true}
	var result []string
	for _, raw := range emails {
		address, ok := normalizePortalEmail(raw)
		if !ok {
			return nil, invalidPortalCC("%q is not an email address", strings.TrimSpace(raw))
		}
		if seen[address] {
			continue
		}
		seen[address] = true
		result = append(result, address)
	}
	if len(result) > PortalCCLimit {
		return nil, invalidPortalCC("copy at most %d people", PortalCCLimit)
	}
	if mode != model.SupportPortalCCColleagues {
		return result, nil
	}
	options, err := s.CCOptions(ctx, access)
	if err != nil {
		return nil, err
	}
	for _, address := range result {
		if slices.ContainsFunc(previous, func(existing string) bool { return strings.EqualFold(existing, address) }) {
			continue
		}
		domain := address[strings.LastIndex(address, "@")+1:]
		colleague := slices.ContainsFunc(options.Colleagues, func(c repository.PortalColleague) bool { return c.Email == address })
		if !colleague && !slices.Contains(options.Domains, domain) {
			return nil, invalidPortalCC("%s is not a colleague at your company", address)
		}
	}
	return result, nil
}

// SetRequestCC replaces who a request copies. Only the requester changes it;
// newly copied people get one email about the request.
// base is the list the customer started from, when known; only their own
// additions and removals relative to it are applied.
func (s *CustomerPortalService) SetRequestCC(ctx context.Context, access *PortalAccess, reference string, emails []string, base []string) (*PortalRequestDetail, error) {
	if access.Workspace.Settings.PortalCCMode == "" {
		return nil, ErrPortalCCDisabled
	}
	request, err := s.findRequest(ctx, access, reference)
	if err != nil {
		return nil, err
	}
	if request == nil {
		return nil, ErrPortalRequestNotFound
	}
	if !request.Own {
		return nil, ErrPortalCCDenied
	}
	conv := request.Conversation
	if conv.AnonymizedAt != nil {
		return nil, ErrPortalReplyUnavailable
	}
	previous := []string(conv.EmailCC)
	if base != nil {
		previous = normalizeSupportEmailList(base)
	}
	next, err := s.normalizeCC(ctx, access, derefString(conv.CustomerEmail), emails, append(previous, conv.EmailCC...))
	if err != nil {
		return nil, err
	}
	added := supportEmailListDifference(next, previous)
	removed := supportEmailListDifference(previous, next)
	if len(added) == 0 && len(removed) == 0 {
		return s.RequestDetail(ctx, access, reference)
	}
	// Apply only this change to the stored list, so a Cc an inbound email or
	// a teammate added meanwhile is kept.
	stored, err := s.repo.UpdateConversationCC(ctx, access.Workspace.ID, conv.ID, added, removed)
	if err != nil {
		return nil, err
	}
	conv.EmailCC = model.DocsStringArray(stored)
	s.recordPortalCCChange(ctx, access, conv, added, removed)
	for _, address := range added {
		s.sendCopyNotice(ctx, access, conv, reference, address)
	}
	return s.RequestDetail(ctx, access, reference)
}

// recordPortalCCChange notes a customer's copy change on the timeline and in
// the portal audit log.
func (s *CustomerPortalService) recordPortalCCChange(ctx context.Context, access *PortalAccess, conv *model.SupportConversation, added, removed []string) {
	actor := derefString(s.portalReplySenderName(ctx, access, false))
	changes := make([]string, 0, len(added)+len(removed))
	for _, address := range added {
		changes = append(changes, fmt.Sprintf("%s added %s to Cc.", actor, address))
	}
	for _, address := range removed {
		changes = append(changes, fmt.Sprintf("%s removed %s from Cc.", actor, address))
	}
	message := &model.SupportMessage{
		WorkspaceID: conv.WorkspaceID, ConversationID: conv.ID, SenderType: "system", SenderDisplayName: &actor,
		Content: strings.Join(changes, " "), IsInternal: true, MessageType: "system",
		SystemEventType: model.SupportSystemEventTypeStrPtr(model.SystemEventEmailRecipientsUpdated),
	}
	if err := s.inbox.messageRepo.Create(ctx, message); err != nil {
		slog.ErrorContext(ctx, "portal cc timeline event failed", "workspace_id", conv.WorkspaceID, "conversation_id", conv.ID, "error", err)
	} else if s.inbox.wsPublisher != nil {
		s.inbox.wsPublisher.Publish(websocket.SupportMessageEvent(conv.WorkspaceID, message, ""))
	}
	metadata, _ := json.Marshal(map[string]any{"added": added, "removed": removed})
	identityID := access.Identity.ID
	if err := s.repo.CreateAuditEvent(ctx, &model.SupportPortalAuditEvent{
		ID: uuid.NewString(), WorkspaceID: conv.WorkspaceID, ConversationID: conv.ID, PortalIdentityID: &identityID,
		ActorType: model.SupportPortalActorCustomer, EventType: model.SupportPortalAuditCCUpdated, Metadata: string(metadata), OccurredAt: time.Now().UTC(),
	}); err != nil {
		slog.WarnContext(ctx, "portal cc audit failed", "workspace_id", conv.WorkspaceID, "conversation_id", conv.ID, "error", err)
	}
}

// sendCopyNotice emails a newly copied person about the request: who copied
// them, the latest message, and a portal link when they may sign in.
// Repeat notices for the same request and address within a day are skipped.
func (s *CustomerPortalService) sendCopyNotice(ctx context.Context, access *PortalAccess, conv *model.SupportConversation, reference, address string) {
	if s.sender == nil {
		return
	}
	now := time.Now().UTC()
	counts, err := s.repo.CopyNoticeCounts(ctx, access.Workspace.ID, conv.ID, access.Identity.ID, address, now.Add(-portalCCNoticeInterval), now.Add(-time.Hour))
	if err != nil {
		slog.ErrorContext(ctx, "portal copy notice throttle lookup failed", "workspace_id", access.Workspace.ID, "error", err)
		return
	}
	if counts.ForRequest > 0 || counts.ForAddress >= portalCCNoticesPerAddress || counts.ForRequester >= portalCCNoticesPerRequester || counts.ForWorkspace >= portalCCNoticesPerWorkspace {
		slog.InfoContext(ctx, "portal copy notice throttled", "workspace_id", access.Workspace.ID, "conversation_id", conv.ID)
		return
	}
	// Blocked people get no mail from the portal.
	contacts, err := s.repo.ContactsForEmail(ctx, access.Workspace.ID, address)
	if err != nil {
		slog.ErrorContext(ctx, "portal copy notice contact lookup failed", "workspace_id", access.Workspace.ID, "error", err)
		return
	}
	for _, contact := range contacts {
		if derefString(contact.PortalAccess) == model.PortalAccessBlocked {
			return
		}
	}
	eligibility, err := s.eligibility(ctx, s.repo, &access.Workspace, address)
	if err != nil {
		slog.ErrorContext(ctx, "portal copy notice eligibility failed", "workspace_id", access.Workspace.ID, "error", err)
		return
	}
	link := ""
	if eligibility.Eligible {
		link = s.address(ctx, access.Workspace.ID, access.Workspace.Slug).request(reference)
	}
	actor := derefString(s.portalReplySenderName(ctx, access, false))
	latest := s.latestPortalMessage(ctx, access.Workspace.ID, conv.ID)
	subject, htmlBody, textBody := portalCopyNoticeEmail(actor, conv.DisplayID, conv.Subject, latest, link)
	if err := s.sender.SendEmail(address, subject, htmlBody, textBody); err != nil {
		slog.ErrorContext(ctx, "portal copy notice delivery failed", "workspace_id", access.Workspace.ID, "error", err)
		return
	}
	metadata, _ := json.Marshal(map[string]string{"notified": address})
	requester := access.Identity.ID
	if err := s.repo.CreateAuditEvent(ctx, &model.SupportPortalAuditEvent{
		ID: uuid.NewString(), WorkspaceID: access.Workspace.ID, ConversationID: conv.ID, PortalIdentityID: &requester,
		ActorType: model.SupportPortalActorSystem, EventType: model.SupportPortalAuditCCNotified, Metadata: string(metadata), OccurredAt: now,
	}); err != nil {
		slog.WarnContext(ctx, "portal copy notice audit failed", "workspace_id", access.Workspace.ID, "error", err)
	}
}

// latestPortalMessage is the newest message the portal shows, as text.
func (s *CustomerPortalService) latestPortalMessage(ctx context.Context, workspaceID, conversationID string) string {
	messages, err := s.inbox.ListPortalConversationMessages(ctx, workspaceID, conversationID)
	if err != nil {
		return ""
	}
	for i := len(messages) - 1; i >= 0; i-- {
		if portalMessageVisible(&messages[i]) {
			return projectPortalMessage(&messages[i], "").Content
		}
	}
	return ""
}

func portalCopyNoticeEmail(actor string, number int, subject, latest, link string) (string, string, string) {
	title := fmt.Sprintf("%s copied you on request #%d", actor, number)
	quote := latest
	if len([]rune(quote)) > 1200 {
		quote = string([]rune(quote)[:1200]) + "…"
	}
	var htmlBody, textBody strings.Builder
	htmlBody.WriteString(`<p>` + html.EscapeString(title) + `: <strong>` + html.EscapeString(subject) + `</strong></p>`)
	textBody.WriteString(title + ": " + subject + "\n\n")
	if quote != "" {
		htmlBody.WriteString(`<blockquote style="margin:0 0 16px;padding-left:12px;border-left:3px solid #ddd;white-space:pre-wrap">` + html.EscapeString(quote) + `</blockquote>`)
		textBody.WriteString(quote + "\n\n")
	}
	htmlBody.WriteString(`<p>You will get the team's replies by email.`)
	textBody.WriteString("You will get the team's replies by email.")
	if link != "" {
		htmlBody.WriteString(` You can also <a href="` + html.EscapeString(link) + `">follow the request in the support portal</a>.`)
		textBody.WriteString(" Follow the request in the support portal: " + link)
	}
	htmlBody.WriteString(`</p>`)
	return title, htmlBody.String(), textBody.String()
}
