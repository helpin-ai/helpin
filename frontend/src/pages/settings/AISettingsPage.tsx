import { useState } from "react";
import {
  AIConnectionDialog,
  type AIConnectionDialogMode,
} from "@/components/agents/AIConnectionDialog";
import { AIConnectionRow } from "@/components/agents/AIConnectionRow";
import { AIConnectionModelsDialog } from "@/components/agents/AIConnectionModelsDialog";
import { AISetupHelp } from "@/components/agents/AISetupHelp";
import { AIEmptyHero } from "@/components/settings/ai/AIEmptyHero";
import { AISectionLabel } from "@/components/settings/ai/AISectionLabel";
import { useAIConnections } from "@/hooks/queries/useAIConnections";
import { useAIProfiles } from "@/hooks/queries/useAIProfiles";
import type { AIConnection } from "@/lib/services/aiConnectionService";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
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
import { AiNetworkIcon, Key01Icon, PlusSignIcon } from "@/lib/icons";
import { SettingsPageFrame } from "./SettingsPageFrame";

export function AISettingsPage({
  scope = "workspace",
}: {
  scope?: "personal" | "workspace";
}) {
  return (
    <SettingsPageFrame section="ai">
      {({ workspaceId, permissions }) =>
        permissions.has("workspace.update") ? (
          <Tabs
            key={`${workspaceId}-${scope}`}
            defaultValue={scope}
            className="gap-5"
          >
            <TabsList variant="quiet" aria-label="AI setup scope">
              <TabsTrigger value="workspace">Workspace</TabsTrigger>
              <TabsTrigger value="personal">Personal</TabsTrigger>
            </TabsList>
            <TabsContent value="workspace">
              <AISettingsContent
                workspaceId={workspaceId}
                scope="workspace"
                canManage
              />
            </TabsContent>
            <TabsContent value="personal">
              <AISettingsContent
                workspaceId={workspaceId}
                scope="personal"
                canManage
              />
            </TabsContent>
          </Tabs>
        ) : (
          <div key={workspaceId} className="space-y-8">
            <AISettingsContent
              workspaceId={workspaceId}
              scope="personal"
              canManage
              showScope
            />
            <AISettingsContent
              workspaceId={workspaceId}
              scope="workspace"
              canManage={false}
              showScope
            />
          </div>
        )
      }
    </SettingsPageFrame>
  );
}

