import { billingEnabled } from '@edition/config';
import { SettingsSaveBar } from './SettingsSaveBar';
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
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { cn } from '@/lib/utils';
import { Camera01Icon, ArrowRight01Icon, GlobeIcon, Loading01Icon, Delete01Icon } from '@/lib/icons';
import { QuietSearchInput } from '@/components/design-system/quiet';
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

export function GeneralTab({ workspaceId, editable, canDeleteWorkspace }: {
  workspaceId: string;
  editable: boolean;
  /** Only the workspace owner can delete it; the server enforces the same rule. */
  canDeleteWorkspace: boolean;
}) {
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const { data: contentSources = [] } = useSupportContentSources(workspaceId);
  const createWebsiteSource = useCreateSupportContentSource(workspaceId);
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [name, setName] = useState(workspace?.name ?? '');
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
  const [keyChangeOpen, setKeyChangeOpen] = useState(false);
  const [pendingKey, setPendingKey] = useState('');
  const [keyHistory, setKeyHistory] = useState<{ old_key: string; new_key: string; changed_at: string }[]>([]);

  useEffect(() => {
    setName(workspace?.name ?? '');
    setWebsiteUrl(workspace?.website_url ?? '');
    setWorkspaceKeyInput(workspace?.workspace_key ?? '');
    setLogoUrl(workspace?.logo_url ?? '');
    setTimezone(workspace?.timezone ?? 'UTC');
  }, [workspace?.id, workspace?.updated_at]);

  useEffect(() => {
    if (!workspaceId) return;
    workspacesService.getKeyHistory(workspaceId).then((res) => {
      if (res.data) {
        setKeyHistory(res.data.map((h) => ({ old_key: h.old_key, new_key: h.new_key, changed_at: h.changed_at })));
      }
    });
  }, [workspaceId, workspace?.updated_at]);

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
    const trimmedKey = workspaceKeyInput.trim().toUpperCase();
    if (trimmedKey && trimmedKey !== workspace?.workspace_key) {
      setPendingKey(trimmedKey);
      setKeyChangeOpen(true);
      return;
    }
    await performSave();
  };

  const performSave = async (keyOverride?: string) => {
    setSaving(true);
    const updates: Record<string, unknown> = {
      name: name.trim(),
      website_url: websiteUrl.trim(),
      timezone,
    };
    const trimmedKey = (keyOverride || workspaceKeyInput.trim().toUpperCase());
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

  const confirmKeyChange = async () => {
    setKeyChangeOpen(false);
    await performSave(pendingKey);
    setPendingKey('');
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
      {editable && (
        <SettingsSaveBar>
          <Button onClick={handleSave} disabled={saving}>
            {saving ? 'Saving...' : 'Save'}
          </Button>
        </SettingsSaveBar>
      )}
      <Card className={LINEAR_CARD_CLASS}>
        <CardContent className="space-y-6 pt-6">
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
                      <Loading01Icon className="h-5 w-5 text-white animate-spin" />
                    ) : (
                      <Camera01Icon className="h-5 w-5 text-white" />
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

          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
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
              <div className="flex items-center gap-1.5">
                <Label htmlFor="ws-key">Task Key Prefix</Label>
                <TooltipProvider>
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <span className="text-muted-foreground cursor-help">
                        <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><circle cx="12" cy="12" r="10"/><path d="M9.09 9a3 3 0 0 1 5.83 1c0 2-3 3-3 3"/><path d="M12 17h.01"/></svg>
                      </span>
                    </TooltipTrigger>
                    <TooltipContent>
                      <p className="max-w-[200px]">2-5 uppercase letters used in task IDs (e.g. {workspace?.workspace_key || 'ACM'}-123). Changing this updates new task keys; old references persist.</p>
                    </TooltipContent>
                  </Tooltip>
                </TooltipProvider>
              </div>
              <Input
                id="ws-key"
                value={workspaceKeyInput}
                onChange={(e) => setWorkspaceKeyInput(e.target.value.replace(/[^a-zA-Z]/g, '').toUpperCase().slice(0, 5))}
                disabled={!editable}
                placeholder="ACM"
                maxLength={5}
              />
              {keyHistory.length > 0 && (
                <div className="flex flex-wrap items-center gap-1.5">
                  <span className="text-[11px] text-muted-foreground">Previously:</span>
                  {keyHistory.map((h, i) => (
                    <Badge key={i} variant="outline" className="text-[11px]">{h.old_key}</Badge>
                  ))}
                </div>
              )}
            </div>
          </div>

          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
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
                  Optional public website. Bare domains are normalized to `https://...`.
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
                          Connected as a Website Content Source. Manage sync and Support AI access from Knowledge.
                        </p>
                      ) : (
                        <p className="text-xs text-muted-foreground">
                          Saved for identity, but not yet connected as a Content Source for Support AI.
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
              <div className="flex items-center gap-1.5">
                <Label htmlFor="ws-tz">Timezone</Label>
                <TooltipProvider>
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <span className="text-muted-foreground cursor-help">
                        <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><circle cx="12" cy="12" r="10"/><path d="M9.09 9a3 3 0 0 1 5.83 1c0 2-3 3-3 3"/><path d="M12 17h.01"/></svg>
                      </span>
                    </TooltipTrigger>
                    <TooltipContent>
                      <p className="max-w-[200px]">Used for sprint boundaries, due dates, and reporting. All members see the same deadlines.</p>
                    </TooltipContent>
                  </Tooltip>
                </TooltipProvider>
              </div>
              <Popover>
                <PopoverTrigger asChild>
                  <Button variant="outline" className="w-full justify-between font-normal" disabled={!editable}>
                    <span className="flex items-center gap-2">
                      <GlobeIcon className="h-4 w-4 text-muted-foreground" />
                      {timezone}
                      {selectedTz && <span className="text-muted-foreground">({selectedTz.offset})</span>}
                    </span>
                    <ArrowRight01Icon className="h-4 w-4 text-muted-foreground" />
                  </Button>
                </PopoverTrigger>
                <PopoverContent className="w-[320px] p-0" align="start">
                  <div className="border-b p-2">
                      <QuietSearchInput
                        placeholder="Search timezones..."
                        value={tzSearch}
                        onChange={(e) => setTzSearch(e.target.value)}
                      />
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
            </div>
          </div>

          <p className="text-xs text-muted-foreground">
            Current date and time: <span className="font-medium text-foreground">{currentTime}</span>
          </p>


        </CardContent>
      </Card>
      {canDeleteWorkspace && (
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
                <Delete01Icon className="h-4 w-4 mr-2" />
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
            <p className="text-sm text-muted-foreground">
              {billingEnabled ? 'If this workspace has an active subscription, deleting it will cancel the subscription immediately. Past invoices and payment records remain available in billing records.' : 'Deleting this workspace removes its members, settings, and data.'}
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
              {deleting ? <><Loading01Icon className="h-4 w-4 mr-2 animate-spin" />Deleting...</> : 'Delete workspace'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={keyChangeOpen} onOpenChange={(open) => { setKeyChangeOpen(open); if (!open) setPendingKey(''); }}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Change workspace key</DialogTitle>
            <DialogDescription>
              Changing from <span className="font-semibold text-foreground">{workspace?.workspace_key}</span> to <span className="font-semibold text-foreground">{pendingKey}</span>.
            </DialogDescription>
          </DialogHeader>
          <p className="text-sm text-muted-foreground">
            Existing references like <span className="font-mono">{workspace?.workspace_key}-123</span> in branches, bookmarks, and docs will continue to work.
            New task keys will use <span className="font-mono">{pendingKey}-123</span>.
          </p>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setKeyChangeOpen(false); setPendingKey(''); setWorkspaceKeyInput(workspace?.workspace_key ?? ''); }}>
              Cancel
            </Button>
            <Button onClick={confirmKeyChange} disabled={saving}>
              {saving ? 'Saving...' : 'Confirm change'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
