import { useState } from "react";
import { toast } from "sonner";
import {
  useConnectionModels,
  useRefreshConnectionModels,
} from "@/hooks/queries/useAIConnections";
import {
  useAIProfiles,
  useAISettings,
  useEnableAIModel,
  useSetAIModelVisibility,
  useDeleteAIProfile,
} from "@/hooks/queries/useAIProfiles";
import type {
  AIConnection,
  DiscoveredAIModel,
} from "@/lib/services/aiConnectionService";
import type { AIProfile } from "@/lib/services/aiProfileService";
import { catalogLabel, modelCatalogFor } from "@/lib/aiProviders";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { QuietSearchInput } from "@/components/design-system/quiet";
import { Input } from "@/components/ui/input";
import { QuickTooltip } from "@/components/ui/quick-tooltip";
import { useConfirm } from "@/components/ui/confirm-dialog";
import { ArrowReloadHorizontalIcon } from "@/lib/icons";
import { AISetupHelp } from "./AISetupHelp";
import { AIConfiguredModelRow } from "./AIConfiguredModelRow";
import { AIProfileEditor } from "./AIProfileEditor";
import { AIConnectionPolicyNotice } from "./AIConnectionPolicyNotice";

export function AIConnectionModelsDialog({
  workspaceId,
  connection,
  connections,
  canManage,
  onClose,
}: {
  workspaceId: string;
  connection: AIConnection;
  connections: AIConnection[];
  canManage: boolean;
  onClose: () => void;
}) {
  const [search, setSearch] = useState("");
  const [custom, setCustom] = useState("");
  const [editing, setEditing] = useState<AIProfile | null>(null);
  const liveDiscovery =
    connection.funding !== "managed" &&
    ["openai", "anthropic", "openrouter", "openai_chatgpt"].includes(connection.provider);
  const canRefresh = canManage && liveDiscovery && connection.status === "connected";
  const profiles = useAIProfiles(workspaceId);
  const discovered = useConnectionModels(workspaceId, connection.id, true, {
    refreshOnOpen: canRefresh,
  });
  const refresh = useRefreshConnectionModels(workspaceId, connection.id);
  const save = useEnableAIModel(workspaceId);
  const visibility = useSetAIModelVisibility(workspaceId);
  const remove = useDeleteAIProfile(workspaceId);
  const settings = useAISettings(workspaceId);
  const confirm = useConfirm();
  const busy =
    save.isPending ||
    visibility.isPending ||
    remove.isPending;
  const existing = (profiles.data ?? []).filter(
    (p) => p.primary.connection_id === connection.id,
  );
  const known = new Set(existing.map((p) => p.primary.model.model));
  const available =
    discovered.data?.source === "catalog" && connection.funding !== "managed"
      ? [
          ...new Map(
            [
              ...modelCatalogFor(connection.provider).flatMap((group) =>
                group.models.map((m) => ({
                  id: m.selectionModel,
                  name: m.label,
                })),
              ),
              ...discovered.data.models,
            ].map((m) => [m.id, m]),
          ).values(),
        ]
      : (discovered.data?.models ?? []);
  const availableIDs = new Set(available.map((m) => m.id));
  const query = search.trim().toLowerCase();
  const matches = (name: string, id: string) =>
    `${name} ${id}`.toLowerCase().includes(query);
  const savedRows = existing.filter((p) =>
    matches(p.name, p.primary.model.model),
  );
  const newRows = available.filter(
    (m) => !known.has(m.id) && matches(m.name, m.id),
  );
  const editable = canManage && !profiles.isPending && !profiles.isError;
  const canEnable =
    editable &&
    connection.status === "connected" &&
    connection.policy?.allowed !== false;

  async function refreshList() {
    if (!canRefresh) {
      await discovered.refetch();
      return;
    }
    try {
      await refresh.mutateAsync();
    } catch {
      toast.error("Could not refresh models.");
    }
  }

  async function enable(model: DiscoveredAIModel) {
    try {
      await save.mutateAsync({
        connection_id: connection.id,
        model: model.id,
        name: model.name,
      });
      setCustom("");
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : "Could not enable model.",
      );
    }
  }
  async function toggle(profile: AIProfile, shown: boolean) {
    try {
      await visibility.mutateAsync({ profile, hidden: !shown });
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : "Could not update visibility.",
      );
    }
  }
  async function deleteModel(profile: AIProfile) {
    if (
      !(await confirm({
        title: `Remove “${profile.name}”?`,
        description:
          "Remove this saved configuration. Models used by agents must be reassigned first. To only hide it from model pickers, use the toggle instead.",
        confirmText: "Remove",
        variant: "destructive",
      }))
    )
      return;
    try {
      await remove.mutateAsync(profile);
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : "Could not remove model.",
      );
    }
  }
  const customID = custom.trim();
  const customExists = known.has(customID);

  return (
    <>
      <Dialog
        open
        onOpenChange={(open) => {
          if (!open && !busy) onClose();
        }}
      >
        <DialogContent className="flex max-h-[85vh] flex-col gap-0 overflow-hidden p-0 sm:max-w-2xl">
          <DialogHeader className="border-b px-6 py-4 text-left">
            <DialogTitle>{connection.name} models</DialogTitle>
            <div className="flex items-center gap-1">
              <DialogDescription>
                {connection.scope === "personal"
                  ? "Personal · Your chats and manual runs"
                  : "Workspace-wide · Chats, agents, and automations"}
              </DialogDescription>
              <AISetupHelp
                label="About model access"
                description={
                  connection.scope === "personal"
                    ? "Only you can use this connection. Choose its models in Ask Agent or when starting a manual run. Agent defaults and scheduled runs require a workspace-wide model."
                    : "Members can choose these models in Ask Agent or assign them to agents, including scheduled and automated runs. Only administrators can change shared model settings."
                }
              />
            </div>
          </DialogHeader>
          <div className="flex items-center gap-2 px-6 py-3">
            <QuietSearchInput
              aria-label="Search models"
              placeholder="Search models…"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              containerClassName="flex-1"
            />
            {canManage && liveDiscovery && (
              <QuickTooltip label="Models refresh when you open this chooser. Refresh again to check for new models; your saved choices stay unchanged.">
                <Button
                  size="icon"
                  variant="ghost"
                  aria-label="Refresh models"
                  disabled={
                    refresh.isPending ||
                    discovered.isFetching ||
                    connection.status !== "connected"
                  }
                  onClick={() => void refreshList()}
                >
                  <ArrowReloadHorizontalIcon
                    className={
                      refresh.isPending || discovered.isFetching
                        ? "size-4 animate-spin"
                        : "size-4"
                    }
                  />
                </Button>
              </QuickTooltip>
            )}
          </div>
          <div className="flex items-center justify-between px-6 pb-2 text-xs text-muted-foreground">
            <span>Model</span>
            <span className="flex items-center gap-1">
              Show in model pickers
              <AISetupHelp
                label="About model visibility"
                description="Show in Ask Agent and agent model pickers. Hidden models stay on the AI models settings page. Existing selections and defaults keep working. Newly discovered models are not added automatically."
              />
            </span>
          </div>
          <div className="min-h-0 flex-1 overflow-y-auto border-y px-6">
            <AIConnectionPolicyNotice policy={connection.policy} />
            {(discovered.data?.warning || discovered.isError) && (
              <p
                role="status"
                className="py-3 text-xs text-amber-700 dark:text-amber-400"
              >
                {discovered.data?.warning ?? "Could not load available models."}{" "}
                <button
                  className="underline"
                  disabled={refresh.isPending || discovered.isFetching}
                  onClick={() => void refreshList()}
                >
                  Retry
                </button>
              </p>
            )}
            {profiles.isError ? (
              <p role="alert" className="py-4 text-sm">
                Could not load saved models.{" "}
                <button
                  className="underline"
                  onClick={() => void profiles.refetch()}
                >
                  Retry
                </button>
              </p>
            ) : profiles.isPending ? (
              <p role="status" className="py-4 text-sm text-muted-foreground">
                Loading saved models…
              </p>
            ) : (
              <>
                {savedRows.map((profile) => {
                  const modelID = profile.primary.model.model;
                  const isDefault =
                    (settings.data?.personal_default_profile_id || settings.data?.default_profile_id) === profile.id;
                  const notListed =
                    discovered.data?.source === "provider" &&
                    !discovered.data.stale &&
                    !availableIDs.has(modelID);
                  return (
                    <AIConfiguredModelRow
                      key={profile.id}
                      profile={profile}
                      isDefault={isDefault}
                      notListed={notListed}
                      editable={editable}
                      busy={busy}
                      onEdit={() => setEditing(profile)}
                      onRemove={() => void deleteModel(profile)}
                      onVisibility={(shown) => void toggle(profile, shown)}
                    />
                  );
                })}
                {newRows.map((model) => (
                  <div
                    key={model.id}
                    className="flex items-center gap-3 border-b py-3 last:border-0"
                  >
                    <div className="min-w-0 flex-1">
                      <p className="truncate text-sm">{model.name}</p>
                      {model.id !== model.name && (
                        <p className="truncate text-xs text-muted-foreground">
                          {model.id}
                        </p>
                      )}
                    </div>
                    <Switch
                      aria-label={`Show ${model.name} in model pickers`}
                      checked={false}
                      disabled={!canEnable || busy}
                      onCheckedChange={() => void enable(model)}
                    />
                  </div>
                ))}
                {discovered.isPending && (
                  <p
                    role="status"
                    className="py-4 text-sm text-muted-foreground"
                  >
                    Finding models…
                  </p>
                )}
                {!discovered.isPending &&
                  savedRows.length + newRows.length === 0 && (
                    <p className="py-6 text-center text-sm text-muted-foreground">
                      {query ? "No matching models." : "No models listed yet."}
                    </p>
                  )}
              </>
            )}
          </div>
          {settings.isError && connection.scope === "workspace" && (
            <p className="px-6 pt-3 text-xs text-destructive">
              Could not load the workspace default.{" "}
              <button
                className="underline"
                onClick={() => void settings.refetch()}
              >
                Retry
              </button>
            </p>
          )}
          <div className="px-6 py-4">
            {canManage &&
            connection.funding !== "managed" ? (
              <details>
                <summary className="cursor-pointer text-sm text-muted-foreground">
                  Add a model by ID
                </summary>
                <form
                  className="mt-3 flex items-center gap-2"
                  onSubmit={(e) => {
                    e.preventDefault();
                    if (customID && !customExists && canEnable && !busy)
                      void enable({
                        id: customID,
                        name:
                          catalogLabel(connection.provider, customID) ??
                          customID,
                      });
                  }}
                >
                  <Input
                    aria-label="Model ID"
                    placeholder="Exact model ID"
                    maxLength={200}
                    value={custom}
                    onChange={(e) => setCustom(e.target.value)}
                  />
                  <AISetupHelp
                    label="About custom model IDs"
                    description={connection.provider === "openai_chatgpt"
                      ? "Use the exact subscription-runtime ID if a model is missing after refresh. The model must be available to your connected ChatGPT account."
                      : "Use the exact ID from your provider when discovery does not list a model. Provider access and runtime support are still required."}
                  />
                  <Button
                    size="sm"
                    disabled={!customID || customExists || !canEnable || busy}
                  >
                    {customExists ? "Already added" : "Add"}
                  </Button>
                </form>
              </details>
            ) : (
              <span className="flex items-center gap-1 text-xs text-muted-foreground">
                {canManage
                  ? "Supported models"
                  : "Managed by your workspace admins"}
                <AISetupHelp
                  label="About available models"
                  description={
                    canManage
                      ? "This connection uses the supported model catalog. Model discovery does not change managed defaults or subscription runtime support."
                      : "Ask an administrator to change shared model visibility. You can manage your own personal connections in Settings."
                  }
                />
              </span>
            )}
          </div>
          <div className="flex justify-end border-t px-6 py-3"><Button size="sm" disabled={busy} onClick={onClose}>Done</Button></div>
        </DialogContent>
      </Dialog>
      {editing && (
        <AIProfileEditor
          workspaceId={workspaceId}
          scope={editing.scope}
          profile={editing}
          connections={connections}
          onClose={() => setEditing(null)}
        />
      )}
    </>
  );
}
