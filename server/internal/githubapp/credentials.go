package githubapp

import (
	"context"
	"crypto/hmac"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
)

// ErrNotConfigured reports that no usable GitHub App credentials are available.
var ErrNotConfigured = errors.New("github app is not configured")

// Credentials are the instance-wide GitHub App settings. PrivateKey,
// ClientSecret and WebhookSecret are secrets and must never be logged or
// returned to API clients.
type Credentials struct {
	AppID         string
	Slug          string
	ClientID      string
	ClientSecret  string
	PrivateKey    string
	WebhookSecret string
	HTMLURL       string
}

// Usable reports whether the credentials can sign App JWTs.
func (c Credentials) Usable() bool {
	return strings.TrimSpace(c.AppID) != "" && strings.TrimSpace(c.PrivateKey) != ""
}

// InstallURL returns the public installation URL for the App, or "" when the
// slug is unknown.
func (c Credentials) InstallURL() string {
	slug := strings.TrimSpace(c.Slug)
	if slug == "" {
		return ""
	}
	return "https://github.com/apps/" + slug + "/installations/new"
}

// CredentialSource yields the current GitHub App credentials. Implementations
// return zero Credentials (not an error) when no App is configured.
type CredentialSource interface {
	Current(ctx context.Context) (Credentials, error)
}

// StaticSource returns a CredentialSource that always yields creds.
func StaticSource(creds Credentials) CredentialSource {
	return staticSource{creds: creds}
}

type staticSource struct {
	creds Credentials
}

func (s staticSource) Current(context.Context) (Credentials, error) {
	return s.creds, nil
}

// VerifyWebhookSignature checks a GitHub X-Hub-Signature-256 header against
// body using secret, in constant time. An empty secret never verifies.
func VerifyWebhookSignature(secret string, body []byte, signature string) bool {
	if secret == "" {
		return false
	}
	const prefix = "sha256="
	signature = strings.TrimSpace(signature)
	if !strings.HasPrefix(signature, prefix) {
		return false
	}
	got, err := hex.DecodeString(signature[len(prefix):])
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return hmac.Equal(mac.Sum(nil), got)
}

// SignWebhookBody returns the X-Hub-Signature-256 value for body. It is used
// by tests and by callers that need to produce GitHub-compatible signatures.
func SignWebhookBody(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// signingKeyCache keeps the parsed RSA key for the most recent PEM so a
// rotated key is picked up without reparsing on every request.
type signingKeyCache struct {
	mu  sync.Mutex
	pem string
	key *rsa.PrivateKey
}

func (c *signingKeyCache) get(privateKeyPEM string) (*rsa.PrivateKey, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.key != nil && c.pem == privateKeyPEM {
		return c.key, nil
	}
	key, err := parsePrivateKey(privateKeyPEM)
	if err != nil {
		return nil, err
	}
	c.pem = privateKeyPEM
	c.key = key
	return key, nil
}
