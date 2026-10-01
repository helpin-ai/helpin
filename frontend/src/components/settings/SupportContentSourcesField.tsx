import { useEffect, useState } from 'react';
import {
  ArrowRight01Icon,
  ViewIcon,
  LinkSquare01Icon,
  File01Icon,
  GlobeIcon,
  InformationCircleIcon,
  Link01Icon,
  Loading03Icon,
  PencilEdit01Icon,
  PlusSignIcon,
  ArrowReloadHorizontalIcon,
  Delete01Icon,
} from '@/lib/icons';
import { toast } from 'sonner';
import {
  useAgentContentSources,
  useCreateSupportContentSource,
  useDeleteSupportContentSource,
  useReindexSupportContentSource,
  useSupportContentSources,
  useSupportContentSourcePages,
  useUpdateAgentContentSources,
  useUpdateSupportContentSource,
} from '@/hooks/queries/useSupport';
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from '@/components/ui/alert-dialog';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Favicon } from '@/components/ui/favicon';
import { Input } from '@/components/ui/input';
import { QuietSearchInput } from '@/components/design-system/quiet';
import { Label } from '@/components/ui/label';
import { Progress } from '@/components/ui/progress';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Sheet, SheetContent, SheetDescription, SheetFooter, SheetHeader, SheetTitle } from '@/components/ui/sheet';
import { Skeleton } from '@/components/ui/skeleton';
import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { cn } from '@/lib/utils';
import type { CreateSupportContentSourceRequest, SupportContentSource, UpdateSupportContentSourceRequest } from '@/lib/pmTypes';
import { CRAWL_SOURCE_OPTIONS, STATUS_META, STEP_ORDER, type WizardStep } from './support-content-sources/constants';
import { PageContentPreview } from './support-content-sources/PageContentPreview';
import { ReviewRow } from './support-content-sources/ReviewRow';

type ContentSourceDraft = {
  name: string;
  startUrl: string;
  crawlLimit: string;
  crawlDepth: string;
  crawlSource: 'all' | 'sitemaps' | 'links';
  formats: string[];
  render: boolean;
  includeExternalLinks: boolean;
  includeSubdomains: boolean;
  includePatternsText: string;
  excludePatternsText: string;
  maxAgeSeconds: string;
  jsonPrompt: string;
  jsonResponseFormatText: string;
};

