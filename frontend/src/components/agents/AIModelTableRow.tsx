import { toast } from "sonner";
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
import { MoreHorizontalIcon } from "@/lib/icons";
import {
  catalogLabel,
  connectionStatusInfo,
  providerLabel,
} from "@/lib/aiProviders";
import type { AIConnection } from "@/lib/services/aiConnectionService";
import type { AIProfile, AISettings } from "@/lib/services/aiProfileService";
import {
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
  const personalDefault = useSetPersonalDefaultAIProfile(workspaceId);
  const workspaceDefault = useSetDefaultAIProfile(workspaceId);
  const visibility = useSetAIModelVisibility(workspaceId);
  const busy =
    personalDefault.isPending ||
    workspaceDefault.isPending ||
    visibility.isPending;
  const editable = profile.scope === "personal" || canManageWorkspace;
  const isPersonalDefault =
    settings?.personal_default_profile_id === profile.id;
  const isWorkspaceDefault = settings?.default_profile_id === profile.id;
  const isDefault =
    (settings?.personal_default_profile_id || settings?.default_profile_id) ===
    profile.id;
  const policy = profile.primary_policy ?? connection?.policy;
  const ready =
    !!settings &&
    connection?.status === "connected" &&
    policy?.allowed !== false &&
    !profile.hidden_from_ask_agent;
  const info = connection ? connectionStatusInfo(connection.status) : null;
  const status = !connection
    ? "Connection missing"
    : policy?.allowed === false
      ? "Unavailable"
      : profile.hidden_from_ask_agent
        ? "Hidden from pickers"
        : info!.label;
  const statusTone =
    !connection ||
    policy?.allowed === false ||
    connection.status !== "connected"
      ? "text-amber-700 dark:text-amber-400"
      : profile.hidden_from_ask_agent
        ? "text-muted-foreground"
        : "text-quiet-positive";
  const modelName =
    catalogLabel(profile.primary.model.provider, profile.primary.model.model) ??
    profile.primary.model.model;
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
          <span className="truncate text-sm font-medium" title={modelName}>
            {modelName}
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
            {connection?.name ?? providerLabel(profile.primary.model.provider)}
          </span>
        </div>
      </TableCell>
      <TableCell className="whitespace-nowrap text-sm">
        {profile.scope === "personal" ? "Personal" : "Workspace-wide"}
      </TableCell>
      <TableCell>
        <div className="flex items-center gap-1 whitespace-nowrap">
          <span className={`text-xs ${statusTone}`}>{status}</span>
          <AISetupHelp
            label={`Status details for ${profile.name}`}
            description={
              !connection
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
          {policy?.pricing && (
            <QuickTooltip
              label={
                aiUsagePricingText(policy.pricing) ??
                "Usage follows your connection policy."
              }
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
              disabled={!settings || (!isPersonalDefault && !ready)}
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
      </TableCell>
    </TableRow>
  );
}