function AISettingsContent({
  workspaceId,
  scope,
  canManage,
  showScope = false,
}: {
  workspaceId: string;
  scope: "personal" | "workspace";
  canManage: boolean;
  showScope?: boolean;
}) {
  const [dialog, setDialog] = useState<AIConnectionDialogMode | null>(null);
  const [modelsFor, setModelsFor] = useState<AIConnection | null>(null);
  const connections = useAIConnections(workspaceId);
  const profiles = useAIProfiles(workspaceId, {
    enabled: connections.data?.enabled === true,
  });
  const all = connections.data?.connections ?? [];
  const own = all.filter((c) => (c.scope || "personal") === scope);
  const unavailable = [
    ...new Map(
      (profiles.data ?? [])
        .filter(
          (p) =>
            p.scope === scope &&
            !all.some((c) => c.id === p.primary.connection_id),
        )
        .map((p) => [p.primary.connection_id, p]),
    ).values(),
  ];
  if (connections.isPending) return <Skeleton className="h-40 w-full" />;
  if (connections.isError)
    return (
      <Alert variant="destructive">
        <AlertTitle>Could not load AI connections</AlertTitle>
        <AlertDescription>
          <Button
            variant="outline"
            size="sm"
            onClick={() => void connections.refetch()}
          >
            Retry
          </Button>
        </AlertDescription>
      </Alert>
    );
  if (!connections.data?.enabled)
    return showScope && scope === "workspace" ? null : (
      <AIEmptyHero
        icon={AiNetworkIcon}
        title="AI connections are not configured"
        description="Ask your administrator to configure AI connection encryption and the agent runtime."
      />
    );
  const knowledge =
    scope === "workspace" ? connections.data.knowledge : undefined;
  const knowledgeDetail = knowledge
    ? `${knowledge.embeddings_configured ? `${knowledge.embedding_detail || "Embeddings configured"}: ${knowledge.embedding_model} (${knowledge.embedding_dimensions} dimensions). Connectivity has not been verified.` : "Semantic search is not configured. Knowledge search uses keyword matching only. Connect OpenAI or OpenRouter as a workspace connection, or configure a server key."} Agent model choices do not configure help-center AI answers or automatic triage; these use server provider settings.${knowledge.chat_providers.length === 0 ? " No server chat provider is configured." : ""}`
    : "";
  const selected = all.find((c) => c.id === modelsFor?.id) ?? modelsFor;
  return (
    <div className="space-y-4">
      <section>
        <AISectionLabel
          label={
            showScope
              ? `${scope === "personal" ? "Personal" : "Workspace"} connections`
              : "Connections"
          }
          count={own.length}
          action={
            canManage ? (
              <Button
                size="sm"
                className="gap-1.5"
                onClick={() => setDialog({ kind: "add" })}
              >
                <PlusSignIcon className="size-4" />
                Add connection
              </Button>
            ) : (
              <AISetupHelp
                label="About shared connections"
                description="Workspace models are available in Ask Agent. Only administrators can manage shared connections and model visibility."
              />
            )
          }
        />
        {own.length === 0 ? (
          <AIEmptyHero
            icon={Key01Icon}
            title="No connections yet"
            description={
              canManage
                ? "Connect a provider, then choose which models appear in Ask Agent."
                : "Your workspace admins can add shared models."
            }
          />
        ) : (
          <div className="overflow-x-auto rounded-lg border">
            <Table>
              <TableHeader>
                <TableRow className="bg-muted/40 hover:bg-muted/40">
                  <TableHead>Connection</TableHead>
                  <TableHead>Provider</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Details</TableHead>
                  <TableHead className="text-right">Models</TableHead>
                  {canManage && (
                    <TableHead>
                      <span className="sr-only">Actions</span>
                    </TableHead>
                  )}
                </TableRow>
              </TableHeader>
              <TableBody>
                {own.map((connection) => (
                  <AIConnectionRow
                    key={connection.id}
                    workspaceId={workspaceId}
                    connection={connection}
                    canManage={canManage}
                    onReconnect={(c) =>
                      setDialog({ kind: "reconnect", connection: c })
                    }
                    onContinueLogin={(c) =>
                      setDialog({
                        kind: "reconnect",
                        connection: c,
                        autoPoll: true,
                      })
                    }
                    onModels={() => setModelsFor(connection)}
                    modelCount={
                      profiles.data?.filter(
                        (p) =>
                          p.primary.connection_id === connection.id &&
                          !p.hidden_from_ask_agent,
                      ).length
                    }
                  />
                ))}
              </TableBody>
            </Table>
          </div>
        )}
      </section>
      {unavailable.length > 0 && (
        <section className="space-y-2">
          <div className="flex items-center gap-1 text-sm font-medium">
            Models needing a connection
            <AISetupHelp
              label="About unavailable connections"
              description="These saved configurations are preserved. Open their model settings to choose an available connection; existing run history is unchanged."
            />
          </div>
          {unavailable.map((p) => (
            <Button
              key={p.primary.connection_id}
              variant="outline"
              size="sm"
              className="mr-2"
              aria-label={`Models for unavailable connection ${p.primary.connection_id}`}
              onClick={() =>
                setModelsFor({
                  id: p.primary.connection_id,
                  name: "Unavailable connection",
                  provider: p.primary.model.provider,
                  scope: p.scope,
                  user_id: p.user_id,
                  status: "disconnected",
                })
              }
            >
              {p.name}
            </Button>
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
      {dialog && (
        <AIConnectionDialog
          workspaceId={workspaceId}
          scope={scope}
          open
          mode={dialog}
          models={connections.data.models}
          onOpenChange={(open) => {
            if (!open) setDialog(null);
          }}
          onConnected={(c) => {
            setDialog(null);
            setModelsFor(c);
          }}
        />
      )}
      {selected && (
        <AIConnectionModelsDialog
          workspaceId={workspaceId}
          connection={selected}
          connections={all}
          canManage={canManage}
          onClose={() => setModelsFor(null)}
        />
      )}
    </div>
  );
}
