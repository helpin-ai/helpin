/** Capability keys reported by `GET /workspaces/{id}/capabilities`. */
export type CapabilityKey =
  | 'ai_chat'
  | 'ai_embeddings'
  | 'email_outbound'
  | 'support_widget'
  | 'support_email_inbound'
  | 'github'
  | 'object_storage'
  | 'workers';

export type CapabilityStatus = 'ready' | 'needs_setup' | 'unable_to_verify' | 'unavailable';

export type CapabilityActionKind = 'open_settings' | 'server_config' | 'send_test_email' | 'test_ai_connection';

export interface CapabilityAction {
  kind: CapabilityActionKind;
  label: string;
  /** Workspace-relative route, for example `settings/ai`. */
  path?: string;
  connection_id?: string;
}

export interface Capability {
  key: CapabilityKey;
  status: CapabilityStatus;
  detail: string;
  required: boolean;
  checked_at?: string;
  action?: CapabilityAction;
}

export interface CapabilitiesResponse {
  edition: 'community' | 'enterprise';
  capabilities: Capability[];
}

/** Response of `POST /workspaces/{id}/email/test` for 200, 422 and 429. */
export interface TestEmailResult {
  ok: boolean;
  error?: string;
  recipient?: string;
  /** Set by the client when the server answered 429. */
  rate_limited?: boolean;
}
