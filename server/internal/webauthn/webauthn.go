package appwebauthn

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	gwebauthn "github.com/go-webauthn/webauthn/webauthn"

	"github.com/helpin-ai/helpin/server/internal/cache"
)

const (
	SessionKindRegistration   = "registration"
	SessionKindAuthentication = "authentication"
	SessionKindKnownUserLogin = "authentication_known_user"
	defaultRPDisplayName      = "Helpin"
	sessionCacheKeyPrefix     = "webauthn:session:"
	sessionCacheTagPrefix     = "webauthn:session:"
)

var (
	ErrNotConfigured     = errors.New("passkeys are not configured")
	ErrSessionNotFound   = errors.New("passkey session not found or expired")
	ErrUnexpectedSession = errors.New("passkey session type is invalid")
)

type storedSession struct {
	Kind string                `json:"kind"`
	Data gwebauthn.SessionData `json:"data"`
}

type SessionInfo struct {
	Kind   string
	UserID []byte
}

// Client wraps the upstream WebAuthn library with short-lived server-side session storage.
type Client struct {
	relyingParty *gwebauthn.WebAuthn
	sessionStore cache.Cache
}

func NewClient(rpID string, rpOrigins []string, sessionStore cache.Cache) (*Client, error) {
	rpID = strings.TrimSpace(rpID)
	if rpID == "" || len(rpOrigins) == 0 {
		return &Client{sessionStore: sessionStore}, nil
	}

	config := &gwebauthn.Config{
		RPID:          rpID,
		RPDisplayName: defaultRPDisplayName,
		RPOrigins:     rpOrigins,
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			UserVerification: protocol.VerificationPreferred,
		},
	}

	relyingParty, err := gwebauthn.New(config)
	if err != nil {
		return nil, fmt.Errorf("create webauthn relying party: %w", err)
	}

	return &Client{
		relyingParty: relyingParty,
		sessionStore: sessionStore,
	}, nil
}

func (c *Client) Configured() bool {
	return c != nil && c.relyingParty != nil
}

func (c *Client) BeginRegistration(ctx context.Context, user gwebauthn.User, exclusions []protocol.CredentialDescriptor) (*protocol.CredentialCreation, string, error) {
	if !c.Configured() {
		return nil, "", ErrNotConfigured
	}

	options, session, err := c.relyingParty.BeginRegistration(
		user,
		gwebauthn.WithAuthenticatorSelection(protocol.AuthenticatorSelection{
			UserVerification: protocol.VerificationPreferred,
		}),
		gwebauthn.WithResidentKeyRequirement(protocol.ResidentKeyRequirementRequired),
		gwebauthn.WithExclusions(exclusions),
	)
	if err != nil {
		return nil, "", err
	}

	if err := c.storeSession(ctx, SessionKindRegistration, session); err != nil {
		return nil, "", err
	}

	return options, session.Challenge, nil
}

func (c *Client) BeginAuthentication(ctx context.Context) (*protocol.CredentialAssertion, string, error) {
	if !c.Configured() {
		return nil, "", ErrNotConfigured
	}

	options, session, err := c.relyingParty.BeginDiscoverableLogin(
		gwebauthn.WithUserVerification(protocol.VerificationPreferred),
	)
	if err != nil {
		return nil, "", err
	}

	if err := c.storeSession(ctx, SessionKindAuthentication, session); err != nil {
		return nil, "", err
	}

	return options, session.Challenge, nil
}

func (c *Client) BeginAuthenticationForUser(ctx context.Context, user gwebauthn.User) (*protocol.CredentialAssertion, string, error) {
	if !c.Configured() {
		return nil, "", ErrNotConfigured
	}

	options, session, err := c.relyingParty.BeginLogin(
		user,
		gwebauthn.WithUserVerification(protocol.VerificationPreferred),
	)
	if err != nil {
		return nil, "", err
	}

	if err := c.storeSession(ctx, SessionKindKnownUserLogin, session); err != nil {
		return nil, "", err
	}

	return options, session.Challenge, nil
}

func (c *Client) FinishRegistration(ctx context.Context, user gwebauthn.User, challenge string, payload []byte) (*gwebauthn.Credential, error) {
	if !c.Configured() {
		return nil, ErrNotConfigured
	}

	session, err := c.consumeSession(ctx, challenge, SessionKindRegistration)
	if err != nil {
		return nil, err
	}

	request, err := newJSONRequest(payload)
	if err != nil {
		return nil, err
	}

	return c.relyingParty.FinishRegistration(user, session.Data, request)
}

