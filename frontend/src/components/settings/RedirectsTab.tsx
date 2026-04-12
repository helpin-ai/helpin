import { useCallback, useEffect, useState, type FormEvent } from 'react';
import { docsRedirectService, type DocsRedirect } from '@/lib/services/docsRedirectService';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Skeleton } from '@/components/ui/skeleton';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Delete01Icon, PlusSignIcon, Search01Icon, PencilEdit01Icon } from '@/lib/icons';
import { toast } from 'sonner';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { formatRedirectTargetPath, normalizeRedirectSourcePathForDisplay } from '@/lib/docsRedirectPaths';

const PER_PAGE = 20;

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
  const [saving, setSaving] = useState(false);
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const [deleteConfirmId, setDeleteConfirmId] = useState<string | null>(null);

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
    setEditRedirect(r);
    setCreateOpen(true);
  };

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (!sourcePath.trim() || !targetCollectionSlug.trim()) {
      toast.error('Source path and target collection slug are required.');
      return;
    }
    setSaving(true);
    try {
      if (editRedirect) {
        const { error } = await docsRedirectService.update(workspaceId, editRedirect.id, {
          source_path: normalizeRedirectSourcePathForDisplay(sourcePath),
          target_collection_slug: targetCollectionSlug.trim(),
          target_article_slug: targetArticleSlug.trim() || undefined,
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
          target_collection_slug: targetCollectionSlug.trim(),
          target_article_slug: targetArticleSlug.trim() || undefined,
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
            <div className="relative">
              <Search01Icon className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                className="h-9 w-60 pl-9"
                placeholder="Search by source path..."
              />
            </div>
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
                  <TableHead className="w-[30%]">Source Path</TableHead>
                  <TableHead className="w-[30%]">Target</TableHead>
                  <TableHead className="w-[100px]">Type</TableHead>
                  <TableHead className="w-[90px]">Source</TableHead>
                  <TableHead className="w-[100px]">Created</TableHead>
                  {editable && <TableHead className="w-[70px]" />}
                </TableRow>
              </TableHeader>
              <TableBody>
                {redirects.map((r) => (
                  <TableRow key={r.id}>
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
                <Label htmlFor="source-path">Source Path</Label>
                <Input
                  id="source-path"
                  value={sourcePath}
                  onChange={(e) => setSourcePath(e.target.value)}
                  placeholder="/old-page"
                  required
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="target-collection">Target Collection Slug</Label>
                <Input
                  id="target-collection"
                  value={targetCollectionSlug}
                  onChange={(e) => setTargetCollectionSlug(e.target.value)}
                  placeholder="account-management"
                  required
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="target-article">Target Article Slug <span className="text-muted-foreground font-normal">(optional)</span></Label>
                <Input
                  id="target-article"
                  value={targetArticleSlug}
                  onChange={(e) => setTargetArticleSlug(e.target.value)}
                  placeholder="team-members"
                />
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
    </>
  );
}
