package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// Customer portal errors. Handlers translate them into public responses.
var (
	ErrPortalUnavailable             = errors.New("portal unavailable")
	ErrPortalAuthInvalid             = errors.New("invalid portal authentication")
	ErrPortalIntakeDisabled          = errors.New("portal intake is disabled")
	ErrPortalAnonymousIntakeDisabled = errors.New("anonymous portal intake is disabled")
	ErrPortalRequestInvalid          = errors.New("subject and description are required")
	ErrPortalRequestNotFound         = errors.New("portal request not found")
	ErrPortalReplyInvalid            = errors.New("enter a reply or attach a file")
	ErrPortalReplyUnavailable        = errors.New("this request cannot receive replies")
	ErrPortalAttachmentsUnavailable  = errors.New("portal file uploads are disabled")
	ErrPortalAttachmentsInvalid      = errors.New("one or more attachments are unavailable")
	ErrPortalTooManyAttachments      = fmt.Errorf("attach up to %d files per message", PortalMaxAttachmentsPerMessage)
)

const (
	portalLinkTTL           = 15 * time.Minute
	portalSessionTTL        = 7 * 24 * time.Hour
	portalIntakeTTL         = time.Hour
	portalReconcileInterval = 5 * time.Minute
	// Sign-in links are throttled per address and per workspace so the public
	// endpoints cannot be used to flood an inbox or the sending domain.
	portalLinkCooldown           = time.Minute
	portalLinksPerAddressPerHour = 5
	portalLinksPerWorkspaceHour  = 300
)

// PortalMaxAttachmentsPerMessage caps the files on one portal request or reply.
const PortalMaxAttachmentsPerMessage = 10

// PortalSessionTTL is the lifetime of a portal session cookie.
const PortalSessionTTL = portalSessionTTL

type portalEmailSender interface {
	SendEmail(to, subject, htmlBody, textBody string) error
}

// CustomerPortalService serves the public customer portal: magic-link
// sessions, anonymous intake, and request projections over support
// conversations. Every method takes a workspace resolved by ResolveWorkspace
// or an access resolved by Authenticate, so settings and sessions are loaded
// once per request.
type CustomerPortalService struct {
	repo    *repository.CustomerPortalRepository
	inbox   *SupportInboxService
	sender  portalEmailSender
	baseURL string
	// helpcenterDomain, when set, serves portals on help centers at /requests.
	helpcenterDomain string
}

// NewCustomerPortalService creates the customer portal service. It registers
// its sender's readiness with the inbox so anonymous intake settings are
// validated against the sender that actually emails submitters.
func NewCustomerPortalService(repo *repository.CustomerPortalRepository, inbox *SupportInboxService, sender portalEmailSender, baseURL string) *CustomerPortalService {
	s := &CustomerPortalService{repo: repo, inbox: inbox, sender: sender, baseURL: baseURL}
	if inbox != nil {
		inbox.SetPortalConfirmationEmailReady(s.senderReady)
	}
	return s
}

// senderReady reports whether portal emails can be sent. A sender that can
// be reconfigured at runtime reports its current state.
func (s *CustomerPortalService) senderReady() bool {
	if s.sender == nil {
		return false
	}
	if configurable, ok := s.sender.(interface{ Configured() bool }); ok {
		return configurable.Configured()
	}
	return true
}

// PortalWorkspace is a workspace whose portal is enabled.
type PortalWorkspace struct {
	ID      string
	Slug    string
	Name    string
	LogoURL string
	// WebsiteURL supplies the favicon fallback the app uses for workspaces.
	WebsiteURL string
	Settings   model.SupportInboxSettings
}

// PortalAccess is an authenticated portal session in one workspace.
type PortalAccess struct {
	Workspace PortalWorkspace
	Session   model.PortalSession
	Identity  model.SupportPortalIdentity
}

