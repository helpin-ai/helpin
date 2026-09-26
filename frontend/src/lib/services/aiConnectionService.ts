import { api } from "@/lib/api";

export interface AIExecutionPolicySnapshot {
  mode: "community" | "ee";
  funding_mode: string;
  flat_tariff?: {
    version: string;
    currency: string;
    microusd_per_million: number;
    accounting_version: string;
  };
}
export interface AIConnectionPolicyView {
  allowed: boolean;
  message?: string;
  pricing?: AIExecutionPolicySnapshot;
}

export interface AIModelEndpoint {
  id: string;
  base_url: string;
  auth_mode: "api_key" | "none";
}

export interface AIConnection {
  endpoint?: AIModelEndpoint;
  funding?: "customer" | "managed";
  policy?: AIConnectionPolicyView;
  id: string;
  scope: "personal" | "workspace";
  user_id: string | null;
  name: string;
  provider: string;
  status: "pending" | "connected" | "reauthorization_required" | "disconnected";
  account_id?: string;
  expires_at?: string;
  /** Time of the last explicit connection test. */
  last_verified_at?: string;
  /** Error from the last test; absent alongside `last_verified_at` when it passed. */
  last_verification_error?: string;
}
/** Result of a live connection test. */
export interface AIConnectionTestResult {
  ok: boolean;
  model: string;
  latency_ms: number;
  error?: string;
}
export interface AIConnectionModel {
  provider: string;
  selection_model: string;
  label: string;
  tier: string;
}
export interface AIConnectionSelection {
  ai_profile_id?: string;
  model_connection_id?: string;
  model_name?: string;
}
export interface AIConnectionLogin {
  connection: AIConnection;
  verification_url?: string;
  user_code?: string;
  expires_at?: string;
  interval_seconds?: number;
}
const path = (workspace: string, suffix = "") =>
  `/ai-connections${suffix}?workspace_id=${encodeURIComponent(workspace)}`;
export const aiConnectionService = {
  endpoints: (workspace: string) =>
    api.get<AIModelEndpoint[] | null>(path(workspace, "/endpoints")),
  list: (workspace: string) =>
    api.get<{
      enabled: boolean;
      knowledge?: {
        embeddings_configured: boolean;
        embedding_model: string;
        embedding_dimensions: number;
        /** "server", "workspace", or "" when no provider serves this workspace. */
        embedding_source?: "server" | "workspace" | "";
        embedding_provider?: string;
        embedding_detail?: string;
        chat_providers: string[];
      };
      connections: AIConnection[];
      models: AIConnectionModel[];
    }>(path(workspace, "/")),
  create: (
    workspace: string,
    request: {
      name: string;
      provider: string;
      api_key?: string;
      scope?: "personal" | "workspace";
      endpoint_id?: string;
    },
  ) => api.post<AIConnectionLogin>(path(workspace, "/"), request),
  poll: (workspace: string, id: string) =>
    api.post<AIConnectionLogin>(
      path(workspace, `/${encodeURIComponent(id)}/poll`),
      {},
    ),
  reconnect: (workspace: string, id: string, api_key?: string) =>
    api.post<AIConnectionLogin>(
      path(workspace, `/${encodeURIComponent(id)}/reconnect`),
      { api_key },
    ),
  test: (workspace: string, id: string, model?: string) =>
    api.post<AIConnectionTestResult>(
      path(workspace, `/${encodeURIComponent(id)}/test`),
      model ? { model } : {},
    ),
  disconnect: (workspace: string, id: string) =>
    api.del<void>(path(workspace, `/${encodeURIComponent(id)}`)),
};
