import { AISettingsLink } from "@/components/agents/AISettingsLink";
import { useWorkspaceStore } from "@/stores/workspaceStore";
import type { AIConnectionSelection } from "@/lib/services/aiConnectionService";
import { AIProfilePicker } from "./AIProfilePicker";

// Keep the existing launch-surface adapter while legacy request fields remain supported.
export function AIConnectionPicker({
  workspaceId,
  value,
  onChange,
  disabled = false,
  locked = false,
}: {
  workspaceId: string;
  value: AIConnectionSelection;
  onChange: (value: AIConnectionSelection) => void;
  disabled?: boolean;
  locked?: boolean;
}) {
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  if (locked && value.model_connection_id && !value.ai_profile_id) {
    return (
      <div className="space-y-2">
        <p className="text-xs text-quiet-text-secondary">
          This run keeps its saved AI connection
          {value.model_name ? ` · ${value.model_name}` : ""}.
        </p>
        {workspace?.id === workspaceId && (
          <AISettingsLink
            className="text-xs underline"
            slug={workspace.slug}
          >
            Manage my AI connections
          </AISettingsLink>
        )}
      </div>
    );
  }
  return (
    <AIProfilePicker
      workspaceId={workspaceId}
      value={value.ai_profile_id}
      onChange={(id) => onChange(id ? { ai_profile_id: id } : {})}
      disabled={disabled || locked}
    />
  );
}
