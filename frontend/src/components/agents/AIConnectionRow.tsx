import { toast } from "sonner";
import { AIConnectionPolicyNotice } from "./AIConnectionPolicyNotice";
import { AIConnectionStatusBadge } from "./AIConnectionStatusBadge";
import { ProviderIconTile } from "./ProviderIcon";
import { useDisconnectAIConnection } from "@/hooks/queries/useAIConnections";
import { providerLabel } from "@/lib/aiProviders";
import type { AIConnection } from "@/lib/services/aiConnectionService";
import { useConfirm } from "@/components/ui/confirm-dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { QuickTooltip } from "@/components/ui/quick-tooltip";
import { TableCell, TableRow } from "@/components/ui/table";
import { ArrowReloadHorizontalIcon, Key01Icon, Unlink01Icon } from "@/lib/icons";

function expiryLabel(connection: AIConnection): string | null {
  if (!connection.expires_at || connection.status === "disconnected") return null;
  const at = new Date(connection.expires_at);
  return Number.isFinite(at.getTime())
    ? `Access expires ${at.toLocaleTimeString([], { hour: "numeric", minute: "2-digit" })}`
    : null;
}

export function AIConnectionRow({
  workspaceId,
  connection,
  canManage,
  onReconnect,
  onContinueLogin,
  onModels,
  modelCount,
}: {
  workspaceId: string;
  connection: AIConnection;
  canManage: boolean;
  onReconnect: (connection: AIConnection) => void;
  onContinueLogin: (connection: AIConnection) => void;
  onModels?: () => void;
  modelCount?: number;
}) {
  const confirm = useConfirm();
  const disconnect = useDisconnectAIConnection(workspaceId);
  const managed = connection.funding === "managed";
  const expiry = expiryLabel(connection);

  async function requestDisconnect() {
    const confirmed = await confirm({
      title: `Disconnect “${connection.name}”?`,
      description:
        "Models that use this connection stop working for new runs. Runs already accepted keep their route.",
      confirmText: "Disconnect",
      variant: "destructive",
    });
    if (!confirmed) return;
    try {
      await disconnect.mutateAsync(connection.id);
      toast.success("Connection disconnected");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Could not disconnect.");
    }
  }

  return (
    <TableRow className="group/connection">
      <TableCell className="py-4">
        <div className="flex items-center gap-3">
          <ProviderIconTile provider={connection.provider} />
          <div className="min-w-0">
            <p className="truncate text-sm font-medium">{connection.name}</p>
            <p className="mt-0.5 truncate text-xs text-muted-foreground">
              {connection.scope === "personal" ? "Personal" : "Workspace"}
              {managed ? " · Managed by your administrator" : ""}
            </p>
          </div>
        </div>
      </TableCell>
      <TableCell>
        <span className="text-sm">{providerLabel(connection.provider)}</span>
      </TableCell>
      <TableCell>
        <div className="flex flex-wrap items-center gap-1.5">
          <AIConnectionStatusBadge status={connection.status} />
          {managed && (
            <Badge variant="secondary" className="text-[10px]">
              Managed
            </Badge>
          )}
        </div>
      </TableCell>
      <TableCell>
        <div className="max-w-[320px] space-y-0.5 text-xs text-muted-foreground">
          {connection.endpoint && (
            <p className="break-all">
              {connection.endpoint.base_url}
              {connection.endpoint.auth_mode === "none" ? " · No API key is sent" : ""}
            </p>
          )}
          <AIConnectionPolicyNotice policy={connection.policy} />
          {expiry && <p>{expiry}</p>}
        </div>
      </TableCell>
      {onModels && <TableCell className="text-right"><Button variant="outline" size="sm" aria-label={`Models for ${connection.name}`} onClick={onModels}>Models{modelCount !== undefined ? ` · ${modelCount}` : ''}</Button></TableCell>}
      {canManage && (
        <TableCell className="text-right">
          {managed ? (
            <span className="text-xs text-muted-foreground">Managed</span>
          ) : (
            <div className="flex justify-end gap-1 transition-opacity [@media(hover:hover)]:opacity-0 group-hover/connection:opacity-100 group-focus-within/connection:opacity-100 data-[attention=true]:opacity-100" data-attention={connection.status !== "connected"}>
              {connection.status === "pending" ? (
                <QuickTooltip label="Continue login">
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-8 w-8"
                    aria-label={`Continue login for ${connection.name}`}
                    onClick={() => onContinueLogin(connection)}
                  >
                    <Key01Icon className="h-4 w-4" aria-hidden="true" />
                  </Button>
                </QuickTooltip>
              ) : (
                <QuickTooltip label="Reconnect">
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-8 w-8"
                    aria-label={`Reconnect ${connection.name}`}
                    onClick={() => onReconnect(connection)}
                  >
                    <ArrowReloadHorizontalIcon className="h-4 w-4" aria-hidden="true" />
                  </Button>
                </QuickTooltip>
              )}
              <QuickTooltip label="Disconnect">
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-8 w-8 text-destructive hover:bg-destructive/10 hover:text-destructive"
                  disabled={disconnect.isPending}
                  aria-label={`Disconnect ${connection.name}`}
                  onClick={() => void requestDisconnect()}
                >
                  <Unlink01Icon className="h-4 w-4" aria-hidden="true" />
                </Button>
              </QuickTooltip>
            </div>
          )}
        </TableCell>
      )}
    </TableRow>
  );
}
