import { AIConnectionPolicyNotice } from "./AIConnectionPolicyNotice";
import type { AIExecutionPolicySnapshot } from "@/lib/services/aiConnectionService";

function object(value: unknown): Record<string, unknown> | undefined {
  return value && typeof value === "object" && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : undefined;
}

// Read only the accepted run snapshot. Current profile edits and tariffs must
// never change the route or fee shown for an existing run.
export function AIExecutionDetails({
  input,
}: {
  input?: Record<string, unknown>;
}) {
  const selection = object(input?.ai_selection);
  const route = object(selection?.route);
  const model = object(route?.model);
  if (typeof model?.provider !== "string" || typeof model.model !== "string")
    return null;
  const endpoint = object(model.endpoint);
  const policy = object(selection?.policy);
  const pricing =
    policy &&
    (policy.mode === "ee" || policy.mode === "community") &&
    typeof policy.funding_mode === "string"
      ? (policy as unknown as AIExecutionPolicySnapshot)
      : undefined;
  return (
    <details className="px-3.5 py-2 text-xs text-quiet-text-secondary">
      <summary className="cursor-pointer">
        AI: {model.provider} · {model.model}
      </summary>
      <div className="mt-1 space-y-1">
        <p>
          {selection?.connection_scope === "personal"
            ? "Personal connection"
            : "Workspace connection"}
          {selection?.fallback_reason === "primary_connection_unavailable"
            ? " · Fallback selected before execution"
            : ""}
        </p>
        {typeof endpoint?.id === "string" && (
          <p>
            Endpoint: {endpoint.id} · {endpoint.auth_mode === "none" ? "No authentication" : "API key"}
          </p>
        )}
        <AIConnectionPolicyNotice policy={{ allowed: true, pricing }} />
      </div>
    </details>
  );
}
