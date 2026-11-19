import { useContext, useState } from "react";
import { AIModelsPricingContext } from "./AIModelsPricingContext";
import { toast } from "sonner";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from "@/components/ui/dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { TableCell, TableRow } from "@/components/ui/table";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
} from "@/components/ui/dropdown-menu";
import { QuickTooltip } from "@/components/ui/quick-tooltip";
import { ArrowDown01Icon, MoreHorizontalIcon } from "@/lib/icons";
import {
  catalogLabel,
  modelNameWithThinking,
  connectionStatusInfo,
  providerLabel,
} from "@/lib/aiProviders";
import type { AIConnection } from "@/lib/services/aiConnectionService";
import type { AIProfile, AISettings } from "@/lib/services/aiProfileService";
import {
  useSetAgentSharing,
  useSetPersonalDefaultAIProfile,
  useSetDefaultAIProfile,
  useSetAIModelVisibility,
} from "@/hooks/queries/useAIProfiles";
import { ProviderIcon } from "./ProviderIcon";
import { AISetupHelp } from "./AISetupHelp";
import { aiUsagePricingText } from "@edition/ai";

export function AIModelTableRow({
  workspaceId,
  profile,
  connection,
  settings,
  canManageWorkspace,
  onEdit,
  onReconnect,
}: {
  workspaceId: string;
  profile: AIProfile;
  connection?: AIConnection;
  settings?: AISettings;
  canManageWorkspace: boolean;
  onEdit: () => void;
  onReconnect: () => void;
}) {
  const sharedPricing = useContext(AIModelsPricingContext);
  const personalDefault = useSetPersonalDefaultAIProfile(workspaceId);
  const workspaceDefault = useSetDefaultAIProfile(workspaceId);
  const visibility = useSetAIModelVisibility(workspaceId);
  const sharing = useSetAgentSharing(workspaceId);
  const [accessChange, setAccessChange] = useState<{ profile: AIProfile; shared: boolean } | null>(null);
  const accessLabel = profile.scope === "workspace" || profile.agent_grant_id ? "Workspace-wide" : "Personal";
  const busy = sharing.isPending ||
    personalDefault.isPending ||
    workspaceDefault.isPending ||
    visibility.isPending;
  const editable = profile.scope === "personal" ? !!connection || !profile.agent_grant_id : canManageWorkspace;
  const isPersonalDefault =
    settings?.personal_default_profile_id === profile.id;
  const isWorkspaceDefault = settings?.default_profile_id === profile.id;
  const isDefault =
    (settings?.personal_default_profile_id || settings?.default_profile_id) ===
    profile.id;
  const policy = profile.primary_policy ?? connection?.policy;
  const pricing = aiUsagePricingText(policy?.pricing);
  const connectionStatus = connection?.status ?? profile.subscription_status;
  const ready =
    !!settings &&
    connectionStatus === "connected" &&
    policy?.allowed !== false &&
    !profile.hidden_from_ask_agent;
  const info = connectionStatus ? connectionStatusInfo(connectionStatus) : null;
  const status = !info
    ? "Connection missing"
    : policy?.allowed === false
      ? "Unavailable"
      : profile.hidden_from_ask_agent
        ? "Hidden from pickers"
        : info.label;
  const statusTone =
    policy?.allowed === false || connectionStatus !== "connected"
      ? "text-amber-700 dark:text-amber-400"
      : profile.hidden_from_ask_agent
        ? "text-muted-foreground"
        : "text-quiet-positive";
  const modelName =
    catalogLabel(profile.primary.model.provider, profile.primary.model.model) ??
    profile.primary.model.model;
  const displayName = modelNameWithThinking(modelName, profile.primary.model.controls.reasoning_effort);
  const normalizedName = (name: string) =>
    name.toLowerCase().replace(/[^a-z0-9]/g, "");
  const hasCustomName =
    normalizedName(profile.name) !== normalizedName(modelName) &&
    normalizedName(profile.name) !==
      normalizedName(profile.primary.model.model);
  async function update(action: () => Promise<unknown>, message: string) {
    try {
      await action();
      toast.success(message);
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : "Could not update model.",
      );
    }
  }
  return (
    <TableRow>
      <TableCell className="min-w-[180px] max-w-[300px] py-4">
        <div className="flex flex-wrap items-center gap-2">
          <span className="truncate text-sm font-medium" title={displayName}>
            {displayName}
          </span>
          {isDefault && (
            <QuickTooltip
              label={
                isPersonalDefault
                  ? "Your default for new Ask Agent chats in this workspace. Saved chats and explicit agent choices keep their models."
                  : "Used for new Ask Agent chats and agents without their own model. Make my default sets a personal choice for new chats only."
              }
            >
              <Badge variant="secondary" tabIndex={0}>
                Default
              </Badge>
            </QuickTooltip>
          )}
          {!isDefault && isWorkspaceDefault && canManageWorkspace && (
            <QuickTooltip label="Shared default for workspace members without a personal choice, and agents without an explicit model.">
              <Badge variant="secondary" tabIndex={0}>
                Workspace default
              </Badge>
            </QuickTooltip>
          )}
        </div>
        {hasCustomName && (
          <p
            className="mt-0.5 truncate text-xs text-muted-foreground"
            title={profile.name}
          >
            {profile.name}
          </p>
        )}
        {profile.fallback && (
          <span className="sr-only">Fallback configured</span>
        )}
      </TableCell>
      <TableCell className="max-w-[220px]">
        <div className="flex items-center gap-2">
          <ProviderIcon
            provider={connection?.provider ?? profile.primary.model.provider}
            className="size-4 shrink-0"
          />
          <span className="truncate text-sm" title={connection?.name}>
            {connection?.name ?? profile.subscription_name ?? providerLabel(profile.primary.model.provider)}
          </span>
        </div>
      </TableCell>
      <TableCell className="whitespace-nowrap text-sm">
        {connection?.provider === "openai_chatgpt" && connection.scope === "personal" ? (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="sm" className="-ml-2" disabled={busy} aria-label={`Access for ${profile.name}: ${accessLabel}`}>
                {accessLabel}<ArrowDown01Icon className="size-3" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="start">
              <DropdownMenuItem disabled={!profile.agent_grant_id} onSelect={() => setAccessChange({ profile, shared: false })}>Personal</DropdownMenuItem>
              <DropdownMenuItem disabled={!!profile.agent_grant_id || connection.status !== "connected"} onSelect={() => setAccessChange({ profile, shared: true })}>Workspace-wide</DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        ) : accessLabel}
      </TableCell>
      <TableCell>
        <div className="flex items-center gap-1 whitespace-nowrap">
          <span className={`text-xs ${statusTone}`}>{status}</span>
          <AISetupHelp
            label={`Status details for ${profile.name}`}
            description={
              !connection && profile.agent_grant_id ? "Shared by the subscription owner for workspace agent runs." : !connection
                ? "Choose another connection in Model settings. Your saved configuration and history are preserved."
                : policy?.allowed === false
                  ? (policy.message ?? "Unavailable for new runs.")
                  : profile.hidden_from_ask_agent
                    ? "Hidden from new choices in Ask Agent and agent model pickers. Existing selections and defaults keep working. You can always manage it here."
                    : connection.status === "connected"
                      ? "The connection is available. This does not verify that the provider can run this particular model."
                      : "Reconnect this connection to use its models for new runs."
            }
          />
          {pricing && pricing !== sharedPricing && (
            <QuickTooltip
              label={pricing}
            >
              <button
                type="button"
                className="text-xs text-muted-foreground underline"
                aria-label={`Usage details for ${profile.name}`}
              >
                Usage
              </button>
            </QuickTooltip>
          )}
        </div>
      </TableCell>
      <TableCell className="text-right">
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              variant="ghost"
              size="icon"
              className="size-8"
              aria-label={`Actions for ${profile.name}`}
              disabled={busy}
            >
              <MoreHorizontalIcon className="size-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem
              disabled={!settings || (!isPersonalDefault && (!ready || (!connection && !!profile.agent_grant_id)))}
              onSelect={() =>
                void update(
                  () =>
                    personalDefault.mutateAsync(
                      isPersonalDefault ? null : profile.id,
                    ),
                  isPersonalDefault
                    ? "Using workspace default"
                    : "Personal default updated",
                )
              }
            >
              {isPersonalDefault ? "Use workspace default" : "Make my default"}
            </DropdownMenuItem>
            {canManageWorkspace && profile.scope === "workspace" && (
              <DropdownMenuItem
                disabled={!settings || (!isWorkspaceDefault && !ready)}
                onSelect={() =>
                  void update(
                    () =>
                      workspaceDefault.mutateAsync(
                        isWorkspaceDefault ? null : profile.id,
                      ),
                    "Workspace default updated",
                  )
                }
              >
                {isWorkspaceDefault
                  ? "Clear workspace default"
                  : "Set workspace default"}
              </DropdownMenuItem>
            )}
            {editable && (
              <>
                <DropdownMenuSeparator />
                <DropdownMenuItem onSelect={onEdit}>
                  Model settings
                </DropdownMenuItem>
                {connection &&
                  connection.status !== "connected" &&
                  connection.funding !== "managed" && (
                    <DropdownMenuItem onSelect={onReconnect}>
                      {connection.status === "pending"
                        ? "Continue login"
                        : "Reconnect connection"}
                    </DropdownMenuItem>
                  )}
                <DropdownMenuItem
                  onSelect={() =>
                    void update(
                      () =>
                        visibility.mutateAsync({
                          profile,
                          hidden: !profile.hidden_from_ask_agent,
                        }),
                      profile.hidden_from_ask_agent
                        ? "Model shown in model pickers"
                        : "Model hidden from model pickers",
                    )
                  }
                >
                  {profile.hidden_from_ask_agent
                    ? "Show in model pickers"
                    : "Hide from model pickers"}
                </DropdownMenuItem>
              </>
            )}
          </DropdownMenuContent>
        </DropdownMenu>
        <Dialog open={accessChange !== null} onOpenChange={open => { if (!open) setAccessChange(null); }}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{accessChange?.shared ? "Share this model with the workspace?" : "Make this model personal?"}</DialogTitle>
              <DialogDescription>{!accessChange?.shared
                ? "Agents using this shared model will need another model. Active shared runs lose access."
                : `Workspace members and scheduled agents can use ${profile.name} through your ${connection?.name || "ChatGPT"} subscription.`}</DialogDescription>
            </DialogHeader>
            <DialogFooter>
              <Button variant="ghost" onClick={() => setAccessChange(null)}>Cancel</Button>
              <Button disabled={sharing.isPending} onClick={() => void update(async () => {
                if (!accessChange) return;
                await sharing.mutateAsync(accessChange);
                setAccessChange(null);
              }, "Model access updated")}>{accessChange?.shared ? "Share model" : "Make personal"}</Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </TableCell>
    </TableRow>
  );
}