// PortalConfiguration is the public portal configuration.
type PortalConfiguration struct {
	Enabled                bool                   `json:"enabled"`
	IntakeEnabled          bool                   `json:"intake_enabled"`
	AnonymousIntakeEnabled bool                   `json:"anonymous_intake_enabled"`
	FileUploadsEnabled     bool                   `json:"file_uploads_enabled"`
	Attachments            PortalAttachmentPolicy `json:"attachments"`
	Branding               map[string]string      `json:"branding"`
	// PublicURL is where this portal is served; the app's /portal path
	// redirects there.
	PublicURL string `json:"public_url"`
}

// PortalAttachmentPolicy is the upload rules the portal UI applies before
// uploading. The server enforces the same rules.
type PortalAttachmentPolicy struct {
	SupportAttachmentPolicy
	MaxFiles int `json:"max_files"`
}

// ResolveWorkspace returns the enabled portal for slug. Unknown slugs and
// disabled portals are indistinguishable to callers.
func (s *CustomerPortalService) ResolveWorkspace(ctx context.Context, slug string) (*PortalWorkspace, error) {
	ws, err := s.repo.WorkspaceBySlug(ctx, slug)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			slog.ErrorContext(ctx, "portal workspace lookup failed", "error", err)
		}
		return nil, ErrPortalUnavailable
	}
	_, settings, err := s.inbox.GetInstallation(ctx, ws.ID)
	if err != nil {
		slog.ErrorContext(ctx, "portal settings lookup failed", "workspace_id", ws.ID, "error", err)
		return nil, ErrPortalUnavailable
	}
	if settings == nil || !settings.PortalEnabled {
		return nil, ErrPortalUnavailable
	}
	return &PortalWorkspace{ID: ws.ID, Slug: ws.Slug, Name: ws.Name, LogoURL: derefString(ws.LogoURL), WebsiteURL: derefString(ws.WebsiteURL), Settings: *settings}, nil
}

// Configuration returns the public configuration for an enabled portal.
func (s *CustomerPortalService) Configuration(ctx context.Context, ws *PortalWorkspace) PortalConfiguration {
	return PortalConfiguration{
		Enabled:                true,
		IntakeEnabled:          ws.Settings.PortalIntakeEnabled,
		AnonymousIntakeEnabled: s.anonymousIntakeEnabled(ws),
		FileUploadsEnabled:     ws.Settings.FileUploadsEnabled,
		Attachments:            PortalAttachmentPolicy{SupportAttachmentPolicy: CurrentSupportAttachmentPolicy(), MaxFiles: PortalMaxAttachmentsPerMessage},
		Branding:               s.branding(ctx, ws),
		PublicURL:              s.PublicURL(ctx, ws),
	}
}

// branding presents the portal as the workspace's own support, using the
// same name, logo, and colour customers see in the chat widget, falling
// back to the workspace's name and logo.
func (s *CustomerPortalService) branding(ctx context.Context, ws *PortalWorkspace) map[string]string {
	name := strings.TrimSpace(ws.Settings.WidgetName)
	if name == "" {
		name = strings.TrimSpace(ws.Name)
	}
	if name == "" {
		name = "Customer support"
	}
	branding := map[string]string{"name": name, "assistant_name": s.assistantName(ctx, ws)}
	// As in the app: the widget logo, the workspace logo, then the site's favicon.
	for _, logo := range []string{ws.Settings.LogoURL, ws.LogoURL, portalFaviconURL(ws.WebsiteURL)} {
		if logo = strings.TrimSpace(logo); logo != "" {
			branding["logo_url"] = logo
			break
		}
	}
	if color := strings.TrimSpace(ws.Settings.BrandColor); color != "" {
		branding["brand_color"] = color
	}
	return branding
}

// portalFaviconURL mirrors the app's favicon helper: the website's host,
// without www, through Google's favicon service at 128px.
func portalFaviconURL(website string) string {
	website = strings.TrimSpace(website)
	if website == "" {
		return ""
	}
	if !strings.Contains(website, "://") {
		website = "https://" + website
	}
	parsed, err := url.Parse(website)
	if err != nil || parsed.Hostname() == "" {
		return ""
	}
	host := strings.TrimPrefix(strings.ToLower(parsed.Hostname()), "www.")
	return "https://www.google.com/s2/favicons?domain=" + url.QueryEscape(host) + "&sz=128"
}