func (c *Client) FinishAuthentication(ctx context.Context, challenge string, payload []byte, handler gwebauthn.DiscoverableUserHandler) (gwebauthn.User, *gwebauthn.Credential, error) {
	if !c.Configured() {
		return nil, nil, ErrNotConfigured
	}

	session, err := c.consumeSession(ctx, challenge, SessionKindAuthentication)
	if err != nil {
		return nil, nil, err
	}

	request, err := newJSONRequest(payload)
	if err != nil {
		return nil, nil, err
	}

	return c.relyingParty.FinishPasskeyLogin(handler, session.Data, request)
}

func (c *Client) FinishAuthenticationForUser(ctx context.Context, user gwebauthn.User, challenge string, payload []byte) (*gwebauthn.Credential, error) {
	if !c.Configured() {
		return nil, ErrNotConfigured
	}

	session, err := c.consumeSession(ctx, challenge, SessionKindKnownUserLogin)
	if err != nil {
		return nil, err
	}

	request, err := newJSONRequest(payload)
	if err != nil {
		return nil, err
	}

	return c.relyingParty.FinishLogin(user, session.Data, request)
}

func (c *Client) storeSession(ctx context.Context, kind string, session *gwebauthn.SessionData) error {
	if session == nil {
		return fmt.Errorf("missing webauthn session")
	}
	if c.sessionStore == nil {
		return fmt.Errorf("missing passkey session store")
	}

	encoded, err := json.Marshal(storedSession{
		Kind: kind,
		Data: *session,
	})
	if err != nil {
		return fmt.Errorf("marshal passkey session: %w", err)
	}

	if err := c.sessionStore.Set(ctx, sessionKey(session.Challenge), encoded, 5*time.Minute, sessionTag(session.Challenge)); err != nil {
		return fmt.Errorf("store passkey session: %w", err)
	}

	return nil
}

func (c *Client) consumeSession(ctx context.Context, challenge, expectedKind string) (*storedSession, error) {
	if c.sessionStore == nil {
		return nil, ErrSessionNotFound
	}
	if strings.TrimSpace(challenge) == "" {
		return nil, ErrSessionNotFound
	}

	raw, found, err := c.sessionStore.Get(ctx, sessionKey(challenge))
	if err != nil {
		return nil, fmt.Errorf("load passkey session: %w", err)
	}
	if !found {
		return nil, ErrSessionNotFound
	}

	var session storedSession
	if err := json.Unmarshal(raw, &session); err != nil {
		return nil, fmt.Errorf("decode passkey session: %w", err)
	}
	if session.Kind != expectedKind {
		return nil, ErrUnexpectedSession
	}

	_ = c.sessionStore.InvalidateTags(ctx, sessionTag(challenge))

	return &session, nil
}

func (c *Client) SessionInfo(ctx context.Context, challenge string) (*SessionInfo, error) {
	session, err := c.peekSession(ctx, challenge)
	if err != nil {
		return nil, err
	}

	return &SessionInfo{
		Kind:   session.Kind,
		UserID: append([]byte(nil), session.Data.UserID...),
	}, nil
}

func newJSONRequest(payload []byte) (*http.Request, error) {
	request, err := http.NewRequest(http.MethodPost, "https://webauthn.helpin.local", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create webauthn request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	return request, nil
}

func sessionKey(challenge string) string {
	return sessionCacheKeyPrefix + challenge
}

func sessionTag(challenge string) string {
	return sessionCacheTagPrefix + challenge
}

func (c *Client) peekSession(ctx context.Context, challenge string) (*storedSession, error) {
	if c.sessionStore == nil {
		return nil, ErrSessionNotFound
	}
	if strings.TrimSpace(challenge) == "" {
		return nil, ErrSessionNotFound
	}

	raw, found, err := c.sessionStore.Get(ctx, sessionKey(challenge))
	if err != nil {
		return nil, fmt.Errorf("load passkey session: %w", err)
	}
	if !found {
		return nil, ErrSessionNotFound
	}

	var session storedSession
	if err := json.Unmarshal(raw, &session); err != nil {
		return nil, fmt.Errorf("decode passkey session: %w", err)
	}

	return &session, nil
}
