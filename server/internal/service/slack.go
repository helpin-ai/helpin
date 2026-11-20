package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/channels/slack"
	"github.com/helpin-ai/helpin/server/internal/crypto"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// SlackConfig holds operator-owned OAuth and signature credentials.
type SlackConfig struct{ ClientID, ClientSecret, SigningSecret, RedirectURL string }

// SlackService manages the Slack workspace installation shared by PM and Support.
type SlackService struct {
	triage   *PMTriageService
	insights *PMTaskInsightsService
	tasks    *PMTaskService
	comments *PMCommentService
	authz    *authorization.AuthzService
	appURL   string
	channels *SupportChannelService
	repo     *repository.SlackRepository
	api      *slack.Client
	cfg      SlackConfig
}

// NewSlackService builds the installation and channel routing service.
func NewSlackService(channels *SupportChannelService, repo *repository.SlackRepository, cfg SlackConfig) *SlackService {
	s := &SlackService{channels: channels, repo: repo, api: slack.NewClient(), cfg: cfg}
	channels.slack = s
	return s
}

func (s *SlackService) enabled() bool {
	return s.cfg.ClientID != "" && s.cfg.ClientSecret != "" && s.cfg.SigningSecret != "" && s.cfg.RedirectURL != "" && len(s.channels.cfg.EncryptionKey) == 32
}

// Settings returns the installation display information and opted-in routes.
func (s *SlackService) Settings(ctx context.Context, workspace string) (map[string]any, error) {
	result := map[string]any{"configured": s.enabled(), "routes": []model.SlackRoute{}}
	a, err := s.repo.Installation(ctx, workspace)
	if err != nil || a == nil {
		return result, err
	}
	routes, err := s.repo.Routes(ctx, a.ID)
	result["routes"] = routes
	result["workspace_name"] = a.DisplayName
	result["connected"] = true
	result["ai_enabled"] = !a.AIRepliesDisabled
	return result, err
}

// StartInstall returns an OAuth authorization URL tied to this user and workspace.
func (s *SlackService) StartInstall(ctx context.Context, workspace, user string) (string, error) {
	if !s.enabled() {
		return "", errors.New("Slack app credentials are not configured")
	}
	raw, hash, err := newWebhookSecret()
	if err != nil {
		return "", err
	}
	if err = s.repo.SaveState(ctx, hash, workspace, user); err != nil {
		return "", err
	}
	params := url.Values{"client_id": {s.cfg.ClientID}, "redirect_uri": {s.cfg.RedirectURL}, "state": {raw}, "scope": {"app_mentions:read,channels:read,groups:read,channels:history,groups:history,chat:write,files:read,files:write,users:read,commands"}}
	return "https://slack.com/oauth/v2/authorize?" + params.Encode(), nil
}

// CompleteInstall exchanges a single-use grant and stores an encrypted bot token.
func (s *SlackService) CompleteInstall(ctx context.Context, workspace, user, raw, code string) error {
	if !s.enabled() || raw == "" || code == "" {
		return ErrChannelInvalid
	}
	hash := sha256.Sum256([]byte(raw))
	ok, err := s.repo.ConsumeState(ctx, hex.EncodeToString(hash[:]), workspace, user)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: expired Slack authorization", ErrChannelInvalid)
	}
	grant, err := s.api.Exchange(ctx, s.cfg.ClientID, s.cfg.ClientSecret, s.cfg.RedirectURL, code)
	if err != nil {
		return ErrChannelProviderUnavailable
	}
	a, err := s.repo.Installation(ctx, workspace)
	if err != nil {
		return err
	}
	if a != nil && a.ExternalAccountID != grant.Team.ID {
		return ErrChannelAlreadyConnected
	}
	if a == nil {
		a, err = s.repo.PreviousInstallation(ctx, workspace, grant.Team.ID)
		if err != nil {
			return err
		}
	}
	create := a == nil
	if create {
		a = &model.SupportChannelAccount{ID: uuid.NewString(), WorkspaceID: workspace, Provider: "slack", ExternalAccountID: grant.Team.ID, ConnectedBy: &user, AIRepliesDisabled: true, Status: "active"}
	}
	token, err := crypto.EncryptStringWithAAD(grant.Token, s.channels.cfg.EncryptionKey, channelTokenAAD(a.ID))
	if err != nil {
		return err
	}
	if !create && a.DisabledAt != nil {
		err := s.repo.Reactivate(ctx, a, token, grant.BotUserID, grant.Team.Name, user)
		if err != nil && isUniqueConstraintViolation(err) {
			return ErrChannelAlreadyConnected
		}
		return err
	}
	if !create {
		return s.channels.accounts.Update(ctx, workspace, a.ID, map[string]any{"token_encrypted": token, "handle": grant.BotUserID, "display_name": grant.Team.Name, "status": "active", "auth_error_count": 0})
	}
	a.TokenEncrypted = token
	a.Handle = grant.BotUserID
	a.DisplayName = grant.Team.Name
	if err = s.channels.accounts.Create(ctx, a); err != nil && isUniqueConstraintViolation(err) {
		return ErrChannelAlreadyConnected
	}
	return err
}