// defaultPortalAssistantName labels AI replies when no agent name is set.
const defaultPortalAssistantName = "AI assistant"

// assistantName is the one name customers see on every AI reply: the portal
// AI agent, else the support AI agent, else a neutral label.
func (s *CustomerPortalService) assistantName(ctx context.Context, ws *PortalWorkspace) string {
	if s.inbox == nil || s.inbox.agentRepo == nil {
		return defaultPortalAssistantName
	}
	for _, id := range []*string{ws.Settings.PortalAIAgentID, ws.Settings.AIAgentID} {
		if id == nil || strings.TrimSpace(*id) == "" {
			continue
		}
		agent, err := s.inbox.agentRepo.GetByID(ctx, ws.ID, *id)
		if err != nil {
			slog.WarnContext(ctx, "portal assistant lookup failed", "workspace_id", ws.ID, "error", err)
			continue
		}
		if agent != nil && strings.TrimSpace(agent.Name) != "" {
			return strings.TrimSpace(agent.Name)
		}
	}
	return defaultPortalAssistantName
}

// RequestLink emails a sign-in link to an eligible address. It deliberately
// reports nothing and sends nothing otherwise, so callers cannot learn whether
// an address exists, is eligible, or was throttled. No identity is created
// until the address owner exchanges the link.
func (s *CustomerPortalService) RequestLink(ctx context.Context, ws *PortalWorkspace, address string) {
	address, ok := normalizePortalEmail(address)
	if !ok {
		return
	}
	result, err := s.eligibility(ctx, s.repo, ws, address)
	if err != nil {
		slog.ErrorContext(ctx, "portal eligibility lookup failed", "workspace_id", ws.ID, "error", err)
		return
	}
	if !result.Eligible {
		slog.InfoContext(ctx, "portal link not issued to ineligible address", "workspace_id", ws.ID)
		return
	}
	s.sendLink(ctx, ws, address, nil)
}

// sendLink issues and emails a sign-in link, reporting whether it was sent.
// Callers check eligibility first.
func (s *CustomerPortalService) sendLink(ctx context.Context, ws *PortalWorkspace, address string, conversationID *string) bool {
	address, ok := normalizePortalEmail(address)
	if !ok || s.sender == nil {
		return false
	}
	now := time.Now()
	if !s.linkAllowed(ctx, ws.ID, address, now, conversationID != nil) {
		return false
	}
	secret, err := portalSecret()
	if err != nil {
		slog.ErrorContext(ctx, "portal link generation failed", "workspace_id", ws.ID, "error", err)
		return false
	}
	link := &model.PortalMagicLink{ID: uuid.NewString(), WorkspaceID: ws.ID, Email: address, TokenHash: portalHash(secret), ConversationID: conversationID, ExpiresAt: now.Add(portalLinkTTL)}
	if err := s.repo.CreateLink(ctx, link); err != nil {
		slog.ErrorContext(ctx, "portal link persistence failed", "workspace_id", ws.ID, "error", err)
		return false
	}
	target := s.address(ctx, ws.ID, ws.Slug).callback(secret)
	subject, htmlBody, textBody := portalLinkEmail(target, conversationID != nil)
	if err := s.sender.SendEmail(address, subject, htmlBody, textBody); err != nil {
		slog.ErrorContext(ctx, "portal link delivery failed", "workspace_id", ws.ID, "error", err)
		return false
	}
	return true
}

// linkAllowed applies hourly per-address and per-workspace caps. The
// per-address cooldown only applies to plain sign-in links so a request
// confirmation is not lost right after a sign-in request.
func (s *CustomerPortalService) linkAllowed(ctx context.Context, workspaceID, address string, now time.Time, confirmation bool) bool {
	counts, err := s.repo.LinkCountsSince(ctx, workspaceID, address, now.Add(-time.Hour))
	if err != nil {
		slog.ErrorContext(ctx, "portal link throttle lookup failed", "workspace_id", workspaceID, "error", err)
		return false
	}
	if counts.ForWorkspace >= portalLinksPerWorkspaceHour {
		slog.WarnContext(ctx, "portal link workspace limit reached", "workspace_id", workspaceID)
		return false
	}
	if counts.ForEmail >= portalLinksPerAddressPerHour {
		slog.InfoContext(ctx, "portal link address limit reached", "workspace_id", workspaceID)
		return false
	}
	if !confirmation && counts.LastForEmail != nil && now.Sub(*counts.LastForEmail) < portalLinkCooldown {
		slog.InfoContext(ctx, "portal link cooldown active", "workspace_id", workspaceID)
		return false
	}
	return true
}

