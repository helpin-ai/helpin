import { useCallback, useEffect, useState, type FormEvent } from 'react';
import { docsRedirectService, type DocsRedirect } from '@/lib/services/docsRedirectService';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Checkbox } from '@/components/ui/checkbox';
import { Label } from '@/components/ui/label';
import { Skeleton } from '@/components/ui/skeleton';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Delete01Icon, PlusSignIcon, PencilEdit01Icon } from '@/lib/icons';
import { QuietSearchInput } from '@/components/design-system/quiet';
import { toast } from 'sonner';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { formatRedirectTargetPath, normalizeRedirectSourcePathForDisplay } from '@/lib/docsRedirectPaths';
import { CollectionTreePicker } from '@/components/docs/CollectionTreePicker';
import { useDocsSpaces, useDocsCollections, useDocsDocuments } from '@/hooks/queries';
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover';
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from '@/components/ui/command';
import { ArrowDown01Icon, Tick01Icon, File01Icon } from '@/lib/icons';
import type { DocsDocument } from '@/lib/docsTypes';

const PER_PAGE = 20;

function ArticleSearchPicker({
  articles,
  value,
  onChange,
}: {
  articles: DocsDocument[]
  value: string
  onChange: (slug: string) => void
}) {
  const [open, setOpen] = useState(false)
  const selected = articles.find((a) => (a.hc_slug || a.id) === value)

  return (
    <div className="space-y-1">
      <Label className="text-xs text-muted-foreground">Article <span className="font-normal">(optional)</span></Label>
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger asChild>
          <button
            type="button"
            className="flex h-8 w-full items-center justify-between rounded-md border border-input bg-background px-3 text-xs ring-offset-background hover:bg-accent hover:text-accent-foreground"
          >
            <span className="truncate">
              {selected ? selected.title : 'Collection-level redirect'}
            </span>
            <ArrowDown01Icon className="h-3.5 w-3.5 shrink-0 opacity-50" />
          </button>
        </PopoverTrigger>
        <PopoverContent className="w-[300px] p-0" align="start">
          <Command>
            <CommandInput placeholder="Search articles…" className="h-8 text-xs" />
            <CommandList>
              <CommandEmpty>No articles found.</CommandEmpty>
              <CommandGroup>
                <CommandItem
                  value="__none__"
                  onSelect={() => { onChange(''); setOpen(false); }}
                  className="text-xs"
                >
                  <span className="text-muted-foreground">Collection-level redirect</span>
                  {!value && <Tick01Icon className="ml-auto h-3 w-3" />}
                </CommandItem>
                {articles.map((d) => {
                  const slug = d.hc_slug || d.id
                  return (
                    <CommandItem
                      key={d.id}
                      value={d.title}
                      onSelect={() => { onChange(slug); setOpen(false); }}
                      className="text-xs"
                    >
                      <File01Icon className="mr-1.5 h-3 w-3 shrink-0 text-muted-foreground" />
                      <span className="truncate">{d.title}</span>
                      {value === slug && <Tick01Icon className="ml-auto h-3 w-3" />}
                    </CommandItem>
                  )
                })}
              </CommandGroup>
            </CommandList>
          </Command>
        </PopoverContent>
      </Popover>
    </div>
  )
}

function formatTarget(redirect: DocsRedirect): string {
  return redirect.target_path ?? formatRedirectTargetPath(redirect.target_collection_slug, redirect.target_article_slug);
}

function typeBadge(type: DocsRedirect['type']) {
  switch (type) {
    case 'imported':
      return <Badge variant="secondary">Imported</Badge>;
    case 'slug_change':
      return <Badge variant="outline" className="border-amber-300 bg-amber-50 text-amber-700 dark:border-amber-700 dark:bg-amber-950 dark:text-amber-300">Slug Change</Badge>;
    case 'auto_article_move':
      return <Badge variant="outline" className="border-blue-300 bg-blue-50 text-blue-700 dark:border-blue-700 dark:bg-blue-950 dark:text-blue-300">Article Move</Badge>;
    case 'auto_collection_rename':
      return <Badge variant="outline" className="border-purple-300 bg-purple-50 text-purple-700 dark:border-purple-700 dark:bg-purple-950 dark:text-purple-300">Collection Rename</Badge>;
    case 'manual':
    default:
      return <Badge variant="default">Manual</Badge>;
  }
}

