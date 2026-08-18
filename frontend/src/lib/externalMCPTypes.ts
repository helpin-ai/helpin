export type ExternalMCPStatus =
  | 'pending_oauth'
  | 'connected'
  | 'reauthorization_required'
  | 'insufficient_scope'
  | 'remote_disabled'
  | 'error'
  | 'disconnected';

export type ExternalMCPAuthType = 'oauth' | 'bearer_token' | 'headers' | 'none';

export type ExternalMCPTool = {
  id: string;
  workspace_id: string;
  server_id: string;
  remote_name: string;
  runtime_alias: string;
  description: string;
  input_schema: Record<string, unknown>;
  access: 'read' | 'write';
  enabled: boolean;
  schema_hash: string;
  last_seen_at: string;
  created_at: string;
  updated_at: string;
};

export type ExternalMCPServer = {
  id: string;
  workspace_id: string;
  name: string;
  server_name: string;
  provider: 'customer_io' | 'custom';
  endpoint_url: string;
  transport: 'streamable_http';
  auth_type: ExternalMCPAuthType;
  status: ExternalMCPStatus;
  enabled: boolean;
  oauth_scopes: string[];
  remote_identity?: Record<string, unknown>;
  authorized_by_user_id?: string;
  access_token_expires_at?: string;
  last_health_checked_at?: string;
  last_tool_sync_at?: string;
  last_error_code?: string;
  last_error_message?: string;
  created_by: string;
  created_at: string;
  updated_at: string;
  tools?: ExternalMCPTool[];
};

export type ExternalMCPProvider = {
  provider: 'customer_io' | 'custom';
  region: string;
  name: string;
  website_url: string;
  endpoint_url: string;
  auth_type: ExternalMCPAuthType;
  default_scopes: string[];
  optional_scopes: string[];
};

export type CreateExternalMCPServerRequest = {
  name: string;
  provider: 'customer_io' | 'custom';
  region?: string;
  endpoint_url?: string;
  auth_type?: ExternalMCPAuthType;
  oauth_scopes?: string[];
  bearer_token?: string;
  headers?: Record<string, string>;
};

export type UpdateExternalMCPToolsRequest = {
  tools: Array<{ id: string; enabled: boolean; access: 'read' | 'write' }>;
};
