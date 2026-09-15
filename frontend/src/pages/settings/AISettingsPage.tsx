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
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  Table,
  TableBody,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { AiNetworkIcon, Key01Icon, PlusSignIcon, Shield01Icon, SparklesIcon } from "@/lib/icons";
import { SettingsPageFrame } from "./SettingsPageFrame";

export function AISettingsPage({ scope = "workspace" }: { scope?: "personal" | "workspace" }) {
  return (
    <SettingsPageFrame section="ai">
      {({ workspaceId, permissions }) => (
        <Tabs key={`${workspaceId}-${scope}`} defaultValue={scope} className="gap-5">
          <TabsList variant="quiet" aria-label="AI setup scope">
            <TabsTrigger value="workspace">Workspace</TabsTrigger>
            <TabsTrigger value="personal">Personal</TabsTrigger>
          </TabsList>
          <TabsContent value="workspace">
            <AISettingsContent workspaceId={workspaceId} scope="workspace" canManage={permissions.has("workspace.update")} />
          </TabsContent>
          <TabsContent value="personal">
            <AISettingsContent workspaceId={workspaceId} scope="personal" canManage />
          </TabsContent>
        </Tabs>
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
  const createProfileButton = <Button size="sm" className="gap-1.5" disabled={!canCreateProfile} onClick={() => setEditing("new")}><PlusSignIcon className="h-4 w-4" />Create profile</Button>;

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
        {scope === "workspace" && settings.isError && (
          <Alert variant="destructive" className="mb-3">
            <AlertTitle>Could not load the workspace default</AlertTitle>
            <AlertDescription><Button size="sm" variant="ghost" onClick={() => void settings.refetch()}>Retry</Button></AlertDescription>
          </Alert>
        )}
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
              canCreateProfile ? createProfileButton : canManage ? (
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
                    canSetDefault={scope === "workspace" && !settings.isPending && !settings.isError && !setDefault.isPending}
                    onEdit={setEditing}
                    onSetDefault={(p) => void chooseDefault(p.id)}
                    onClearDefault={() => void chooseDefault(null)}
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
