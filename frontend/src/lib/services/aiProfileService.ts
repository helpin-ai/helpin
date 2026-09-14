import { api } from "@/lib/api";

export interface AIModelControls {
  reasoning_effort?: string;
  service_tier?: string;
  openrouter?: { provider?: { quantizations?: string[] } };
}

export interface AIProfileRoute {
  connection_id: string;
  model: { provider: string; model: string; controls: AIModelControls };
}

export interface AIProfile {
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

export const aiProfileService = {
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
    api.get<{ default_profile_id: string | null }>(
      `/ai-settings?workspace_id=${encodeURIComponent(workspace)}`,
    ),
  setDefault: (workspace: string, default_profile_id: string | null) =>
    api.put(`/ai-settings?workspace_id=${encodeURIComponent(workspace)}`, {
      default_profile_id,
    }),
};
