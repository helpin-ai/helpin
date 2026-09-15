import { useMemo, useState } from "react";
import { toast } from "sonner";
import {
  AIConnectionDialog,
  type AIConnectionDialogMode,
} from "@/components/agents/AIConnectionDialog";
import { AIConnectionRow } from "@/components/agents/AIConnectionRow";
import { AIProfileRow } from "@/components/agents/AIProfileRow";
import { AIProfileEditor } from "@/components/agents/AIProfileEditor";
import { AIEmptyHero } from "@/components/settings/ai/AIEmptyHero";
import { AISectionLabel } from "@/components/settings/ai/AISectionLabel";
import { useAIConnections } from "@/hooks/queries/useAIConnections";
import {
  useAIProfiles,
  useAISettings,
  useSetDefaultAIProfile,
} from "@/hooks/queries/useAIProfiles";
import type { AIConnection } from "@/lib/services/aiConnectionService";
import type { AIProfile } from "@/lib/services/aiProfileService";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { QuickTooltip } from "@/components/ui/quick-tooltip";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/design-system/quiet-dropdown-select";
import { AiNetworkIcon, Key01Icon, PlusSignIcon, Shield01Icon, SparklesIcon } from "@/lib/icons";
import { SettingsPageFrame } from "./SettingsPageFrame";

export function AISettingsPage({ scope }: { scope: "personal" | "workspace" }) {
  return (
    <SettingsPageFrame section={scope === "personal" ? "ai-connections" : "ai"}>
      {({ workspaceId, permissions }) => (
        <AISettingsContent
          key={`${workspaceId}-${scope}`}
          workspaceId={workspaceId}
          scope={scope}
          canManage={scope === "personal" || permissions.has("workspace.update")}
        />
      )}
    </SettingsPageFrame>
  );
}

