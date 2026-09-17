package service

import (
	"context"
	"encoding/base64"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/model"
)

var cliVerifierPattern = regexp.MustCompile(`^[A-Za-z0-9._~-]{43,128}$`)

// CLITokenResponse contains a short-lived opaque access token and rotating refresh token.
type CLITokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

// Authorize grants browser consent for exactly one current workspace membership.
func (s *CLIService) Authorize(ctx context.Context, userID string, d model.CLIConsentDecision, mfaSatisfied bool) (string, error) {
	if err := s.validateQuery(d.Query); err != nil {
		return "", err
	}
	if _, err := s.actor(ctx, d.WorkspaceID, userID); err != nil {
		return "", err
	}
	if err := s.requireMFA(ctx, d.WorkspaceID, userID, mfaSatisfied); err != nil {
		return "", err
	}
	raw, err := randomMCPSecret("hcli_code_")
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	c := &model.CLIConnection{MFASatisfied: mfaSatisfied, ID: uuid.NewString(), UserID: userID, WorkspaceID: d.WorkspaceID, ClientID: cliClientID, Resource: s.resource(), Scope: cliScope, CreatedAt: now}
	token := &model.CLIToken{Hash: mcpHash(raw), ConnectionID: c.ID, Kind: "code", RedirectURI: d.Query.RedirectURI, CodeChallenge: d.Query.CodeChallenge, ExpiresAt: now.Add(2 * time.Minute), CreatedAt: now}
	if err = s.repo.Consent(ctx, c, token); err != nil {
		return "", err
	}
	redirect, err := url.Parse(d.Query.RedirectURI)
	if err != nil {
		return "", err
	}
	q := redirect.Query()
	q.Set("code", raw)
	q.Set("state", d.Query.State)
	q.Set("iss", s.issuer())
	redirect.RawQuery = q.Encode()
	return redirect.String(), nil
}

// Exchange consumes authorization codes or rotates refresh tokens atomically.
func (s *CLIService) Exchange(ctx context.Context, grant, raw, client, resource, redirect, verifier string) (*CLITokenResponse, error) {
	if !s.Enabled() {
		return nil, ErrCLIDisabled
	}
	if client != cliClientID || resource != s.resource() || len(raw) > 256 {
		return nil, ErrCLIUnauthorized
	}
	kind := "code"
	if grant == "refresh_token" {
		kind = "refresh"
	} else if grant != "authorization_code" {
		return nil, ErrCLIInvalid
	}
	old, err := s.repo.Token(ctx, mcpHash(raw))
	if err != nil {
		return nil, err
	}
	if old == nil || old.Kind != kind || !old.ExpiresAt.After(time.Now()) {
		return nil, ErrCLIUnauthorized
	}
	c, err := s.connection(ctx, old.ConnectionID)
	if err != nil {
		return nil, err
	}
	if kind == "code" {
		valid := cliVerifierPattern.MatchString(verifier)
		if redirect != old.RedirectURI || !valid || !verifyMCPCodeChallenge(verifier, old.CodeChallenge) {
			return nil, ErrCLIUnauthorized
		}
	}
	access, err := randomMCPSecret("hcli_access_")
	if err != nil {
		return nil, err
	}
	refresh, err := randomMCPSecret("hcli_refresh_")
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	pair := []model.CLIToken{{Hash: mcpHash(access), ConnectionID: c.ID, Kind: "access", ExpiresAt: now.Add(10 * time.Minute), CreatedAt: now}, {Hash: mcpHash(refresh), ConnectionID: c.ID, Kind: "refresh", ExpiresAt: now.Add(30 * 24 * time.Hour), CreatedAt: now}}
	ok, err := s.repo.Rotate(ctx, old, pair, now)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrCLIUnauthorized
	}
	return &CLITokenResponse{AccessToken: access, RefreshToken: refresh, TokenType: "Bearer", ExpiresIn: 600, Scope: cliScope}, nil
}

// RevokeToken revokes the whole consent family using possession of its token.
func (s *CLIService) RevokeToken(ctx context.Context, raw, client string) error {
	if !s.Enabled() {
		return ErrCLIDisabled
	}
	if client != cliClientID || len(raw) > 256 {
		return ErrCLIInvalid
	}
	t, err := s.repo.Token(ctx, mcpHash(raw))
	if err != nil {
		return err
	}
	if t == nil || t.Kind == "code" {
		return nil
	}
	return s.repo.Revoke(ctx, t.ConnectionID, time.Now().UTC())
}
func (s *CLIService) validateQuery(q model.CLIAuthorizationQuery) error {
	if !s.Enabled() {
		return ErrCLIDisabled
	}
	if q.ClientID != cliClientID || q.Resource != s.resource() || q.ResponseType != "code" || q.Scope != cliScope || q.CodeChallengeMethod != "S256" || len(q.State) < 16 || len(q.State) > 256 {
		return ErrCLIInvalid
	}
	challenge, err := base64.RawURLEncoding.DecodeString(q.CodeChallenge)
	if err != nil || len(challenge) != 32 {
		return ErrCLIInvalid
	}
	u, err := url.Parse(q.RedirectURI)
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.Port() == "" || u.Path != "/callback" || u.RawQuery != "" || u.Fragment != "" || u.User != nil || strings.Contains(u.Host, "%") {
		return ErrCLIInvalid
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil || port < 1 || port > 65535 {
		return ErrCLIInvalid
	}
	return nil
}
