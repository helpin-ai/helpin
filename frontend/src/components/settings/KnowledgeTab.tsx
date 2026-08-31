import { Fragment, useEffect, useRef, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { ArrowReloadHorizontalIcon, BookOpen01Icon, Delete01Icon, File01Icon, GlobeIcon, InformationCircleIcon, LinkSquare01Icon, MagicWand01Icon, PencilEdit01Icon, PlusSignIcon, Settings02Icon } from '@/lib/icons';
import { useAllDocsCollections, useDocsDocuments, useDocsSpaces } from '@/hooks/queries';
import { useChatSettings, useAgentKnowledgeSources, useUpdateAgentKnowledgeSources, useReindexAgentKnowledgeSource, useSupportContentSources, useCreateSupportContentSource, useCreateSupportContentSourceFile, useUpdateSupportContentSource, useDeleteSupportContentSource, useReindexSupportContentSource } from '@/hooks/queries/useSupport';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { workspacesService } from '@/lib/services/workspacesService';
import {
  companyProductContextGenerateDisabledReason,
  companyProductContextGenerateLabel,
  shouldConfirmCompanyProductContextReplacement,
} from '@/lib/companyProductContext';
import { findWorkspaceWebsiteContentSource } from '@/lib/workspaceWebsiteSource';
import {
  buildKnowledgeSourceRows,
  knowledgeSourceCanBeManaged,
  knowledgeSourceTypeOptions,
  type KnowledgeSourceType,
} from '@/lib/knowledgeSourcesPresentation';
import { filterDocsArticlesByTitle, scheduleFocusDocsArticleSearchInput } from '@/lib/knowledgeDocsArticleSelector';
import {
  buildWebsiteSourcePayload,
  createWebsiteSourceDraft,
  draftFromWebsiteSource,
  suggestWebsiteSourceName,
  type WebsiteSourceDraft,
} from '@/lib/knowledgeSourceWebsiteForm';
import type { AgentKnowledgeSourceRequest } from '@/lib/pmTypes';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { UpgradeRequiredDialog } from '@/components/billing/UpgradeRequiredDialog';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Favicon } from '@/components/ui/favicon';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Textarea } from '@/components/ui/textarea';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { LINEAR_CARD_CLASS } from './settingsConstants';
import { getUpgradeRequiredReason, type UpgradeRequiredReason } from '@/lib/upgradeRequired';
import { cn } from '@/lib/utils';
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
  if (hasAnySource) return null;

  return (
    <div className="space-y-4">
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
  const [sourceChooserOpen, setSourceChooserOpen] = useState(false);
  const [sourceChooserStep, setSourceChooserStep] = useState<'choose' | 'website' | 'helpin_docs' | 'file'>('choose');
  const [websiteDraft, setWebsiteDraft] = useState<WebsiteSourceDraft>(() => createWebsiteSourceDraft());
  const [editingWebsiteSourceId, setEditingWebsiteSourceId] = useState<string | null>(null);
  const [websiteAdvancedOpen, setWebsiteAdvancedOpen] = useState(false);
  const [includePatternsEnabled, setIncludePatternsEnabled] = useState(false);
  const [excludePatternsEnabled, setExcludePatternsEnabled] = useState(false);
  const [docsScopeType, setDocsScopeType] = useState<'space' | 'collection' | 'article'>('space');
  const [selectedDocsSpaceId, setSelectedDocsSpaceId] = useState('');
  const [selectedDocsCollectionId, setSelectedDocsCollectionId] = useState('');
  const [selectedDocsDocumentId, setSelectedDocsDocumentId] = useState('');
  const [docsArticleSearch, setDocsArticleSearch] = useState('');
  const docsArticleSearchInputRef = useRef<HTMLInputElement>(null);
  const fileSourceInputRef = useRef<HTMLInputElement>(null);
  const [selectedFileSource, setSelectedFileSource] = useState<File | null>(null);
  const [fileSourceName, setFileSourceName] = useState('');
  const [upgradeDialogReason, setUpgradeDialogReason] = useState<UpgradeRequiredReason | null>(null);
  const savedCompanyProductContext = workspace?.company_product_context ?? workspace?.description ?? '';
  const { data: chatSettings, isLoading: chatSettingsLoading } = useChatSettings(workspaceId);
  const { data: docsSpaces = [], isLoading: docsSpacesLoading } = useDocsSpaces(workspaceId);
  const { data: allDocsCollections = [], isLoading: docsCollectionsLoading } = useAllDocsCollections(workspaceId);
  const { data: selectedSpaceDocuments = [], isLoading: docsDocumentsLoading } = useDocsDocuments(
    workspaceId,
    selectedDocsSpaceId ? { space_id: selectedDocsSpaceId, status: 'published' } : undefined,
  );
  const { data: contentSources = [] } = useSupportContentSources(workspaceId);
  const chatWidgetAgentId = chatSettings?.settings?.ai_agent_id ?? '';
  const { data: knowledgeSources = [], isLoading: knowledgeSourcesLoading } = useAgentKnowledgeSources(workspaceId, chatWidgetAgentId || undefined);
  const updateKnowledgeSources = useUpdateAgentKnowledgeSources(workspaceId);
  const reindexKnowledgeSource = useReindexAgentKnowledgeSource(workspaceId);
  const createWebsiteSource = useCreateSupportContentSource(workspaceId);
  const createFileSource = useCreateSupportContentSourceFile(workspaceId);
  const updateWebsiteSource = useUpdateSupportContentSource(workspaceId);
  const reindexContentSource = useReindexSupportContentSource(workspaceId);
  const deleteContentSource = useDeleteSupportContentSource(workspaceId);

  const docsKnowledgeSpaceIds = new Set(docsSpaces.map((space) => space.id));
  const docsKnowledgeSources = knowledgeSources.filter((source) => docsKnowledgeSpaceIds.has(source.space_id));
  const externalDocsSpaces = docsSpaces.filter((space) => space.type === 'external_capable');
  const internalDocsSpaces = docsSpaces.filter((space) => space.type === 'internal');
  const selectedSpaceCollections = allDocsCollections.filter((collection) => collection.space_id === selectedDocsSpaceId);
  const filteredDocsArticles = filterDocsArticlesByTitle(selectedSpaceDocuments, docsArticleSearch);
  const websiteContentSource = findWorkspaceWebsiteContentSource(workspace?.website_url, contentSources);
  const hasAnySource = docsSpaces.length > 0 || contentSources.length > 0 || Boolean(workspace?.website_url);
  const workspaceSlug = workspace?.slug ?? '';
  const shouldCollapseCompanyContext = companyDescription.length > 700 || companyDescription.split(/\r?\n/).length > 8;
  const generateCompanyContextDisabledReason = companyProductContextGenerateDisabledReason(workspace?.website_url);
  const sourceRows = buildKnowledgeSourceRows({
    websiteSources: contentSources,
    docsSpaces,
    docsKnowledgeSources,
  });

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

  const knowledgeSourceRequests = (): AgentKnowledgeSourceRequest[] => docsKnowledgeSources.map((source) => ({
    scope_type: source.scope_type ?? 'space',
    space_id: source.space_id,
    collection_id: source.collection_id ?? null,
    document_id: source.document_id ?? null,
  }));

  const scopedSourceKey = (source: AgentKnowledgeSourceRequest) => [
    source.scope_type,
    source.space_id,
    source.collection_id ?? '',
    source.document_id ?? '',
  ].join(':');

  const submitDocsSource = () => {
    if (!chatWidgetAgentId) {
      return;
    }

    const nextSource: AgentKnowledgeSourceRequest = {
      scope_type: docsScopeType,
      space_id: selectedDocsSpaceId,
      collection_id: docsScopeType === 'collection' ? selectedDocsCollectionId : null,
      document_id: docsScopeType === 'article' ? selectedDocsDocumentId : null,
    };
    if (!nextSource.space_id || (docsScopeType === 'collection' && !selectedDocsCollectionId) || (docsScopeType === 'article' && !selectedDocsDocumentId)) {
      return;
    }

    const existing = knowledgeSourceRequests();
    const seen = new Set(existing.map(scopedSourceKey));
    if (!seen.has(scopedSourceKey(nextSource))) {
      existing.push(nextSource);
    }
    updateKnowledgeSources.mutate({ agentId: chatWidgetAgentId, sources: existing });
    setSourceChooserOpen(false);
    setSourceChooserStep('choose');
  };

  const openWebsiteSourceModal = (sourceId?: string) => {
    const source = sourceId ? contentSources.find((item) => item.id === sourceId) : null;
    const nextDraft = source
      ? draftFromWebsiteSource(source)
      : createWebsiteSourceDraft({
        initialUrl: workspace?.website_url ?? '',
        workspaceName: workspace?.name ?? '',
      });
    setEditingWebsiteSourceId(source?.id ?? null);
    setWebsiteDraft(nextDraft);
    setIncludePatternsEnabled(Boolean(nextDraft.includePatternsText.trim()));
    setExcludePatternsEnabled(Boolean(nextDraft.excludePatternsText.trim()));
    setWebsiteAdvancedOpen(false);
    setSourceChooserStep('website');
    setSourceChooserOpen(true);
  };

  const updateWebsiteDraft = <Key extends keyof WebsiteSourceDraft>(key: Key, value: WebsiteSourceDraft[Key]) => {
    setWebsiteDraft((current) => ({ ...current, [key]: value }));
  };

  const handleWebsiteUrlChange = (value: string) => {
    setWebsiteDraft((current) => {
      const suggestedName = !current.name.trim() ? suggestWebsiteSourceName(value) : '';
      return {
        ...current,
        startUrl: value,
        name: suggestedName || current.name,
      };
    });
  };

  const handleIncludePatternsEnabledChange = (checked: boolean) => {
    setIncludePatternsEnabled(checked);
    if (!checked) {
      updateWebsiteDraft('includePatternsText', '');
    }
  };

  const handleExcludePatternsEnabledChange = (checked: boolean) => {
    setExcludePatternsEnabled(checked);
    if (!checked) {
      updateWebsiteDraft('excludePatternsText', '');
    }
  };

  const handleSubmitWebsiteSource = async () => {
    if (!websiteDraft.name.trim() || !websiteDraft.startUrl.trim()) {
      return;
    }

    const payload = buildWebsiteSourcePayload(websiteDraft);
    if (editingWebsiteSourceId) {
      await updateWebsiteSource.mutateAsync({ contentSourceId: editingWebsiteSourceId, payload });
      toast.success('Website source updated - syncing now');
    } else {
      await createWebsiteSource.mutateAsync(payload);
      toast.success('Website source added - syncing now');
    }

    setSourceChooserOpen(false);
    setSourceChooserStep('choose');
    setEditingWebsiteSourceId(null);
    setWebsiteDraft(createWebsiteSourceDraft());
    setIncludePatternsEnabled(false);
    setExcludePatternsEnabled(false);
  };

  const handleFileSourceChange = (file: File | null) => {
    setSelectedFileSource(file);
    if (file && !fileSourceName.trim()) {
      setFileSourceName(suggestFileSourceName(file.name));
    }
  };

  const handleSubmitFileSource = async () => {
    if (!selectedFileSource || !fileSourceName.trim()) {
      return;
    }
    await createFileSource.mutateAsync({ file: selectedFileSource, name: fileSourceName.trim() });
    toast.success('File source added - syncing now');
    setSourceChooserOpen(false);
    setSourceChooserStep('choose');
    setSelectedFileSource(null);
    setFileSourceName('');
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

  const handleChooseSourceType = (sourceType: KnowledgeSourceType) => {
    if (sourceType === 'website') {
      openWebsiteSourceModal();
      return;
    }
    if (sourceType === 'helpin_docs') {
      setDocsScopeType('space');
      setSelectedDocsSpaceId('');
      setSelectedDocsCollectionId('');
      setSelectedDocsDocumentId('');
      setDocsArticleSearch('');
      setSourceChooserStep('helpin_docs');
      return;
    }
    if (sourceType === 'file') {
      setSelectedFileSource(null);
      setFileSourceName('');
      setSourceChooserStep('file');
    }
  };

  const handleDocsArticleSelectorOpenChange = (open: boolean) => {
    if (open) {
      scheduleFocusDocsArticleSearchInput(() => docsArticleSearchInputRef.current);
    }
  };

  const handleManageSource = (source: ReturnType<typeof buildKnowledgeSourceRows>[number]) => {
    if (source.type === 'website') {
      openWebsiteSourceModal(source.sourceId);
      return;
    }
    setSourceChooserStep('helpin_docs');
    setSourceChooserOpen(true);
  };

  const handleSyncSource = (source: ReturnType<typeof buildKnowledgeSourceRows>[number]) => {
    if (source.type === 'website' || source.type === 'file') {
      reindexContentSource.mutate(source.sourceId);
      return;
    }
    if (chatWidgetAgentId) {
      reindexKnowledgeSource.mutate({ agentId: chatWidgetAgentId, spaceId: source.sourceId });
    }
  };

  const handleRemoveSource = (source: ReturnType<typeof buildKnowledgeSourceRows>[number]) => {
    if (source.type === 'website' || source.type === 'file') {
      if (window.confirm(`Remove ${source.name} from sources?`)) {
        deleteContentSource.mutate(source.sourceId);
      }
      return;
    }
    if (chatWidgetAgentId) {
      const removeKey = scopedSourceKey({
        scope_type: source.scopeType ?? 'space',
        space_id: source.spaceId ?? source.sourceId,
        collection_id: source.collectionId ?? null,
        document_id: source.documentId ?? null,
      });
      updateKnowledgeSources.mutate({
        agentId: chatWidgetAgentId,
        sources: knowledgeSourceRequests().filter((item) => scopedSourceKey(item) !== removeKey),
      });
    }
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
      workspace_id: workspace.id,
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

      <Card id="company-context" className={cn(LINEAR_CARD_CLASS, 'scroll-mt-6')}>
        <CardHeader>
          <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
            <div className="min-w-0 space-y-1">
              <div className="flex items-center gap-2">
                <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-muted text-muted-foreground">
                  <BookOpen01Icon className="h-4 w-4" />
                </div>
                <div>
                  <p className="text-sm font-medium">Company/Product Context</p>
                  <p className="text-xs text-muted-foreground">
                    Plain-text context used by Helpin AI agents for support answers, docs, planning, automation, and product-aware work.
                  </p>
                </div>
              </div>
            </div>
            <Tooltip>
              <TooltipTrigger asChild>
                <span className="inline-flex w-fit">
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
                </span>
              </TooltipTrigger>
              {generateCompanyContextDisabledReason ? (
                <TooltipContent side="bottom" align="end" className="max-w-64 text-xs leading-relaxed">
                  {generateCompanyContextDisabledReason}
                </TooltipContent>
              ) : null}
            </Tooltip>
          </div>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="space-y-2">
            <div>
              <Label htmlFor="company-product-context">Company/Product context</Label>
            </div>
            <div className={shouldCollapseCompanyContext && !companyContextExpanded ? 'relative max-h-44 overflow-hidden' : 'relative'}>
              <Textarea
                id="company-product-context"
                value={companyDescription}
                onChange={(event) => setCompanyDescription(event.target.value)}
                placeholder="Plain text about what your company or product does, who it serves, and what problems it solves."
                rows={companyContextExpanded ? 14 : 8}
                className={cn(
                  shouldCollapseCompanyContext && 'pb-10',
                  shouldCollapseCompanyContext && !companyContextExpanded && 'resize-none',
                )}
              />
              {shouldCollapseCompanyContext ? (
                <div
                  className={cn(
                    'absolute inset-x-px bottom-px flex h-16 items-end justify-center rounded-b-md pb-2',
                    !companyContextExpanded && 'bg-gradient-to-t from-background via-background/90 to-transparent',
                  )}
                >
                  <Button
                    type="button"
                    variant="secondary"
                    size="sm"
                    className="h-7 border border-border/70 px-2 text-xs shadow-sm"
                    onClick={() => setCompanyContextExpanded((current) => !current)}
                  >
                    {companyContextExpanded ? 'Show less' : 'Show more'}
                  </Button>
                </div>
              ) : null}
            </div>
          </div>
          <div className="flex justify-end">
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

      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader>
          <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
            <div className="min-w-0 space-y-1">
              <div className="flex items-center gap-2">
                <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-muted text-muted-foreground">
                  <BookOpen01Icon className="h-4 w-4" />
                </div>
                <div>
                  <CardTitle className="text-base">Sources</CardTitle>
                  <p className="text-xs text-muted-foreground">
                    Connect websites and Helpin docs that AI agents can use as searchable knowledge.
                  </p>
                </div>
              </div>
            </div>
            <Button
              type="button"
              size="sm"
              onClick={() => {
                setSourceChooserStep('choose');
                setEditingWebsiteSourceId(null);
                setSourceChooserOpen(true);
              }}
            >
              <PlusSignIcon className="h-4 w-4" />
              Add source
            </Button>
          </div>
        </CardHeader>
        <CardContent className="space-y-5">
          {sourceRows.length === 0 ? (
            <div className="rounded-xl border border-dashed border-border/80 bg-muted/20 px-4 py-8 text-center">
              <div className="mx-auto mb-3 flex h-10 w-10 items-center justify-center rounded-full bg-muted text-muted-foreground">
                <BookOpen01Icon className="h-5 w-5" />
              </div>
              <p className="text-sm font-medium">No sources connected yet</p>
              <p className="mx-auto mt-1 max-w-lg text-sm leading-6 text-muted-foreground">
                Add a website or select Helpin docs so AI agents can search trusted workspace knowledge.
              </p>
            </div>
          ) : null}

          {sourceRows.length > 0 ? (
            <div className="overflow-hidden rounded-xl border border-border/70">
              <Table className="min-w-[780px]">
                <TableHeader>
                  <TableRow className="hover:bg-transparent">
                    <TableHead className="w-[36%]">Source</TableHead>
                    <TableHead>Type</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>Indexed</TableHead>
                    <TableHead>Last sync</TableHead>
                    <TableHead className="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                {sourceRows.map((source) => {
                  const Icon = sourceTypeIcon(source.type);
                  const status = knowledgeSourceStatusMeta(source.status);
                  const isSyncing = source.status === 'queued' || source.status === 'running';
                  return (
                    <Fragment key={source.id}>
                      <TableRow>
                        <TableCell className="max-w-0">
                          <div className="flex min-w-0 items-center gap-3">
                            <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
                              {source.type === 'website' ? (
                                <Favicon
                                  url={source.description}
                                  name={source.name}
                                  size={32}
                                  className="h-5 w-5 rounded-md"
                                  fallbackClassName="text-[8px]"
                                />
                              ) : (
                                <Icon className="h-4 w-4" />
                              )}
                            </div>
                            <div className="min-w-0 flex-1">
                              <p className="truncate text-sm font-medium">{source.name}</p>
                              <p className="mt-1 truncate text-xs text-muted-foreground">{source.description}</p>
                            </div>
                          </div>
                        </TableCell>
                        <TableCell>
                          <Badge variant="outline">{source.typeLabel}</Badge>
                        </TableCell>
                        <TableCell>
                          <Badge variant="outline" className={status.className}>
                            {status.label}
                          </Badge>
                        </TableCell>
                        <TableCell className="text-sm text-muted-foreground">{source.countLabel}</TableCell>
                        <TableCell className="text-sm text-muted-foreground">
                          {source.lastSyncAt ? formatSourceSyncTime(source.lastSyncAt) : 'Not synced yet'}
                        </TableCell>
	                        <TableCell>
	                          <div className="flex justify-end gap-1">
	                          {knowledgeSourceCanBeManaged(source) ? (
	                            <Tooltip>
	                              <TooltipTrigger asChild>
	                                <Button
	                                  type="button"
	                                  size="icon"
	                                  variant="ghost"
	                                  className="h-8 w-8"
	                                  onClick={() => handleManageSource(source)}
	                                  aria-label="Edit source"
	                                >
	                                  <PencilEdit01Icon className="h-3.5 w-3.5" />
	                                </Button>
	                              </TooltipTrigger>
	                              <TooltipContent>Edit source</TooltipContent>
	                            </Tooltip>
	                          ) : null}
	                          <Tooltip>
	                            <TooltipTrigger asChild>
	                              <Button
	                                type="button"
	                                size="icon"
	                                variant="ghost"
	                                className="h-8 w-8"
	                                onClick={() => handleSyncSource(source)}
	                                disabled={
	                                  ((source.type === 'website' || source.type === 'file') && reindexContentSource.isPending)
	                                  || (source.type === 'helpin_docs' && (!chatWidgetAgentId || reindexKnowledgeSource.isPending))
	                                }
	                                aria-label="Sync now"
	                              >
	                                <ArrowReloadHorizontalIcon className={cn(
	                                  'h-3.5 w-3.5',
	                                  (((source.type === 'website' || source.type === 'file') && reindexContentSource.isPending)
	                                    || (source.type === 'helpin_docs' && reindexKnowledgeSource.isPending)) && 'animate-spin',
	                                )} />
	                              </Button>
	                            </TooltipTrigger>
	                            <TooltipContent>Sync now</TooltipContent>
	                          </Tooltip>
	                          <Tooltip>
	                            <TooltipTrigger asChild>
	                              <Button
	                                type="button"
	                                size="icon"
	                                variant="ghost"
	                                className="h-8 w-8 text-muted-foreground hover:text-destructive"
	                                onClick={() => handleRemoveSource(source)}
	                                disabled={
	                                  ((source.type === 'website' || source.type === 'file') && deleteContentSource.isPending)
	                                  || (source.type === 'helpin_docs' && (!chatWidgetAgentId || updateKnowledgeSources.isPending))
	                                }
	                                aria-label="Delete source"
	                              >
	                                <Delete01Icon className="h-3.5 w-3.5" />
	                              </Button>
	                            </TooltipTrigger>
	                            <TooltipContent>Delete source</TooltipContent>
	                          </Tooltip>
	                          </div>
	                        </TableCell>
                      </TableRow>
                      {isSyncing ? (
                        <TableRow key={`${source.id}:progress`} className="hover:bg-transparent">
                          <TableCell colSpan={6} className="px-4 py-2">
                            <div className="flex items-center justify-between text-xs text-muted-foreground">
                              <span>{source.status === 'queued' ? 'Queued for sync' : 'Syncing source'}</span>
                              <span className="tabular-nums">{Math.max(0, Math.min(source.progress, 100))}%</span>
                            </div>
                            <div className="mt-1 h-1.5 overflow-hidden rounded-full bg-muted">
                              <div
                                className="h-full rounded-full bg-primary transition-all"
                                style={{ width: `${Math.max(0, Math.min(source.progress, 100))}%` }}
                              />
                            </div>
                          </TableCell>
                        </TableRow>
                      ) : null}
                      {source.error ? (
                        <TableRow key={`${source.id}:error`} className="hover:bg-transparent">
                          <TableCell colSpan={6} className="px-4 py-2 text-xs text-destructive">
                            {source.error}
                          </TableCell>
                        </TableRow>
                      ) : null}
                    </Fragment>
                  );
                })}
                </TableBody>
              </Table>
            </div>
          ) : null}

          {workspace?.website_url && !websiteContentSource ? (
            <div className="rounded-xl border border-border/70 bg-background px-4 py-3">
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
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  onClick={() => openWebsiteSourceModal()}
                  disabled={createWebsiteSource.isPending}
                >
                  Add as source
                </Button>
              </div>
            </div>
          ) : null}

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
      <Dialog
        open={sourceChooserOpen}
        onOpenChange={(open) => {
          setSourceChooserOpen(open);
          if (!open) {
            setSourceChooserStep('choose');
            setEditingWebsiteSourceId(null);
            setWebsiteDraft(createWebsiteSourceDraft());
            setWebsiteAdvancedOpen(false);
            setIncludePatternsEnabled(false);
            setExcludePatternsEnabled(false);
            setSelectedFileSource(null);
            setFileSourceName('');
          }
        }}
      >
        <DialogContent className="sm:max-w-2xl">
          <DialogHeader>
            <DialogTitle>
              {sourceChooserStep === 'website'
                ? (editingWebsiteSourceId ? 'Manage website source' : 'Add website source')
                : sourceChooserStep === 'helpin_docs' ? 'Add Helpin docs'
                  : sourceChooserStep === 'file' ? 'Add PDF or file'
                    : 'Add source'}
            </DialogTitle>
            <DialogDescription>
              {sourceChooserStep === 'website'
                ? 'Configure the website pages Helpin AI agents can search.'
                : sourceChooserStep === 'helpin_docs'
                  ? 'Select Help Center spaces that Helpin AI agents can search.'
                  : sourceChooserStep === 'file'
                    ? 'Upload a PDF, Markdown, or text file for Helpin AI agents to search.'
                    : 'Choose where Helpin should pull searchable knowledge from.'}
            </DialogDescription>
          </DialogHeader>
          {sourceChooserStep === 'choose' ? (
            <div className="grid gap-3">
              {knowledgeSourceTypeOptions.map((option) => {
                const Icon = sourceTypeIcon(option.id);
                return (
                  <button
                    key={option.id}
                    type="button"
                    className={cn(
                      'flex w-full items-start gap-3 rounded-lg border border-border/70 bg-background px-4 py-3 text-left transition-colors hover:bg-muted/40',
                      !option.available && 'cursor-not-allowed opacity-60 hover:bg-background',
                    )}
                    disabled={!option.available}
                    onClick={() => handleChooseSourceType(option.id)}
                  >
                    <span className="mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
                      <Icon className="h-4 w-4" />
                    </span>
                    <span className="min-w-0 flex-1">
                      <span className="flex items-center gap-2 text-sm font-medium">
                        {option.label}
                        {!option.available ? <Badge variant="secondary">Coming soon</Badge> : null}
                      </span>
                      <span className="mt-1 block text-xs leading-5 text-muted-foreground">
                        {option.description}
                      </span>
                    </span>
                  </button>
                );
              })}
            </div>
          ) : sourceChooserStep === 'website' ? (
            <div className="space-y-4">
              <div className="grid gap-3 sm:grid-cols-2">
                <div className="space-y-2 sm:col-span-2">
                  <Label htmlFor="knowledge-website-url">Website URL</Label>
                  <Input
                    id="knowledge-website-url"
                    value={websiteDraft.startUrl}
                    onChange={(event) => handleWebsiteUrlChange(event.target.value)}
                    placeholder="https://example.com"
                    autoFocus
                  />
                  <p className="text-xs leading-5 text-muted-foreground">
                    Subdomains are not included. Add docs.example.com or help.example.com as separate sources.
                  </p>
                </div>
                <div className="space-y-2 sm:col-span-2">
                  <Label htmlFor="knowledge-website-name">Source name</Label>
                  <Input
                    id="knowledge-website-name"
                    value={websiteDraft.name}
                    onChange={(event) => updateWebsiteDraft('name', event.target.value)}
                    placeholder="Marketing website"
                  />
                </div>
                <div className="space-y-3 sm:col-span-2">
                  <div className="space-y-2 rounded-lg border border-border/70 px-3 py-3">
                    <div className="flex items-center gap-2">
                      <Checkbox
                        id="knowledge-include-patterns-enabled"
                        checked={includePatternsEnabled}
                        onCheckedChange={(checked) => handleIncludePatternsEnabledChange(checked === true)}
                      />
                      <Label htmlFor="knowledge-include-patterns-enabled" className="text-sm font-medium">
                        Only include specific pages
                      </Label>
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <button
                            type="button"
                            className="inline-flex h-5 w-5 items-center justify-center rounded-full text-muted-foreground hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/30"
                            aria-label="Only include specific pages help"
                          >
                            <InformationCircleIcon className="h-3.5 w-3.5" />
                          </button>
                        </TooltipTrigger>
                        <TooltipContent side="top" align="start" className="z-[70] max-w-72 text-xs leading-relaxed">
                          Use this when Helpin should learn from only specific areas of the site, like docs, help, or pricing pages. Leave unchecked to allow the normal crawl.
                        </TooltipContent>
                      </Tooltip>
                    </div>
                    {includePatternsEnabled ? (
                      <Textarea
                        id="knowledge-include-patterns"
                        value={websiteDraft.includePatternsText}
                        onChange={(event) => updateWebsiteDraft('includePatternsText', event.target.value)}
                        placeholder={'/docs/*\n/help/*\n/pricing'}
                        rows={3}
                      />
                    ) : null}
                  </div>

                  <div className="space-y-2 rounded-lg border border-border/70 px-3 py-3">
                    <div className="flex items-center gap-2">
                      <Checkbox
                        id="knowledge-exclude-patterns-enabled"
                        checked={excludePatternsEnabled}
                        onCheckedChange={(checked) => handleExcludePatternsEnabledChange(checked === true)}
                      />
                      <Label htmlFor="knowledge-exclude-patterns-enabled" className="text-sm font-medium">
                        Skip specific pages
                      </Label>
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <button
                            type="button"
                            className="inline-flex h-5 w-5 items-center justify-center rounded-full text-muted-foreground hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/30"
                            aria-label="Skip specific pages help"
                          >
                            <InformationCircleIcon className="h-3.5 w-3.5" />
                          </button>
                        </TooltipTrigger>
                        <TooltipContent side="top" align="start" className="z-[70] max-w-72 text-xs leading-relaxed">
                          Use this to keep noisy, private, or irrelevant pages out of agent knowledge. Excluded pages are skipped even if they are found during the crawl.
                        </TooltipContent>
                      </Tooltip>
                    </div>
                    {excludePatternsEnabled ? (
                      <Textarea
                        id="knowledge-exclude-patterns"
                        value={websiteDraft.excludePatternsText}
                        onChange={(event) => updateWebsiteDraft('excludePatternsText', event.target.value)}
                        placeholder={'/blog/*\n/careers/*\n/login'}
                        rows={3}
                      />
                    ) : null}
                  </div>
                </div>
              </div>

              <div className="rounded-lg border border-border/70">
                <button
                  type="button"
                  className="flex w-full items-center justify-between px-4 py-3 text-left text-sm font-medium"
                  onClick={() => setWebsiteAdvancedOpen((current) => !current)}
                >
                  Advanced settings
                  <span className="text-xs text-muted-foreground">{websiteAdvancedOpen ? 'Hide' : 'Show'}</span>
                </button>
                {websiteAdvancedOpen ? (
                  <div className="grid gap-4 border-t border-border/70 px-4 py-4 sm:grid-cols-2">
                    <div className="space-y-2">
                      <Label htmlFor="knowledge-crawl-source">Crawl source</Label>
                      <Select
                        value={websiteDraft.crawlSource}
                        onValueChange={(value) => updateWebsiteDraft('crawlSource', value as WebsiteSourceDraft['crawlSource'])}
                      >
                        <SelectTrigger id="knowledge-crawl-source">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="all">All discoverable pages</SelectItem>
                          <SelectItem value="sitemaps">Sitemaps only</SelectItem>
                          <SelectItem value="links">Links only</SelectItem>
                        </SelectContent>
                      </Select>
                    </div>
                    <div className="grid grid-cols-2 gap-3">
                      <div className="space-y-2">
                        <Label htmlFor="knowledge-crawl-limit">Page limit</Label>
                        <Input
                          id="knowledge-crawl-limit"
                          type="number"
                          min={1}
                          value={websiteDraft.crawlLimit}
                          onChange={(event) => updateWebsiteDraft('crawlLimit', event.target.value)}
                        />
                      </div>
                      <div className="space-y-2">
                        <Label htmlFor="knowledge-crawl-depth">Depth</Label>
                        <Input
                          id="knowledge-crawl-depth"
                          type="number"
                          min={1}
                          value={websiteDraft.crawlDepth}
                          onChange={(event) => updateWebsiteDraft('crawlDepth', event.target.value)}
                        />
                      </div>
                    </div>
                  </div>
                ) : null}
              </div>

              <div className="flex justify-between">
                <Button type="button" variant="ghost" size="sm" onClick={() => setSourceChooserStep('choose')}>
                  Back
                </Button>
                <Button
                  type="button"
                  size="sm"
                  onClick={() => void handleSubmitWebsiteSource()}
                  disabled={
                    !websiteDraft.name.trim()
                    || !websiteDraft.startUrl.trim()
                    || createWebsiteSource.isPending
                    || updateWebsiteSource.isPending
                  }
                >
                  {createWebsiteSource.isPending || updateWebsiteSource.isPending
                    ? 'Saving...'
                    : editingWebsiteSourceId ? 'Save source' : 'Add source'}
                </Button>
              </div>
            </div>
          ) : sourceChooserStep === 'file' ? (
            <div className="space-y-4">
	              <div className="space-y-2">
	                <Label htmlFor="knowledge-file-source">File</Label>
	                {selectedFileSource ? (
	                  <p className="truncate text-xs text-muted-foreground">
	                    {selectedFileSource.name} - {formatFileSize(selectedFileSource.size)}
	                  </p>
	                ) : null}
	                <Input
	                  ref={fileSourceInputRef}
	                  id="knowledge-file-source"
	                  type="file"
	                  className="sr-only"
	                  accept=".pdf,.md,.markdown,.txt,.csv,application/pdf,text/markdown,text/plain,text/csv"
	                  onChange={(event) => handleFileSourceChange(event.target.files?.[0] ?? null)}
	                />
	                <Button
	                  type="button"
	                  variant="outline"
	                  size="sm"
	                  onClick={() => fileSourceInputRef.current?.click()}
	                  autoFocus
	                >
	                  <File01Icon className="h-4 w-4" />
	                  {selectedFileSource ? 'Change file' : 'Choose file'}
	                </Button>
	                <p className="text-xs leading-5 text-muted-foreground">
	                  PDF, Markdown, text, and CSV files are supported.
	                </p>
	              </div>
              <div className="space-y-2">
                <Label htmlFor="knowledge-file-source-name">Source name</Label>
                <Input
                  id="knowledge-file-source-name"
                  value={fileSourceName}
                  onChange={(event) => setFileSourceName(event.target.value)}
                  placeholder="Product guide"
                />
              </div>
	              <div className="flex justify-between">
                <Button type="button" variant="ghost" size="sm" onClick={() => setSourceChooserStep('choose')}>
                  Back
                </Button>
                <Button
                  type="button"
                  size="sm"
                  onClick={() => void handleSubmitFileSource()}
                  disabled={!selectedFileSource || !fileSourceName.trim() || createFileSource.isPending}
                >
                  {createFileSource.isPending ? 'Uploading...' : 'Add source'}
                </Button>
              </div>
            </div>
          ) : (
            <div className="space-y-4">
              {docsSpacesLoading || docsCollectionsLoading || docsDocumentsLoading || knowledgeSourcesLoading ? (
                <div className="space-y-3">
                  <Skeleton className="h-14 rounded-lg" />
                  <Skeleton className="h-14 rounded-lg" />
                </div>
              ) : docsSpaces.length === 0 ? (
                <div className="rounded-lg border border-dashed border-border/80 bg-muted/20 px-4 py-8 text-center">
                  <p className="text-sm font-medium">No docs spaces yet</p>
                  <p className="mx-auto mt-1 max-w-sm text-xs leading-5 text-muted-foreground">
                    Create an internal or external docs space first, then return here to add it as a source.
                  </p>
                  <Button type="button" variant="outline" size="sm" className="mt-4" onClick={openDocs}>
                    <PlusSignIcon className="h-4 w-4" />
                    Open docs
                  </Button>
                </div>
              ) : (
                <div className="space-y-4">
                  <div className="space-y-2">
                    <Label>Source scope</Label>
                    <div className="grid grid-cols-3 gap-2">
                      {(['space', 'collection', 'article'] as const).map((scope) => (
                        <Button
                          key={scope}
                          type="button"
                          variant={docsScopeType === scope ? 'default' : 'outline'}
                          size="sm"
                          onClick={() => {
                            setDocsScopeType(scope);
                            setSelectedDocsCollectionId('');
                            setSelectedDocsDocumentId('');
                            setDocsArticleSearch('');
                          }}
                        >
                          {scope === 'space' ? 'Space' : scope === 'collection' ? 'Collection' : 'Article'}
                        </Button>
                      ))}
                    </div>
                  </div>

                  <div className="space-y-2">
                    <Label>Docs space</Label>
                    <Select
                      value={selectedDocsSpaceId}
                      onValueChange={(value) => {
                        setSelectedDocsSpaceId(value);
                        setSelectedDocsCollectionId('');
                        setSelectedDocsDocumentId('');
                        setDocsArticleSearch('');
                      }}
                    >
                      <SelectTrigger>
                        <SelectValue placeholder="Select a docs space" />
                      </SelectTrigger>
                      <SelectContent>
                        {externalDocsSpaces.length > 0 ? (
                          <>
                            <div className="px-2 py-1.5 text-xs font-medium text-muted-foreground">External docs</div>
                            {externalDocsSpaces.map((space) => (
                              <SelectItem key={space.id} value={space.id}>{space.name}</SelectItem>
                            ))}
                          </>
                        ) : null}
                        {internalDocsSpaces.length > 0 ? (
                          <>
                            <div className="px-2 py-1.5 text-xs font-medium text-muted-foreground">Internal docs</div>
                            {internalDocsSpaces.map((space) => (
                              <SelectItem key={space.id} value={space.id}>{space.name}</SelectItem>
                            ))}
                          </>
                        ) : null}
                      </SelectContent>
                    </Select>
                  </div>

                  {docsScopeType === 'collection' ? (
                    <div className="space-y-2">
                      <Label>Docs collection</Label>
                      <Select value={selectedDocsCollectionId} onValueChange={setSelectedDocsCollectionId} disabled={!selectedDocsSpaceId || selectedSpaceCollections.length === 0}>
                        <SelectTrigger>
                          <SelectValue placeholder={selectedDocsSpaceId ? 'Select a collection' : 'Select a space first'} />
                        </SelectTrigger>
                        <SelectContent>
                          {selectedSpaceCollections.map((collection) => (
                            <SelectItem key={collection.id} value={collection.id}>
                              {'  '.repeat(collection.depth)}{collection.name}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>
                  ) : null}

                  {docsScopeType === 'article' ? (
                    <div className="space-y-2">
                      <Label>Docs article</Label>
                      <Select
                        value={selectedDocsDocumentId}
                        onValueChange={setSelectedDocsDocumentId}
                        onOpenChange={handleDocsArticleSelectorOpenChange}
                        disabled={!selectedDocsSpaceId || selectedSpaceDocuments.length === 0}
                      >
                        <SelectTrigger>
                          <SelectValue placeholder={selectedDocsSpaceId ? (selectedSpaceDocuments.length === 0 ? 'No published articles' : 'Select an article') : 'Select a space first'} />
                        </SelectTrigger>
                        <SelectContent>
                          <div className="sticky top-0 z-10 border-b bg-popover p-2">
                            <Input
                              ref={docsArticleSearchInputRef}
                              value={docsArticleSearch}
                              onChange={(event) => {
                                setDocsArticleSearch(event.target.value);
                                setSelectedDocsDocumentId('');
                              }}
                              onKeyDown={(event) => event.stopPropagation()}
                              onPointerDown={(event) => event.stopPropagation()}
                              placeholder="Search articles"
                              className="h-8"
                            />
                          </div>
                          {filteredDocsArticles.length > 0 ? (
                            filteredDocsArticles.map((document) => (
                              <SelectItem key={document.id} value={document.id}>{document.title}</SelectItem>
                            ))
                          ) : (
                            <div className="px-3 py-6 text-center text-sm text-muted-foreground">No matching articles</div>
                          )}
                        </SelectContent>
                      </Select>
                    </div>
                  ) : null}
                </div>
              )}
              <div className="flex justify-between">
                <Button type="button" variant="ghost" size="sm" onClick={() => setSourceChooserStep('choose')}>
                  Back
                </Button>
                <Button
                  type="button"
                  size="sm"
                  onClick={submitDocsSource}
                  disabled={
                    !chatWidgetAgentId
                    || updateKnowledgeSources.isPending
                    || !selectedDocsSpaceId
                    || (docsScopeType === 'collection' && !selectedDocsCollectionId)
                    || (docsScopeType === 'article' && !selectedDocsDocumentId)
                  }
                >
                  {updateKnowledgeSources.isPending ? 'Adding...' : 'Add source'}
                </Button>
              </div>
            </div>
          )}
        </DialogContent>
      </Dialog>
    </>
  );
}

function sourceTypeIcon(sourceType: KnowledgeSourceType) {
  switch (sourceType) {
    case 'website':
      return GlobeIcon;
    case 'helpin_docs':
      return BookOpen01Icon;
    case 'file':
      return File01Icon;
    default:
      return BookOpen01Icon;
  }
}

function knowledgeSourceStatusMeta(status: string) {
  switch (status) {
    case 'queued':
      return { label: 'Queued', className: 'border-amber-500/40 bg-amber-500/10 text-amber-700' };
    case 'running':
      return { label: 'Syncing', className: 'border-sky-500/40 bg-sky-500/10 text-sky-700' };
    case 'ready':
      return { label: 'Ready', className: 'border-emerald-500/40 bg-emerald-500/10 text-emerald-700' };
    case 'failed':
      return { label: 'Failed', className: 'border-destructive/40 bg-destructive/10 text-destructive' };
    case 'stale':
      return { label: 'Outdated', className: 'border-orange-500/40 bg-orange-500/10 text-orange-700' };
    case 'disabled':
      return { label: 'Disabled', className: 'border-muted-foreground/30 bg-muted text-muted-foreground' };
    default:
      return { label: 'Unknown', className: 'border-border/70 bg-muted text-muted-foreground' };
  }
}

function formatSourceSyncTime(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return 'Sync time unavailable';
  }
  return `Last synced ${date.toLocaleDateString(undefined, { month: 'short', day: 'numeric' })}`;
}

function suggestFileSourceName(fileName: string) {
  const trimmed = fileName.trim();
  if (!trimmed) {
    return 'Uploaded file';
  }
  const withoutExtension = trimmed.replace(/\.[^.]+$/, '');
  return withoutExtension || trimmed;
}

function formatFileSize(bytes: number) {
  if (!Number.isFinite(bytes) || bytes <= 0) {
    return '0B';
  }
  if (bytes < 1024) {
    return `${bytes}B`;
  }
  if (bytes < 1024 * 1024) {
    return `${Math.round(bytes / 1024)}KB`;
  }
  return `${(bytes / (1024 * 1024)).toFixed(bytes >= 10 * 1024 * 1024 ? 0 : 1)}MB`;
}
