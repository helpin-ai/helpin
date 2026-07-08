import { useEffect, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { BookOpen01Icon, LinkSquare01Icon, MagicWand01Icon, PlusSignIcon, Settings02Icon } from '@/lib/icons';
import { useDocsSpaces } from '@/hooks/queries';
import { useChatSettings, useAgentKnowledgeSources, useUpdateAgentKnowledgeSources, useReindexAgentKnowledgeSource, useSupportContentSources, useCreateSupportContentSource } from '@/hooks/queries/useSupport';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { workspacesService } from '@/lib/services/workspacesService';
import {
  companyProductContextGenerateDisabledReason,
  companyProductContextGenerateHelper,
  companyProductContextGenerateLabel,
  shouldConfirmCompanyProductContextReplacement,
} from '@/lib/companyProductContext';
import { findWorkspaceWebsiteContentSource, buildWorkspaceWebsiteContentSourcePayload } from '@/lib/workspaceWebsiteSource';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { UpgradeRequiredDialog } from '@/components/billing/UpgradeRequiredDialog';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Favicon } from '@/components/ui/favicon';
import { Label } from '@/components/ui/label';
import { Skeleton } from '@/components/ui/skeleton';
import { Textarea } from '@/components/ui/textarea';
import { SupportContentSourcesField } from './SupportContentSourcesField';
import { SupportKnowledgeSourcesField } from './SupportKnowledgeSourcesField';
import { LINEAR_CARD_CLASS } from './settingsConstants';
import { getUpgradeRequiredReason, type UpgradeRequiredReason } from '@/lib/upgradeRequired';
import { toast } from 'sonner';

function KnowledgePageIntro({
  hasAnySource,
  workspaceSlug,
  onOpenDocs,
  onOpenAIAssistant,
}: {
  hasAnySource: boolean;
  workspaceSlug?: string;
  onOpenDocs: () => void;
  onOpenAIAssistant: () => void;
}) {
  return (
    <div className="space-y-4">
      <div>
        <h2 className="text-xl font-semibold">Knowledge</h2>
        <p className="mt-1 max-w-3xl text-sm leading-6 text-muted-foreground">
          Manage the docs and websites that AI can search when answering questions or helping teammates.
        </p>
      </div>

      {!hasAnySource ? (
        <div className="rounded-xl border border-dashed border-border/80 bg-card px-6 py-8">
          <div className="mx-auto max-w-xl text-center">
            <div className="mx-auto mb-3 flex h-11 w-11 items-center justify-center rounded-full bg-muted text-muted-foreground">
              <BookOpen01Icon className="h-5 w-5" />
            </div>
            <h3 className="text-base font-semibold">No knowledge sources yet</h3>
            <p className="mt-2 text-sm leading-6 text-muted-foreground">
              Add a public docs space or website source to make workspace knowledge available to AI.
            </p>
            {workspaceSlug ? (
              <div className="mt-4 flex flex-wrap justify-center gap-2">
                <Button type="button" size="sm" onClick={onOpenDocs}>
                  <PlusSignIcon className="h-4 w-4" />
                  Open docs
                </Button>
                <Button type="button" variant="outline" size="sm" onClick={onOpenAIAssistant}>
                  <Settings02Icon className="h-4 w-4" />
                  AI Assistant
                </Button>
              </div>
            ) : null}
          </div>
        </div>
      ) : null}
    </div>
  );
}