function AISettingsContent({
  workspaceId,
  scope,
  canManage,
}: {
  workspaceId: string;
  scope: "personal" | "workspace";
  canManage: boolean;
}) {
  const [dialog, setDialog] = useState<AIConnectionDialogMode | null>(null);
  const [editing, setEditing] = useState<AIProfile | "new" | null>(null);

  const connections = useAIConnections(workspaceId);
  const enabled = connections.data?.enabled === true;
  const profiles = useAIProfiles(workspaceId, { enabled });
  const settings = useAISettings(workspaceId, { enabled: enabled && scope === "workspace" });
  const setDefault = useSetDefaultAIProfile(workspaceId);

  const allConnections = connections.data?.connections ?? [];
  const ownConnections = useMemo(
    () => allConnections.filter((c) => (c.scope || "personal") === scope),
    [allConnections, scope],
  );
  const ownProfiles = useMemo(
    () => (profiles.data ?? []).filter((p) => p.scope === scope),
    [profiles.data, scope],
  );

  async function chooseDefault(profileId: string | null) {
    try {
      await setDefault.mutateAsync(profileId);
      toast.success(profileId ? "Workspace default updated" : "Workspace default cleared");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Could not update the default.");
    }
  }

  if (connections.isPending)
    return (
      <div className="space-y-2">
        <Skeleton className="h-3 w-28" />
        <Skeleton className="h-40 w-full" />
      </div>
    );

  if (connections.isError)
    return (
      <Alert variant="destructive">
        <AlertTitle>Could not load AI connections</AlertTitle>
        <AlertDescription className="space-y-2">
          <p>
            {connections.error instanceof Error
              ? connections.error.message
              : "Check your connection and try again."}
          </p>
          <Button size="sm" variant="outline" onClick={() => void connections.refetch()}>
            Retry
          </Button>
        </AlertDescription>
      </Alert>
    );

  if (!enabled)
    return (
      <AIEmptyHero
        icon={AiNetworkIcon}
        title="AI connections are not configured"
        description="Ask your administrator to configure AI connection encryption and the agent runtime."
      />
    );

  const models = connections.data?.models ?? [];
  const defaultProfileId = settings.data?.default_profile_id ?? null;
  const canCreateProfile = canManage && ownConnections.length > 0;
  const createProfileButton = <Button size="sm" variant="outline" disabled={!canCreateProfile} onClick={() => setEditing("new")}>Create profile</Button>;

  return (
    <div className="space-y-7">
      {!canManage && (
        <Alert>
          <Shield01Icon className="h-4 w-4" />
          <AlertTitle>Connections are read-only</AlertTitle>
          <AlertDescription>
            A workspace admin can add shared connections, edit profiles, and set the default.
          </AlertDescription>
        </Alert>
      )}

      <section>
        <AISectionLabel
          label="Connections"
          count={ownConnections.length}
          action={
            canManage ? (
              <Button size="sm" className="gap-1.5" onClick={() => setDialog({ kind: "add" })}>
                <PlusSignIcon className="h-4 w-4" />
                Add connection
              </Button>
            ) : undefined
          }
        />
        {ownConnections.length === 0 ? (
          <AIEmptyHero
            icon={Key01Icon}
            title="No connections yet"
            description={
              canManage
                ? scope === "personal"
                  ? "Add an API key or connect your ChatGPT subscription, then create a profile."
                  : "Add a shared API-key connection or an approved endpoint, then create a profile."
                : "A workspace administrator can add a shared connection."
            }
            action={
              canManage ? (
                <Button size="sm" className="gap-1.5" onClick={() => setDialog({ kind: "add" })}>
                  <PlusSignIcon className="h-4 w-4" />
                  Add your first connection
                </Button>
              ) : undefined
            }
          />
        ) : (
          <div className="overflow-hidden rounded-lg border border-border">
            <Table>
              <TableHeader>
                <TableRow className="bg-muted/40 hover:bg-muted/40">
                  <TableHead className="min-w-[240px]">Connection</TableHead>
                  <TableHead className="w-[180px]">Provider</TableHead>
                  <TableHead className="w-[160px]">Status</TableHead>
                  <TableHead className="min-w-[220px]">Details</TableHead>
                  {canManage && (
                    <TableHead className="w-[112px] text-right">
                      <span className="sr-only">Actions</span>
                    </TableHead>
                  )}
                </TableRow>
              </TableHeader>
              <TableBody>
                {ownConnections.map((connection) => (
                  <AIConnectionRow
                    key={connection.id}
                    workspaceId={workspaceId}
                    connection={connection}
                    canManage={canManage}
                    onReconnect={(c) => setDialog({ kind: "reconnect", connection: c })}
                    onContinueLogin={(c: AIConnection) =>
                      setDialog({ kind: "reconnect", connection: c, autoPoll: true })
                    }
                  />
                ))}
              </TableBody>
            </Table>
          </div>
        )}
      </section>

      {scope === "workspace" && (
        <WorkspaceDefaultSection
          canManage={canManage}
          profiles={ownProfiles}
          defaultProfileId={defaultProfileId}
          isSaving={setDefault.isPending}
          settingsError={settings.isError}
          profilesError={profiles.isError}
          isLoading={settings.isPending || profiles.isPending}
          onRetry={() => {
            if (settings.isError) void settings.refetch();
            if (profiles.isError) void profiles.refetch();
          }}
          onChange={chooseDefault}
        />
      )}

      <section>
        <AISectionLabel
          label="Profiles"
          count={ownProfiles.length}
          action={
            canManage ? (
              canCreateProfile ? createProfileButton : (
                <QuickTooltip label="Add a connection before creating a profile.">
                  <span className="inline-flex rounded-md focus-visible:outline-2 focus-visible:outline-ring" tabIndex={0}>
                    {createProfileButton}
                  </span>
                </QuickTooltip>
              )
            ) : undefined
          }
        />
        {profiles.isPending ? (
          <Skeleton className="h-32 w-full" />
        ) : profiles.isError ? (
          <Alert variant="destructive">
            <AlertTitle>Could not load profiles</AlertTitle>
            <AlertDescription>
              <Button size="sm" variant="outline" onClick={() => void profiles.refetch()}>
                Retry
              </Button>
            </AlertDescription>
          </Alert>
        ) : ownProfiles.length === 0 ? (
          <AIEmptyHero
            icon={SparklesIcon}
            title="No profiles yet"
            description="A profile combines a connection, a model, and an optional fallback."
            action={
              canCreateProfile ? (
                <Button size="sm" onClick={() => setEditing("new")}>
                  Create profile
                </Button>
              ) : canManage ? (
                <Button size="sm" variant="outline" onClick={() => setDialog({ kind: "add" })}>
                  Add a connection first
                </Button>
              ) : undefined
            }
          />
        ) : (
          <div className="overflow-hidden rounded-lg border border-border">
            <Table>
              <TableHeader>
                <TableRow className="bg-muted/40 hover:bg-muted/40">
                  <TableHead className="min-w-[220px]">Profile</TableHead>
                  <TableHead className="min-w-[240px]">Primary route</TableHead>
                  <TableHead className="min-w-[240px]">Fallback</TableHead>
                  {canManage && (
                    <TableHead className="w-[132px] text-right">
                      <span className="sr-only">Actions</span>
                    </TableHead>
                  )}
                </TableRow>
              </TableHeader>
              <TableBody>
                {ownProfiles.map((profile) => (
                  <AIProfileRow
                    key={profile.id}
                    workspaceId={workspaceId}
                    profile={profile}
                    connections={allConnections}
                    isDefault={scope === "workspace" && defaultProfileId === profile.id}
                    canManage={canManage}
                    canSetDefault={scope === "workspace"}
                    onEdit={setEditing}
                    onSetDefault={(p) => void chooseDefault(p.id)}
                  />
                ))}
              </TableBody>
            </Table>
          </div>
        )}
      </section>

      {dialog && (
        <AIConnectionDialog
          workspaceId={workspaceId}
          scope={scope}
          open
          mode={dialog}
          models={models}
          onOpenChange={(next) => {
            if (!next) setDialog(null);
          }}
        />
      )}

      {editing && (
        <AIProfileEditor
          workspaceId={workspaceId}
          scope={scope}
          profile={editing === "new" ? undefined : editing}
          connections={allConnections}
          onClose={() => setEditing(null)}
        />
      )}
    </div>
  );
}

