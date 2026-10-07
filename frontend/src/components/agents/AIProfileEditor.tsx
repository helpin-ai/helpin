import { useId, useState } from "react";
import { toast } from "sonner";
import { AIRouteFields } from "./AIRouteFields";
import { AISetupHelp } from "./AISetupHelp";
import { useSaveAIProfile } from "@/hooks/queries/useAIProfiles";
import type { AIConnection } from "@/lib/services/aiConnectionService";
import type { AIProfile, AIProfileRoute } from "@/lib/services/aiProfileService";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { QuietUnderlineInput } from "@/components/design-system/quiet";

const emptyRoute = (): AIProfileRoute => ({
  connection_id: "",
  model: { provider: "", model: "", controls: {} },
});

export function AIProfileEditor({
  workspaceId,
  scope,
  profile,
  connections,
  onClose,
}: {
  workspaceId: string;
  scope: "personal" | "workspace";
  profile?: AIProfile;
  connections: AIConnection[];
  onClose: () => void;
}) {
  const id = useId();
  const [name, setName] = useState(profile?.name ?? "");
  const [primary, setPrimary] = useState(profile?.primary ?? emptyRoute());
  const [fallback, setFallback] = useState<AIProfileRoute | null>(profile?.fallback ?? null);
  const save = useSaveAIProfile(workspaceId);

  // Primaries stay in scope. Preserve existing authorized shared fallbacks
  // on personal configurations without offering other members' connections.
  const scoped = connections.filter((connection) => connection.scope === scope);
  const fallbackOptions = connections.filter(
    (connection) => connection.id !== primary.connection_id && (connection.scope === scope || connection.id === profile?.fallback?.connection_id),
  );

  function validate(): string | null {
    if (!name.trim()) return "Name is required";
    if (!primary.connection_id) return "Choose a primary connection";
    if (!primary.model.model.trim()) return "Enter a primary model";
    if (fallback) {
      if (!fallback.connection_id) return "Choose a fallback connection";
      if (!fallback.model.model.trim()) return "Enter a fallback model";
      if (fallback.connection_id === primary.connection_id)
        return "The fallback must use a different connection";
    }
    return null;
  }

  async function submit() {
    const problem = validate();
    if (problem) {
      toast.error(problem);
      return;
    }
    try {
      await save.mutateAsync({
        value: { name: name.trim(), scope, primary, fallback, revision: profile?.revision },
        id: profile?.id,
      });
      toast.success(profile ? "Model settings updated" : "Model added");
      onClose();
    } catch (error) {
      const message = error instanceof Error ? error.message : "Could not save model settings.";
      toast.error(
        message.toLowerCase().includes("revision")
          ? "This model configuration changed elsewhere. Close and reopen it to edit the latest version."
          : message,
      );
    }
  }

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !save.isPending) onClose();
      }}
    >
      <DialogContent className="grid max-h-[88vh] gap-0 overflow-hidden p-0 sm:max-w-lg" onOpenAutoFocus={(event) => {
        const nameField = document.getElementById(`${id}-name`);
        if (nameField) { event.preventDefault(); nameField.focus(); }
      }}>
        <DialogHeader className="space-y-1.5 border-b border-border/60 px-6 py-4 text-left">
          <DialogTitle>{profile ? `Edit “${profile.name}”` : "Add model"}</DialogTitle>
          <div className="flex items-center gap-1">
            <DialogDescription>{scope === "personal" ? "Personal model · Only you" : "Workspace model · Shared with members"}</DialogDescription>
            <AISetupHelp label="About this model" description={scope === "personal" ? "For your manual runs in this workspace. Changes apply to new runs." : "For workspace members and automation. Changes apply to new runs."} />
          </div>
        </DialogHeader>

        <form
          className="min-h-0 max-h-[calc(88vh-8.5rem)] space-y-5 overflow-y-auto px-6 py-5"
          onSubmit={(event) => {
            event.preventDefault();
            void submit();
          }}
        >
          <div className="space-y-1.5">
            <Label htmlFor={`${id}-name`}>Display name</Label>
            <QuietUnderlineInput
              id={`${id}-name`}
              value={name}
              onChange={(event) => setName(event.target.value)}
              maxLength={100}
              placeholder="e.g. Everyday tasks"
              disabled={save.isPending}
            />
          </div>

          <section>
            {fallback && <h3 className="mb-3 text-sm font-medium">Primary model</h3>}
            <AIRouteFields
              workspaceId={workspaceId}
              route={primary}
              onChange={setPrimary}
              connections={scoped}
              disabled={save.isPending}
              emptyHint={
                scope === "personal"
                  ? "Add a personal connection before adding a model."
                  : "Add a shared connection before adding a model."
              }
            />
          </section>

          <section>
            <div className="mb-3 flex items-center gap-1">
              {fallback && <h3 className="text-sm font-medium">Fallback model</h3>}
              {!fallback && <Button type="button" size="sm" variant="ghost" className="h-auto p-0 text-sm" disabled={save.isPending} onClick={() => setFallback(emptyRoute())}>Add fallback</Button>}
              <AISetupHelp label="About fallback" description="Optional. Uses a different connection only if the primary is unavailable before a run starts. An accepted run keeps its selected model and connection." />
              {fallback && <Button type="button" size="sm" variant="ghost" className="ml-auto h-auto p-0 text-xs" disabled={save.isPending} onClick={() => setFallback(null)}>Remove fallback</Button>}
            </div>
            {fallback &&
              (primary.connection_id ? (
                <AIRouteFields
                  workspaceId={workspaceId}
                  route={fallback}
                  onChange={setFallback}
                  connections={fallbackOptions}
                  disabled={save.isPending}
                  emptyHint="Add a second connection in this scope to use a fallback."
                />
              ) : (
                <p className="text-xs text-muted-foreground">Choose a primary connection first.</p>
              ))}
          </section>

          <button type="submit" className="sr-only">
            {profile ? "Save changes" : "Add model"}
          </button>
        </form>

        <DialogFooter className="border-t border-border/60 px-6 py-3">
          <Button
            type="button"
            size="sm"
            variant="ghost"
            onClick={onClose}
            disabled={save.isPending}
          >
            Cancel
          </Button>
          <Button type="button" size="sm" disabled={save.isPending} onClick={() => void submit()}>
            {save.isPending ? "Saving…" : profile ? "Save changes" : "Add model"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
