package service

// "Ask our team to reply" links in AI-written emails. The link opens a page
// that asks the customer to confirm; only the POST from that page hands the
// conversation to the team, because mail scanners open links on their own.

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const (
	supportTeamReplyLinkTTL     = 30 * 24 * time.Hour
	supportTeamReplyNoteMaxRune = 2000
	// supportTeamReplyRepeatWindow treats a second press soon after the first
	// as the same request, so a double click still shows the confirmation.
	supportTeamReplyRepeatWindow = 10 * time.Minute
	supportTeamReplyReason       = "customer_requested_human"
	supportTeamReplyReasonNote   = "The customer asked for a reply from the team with the link in an AI email."
)

// Team-reply page states.
const (
	SupportTeamReplyStateOpen    = "open"
	SupportTeamReplyStateDone    = "done"
	SupportTeamReplyStateAlready = "already"
	SupportTeamReplyStateExpired = "expired"
)

// ErrSupportTeamReplyLinkInvalid reports a link that was not issued by this
// server or whose conversation no longer exists.
var ErrSupportTeamReplyLinkInvalid = errors.New("this link is not valid")

// SupportTeamReplyPage is what the public team-reply page shows.
type SupportTeamReplyPage struct {
	State         string            `json:"state"`
	Branding      map[string]string `json:"branding"`
	Subject       string            `json:"subject,omitempty"`
	CustomerEmail string            `json:"customer_email,omitempty"`
	ReplyTimeText string            `json:"reply_time_text"`
}

type supportTeamReplyInbox interface {
	GetInstallation(ctx context.Context, workspaceID string) (*model.SupportWidgetInstallation, *model.SupportInboxSettings, error)
	CreateConversationMessage(ctx context.Context, workspaceID, ticketID string, req model.CreateMessageRequest, senderType string, senderUserID, senderAgentID *string, senderDisplayName *string) (*model.SupportMessage, error)
}

type supportTeamReplyConversations interface {
	GetByID(ctx context.Context, workspaceID, id, workspaceMemberID, role string) (*model.SupportConversation, error)
}

type supportTeamReplyWorkspaces interface {
	GetByID(ctx context.Context, id string) (*model.Workspace, error)
}

type supportTeamReplyEscalator interface {
	EscalateToHumanForMessageWithIssue(ctx context.Context, workspaceID, conversationID, messageID, reason, issueKey, issueSummary string, briefs ...SupportHandoffBrief) error
}

// SupportTeamReplyService issues and redeems team-reply links.
type SupportTeamReplyService struct {
	key           []byte
	appBaseURL    string
	conversations supportTeamReplyConversations
	workspaces    supportTeamReplyWorkspaces
	inbox         supportTeamReplyInbox
	escalator     supportTeamReplyEscalator
	now           func() time.Time
}

// NewSupportTeamReplyService derives a signing key used only for these links,
// so a link can never be read as any other token signed with the same secret.
func NewSupportTeamReplyService(secret, appBaseURL string, conversations supportTeamReplyConversations, workspaces supportTeamReplyWorkspaces, inbox supportTeamReplyInbox, escalator supportTeamReplyEscalator) *SupportTeamReplyService {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("helpin:support-team-reply:v1"))
	return &SupportTeamReplyService{
		key:           mac.Sum(nil),
		appBaseURL:    strings.TrimRight(strings.TrimSpace(appBaseURL), "/"),
		conversations: conversations,
		workspaces:    workspaces,
		inbox:         inbox,
		escalator:     escalator,
		now:           time.Now,
	}
}

// TeamReplyURL returns the page link for one conversation, or "" when links
// cannot be issued.
func (s *SupportTeamReplyService) TeamReplyURL(workspaceID, conversationID string) string {
	if s == nil || s.appBaseURL == "" || strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(conversationID) == "" {
		return ""
	}
	return s.appBaseURL + "/team-reply/" + url.PathEscape(s.sign(workspaceID, conversationID, s.now().Add(supportTeamReplyLinkTTL)))
}

// Page describes the link's conversation without changing anything.
func (s *SupportTeamReplyService) Page(ctx context.Context, token string) (*SupportTeamReplyPage, error) {
	link, err := s.verify(token)
	if err != nil {
		return nil, err
	}
	conv, settings, page, err := s.load(ctx, link)
	if err != nil {
		return nil, err
	}
	switch {
	case s.now().After(link.expiresAt):
		page.State = SupportTeamReplyStateExpired
	case supportTeamReplyWithTeam(conv):
		page.State = SupportTeamReplyStateAlready
	default:
		page.State = SupportTeamReplyStateOpen
	}
	page.ReplyTimeText = supportTeamReplyTimeText(*settings, s.now())
	return page, nil
}

