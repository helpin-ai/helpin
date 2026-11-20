import { AIModelsPricingContext, sharedAIModelsPricing } from "@/components/agents/AIModelsPricingContext";
import { aiUsagePricingText } from "@edition/ai";
import { useRef, useState } from "react";
import {
  AIConnectionDialog,
  type AIConnectionDialogMode,
} from "@/components/agents/AIConnectionDialog";
import { AIConnectionRow } from "@/components/agents/AIConnectionRow";
import { AIConnectionModelsDialog } from "@/components/agents/AIConnectionModelsDialog";
import { AIAddModelsDialog } from "@/components/agents/AIAddModelsDialog";
import { RemoveAIModelDialog } from "@/components/agents/RemoveAIModelDialog";
import { AIModelTableRow } from "@/components/agents/AIModelTableRow";
import { AIProfileEditor } from "@/components/agents/AIProfileEditor";
import { AISetupHelp } from "@/components/agents/AISetupHelp";
import { AIEmptyHero } from "@/components/settings/ai/AIEmptyHero";
import { useAIConnections } from "@/hooks/queries/useAIConnections";
import { useAIProfiles, useAISettings } from "@/hooks/queries/useAIProfiles";
import type { AIConnection } from "@/lib/services/aiConnectionService";
import type { AIProfile } from "@/lib/services/aiProfileService";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
import { useConfirmAIConnectionDisconnect } from "@/hooks/useConfirmAIConnectionDisconnect";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { MoreHorizontalIcon, AiNetworkIcon, PlusSignIcon } from "@/lib/icons";
import { SettingsPageFrame } from "./SettingsPageFrame";

// Retain legacy routes; both now land on the same model list.
export function AISettingsPage(_props: { scope?: "personal" | "workspace" }) {
  return (
    <SettingsPageFrame section="ai">
      {({ workspaceId, permissions }) => (
        <AISettingsContent
          key={workspaceId}
          workspaceId={workspaceId}
          canManageWorkspace={permissions.has("workspace.update")}
        />
      )}
    </SettingsPageFrame>
  );
}

