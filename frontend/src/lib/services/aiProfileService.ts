import type {
  AIConnectionPolicyView,
  AIModelEndpoint,
} from "./aiConnectionService";
import { api } from "@/lib/api";

export interface AIModelControls {
  reasoning_effort?: string;
  service_tier?: string;
  openrouter?: { provider?: { quantizations?: string[] } };
}

export interface AIProfileRoute {
  connection_id: string;
  model: {
    provider: string;
    model: string;
    controls: AIModelControls;
    endpoint?: AIModelEndpoint;
  };
}

export interface AIProfile {
  hidden_from_ask_agent?: boolean;
  primary_policy?: AIConnectionPolicyView;
  fallback_policy?: AIConnectionPolicyView;
  id: string;
  workspace_id: string;
  user_id: string | null;
  scope: "personal" | "workspace";
  name: string;
  revision: number;
  primary: AIProfileRoute;
  fallback: AIProfileRoute | null;
}

export type SaveAIProfile = Pick<
  AIProfile,
  "name" | "scope" | "primary" | "fallback"
> & { revision?: number };

const path = (workspace: string, suffix = "") =>
  `/ai-profiles${suffix}?workspace_id=${encodeURIComponent(workspace)}`;

export interface AISettings {
  default_profile_id: string | null;
  personal_default_profile_id?: string | null;
}

export const aiProfileService = {
  setPersonalDefault: (workspace: string, default_profile_id: string | null) =>
    api.put(`/ai-settings/personal?workspace_id=${encodeURIComponent(workspace)}`, { default_profile_id }),
  enableModel: (workspace: string, value: {connection_id: string; model: string; name: string}) => api.post<AIProfile>(path(workspace, '/models'), value),
  setVisibility: (workspace: string, profile: AIProfile, hidden: boolean) =>
    api.put<AIProfile>(path(workspace, `/${encodeURIComponent(profile.id)}/visibility`), {revision: profile.revision, hidden_from_ask_agent: hidden}),
  list: (workspace: string) => api.get<AIProfile[]>(path(workspace, "/")),
  save: (workspace: string, value: SaveAIProfile, id?: string) =>
    id
      ? api.put<AIProfile>(path(workspace, `/${encodeURIComponent(id)}`), value)
      : api.post<AIProfile>(path(workspace, "/"), value),
  remove: (workspace: string, profile: AIProfile) =>
    api.del<void>(
      `${path(workspace, `/${encodeURIComponent(profile.id)}`)}&revision=${profile.revision}`,
    ),
  settings: (workspace: string) =>
    api.get<AISettings>(
      `/ai-settings?workspace_id=${encodeURIComponent(workspace)}`,
    ),
  setDefault: (workspace: string, default_profile_id: string | null) =>
    api.put(`/ai-settings?workspace_id=${encodeURIComponent(workspace)}`, {
      default_profile_id,
    }),
};
