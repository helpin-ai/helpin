import { BookOpen, Globe } from 'lucide-react';
import { useDocsSpaces } from '@/hooks/queries';
import { useChatSettings, useSupportAgents, useAgentKnowledgeSources, useUpdateAgentKnowledgeSources, useReindexAgentKnowledgeSource } from '@/hooks/queries/useSupport';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { SupportContentSourcesField } from './SupportContentSourcesField';
import { SupportKnowledgeSourcesField } from './SupportKnowledgeSourcesField';
import { LINEAR_CARD_CLASS } from './settingsConstants';

export function KnowledgeTab({ workspaceId }: { workspaceId: string }) {
  const { data: chatSettings, isLoading: chatSettingsLoading } = useChatSettings(workspaceId);
  const { data: docsSpaces = [], isLoading: docsSpacesLoading } = useDocsSpaces(workspaceId);
  const { data: supportAgents = [], isLoading: supportAgentsLoading } = useSupportAgents(workspaceId);
  const chatWidgetAgentId = chatSettings?.settings?.ai_agent_id ?? '';
  const { data: knowledgeSources = [], isLoading: knowledgeSourcesLoading } = useAgentKnowledgeSources(workspaceId, chatWidgetAgentId || undefined);
  const updateKnowledgeSources = useUpdateAgentKnowledgeSources(workspaceId);
  const reindexKnowledgeSource = useReindexAgentKnowledgeSource(workspaceId);

  const externalDocsSpaces = docsSpaces.filter((space) => space.type === 'external_capable');
  const externalDocsSpaceIds = new Set(externalDocsSpaces.map((space) => space.id));
  const externalKnowledgeSources = knowledgeSources.filter((source) => externalDocsSpaceIds.has(source.space_id));

  const toggleSpace = (spaceId: string) => {
    if (!chatWidgetAgentId) {
      return;
    }

    const current = externalKnowledgeSources.map((source) => source.space_id);
    const next = current.includes(spaceId)
      ? current.filter((id) => id !== spaceId)
      : [...current, spaceId];

    updateKnowledgeSources.mutate({ agentId: chatWidgetAgentId, spaceIds: next });
  };

  if (supportAgentsLoading || chatSettingsLoading) {
    return (
      <div className="space-y-4">
        <Skeleton className="h-28 w-full rounded-none" />
        <Skeleton className="h-64 w-full rounded-none" />
        <Skeleton className="h-64 w-full rounded-none" />
      </div>
    );
  }

  if (supportAgents.length === 0) {
    return (
      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader>
          <CardTitle className="text-base">Knowledge</CardTitle>
          <CardDescription>Knowledge sources are assigned per support agent right now.</CardDescription>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">
            No support agents are available yet. Create a support agent first, then return here to connect help center docs and website content.
          </p>
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="space-y-4">
      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader>
          <div className="flex items-center gap-2">
            <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-muted text-muted-foreground">
              <BookOpen className="h-4 w-4" />
            </div>
            <div>
              <CardTitle className="text-base">Help Center Docs</CardTitle>
              <CardDescription>
                Select published help center spaces to index and make searchable for the Chat Widget support agent.
              </CardDescription>
            </div>
          </div>
        </CardHeader>
        <CardContent>
          {docsSpacesLoading || knowledgeSourcesLoading ? (
            <div className="space-y-3">
              <Skeleton className="h-16 rounded-none" />
              <Skeleton className="h-16 rounded-none" />
            </div>
          ) : !chatWidgetAgentId ? (
            <p className="text-sm text-muted-foreground">
              Assign a support agent in Chat Widget to control which Help Center docs are available to support AI.
            </p>
          ) : (
            <SupportKnowledgeSourcesField
              agentId={chatWidgetAgentId}
              spaces={externalDocsSpaces}
              knowledgeSources={externalKnowledgeSources}
              onToggle={toggleSpace}
              onReindex={(spaceId) => reindexKnowledgeSource.mutate({ agentId: chatWidgetAgentId, spaceId })}
              reindexingSpaceId={reindexKnowledgeSource.variables?.spaceId}
              disabled={updateKnowledgeSources.isPending || reindexKnowledgeSource.isPending}
            />
          )}
        </CardContent>
      </Card>

      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader>
          <div className="flex items-center gap-2">
            <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-muted text-muted-foreground">
              <Globe className="h-4 w-4" />
            </div>
            <div>
              <CardTitle className="text-base">Website Content Sources</CardTitle>
              <CardDescription>
                Manage crawled websites and choose which of them are available to the Chat Widget support agent.
              </CardDescription>
            </div>
          </div>
        </CardHeader>
        <CardContent>
          {chatWidgetAgentId ? (
            <SupportContentSourcesField
              workspaceId={workspaceId}
              agentId={chatWidgetAgentId}
              disabled={updateKnowledgeSources.isPending}
            />
          ) : (
            <p className="text-sm text-muted-foreground">
              Assign a support agent in Chat Widget to attach website sources to support AI.
            </p>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