export function SupportContentSourcesField({
  workspaceId,
  agentId,
  disabled = false,
  embedded = false,
  createRequestToken = 0,
  editRequestToken = 0,
  editSourceId = '',
  hideEmptyState = false,
  renderList = true,
}: {
  workspaceId: string;
  agentId?: string;
  disabled?: boolean;
  embedded?: boolean;
  createRequestToken?: number;
  editRequestToken?: number;
  editSourceId?: string;
  hideEmptyState?: boolean;
  renderList?: boolean;
}) {
  const { data: sources = [], isLoading } = useSupportContentSources(workspaceId);
  const { data: selectedSourceIds = [], isLoading: selectedLoading } = useAgentContentSources(workspaceId, agentId);
  const createSource = useCreateSupportContentSource(workspaceId);
  const updateSource = useUpdateSupportContentSource(workspaceId);
  const deleteSource = useDeleteSupportContentSource(workspaceId);
  const reindexSource = useReindexSupportContentSource(workspaceId);
  const updateAgentSources = useUpdateAgentContentSources(workspaceId);

  const [wizardOpen, setWizardOpen] = useState(false);
  const [wizardStep, setWizardStep] = useState<WizardStep>('connect');
  const [editingSource, setEditingSource] = useState<SupportContentSource | null>(null);
  const [draft, setDraft] = useState<ContentSourceDraft>(createDefaultDraft());
  const [deleteTarget, setDeleteTarget] = useState<SupportContentSource | null>(null);
  const [viewingSource, setViewingSource] = useState<SupportContentSource | null>(null);
  const [advancedOpen, setAdvancedOpen] = useState(false);

  const isMutating = disabled
    || createSource.isPending
    || updateSource.isPending
    || deleteSource.isPending
    || reindexSource.isPending
    || updateAgentSources.isPending;

  const selectedSet = new Set(selectedSourceIds);
  const hasAgent = !!agentId;
  const willUseInCurrentAgent = !hasAgent ? null : (!editingSource || selectedSet.has(editingSource.id));

  const openCreateWizard = () => {
    setEditingSource(null);
    setDraft(createDefaultDraft());
    setWizardStep('connect');
    setAdvancedOpen(false);
    setWizardOpen(true);
  };

  useEffect(() => {
    if (createRequestToken > 0) {
      openCreateWizard();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [createRequestToken]);

  useEffect(() => {
    if (editRequestToken <= 0 || !editSourceId) {
      return;
    }
    const source = sources.find((item) => item.id === editSourceId);
    if (source) {
      openEditWizard(source);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [editRequestToken, editSourceId, sources]);

  const openEditWizard = (source: SupportContentSource) => {
    setEditingSource(source);
    setDraft(draftFromSource(source));
    setWizardStep('connect');
    setAdvancedOpen(false);
    setWizardOpen(true);
  };

  const setPatternText = (field: 'includePatternsText' | 'excludePatternsText', value: string) => {
    setDraft((current) => ({ ...current, [field]: value }));
  };

  const handleUrlChange = (value: string) => {
    setDraft((current) => {
      const next: ContentSourceDraft = { ...current, startUrl: value };
      if (!current.name.trim()) {
        const suggested = suggestNameFromURL(value);
        if (suggested) {
          next.name = suggested;
        }
      }
      return next;
    });
  };

  const toggleSelected = async (sourceId: string, checked: boolean) => {
    if (!agentId) {
      return;
    }
    const next = checked
      ? uniqueStrings([...selectedSourceIds, sourceId])
      : selectedSourceIds.filter((id) => id !== sourceId);
    await updateAgentSources.mutateAsync({ agentId, contentSourceIds: next });
  };

  const handleReindex = async (sourceId: string) => {
    await reindexSource.mutateAsync(sourceId);
    toast.success('Re-sync started');
  };

  const handleDelete = async () => {
    if (!deleteTarget) {
      return;
    }
    await deleteSource.mutateAsync(deleteTarget.id);
    toast.success('Website source removed');
    setDeleteTarget(null);
  };

  const submitWizard = async () => {
    if (!validateStep(draft, wizardStep)) {
      return;
    }

    const payload = buildPayload(draft);
    let sourceId = editingSource?.id ?? '';

    if (editingSource) {
      const updated = await updateSource.mutateAsync({
        contentSourceId: editingSource.id,
        payload: payload as UpdateSupportContentSourceRequest,
      });
      sourceId = updated.id;
      toast.success('Website source updated — re-syncing now');
    } else {
      const created = await createSource.mutateAsync(payload);
      sourceId = created.id;
      toast.success('Website added — syncing now');
    }

    if (!editingSource && agentId) {
      const nextSelected = uniqueStrings([...selectedSourceIds, sourceId]);
      await updateAgentSources.mutateAsync({ agentId, contentSourceIds: nextSelected });
    }

    setWizardOpen(false);
    setEditingSource(null);
    setDraft(createDefaultDraft());
    setWizardStep('connect');
  };

  const handleNext = () => {
    if (!validateStep(draft, wizardStep)) {
      return;
    }
    const currentIndex = STEP_ORDER.indexOf(wizardStep);
    if (currentIndex < STEP_ORDER.length - 1) {
      setWizardStep(STEP_ORDER[currentIndex + 1]);
    }
  };

  const handlePrevious = () => {
    const currentIndex = STEP_ORDER.indexOf(wizardStep);
    if (currentIndex > 0) {
      setWizardStep(STEP_ORDER[currentIndex - 1]);
    }
  };

  if (isLoading || selectedLoading) {
    return (
      <div className={cn('space-y-3', !embedded && 'rounded-xl border border-border/70 bg-card p-4')}>
        <Skeleton className="h-16 rounded-lg" />
        <Skeleton className="h-24 rounded-lg" />
      </div>
    );
  }

  return (
    <>
      <div className={cn(embedded ? 'space-y-3' : 'overflow-hidden rounded-xl border border-border/70 bg-card', !renderList && 'hidden')}>
        {!embedded && (
        <div className="flex flex-col gap-3 border-b border-border/70 px-4 py-4 sm:flex-row sm:items-start sm:justify-between">
          <div className="min-w-0 space-y-1">
            <div className="flex items-center gap-2">
              <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-muted text-muted-foreground">
                <GlobeIcon className="h-4 w-4" />
              </div>
              <div>
                <p className="text-sm font-medium">Website Content</p>
                <p className="text-xs text-muted-foreground">
                  Import content from websites so your support AI can use it to answer questions.
                </p>
              </div>
            </div>
          </div>

          <Button type="button" size="sm" onClick={openCreateWizard} disabled={isMutating}>
            <PlusSignIcon className="h-4 w-4" />
            Add website
          </Button>
        </div>
        )}

        <div className={embedded ? 'space-y-3' : 'space-y-3 p-4'}>
          {!hasAgent && (!hideEmptyState || sources.length > 0) && (
            <div className="rounded-lg border border-dashed border-border/80 bg-muted/20 px-4 py-3 text-sm text-muted-foreground">
              Add, edit, and sync website content now. Attach these sources to support AI after you select a support agent in AI Assistant.
            </div>
          )}
          {sources.length === 0 ? (
            hideEmptyState ? null : (
            <div className="rounded-xl border border-dashed border-border/80 bg-muted/20 px-4 py-10 text-center">
              <div className="mx-auto mb-3 flex h-10 w-10 items-center justify-center rounded-full bg-muted text-muted-foreground">
                <GlobeIcon className="h-5 w-5" />
              </div>
              <p className="text-sm font-medium">No websites added yet</p>
              <p className="mx-auto mt-1 max-w-lg text-sm text-muted-foreground">
                Add a website to give your support AI access to its content when answering customer questions.
              </p>
            </div>
            )
          ) : (
            sources.map((source) => {
              const selected = hasAgent && selectedSet.has(source.id);
              const statusMeta = STATUS_META[source.sync_status];
              const StatusIcon = statusMeta?.icon ?? GlobeIcon;
              const progress = Math.max(0, Math.min(source.sync_progress ?? 0, 100));
              const indexedPages = source.indexed_pages ?? 0;
              const indexedChunks = source.indexed_chunks ?? 0;
              const hasIndexedContent = indexedPages > 0 || indexedChunks > 0;
              const syncMessage = source.sync_status === 'queued'
                ? (hasIndexedContent ? 'Queued for refresh. Existing indexed content remains available.' : 'Queued - sync will start shortly.')
                : (hasIndexedContent ? 'Refreshing website content. Existing indexed pages remain available.' : 'Syncing...');

              return (
                <div key={source.id} className={cn(
                  'rounded-xl border px-4 py-3 transition-colors',
                  selected ? 'border-border bg-muted/25' : 'border-border/70 bg-background',
                )}>
                  <div className="flex items-start gap-3">
                    {hasAgent ? (
                      <Checkbox
                        checked={selected}
                        disabled={isMutating}
                        onCheckedChange={(value) => void toggleSelected(source.id, Boolean(value))}
                        className="mt-0.5"
                      />
                    ) : (
                      <div className="mt-0.5 h-4 w-4 shrink-0 rounded-sm border border-border/70 bg-muted/30" />
                    )}

                    <div className="min-w-0 flex-1">
                      {/* Row 1: name + status badge + actions */}
                      <div className="flex items-center justify-between gap-2">
                        <div className="flex min-w-0 items-center gap-2">
                          <Favicon
                            url={source.start_url}
                            name={source.name}
                            size={32}
                            className="h-5 w-5 rounded-md"
                            fallbackClassName="text-[8px]"
                          />
                          <span className="truncate text-sm font-medium">{source.name}</span>
                          {statusMeta && (
                            <Badge variant="outline" className={cn('shrink-0', statusMeta.className)}>
                              <StatusIcon className={cn('mr-1 h-3 w-3', (source.sync_status === 'queued' || source.sync_status === 'running') && 'animate-spin')} />
                              {statusMeta.label}
                            </Badge>
                          )}
                        </div>
                        <div className="flex shrink-0 items-center gap-1">
                          <Button type="button" size="icon" variant="ghost" className="h-7 w-7" onClick={() => setViewingSource(source)} disabled={isMutating} title="View pages">
                            <File01Icon className="h-3.5 w-3.5" />
                          </Button>
                          <Button type="button" size="icon" variant="ghost" className="h-7 w-7" onClick={() => openEditWizard(source)} disabled={isMutating} title="Edit">
                            <PencilEdit01Icon className="h-3.5 w-3.5" />
                          </Button>
                          <Button type="button" size="icon" variant="ghost" className="h-7 w-7" onClick={() => void handleReindex(source.id)} disabled={isMutating} title="Re-sync">
                            <ArrowReloadHorizontalIcon className={cn('h-3.5 w-3.5', reindexSource.isPending && 'animate-spin')} />
                          </Button>
                          <Button type="button" size="icon" variant="ghost" className="h-7 w-7 text-muted-foreground hover:text-destructive" onClick={() => setDeleteTarget(source)} disabled={isMutating} title="Remove">
                            <Delete01Icon className="h-3.5 w-3.5" />
                          </Button>
                        </div>
                      </div>

                      {/* Row 2: URL + meta */}
                      <div className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-0.5 text-xs text-muted-foreground">
                        <a
                          href={source.start_url}
                          target="_blank"
                          rel="noreferrer"
                          className="inline-flex items-center gap-1 transition-colors hover:text-foreground"
                        >
                          <Link01Icon className="h-3 w-3" />
                          {stripProtocol(source.start_url)}
                        </a>
                        <span>Max {source.crawl_limit ?? 100} {(source.crawl_limit ?? 100) === 1 ? 'page' : 'pages'}</span>
                        <span>{indexedPages} {indexedPages === 1 ? 'page' : 'pages'} · {indexedChunks} {indexedChunks === 1 ? 'chunk' : 'chunks'}</span>
                        <span>{formatSyncTime(source.last_sync_completed_at, source.last_sync_started_at)}</span>
                      </div>

                      {/* Progress bar (syncing) */}
                      {(source.sync_status === 'queued' || source.sync_status === 'running') && (
                        <div className="mt-2 space-y-1">
                          <div className="flex items-center justify-between text-xs text-muted-foreground">
                            <span>{syncMessage}</span>
                            <span className="tabular-nums">{progress}%</span>
                          </div>
                          <Progress value={progress} className="h-1.5" />
                        </div>
                      )}

                      {/* Error message */}
                      {source.last_sync_error && (
                        <div className="mt-2 space-y-1">
                          <p className="text-xs text-destructive">
                            {friendlyError(source.last_sync_error)}
                          </p>
                          {hasIndexedContent && (
                            <p className="text-xs text-muted-foreground">
                              Last successful indexed content remains available while this refresh error is unresolved.
                            </p>
                          )}
                        </div>
                      )}
                    </div>
                  </div>
                </div>
              );
            })
          )}
        </div>
      </div>

      <Sheet open={wizardOpen} onOpenChange={(open) => {
        setWizardOpen(open);
        if (!open) {
          setEditingSource(null);
          setDraft(createDefaultDraft());
          setWizardStep('connect');
          setAdvancedOpen(false);
        }
      }}>
        <SheetContent side="right" className="w-full border-l border-border/70 p-0 data-[side=right]:w-full data-[side=right]:sm:max-w-6xl" showCloseButton>
          <SheetHeader className="border-b border-border/70 px-6 py-5 text-left">
            <SheetTitle>
              {editingSource ? 'Edit website source' : 'Add website source'}
            </SheetTitle>
            <SheetDescription>
              {editingSource
                ? 'Update the settings for this website source. Changes will trigger a fresh sync.'
                : 'Import content from a website so your support AI can reference it when answering questions.'}
            </SheetDescription>
          </SheetHeader>

          <ScrollArea className="h-[calc(100vh-10rem)]">
            <div className="space-y-6 px-6 py-4">
              {wizardStep === 'connect' && (
                <div className="space-y-6">
                  <div className="grid gap-5 sm:grid-cols-2">
                    <div className="space-y-2 sm:col-span-2">
                      <div className="flex items-center gap-1.5">
                        <Label htmlFor="content-source-url">Website URL</Label>
                        <TooltipProvider>
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <InformationCircleIcon className="h-3.5 w-3.5 text-muted-foreground cursor-help" />
                            </TooltipTrigger>
                            <TooltipContent side="top" className="max-w-[280px] space-y-1.5 text-xs leading-relaxed">
                              <p>For best results, sync <strong>support-focused content</strong> like help articles, product guides, or knowledge base pages.</p>
                              <p>Avoid marketing pages, product listings, or pages with complex layouts — these can reduce answer quality.</p>
                              <p>For multilingual sites, syncing one language version is sufficient unless the content differs between languages.</p>
                              <p>Website content is checked and synced automatically every day.</p>
                            </TooltipContent>
                          </Tooltip>
                        </TooltipProvider>
                      </div>
                      <Input
                        id="content-source-url"
                        value={draft.startUrl}
                        onChange={(event) => handleUrlChange(event.target.value)}
                        placeholder="https://docs.example.com"
                      />
                      <p className="text-xs text-muted-foreground">
                        The starting page for your website. All linked pages within this domain will be discovered automatically.
                      </p>
                    </div>

                    <div className="space-y-2 sm:col-span-2">
                      <Label htmlFor="content-source-name">Source name</Label>
                      <Input
                        id="content-source-name"
                        value={draft.name}
                        onChange={(event) => setDraft((current) => ({ ...current, name: event.target.value }))}
                        placeholder="My docs site"
                      />
                      <p className="text-xs text-muted-foreground">
                        A friendly name to identify this source in your list.
                      </p>
                    </div>
                  </div>

                  <div className="grid gap-5 md:grid-cols-2">
                    <div className="space-y-2">
                      <Label htmlFor="content-source-include-patterns">URLs to include</Label>
                      <Textarea
                        id="content-source-include-patterns"
                        rows={4}
                        value={draft.includePatternsText}
                        onChange={(event) => setPatternText('includePatternsText', event.target.value)}
                        placeholder={'/docs/*\n/blog/guides/*'}
                      />
                      <p className="text-xs text-muted-foreground">
                        One pattern per line. Only matching URLs will be imported. Leave empty to include everything.
                      </p>
                    </div>

                    <div className="space-y-2">
                      <Label htmlFor="content-source-exclude-patterns">URLs to exclude</Label>
                      <Textarea
                        id="content-source-exclude-patterns"
                        rows={4}
                        value={draft.excludePatternsText}
                        onChange={(event) => setPatternText('excludePatternsText', event.target.value)}
                        placeholder={'/login*\n/pricing*\n*?preview=*'}
                      />
                      <p className="text-xs text-muted-foreground">
                        Matching URLs will be skipped. Use this to exclude login pages, pricing, or other irrelevant content.
                      </p>
                    </div>
                  </div>

                  {/* Collapsible advanced settings */}
                  <div className="rounded-xl border border-border/70">
                    <button
                      type="button"
                      onClick={() => setAdvancedOpen((prev) => !prev)}
                      className="flex w-full items-center gap-2 px-4 py-3 text-sm text-muted-foreground transition-colors hover:text-foreground"
                    >
                      <ArrowRight01Icon className={cn('h-4 w-4 transition-transform', advancedOpen && 'rotate-90')} />
                      <span className="font-medium">Advanced settings</span>
                    </button>

                    {advancedOpen && (
                      <div className="space-y-5 border-t border-border/70 px-4 py-4">
                        <div className="grid gap-5 md:grid-cols-3">
                          <div className="space-y-2">
                            <Label>Page discovery</Label>
                            <Select value={draft.crawlSource} onValueChange={(value: 'all' | 'sitemaps' | 'links') => setDraft((current) => ({ ...current, crawlSource: value }))}>
                              <SelectTrigger>
                                <SelectValue />
                              </SelectTrigger>
                              <SelectContent>
                                {CRAWL_SOURCE_OPTIONS.map((option) => (
                                  <SelectItem key={option.value} value={option.value}>{option.label}</SelectItem>
                                ))}
                              </SelectContent>
                            </Select>
                            <p className="text-xs text-muted-foreground">
                              {CRAWL_SOURCE_OPTIONS.find((option) => option.value === draft.crawlSource)?.description}
                            </p>
                          </div>

                          <div className="space-y-2">
                            <Label htmlFor="content-source-limit">Max pages</Label>
                            <Input
                              id="content-source-limit"
                              type="number"
                              min={1}
                              max={1000}
                              value={draft.crawlLimit}
                              onChange={(event) => setDraft((current) => ({ ...current, crawlLimit: event.target.value }))}
                            />
                            <p className="text-xs text-muted-foreground">
                              Maximum number of pages to import from this website.
                            </p>
                          </div>

                          <div className="space-y-2">
                            <Label htmlFor="content-source-depth">Link depth</Label>
                            <Input
                              id="content-source-depth"
                              type="number"
                              min={1}
                              max={10}
                              value={draft.crawlDepth}
                              onChange={(event) => setDraft((current) => ({ ...current, crawlDepth: event.target.value }))}
                            />
                            <p className="text-xs text-muted-foreground">
                              How many levels of links to follow from the start page.
                            </p>
                          </div>
                        </div>

                        <div className="flex items-start justify-between rounded-xl border border-border/70 bg-muted/20 px-4 py-3">
                          <div className="pr-4">
                            <p className="text-sm font-medium">Include subdomains</p>
                            <p className="text-xs text-muted-foreground">
                              Also import pages from subdomains (e.g. docs.example.com, help.example.com).
                            </p>
                          </div>
                          <Switch checked={draft.includeSubdomains} onCheckedChange={(checked) => setDraft((current) => ({ ...current, includeSubdomains: checked }))} />
                        </div>
                      </div>
                    )}
                  </div>
                </div>
              )}

              {wizardStep === 'review' && (
                <div className="space-y-6">
                  <div>
                    <h3 className="text-lg font-semibold">Review and confirm</h3>
                    <p className="text-sm text-muted-foreground">
                      Double-check your settings before we start importing content from this website.
                    </p>
                  </div>

                  <div className="overflow-hidden rounded-2xl border border-border/70">
                    <div className="grid gap-0 divide-y divide-border/70">
                      <ReviewRow
                        label="Source"
                        value={(
                          <div className="space-y-1">
                            <div className="inline-flex items-center gap-2 font-medium">
                              <Favicon
                                url={draft.startUrl}
                                name={draft.name}
                                size={32}
                                className="h-4 w-4 rounded-sm border-none bg-transparent"
                                fallbackClassName="text-[8px]"
                              />
                              {draft.name}
                            </div>
                            <div className="text-xs text-muted-foreground">{draft.startUrl}</div>
                          </div>
                        )}
                      />
                      <ReviewRow label="Page discovery" value={`${labelForCrawlSource(draft.crawlSource)} · up to ${draft.crawlLimit || '100'} pages · ${draft.crawlDepth || '2'} levels deep`} />
                      <ReviewRow label="Filters" value={reviewFilters(draft)} />
                      <ReviewRow
                        label="Agent"
                        value={
                          willUseInCurrentAgent === null
                            ? 'Saved to this workspace. Attach to a support agent later.'
                            : willUseInCurrentAgent
                              ? 'Will be used by the current support agent'
                              : 'Saved but not attached to the current agent'
                        }
                      />
                    </div>
                  </div>
                </div>
              )}
            </div>
          </ScrollArea>

          <SheetFooter className="border-t border-border/70 px-6 py-4 sm:flex-row sm:items-center sm:justify-between">
            <div className="flex items-center gap-2">
              {wizardStep !== 'connect' && (
                <Button type="button" variant="outline" onClick={handlePrevious} disabled={isMutating}>
                  Previous
                </Button>
              )}
            </div>
            <div className="flex items-center gap-2">
              <Button type="button" variant="outline" onClick={() => setWizardOpen(false)} disabled={isMutating}>
                Cancel
              </Button>
              {wizardStep === 'review' ? (
                <Button type="button" onClick={() => void submitWizard()} disabled={isMutating}>
                  {(createSource.isPending || updateSource.isPending || updateAgentSources.isPending) && <Loading03Icon className="h-4 w-4 animate-spin" />}
                  {editingSource ? 'Save and re-sync' : 'Add and sync'}
                </Button>
              ) : (
                <Button type="button" onClick={handleNext} disabled={isMutating}>
                  Next
                </Button>
              )}
            </div>
          </SheetFooter>
        </SheetContent>
      </Sheet>

      <Sheet open={!!viewingSource} onOpenChange={(open) => { if (!open) setViewingSource(null); }}>
        <SheetContent side="right" className="w-full border-l border-border/70 p-0 data-[side=right]:w-[88vw] data-[side=right]:sm:max-w-[88vw]" showCloseButton>
          <SheetHeader className="border-b border-border/70 px-6 py-5 text-left">
            <SheetTitle>{viewingSource?.name} — Synced Pages</SheetTitle>
            <SheetDescription>
              All pages crawled from this website source.
            </SheetDescription>
          </SheetHeader>
          <ScrollArea className="h-[calc(100vh-8rem)]">
            {viewingSource && (
              <ContentSourcePagesPanel workspaceId={workspaceId} contentSourceId={viewingSource.id} />
            )}
          </ScrollArea>
        </SheetContent>
      </Sheet>

      <AlertDialog open={!!deleteTarget} onOpenChange={(open) => !open && setDeleteTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Remove website source?</AlertDialogTitle>
            <AlertDialogDescription>
              All imported content from <span className="font-medium">{deleteTarget?.name ?? 'this source'}</span> will be permanently deleted. Your support AI will no longer be able to reference it.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleteSource.isPending}>Cancel</AlertDialogCancel>
            <AlertDialogAction onClick={() => void handleDelete()} variant="destructive" disabled={deleteSource.isPending}>
              {deleteSource.isPending && <Loading03Icon className="h-4 w-4 animate-spin" />}
              Remove
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}

function ContentSourcePagesPanel({ workspaceId, contentSourceId }: { workspaceId: string; contentSourceId: string }) {
  const { data: pages, isLoading } = useSupportContentSourcePages(workspaceId, contentSourceId);
  const [search, setSearch] = useState('');
  const [previewPageId, setPreviewPageId] = useState<string | null>(null);

  const filteredPages = (pages ?? []).filter((page) => {
    if (!search.trim()) return true;
    const q = search.toLowerCase();
    return page.title?.toLowerCase().includes(q) || page.url?.toLowerCase().includes(q);
  });

  if (isLoading) {
    return (
      <div className="space-y-3 p-6">
        <Skeleton className="h-14 rounded-lg" />
        <Skeleton className="h-14 rounded-lg" />
        <Skeleton className="h-14 rounded-lg" />
      </div>
    );
  }

  if (!pages || pages.length === 0) {
    return (
      <div className="px-6 py-16 text-center">
        <div className="mx-auto mb-3 flex h-10 w-10 items-center justify-center rounded-full bg-muted text-muted-foreground">
          <File01Icon className="h-5 w-5" />
        </div>
        <p className="text-sm font-medium">No pages synced yet</p>
        <p className="mx-auto mt-1 max-w-md text-sm text-muted-foreground">
          Pages will appear here once the sync completes.
        </p>
      </div>
    );
  }

  if (previewPageId) {
    return (
      <PageContentPreview
        workspaceId={workspaceId}
        contentSourceId={contentSourceId}
        pageId={previewPageId}
        onBack={() => setPreviewPageId(null)}
      />
    );
  }

  return (
    <div>
      <div className="border-b border-border/70 px-6 py-3">
        <QuietSearchInput
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Search pages by title or URL…"
        />
        <p className="mt-2 text-xs text-muted-foreground">
          {search.trim()
            ? `${filteredPages.length} of ${pages.length} pages`
            : `${pages.length} ${pages.length === 1 ? 'page' : 'pages'} indexed`}
        </p>
      </div>

      {filteredPages.length === 0 ? (
        <div className="px-6 py-12 text-center">
          <p className="text-sm text-muted-foreground">No pages match your search.</p>
        </div>
      ) : (
        <div className="divide-y divide-border/70">
          {filteredPages.map((page) => (
            <div key={page.id} className="flex items-start gap-3 px-6 py-3">
              <Favicon
                url={page.url}
                name={page.title || page.url}
                size={16}
                className="mt-0.5 h-4 w-4 rounded-sm border-none bg-transparent"
                fallbackClassName="text-[8px]"
              />
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm font-medium">{page.title || 'Untitled'}</p>
                <div className="mt-0.5 flex flex-wrap items-center gap-x-3 gap-y-0.5 text-xs text-muted-foreground">
                  <a
                    href={page.url}
                    target="_blank"
                    rel="noreferrer"
                    className="inline-flex items-center gap-1 transition-colors hover:text-foreground"
                  >
                    <LinkSquare01Icon className="h-3 w-3" />
                    <span className="max-w-[360px] truncate">{page.url}</span>
                  </a>
                  {page.content_length > 0 && (
                    <span>{formatContentLength(page.content_length)}</span>
                  )}
                  {page.last_crawled_at && (
                    <span>
                      Crawled {new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(page.last_crawled_at))}
                    </span>
                  )}
                </div>
              </div>
              <div className="flex shrink-0 items-center gap-1 mt-0.5">
                <Button
                  type="button"
                  size="icon"
                  variant="ghost"
                  className="h-7 w-7"
                  onClick={() => setPreviewPageId(page.id)}
                  title="Preview content"
                >
                  <ViewIcon className="h-3.5 w-3.5" />
                </Button>
                <Badge
                  variant="outline"
                  className={cn(
                    page.http_status >= 200 && page.http_status < 300
                      ? 'border-emerald-500/40 bg-emerald-500/10 text-emerald-700'
                      : page.http_status >= 400
                        ? 'border-destructive/40 bg-destructive/10 text-destructive'
                        : 'border-amber-500/40 bg-amber-500/10 text-amber-700',
                  )}
                >
                  {page.http_status || '—'}
                </Badge>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function createDefaultDraft(): ContentSourceDraft {
  return {
    name: '',
    startUrl: '',
    crawlLimit: '100',
    crawlDepth: '2',
    crawlSource: 'all',
    formats: ['markdown'],
    render: true,
    includeExternalLinks: false,
    includeSubdomains: false,
    includePatternsText: '',
    excludePatternsText: '',
    maxAgeSeconds: '86400',
    jsonPrompt: '',
    jsonResponseFormatText: '',
  };
}

function draftFromSource(source: SupportContentSource): ContentSourceDraft {
  return {
    name: source.name,
    startUrl: source.start_url,
    crawlLimit: String(source.crawl_limit || 100),
    crawlDepth: String(source.crawl_depth || 2),
    crawlSource: source.crawl_source || 'all',
    formats: source.formats?.length ? [...source.formats] : ['markdown'],
    render: source.render,
    includeExternalLinks: source.include_external_links,
    includeSubdomains: source.include_subdomains,
    includePatternsText: (source.include_patterns ?? []).join('\n'),
    excludePatternsText: (source.exclude_patterns ?? []).join('\n'),
    maxAgeSeconds: String(source.max_age_seconds || 86400),
    jsonPrompt: source.json_prompt ?? '',
    jsonResponseFormatText: source.json_response_format ? JSON.stringify(source.json_response_format, null, 2) : '',
  };
}

function validateStep(draft: ContentSourceDraft, step: WizardStep) {
  if (step === 'connect') {
    if (!draft.name.trim()) {
      toast.error('Source name is required');
      return false;
    }
    if (!isAbsoluteURL(draft.startUrl)) {
      toast.error('Enter a valid URL starting with https://');
      return false;
    }
    if (!isPositiveInteger(draft.crawlLimit)) {
      toast.error('Max pages must be a positive number');
      return false;
    }
    if (!isPositiveInteger(draft.crawlDepth)) {
      toast.error('Link depth must be a positive number');
      return false;
    }
  }

  return true;
}

function buildPayload(draft: ContentSourceDraft): CreateSupportContentSourceRequest {
  const payload: CreateSupportContentSourceRequest = {
    name: draft.name.trim(),
    start_url: draft.startUrl.trim(),
    crawl_limit: parsePositiveInteger(draft.crawlLimit, 100),
    crawl_depth: parsePositiveInteger(draft.crawlDepth, 2),
    crawl_source: draft.crawlSource,
    formats: uniqueStrings(draft.formats),
    render: draft.render,
    include_external_links: draft.includeExternalLinks,
    include_subdomains: draft.includeSubdomains,
    include_patterns: parseLineList(draft.includePatternsText),
    exclude_patterns: parseLineList(draft.excludePatternsText),
    crawl_purposes: ['search', 'ai-input'],
    max_age_seconds: parsePositiveInteger(draft.maxAgeSeconds, 86400),
  };

  if (draft.formats.includes('json') && draft.jsonPrompt.trim()) {
    payload.json_prompt = draft.jsonPrompt.trim();
  }
  if (draft.jsonResponseFormatText.trim()) {
    payload.json_response_format = JSON.parse(draft.jsonResponseFormatText);
  }

  return payload;
}

function formatSyncTime(completedAt?: string | null, startedAt?: string | null) {
  const value = completedAt || startedAt;
  if (!value) {
    return 'Not synced yet';
  }

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return 'Sync time unavailable';
  }

  return `Updated ${new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(date)}`;
}

function labelForCrawlSource(value: ContentSourceDraft['crawlSource']) {
  return CRAWL_SOURCE_OPTIONS.find((option) => option.value === value)?.label ?? 'Sitemaps and links';
}

function reviewFilters(draft: ContentSourceDraft) {
  const filters = [];
  const includes = parseLineList(draft.includePatternsText).length;
  const excludes = parseLineList(draft.excludePatternsText).length;
  if (includes > 0) {
    filters.push(`${includes} include pattern${includes === 1 ? '' : 's'}`);
  }
  if (excludes > 0) {
    filters.push(`${excludes} exclude pattern${excludes === 1 ? '' : 's'}`);
  }
  if (filters.length === 0) {
    return 'No filters — all discovered pages will be imported';
  }
  return filters.join(' · ');
}

function parseLineList(value: string) {
  return value
    .split('\n')
    .map((item) => item.trim())
    .filter(Boolean);
}

function parsePositiveInteger(value: string, fallback: number) {
  const parsed = Number.parseInt(value, 10);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return fallback;
  }
  return parsed;
}

function isPositiveInteger(value: string) {
  const parsed = Number.parseInt(value, 10);
  return Number.isFinite(parsed) && parsed > 0;
}

function isAbsoluteURL(value: string) {
  try {
    const parsed = new URL(value);
    return parsed.protocol === 'http:' || parsed.protocol === 'https:';
  } catch {
    return false;
  }
}

function suggestNameFromURL(value: string) {
  try {
    const parsed = new URL(value);
    return stripWWW(parsed.hostname);
  } catch {
    return '';
  }
}

function stripProtocol(value: string) {
  return value.replace(/^https?:\/\//, '');
}

function stripWWW(value: string) {
  return value.replace(/^www\./, '');
}

function uniqueStrings(values: string[]) {
  return Array.from(new Set(values));
}

function formatContentLength(bytes: number) {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function friendlyError(raw: string) {
  if (raw.includes('cannot unmarshal')) {
    return 'Sync failed due to an unexpected response format. Try again — this is usually temporary.';
  }
  if (raw.includes('cloudflare crawl client is not configured')) {
    return 'Website sync is not configured. Check that your Cloudflare credentials are set.';
  }
  if (raw.includes('timeout') || raw.includes('deadline exceeded')) {
    return 'Sync timed out. The website may be too large or slow to respond. Try reducing the page limit.';
  }
  if (raw.includes('embedding') || raw.includes('openai')) {
    return 'Failed to process page content. Check that your embedding provider is configured correctly.';
  }
  // Truncate long raw errors to keep the UI clean.
  if (raw.length > 120) {
    return `${raw.slice(0, 120)}\u2026`;
  }
  return raw;
}
