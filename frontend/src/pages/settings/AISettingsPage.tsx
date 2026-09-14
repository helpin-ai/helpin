import { useCallback, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { aiConnectionService } from "@/lib/services/aiConnectionService";
import {
  aiProfileService,
  type AIProfile,
} from "@/lib/services/aiProfileService";
import { AIConnectionsDialog } from "@/components/agents/AIConnectionsDialog";
import { AIProfileEditor } from "@/components/agents/AIProfileEditor";
import {
  QuietSection,
  QuietListRow,
  QuietEmptyState,
  QuietTextAction,
  QuietPrimaryAction,
} from "@/components/design-system/quiet";
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from "@/components/design-system/quiet-dropdown-select";
import { Label } from "@/components/ui/label";
import { SettingsPageFrame } from "./SettingsPageFrame";

export function AISettingsPage({ scope }: { scope: "personal" | "workspace" }) {
  return (
    <SettingsPageFrame section={scope === "personal" ? "ai-connections" : "ai"}>
      {({ workspaceId, currentWorkspaceName, permissions }) => (
        <AISettingsContent
          key={`${workspaceId}-${scope}`}
          workspaceId={workspaceId}
          workspaceName={currentWorkspaceName}
          scope={scope}
          canManage={
            scope === "personal" || permissions.has("workspace.update")
          }
        />
      )}
    </SettingsPageFrame>
  );
}

function AISettingsContent({
  workspaceId,
  workspaceName,
  scope,
  canManage,
}: {
  workspaceId: string;
  workspaceName: string;
  scope: "personal" | "workspace";
  canManage: boolean;
}) {
  const cache = useQueryClient();
  const [manageConnections, setManageConnections] = useState(false);
  const [editing, setEditing] = useState<AIProfile | "new" | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const connections = useQuery({
    queryKey: ["ai-connections", workspaceId],
    queryFn: async () => {
      const res = await aiConnectionService.list(workspaceId);
      if (res.error || !res.data)
        throw new Error(res.error || "Unable to load connections");
      return res.data;
    },
  });
  const profiles = useQuery({
    queryKey: ["ai-profiles", workspaceId],
    enabled: connections.data?.enabled === true,
    queryFn: async () => {
      const res = await aiProfileService.list(workspaceId);
      if (res.error || !res.data)
        throw new Error(res.error || "Unable to load profiles");
      return res.data;
    },
  });
  const settings = useQuery({
    queryKey: ["ai-settings", workspaceId],
    enabled: scope === "workspace" && connections.data?.enabled === true,
    queryFn: async () => {
      const res = await aiProfileService.settings(workspaceId);
      if (res.error || !res.data)
        throw new Error(res.error || "Unable to load AI settings");
      return res.data;
    },
  });
  const refresh = useCallback(() => {
    for (const key of ["ai-connections", "ai-profiles", "ai-settings"])
      void cache.invalidateQueries({ queryKey: [key, workspaceId] });
  }, [cache, workspaceId]);
  async function mutate(action: () => Promise<{ error?: string | null }>) {
    setBusy(true);
    setError("");
    try {
      const res = await action();
      if (res.error) setError(res.error);
      else refresh();
    } catch {
      setError("Unable to update AI settings. Please retry.");
    } finally {
      setBusy(false);
    }
  }
  if (connections.isPending)
    return (
      <p role="status" className="text-sm text-quiet-text-secondary">
        Loading AI connections…
      </p>
    );
  if (connections.isError)
    return (
      <QuietEmptyState
        title="Connections could not be loaded"
        description="Check your connection and try again."
        action={
          <QuietTextAction onClick={() => void connections.refetch()}>
            Retry
          </QuietTextAction>
        }
      />
    );
  if (!connections.data.enabled)
    return (
      <QuietEmptyState
        title="AI connections are not configured"
        description="Ask your administrator to configure AI connection encryption and the agent runtime."
      />
    );
  const ownConnections = connections.data.connections.filter(
    (c) => (c.scope || "personal") === scope,
  );
  const ownProfiles = (profiles.data ?? []).filter((p) => p.scope === scope);
  return (
    <div className="space-y-4">
      <p className="text-sm text-quiet-text-secondary">
        {scope === "personal"
          ? `Your connections and profiles in ${workspaceName}. They are available for your manual runs and chats.`
          : `Shared AI configuration for ${workspaceName}. Agents and automation inherit the workspace default unless a shared agent profile is selected.`}
      </p>
      {error && (
        <p role="alert" className="text-sm text-destructive">
          {error}
        </p>
      )}
      <QuietSection
        title="Connections"
        action={
          canManage ? (
            <QuietTextAction onClick={() => setManageConnections(true)}>
              Manage connections
            </QuietTextAction>
          ) : undefined
        }
      >
        {ownConnections.length === 0 ? (
          <QuietEmptyState
            title="No connections yet"
            description={
              canManage
                ? scope === "personal"
                  ? "Add an API key or connect your ChatGPT subscription, then create a profile."
                  : "Add a shared API-key connection, then create a profile."
                : "A workspace administrator can add a shared API-key connection."
            }
          />
        ) : (
          ownConnections.map((c) => (
            <QuietListRow
              key={c.id}
              title={c.name}
              meta={`${c.provider} · ${c.status.replaceAll("_", " ")}`}
            />
          ))
        )}
      </QuietSection>
      {scope === "workspace" && (
        <QuietSection title="Workspace default">
          {settings.isError ? (
            <QuietTextAction onClick={() => void settings.refetch()}>
              Retry loading the default
            </QuietTextAction>
          ) : (
            <>
              <Label htmlFor="workspace-ai-default">Default profile</Label>
              <Select
                value={settings.data?.default_profile_id || "none"}
                disabled={
                  !canManage ||
                  busy ||
                  settings.isPending ||
                  profiles.isPending ||
                  profiles.isError
                }
                onValueChange={(id) =>
                  void mutate(() =>
                    aiProfileService.setDefault(
                      workspaceId,
                      id === "none" ? null : id,
                    ),
                  )
                }
              >
                <SelectTrigger
                  id="workspace-ai-default"
                  variant="underline"
                  className="w-full px-0.5"
                >
                  <SelectValue placeholder="Choose a shared profile" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="none">Not configured</SelectItem>
                  {ownProfiles.map((p) => (
                    <SelectItem key={p.id} value={p.id}>
                      {p.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </>
          )}
        </QuietSection>
      )}
      <QuietSection
        title="Profiles"
        action={
          canManage ? (
            <QuietPrimaryAction
              disabled={connections.data.connections.length === 0 || busy}
              onClick={() => setEditing("new")}
            >
              Create profile
            </QuietPrimaryAction>
          ) : undefined
        }
      >
        {profiles.isPending ? (
          <p role="status" className="text-sm text-quiet-text-secondary">
            Loading profiles…
          </p>
        ) : profiles.isError ? (
          <QuietTextAction onClick={() => void profiles.refetch()}>
            Retry loading profiles
          </QuietTextAction>
        ) : ownProfiles.length === 0 ? (
          <QuietEmptyState
            title="No profiles yet"
            description="A profile combines a connection, a model, and optional fallback settings."
          />
        ) : (
          ownProfiles.map((p) => (
            <QuietListRow
              key={p.id}
              title={p.name}
              meta={`${p.primary.model.provider} · ${p.primary.model.model}`}
              detail={
                p.fallback
                  ? `Fallback: ${p.fallback.model.provider} · ${p.fallback.model.model}, before execution only`
                  : "No fallback"
              }
              trailing={
                canManage ? (
                  <div className="flex flex-wrap gap-2">
                    <QuietTextAction
                      disabled={busy}
                      onClick={() => setEditing(p)}
                    >
                      Edit
                    </QuietTextAction>
                    <QuietTextAction
                      disabled={busy}
                      onClick={() =>
                        void mutate(() =>
                          aiProfileService.remove(workspaceId, p),
                        )
                      }
                    >
                      Delete
                    </QuietTextAction>
                  </div>
                ) : undefined
              }
            />
          ))
        )}
      </QuietSection>
      <AIConnectionsDialog
        workspaceId={workspaceId}
        scope={scope}
        open={manageConnections}
        onOpenChange={setManageConnections}
        connections={ownConnections}
        models={connections.data.models}
        onChanged={refresh}
      />
      {editing && (
        <AIProfileEditor
          workspaceId={workspaceId}
          scope={scope}
          profile={editing === "new" ? undefined : editing}
          connections={connections.data.connections}
          onClose={() => setEditing(null)}
          onSaved={refresh}
        />
      )}
    </div>
  );
}
