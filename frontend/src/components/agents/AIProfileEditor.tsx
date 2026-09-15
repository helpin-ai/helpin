import { useId, useState } from "react";
import { toast } from "sonner";
import { AIRouteFields } from "./AIRouteFields";
import { AISectionLabel } from "@/components/settings/ai/AISectionLabel";
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

  // Both routes stay inside the page's scope; a shared profile must never point
  // at someone's personal connection.
  const scoped = connections.filter((connection) => connection.scope === scope);
  const fallbackOptions = scoped.filter(
    (connection) => connection.id !== primary.connection_id,
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
      toast.success(profile ? "Profile updated" : "Profile created");
      onClose();
    } catch (error) {
      const message = error instanceof Error ? error.message : "Could not save the profile.";
      toast.error(
        message.toLowerCase().includes("revision")
          ? "This profile changed elsewhere. Close and reopen it to edit the latest version."
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
      <DialogContent className="grid max-h-[88vh] gap-0 overflow-hidden p-0 sm:max-w-xl">
        <DialogHeader className="space-y-1.5 border-b border-border/60 px-6 py-4 text-left">
          <DialogTitle>{profile ? `Edit “${profile.name}”` : "Create AI profile"}</DialogTitle>
          <DialogDescription>
            {scope === "personal"
              ? "For your manual runs in this workspace."
              : "For workspace members and automation."}{" "}
            Changes apply to new runs.
          </DialogDescription>
        </DialogHeader>

        <form
          className="min-h-0 max-h-[calc(88vh-8.5rem)] space-y-5 overflow-y-auto px-6 py-5"
          onSubmit={(event) => {
            event.preventDefault();
            void submit();
          }}
        >
          <div className="space-y-1.5">
            <Label htmlFor={`${id}-name`}>Name</Label>
            <QuietUnderlineInput
              id={`${id}-name`}
              value={name}
              onChange={(event) => setName(event.target.value)}
              maxLength={100}
              placeholder="Daily driver"
              disabled={save.isPending}
            />
          </div>

          <section>
            <AISectionLabel label="Primary route" />
            <AIRouteFields
              route={primary}
              onChange={setPrimary}
              connections={scoped}
              disabled={save.isPending}
              emptyHint={
                scope === "personal"
                  ? "Add a personal connection before creating a profile."
                  : "Add a shared connection before creating a profile."
              }
            />
          </section>

          <section>
            <AISectionLabel
              label="Fallback route"
              action={
                <Button
                  type="button"
                  size="sm"
                  variant="ghost"
                  className="h-auto p-0 text-xs"
                  disabled={save.isPending}
                  onClick={() => setFallback(fallback ? null : emptyRoute())}
                >
                  {fallback ? "Remove fallback" : "Add fallback"}
                </Button>
              }
            />
            <p className="mb-3 text-[13px] text-muted-foreground">
              Used only if the primary connection is unavailable before a run starts. An accepted
              run keeps its selected route.
            </p>
            {fallback &&
              (primary.connection_id ? (
                <AIRouteFields
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
            {profile ? "Save changes" : "Create profile"}
          </button>
        </form>

        <DialogFooter className="border-t border-border/60 px-6 py-3">
          <Button
            type="button"
            size="sm"
            variant="outline"
            onClick={onClose}
            disabled={save.isPending}
          >
            Cancel
          </Button>
          <Button type="button" size="sm" disabled={save.isPending} onClick={() => void submit()}>
            {save.isPending ? "Saving…" : profile ? "Save changes" : "Create profile"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
