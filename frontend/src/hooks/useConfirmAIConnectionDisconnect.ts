import { toast } from "sonner";
import { useDisconnectAIConnection } from "@/hooks/queries/useAIConnections";
import { useConfirm } from "@/components/ui/confirm-dialog";
import type { AIProfile } from "@/lib/services/aiProfileService";
import type { AIConnection } from "@/lib/services/aiConnectionService";

export function useConfirmAIConnectionDisconnect(
  workspaceId: string,
  profiles: AIProfile[],
) {
  const confirm = useConfirm();
  const disconnect = useDisconnectAIConnection(workspaceId);

  async function requestDisconnect(connection: AIConnection) {
    const affected = profiles.filter(
      (p) =>
        p.primary.connection_id === connection.id ||
        p.fallback?.connection_id === connection.id,
    );
    const labels = affected.map(
      (p) =>
        p.name +
        " (" +
        (p.agent_grant_id || p.scope === "workspace"
          ? "Workspace-wide"
          : "Personal") +
        ")",
    );
    const confirmed = await confirm({
      title: `Disconnect “${connection.name}”?`,
      description:
        "New runs using this connection will need another model." +
        (labels.length ? " Affected models: " + labels.join(", ") + "." : ""),
      confirmText: "Disconnect",
      variant: "destructive",
    });
    if (!confirmed) return;
    try {
      await disconnect.mutateAsync(connection.id);
      toast.success("Connection disconnected");
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : "Could not disconnect.",
      );
    }
  }

  return { requestDisconnect, pending: disconnect.isPending };
}
