package service

import (
	"context"
	"encoding/json"

	agentruntime "github.com/helpin-ai/agent-runtime-go"
	"github.com/helpin-ai/helpin/server/internal/model"
)

type ExternalMCPServiceConfig struct {
	Enabled                bool
	CustomServersEnabled   bool
	EncryptionKey          string
	AllowedHosts           []string
	OAuthRedirectURL       string
	AppBaseURL             string
	OAuthClientID          string
	OAuthClientSecret      string
	OAuthClientAuthMethod  string
	AllowInsecureLocalhost bool
}

type ExternalMCPProviderOption struct {
	Provider       string   `json:"provider"`
	Region         string   `json:"region"`
	Name           string   `json:"name"`
	EndpointURL    string   `json:"endpoint_url"`
	AuthType       string   `json:"auth_type"`
	DefaultScopes  []string `json:"default_scopes"`
	OptionalScopes []string `json:"optional_scopes"`
}

type CreateExternalMCPServerRequest struct {
	Name        string            `json:"name"`
	Provider    string            `json:"provider"`
	Region      string            `json:"region,omitempty"`
	EndpointURL string            `json:"endpoint_url,omitempty"`
	AuthType    string            `json:"auth_type,omitempty"`
	OAuthScopes []string          `json:"oauth_scopes,omitempty"`
	BearerToken string            `json:"bearer_token,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
}

type UpdateExternalMCPServerRequest struct {
	Name    *string `json:"name,omitempty"`
	Enabled *bool   `json:"enabled,omitempty"`
}

type ExternalMCPToolPolicyInput struct {
	ID      string `json:"id"`
	Enabled bool   `json:"enabled"`
	Access  string `json:"access"`
}

type UpdateExternalMCPToolsRequest struct {
	Tools []ExternalMCPToolPolicyInput `json:"tools"`
}

type ExternalMCPOAuthStart struct {
	AuthorizationURL string `json:"authorization_url"`
}

type ExternalMCPOAuthCallbackResult struct {
	ServerID    string
	WorkspaceID string
	ReturnPath  string
}

// ExternalMCPOAuthAuthorizer revalidates the initiating user's workspace
// permission after single-use state consumption and before exchanging tokens.
type ExternalMCPOAuthAuthorizer func(context.Context, string, string) error

type ExternalMCPRunServer = agentruntime.RunMCPServer
type ExternalMCPRunTool = agentruntime.RunMCPTool
type ExternalMCPRunCredential = agentruntime.RunMCPCredential

type ExternalMCPResolvedRun struct {
	Servers  []ExternalMCPRunServer
	Bindings []model.AgentRunExternalMCPBinding
}

type ExternalMCPRunCredentialUpdate struct {
	AgentRunID   string
	RuntimeRunID string
	ServerID     string
	Credential   *ExternalMCPRunCredential
}

func marshalStringList(values []string) json.RawMessage {
	payload, _ := json.Marshal(values)
	return payload
}