function WorkspaceDefaultSection({
  canManage,
  profiles,
  defaultProfileId,
  isSaving,
  settingsError,
  profilesError,
  isLoading,
  onRetry,
  onChange,
}: {
  canManage: boolean;
  profiles: AIProfile[];
  defaultProfileId: string | null;
  isSaving: boolean;
  settingsError: boolean;
  profilesError: boolean;
  isLoading: boolean;
  onRetry: () => void;
  onChange: (profileId: string | null) => void;
}) {
  const reason = !canManage
    ? "Only workspace admins can change the default."
    : profilesError
      ? "Could not load profiles."
      : settingsError
        ? "Could not load the current default."
        : profiles.length === 0
          ? "Create a shared profile to choose a default."
          : isSaving
            ? "Saving…"
            : "Agents and automation use this profile unless a shared agent profile is selected.";
  const disabled =
    !canManage || profilesError || settingsError || profiles.length === 0 || isSaving;

  return (
    <section>
      <AISectionLabel label="Default profile" />
      <div className="rounded-lg border border-border bg-card px-4 py-3.5">
        <Label htmlFor="workspace-ai-default">Default profile</Label>
        {isLoading ? (
          <Skeleton className="mt-2 h-8 w-64" />
        ) : (
          <Select
            value={defaultProfileId || "none"}
            disabled={disabled}
            onValueChange={(id) => onChange(id === "none" ? null : id)}
          >
            <SelectTrigger
              id="workspace-ai-default"
              aria-describedby="workspace-ai-default-reason"
              variant="underline"
              className="mt-1 w-full px-0.5"
            >
              <SelectValue placeholder="Choose a shared profile" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="none">Not configured</SelectItem>
              {profiles.map((profile) => (
                <SelectItem
                  key={profile.id}
                  value={profile.id}
                  textValue={profile.name}
                  disabled={profile.primary_policy?.allowed === false}
                >
                  <span>{profile.name}</span>
                  {profile.primary_policy?.allowed === false && (
                    <span className="block text-[11px] text-muted-foreground">
                      {profile.primary_policy.message ?? "Unavailable for new runs"}
                    </span>
                  )}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        )}
        <p id="workspace-ai-default-reason" className="mt-1.5 text-xs text-muted-foreground">
          {reason}
          {(profilesError || settingsError) && (
            <button type="button" className="ml-1.5 underline" onClick={onRetry}>
              Retry
            </button>
          )}
        </p>
      </div>
    </section>
  );
}
