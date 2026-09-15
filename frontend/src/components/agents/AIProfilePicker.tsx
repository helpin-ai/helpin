import { AIConnectionPolicyNotice } from "./AIConnectionPolicyNotice";
import { aiUsagePricingText } from "@edition/ai";
import type { AIProfile } from "@/lib/services/aiProfileService";
import { ProviderIcon } from "./ProviderIcon";
import { useId } from "react";
import { AISettingsLink } from "@/components/agents/AISettingsLink";
import { useAIProfiles, useAISettings } from "@/hooks/queries/useAIProfiles";
import { catalogLabel, providerShortLabel } from "@/lib/aiProviders";
import { QuickTooltip } from "@/components/ui/quick-tooltip";
import { Badge } from "@/components/ui/badge";
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

function routeSummary(route: {
  model: { provider: string; model: string };
}): string {
  return `${providerShortLabel(route.model.provider)} · ${route.model.model}`;
}

/** Everything the compact route line leaves to hover. */
function routeDetail(
  profile: AIProfile,
  defaultSource: "Agent" | "Workspace" | null,
): string {
  const parts = [
    defaultSource ? `${defaultSource} default: ${profile.name}` : profile.name,
    `Primary: ${routeSummary(profile.primary)}`,
    profile.fallback
      ? `Fallback before execution: ${routeSummary(profile.fallback)}`
      : "No fallback",
  ];
  const pricing = profile.primary_policy?.pricing;
  const fee = pricing ? aiUsagePricingText(pricing) : null;
  if (fee) parts.push(fee);
  return parts.join(" · ");
}

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
  const query = useAIProfiles(workspaceId);
  const settings = useAISettings(workspaceId, {
    enabled: !value && !defaultProfileId && !!query.data?.length,
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
  const blockedPolicy = selected?.primary_policy?.allowed === false;
  const inheritedId = defaultProfileId || settings.data?.default_profile_id;
  const inherited = profiles.find((p) => p.id === inheritedId);
  return (
    <div className="min-w-0 space-y-2">
      <Label htmlFor={id}>AI profile</Label>
      <Select
        value={value || inherited?.id || "default"}
        disabled={disabled}
        onValueChange={(next) =>
          // Choosing the profile the default points at keeps inheriting, so the
          // agent follows a later change to the default.
          onChange(next === "default" || next === inherited?.id ? null : next)
        }
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
          {!inherited && (
            <SelectItem value="default">
              {sharedOnly ? "Workspace default" : "Agent default"}
            </SelectItem>
          )}
          {value && !selected && (
            <SelectItem value={value} disabled>
              Saved profile · unavailable for new runs
            </SelectItem>
          )}
          {profiles.map((p) => (
            <SelectItem
              key={p.id}
              value={p.id}
              textValue={p.name}
              disabled={p.primary_policy?.allowed === false}
            >
              <span className="flex items-center gap-2">
                <ProviderIcon
                  provider={p.primary.model.provider}
                  className="h-3.5 w-3.5 shrink-0"
                />
                <span className="truncate">
                  {p.name} · {p.scope === "personal" ? "Personal" : "Workspace"}
                </span>
                {p.id === inherited?.id && (
                  <Badge variant="secondary" className="shrink-0 text-[10px]">
                    Default
                  </Badge>
                )}
              </span>
              {p.primary_policy?.allowed === false && (
                <span className="block text-[11px] text-quiet-text-tertiary">
                  {p.primary_policy.message ?? "Unavailable for new runs"}
                </span>
              )}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
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
        // One compact route line. The full detail (source, exact model,
        // fallback, pricing) is on hover so the launch form stays scannable.
        <QuickTooltip label={routeDetail(selected, value ? null : defaultProfileId ? "Agent" : "Workspace")}>
          <p className="flex w-fit items-center gap-1.5 text-xs text-quiet-text-secondary">
            <ProviderIcon
              provider={selected.primary.model.provider}
              className="h-3 w-3 shrink-0"
            />
            <span className="truncate">
              {providerShortLabel(selected.primary.model.provider)} ·{" "}
              {catalogLabel(
                selected.primary.model.provider,
                selected.primary.model.model,
              ) ?? selected.primary.model.model}
            </span>
          </p>
        </QuickTooltip>
      )}
      {blockedPolicy && (
        <AIConnectionPolicyNotice policy={selected?.primary_policy} />
      )}
      {workspace?.id === workspaceId && (!selected || blockedPolicy) && (
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