func portalLinkEmail(target string, confirmation bool) (subject, htmlBody, textBody string) {
	escaped := html.EscapeString(target)
	if confirmation {
		return "Confirm your support request",
			`<p>We received a support request from this email address. If you sent it, confirm to view and reply to it in the support portal (link valid for 15 minutes): <a href="` + escaped + `">Confirm and sign in</a></p><p>If you did not send this request, you can ignore this email.</p>`,
			"We received a support request from this email address. If you sent it, confirm to view and reply to it in the support portal (valid for 15 minutes):\n" + target + "\n\nIf you did not send this request, you can ignore this email."
	}
	return "Sign in to your support portal",
		`<p>Use this link to sign in (valid for 15 minutes): <a href="` + escaped + `">Sign in</a></p>`,
		"Sign in to your support portal (valid for 15 minutes): " + target
}

// Exchange consumes a sign-in link and returns a new session secret. The
// address must still be eligible, and the identity is bound to the matched
// contact. It claims the address's eligible conversations and, for an intake
// confirmation link, publishes that one intake request.
func (s *CustomerPortalService) Exchange(ctx context.Context, ws *PortalWorkspace, secret string) (string, *model.SupportPortalIdentity, error) {
	if !validPortalSecret(secret) {
		return "", nil, ErrPortalAuthInvalid
	}
	sessionSecret, err := portalSecret()
	if err != nil {
		return "", nil, err
	}
	now := time.Now()
	var identity model.SupportPortalIdentity
	_, err = s.repo.ConsumeLink(ctx, ws.ID, portalHash(secret), now, func(tx *gorm.DB, link *model.PortalMagicLink) (*model.PortalSession, error) {
		candidate := model.SupportPortalIdentity{ID: uuid.NewString(), WorkspaceID: ws.ID, Email: link.Email}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "workspace_id"}, {Name: "email"}}, DoNothing: true}).Create(&candidate).Error; err != nil {
			return nil, err
		}
		if err := tx.Where("workspace_id = ? AND email = ?", ws.ID, link.Email).First(&identity).Error; err != nil {
			return nil, err
		}
		// Access can change during the link's lifetime. Rejecting rolls back,
		// leaving the link usable until it expires.
		txRepo := s.repo.WithTx(tx)
		result, err := s.eligibility(ctx, txRepo, ws, link.Email)
		if err != nil {
			return nil, err
		}
		if !result.Eligible {
			return nil, ErrPortalAuthInvalid
		}
		// A different contact now owns this email: sessions issued for the
		// previous contact must not continue as the new one.
		if derefString(identity.CRMContactID) != derefString(result.ContactID) {
			if err := txRepo.RevokeIdentitySessions(ctx, ws.ID, identity.ID, now); err != nil {
				return nil, err
			}
		}
		if err := txRepo.BindIdentityContact(ctx, ws.ID, identity.ID, result.ContactID); err != nil {
			return nil, err
		}
		identity.CRMContactID = result.ContactID
		if err := s.repo.AssociateVerifiedEmail(tx, ws.ID, link.Email, identity.ID); err != nil {
			return nil, err
		}
		if link.ConversationID != nil {
			mode := ""
			if portalAIQueueAllowed(ws.Settings, nil) {
				mode = ws.Settings.PortalAIMode
			}
			if err := s.repo.ClaimIntakeRequest(tx, ws.ID, *link.ConversationID, identity.ID, mode); err != nil {
				return nil, err
			}
		}
		reconciledAt := now
		return &model.PortalSession{ID: uuid.NewString(), WorkspaceID: ws.ID, IdentityID: identity.ID, CRMContactID: result.ContactID, TokenHash: portalHash(sessionSecret), ExpiresAt: now.Add(portalSessionTTL), ReconciledAt: &reconciledAt}, nil
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, ErrPortalAuthInvalid) {
			return "", nil, ErrPortalAuthInvalid
		}
		return "", nil, fmt.Errorf("exchange portal link: %w", err)
	}
	return sessionSecret, &identity, nil
}