export function KnowledgeTab({ workspaceId }: { workspaceId: string }) {
  const navigate = useNavigate();
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const [companyDescription, setCompanyDescription] = useState(workspace?.company_product_context ?? workspace?.description ?? '');
  const [savingDescription, setSavingDescription] = useState(false);
  const [generatingCompanyContext, setGeneratingCompanyContext] = useState(false);
  const [companyContextExpanded, setCompanyContextExpanded] = useState(false);
  const [upgradeDialogReason, setUpgradeDialogReason] = useState<UpgradeRequiredReason | null>(null);
  const savedCompanyProductContext = workspace?.company_product_context ?? workspace?.description ?? '';
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
  const hasAnySource = externalDocsSpaces.length > 0 || contentSources.length > 0 || Boolean(workspace?.website_url);
  const workspaceSlug = workspace?.slug ?? '';
  const shouldCollapseCompanyContext = companyDescription.length > 700 || companyDescription.split(/\r?\n/).length > 8;
  const generateCompanyContextDisabledReason = companyProductContextGenerateDisabledReason(workspace?.website_url);

  useEffect(() => {
    setCompanyDescription(workspace?.company_product_context ?? workspace?.description ?? '');
    setCompanyContextExpanded(false);
  }, [workspace?.id, workspace?.company_product_context, workspace?.description]);

  const openDocs = () => {
    if (!workspaceSlug) return;
    void navigate({ to: '/w/$slug/docs', params: { slug: workspaceSlug } });
  };

  const openAIAssistantSettings = () => {
    if (!workspaceSlug) return;
    void navigate({ to: '/w/$slug/settings/support-ai-assistant', params: { slug: workspaceSlug } });
  };

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

  const handleSaveCompanyDescription = async () => {
    if (!workspace) return;
    setSavingDescription(true);
    const { data, error } = await workspacesService.update(workspace.id, {
      company_product_context: companyDescription.trim(),
    });
    setSavingDescription(false);
    if (error) {
      toast.error('Failed to save company/product context', { description: error });
      return;
    }
    if (data) {
      useWorkspaceStore.getState().setCurrentWorkspace(data);
    }
    toast.success('Company/Product Context saved');
  };

  const handleGenerateCompanyContext = async () => {
    if (!workspace) return;
    if (!workspace.website_url) {
      toast.error('Website URL required', { description: 'Add a website URL in General settings to generate context.' });
      return;
    }
    if (
      shouldConfirmCompanyProductContextReplacement(companyDescription, savedCompanyProductContext) &&
      !window.confirm('Replace the current unsaved company/product context with an AI-generated draft?')
    ) {
      return;
    }

    setGeneratingCompanyContext(true);
    const { data, error } = await workspacesService.generateCompanyProductDescription({
      workspace_name: workspace.name,
      website_url: workspace.website_url,
    });
    setGeneratingCompanyContext(false);
    if (error) {
      const reason = getUpgradeRequiredReason(error);
      if (reason) {
        setUpgradeDialogReason(reason);
        return;
      }
      toast.error('Could not generate company/product context', { description: error });
      return;
    }
    const generated = data?.company_product_context || data?.description || '';
    if (!generated.trim()) {
      toast.error('Generated company/product context was empty');
      return;
    }
    setCompanyDescription(generated);
    setCompanyContextExpanded(true);
    toast.success('AI draft generated. Review and save when ready.');
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
    <>
    <div className="space-y-4">
      <KnowledgePageIntro
        hasAnySource={hasAnySource}
        workspaceSlug={workspaceSlug}
        onOpenDocs={openDocs}
        onOpenAIAssistant={openAIAssistantSettings}
      />

      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader>
          <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
            <div className="flex items-center gap-2">
              <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-muted text-muted-foreground">
                <BookOpen01Icon className="h-4 w-4" />
              </div>
              <div>
                <CardTitle className="text-base">Company/Product Context</CardTitle>
              </div>
            </div>
            <div className="flex flex-col items-start gap-1 sm:items-end">
              <Button
                type="button"
                size="sm"
                variant="outline"
                onClick={() => void handleGenerateCompanyContext()}
                disabled={generatingCompanyContext || Boolean(generateCompanyContextDisabledReason)}
              >
                <MagicWand01Icon className="h-4 w-4" />
                {generatingCompanyContext ? 'Generating...' : companyProductContextGenerateLabel(savedCompanyProductContext)}
              </Button>
              <p className="max-w-xs text-xs leading-5 text-muted-foreground sm:text-right">
                {generateCompanyContextDisabledReason || companyProductContextGenerateHelper}
              </p>
            </div>
          </div>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="space-y-2">
            <div className="flex items-center justify-between gap-3">
              <Label htmlFor="company-product-context">Company/Product description</Label>
              {shouldCollapseCompanyContext ? (
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  className="h-7 px-2 text-xs"
                  onClick={() => setCompanyContextExpanded((current) => !current)}
                >
                  {companyContextExpanded ? 'Show less' : 'Show more'}
                </Button>
              ) : null}
            </div>
            <div className={shouldCollapseCompanyContext && !companyContextExpanded ? 'relative max-h-44 overflow-hidden' : 'relative'}>
              <Textarea
                id="company-product-context"
                value={companyDescription}
                onChange={(event) => setCompanyDescription(event.target.value)}
                placeholder="Describe what your company or product does, who it serves, and what problems it solves."
                rows={companyContextExpanded ? 14 : 8}
                className={shouldCollapseCompanyContext && !companyContextExpanded ? 'resize-none' : undefined}
              />
              {shouldCollapseCompanyContext && !companyContextExpanded ? (
                <div className="pointer-events-none absolute inset-x-px bottom-px h-16 rounded-b-md bg-gradient-to-t from-background via-background/90 to-transparent" />
              ) : null}
            </div>
          </div>
          <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
            <p className="text-sm text-muted-foreground">
              Used by Helpin agents as workspace-level context for support answers, docs, planning, automation, and product-aware work.
            </p>
            <Button
              type="button"
              size="sm"
              onClick={() => void handleSaveCompanyDescription()}
              disabled={savingDescription || companyDescription === savedCompanyProductContext}
            >
              {savingDescription ? 'Saving...' : 'Save'}
            </Button>
          </div>
        </CardContent>
      </Card>

      <div className="space-y-3">
        {workspace?.website_url && (
          <div className="rounded-xl border border-border/70 bg-card p-4">
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
                <p className="flex items-center gap-1.5 text-sm">
                  <LinkSquare01Icon className="h-3.5 w-3.5 text-muted-foreground" />
                  {workspace.website_url}
                </p>
              </div>
              {!websiteContentSource && (
                <Button
                  type="button"
                  size="sm"
                  onClick={() => void handleAddWorkspaceWebsiteSource()}
                  disabled={createWebsiteSource.isPending}
                >
                  {createWebsiteSource.isPending ? 'Adding...' : 'Add as source'}
                </Button>
              )}
            </div>
          </div>
        )}
        <SupportContentSourcesField
          workspaceId={workspaceId}
          agentId={chatWidgetAgentId || undefined}
          disabled={updateKnowledgeSources.isPending}
        />
      </div>

      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader>
          <div className="flex items-center gap-2">
            <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-muted text-muted-foreground">
              <BookOpen01Icon className="h-4 w-4" />
            </div>
            <div>
              <CardTitle className="text-base">Help Center Docs</CardTitle>
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
              onOpenDocs={openDocs}
            />
          )}
          {!chatWidgetAgentId && (
            <p className="mt-3 text-sm text-muted-foreground">
              Select a support agent in AI Assistant to attach docs.
            </p>
          )}
        </CardContent>
      </Card>
    </div>
      <UpgradeRequiredDialog
        open={upgradeDialogReason !== null}
        onOpenChange={(open) => {
          if (!open) setUpgradeDialogReason(null);
        }}
        reason={upgradeDialogReason}
      />
    </>
  );
}