// Request hands the conversation to the team without a handoff email, since
// the page confirms it, and adds the customer's optional note to the thread.
func (s *SupportTeamReplyService) Request(ctx context.Context, token, note string) (*SupportTeamReplyPage, error) {
	link, err := s.verify(token)
	if err != nil {
		return nil, err
	}
	conv, settings, page, err := s.load(ctx, link)
	if err != nil {
		return nil, err
	}
	now := s.now()
	page.ReplyTimeText = supportTeamReplyTimeText(*settings, now)
	if now.After(link.expiresAt) {
		page.State = SupportTeamReplyStateExpired
		return page, nil
	}
	if supportTeamReplyWithTeam(conv) {
		page.State = SupportTeamReplyStateAlready
		if conv.CustomerRequestedHumanAt != nil && now.Sub(*conv.CustomerRequestedHumanAt) < supportTeamReplyRepeatWindow {
			page.State = SupportTeamReplyStateDone
		}
		return page, nil
	}
	brief := SupportHandoffBrief{ReasonNote: supportTeamReplyReasonNote, CustomerNotified: true}
	if err := s.escalator.EscalateToHumanForMessageWithIssue(ctx, conv.WorkspaceID, conv.ID, "", supportTeamReplyReason, "", "", brief); err != nil {
		return nil, err
	}
	slog.InfoContext(ctx, "support team reply requested", "workspace_id", conv.WorkspaceID, "conversation_id", conv.ID)
	if note = strings.TrimSpace(note); note != "" {
		if utf8.RuneCountInString(note) > supportTeamReplyNoteMaxRune {
			note = string([]rune(note)[:supportTeamReplyNoteMaxRune])
		}
		if _, err := s.inbox.CreateConversationMessage(ctx, conv.WorkspaceID, conv.ID, model.CreateMessageRequest{Content: note, MessageType: "reply"}, "customer", nil, nil, conv.CustomerName); err != nil {
			// The handoff already happened; losing the note must not undo it.
			slog.ErrorContext(ctx, "support team reply note failed", "workspace_id", conv.WorkspaceID, "conversation_id", conv.ID, "error", err)
		}
	}
	page.State = SupportTeamReplyStateDone
	return page, nil
}

func (s *SupportTeamReplyService) load(ctx context.Context, link supportTeamReplyLink) (*model.SupportConversation, *model.SupportInboxSettings, *SupportTeamReplyPage, error) {
	conv, err := s.conversations.GetByID(ctx, link.workspaceID, link.conversationID, "", model.RoleOwner)
	if err != nil {
		return nil, nil, nil, err
	}
	if conv == nil || conv.AnonymizedAt != nil || conv.MergedIntoID != nil {
		return nil, nil, nil, ErrSupportTeamReplyLinkInvalid
	}
	_, settings, err := s.inbox.GetInstallation(ctx, conv.WorkspaceID)
	if err != nil {
		return nil, nil, nil, err
	}
	if settings == nil {
		defaults := model.DefaultSupportInboxSettings()
		settings = &defaults
	}
	var workspaceName, workspaceLogo, website string
	if s.workspaces != nil {
		if ws, err := s.workspaces.GetByID(ctx, conv.WorkspaceID); err == nil && ws != nil {
			workspaceName, workspaceLogo, website = ws.Name, derefString(ws.LogoURL), derefString(ws.WebsiteURL)
		}
	}
	return conv, settings, &SupportTeamReplyPage{
		Branding:      supportPublicBranding(workspaceName, workspaceLogo, website, *settings),
		Subject:       strings.TrimSpace(conv.Subject),
		CustomerEmail: strings.TrimSpace(derefString(conv.CustomerEmail)),
	}, nil
}

// supportTeamReplyWithTeam reports a conversation a person already owns.
func supportTeamReplyWithTeam(conv *model.SupportConversation) bool {
	return supportConversationHumanOwned(conv) || conv.CustomerRequestedHumanAt != nil || derefString(conv.AIState) == "escalated"
}

// supportTeamReplyTimeText is the reply expectation in the workspace's terms,
// with its outside-hours message when the team is away.
func supportTeamReplyTimeText(settings model.SupportInboxSettings, now time.Time) string {
	availability := resolveSupportAvailability(settings, now)
	if !availability.IsWithinOfficeHours {
		if text := strings.TrimSpace(availability.WidgetAvailability.ReplyTimeText); text != "" {
			return text
		}
	}
	if phrase := shortReplyTimePhrase(availability.WidgetAvailability.ReplyTimePreset, availability.WidgetAvailability.ReplyTimeMinutes); phrase != "" {
		return "We usually reply in " + phrase + "."
	}
	return "We'll reply by email."
}

type supportTeamReplyLink struct {
	workspaceID    string
	conversationID string
	expiresAt      time.Time
}

func (s *SupportTeamReplyService) sign(workspaceID, conversationID string, expiresAt time.Time) string {
	payload := strings.Join([]string{"1", workspaceID, conversationID, strconv.FormatInt(expiresAt.Unix(), 10)}, "|")
	encoded := base64.RawURLEncoding.EncodeToString([]byte(payload))
	return encoded + "." + base64.RawURLEncoding.EncodeToString(s.mac(encoded))
}

func (s *SupportTeamReplyService) verify(token string) (supportTeamReplyLink, error) {
	encoded, signature, ok := strings.Cut(strings.TrimSpace(token), ".")
	if !ok || len(token) > 512 {
		return supportTeamReplyLink{}, ErrSupportTeamReplyLinkInvalid
	}
	got, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil || !hmac.Equal(got, s.mac(encoded)) {
		return supportTeamReplyLink{}, ErrSupportTeamReplyLinkInvalid
	}
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return supportTeamReplyLink{}, ErrSupportTeamReplyLinkInvalid
	}
	parts := strings.Split(string(payload), "|")
	if len(parts) != 4 || parts[0] != "1" || parts[1] == "" || parts[2] == "" {
		return supportTeamReplyLink{}, ErrSupportTeamReplyLinkInvalid
	}
	expires, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		return supportTeamReplyLink{}, ErrSupportTeamReplyLinkInvalid
	}
	return supportTeamReplyLink{workspaceID: parts[1], conversationID: parts[2], expiresAt: time.Unix(expires, 0)}, nil
}

func (s *SupportTeamReplyService) mac(encoded string) []byte {
	mac := hmac.New(sha256.New, s.key)
	mac.Write([]byte(encoded))
	return mac.Sum(nil)
}