// Authenticate resolves a session secret to its session and identity and
// rechecks portal eligibility on every request. A session that is no longer
// authorized is revoked, so blocking a contact, changing its email, deleting
// it, or switching access mode takes effect on the next request. It returns
// ErrPortalAuthInvalid only for an invalid or unauthorized session; lookup
// failures are returned as other errors and leave the session intact.
func (s *CustomerPortalService) Authenticate(ctx context.Context, ws *PortalWorkspace, secret string) (*PortalAccess, error) {
	if !validPortalSecret(secret) {
		return nil, ErrPortalAuthInvalid
	}
	session, err := s.repo.FindSession(ctx, ws.ID, portalHash(secret), time.Now())
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrPortalAuthInvalid
	}
	if err != nil {
		return nil, fmt.Errorf("find portal session: %w", err)
	}
	var identity model.SupportPortalIdentity
	if err := s.repo.FindSessionIdentity(ctx, ws.ID, session.IdentityID, &identity); errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrPortalAuthInvalid
	} else if err != nil {
		return nil, fmt.Errorf("find portal identity: %w", err)
	}
	authorized, err := s.sessionStillAuthorized(ctx, ws, session, &identity)
	if err != nil {
		return nil, fmt.Errorf("check portal eligibility: %w", err)
	}
	if !authorized {
		if err := s.repo.RevokeSessionByID(ctx, session.ID, time.Now()); err != nil {
			slog.ErrorContext(ctx, "portal session revocation failed", "workspace_id", ws.ID, "error", err)
		}
		slog.InfoContext(ctx, "portal session revoked: no longer eligible", "workspace_id", ws.ID, "portal_identity_id", identity.ID)
		return nil, ErrPortalAuthInvalid
	}
	return &PortalAccess{Workspace: *ws, Session: *session, Identity: identity}, nil
}

// AuthenticatePortalSocket validates a portal session for a live-update
// socket and lists the requests it may receive signals for.
func (s *CustomerPortalService) AuthenticatePortalSocket(ctx context.Context, slug, sessionSecret string) (*model.PortalSocketGrant, error) {
	ws, err := s.ResolveWorkspace(ctx, slug)
	if err != nil {
		return nil, websocket.ErrPortalSocketUnauthorized
	}
	access, err := s.Authenticate(ctx, ws, sessionSecret)
	if errors.Is(err, ErrPortalAuthInvalid) {
		return nil, websocket.ErrPortalSocketUnauthorized
	}
	if err != nil {
		return nil, err
	}
	references, err := s.repo.OwnedRequestReferences(ctx, ws.ID, access.Identity.ID)
	if err != nil {
		return nil, fmt.Errorf("list portal socket requests: %w", err)
	}
	return &model.PortalSocketGrant{WorkspaceID: ws.ID, IdentityID: access.Identity.ID, References: references}, nil
}

// Logout revokes a session secret.
func (s *CustomerPortalService) Logout(ctx context.Context, ws *PortalWorkspace, secret string) {
	if !validPortalSecret(secret) {
		return
	}
	if err := s.repo.RevokeSession(ctx, ws.ID, portalHash(secret), time.Now()); err != nil {
		slog.ErrorContext(ctx, "portal session revocation failed", "workspace_id", ws.ID, "error", err)
	}
}

func normalizePortalEmail(address string) (string, bool) {
	address = strings.TrimSpace(address)
	parsed, err := mail.ParseAddress(address)
	if err != nil || parsed.Address != address {
		return "", false
	}
	return strings.ToLower(address), true
}

func portalSecret() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func validPortalSecret(secret string) bool {
	if len(secret) != 64 {
		return false
	}
	_, err := hex.DecodeString(secret)
	return err == nil
}

func portalHash(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}
