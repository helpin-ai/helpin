import { useCallback, useEffect, useMemo, useState } from 'react';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { workspacesService } from '@/lib/services/workspacesService';
import { findWorkspaceWebsiteContentSource, buildWorkspaceWebsiteContentSourcePayload } from '@/lib/workspaceWebsiteSource';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Favicon } from '@/components/ui/favicon';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { cn } from '@/lib/utils';
import { Camera, ChevronRight, Globe, Loader2, Search, Trash2 } from 'lucide-react';
import { useNavigate } from '@tanstack/react-router';
import { useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { useCreateSupportContentSource, useSupportContentSources } from '@/hooks/queries/useSupport';
import { LINEAR_CARD_CLASS } from './settingsConstants';

const TIMEZONE_LIST: { id: string; offset: string; searchKey: string }[] = (() => {
  const names = Intl.supportedValuesOf('timeZone');
  const now = new Date();
  return names.map((tz) => {
    const fmt = new Intl.DateTimeFormat('en-US', { timeZone: tz, timeZoneName: 'shortOffset' });
    const parts = fmt.formatToParts(now);
    const gmtStr = parts.find((p) => p.type === 'timeZoneName')?.value ?? '';
    const offset = gmtStr === 'GMT' ? 'UTC+00:00' : gmtStr.replace('GMT', 'UTC');
    return { id: tz, offset, searchKey: `${tz} ${offset}`.toLowerCase() };
  });
})();

export function GeneralTab({ workspaceId, editable }: {
  workspaceId: string;
  editable: boolean;
}) {
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const { data: contentSources = [] } = useSupportContentSources(workspaceId);
  const createWebsiteSource = useCreateSupportContentSource(workspaceId);
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [name, setName] = useState(workspace?.name ?? '');
  const [description, setDescription] = useState(workspace?.description ?? '');
  const [websiteUrl, setWebsiteUrl] = useState(workspace?.website_url ?? '');
  const [workspaceKeyInput, setWorkspaceKeyInput] = useState(workspace?.workspace_key ?? '');
  const [timezone, setTimezone] = useState(workspace?.timezone ?? 'UTC');
  const [logoUrl, setLogoUrl] = useState(workspace?.logo_url ?? '');
  const [uploadingLogo, setUploadingLogo] = useState(false);
  const [saving, setSaving] = useState(false);
  const [tzSearch, setTzSearch] = useState('');
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [deleteConfirmText, setDeleteConfirmText] = useState('');
  const [deleting, setDeleting] = useState(false);

  useEffect(() => {
    setName(workspace?.name ?? '');
    setDescription(workspace?.description ?? '');
    setWebsiteUrl(workspace?.website_url ?? '');
    setWorkspaceKeyInput(workspace?.workspace_key ?? '');
    setLogoUrl(workspace?.logo_url ?? '');
    setTimezone(workspace?.timezone ?? 'UTC');
  }, [workspace?.id, workspace?.updated_at]);

  const savedWebsiteUrl = workspace?.website_url;
  const websiteContentSource = useMemo(
    () => findWorkspaceWebsiteContentSource(savedWebsiteUrl, contentSources),
    [savedWebsiteUrl, contentSources],
  );

  const selectedTz = useMemo(() => TIMEZONE_LIST.find((tz) => tz.id === timezone), [timezone]);

  const formatNow = useCallback(() =>
    new Intl.DateTimeFormat('en-US', {
      timeZone: timezone,
      month: 'short', day: 'numeric',
      hour: 'numeric', minute: '2-digit',
      hour12: true,
    }).format(new Date()),
    [timezone],
  );
  const [currentTime, setCurrentTime] = useState(formatNow);
  useEffect(() => {
    setCurrentTime(formatNow());
    const id = setInterval(() => setCurrentTime(formatNow()), 60_000);
    return () => clearInterval(id);
  }, [formatNow]);

  const filteredTimezones = useMemo(() => {
    if (!tzSearch) return TIMEZONE_LIST;
    const q = tzSearch.toLowerCase();
    return TIMEZONE_LIST.filter((tz) => tz.searchKey.includes(q));
  }, [tzSearch]);

  const handleLogoUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    if (!file.type.startsWith('image/')) {
      toast.error('Please select an image file');
      return;
    }
    if (file.size > 2 * 1024 * 1024) {
      toast.error('Image must be under 2MB');
      return;
    }
    setUploadingLogo(true);
    const { data, error } = await workspacesService.uploadLogo(workspaceId, file);
    setUploadingLogo(false);
    e.target.value = '';
    if (error || !data) {
      toast.error(error ?? 'Upload failed');
      return;
    }
    toast.success('Logo updated');
    setLogoUrl(data.logo_url ?? '');
    useWorkspaceStore.getState().setCurrentWorkspace(data);
  };

  const handleRemoveLogo = async () => {
    setSaving(true);
    const { data, error } = await workspacesService.deleteLogo(workspaceId);
    setSaving(false);
    if (error) {
      toast.error(error);
    } else {
      toast.success('Logo removed');
      setLogoUrl('');
      if (data) useWorkspaceStore.getState().setCurrentWorkspace(data);
    }
  };

  const handleSave = async () => {
    if (!name.trim()) {
      toast.error('Workspace name is required');
      return;
    }
    setSaving(true);
    const updates: Record<string, unknown> = {
      name: name.trim(),
      description: description.trim() || undefined,
      website_url: websiteUrl.trim(),
      timezone,
    };
    const trimmedKey = workspaceKeyInput.trim().toUpperCase();
    if (trimmedKey && trimmedKey !== workspace?.workspace_key) {
      updates.workspace_key = trimmedKey;
    }
    const { data, error } = await workspacesService.update(workspaceId, updates);
    setSaving(false);
    if (error) {
      toast.error(error);
    } else {
      toast.success('Workspace updated');
      if (data) {
        useWorkspaceStore.getState().setCurrentWorkspace(data);
      }
    }
  };

  const handleDelete = async () => {
    setDeleting(true);
    const { error } = await workspacesService.delete(workspaceId);
    setDeleting(false);
    if (error) {
      toast.error(error);
      return;
    }
    toast.success('Workspace deleted');
    queryClient.invalidateQueries({ queryKey: ['workspaces'] });
    navigate({ to: '/workspaces' });
  };

  const handleAddWebsiteSource = async () => {
    if (!workspace || !workspace.website_url) {
      return;
    }

    await createWebsiteSource.mutateAsync(
      buildWorkspaceWebsiteContentSourcePayload(workspace.name, workspace.website_url),
    );
    toast.success('Website source added and syncing');
  };

  const openKnowledgeSettings = () => {
    if (!workspace || !workspace.slug) {
      return;
    }
    navigate({
      to: '/w/$slug/settings/$section',
      params: { slug: workspace.slug, section: 'knowledge' },
    });
  };

  const websiteSourceStatusLabel = useMemo(() => {
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
  }, [websiteContentSource?.sync_status]);

  return (
    <div className="space-y-6">
      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader>
          <CardTitle>General</CardTitle>
        </CardHeader>
        <CardContent className="space-y-6">
          <div className="space-y-2">
            <Label>Logo</Label>
            <div className="flex items-center gap-4">
              <div className="relative group">
                <Favicon
                  src={logoUrl || undefined}
                  url={websiteUrl || workspace?.website_url}
                  name={name || workspace?.name}
                  size={128}
                  className="h-16 w-16 rounded-lg"
                  fallbackClassName="text-xl"
                />
                {editable && (
                  <label className="absolute inset-0 flex items-center justify-center rounded-lg bg-black/50 opacity-0 group-hover:opacity-100 transition-opacity cursor-pointer">
                    {uploadingLogo ? (
                      <Loader2 className="h-5 w-5 text-white animate-spin" />
                    ) : (
                      <Camera className="h-5 w-5 text-white" />
                    )}
                    <input
                      type="file"
                      accept="image/*"
                      className="hidden"
                      onChange={handleLogoUpload}
                      disabled={uploadingLogo}
                    />
                  </label>
                )}
              </div>
              <div className="space-y-1">
                <p className="text-sm text-muted-foreground">
                  Upload a logo for your workspace. Recommended size: 128x128px.
                </p>
                {editable && logoUrl && (
                  <Button variant="ghost" size="sm" className="h-7 text-xs text-destructive hover:text-destructive" onClick={handleRemoveLogo} disabled={saving}>
                    Remove logo
                  </Button>
                )}
              </div>
            </div>
          </div>

          <div className="space-y-2">
            <Label htmlFor="ws-name">Workspace Name</Label>
            <Input
              id="ws-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              disabled={!editable}
              placeholder="My Workspace"
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="ws-key">Task Key Prefix</Label>
            <Input
              id="ws-key"
              value={workspaceKeyInput}
              onChange={(e) => setWorkspaceKeyInput(e.target.value.replace(/[^a-zA-Z]/g, '').toUpperCase().slice(0, 5))}
              disabled={!editable}
              placeholder="ACM"
              maxLength={5}
            />
            <p className="text-xs text-muted-foreground">
              2-5 uppercase letters used in task identifiers (e.g. {workspace?.workspace_key || '...'}-123).
              {workspace?.workspace_key && ' Changing this will update new task keys. Old references will continue to work.'}
            </p>
          </div>

          <div className="space-y-2">
            <Label htmlFor="ws-desc">Description</Label>
            <Textarea
              id="ws-desc"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              disabled={!editable}
              placeholder="A brief description of this workspace"
              rows={3}
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="ws-website">Website</Label>
            <Input
              id="ws-website"
              type="url"
              value={websiteUrl}
              onChange={(e) => setWebsiteUrl(e.target.value)}
              disabled={!editable}
              placeholder="https://acme.com"
            />
            {!savedWebsiteUrl && (
              <p className="text-xs text-muted-foreground">
                Optional public website for this workspace. We normalize bare domains to `https://...`.
              </p>
            )}
            {savedWebsiteUrl && (
              <div className="rounded-lg border border-border/70 bg-muted/20 p-3">
                <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                  <div className="space-y-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <p className="text-sm font-medium">Workspace website</p>
                      {websiteSourceStatusLabel && <Badge variant="secondary">{websiteSourceStatusLabel}</Badge>}
                    </div>
                    {websiteContentSource ? (
                      <p className="text-xs text-muted-foreground">
                        This website is connected as a Website Content Source. You can manage sync and Support AI access from Knowledge.
                      </p>
                    ) : (
                      <p className="text-xs text-muted-foreground">
                        This website is saved for workspace identity, but it is not yet connected as a Website Content Source for Support AI.
                      </p>
                    )}
                  </div>
                  <div className="flex flex-wrap items-center gap-2">
                    {!websiteContentSource && editable && (
                      <Button
                        type="button"
                        size="sm"
                        onClick={() => void handleAddWebsiteSource()}
                        disabled={createWebsiteSource.isPending}
                      >
                        {createWebsiteSource.isPending ? 'Adding...' : 'Add as Source'}
                      </Button>
                    )}
                    <Button type="button" size="sm" variant="outline" onClick={openKnowledgeSettings}>
                      Open Knowledge
                    </Button>
                  </div>
                </div>
              </div>
            )}
          </div>

          <div className="space-y-2">
            <Label htmlFor="ws-tz">Timezone</Label>
            <p className="text-xs text-muted-foreground">
              Used for sprint boundaries, due dates, and reporting. All members see the same deadlines.
            </p>
            <Popover>
              <PopoverTrigger asChild>
                <Button variant="outline" className="w-full justify-between font-normal" disabled={!editable}>
                  <span className="flex items-center gap-2">
                    <Globe className="h-4 w-4 text-muted-foreground" />
                    {timezone}
                    {selectedTz && <span className="text-muted-foreground">({selectedTz.offset})</span>}
                  </span>
                  <ChevronRight className="h-4 w-4 text-muted-foreground" />
                </Button>
              </PopoverTrigger>
              <PopoverContent className="w-[320px] p-0" align="start">
                <div className="p-2 border-b">
                  <div className="flex items-center gap-2 px-2">
                    <Search className="h-4 w-4 text-muted-foreground shrink-0" />
                    <input
                      className="flex-1 bg-transparent text-sm outline-none placeholder:text-muted-foreground"
                      placeholder="Search timezones..."
                      value={tzSearch}
                      onChange={(e) => setTzSearch(e.target.value)}
                    />
                  </div>
                </div>
                <div className="max-h-[280px] overflow-y-auto p-1">
                  {filteredTimezones.length === 0 ? (
                    <p className="py-4 text-center text-xs text-muted-foreground">No timezones found</p>
                  ) : (
                    filteredTimezones.map((tz) => (
                      <button
                        key={tz.id}
                        type="button"
                        className={cn(
                          'w-full rounded-sm px-2 py-1.5 text-left text-sm hover:bg-accent flex items-center justify-between',
                          tz.id === timezone && 'bg-accent font-medium',
                        )}
                        onClick={() => { setTimezone(tz.id); setTzSearch(''); }}
                      >
                        <span>{tz.id}</span>
                        <span className="text-xs text-muted-foreground ml-2 shrink-0">{tz.offset}</span>
                      </button>
                    ))
                  )}
                </div>
              </PopoverContent>
            </Popover>
            <p className="text-xs text-muted-foreground">
              Current date and time: <span className="font-medium text-foreground">{currentTime}</span>
            </p>
          </div>

          {editable && (
            <div className="flex justify-end">
              <Button onClick={handleSave} disabled={saving}>
                {saving ? 'Saving...' : 'Save'}
              </Button>
            </div>
          )}
        </CardContent>
      </Card>
      {editable && (
        <Card className={cn(LINEAR_CARD_CLASS, 'border-destructive/30')}>
          <CardHeader>
            <CardTitle className="text-destructive">Danger Zone</CardTitle>
            <CardDescription>
              Irreversible actions that permanently affect this workspace.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium">Delete this workspace</p>
                <p className="text-xs text-muted-foreground">
                  Permanently delete this workspace and all of its data including tasks, epics, sprints, attachments, and settings. This action cannot be undone.
                </p>
              </div>
              <Button variant="destructive" onClick={() => setDeleteOpen(true)}>
                <Trash2 className="h-4 w-4 mr-2" />
                Delete
              </Button>
            </div>
          </CardContent>
        </Card>
      )}

      <Dialog open={deleteOpen} onOpenChange={(open) => { setDeleteOpen(open); if (!open) setDeleteConfirmText(''); }}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete workspace</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <p className="text-sm text-muted-foreground">
              This will permanently delete <span className="font-semibold text-foreground">{workspace?.name}</span> and all of its data including tasks, epics, sprints, comments, attachments, and settings. This action cannot be undone.
            </p>
            <div className="space-y-2">
              <Label htmlFor="delete-confirm">
                Type <span className="font-mono font-semibold text-destructive">{workspace?.slug}</span> to confirm
              </Label>
              <Input
                id="delete-confirm"
                value={deleteConfirmText}
                onChange={(e) => setDeleteConfirmText(e.target.value)}
                placeholder={workspace?.slug ?? ''}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setDeleteOpen(false); setDeleteConfirmText(''); }}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              disabled={deleteConfirmText.trim() !== workspace?.slug || deleting}
              onClick={handleDelete}
            >
              {deleting ? <><Loader2 className="h-4 w-4 mr-2 animate-spin" />Deleting...</> : 'Delete workspace'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