function AISettingsContent({
  workspaceId,
  canManageWorkspace,
}: {
  workspaceId: string;
  canManageWorkspace: boolean;
}) {
  const [adding, setAdding] = useState<"connections" | "providers" | null>(null);
  const [modelOrigin, setModelOrigin] = useState<"connections" | "manage">("connections");
  const [managing, setManaging] = useState(false);
  const connectionsDialogRef = useRef<HTMLDivElement>(null);
  const [dialog, setDialog] = useState<AIConnectionDialogMode | null>(null);
  const [modelsFor, setModelsFor] = useState<AIConnection | null>(null);
  const [removing, setRemoving] = useState<AIProfile | null>(null);
  const [editing, setEditing] = useState<AIProfile | null>(null);
  const connections = useAIConnections(workspaceId);
  const profiles = useAIProfiles(workspaceId, {
    enabled: connections.data?.enabled === true,
  });
  const settings = useAISettings(workspaceId, {
    enabled: connections.data?.enabled === true,
  });
  const all = connections.data?.connections ?? [];
  const models = profiles.data ?? [];
  const { requestDisconnect } = useConfirmAIConnectionDisconnect(workspaceId, models);
  const editable = (scope: string) =>
    scope === "personal" || canManageWorkspace;
  const selected = all.find((c) => c.id === modelsFor?.id) ?? modelsFor;
  function openAddModels() {
    setAdding("connections");
    if (connections.isStale && !connections.isFetching) void connections.refetch();
  }
  function openModels(connection: AIConnection) {
    setModelOrigin(managing ? "manage" : "connections");
    setAdding(null);
    setManaging(false);
    setModelsFor(connection);
  }
  function reconnect(connection: AIConnection) {
    setAdding(null);
    setManaging(false);
    setDialog({
      kind: "reconnect",
      connection,
      autoPoll: connection.status === "pending",
    });
  }
  if (connections.isPending) return <Skeleton className="h-40 w-full" />;
  if (connections.isError)
    return (
      <Alert variant="destructive">
        <AlertTitle>Could not load AI connections</AlertTitle>
        <AlertDescription>
          <Button
            size="sm"
            variant="outline"
            onClick={() => void connections.refetch()}
          >
            Retry
          </Button>
        </AlertDescription>
      </Alert>
    );
  if (!connections.data?.enabled)
    return (
      <AIEmptyHero
        icon={AiNetworkIcon}
        title="AI connections are not configured"
        description="Ask your administrator to configure AI connection encryption and the agent runtime."
      />
    );
  const usageGroups = new Map<string, string[]>();
  for (const model of models) {
    const connection = all.find(c => c.id === model.primary.connection_id);
    const pricing = aiUsagePricingText((model.primary_policy ?? connection?.policy)?.pricing);
    if (!pricing) continue;
    const names = usageGroups.get(pricing) ?? [];
    const connectionName = connection?.name ?? model.subscription_name;
    const needsConnection = connectionName && models.some(other => other.id !== model.id && other.name === model.name);
    names.push(needsConnection ? `${model.name} (${connectionName})` : model.name);
    usageGroups.set(pricing, names);
  }
  const commonPricing = sharedAIModelsPricing(all.map(c => c.policy));
  if (!models.length && commonPricing) usageGroups.set(commonPricing, []);
  const knowledge = connections.data.knowledge;
  const knowledgeDetail = knowledge
    ? `${knowledge.embeddings_configured ? `${knowledge.embedding_detail || "Embeddings configured"}: ${knowledge.embedding_model} (${knowledge.embedding_dimensions} dimensions). Connectivity has not been verified.` : "Semantic search is not configured. Knowledge search uses keyword matching only. Connect OpenAI or OpenRouter as a workspace connection, or configure a server key."} Agent model choices do not configure help-center AI answers or automatic triage; these use server provider settings.${knowledge.chat_providers.length === 0 ? " No server chat provider is configured." : ""}`
    : "";
  return (
    <AIModelsPricingContext.Provider value={commonPricing && usageGroups.has(commonPricing) ? commonPricing : null}>
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <span className="text-sm font-medium">
          Models
          {!profiles.isPending && !profiles.isError
            ? ` · ${models.length}`
            : ""}
        </span>
        <div className="flex items-center gap-2">
          <Button size="sm" onClick={openAddModels}>
            <PlusSignIcon className="size-4" />
            Add models
          </Button>
          <DropdownMenu>
            <DropdownMenuTrigger asChild><Button size="icon" variant="ghost" className="size-8" aria-label="AI model settings actions"><MoreHorizontalIcon className="size-4" /></Button></DropdownMenuTrigger>
            <DropdownMenuContent align="end"><DropdownMenuItem onSelect={() => setManaging(true)}>Manage connections</DropdownMenuItem></DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>
      {settings.isError && (
        <Alert variant="destructive">
          <AlertTitle>Could not load defaults</AlertTitle>
          <AlertDescription>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => void settings.refetch()}
            >
              Retry defaults
            </Button>
          </AlertDescription>
        </Alert>
      )}
      {profiles.isPending ? (
        <Skeleton className="h-40 w-full" />
      ) : profiles.isError ? (
        <Alert variant="destructive">
          <AlertTitle>Could not load models</AlertTitle>
          <AlertDescription>
            <Button
              size="sm"
              variant="ghost"
              onClick={() => void profiles.refetch()}
            >
              Retry models
            </Button>
          </AlertDescription>
        </Alert>
      ) : models.length === 0 ? (
        <AIEmptyHero
          icon={AiNetworkIcon}
          title="No models added"
          description="Add models from a connected provider, or connect a new one."
        />
      ) : (
        <div className="overflow-x-auto rounded-lg border">
          <Table>
            <TableHeader>
              <TableRow className="bg-muted/40 hover:bg-muted/40">
                <TableHead>Model</TableHead>
                <TableHead>Connection</TableHead>
                <TableHead>
                  <span className="inline-flex items-center gap-1">
                    Access
                    <AISetupHelp
                      label="About model access"
                      description="Personal models are for your Ask Agent chats and manual runs. Workspace-wide models can also be assigned to agents for scheduled and automated runs. Administrators manage shared connections. A ChatGPT subscription owner can share an individual model."
                    />
                  </span>
                </TableHead>
                <TableHead>Defaults</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>
                  <span className="sr-only">Actions</span>
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {models.map((profile) => (
                <AIModelTableRow
                  key={profile.id}
                  workspaceId={workspaceId}
                  profile={profile}
                  connection={all.find(
                    (c) => c.id === profile.primary.connection_id,
                  )}
                  settings={settings.isError ? undefined : settings.data}
                  canManageWorkspace={canManageWorkspace}
                  onRemove={setRemoving}
                  onEdit={() => setEditing(profile)}
                  onReconnect={() => {
                    const connection = all.find(
                      (c) => c.id === profile.primary.connection_id,
                    );
                    if (connection) reconnect(connection);
                  }}
                />
              ))}
            </TableBody>
          </Table>
        </div>
      )}
      {usageGroups.size > 0 && (
        <section aria-label="Model usage" className="space-y-1 text-xs text-muted-foreground">
          <h3 className="font-medium text-foreground">Usage</h3>
          {Array.from(usageGroups, ([pricing, names]) => (
            <p key={pricing}>
              {names.length < models.length && <strong className="font-medium">{names.join(", ")} — </strong>}
              {pricing}
            </p>
          ))}
        </section>
      )}
      {knowledge && (
        <div className="flex items-center gap-1 text-xs text-muted-foreground">
          <span>
            Knowledge search ·{" "}
            {knowledge.embeddings_configured
              ? "Semantic search configured"
              : "Keyword search only"}
          </span>
          <AISetupHelp
            label="About knowledge search"
            description={knowledgeDetail}
          />
        </div>
      )}
      {adding && (
        <AIAddModelsDialog
          connections={all}
          models={connections.data.models}
          canManageWorkspace={canManageWorkspace}
          startWithProviders={adding === "providers"}
          onDisconnect={requestDisconnect}
          onClose={() => setAdding(null)}
          onModels={openModels}
          onReconnect={reconnect}
          onConnect={(provider) => {
            setAdding(null);
            setDialog({ kind: "add", provider });
          }}
        />
      )}
      {managing && (
        <Dialog
          open
          onOpenChange={(open) => {
            if (!open) setManaging(false);
          }}
        >
          <DialogContent
            ref={connectionsDialogRef}
            className="max-h-[85vh] overflow-y-auto sm:max-w-3xl"
            onOpenAutoFocus={(event) => {
              // Announce the dialog without opening the first row's help tooltip.
              event.preventDefault();
              connectionsDialogRef.current?.focus();
            }}
          >
            <DialogHeader className="text-left">
              <DialogTitle>Manage connections</DialogTitle>
              <DialogDescription>
                Manage provider access and the models available through each connection.
              </DialogDescription>
            </DialogHeader>
            {all.length ? (
              <div className="overflow-x-auto">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Connection</TableHead>
                      <TableHead>Access</TableHead>
                      <TableHead>Status</TableHead>
                      <TableHead>Models</TableHead>
                      <TableHead>
                        <span className="sr-only">Actions</span>
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {all.map((connection) => (
                      <AIConnectionRow
                        key={connection.id}
                        workspaceId={workspaceId}
                        connection={connection}
                        canManage={editable(connection.scope)}
                        onReconnect={reconnect}
                        onContinueLogin={reconnect}
                        onModels={() => openModels(connection)}
                        profiles={models}
                        modelCount={
                          profiles.data?.filter(
                            (p) =>
                              p.primary.connection_id === connection.id,
                          ).length
                        }
                      />
                    ))}
                  </TableBody>
                </Table>
              </div>
            ) : (
              <p className="py-4 text-sm text-muted-foreground">
                No connections yet.
              </p>
            )}
            {(profiles.data ?? [])
              .filter((p) => !p.agent_grant_id && !all.some((c) => c.id === p.primary.connection_id))
              .map((profile) => (
                <Button
                  key={profile.id}
                  variant="outline"
                  size="sm"
                  disabled={!editable(profile.scope)}
                  onClick={() => {
                    setManaging(false);
                    setEditing(profile);
                  }}
                >
                  Repair {profile.name}
                </Button>
              ))}
            <Button
              size="sm"
              variant="outline"
              className="w-fit"
              onClick={() => {
                setManaging(false);
                setAdding("providers");
              }}
            >
              <PlusSignIcon className="size-4" />
              Add connection
            </Button>
          </DialogContent>
        </Dialog>
      )}
      {dialog && (
        <AIConnectionDialog
          workspaceId={workspaceId}
          scope={
            dialog.kind === "reconnect" ? dialog.connection.scope : "personal"
          }
          canChooseScope={canManageWorkspace}
          open
          mode={dialog}
          models={connections.data.models}
          onOpenChange={(open) => {
            if (!open) setDialog(null);
          }}
          onBack={() => { setDialog(null); setAdding("providers"); }}
          onConnected={(connection) => {
            setDialog(null);
            setModelOrigin("connections");
            setModelsFor(connection);
          }}
        />
      )}
      {selected && (
        <AIConnectionModelsDialog
          workspaceId={workspaceId}
          connection={selected}
          connections={all}
          canManage={editable(selected.scope)}
          onRemove={setRemoving}
          onBack={() => {
            setModelsFor(null);
            if (modelOrigin === "manage") setManaging(true);
            else setAdding("connections");
          }}
          onClose={() => setModelsFor(null)}
        />
      )}
      {removing && <RemoveAIModelDialog workspaceId={workspaceId} profile={removing} onClose={() => setRemoving(null)} />}
      {editing && (
        <AIProfileEditor
          workspaceId={workspaceId}
          scope={editing.scope}
          profile={editing}
          connections={all}
          onClose={() => setEditing(null)}
        />
      )}
    </div>
    </AIModelsPricingContext.Provider>
  );
}
