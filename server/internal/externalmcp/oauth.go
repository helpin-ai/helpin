package externalmcp

import (
	"context"
	"net/url"

	mcpauth "github.com/helpin-ai/agent-runtime-go/mcpauth"
)

type ProtectedResourceMetadata = mcpauth.ProtectedResourceMetadata
type AuthorizationServerMetadata = mcpauth.AuthorizationServerMetadata
type OAuthConfiguration = mcpauth.Configuration
type OAuthClientRegistration = mcpauth.ClientRegistration
type OAuthToken = mcpauth.Token
type OAuthAuthorizationRequest = mcpauth.AuthorizationRequest

// oauthProtocolClient layers the reusable SDK protocol implementation over
// Helpin's app-owned DNS, egress, response-size, and provider rollout policy.
func (c *Client) oauthProtocolClient() (*mcpauth.Client, error) {
	allowedHosts := append([]string(nil), c.allowed...)
	if c.baseSite != "" {
		allowedHosts = append(allowedHosts, "*."+c.baseSite)
	}
	options := []mcpauth.Option{
		mcpauth.WithHTTPClient(c.HTTPClient()),
		mcpauth.WithAllowedHosts(allowedHosts...),
		mcpauth.WithURLValidator(func(candidate *url.URL) error {
			return c.ValidateRelatedURL(candidate.String())
		}),
	}
	if c.allowLocal {
		options = append(options, mcpauth.WithInsecureLocalhost())
	}
	return mcpauth.NewClient(c.Endpoint(), options...)
}

func (c *Client) DiscoverOAuth(ctx context.Context) (*OAuthConfiguration, error) {
	client, err := c.oauthProtocolClient()
	if err != nil {
		return nil, err
	}
	return client.Discover(ctx)
}

func (c *Client) RegisterOAuthClient(ctx context.Context, endpoint, redirectURI string) (*OAuthClientRegistration, error) {
	client, err := c.oauthProtocolClient()
	if err != nil {
		return nil, err
	}
	return client.Register(ctx, endpoint, redirectURI)
}

func (c *Client) NewOAuthAuthorizationRequest(configuration *OAuthConfiguration, clientID, redirectURI string, scopes []string) (*OAuthAuthorizationRequest, error) {
	client, err := c.oauthProtocolClient()
	if err != nil {
		return nil, err
	}
	return client.NewAuthorizationRequest(configuration, clientID, redirectURI, scopes)
}

func (c *Client) ExchangeCode(ctx context.Context, endpoint, clientID, clientSecret, authMethod, code, verifier, redirectURI, resource string) (*OAuthToken, error) {
	client, err := c.oauthProtocolClient()
	if err != nil {
		return nil, err
	}
	return client.ExchangeCode(ctx, endpoint, clientID, clientSecret, authMethod, code, verifier, redirectURI, resource)
}

func (c *Client) RefreshToken(ctx context.Context, endpoint, clientID, clientSecret, authMethod, refreshToken, resource string) (*OAuthToken, error) {
	client, err := c.oauthProtocolClient()
	if err != nil {
		return nil, err
	}
	return client.Refresh(ctx, endpoint, clientID, clientSecret, authMethod, refreshToken, resource)
}
