import { useAgents } from "@/hooks/queries/useAgents";
import { useDeleteAIProfile } from "@/hooks/queries/useAIProfiles";
import type { AIProfile } from "@/lib/services/aiProfileService";
import { Button } from "@/components/ui/button";
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";

export function RemoveAIModelDialog({
  workspaceId,
  profile,
  onClose,
}: {
  workspaceId: string;
  profile: AIProfile;
  onClose: () => void;
}) {
  const agents = useAgents(workspaceId);
  const remove = useDeleteAIProfile(workspaceId);
  const assigned =
    agents.data?.filter((agent) => agent.ai_profile_id === profile.id) ?? [];
  const blocked = assigned.length > 0 || !!profile.agent_grant_id;
  return (
    <AlertDialog
      open
      onOpenChange={(open) => {
        if (!open && !remove.isPending) onClose();
      }}
    >
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Remove “{profile.name}”?</AlertDialogTitle>
          <AlertDialogDescription>
            {assigned.length > 0
              ? "Choose another model for these agents before removing this one."
              : "Remove this saved model from AI Models and model pickers."}
          </AlertDialogDescription>
        </AlertDialogHeader>
        {assigned.length > 0 && (
          <ul className="max-h-44 list-disc overflow-y-auto pl-5 text-sm">
            {assigned.map((agent) => (
              <li key={agent.id}>{agent.name}</li>
            ))}
          </ul>
        )}
        {profile.agent_grant_id && (
          <p className="text-sm">
            Change this model’s access to Personal in AI Models before removing
            it.
          </p>
        )}
        {agents.isLoading && (
          <p role="status" className="text-sm text-muted-foreground">
            Checking agent assignments…
          </p>
        )}
        {agents.isError && !blocked && (
          <p className="text-sm text-muted-foreground">
            Agent assignments could not be listed. Helpin will check them before
            removing the model.
          </p>
        )}
        {remove.isError && (
          <p role="alert" className="text-sm text-destructive">
            {remove.error.message}
          </p>
        )}
        <AlertDialogFooter>
          <AlertDialogCancel disabled={remove.isPending}>
            {blocked ? "Close" : "Cancel"}
          </AlertDialogCancel>
          {!blocked && (
            <Button
              variant="destructive"
              disabled={agents.isLoading || remove.isPending}
              onClick={() => remove.mutate(profile, { onSuccess: onClose })}
            >
              {remove.isPending ? "Removing…" : "Remove model"}
            </Button>
          )}
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