func (s *SlackService) account(ctx context.Context, workspace string) (*model.SupportChannelAccount, string, error) {
	a, err := s.repo.Installation(ctx, workspace)
	if err != nil {
		return nil, "", err
	}
	if a == nil {
		return nil, "", repository.ErrSupportChannelAccountNotFound
	}
	token, err := s.channels.decryptToken(a)
	return a, token, err
}

// SlackChannel describes a channel that the installed bot can access.
type SlackChannel struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	IsShared      bool   `json:"is_ext_shared"`
	IsPrivate     bool   `json:"is_private"`
	PendingShared bool   `json:"is_pending_ext_shared"`
	IsArchived    bool   `json:"is_archived"`
	IsMember      bool   `json:"is_member"`
}

// AvailableChannels lists the bot's joined channels across Slack's paged response.
func (s *SlackService) AvailableChannels(ctx context.Context, workspace string) ([]SlackChannel, error) {
	_, token, err := s.account(ctx, workspace)
	if err != nil {
		return nil, err
	}
	channels := []SlackChannel{}
	cursor := ""
	for {
		var page struct {
			Channels []SlackChannel
			Metadata struct {
				Next string `json:"next_cursor"`
			} `json:"response_metadata"`
		}
		if err := s.api.Get(ctx, token, "users.conversations", url.Values{"types": {"public_channel,private_channel"}, "exclude_archived": {"true"}, "limit": {"200"}, "cursor": {cursor}}, &page); err != nil {
			return nil, err
		}
		for _, ch := range page.Channels {
			if !ch.IsArchived {
				channels = append(channels, ch)
			}
		}
		if page.Metadata.Next == "" {
			return channels, nil
		}
		cursor = page.Metadata.Next
	}
}

// Configure validates both the Slack channel and Helpin destination.
func (s *SlackService) Configure(ctx context.Context, workspace string, row model.SlackRoute) error {
	a, token, err := s.account(ctx, workspace)
	if err != nil {
		return err
	}
	module := model.ModuleSupport
	if row.Purpose == "pm" {
		module = model.ModulePM
	}
	actor := authorization.GetActor(ctx)
	if actor == nil {
		return ErrChannelInvalid
	}
	allowed, err := s.authz.CanAccessModule(ctx, actor, module)
	if err != nil {
		return err
	}
	if !allowed {
		return fmt.Errorf("%w: this module is unavailable", ErrChannelInvalid)
	}
	var info struct{ Channel SlackChannel }
	if err = s.api.Get(ctx, token, "conversations.info", url.Values{"channel": {row.ChannelID}}, &info); err != nil {
		return ErrChannelProviderUnavailable
	}
	if info.Channel.ID != row.ChannelID || !info.Channel.IsMember || info.Channel.IsArchived {
		return fmt.Errorf("%w: invite @Helpin to an active channel first", ErrChannelInvalid)
	}
	switch row.Purpose {
	case "support", "mixed":
		row.TeamID = nil
		row.EpicID = nil
		row.MailboxID, err = s.channels.mailboxSelection(ctx, workspace, row.MailboxID)
	case "pm":
		if info.Channel.IsShared || info.Channel.PendingShared {
			return fmt.Errorf("%w: choose an internal channel for PM", ErrChannelInvalid)
		}
		row.MailboxID = nil
		if row.TeamID != nil {
			team, e := s.channels.inbox.workspaceRepo.GetTeamByID(ctx, workspace, *row.TeamID)
			if e != nil {
				return e
			}
			if team == nil {
				return ErrChannelInvalid
			}
		}
		err = validateEpicScope(ctx, repository.NewPMEpicRepository(s.channels.inbox.conversationRepo.DB()), workspace, row.EpicID, row.TeamID)
	default:
		return ErrChannelInvalid
	}
	if err != nil {
		return err
	}
	row.AccountID = a.ID
	row.ChannelName = info.Channel.Name
	return s.repo.SetRoute(ctx, row)
}

// Remove disables one channel binding.
func (s *SlackService) Remove(ctx context.Context, workspace, channel string) error {
	a, err := s.repo.Installation(ctx, workspace)
	if err != nil || a == nil {
		return err
	}
	return s.repo.RemoveRoute(ctx, a.ID, channel)
}

// Disconnect clears the local installation and invalidates its bot token.
func (s *SlackService) Disconnect(ctx context.Context, workspace string) error {
	a, token, err := s.account(ctx, workspace)
	if err != nil {
		return err
	}
	if err = s.channels.accounts.Disconnect(ctx, workspace, a.ID, s.channels.now()); err != nil {
		return err
	}
	if err = s.api.Call(ctx, token, "auth.revoke", map[string]string{}, nil); err != nil {
		s.channels.logger.WarnContext(ctx, "Slack token revocation failed", "workspace_id", workspace)
	}
	return nil
}

func (s *SlackService) mentionsBot(text, bot string) bool {
	return strings.Contains(text, "<@"+bot+">")
}
