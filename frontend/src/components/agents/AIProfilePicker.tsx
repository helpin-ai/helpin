import { AIConnectionPolicyNotice } from "./AIConnectionPolicyNotice";
import { useId } from "react";
import { useQuery } from "@tanstack/react-query";
import { AISettingsLink } from "@/components/agents/AISettingsLink";
import { aiProfileService } from "@/lib/services/aiProfileService";
import { useWorkspaceStore } from "@/stores/workspaceStore";
import { Label } from "@/components/ui/label";
import { QuietTextAction } from "@/components/design-system/quiet";
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from "@/components/design-system/quiet-dropdown-select";

export function AIProfilePicker({
  workspaceId,
  value,
  onChange,
  disabled = false,
  sharedOnly = false,
  inDock = false,
  defaultProfileId,
}: {
  workspaceId: string;
  value?: string | null;
  onChange: (value: string | null) => void;
  disabled?: boolean;
  sharedOnly?: boolean;
  inDock?: boolean;
  defaultProfileId?: string | null;
}) {
  const id = useId();
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const query = useQuery({
    queryKey: ["ai-profiles", workspaceId],
    enabled: !!workspaceId,
    queryFn: async () => {
      const res = await aiProfileService.list(workspaceId);
      if (res.error || !res.data)
        throw new Error(res.error || "Unable to load AI profiles");
      return res.data;
    },
  });
  const settings = useQuery({
    queryKey: ["ai-settings", workspaceId],
    enabled:
      !!workspaceId && !value && !defaultProfileId && !!query.data?.length,
    queryFn: async () => {
      const result = await aiProfileService.settings(workspaceId);
      if (result.error || !result.data)
        throw new Error(result.error || "Unable to load AI default");
      return result.data;
    },
    retry: false,
  });
  if (query.isPending)
    return (
      <p role="status" className="text-xs text-quiet-text-secondary">
        Loading AI profiles…
      </p>
    );
  if (query.isError)
    return (
      <QuietTextAction type="button" onClick={() => void query.refetch()}>
        Retry loading AI profiles
      </QuietTextAction>
    );
  const profiles = query.data.filter(
    (p) => !sharedOnly || p.scope === "workspace",
  );
  const selectedId =
    value || defaultProfileId || settings.data?.default_profile_id;
  const selected = profiles.find((p) => p.id === selectedId);
  return (
    <div className="min-w-0 space-y-2">
      <Label htmlFor={id}>AI profile</Label>
      <Select
        value={value || "default"}
        disabled={disabled}
        onValueChange={(next) => onChange(next === "default" ? null : next)}
      >
        <SelectTrigger id={id} variant="underline" className="w-full px-0.5">
          <SelectValue
            placeholder={sharedOnly ? "Workspace default" : "Agent default"}
          />
        </SelectTrigger>
        <SelectContent
          data-helpin-dock-overlay={inDock || undefined}
          className={inDock ? "z-[70]" : undefined}
        >
          <SelectItem value="default">
            {sharedOnly ? "Workspace default" : "Agent default"}
          </SelectItem>
          {value && !selected && (
            <SelectItem value={value} disabled>
              Saved profile · unavailable for new runs
            </SelectItem>
          )}
          {profiles.map((p) => (
            <SelectItem
              key={p.id}
              value={p.id}
              disabled={p.primary_policy?.allowed === false}
            >
              {p.name} · {p.scope === "personal" ? "Personal" : "Workspace"}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      {!value && selected && (
        <p className="text-xs text-quiet-text-secondary">
          {defaultProfileId ? "Agent" : "Workspace"} default: {selected.name}
        </p>
      )}
      {!value && !defaultProfileId && settings.isError && (
        <QuietTextAction type="button" onClick={() => void settings.refetch()}>
          Retry loading the default route
        </QuietTextAction>
      )}
      {!value && !selected && !settings.isError && !settings.isFetching && (
        <p role="status" className="text-xs text-quiet-text-secondary">
          No available default profile. Select a profile or configure the
          workspace default.
        </p>
      )}
      {selected && (
        <p className="text-xs text-quiet-text-secondary">
          {selected.primary.model.provider} · {selected.primary.model.model}
          {selected.fallback
            ? ` · Fallback: ${selected.fallback.model.provider} / ${selected.fallback.model.model}, before execution only`
            : " · No fallback"}
        </p>
      )}
      {selected && (
        <>
          <AIConnectionPolicyNotice
            policy={selected.primary_policy}
            label="Primary"
          />
          {selected.fallback && (
            <AIConnectionPolicyNotice
              policy={selected.fallback_policy}
              label="Fallback"
            />
          )}
        </>
      )}
      {workspace?.id === workspaceId && (
        <AISettingsLink
          className="text-xs underline text-quiet-text-secondary"
          slug={workspace.slug}
          shared={sharedOnly}
        >
          Manage AI profiles
        </AISettingsLink>
      )}
    </div>
  );
}