export function RedirectsTab({ workspaceId, editable }: { workspaceId: string; editable: boolean }) {
  const [redirects, setRedirects] = useState<DocsRedirect[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState('');
  const [debouncedSearch, setDebouncedSearch] = useState('');
  const [typeFilter, setTypeFilter] = useState('all');
  const [createOpen, setCreateOpen] = useState(false);
  const [editRedirect, setEditRedirect] = useState<DocsRedirect | null>(null);
  const [sourcePath, setSourcePath] = useState('');
  const [targetCollectionSlug, setTargetCollectionSlug] = useState('');
  const [targetArticleSlug, setTargetArticleSlug] = useState('');
  const [targetMode, setTargetMode] = useState<'picker' | 'manual'>('picker');
  const [manualTargetPath, setManualTargetPath] = useState('');
  const [pickerSpaceId, setPickerSpaceId] = useState('');
  const [pickerCollectionId, setPickerCollectionId] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const { data: spaces } = useDocsSpaces(workspaceId);
  const externalSpaces = (spaces ?? []).filter((s) => s.type === 'external_capable');
  const { data: pickerCollections } = useDocsCollections(workspaceId, pickerSpaceId || '');
  const { data: pickerDocuments } = useDocsDocuments(workspaceId, pickerSpaceId ? { space_id: pickerSpaceId } : {});
  const pickerArticles = (pickerDocuments ?? []).filter(
    (d) => pickerCollectionId && d.collection_id === pickerCollectionId && d.status === 'published',
  );
  const [pickerArticleSlug, setPickerArticleSlug] = useState('');
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const [deleteConfirmId, setDeleteConfirmId] = useState<string | null>(null);
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const [bulkDeleteConfirm, setBulkDeleteConfirm] = useState(false);
  const [bulkDeleting, setBulkDeleting] = useState(false);

  const toggleSelect = (id: string) => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id); else next.add(id);
      return next;
    });
  };

  const toggleSelectAll = () => {
    if (selectedIds.size === redirects.length) {
      setSelectedIds(new Set());
    } else {
      setSelectedIds(new Set(redirects.map((r) => r.id)));
    }
  };

  const handleBulkDelete = async () => {
    setBulkDeleting(true);
    let failed = 0;
    for (const id of selectedIds) {
      const { error } = await docsRedirectService.delete(workspaceId, id);
      if (error) failed++;
    }
    setBulkDeleting(false);
    setBulkDeleteConfirm(false);
    setSelectedIds(new Set());
    loadRedirects();
    if (failed === 0) {
      toast.success(`Deleted ${selectedIds.size} redirect${selectedIds.size === 1 ? '' : 's'}`);
    } else {
      toast.warning(`Deleted ${selectedIds.size - failed}, ${failed} failed`);
    }
  };

  // Debounce search input
  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedSearch(search);
      setPage(1);
    }, 300);
    return () => clearTimeout(timer);
  }, [search]);

  const loadRedirects = useCallback(async () => {
    setLoading(true);
    try {
      const { data, error } = await docsRedirectService.list(workspaceId, {
        search: debouncedSearch || undefined,
        type: typeFilter !== 'all' ? typeFilter : undefined,
        page,
        per_page: PER_PAGE,
      });
      if (error) {
        toast.error(error);
      } else if (data) {
        setRedirects(data.items ?? []);
        setTotal(data.total);
      }
    } finally {
      setLoading(false);
    }
  }, [workspaceId, debouncedSearch, typeFilter, page]);

  useEffect(() => { loadRedirects(); }, [loadRedirects]);

  const totalPages = Math.max(1, Math.ceil(total / PER_PAGE));

  const resetForm = () => {
    setSourcePath('');
    setTargetCollectionSlug('');
    setTargetArticleSlug('');
    setTargetMode('picker');
    setManualTargetPath('');
    setPickerSpaceId(externalSpaces[0]?.id ?? '');
    setPickerCollectionId(null);
    setPickerArticleSlug('');
  };

  const openCreate = () => {
    resetForm();
    setEditRedirect(null);
    setCreateOpen(true);
  };

  const openEdit = (r: DocsRedirect) => {
    setSourcePath(normalizeRedirectSourcePathForDisplay(r.source_path));
    setTargetCollectionSlug(r.target_collection_slug);
    setTargetArticleSlug(r.target_article_slug ?? '');
    setManualTargetPath(formatRedirectTargetPath(r.target_collection_slug, r.target_article_slug));
    setTargetMode('manual'); // show existing values as path
    setEditRedirect(r);
    setCreateOpen(true);
  };

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();

    // Resolve target fields based on mode
    let finalCollectionSlug = targetCollectionSlug.trim();
    let finalArticleSlug = targetArticleSlug.trim() || undefined;

    if (targetMode === 'picker') {
      // Picker mode: collection is selected via picker
      const col = (pickerCollections ?? []).find((c) => c.id === pickerCollectionId);
      if (!col) {
        toast.error('Select a target collection.');
        return;
      }
      finalCollectionSlug = col.slug;
      finalArticleSlug = pickerArticleSlug || undefined;
    } else {
      // Manual mode: parse path into collection + article slugs
      const trimmed = manualTargetPath.trim().replace(/^\/+/, '');
      if (!trimmed) {
        toast.error('Enter a target path.');
        return;
      }
      // Handle /c/{slug} format
      if (trimmed.startsWith('c/')) {
        finalCollectionSlug = trimmed.slice(2).split('/')[0];
        finalArticleSlug = undefined;
      } else {
        // Legacy format: {collectionSlug}/{articleSlug} or just {collectionSlug}
        const parts = trimmed.split('/').filter(Boolean);
        finalCollectionSlug = parts[0] ?? '';
        finalArticleSlug = parts[1] || undefined;
      }
    }

    if (!sourcePath.trim() || !finalCollectionSlug) {
      toast.error('Old path and target are required.');
      return;
    }
    setSaving(true);
    try {
      if (editRedirect) {
        const { error } = await docsRedirectService.update(workspaceId, editRedirect.id, {
          source_path: normalizeRedirectSourcePathForDisplay(sourcePath),
          target_collection_slug: finalCollectionSlug,
          target_article_slug: finalArticleSlug,
        });
        if (error) {
          toast.error(error);
        } else {
          toast.success('Redirect updated.');
          setCreateOpen(false);
          resetForm();
          setEditRedirect(null);
          loadRedirects();
        }
      } else {
        const { error } = await docsRedirectService.create(workspaceId, {
          source_path: normalizeRedirectSourcePathForDisplay(sourcePath),
          target_collection_slug: finalCollectionSlug,
          target_article_slug: finalArticleSlug,
        });
        if (error) {
          toast.error(error);
        } else {
          toast.success('Redirect created.');
          setCreateOpen(false);
          resetForm();
          loadRedirects();
        }
      }
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (id: string) => {
    setDeletingId(id);
    try {
      const { error } = await docsRedirectService.delete(workspaceId, id);
      if (error) {
        toast.error(error);
      } else {
        toast.success('Redirect deleted.');
        loadRedirects();
      }
    } finally {
      setDeletingId(null);
    }
  };

  if (loading && redirects.length === 0) return <Skeleton className="h-96" />;

  return (
    <>
      <div className="space-y-4">
        <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
          <span className="text-sm text-muted-foreground">{total} {total === 1 ? 'redirect' : 'redirects'}</span>
          <div className="flex items-center gap-3">
            <QuietSearchInput
              containerClassName="w-60"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Search by old path..."
            />
            <Select value={typeFilter} onValueChange={(v) => { setTypeFilter(v); setPage(1); }}>
              <SelectTrigger className="h-9 w-40">
                <SelectValue placeholder="Filter by type" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All Types</SelectItem>
                <SelectItem value="manual">Manual</SelectItem>
                <SelectItem value="auto_article_move">Article Move</SelectItem>
                <SelectItem value="auto_collection_rename">Collection Rename</SelectItem>
                <SelectItem value="slug_change">Slug Change</SelectItem>
                <SelectItem value="imported">Imported</SelectItem>
              </SelectContent>
            </Select>
            {editable && selectedIds.size > 0 && (
              <Button size="sm" variant="destructive" disabled={bulkDeleting} onClick={() => setBulkDeleteConfirm(true)}>
                {bulkDeleting ? (
                  <><span className="h-4 w-4 mr-1 animate-spin rounded-full border-2 border-current border-t-transparent" /> Deleting...</>
                ) : (
                  <><Delete01Icon className="h-4 w-4 mr-1" /> Delete {selectedIds.size}</>
                )}
              </Button>
            )}
            {editable && (
              <Button size="sm" onClick={openCreate}>
                <PlusSignIcon className="h-4 w-4 mr-1" /> Add Redirect
              </Button>
            )}
          </div>
        </div>

        {redirects.length === 0 && !loading ? (
          <div className="flex min-h-[320px] flex-col items-center justify-center gap-3 rounded-lg border border-dashed border-border text-center">
            <div className="space-y-1">
              <p className="font-medium">No redirects yet</p>
              <p className="text-sm text-muted-foreground max-w-md">
                Redirects are created automatically when importing from external tools or when article slugs change.
              </p>
            </div>
            {editable && (
              <Button onClick={openCreate}>
                <PlusSignIcon className="h-4 w-4 mr-1" />
                Add Redirect
              </Button>
            )}
          </div>
        ) : (
          <div className="overflow-hidden rounded-lg border border-border">
            <Table className="table-fixed">
              <TableHeader>
                <TableRow className="bg-muted/40 hover:bg-muted/40">
                  {editable && (
                    <TableHead className="w-[40px]">
                      <Checkbox
                        checked={redirects.length > 0 && selectedIds.size === redirects.length}
                        onCheckedChange={toggleSelectAll}
                        className="h-3.5 w-3.5"
                      />
                    </TableHead>
                  )}
                  <TableHead className="w-[30%]">Old Path</TableHead>
                  <TableHead className="w-[30%]">New Path</TableHead>
                  <TableHead className="w-[100px]">Type</TableHead>
                  <TableHead className="w-[90px]">Source</TableHead>
                  <TableHead className="w-[100px]">Created</TableHead>
                  {editable && <TableHead className="w-[70px]" />}
                </TableRow>
              </TableHeader>
              <TableBody>
                {redirects.map((r) => (
                  <TableRow key={r.id} className={selectedIds.has(r.id) ? 'bg-primary/5' : ''}>
                    {editable && (
                      <TableCell>
                        <Checkbox
                          checked={selectedIds.has(r.id)}
                          onCheckedChange={() => toggleSelect(r.id)}
                          className="h-3.5 w-3.5"
                        />
                      </TableCell>
                    )}
                    <TableCell>
                      <code className="block truncate rounded bg-muted px-1.5 py-0.5 text-xs font-mono" title={normalizeRedirectSourcePathForDisplay(r.source_path)}>{normalizeRedirectSourcePathForDisplay(r.source_path)}</code>
                    </TableCell>
                    <TableCell>
                      <span className="block truncate text-sm text-muted-foreground" title={formatTarget(r)}>{formatTarget(r)}</span>
                    </TableCell>
                    <TableCell>{typeBadge(r.type)}</TableCell>
                    <TableCell className="text-sm text-muted-foreground truncate">{r.source_system ?? '\u2014'}</TableCell>
                    <TableCell className="text-sm text-muted-foreground">
                      {new Date(r.created_at).toLocaleDateString()}
                    </TableCell>
                    {editable && (
                      <TableCell>
                        <div className="flex items-center gap-0.5">
                          <QuickTooltip label="Edit redirect">
                            <Button
                              size="icon"
                              variant="ghost"
                              className="h-7 w-7"
                              onClick={() => openEdit(r)}
                            >
                              <PencilEdit01Icon className="h-3 w-3" />
                            </Button>
                          </QuickTooltip>
                          <QuickTooltip label="Delete redirect">
                            <Button
                              size="icon"
                              variant="ghost"
                              className="h-7 w-7"
                              disabled={deletingId === r.id}
                              onClick={() => setDeleteConfirmId(r.id)}
                            >
                              <Delete01Icon className="h-3 w-3" />
                            </Button>
                          </QuickTooltip>
                        </div>
                      </TableCell>
                    )}
                  </TableRow>
                ))}
                {redirects.length === 0 && loading && (
                  <TableRow>
                    <TableCell colSpan={editable ? 6 : 5} className="py-8 text-center text-sm text-muted-foreground">
                      Loading...
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </div>
        )}

        {totalPages > 1 && (
          <div className="flex items-center justify-between pt-2">
            <span className="text-sm text-muted-foreground">
              Page {page} of {totalPages}
            </span>
            <div className="flex gap-2">
              <Button size="sm" variant="outline" disabled={page <= 1} onClick={() => setPage((p) => p - 1)}>
                Previous
              </Button>
              <Button size="sm" variant="outline" disabled={page >= totalPages} onClick={() => setPage((p) => p + 1)}>
                Next
              </Button>
            </div>
          </div>
        )}
      </div>

      <Dialog open={createOpen} onOpenChange={(open) => { setCreateOpen(open); if (!open) { setEditRedirect(null); resetForm(); } }}>
        <DialogContent className="sm:max-w-md">
          <form onSubmit={handleSubmit}>
            <DialogHeader>
              <DialogTitle>{editRedirect ? 'Edit Redirect' : 'Add Redirect'}</DialogTitle>
            </DialogHeader>
            <div className="space-y-4 py-4">
              <div className="space-y-2">
                <Label htmlFor="source-path">Old Path</Label>
                <Input
                  id="source-path"
                  value={sourcePath}
                  onChange={(e) => setSourcePath(e.target.value)}
                  placeholder="/old-page"
                  required
                />
              </div>

              <div className="space-y-3">
                <Label>New Path</Label>
                <div
                  role="tablist"
                  className="inline-flex rounded-md border border-border/60 p-0.5 text-xs"
                >
                  {(['manual', 'picker'] as const).map((mode) => (
                    <button
                      key={mode}
                      type="button"
                      role="tab"
                      aria-selected={targetMode === mode}
                      onClick={() => setTargetMode(mode)}
                      className={`rounded px-2.5 py-1 font-medium transition-colors ${
                        targetMode === mode
                          ? 'bg-muted text-foreground'
                          : 'text-muted-foreground hover:text-foreground'
                      }`}
                    >
                      {mode === 'manual' ? 'Enter manually' : 'Pick'}
                    </button>
                  ))}
                </div>

                {targetMode === 'manual' ? (
                  <Input
                    value={manualTargetPath}
                    onChange={(e) => setManualTargetPath(e.target.value)}
                    placeholder="e.g. /c/billing or /billing/pricing-faq"
                  />
                ) : (
                  <div className="space-y-3">
                    {externalSpaces.length > 1 && (
                      <div className="space-y-1">
                        <Label className="text-xs text-muted-foreground">Space</Label>
                        <Select
                          value={pickerSpaceId}
                          onValueChange={(v) => { setPickerSpaceId(v); setPickerCollectionId(null); }}
                        >
                          <SelectTrigger className="h-8 text-xs">
                            <SelectValue placeholder="Select space" />
                          </SelectTrigger>
                          <SelectContent>
                            {externalSpaces.map((s) => (
                              <SelectItem key={s.id} value={s.id}>{s.name}</SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      </div>
                    )}
                    {pickerSpaceId && (
                      <>
                        <div className="space-y-1">
                          <Label className="text-xs text-muted-foreground">Collection</Label>
                          <CollectionTreePicker
                            collections={pickerCollections ?? []}
                            spaceId={pickerSpaceId}
                            value={pickerCollectionId}
                            onChange={(id) => { setPickerCollectionId(id); setPickerArticleSlug(''); }}
                            allowNone={false}
                            placeholder="Select target collection…"
                          />
                        </div>
                        {pickerCollectionId && pickerArticles.length > 0 && (
                          <ArticleSearchPicker
                            articles={pickerArticles}
                            value={pickerArticleSlug}
                            onChange={setPickerArticleSlug}
                          />
                        )}
                      </>
                    )}
                  </div>
                )}
              </div>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setCreateOpen(false)}>Cancel</Button>
              <Button type="submit" disabled={saving}>{saving ? 'Saving...' : editRedirect ? 'Update' : 'Save'}</Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={!!deleteConfirmId}
        onOpenChange={(open) => { if (!open) setDeleteConfirmId(null); }}
        title="Delete redirect"
        description="This redirect may be serving live traffic. Deleting it could cause broken links for visitors. This action cannot be undone."
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={() => {
          if (deleteConfirmId) handleDelete(deleteConfirmId);
          setDeleteConfirmId(null);
        }}
      />

      <ConfirmDialog
        open={bulkDeleteConfirm}
        onOpenChange={(open) => { if (!open && !bulkDeleting) setBulkDeleteConfirm(false); }}
        title={`Delete ${selectedIds.size} redirect${selectedIds.size === 1 ? '' : 's'}?`}
        description={bulkDeleting
          ? `Deleting ${selectedIds.size} redirect${selectedIds.size === 1 ? '' : 's'}…`
          : 'These redirects may be serving live traffic. Deleting them could cause broken links for visitors. This action cannot be undone.'}
        confirmLabel={bulkDeleting ? 'Deleting...' : `Delete ${selectedIds.size}`}
        variant="destructive"
        onConfirm={handleBulkDelete}
      />
    </>
  );
}
