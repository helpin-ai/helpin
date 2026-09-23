import { toast } from "sonner";
import { AIConnectionPolicyNotice } from "./AIConnectionPolicyNotice";
import { ProviderIcon, ProviderIconTile } from "./ProviderIcon";
import { useDeleteAIProfile } from "@/hooks/queries/useAIProfiles";
import { catalogLabel, providerLabel } from "@/lib/aiProviders";
import type { AIConnection } from "@/lib/services/aiConnectionService";
import type { AIProfile, AIProfileRoute } from "@/lib/services/aiProfileService";
import { useConfirm } from "@/components/ui/confirm-dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { QuickTooltip } from "@/components/ui/quick-tooltip";
import { TableCell, TableRow } from "@/components/ui/table";
import { CheckmarkCircle02Icon, Delete01Icon, PencilEdit01Icon } from "@/lib/icons";

function routeWarning(route: AIProfileRoute, connections: AIConnection[]): string | null {
  const connection = connections.find((entry) => entry.id === route.connection_id);
  if (!connection) return "Connection was removed";
  if (connection.status === "reauthorization_required") return "Connection needs reconnecting";
  if (connection.status === "disconnected") return "Connection is disconnected";
  if (connection.status === "pending") return "Connection is awaiting login";
  return null;
}

function RouteSummary({
  route,
  connections,
}: {
  route: AIProfileRoute;
  connections: AIConnection[];
}) {
  const connection = connections.find((entry) => entry.id === route.connection_id);
  const friendly = catalogLabel(route.model.provider, route.model.model);
  return (
    <div className="flex items-start gap-2">
      <ProviderIcon
        provider={route.model.provider}
        className="mt-0.5 h-3.5 w-3.5 shrink-0"
      />
      <div className="min-w-0">
        <p className="truncate text-sm">{friendly ?? route.model.model}</p>
        <p className="mt-0.5 truncate text-xs text-muted-foreground">
          {providerLabel(route.model.provider)}
          {friendly ? ` · ${route.model.model}` : ""}
          {connection ? ` · via ${connection.name}` : ""}
        </p>
      </div>
    </div>
  );
}

export function AIProfileRow({
  workspaceId,
  profile,
  connections,
  isDefault,
  canManage,
  canSetDefault,
  onEdit,
  onSetDefault,
  onClearDefault,
}: {
  workspaceId: string;
  profile: AIProfile;
  connections: AIConnection[];
  isDefault: boolean;
  canManage: boolean;
  canSetDefault: boolean;
  onEdit: (profile: AIProfile) => void;
  onSetDefault: (profile: AIProfile) => void;
  onClearDefault?: () => void;
}) {
  const confirm = useConfirm();
  const remove = useDeleteAIProfile(workspaceId);
  const warning = routeWarning(profile.primary, connections);
  const blocked = profile.primary_policy?.allowed === false;

  async function requestDelete() {
    const confirmed = await confirm({
      title: `Delete “${profile.name}”?`,
      description: isDefault
        ? "This is the workspace default. Agents and automation have no default until you choose another."
        : "Agents and runs that reference this profile will show it as unavailable.",
      confirmText: "Delete profile",
      variant: "destructive",
    });
    if (!confirmed) return;
    try {
      await remove.mutateAsync(profile);
      toast.success("Profile deleted");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Could not delete the profile.");
    }
  }

  return (
    <TableRow className="group/profile">
      <TableCell className="py-4">
        <div className="flex items-center gap-3">
          <ProviderIconTile provider={profile.primary.model.provider} />
          <div className="min-w-0">
            <div className="flex min-w-0 flex-wrap items-center gap-1.5">
              <p className="truncate text-sm font-medium">{profile.name}</p>
              {isDefault && (
                <Badge
                  variant="outline"
                  className="gap-1 border-emerald-300/70 bg-emerald-50 text-[10px] text-emerald-700 dark:border-emerald-900 dark:bg-emerald-950/30 dark:text-emerald-300"
                >
                  <CheckmarkCircle02Icon className="h-3 w-3" />
                  Default
                </Badge>
              )}
            </div>
            {warning ? (
              <p className="mt-0.5 truncate text-xs text-amber-700 dark:text-amber-400">{warning}</p>
            ) : (
              <p className="mt-0.5 truncate text-xs text-muted-foreground">
                {profile.scope === "personal" ? "Personal" : "Workspace"}
              </p>
            )}
          </div>
        </div>
      </TableCell>
      <TableCell>
        <RouteSummary route={profile.primary} connections={connections} />
        <div className="mt-1 text-xs text-muted-foreground">
          <AIConnectionPolicyNotice policy={profile.primary_policy} />
        </div>
      </TableCell>
      <TableCell>
        {profile.fallback ? (
          <>
            <RouteSummary route={profile.fallback} connections={connections} />
            <p className="mt-1 text-xs text-muted-foreground">Used before execution only</p>
            <div className="text-xs text-muted-foreground">
              <AIConnectionPolicyNotice policy={profile.fallback_policy} />
            </div>
          </>
        ) : (
          <span className="text-sm text-muted-foreground">No fallback</span>
        )}
      </TableCell>
      {canManage && (
        <TableCell className="text-right">
          <div className="flex justify-end gap-1 transition-opacity [@media(hover:hover)]:opacity-0 group-hover/profile:opacity-100 group-focus-within/profile:opacity-100">
            {canSetDefault && (isDefault ? !!onClearDefault : !blocked) && (
              <QuickTooltip label={isDefault ? "Clear workspace default" : "Set as workspace default"}>
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-8 w-8"
                  aria-label={isDefault ? `Clear ${profile.name} as the workspace default` : `Set ${profile.name} as the workspace default`}
                  onClick={() => isDefault ? onClearDefault?.() : onSetDefault(profile)}
                >
                  <CheckmarkCircle02Icon className="h-4 w-4" aria-hidden="true" />
                </Button>
              </QuickTooltip>
            )}
            <QuickTooltip label="Edit profile">
              <Button
                variant="ghost"
                size="icon"
                className="h-8 w-8"
                aria-label={`Edit ${profile.name}`}
                onClick={() => onEdit(profile)}
              >
                <PencilEdit01Icon className="h-4 w-4" aria-hidden="true" />
              </Button>
            </QuickTooltip>
            <QuickTooltip label="Delete profile">
              <Button
                variant="ghost"
                size="icon"
                className="h-8 w-8 text-destructive hover:bg-destructive/10 hover:text-destructive"
                disabled={remove.isPending}
                aria-label={`Delete ${profile.name}`}
                onClick={() => void requestDelete()}
              >
                <Delete01Icon className="h-4 w-4" aria-hidden="true" />
              </Button>
            </QuickTooltip>
          </div>
        </TableCell>
      )}
    </TableRow>
  );
}
