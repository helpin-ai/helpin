import { BookOpen01Icon, GlobeIcon } from '@/lib/icons';
import { useDocsSpaces } from '@/hooks/queries';
import { useChatSettings, useAgentKnowledgeSources, useUpdateAgentKnowledgeSources, useReindexAgentKnowledgeSource, useSupportContentSources, useCreateSupportContentSource } from '@/hooks/queries/useSupport';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { findWorkspaceWebsiteContentSource, buildWorkspaceWebsiteContentSourcePayload } from '@/lib/workspaceWebsiteSource';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Favicon } from '@/components/ui/favicon';
import { Skeleton } from '@/components/ui/skeleton';
import { SupportContentSourcesField } from './SupportContentSourcesField';
import { SupportKnowledgeSourcesField } from './SupportKnowledgeSourcesField';
import { LINEAR_CARD_CLASS } from './settingsConstants';
import { toast } from 'sonner';

export function KnowledgeTab({ workspaceId }: { workspaceId: string }) {
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const { data: chatSettings, isLoading: chatSettingsLoading } = useChatSettings(workspaceId);
  const { data: docsSpaces = [], isLoading: docsSpacesLoading } = useDocsSpaces(workspaceId);
  const { data: contentSources = [] } = useSupportContentSources(workspaceId);
  const chatWidgetAgentId = chatSettings?.settings?.ai_agent_id ?? '';
  const { data: knowledgeSources = [], isLoading: knowledgeSourcesLoading } = useAgentKnowledgeSources(workspaceId, chatWidgetAgentId || undefined);
  const updateKnowledgeSources = useUpdateAgentKnowledgeSources(workspaceId);
  const reindexKnowledgeSource = useReindexAgentKnowledgeSource(workspaceId);
  const createWebsiteSource = useCreateSupportContentSource(workspaceId);

  const externalDocsSpaces = docsSpaces.filter((space) => space.type === 'external_capable');
  const externalDocsSpaceIds = new Set(externalDocsSpaces.map((space) => space.id));
  const externalKnowledgeSources = knowledgeSources.filter((source) => externalDocsSpaceIds.has(source.space_id));
  const websiteContentSource = findWorkspaceWebsiteContentSource(workspace?.website_url, contentSources);

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

  const handleAddWorkspaceWebsiteSource = async () => {
    if (!workspace?.website_url) {
      return;
    }

    await createWebsiteSource.mutateAsync(
      buildWorkspaceWebsiteContentSourcePayload(workspace.name, workspace.website_url),
    );
    toast.success('Website source added and syncing');
  };

  const websiteSourceStatusLabel = (() => {
    switch (websiteContentSource?.sync_status) {
      case 'queued':
        return 'Queued';
      case 'running':
        return 'Syncing';
      case 'ready':
        return 'Ready';
      case 'failed':
        return 'Failed';
      case 'stale':
        return 'Outdated';
      case 'disabled':
        return 'Disabled';
      default:
        return null;
    }
  })();

  if (chatSettingsLoading) {
    return (
      <div className="space-y-4">
        <Skeleton className="h-28 w-full rounded-xl" />
        <Skeleton className="h-64 w-full rounded-xl" />
        <Skeleton className="h-64 w-full rounded-xl" />
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {!chatWidgetAgentId && (
        <Card className={LINEAR_CARD_CLASS}>
          <CardHeader>
            <CardTitle className="text-base">Knowledge</CardTitle>
            <CardDescription>Manage help center docs and website content sources used across AI experiences.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-2">
            <p className="text-sm text-muted-foreground">
              You can add and sync website content sources now. Help Center docs become attachable to support AI after you assign a support agent in Chat Widget.
            </p>
            <p className="text-sm text-muted-foreground">
              Website sources are saved at the workspace level, so they will be ready to attach once a support agent is configured.
            </p>
          </CardContent>
        </Card>
      )}

      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader>
          <div className="flex items-center gap-2">
            <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-muted text-muted-foreground">
              <BookOpen01Icon className="h-4 w-4" />
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
              <Skeleton className="h-16 rounded-xl" />
              <Skeleton className="h-16 rounded-xl" />
            </div>
          ) : (
            <SupportKnowledgeSourcesField
              agentId={chatWidgetAgentId || undefined}
              spaces={externalDocsSpaces}
              knowledgeSources={externalKnowledgeSources}
              onToggle={toggleSpace}
              onReindex={(spaceId) => chatWidgetAgentId && reindexKnowledgeSource.mutate({ agentId: chatWidgetAgentId, spaceId })}
              reindexingSpaceId={reindexKnowledgeSource.variables?.spaceId}
              disabled={!chatWidgetAgentId || updateKnowledgeSources.isPending || reindexKnowledgeSource.isPending}
            />
          )}
          {!chatWidgetAgentId && (
            <p className="mt-3 text-sm text-muted-foreground">
              Assign a support agent in Chat Widget to choose which Help Center spaces are searchable by support AI.
            </p>
          )}
        </CardContent>
      </Card>

      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader>
          <div className="flex items-center gap-2">
            <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-muted text-muted-foreground">
              <GlobeIcon className="h-4 w-4" />
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
          {workspace?.website_url && (
            <div className="mb-4 rounded-xl border border-border/70 bg-muted/20 p-4">
              <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                <div className="space-y-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <Favicon
                      url={workspace.website_url}
                      name={workspace.name}
                      size={32}
                      className="h-5 w-5 rounded-md"
                      fallbackClassName="text-[8px]"
                    />
                    <p className="text-sm font-medium">Workspace website</p>
                    {websiteSourceStatusLabel && <Badge variant="secondary">{websiteSourceStatusLabel}</Badge>}
                  </div>
                  <p className="text-sm">{workspace.website_url}</p>
                  {websiteContentSource ? (
                    <p className="text-xs text-muted-foreground">
                      This URL is already connected as a Website Content Source for Support AI.
                    </p>
                  ) : (
                    <p className="text-xs text-muted-foreground">
                      This URL is saved on the workspace. Add it here to crawl the site and make it available for Support AI.
                    </p>
                  )}
                </div>
                {!websiteContentSource && (
                  <div className="flex items-center gap-2">
                    <Button
                      type="button"
                      size="sm"
                      onClick={() => void handleAddWorkspaceWebsiteSource()}
                      disabled={createWebsiteSource.isPending}
                    >
                      {createWebsiteSource.isPending ? 'Adding...' : 'Add as Source'}
                    </Button>
                  </div>
                )}
              </div>
            </div>
          )}
          <SupportContentSourcesField
            workspaceId={workspaceId}
            agentId={chatWidgetAgentId || undefined}
            disabled={updateKnowledgeSources.isPending}
          />
        </CardContent>
      </Card>
    </div>
  );
}
