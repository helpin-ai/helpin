export type MCPWorkspacePolicy = {
  workspace_id: string;
  enabled: boolean;
  enforce_read_only: boolean;
  service_accounts_enabled: boolean;
  allowed_toolsets: string[];
  allowed_scopes: string[];
  updated_by?: string;
  created_at?: string;
  updated_at?: string;
};

export type MCPConnection = {
  id: string;
  workspace_id: string;
  user_id: string;
  client_id: string;
  client_name: string;
  scopes: string[];
  toolsets: string[];
  read_only: boolean;
  effective_read_only?: boolean;
  status: 'active' | 'revoked';
  last_used_at?: string;
  revoked_at?: string;
  created_at: string;
  updated_at: string;
};

export type MCPServicePrincipal = {
  id: string;
  workspace_id: string;
  name: string;
  description?: string;
  actor_user_id: string;
  scopes: string[];
  toolsets: string[];
  read_only: boolean;
  effective_read_only?: boolean;
  status: 'active' | 'revoked';
  expires_at?: string;
  last_used_at?: string;
  revoked_at?: string;
  created_by: string;
  created_at: string;
  updated_at: string;
};

export type MCPServiceToken = {
  id: string;
  service_principal_id: string;
  token_prefix: string;
  expires_at?: string;
  last_used_at?: string;
  revoked_at?: string;
  created_at: string;
};

export type MCPServiceTokenSecret = {
  token: string;
  record: MCPServiceToken;
};

export type MCPAuditEvent = {
  id: string;
  workspace_id: string;
  connection_id?: string;
  service_principal_id?: string;
  user_id?: string;
  client_name: string;
  event_type: string;
  tool_name?: string;
  outcome: 'success' | 'denied' | 'error';
  reason_code?: string;
  duration_ms?: number;
  created_at: string;
};

export type MCPDashboard = {
  policy: MCPWorkspacePolicy;
  connections: MCPConnection[];
  service_principals: MCPServicePrincipal[];
  available_toolsets: string[];
  available_scopes: string[];
  mcp_url: string;
  can_manage: boolean;
  can_view_activity: boolean;
  can_use_mcp: boolean;
  platform_enabled: boolean;
  support_setup_available?: boolean;
};

export type UpdateMCPPolicyRequest = Pick<
  MCPWorkspacePolicy,
  'enabled' | 'enforce_read_only' | 'service_accounts_enabled' | 'allowed_toolsets' | 'allowed_scopes'
>;

export type CreateMCPServicePrincipalRequest = {
  name: string;
  description?: string;
  scopes: string[];
  toolsets: string[];
  read_only: boolean;
  expires_at?: string;
};

export type MCPClientRegistration = {
  id: string;
  client_id: string;
  client_name: string;
  client_uri?: string;
  redirect_uris: string[];
};

export type MCPAuthorizationQuery = {
  client_id: string;
  redirect_uri: string;
  response_type: string;
  scope: string;
  state: string;
  code_challenge: string;
  code_challenge_method: string;
};

export type MCPAuthorizationWorkspace = {
  id: string;
  name: string;
  slug: string;
  website_url?: string;
  logo_url?: string;
  role: string;
  allowed_scopes: string[];
  allowed_toolsets: string[];
  read_only_required: boolean;
};

export type MCPAuthorizationRequest = {
  client: MCPClientRegistration;
  query: MCPAuthorizationQuery;
  requested_scopes: string[];
  proposed_toolsets: string[];
  workspaces: MCPAuthorizationWorkspace[];
  read_only_recommended: boolean;
};

export type MCPAuthorizeDecision = {
  query: MCPAuthorizationQuery;
  workspace_id: string;
  scopes: string[];
  toolsets: string[];
  read_only: boolean;
};

export type MCPAuthorizeResult = { redirect_url: string };
