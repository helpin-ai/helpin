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
  defaultSource: "Agent" | "Personal" | "Workspace" | null,
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
  compact = false,
}: {
  workspaceId: string;
  value?: string | null;
  onChange: (value: string | null) => void;
  disabled?: boolean;
  sharedOnly?: boolean;
  inDock?: boolean;
  defaultProfileId?: string | null;
  compact?: boolean;
}) {
  const id = useId();
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const query = useAIProfiles(workspaceId);
  const settings = useAISettings(workspaceId, {
    enabled: !defaultProfileId && !!query.data?.length,
  });
  if (query.isPending)
    return (
      <p role="status" className="text-xs text-quiet-text-secondary">
        {compact ? "Loading…" : "Loading AI models…"}
      </p>
    );
  if (query.isError)
    return (
      <QuietTextAction type="button" onClick={() => void query.refetch()}>
        {compact ? "Retry models" : "Retry loading AI models"}
      </QuietTextAction>
    );
  const personalDefaultId = inDock && !sharedOnly ? settings.data?.personal_default_profile_id : null;
  const inheritedId = defaultProfileId || personalDefaultId || settings.data?.default_profile_id;
  const profiles = query.data.filter(
    (p) => (!sharedOnly || p.scope === "workspace") &&
      (!inDock || !p.hidden_from_ask_agent || p.id === value || p.id === inheritedId),
  );
  const selectedId =
    value || inheritedId;
  const selected = profiles.find((p) => p.id === selectedId);
  const blockedPolicy = selected?.primary_policy?.allowed === false;
  const inherited = profiles.find((p) => p.id === inheritedId);
  profiles.sort((a, b) =>
    Number(b.id === inheritedId) - Number(a.id === inheritedId)
    || Number(a.scope === "personal") - Number(b.scope === "personal")
    || a.name.localeCompare(b.name, undefined, { sensitivity: "base" })
    || a.id.localeCompare(b.id),
  );
  if (compact && !value && !defaultProfileId && settings.isError) {
    return <QuietTextAction type="button" title="Retry loading the default" onClick={() => void settings.refetch()}>Retry default</QuietTextAction>;
  }
  if (compact && !value && !disabled && profiles.length === 0 && workspace?.id === workspaceId) {
    return <AISettingsLink slug={workspace.slug} className="whitespace-nowrap text-xs text-quiet-text-secondary">Set up AI</AISettingsLink>;
  }
  const compactName = selected?.name ?? (value ? disabled ? "Saved model" : "Model unavailable" : settings.isFetching && !defaultProfileId ? "Loading…" : "Choose model");

  return (
    <div className={compact ? "min-w-0 max-w-full" : "min-w-0 space-y-2"}>
      {!compact && (
        <div className="space-y-1">
          <Label htmlFor={id}>AI model</Label>
          <p id={`${id}-help`} className="text-xs text-quiet-text-secondary">
            {sharedOnly
              ? "Workspace-wide models for this agent’s manual and automated runs."
              : inDock
                ? "Personal or workspace-wide models for this chat."
                : "Personal or workspace-wide models for this run only."}
          </p>
        </div>
      )}
      <Select
        value={value || inherited?.id || "default"}
        disabled={disabled}
        onValueChange={(next) =>
          // Choosing the profile the default points at keeps inheriting, so the
          // agent follows a later change to the default.
          onChange(next === "default" || next === inherited?.id ? null : next)
        }
      >
        <SelectTrigger id={id} variant={compact ? "ghost" : "underline"} size={compact ? "sm" : "default"}
          aria-label={compact ? `Change AI model: ${compactName}` : "AI model"}
          aria-describedby={!compact ? `${id}-help` : undefined}
          title={compact ? disabled ? "This conversation keeps its saved AI model" : "Change AI model" : undefined}
          className={compact ? "max-w-[180px] h-7 gap-1 rounded-md px-2 text-xs text-quiet-text-secondary hover:text-quiet-text-primary" : "w-full px-0.5"}>
          <SelectValue placeholder={sharedOnly ? "Workspace default" : "Agent default"}>
            {compact ? compactName : undefined}
          </SelectValue>
        </SelectTrigger>
        <SelectContent
          data-helpin-dock-overlay={inDock || undefined}
          align={compact ? "end" : "start"}
          side={compact ? "top" : undefined}
          className={`${inDock ? "z-[70] " : ""}${compact ? "w-[300px] max-w-[calc(100vw-24px)]" : ""}`}
        >
          {!inherited && !compact && (
            <SelectItem value="default" disabled={compact}>
              {compact ? "Choose model" : sharedOnly ? "Workspace default" : "Agent default"}
            </SelectItem>
          )}
          {value && !selected && (
            <SelectItem value={value} disabled>
              Saved model · unavailable for new runs
            </SelectItem>
          )}
          {profiles.map((p) => (
            <SelectItem
              key={p.id}
              value={p.id}
              textValue={p.name}
              disabled={p.primary_policy?.allowed === false}
            >
              <span className="flex min-w-0 w-full items-center gap-2">
                {!compact && <ProviderIcon provider={p.primary.model.provider} className="h-3.5 w-3.5 shrink-0" />}
                <span className="truncate">{p.name}</span>
                {p.id === inherited?.id && (
                  <Badge variant="secondary" className="shrink-0 text-[10px]">
                    Default
                  </Badge>
                )}
                <span className="ml-auto shrink-0 text-[11px] text-quiet-text-secondary">{p.scope === "personal" ? "Personal" : "Workspace"}</span>
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
      {!compact && !value && !defaultProfileId && settings.isError && (
        <QuietTextAction type="button" onClick={() => void settings.refetch()}>
          Retry loading the default route
        </QuietTextAction>
      )}
      {!compact && !value && !selected && !settings.isError && !settings.isFetching && (
        <p role="status" className="text-xs text-quiet-text-secondary">
          No available default model. Select a model or configure the
          workspace default.
        </p>
      )}
      {!compact && selected && (
        // One compact route line. The full detail (source, exact model,
        // fallback, pricing) is on hover so the launch form stays scannable.
        <QuickTooltip label={routeDetail(selected, value ? null : defaultProfileId ? "Agent" : personalDefaultId ? "Personal" : "Workspace")}>
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
      {!compact && blockedPolicy && (
        <AIConnectionPolicyNotice policy={selected?.primary_policy} />
      )}
      {!compact && workspace?.id === workspaceId && (!selected || blockedPolicy) && (
        <AISettingsLink
          className="text-xs underline text-quiet-text-secondary"
          slug={workspace.slug}
          shared={sharedOnly}
        >
          Manage AI models
        </AISettingsLink>
      )}
    </div>
  );
}
