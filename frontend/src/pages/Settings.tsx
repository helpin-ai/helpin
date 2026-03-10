import { useCallback, useEffect, useMemo, useState, type FormEvent } from 'react';
import { ShortcutImportWizard } from '@/components/pm/ShortcutImportWizard';
import { useCopyToClipboard } from '@/hooks/useCopyToClipboard';
import { useTitle } from '@/hooks/useTitle';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { invalidateWorkspaceTeamsCache } from '@/hooks/useWorkspaceTeams';
import { useWorkspaceAccess, usePermissions } from '@/hooks/queries';
import { settingsService } from '@/lib/services/settingsService';
import { gitService } from '@/lib/services/gitService';
import { agentService } from '@/lib/services/agentService';
import { workspacesService } from '@/lib/services/workspacesService';
import { inviteService } from '@/lib/services/inviteService';
import type { WorkspaceSettings, WorkspaceTeam, WorkspacePerson, JobRoleCriteria, BonusTierConfig, MemberWithUser, Invitation, TeamUserMembership, InvitationTeamPreassignment, TeamEstimateSettings, TeamFieldVisibility, EstimateScale, TeamRepoDefault } from '@/lib/types';
import { pmWorkflowService } from '@/lib/services/pmWorkflowService';
import { pmAutomationService } from '@/lib/services/pmAutomationService';
import { StateTypeIcon } from '@/lib/pmConstants';
import { LabelsSettings } from '@/components/pm/LabelsSettings';
import { StoryTemplatesSettings } from '@/components/pm/StoryTemplatesSettings';
import { UserAvatar, getAvatarColor } from '@/components/pm/UserAvatar';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import type { StateType, WorkflowState, WorkflowWithStates, EpicWorkflowState, PMAutomation, AutomationType, GitIntegration, GitRepository, RunnerHealth } from '@/lib/pmTypes';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Skeleton } from '@/components/ui/skeleton';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Textarea } from '@/components/ui/textarea';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { cn, getInitials } from '@/lib/utils';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { ArrowDown, ArrowUp, Bot, Camera, ChevronRight, Copy, Eye, FileText, FolderKanban, GitBranch, GitPullRequest, Globe, Import, Info, LayoutGrid, ListTree, Loader2, Mail, Pencil, Plus, RefreshCw, Search, Server, Settings2, Sliders, Tag, Trash2, Users, X, Zap, type LucideIcon } from 'lucide-react';
import { Separator } from '@/components/ui/separator';
import { SCALE_LABELS, SCALE_DESCRIPTIONS, getEstimateOptions } from '@/lib/estimateScales';
import { useNavigate } from '@tanstack/react-router';
import { toast } from 'sonner';
import { PipelineSettings } from '@/components/crm/PipelineSettings';
import { EmailAccountConnect } from '@/components/crm/EmailAccountConnect';
import { useAuthStore } from '@/stores/authStore';
import { useAutonomySettings, useUpdateAutonomySettings } from '@/hooks/queries/useCRM';

export type SettingsSection = 'general' | 'members' | 'teams' | 'notifications' | 'people' | 'jobroles' | 'tiers' | 'workflows' | 'workflowstates' | 'labels' | 'story-templates' | 'automations' | 'delivery' | 'ai' | 'import' | 'helpcenter' | 'crm-pipelines' | 'crm-email' | 'crm-autonomy' | 'system' | 'account';

export const SETTINGS_SECTIONS: { id: SettingsSection; label: string; description: string; icon: LucideIcon; group: string }[] = [
  {
    id: 'general',
    label: 'General',
    description: '',
    icon: Settings2,
    group: 'Workspace',
  },
  {
    id: 'members',
    label: 'Members',
    description: '',
    icon: Users,
    group: 'Workspace',
  },
  {
    id: 'teams',
    label: 'Teams',
    description: '',
    icon: Users,
    group: 'Workspace',
  },
  {
    id: 'workflows',
    label: 'Workflows',
    description: '',
    icon: GitBranch,
    group: 'Project Settings',
  },
  {
    id: 'workflowstates',
    label: 'Workflow States',
    description: '',
    icon: ListTree,
    group: 'Project Settings',
  },
  {
    id: 'labels',
    label: 'Labels',
    description: '',
    icon: Tag,
    group: 'Project Settings',
  },
  {
    id: 'story-templates',
    label: 'Story Templates',
    description: 'Define reusable templates for quick story creation.',
    icon: FileText,
    group: 'Project Settings',
  },
  {
    id: 'automations',
    label: 'Automations',
    description: '',
    icon: RefreshCw,
    group: 'Project Settings',
  },
  {
    id: 'delivery',
    label: 'Delivery',
    description: 'Connect GitHub, curate repositories, and monitor shared runner pools.',
    icon: Globe,
    group: 'Project Settings',
  },
  {
    id: 'ai',
    label: 'AI',
    description: 'Choose the workspace planning methodology used for epic PRD and story planning.',
    icon: Bot,
    group: 'Project Settings',
  },
  {
    id: 'import',
    label: 'Import / Export',
    description: 'Import data from Shortcut and other project management tools.',
    icon: Import,
    group: 'Data',
  },
  {
    id: 'helpcenter',
    label: 'Help Center',
    description: 'Configure your public help center branding, domain, and SEO.',
    icon: Globe,
    group: 'Docs',
  },
  {
    id: 'crm-pipelines',
    label: 'Pipelines',
    description: 'Configure deal pipelines and stages.',
    icon: FolderKanban,
    group: 'CRM Settings',
  },
  {
    id: 'crm-email',
    label: 'Email Accounts',
    description: 'Connect Gmail to sync conversations and detect buyer signals.',
    icon: Mail,
    group: 'CRM Settings',
  },
  {
    id: 'crm-autonomy',
    label: 'Autonomy',
    description: 'Configure self-driving deal automation thresholds.',
    icon: Sliders,
    group: 'CRM Settings',
  },
  /* {
    id: 'people',
    label: 'People',
    description: '',
    icon: UserPlus,
    group: 'Reward Settings',
  },
  {
    id: 'jobroles',
    label: 'Job Roles',
    description: '',
    icon: Briefcase,
    group: 'Reward Settings',
  },
  {
    id: 'tiers',
    label: 'Bonus Tiers',
    description: '',
    icon: Award,
    group: 'Reward Settings',
  },
  {
    id: 'system',
    label: 'Reward Defaults',
    description: '',
    icon: Settings2,
    group: 'Reward Settings',
  }, */
];

export const isSettingsSection = (value: string): value is SettingsSection =>
  SETTINGS_SECTIONS.some((section) => section.id === value) || value === 'account';

const LINEAR_CARD_CLASS = 'rounded-none border-border shadow-none dark:border-transparent';

const slugifyTeamHandle = (value: string) =>
  value
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');

export default function Settings({ section, initialWorkflowId, initialTeamId }: { section: SettingsSection; initialWorkflowId?: string; initialTeamId?: string }) {
  useTitle('Settings');
  const { currentWorkspace } = useWorkspaceStore();
  const wsId = currentWorkspace?.id ?? '';
  const { data: access } = useWorkspaceAccess(wsId);
  const { canManageSettings, canManageMembers, canManageTeams, canAdminWorkflows, canAdminLabels, canAdminAutomations, canImport } = usePermissions(access);
  const [settings, setSettings] = useState<WorkspaceSettings | null>(null);
  const [loading, setLoading] = useState(true);

  const load = async (silent = false) => {
    const ws = useWorkspaceStore.getState().currentWorkspace;
    if (!ws?.id) {
      setSettings(null);
      if (!silent) setLoading(false);
      return;
    }
    if (!silent) setLoading(true);
    try {
      const { data } = await settingsService.getAll(ws.id);
      if (data) {
        setSettings(data);
        invalidateWorkspaceTeamsCache();
      }
    } finally {
      if (!silent) setLoading(false);
    }
  };

  useEffect(() => { load(); }, [currentWorkspace?.id]);

  if (loading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-10 w-48" />
        <Skeleton className="h-96" />
      </div>
    );
  }

  if (!settings) {
    return (
      <div className="text-center py-12">
        <p className="text-muted-foreground">Could not load workspace settings.</p>
      </div>
    );
  }

  const workspaceId = currentWorkspace?.id ?? settings.settings.workspace_id;
  if (!workspaceId) {
    return (
      <div className="text-center py-12">
        <p className="text-muted-foreground">Could not determine workspace for settings.</p>
      </div>
    );
  }

  const sectionMeta = SETTINGS_SECTIONS.find((candidate) => candidate.id === section)!;

  const renderSection = () => {
    switch (section) {
      case 'general':
        return (
          <GeneralTab
            workspaceId={workspaceId}
            editable={canManageSettings}
          />
        );
      case 'members':
        return (
          <MembersTab
            workspaceId={workspaceId}
            editable={canManageMembers}
          />
        );
      case 'teams':
        return (
          <TeamsTab
            workspaceId={workspaceId}
            teams={settings.teams}
            userMemberships={settings.user_memberships}
            invitationPreassignments={settings.invitation_team_preassignments}
            teamEstimateSettings={settings.team_estimate_settings}
            teamFieldVisibility={settings.team_field_visibility}
            teamRepoDefaults={settings.team_repo_defaults}
            editable={canManageTeams}
            onRefresh={load}
          />
        );
      case 'people':
        return (
          <PeopleTab
            people={settings.people}
            editable={canManageMembers}
            onRefresh={load}
          />
        );
      case 'jobroles':
        return (
          <JobRolesTab
            workspaceId={workspaceId}
            criteria={settings.job_role_criteria}
            editable={canManageSettings}
            onRefresh={load}
          />
        );
      case 'tiers':
        return (
          <BonusTiersTab
            workspaceId={workspaceId}
            tiers={settings.bonus_tiers}
            editable={canManageSettings}
            onRefresh={load}
          />
        );
      case 'system':
        return (
          <SystemTab
            workspaceId={workspaceId}
            config={settings.settings}
            editable={canManageSettings}
            onRefresh={load}
          />
        );
      case 'delivery':
        return (
          <ProjectDeliveryTab
            workspaceId={workspaceId}
            editable={canManageSettings}
          />
        );
      case 'ai':
        return (
          <AITab
            workspaceId={workspaceId}
            config={settings.settings}
            editable={canManageSettings}
            onRefresh={load}
          />
        );
      case 'workflows':
        return (
          <WorkflowsTab
            workspaceId={workspaceId}
            teams={settings.teams}
            editable={canAdminWorkflows}
            initialTeamId={initialTeamId}
          />
        );
      case 'workflowstates':
        return (
          <WorkflowStatesTab
            workspaceId={workspaceId}
            editable={canAdminWorkflows}
            initialWorkflowId={initialWorkflowId}
          />
        );
      case 'labels':
        return <LabelsSettings workspaceId={workspaceId} initialTeamId={initialTeamId} editable={canAdminLabels} />;
      case 'story-templates':
        return <StoryTemplatesSettings workspaceId={workspaceId} initialTeamId={initialTeamId} />;
      case 'automations':
        return <AutomationsTab workspaceId={workspaceId} teams={settings.teams} editable={canAdminAutomations} />;
      case 'import':
        return <ImportTab workspaceId={workspaceId} editable={canImport} />;
      case 'helpcenter':
        return <HelpcenterTab workspaceId={workspaceId} />;
      case 'crm-pipelines':
        return <PipelineSettings />;
      case 'crm-email':
        return <CRMEmailSettingsTab workspaceId={workspaceId} />;
      case 'crm-autonomy':
        return <CRMAutonomySettingsTab workspaceId={workspaceId} />;
      default:
        return null;
    }
  };

  return (
    <div className="space-y-4">
      {section !== 'teams' && (
        <div>
          <h2 className="text-xl font-semibold">{sectionMeta.label}</h2>
          {sectionMeta.description && (
            <p className="text-sm text-muted-foreground">{sectionMeta.description}</p>
          )}
        </div>
      )}
      {renderSection()}
    </div>
  );
}

/* ============ General Tab ============ */

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

function GeneralTab({ workspaceId, editable }: {
  workspaceId: string;
  editable: boolean;
}) {
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const navigate = useNavigate();
  const [name, setName] = useState(workspace?.name ?? '');
  const [description, setDescription] = useState(workspace?.description ?? '');
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
    setLogoUrl(workspace?.logo_url ?? '');
    setTimezone(workspace?.timezone ?? 'UTC');
  }, [workspace?.id, workspace?.updated_at]);

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
    const { data, error } = await workspacesService.update(workspaceId, {
      name: name.trim(),
      description: description.trim() || undefined,
      timezone,
    });
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
    navigate({ to: '/' });
  };

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
                <UserAvatar
                  name={name || workspace?.name}
                  avatarUrl={logoUrl || undefined}
                  className="h-16 w-16 rounded-lg"
                  fallbackClassName="text-xl rounded-lg"
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
                  Permanently delete this workspace and all of its data including stories, epics, sprints, attachments, and settings. This action cannot be undone.
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
              This will permanently delete <span className="font-semibold text-foreground">{workspace?.name}</span> and all of its data including stories, epics, sprints, comments, attachments, and settings. This action cannot be undone.
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
              disabled={deleteConfirmText !== workspace?.slug || deleting}
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

/* ============ Members Tab ============ */

function MembersTab({ workspaceId, editable }: {
  workspaceId: string;
  editable: boolean;
}) {
  const [members, setMembers] = useState<MemberWithUser[]>([]);
  const [invitations, setInvitations] = useState<Invitation[]>([]);
  const [loading, setLoading] = useState(true);
  const [inviteOpen, setInviteOpen] = useState(false);
  const [invEmail, setInvEmail] = useState('');
  const [invRole, setInvRole] = useState('member');
  const [sending, setSending] = useState(false);
  const [createdJoinUrl, setCreatedJoinUrl] = useState<string | null>(null);

  const loadData = async () => {
    setLoading(true);
    const [membersRes, invitationsRes] = await Promise.all([
      workspacesService.listMembers(workspaceId),
      editable ? inviteService.list(workspaceId) : Promise.resolve({ data: null, error: null }),
    ]);
    if (membersRes.data) setMembers(membersRes.data);
    if (invitationsRes.data) setInvitations(invitationsRes.data);
    setLoading(false);
  };

  useEffect(() => { loadData(); }, [workspaceId]);

  const openInviteDialog = () => {
    setCreatedJoinUrl(null);
    setInvEmail('');
    setInvRole('member');
    setInviteOpen(true);
  };

  const closeInviteDialog = () => {
    setInviteOpen(false);
    if (createdJoinUrl) loadData();
  };

  const handleInvite = async (e: FormEvent) => {
    e.preventDefault();
    setSending(true);
    const { data, error } = await inviteService.send({ workspace_id: workspaceId, email: invEmail, role: invRole });
    setSending(false);
    if (error) {
      toast.error(error);
    } else {
      toast.success(`Invitation sent to ${invEmail}`);
      if (data?.join_url) {
        setCreatedJoinUrl(data.join_url);
      } else {
        setInviteOpen(false);
        loadData();
      }
    }
  };

  const { copy: copyToClipboard } = useCopyToClipboard();
  const handleCopyLink = (joinUrl: string) => {
    copyToClipboard(joinUrl);
    toast.success('Invite link copied to clipboard');
  };

  const handleResend = async (id: string) => {
    const { error } = await inviteService.resend(id, workspaceId);
    if (error) toast.error(error);
    else toast.success('Invitation resent');
  };

  const handleRevoke = async (id: string) => {
    const { error } = await inviteService.revoke(id, workspaceId);
    if (error) toast.error(error);
    else {
      toast.success('Invitation revoked');
      setInvitations((prev) => prev.filter((inv) => inv.id !== id));
    }
  };

  if (loading) return <Skeleton className="h-96" />;

  return (
    <Card className={LINEAR_CARD_CLASS}>
      <CardHeader>
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <CardTitle className="text-base">Members</CardTitle>
            <Badge variant="outline" className="text-xs font-normal">{members.length}</Badge>
          </div>
          {editable && (
            <Button size="sm" onClick={openInviteDialog}>
              <Plus className="h-4 w-4 mr-1" /> Invite Member
            </Button>
          )}
        </div>
      </CardHeader>
      <CardContent>
        {members.length === 0 ? (
          <p className="text-sm text-muted-foreground text-center py-6">No members yet.</p>
        ) : (
          <div className="overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Email</TableHead>
                  <TableHead>Role</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {members.map((m) => (
                  <TableRow key={m.id}>
                    <TableCell className="font-medium">
                      <div className="flex items-center gap-2.5">
                        <UserAvatar name={m.full_name || m.email} className="h-7 w-7" fallbackClassName="text-[10px]" />
                        {m.full_name || '—'}
                      </div>
                    </TableCell>
                    <TableCell className="text-muted-foreground">{m.email}</TableCell>
                    <TableCell>
                      <Badge variant={m.role === 'owner' ? 'default' : 'outline'} className="text-xs">{m.role}</Badge>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}

        {/* Pending Invitations */}
        {editable && invitations.filter((inv) => inv.status === 'pending').length > 0 && (
          <div className="mt-6">
            <h4 className="text-sm font-medium mb-2">Pending Invitations</h4>
            <div className="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Email</TableHead>
                    <TableHead>Role</TableHead>
                    <TableHead>Sent</TableHead>
                    <TableHead className="w-24">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {invitations.filter((inv) => inv.status === 'pending').map((inv) => (
                    <TableRow key={inv.id}>
                      <TableCell className="font-medium">{inv.email}</TableCell>
                      <TableCell><Badge variant="outline" className="text-xs">{inv.role}</Badge></TableCell>
                      <TableCell className="text-muted-foreground">
                        {new Date(inv.created_at).toLocaleDateString()}
                      </TableCell>
                      <TableCell>
                        <div className="flex gap-1">
                          {inv.join_url && (
                            <QuickTooltip label="Copy invite link">
                              <Button size="icon" variant="ghost" onClick={() => handleCopyLink(inv.join_url!)}>
                                <Copy className="h-3.5 w-3.5" />
                              </Button>
                            </QuickTooltip>
                          )}
                          <QuickTooltip label="Resend">
                            <Button size="icon" variant="ghost" onClick={() => handleResend(inv.id)}>
                              <RefreshCw className="h-3.5 w-3.5" />
                            </Button>
                          </QuickTooltip>
                          <QuickTooltip label="Revoke">
                            <Button size="icon" variant="ghost" onClick={() => handleRevoke(inv.id)}>
                              <Trash2 className="h-3.5 w-3.5" />
                            </Button>
                          </QuickTooltip>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          </div>
        )}
      </CardContent>

      <Dialog open={inviteOpen} onOpenChange={closeInviteDialog}>
        <DialogContent>
          {createdJoinUrl ? (
            <>
              <DialogHeader>
                <DialogTitle>Invitation Sent</DialogTitle>
              </DialogHeader>
              <div className="space-y-4 py-4">
                <p className="text-sm text-muted-foreground">Share this link with <span className="font-medium text-foreground">{invEmail}</span> to join the workspace.</p>
                <div className="flex gap-2">
                  <Input value={createdJoinUrl} readOnly className="bg-muted text-xs" />
                  <Button type="button" variant="outline" size="icon" onClick={() => handleCopyLink(createdJoinUrl)}>
                    <Copy className="h-4 w-4" />
                  </Button>
                </div>
              </div>
              <DialogFooter>
                <Button type="button" onClick={closeInviteDialog}>Done</Button>
              </DialogFooter>
            </>
          ) : (
            <form onSubmit={handleInvite}>
              <DialogHeader>
                <DialogTitle>Invite Member</DialogTitle>
              </DialogHeader>
              <div className="space-y-4 py-4">
                <div className="space-y-2">
                  <Label>Email</Label>
                  <Input type="email" placeholder="colleague@example.com" value={invEmail} onChange={(e) => setInvEmail(e.target.value)} required />
                </div>
                <div className="space-y-2">
                  <Label>Role</Label>
                  <div className="space-y-2">
                    {([
                      { value: 'admin', label: 'Admin', description: 'Full access to all settings, members, billing, and workspace configuration.' },
                      { value: 'manager', label: 'Manager', description: 'Can manage teams, projects, sprints, and view performance data.' },
                      { value: 'member', label: 'Member', description: 'Can create and edit stories, epics, and participate in sprints.' },
                      { value: 'viewer', label: 'Viewer', description: 'Read-only access. Can view projects and dashboards but cannot make changes.' },
                    ] as const).map((role) => (
                      <button
                        key={role.value}
                        type="button"
                        onClick={() => setInvRole(role.value)}
                        className={cn(
                          'flex w-full items-start gap-3 rounded-md border p-3 text-left transition-colors',
                          invRole === role.value
                            ? 'border-primary bg-primary/5'
                            : 'border-border hover:bg-muted/50'
                        )}
                      >
                        <div className={cn(
                          'mt-0.5 h-4 w-4 shrink-0 rounded-full border-2',
                          invRole === role.value
                            ? 'border-primary bg-primary'
                            : 'border-muted-foreground/40'
                        )} />
                        <div className="min-w-0">
                          <p className="text-sm font-medium">{role.label}</p>
                          <p className="text-xs text-muted-foreground">{role.description}</p>
                        </div>
                      </button>
                    ))}
                  </div>
                </div>
              </div>
              <DialogFooter>
                <Button type="button" variant="outline" onClick={closeInviteDialog}>Cancel</Button>
                <Button type="submit" disabled={sending}>{sending ? 'Sending...' : 'Send Invite'}</Button>
              </DialogFooter>
            </form>
          )}
        </DialogContent>
      </Dialog>
    </Card>
  );
}

/* ============ Estimate Settings Form ============ */

function EstimateSettingsForm({ teamId, initial, saving, onSave }: {
  teamId: string;
  initial: TeamEstimateSettings | null;
  saving: boolean;
  onSave: (data: { enabled?: boolean; scale?: EstimateScale; extended?: boolean; allow_zero?: boolean; count_unestimated_as_one?: boolean }) => void;
}) {
  const [enabled, setEnabled] = useState(initial?.enabled ?? false);
  const [scale, setScale] = useState<EstimateScale>(initial?.scale ?? 'linear');
  const [extended, setExtended] = useState(initial?.extended ?? false);
  const [allowZero, setAllowZero] = useState(initial?.allow_zero ?? false);
  const [countUnestimated, setCountUnestimated] = useState(initial?.count_unestimated_as_one ?? true);

  useEffect(() => {
    setEnabled(initial?.enabled ?? false);
    setScale(initial?.scale ?? 'linear');
    setExtended(initial?.extended ?? false);
    setAllowZero(initial?.allow_zero ?? false);
    setCountUnestimated(initial?.count_unestimated_as_one ?? true);
  }, [initial, teamId]);

  const scaleOptions = getEstimateOptions(scale, extended, allowZero);

  return (
    <div className="space-y-5 py-2">
      {/* Enable toggle */}
      <div className="flex items-center justify-between">
        <div>
          <p className="text-sm font-medium">Enable estimates</p>
          <p className="text-xs text-muted-foreground">Show effort estimates on issues</p>
        </div>
        <Switch checked={enabled} onCheckedChange={setEnabled} />
      </div>

      {enabled && (
        <>
          {/* Scale selection */}
          <div className="space-y-2">
            <Label className="text-sm">Scale</Label>
            <Select value={scale} onValueChange={(v) => setScale(v as EstimateScale)}>
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {(Object.keys(SCALE_LABELS) as EstimateScale[]).map((s) => (
                  <SelectItem key={s} value={s}>
                    <span className="font-medium">{SCALE_LABELS[s]}</span>
                    <span className="ml-2 text-muted-foreground">{SCALE_DESCRIPTIONS[s]}</span>
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          {/* Preview */}
          <div className="rounded-md border border-border bg-muted/40 px-3 py-2">
            <p className="text-xs text-muted-foreground mb-1">Scale values</p>
            <div className="flex flex-wrap gap-1.5">
              {scaleOptions.map((opt) => (
                <span key={opt.value} className="inline-flex items-center rounded-md border border-border bg-background px-2 py-0.5 text-xs font-medium">
                  {opt.label}
                </span>
              ))}
            </div>
          </div>

          {/* Extended scale */}
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium">Extended scale</p>
              <p className="text-xs text-muted-foreground">Add two additional larger values</p>
            </div>
            <Switch checked={extended} onCheckedChange={setExtended} />
          </div>

          {/* Allow zero */}
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium">Allow zero estimates</p>
              <p className="text-xs text-muted-foreground">Allow issues to be estimated as zero effort</p>
            </div>
            <Switch checked={allowZero} onCheckedChange={setAllowZero} />
          </div>

          {/* Count unestimated as one */}
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium">Count unestimated as 1 point</p>
              <p className="text-xs text-muted-foreground">Unestimated issues count as 1 point in calculations</p>
            </div>
            <Switch checked={countUnestimated} onCheckedChange={setCountUnestimated} />
          </div>
        </>
      )}

      <DialogFooter>
        <Button
          disabled={saving}
          onClick={() => onSave({ enabled, scale, extended, allow_zero: allowZero, count_unestimated_as_one: countUnestimated })}
        >
          {saving ? 'Saving...' : 'Save'}
        </Button>
      </DialogFooter>
    </div>
  );
}

type VisibilityFieldKey = keyof Omit<TeamFieldVisibility, 'id' | 'team_id' | 'created_at' | 'updated_at'>;
type FieldVisibilityGroup = 'Classification' | 'Planning' | 'Other' | 'Panels';

const FIELD_VISIBILITY_FIELDS: { key: VisibilityFieldKey; label: string; group: FieldVisibilityGroup }[] = [
  { key: 'priority', label: 'Priority', group: 'Classification' },
  { key: 'story_type', label: 'Type', group: 'Classification' },
  { key: 'severity', label: 'Severity', group: 'Classification' },
  { key: 'epic', label: 'Epic', group: 'Planning' },
  { key: 'sprint', label: 'Sprint', group: 'Planning' },
  { key: 'estimate', label: 'Estimate', group: 'Planning' },
  { key: 'labels', label: 'Labels', group: 'Other' },
  { key: 'due_date', label: 'Due Date', group: 'Other' },
  { key: 'blocked', label: 'Blocked', group: 'Other' },
  { key: 'delivery', label: 'Delivery', group: 'Panels' },
  { key: 'dev_history', label: 'Development History', group: 'Panels' },
];

function FieldVisibilityForm({ teamId, initial, saving, onSave }: {
  teamId: string;
  initial: TeamFieldVisibility | null;
  saving: boolean;
  onSave: (data: Partial<Omit<TeamFieldVisibility, 'id' | 'team_id' | 'created_at' | 'updated_at'>>) => void;
}) {
  const [fields, setFields] = useState<Record<VisibilityFieldKey, boolean>>(() => {
    const defaults = {} as Record<VisibilityFieldKey, boolean>;
    for (const f of FIELD_VISIBILITY_FIELDS) {
      defaults[f.key] = initial ? initial[f.key] : true;
    }
    return defaults;
  });

  useEffect(() => {
    const next = {} as Record<VisibilityFieldKey, boolean>;
    for (const f of FIELD_VISIBILITY_FIELDS) {
      next[f.key] = initial ? initial[f.key] : true;
    }
    setFields(next);
  }, [initial, teamId]);

  const groups = [...new Set(FIELD_VISIBILITY_FIELDS.map((f) => f.group))];

  return (
    <div className="space-y-5 py-2">
      <p className="text-sm text-muted-foreground">
        Configure which fields and panels appear on stories for this team. State, Owner, Requester, and Team are always visible.
      </p>
      {groups.map((group) => (
        <div key={group} className="space-y-2">
          <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">{group}</p>
          {FIELD_VISIBILITY_FIELDS.filter((f) => f.group === group).map((f) => (
            <div key={f.key} className="flex items-center justify-between">
              <p className="text-sm font-medium">{f.label}</p>
              <Switch
                checked={fields[f.key]}
                onCheckedChange={(checked) => setFields((prev) => ({ ...prev, [f.key]: checked }))}
              />
            </div>
          ))}
        </div>
      ))}
      <DialogFooter>
        <Button
          disabled={saving}
          onClick={() => onSave(fields)}
        >
          {saving ? 'Saving...' : 'Save'}
        </Button>
      </DialogFooter>
    </div>
  );
}

function TeamRepoDefaultForm({
  initial,
  repositories,
  workflowStates,
  loading,
  saving,
  onSave,
}: {
  initial: TeamRepoDefault | null;
  repositories: GitRepository[];
  workflowStates: WorkflowState[];
  loading: boolean;
  saving: boolean;
  onSave: (payload: {
    repository_id: string;
    base_branch?: string;
    branch_template?: string;
    auto_sync_states?: boolean;
    review_state_id?: string;
    done_state_id?: string;
  }) => void | Promise<void>;
}) {
  const [repositoryId, setRepositoryId] = useState(initial?.repository_id ?? '');
  const [baseBranch, setBaseBranch] = useState(initial?.base_branch ?? '');
  const [branchTemplate, setBranchTemplate] = useState(initial?.branch_template ?? 'tp-{display_id}-{slug}');
  const [autoSyncStates, setAutoSyncStates] = useState(initial?.auto_sync_states ?? true);
  const [reviewStateId, setReviewStateId] = useState(initial?.review_state_id ?? 'none');
  const [doneStateId, setDoneStateId] = useState(initial?.done_state_id ?? 'none');

  useEffect(() => {
    setRepositoryId(initial?.repository_id ?? '');
    setBaseBranch(initial?.base_branch ?? '');
    setBranchTemplate(initial?.branch_template ?? 'tp-{display_id}-{slug}');
    setAutoSyncStates(initial?.auto_sync_states ?? true);
    setReviewStateId(initial?.review_state_id ?? 'none');
    setDoneStateId(initial?.done_state_id ?? 'none');
  }, [initial]);

  const selectedRepository = repositories.find((repository) => repository.id === repositoryId) ?? null;

  return (
    <div className="space-y-5 py-2">
      <p className="text-sm text-muted-foreground">
        Stories on this team inherit these delivery defaults. Story detail can still override the repository or base branch.
      </p>
      <div className="space-y-2">
        <Label>Repository</Label>
        <Select value={repositoryId || undefined} onValueChange={setRepositoryId}>
          <SelectTrigger>
            <SelectValue placeholder={loading ? 'Loading repositories...' : 'Select repository'} />
          </SelectTrigger>
          <SelectContent>
            {repositories.map((repository) => (
              <SelectItem key={repository.id} value={repository.id}>
                {repository.full_name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        {selectedRepository && (
          <p className="text-xs text-muted-foreground">
            Default branch: {selectedRepository.default_branch}
          </p>
        )}
      </div>
      <div className="space-y-2">
        <Label>Base branch</Label>
        <Input
          value={baseBranch}
          onChange={(event) => setBaseBranch(event.target.value)}
          placeholder={selectedRepository?.default_branch || 'main'}
        />
      </div>
      <div className="space-y-2">
        <Label>Branch template</Label>
        <Input
          value={branchTemplate}
          onChange={(event) => setBranchTemplate(event.target.value)}
          placeholder="tp-{display_id}-{slug}"
        />
        <p className="text-xs text-muted-foreground">
          Available tokens: {'{display_id}'} and {'{slug}'}.
        </p>
      </div>
      <div className="flex items-center justify-between rounded-lg border border-border/60 px-3 py-2">
        <div>
          <p className="text-sm font-medium">Auto-sync workflow state from PR events</p>
          <p className="text-xs text-muted-foreground">
            When enabled, pull request webhooks can move stories forward automatically.
          </p>
        </div>
        <Switch checked={autoSyncStates} onCheckedChange={setAutoSyncStates} />
      </div>
      <div className="grid gap-4 sm:grid-cols-2">
        <div className="space-y-2">
          <Label>PR opened state</Label>
          <Select value={reviewStateId} onValueChange={setReviewStateId}>
            <SelectTrigger>
              <SelectValue placeholder="Choose review state" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="none">Do not map</SelectItem>
              {workflowStates.map((state) => (
                <SelectItem key={state.id} value={state.id}>
                  {state.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="space-y-2">
          <Label>PR merged state</Label>
          <Select value={doneStateId} onValueChange={setDoneStateId}>
            <SelectTrigger>
              <SelectValue placeholder="Choose done state" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="none">Do not map</SelectItem>
              {workflowStates.map((state) => (
                <SelectItem key={state.id} value={state.id}>
                  {state.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>
      {repositories.length === 0 && !loading && (
        <p className="text-xs text-muted-foreground">
          No repositories are synced yet. Connect GitHub and sync repositories before setting a team delivery default.
        </p>
      )}
      <DialogFooter>
        <Button
          disabled={saving || !repositoryId}
          onClick={() => onSave({
            repository_id: repositoryId,
            base_branch: baseBranch || selectedRepository?.default_branch || 'main',
            branch_template: branchTemplate || 'tp-{display_id}-{slug}',
            auto_sync_states: autoSyncStates,
            review_state_id: reviewStateId !== 'none' ? reviewStateId : undefined,
            done_state_id: doneStateId !== 'none' ? doneStateId : undefined,
          })}
        >
          {saving ? 'Saving...' : 'Save'}
        </Button>
      </DialogFooter>
    </div>
  );
}

/* ============ Teams Tab ============ */

function TeamsTab({ workspaceId, teams, userMemberships, invitationPreassignments, teamEstimateSettings, teamFieldVisibility, teamRepoDefaults, editable, onRefresh }: {
  workspaceId: string;
  teams: WorkspaceTeam[];
  userMemberships: TeamUserMembership[];
  invitationPreassignments: InvitationTeamPreassignment[];
  teamEstimateSettings: TeamEstimateSettings[];
  teamFieldVisibility: TeamFieldVisibility[];
  teamRepoDefaults: TeamRepoDefault[];
  editable: boolean;
  onRefresh: (silent?: boolean) => void | Promise<void>;
}) {
  const navigate = useNavigate();
  const { currentWorkspace } = useWorkspaceStore();
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editTeam, setEditTeam] = useState<WorkspaceTeam | null>(null);
  const [name, setName] = useState('');
  const [handle, setHandle] = useState('');
  const [description, setDescription] = useState('');
  const [saving, setSaving] = useState(false);
  const [query, setQuery] = useState('');
  const [selectedTeamId, setSelectedTeamId] = useState<string | null>(null);

  const [estimateDialogOpen, setEstimateDialogOpen] = useState(false);
  const [estimateSaving, setEstimateSaving] = useState(false);

  const [fieldVisDialogOpen, setFieldVisDialogOpen] = useState(false);
  const [fieldVisSaving, setFieldVisSaving] = useState(false);

  const [repoDialogOpen, setRepoDialogOpen] = useState(false);
  const [repoSaving, setRepoSaving] = useState(false);
  const [repositories, setRepositories] = useState<GitRepository[]>([]);
  const [repositoriesLoading, setRepositoriesLoading] = useState(false);
  const [workflows, setWorkflows] = useState<WorkflowWithStates[]>([]);

  const [memberDialogOpen, setMemberDialogOpen] = useState(false);
  const [workspaceMembers, setWorkspaceMembers] = useState<MemberWithUser[]>([]);
  const [invitations, setInvitations] = useState<Invitation[]>([]);
  const [, setMembersLoading] = useState(false);
  const [savingMember, setSavingMember] = useState(false);
  const [memberSearch, setMemberSearch] = useState('');
  const [deleteTeamConfirm, setDeleteTeamConfirm] = useState<string | null>(null);

  useEffect(() => {
    let mounted = true;
    const loadMembers = async () => {
      setMembersLoading(true);
      try {
        const [membersRes, invitesRes] = await Promise.all([
          workspacesService.listMembers(workspaceId),
          editable ? inviteService.list(workspaceId) : Promise.resolve({ data: null, error: null }),
        ]);
        if (!mounted) return;
        if (membersRes.error) {
          if (editable) toast.error(membersRes.error);
          setWorkspaceMembers([]);
        } else {
          setWorkspaceMembers(
            [...(membersRes.data ?? [])].sort((a, b) =>
              (a.full_name || a.email).localeCompare(b.full_name || b.email),
            ),
          );
        }
        setInvitations((invitesRes.data ?? []).filter((inv) => inv.status === 'pending'));
      } finally {
        if (mounted) setMembersLoading(false);
      }
    };
    loadMembers();
    return () => { mounted = false; };
  }, [workspaceId, editable]);

  useEffect(() => {
    let mounted = true;
    const loadRepositories = async () => {
      setRepositoriesLoading(true);
      const { data, error } = await gitService.listRepositories(workspaceId);
      if (!mounted) return;
      if (error) {
        toast.error(error);
        setRepositories([]);
      } else {
        setRepositories(data ?? []);
      }
      setRepositoriesLoading(false);
    };
    loadRepositories();
    return () => { mounted = false; };
  }, [workspaceId]);

  useEffect(() => {
    let mounted = true;
    const loadWorkflows = async () => {
      const { data, error } = await pmWorkflowService.list(workspaceId);
      if (!mounted) return;
      if (error) {
        toast.error(error);
        setWorkflows([]);
      } else {
        setWorkflows(data ?? []);
      }
    };
    loadWorkflows();
    return () => { mounted = false; };
  }, [workspaceId]);

  const filteredTeams = [...teams]
    .filter((team) => {
      const search = query.trim().toLowerCase();
      if (!search) return true;
      return [team.name, team.handle, team.description]
        .filter(Boolean)
        .some((value) => value!.toLowerCase().includes(search));
    })
    .sort((a, b) => a.name.localeCompare(b.name));

  const selectedTeam = selectedTeamId ? teams.find((t) => t.id === selectedTeamId) ?? null : null;

  const getTeamMemberships = (teamId: string) =>
    userMemberships
      .filter((membership) => membership.team_id === teamId)
      .map((membership) => ({
        membership,
        user: workspaceMembers.find((member) => member.user_id === membership.user_id),
      }))
      .sort((a, b) => (a.user?.full_name || a.user?.email || '').localeCompare(b.user?.full_name || b.user?.email || ''));

  const teamMembers = selectedTeam
    ? getTeamMemberships(selectedTeam.id)
    : [];

  const availableMembers = selectedTeam
    ? workspaceMembers.filter((member) => !userMemberships.some((membership) => membership.team_id === selectedTeam.id && membership.user_id === member.user_id))
    : [];

  const openCreate = () => {
    setEditTeam(null);
    setName('');
    setHandle('');
    setDescription('');
    setDialogOpen(true);
  };

  const openEdit = (team: WorkspaceTeam) => {
    setEditTeam(team);
    setName(team.name);
    setHandle(team.handle ?? '');
    setDescription(team.description ?? '');
    setDialogOpen(true);
  };

  const openMembers = (team: WorkspaceTeam) => {
    setSelectedTeamId(team.id);
    setMemberSearch('');
    setMemberDialogOpen(true);
  };

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setSaving(true);
    const payload = {
      name,
      handle: handle.trim() ? slugifyTeamHandle(handle) : undefined,
      description: description || undefined,
    };
    if (editTeam) {
      const { error } = await settingsService.updateTeam(editTeam.id, payload);
      if (error) toast.error(error);
      else {
        toast.success('Team updated');
        setDialogOpen(false);
        await onRefresh();
      }
    } else {
      const { data, error } = await settingsService.createTeam({ workspace_id: workspaceId, ...payload });
      if (error) toast.error(error);
      else {
        toast.success('Team created');
        setDialogOpen(false);
        setSelectedTeamId(data?.id ?? null);
        await onRefresh();
      }
    }
    setSaving(false);
  };

  const handleDelete = async (id: string) => {
    const { error } = await settingsService.deleteTeam(id);
    if (error) toast.error(error);
    else {
      toast.success('Team deleted');
      setSelectedTeamId(null);
      await onRefresh();
    }
  };

  const handleRoleChange = async (teamId: string, userId: string, role: 'owner' | 'member') => {
    const { error } = await settingsService.updateTeamMember(teamId, userId, { role });
    if (error) toast.error(error);
    else {
      toast.success('Team member updated');
      await onRefresh(true);
    }
  };

  const handleRemoveMember = async (teamId: string, userId: string) => {
    const { error } = await settingsService.removeTeamMember(teamId, userId);
    if (error) toast.error(error);
    else {
      toast.success('Team member removed');
      await onRefresh(true);
    }
  };

  const handleAddInvitation = async (teamId: string, invitationId: string) => {
    const { error } = await settingsService.addTeamInvitation(teamId, invitationId);
    if (error) toast.error(error);
    else {
      toast.success('Invited member pre-assigned to team');
      await onRefresh(true);
    }
  };

  const handleRemoveInvitation = async (teamId: string, invitationId: string) => {
    const { error } = await settingsService.removeTeamInvitation(teamId, invitationId);
    if (error) toast.error(error);
    else {
      toast.success('Invitation pre-assignment removed');
      await onRefresh(true);
    }
  };

  const openSettingsSection = (section: SettingsSection) => {
    const slug = currentWorkspace?.slug;
    if (!slug) return;
    navigate({
      to: '/w/$slug/settings/$section',
      params: { slug, section },
      search: { team: selectedTeamId ?? undefined },
    });
  };

  // ── Detail view (team selected) ──
  if (selectedTeam) {
    const invitedCount = invitationPreassignments.filter((pa) => pa.team_id === selectedTeam.id).length;
    const memberCount = teamMembers.length + invitedCount;
    const teamEstConfig = teamEstimateSettings.find((s) => s.team_id === selectedTeam.id);
    const estimateMeta = teamEstConfig?.enabled
      ? SCALE_LABELS[teamEstConfig.scale]
      : 'Disabled';
    const teamVisConfig = teamFieldVisibility.find((s) => s.team_id === selectedTeam.id);
    const visibleCount = teamVisConfig
      ? FIELD_VISIBILITY_FIELDS.filter((f) => teamVisConfig[f.key]).length
      : FIELD_VISIBILITY_FIELDS.length;
    const fieldVisMeta = `${visibleCount} of ${FIELD_VISIBILITY_FIELDS.length} visible`;
    const teamRepoDefault = teamRepoDefaults.find((config) => config.team_id === selectedTeam.id);
    const repoRecord = repositories.find((repository) => repository.id === teamRepoDefault?.repository_id);
    const teamWorkflowStates = workflows
      .filter(({ workflow }) => !workflow.team_id || workflow.team_id === selectedTeam.id)
      .flatMap(({ states }) => states);
    const deliveryMeta = teamRepoDefault
      ? `${repoRecord?.full_name ?? 'Repo selected'} · ${teamRepoDefault.base_branch}`
      : 'Not configured';
    const settingsGroups: {
      label?: string;
      rows: { key: string; icon: LucideIcon; title: string; description: string; meta: string; action: () => void; disabled: boolean }[];
    }[] = [
      {
        rows: [
          {
            key: 'general',
            icon: Settings2,
            title: 'General',
            description: 'Name, identifier, and broader settings',
            meta: selectedTeam.handle ? `@${selectedTeam.handle}` : '',
            action: () => openEdit(selectedTeam),
            disabled: !editable,
          },
          {
            key: 'members',
            icon: Users,
            title: 'Members',
            description: 'Manage team members',
            meta: `${memberCount} member${memberCount === 1 ? '' : 's'}`,
            action: () => openMembers(selectedTeam),
            disabled: !editable,
          },
        ],
      },
      {
        label: 'Issues, projects, and docs',
        rows: [
          {
            key: 'delivery-defaults',
            icon: GitPullRequest,
            title: 'Delivery defaults',
            description: 'Choose the team repository, base branch, and branch template',
            meta: deliveryMeta,
            action: () => setRepoDialogOpen(true),
            disabled: !editable,
          },
          {
            key: 'estimates',
            icon: LayoutGrid,
            title: 'Estimates',
            description: 'Configure estimate scale and options',
            meta: estimateMeta,
            action: () => setEstimateDialogOpen(true),
            disabled: !editable,
          },
          {
            key: 'field-visibility',
            icon: Eye,
            title: 'Story display',
            description: 'Configure which fields and panels appear on stories',
            meta: fieldVisMeta,
            action: () => setFieldVisDialogOpen(true),
            disabled: !editable,
          },
          {
            key: 'labels',
            icon: Tag,
            title: 'Issue labels',
            description: "Labels available to this team's issues",
            meta: '',
            action: () => openSettingsSection('labels'),
            disabled: false,
          },
        ],
      },
      {
        label: 'Workflow',
        rows: [
          {
            key: 'workflow',
            icon: GitBranch,
            title: 'Issue statuses & automations',
            description: 'Customize issue statuses and automations',
            meta: '',
            action: () => openSettingsSection('workflows'),
            disabled: false,
          },
          {
            key: 'automations',
            icon: RefreshCw,
            title: 'Automations',
            description: 'Sprint and epic automations for this team',
            meta: '',
            action: () => openSettingsSection('automations'),
            disabled: false,
          },
        ],
      },
    ];

    return (
      <>
        <div className="space-y-8">
          <button
            type="button"
            onClick={() => setSelectedTeamId(null)}
            className="inline-flex items-center gap-1.5 text-sm text-muted-foreground transition-colors hover:text-foreground"
          >
            <ChevronRight className="h-4 w-4 rotate-180" />
            Teams
          </button>

          <div className="flex items-center gap-4">
            {(() => {
              const color = getAvatarColor(selectedTeam.name);
              return (
                <div className={cn('flex h-12 w-12 shrink-0 items-center justify-center rounded-xl text-base font-semibold', color.bg, color.text)}>
                  {getInitials(selectedTeam.name)}
                </div>
              );
            })()}
            <div className="flex min-w-0 flex-1 items-center justify-between">
              <h2 className="text-xl font-semibold tracking-tight">{selectedTeam.name}</h2>
              {editable && (
                <div className="flex items-center gap-2">
                  <Button variant="outline" size="sm" onClick={() => openMembers(selectedTeam)}>
                    <Users className="h-4 w-4 mr-1" />
                    Add member
                  </Button>
                  <Button variant="outline" size="sm" className="text-destructive hover:text-destructive" onClick={() => setDeleteTeamConfirm(selectedTeam.id)}>
                    <Trash2 className="h-4 w-4 mr-1" />
                    Delete
                  </Button>
                </div>
              )}
            </div>
          </div>

          {settingsGroups.map((group) => (
            <div key={group.label ?? '_default'} className="space-y-3">
              {group.label && (
                <h3 className="text-sm font-medium text-muted-foreground">{group.label}</h3>
              )}
              <div className="overflow-hidden rounded-lg border border-border bg-background">
                {group.rows.map((row, idx) => (
                  <button
                    key={row.key}
                    type="button"
                    disabled={row.disabled}
                    onClick={row.action}
                    className={cn(
                      'flex w-full items-center gap-4 px-4 py-4 text-left transition-colors',
                      row.disabled ? 'cursor-not-allowed opacity-60' : 'hover:bg-muted/40',
                      idx > 0 && 'border-t border-border',
                    )}
                  >
                    <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
                      <row.icon className="h-4 w-4" />
                    </div>
                    <div className="min-w-0 flex-1">
                      <p className="text-sm font-medium">{row.title}</p>
                      <p className="text-sm text-muted-foreground">{row.description}</p>
                    </div>
                    {row.meta && (
                      <span className="hidden text-sm text-muted-foreground md:block">{row.meta}</span>
                    )}
                    <ChevronRight className="h-4 w-4 shrink-0 text-muted-foreground" />
                  </button>
                ))}
              </div>
            </div>
          ))}
        </div>

        <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
          <DialogContent>
            <form onSubmit={handleSubmit}>
              <DialogHeader>
                <DialogTitle>{editTeam ? 'Edit Team' : 'Create Team'}</DialogTitle>
              </DialogHeader>
              <div className="space-y-4 py-4">
                <div className="space-y-2">
                  <Label>Name</Label>
                  <Input value={name} onChange={e => setName(e.target.value)} required />
                </div>
                <div className="space-y-2">
                  <Label>Handle</Label>
                  <Input
                    value={handle}
                    onChange={e => setHandle(e.target.value)}
                    placeholder={slugifyTeamHandle(name) || 'growth'}
                  />
                  <p className="text-xs text-muted-foreground">Used for mentions like @{slugifyTeamHandle(handle || name) || 'team'}.</p>
                </div>
                <div className="space-y-2">
                  <Label>Description</Label>
                  <Textarea
                    value={description}
                    onChange={e => setDescription(e.target.value)}
                    rows={4}
                    placeholder="Describe what this team owns."
                  />
                </div>
              </div>
              <DialogFooter>
                <Button type="button" variant="outline" onClick={() => setDialogOpen(false)}>Cancel</Button>
                <Button type="submit" disabled={saving}>{saving ? 'Saving...' : 'Save'}</Button>
              </DialogFooter>
            </form>
          </DialogContent>
        </Dialog>

        <Dialog open={memberDialogOpen} onOpenChange={setMemberDialogOpen}>
          <DialogContent className="max-w-xl gap-0 p-0">
            <DialogHeader className="border-b px-5 py-4">
              <DialogTitle className="text-base">{selectedTeam ? `${selectedTeam.name} members` : 'Team Members'}</DialogTitle>
              <p className="text-sm text-muted-foreground">Add or remove members who belong to this team.</p>
            </DialogHeader>

            {editable && (availableMembers.length > 0 || (selectedTeam && invitations.filter((inv) => !invitationPreassignments.some((pa) => pa.invitation_id === inv.id && pa.team_id === selectedTeam.id)).length > 0)) && (
              <div className="border-b">
                <div className="px-5 py-3">
                  <Input
                    placeholder="Search members to add..."
                    value={memberSearch}
                    onChange={(e) => setMemberSearch(e.target.value)}
                    className="h-8 text-sm"
                  />
                </div>
                <div className="max-h-[160px] overflow-y-auto border-t border-border/40">
                  {(() => {
                    const query = memberSearch.toLowerCase();
                    const filteredMembers = availableMembers.filter((m) =>
                      (m.full_name || '').toLowerCase().includes(query) || m.email.toLowerCase().includes(query)
                    );
                    const filteredInvitations = selectedTeam
                      ? invitations
                          .filter((inv) => !invitationPreassignments.some((pa) => pa.invitation_id === inv.id && pa.team_id === selectedTeam.id))
                          .filter((inv) => inv.email.toLowerCase().includes(query))
                      : [];

                    if (filteredMembers.length === 0 && filteredInvitations.length === 0) {
                      return <p className="px-5 py-3 text-xs text-muted-foreground text-center">No members to add</p>;
                    }

                    return (
                      <>
                        {filteredMembers.map((member) => (
                          <button
                            key={member.user_id}
                            type="button"
                            disabled={savingMember}
                            className="flex w-full items-center gap-3 px-5 py-2 text-left transition-colors hover:bg-accent cursor-pointer disabled:opacity-50"
                            onClick={async () => {
                              if (!selectedTeam) return;
                              setSavingMember(true);
                              const { error } = await settingsService.addTeamMember(selectedTeam.id, { user_id: member.user_id, role: 'member' });
                              setSavingMember(false);
                              if (error) { toast.error(error); return; }
                              toast.success(`${member.full_name || member.email} added`);
                              await onRefresh(true);
                            }}
                          >
                            <UserAvatar name={member.full_name || member.email} className="h-6 w-6" fallbackClassName="text-[9px]" />
                            <div className="min-w-0 flex-1">
                              <p className="truncate text-sm">{member.full_name || member.email}</p>
                              {member.full_name && <p className="truncate text-xs text-muted-foreground">{member.email}</p>}
                            </div>
                            <Plus className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                          </button>
                        ))}
                        {filteredInvitations.length > 0 && (
                          <>
                            {filteredMembers.length > 0 && (
                              <div className="px-5 py-1.5 text-[10px] font-medium uppercase tracking-wider text-muted-foreground/70">Pending invitations</div>
                            )}
                            {filteredInvitations.map((inv) => (
                              <button
                                key={inv.id}
                                type="button"
                                disabled={savingMember}
                                className="flex w-full items-center gap-3 px-5 py-2 text-left transition-colors hover:bg-accent cursor-pointer disabled:opacity-50"
                                onClick={async () => {
                                  if (!selectedTeam) return;
                                  await handleAddInvitation(selectedTeam.id, inv.id);
                                }}
                              >
                                <UserAvatar name={inv.email} className="h-6 w-6" fallbackClassName="text-[9px]" />
                                <div className="min-w-0 flex-1">
                                  <p className="truncate text-sm">{inv.email}</p>
                                </div>
                                <Badge variant="secondary" className="text-[10px] px-1.5 py-0">Invited</Badge>
                                <Plus className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                              </button>
                            ))}
                          </>
                        )}
                      </>
                    );
                  })()}
                </div>
              </div>
            )}

            <div className="max-h-[400px] overflow-y-auto">
              {selectedTeam && (teamMembers.length > 0 || invitationPreassignments.filter((pa) => pa.team_id === selectedTeam.id).length > 0) && (
                <div className="px-5 pt-3 pb-1">
                  <p className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground/70">Current members</p>
                </div>
              )}
              {selectedTeam && (teamMembers.length > 0 || invitationPreassignments.filter((pa) => pa.team_id === selectedTeam.id).length > 0) ? (
                <div className="divide-y">
                  {teamMembers.map(({ membership, user }) => (
                    <div key={membership.id} className="flex items-center gap-3 px-5 py-3">
                      <UserAvatar name={user?.full_name || user?.email || membership.user_id} className="h-8 w-8" fallbackClassName="text-xs" />
                      <div className="min-w-0 flex-1">
                        <p className="truncate text-sm font-medium">{user?.full_name || 'Unknown user'}</p>
                        <p className="truncate text-xs text-muted-foreground">{user?.email || membership.user_id}</p>
                      </div>
                      {editable ? (
                        <div className="flex items-center gap-1.5">
                          <Select
                            value={membership.role}
                            onValueChange={(value) => handleRoleChange(membership.team_id, membership.user_id, value as 'owner' | 'member')}
                          >
                            <SelectTrigger className="h-7 w-[100px] text-xs">
                              <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                              <SelectItem value="member">Member</SelectItem>
                              <SelectItem value="owner">Owner</SelectItem>
                            </SelectContent>
                          </Select>
                          <Button size="icon" variant="ghost" className="h-7 w-7" onClick={() => handleRemoveMember(membership.team_id, membership.user_id)}>
                            <X className="h-3.5 w-3.5" />
                          </Button>
                        </div>
                      ) : (
                        <Badge variant="outline" className="text-xs capitalize">{membership.role}</Badge>
                      )}
                    </div>
                  ))}
                  {invitationPreassignments
                    .filter((pa) => pa.team_id === selectedTeam.id)
                    .map((pa) => {
                      const inv = invitations.find((i) => i.id === pa.invitation_id);
                      return (
                        <div key={pa.id} className="flex items-center gap-3 px-5 py-3 bg-muted/30">
                          <UserAvatar name={inv?.email || pa.invitation_id} className="h-8 w-8" fallbackClassName="text-xs" />
                          <div className="min-w-0 flex-1">
                            <p className="truncate text-sm font-medium">{inv?.email || 'Pending invitation'}</p>
                            <p className="truncate text-xs text-muted-foreground">Will join as member when accepted</p>
                          </div>
                          <Badge variant="secondary" className="text-xs">Invited</Badge>
                          {editable && (
                            <Button size="icon" variant="ghost" className="h-7 w-7" onClick={() => handleRemoveInvitation(selectedTeam.id, pa.invitation_id)}>
                              <X className="h-3.5 w-3.5" />
                            </Button>
                          )}
                        </div>
                      );
                    })}
                </div>
              ) : (
                <div className="px-5 py-8 text-center text-sm text-muted-foreground">
                  No team members yet. Add members above.
                </div>
              )}
            </div>
          </DialogContent>
        </Dialog>

        {/* Estimate Settings Dialog */}
        <Dialog open={estimateDialogOpen} onOpenChange={setEstimateDialogOpen}>
          <DialogContent className="max-w-md">
            <DialogHeader>
              <DialogTitle>Estimate Settings</DialogTitle>
            </DialogHeader>
            <EstimateSettingsForm
              teamId={selectedTeam.id}
              initial={teamEstConfig ?? null}
              saving={estimateSaving}
              onSave={async (data) => {
                setEstimateSaving(true);
                const { error } = await settingsService.updateTeamEstimateSettings(selectedTeam.id, data);
                setEstimateSaving(false);
                if (error) {
                  toast.error(error);
                } else {
                  toast.success('Estimate settings updated');
                  setEstimateDialogOpen(false);
                  await onRefresh();
                }
              }}
            />
          </DialogContent>
        </Dialog>

        {/* Story Display Dialog */}
        <Dialog open={fieldVisDialogOpen} onOpenChange={setFieldVisDialogOpen}>
          <DialogContent className="max-w-md">
            <DialogHeader>
              <DialogTitle>Story Display</DialogTitle>
            </DialogHeader>
            <FieldVisibilityForm
              teamId={selectedTeam.id}
              initial={teamVisConfig ?? null}
              saving={fieldVisSaving}
              onSave={async (data) => {
                setFieldVisSaving(true);
                const { error } = await settingsService.updateTeamFieldVisibility(selectedTeam.id, data);
                setFieldVisSaving(false);
                if (error) {
                  toast.error(error);
                } else {
                  toast.success('Story display updated');
                  setFieldVisDialogOpen(false);
                  await onRefresh();
                }
              }}
            />
          </DialogContent>
        </Dialog>

        <Dialog open={repoDialogOpen} onOpenChange={setRepoDialogOpen}>
          <DialogContent className="max-w-md">
            <DialogHeader>
              <DialogTitle>Delivery Defaults</DialogTitle>
            </DialogHeader>
            <TeamRepoDefaultForm
              initial={teamRepoDefault ?? null}
              repositories={repositories}
              workflowStates={teamWorkflowStates}
              loading={repositoriesLoading}
              saving={repoSaving}
              onSave={async (data) => {
                setRepoSaving(true);
                const { error } = await settingsService.updateTeamRepoDefault(selectedTeam.id, data);
                setRepoSaving(false);
                if (error) {
                  toast.error(error);
                } else {
                  toast.success('Delivery defaults updated');
                  setRepoDialogOpen(false);
                  await onRefresh();
                }
              }}
            />
          </DialogContent>
        </Dialog>
      </>
    );
  }

  // ── List view (no team selected) ──
  return (
    <>
      <div className="space-y-6">
        <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
          <div>
            <h2 className="text-xl font-semibold">Teams</h2>
            <p className="text-sm text-muted-foreground">
              Create teams, assign members, and manage team-level PM settings.
            </p>
          </div>
          <div className="flex items-center gap-3">
            {teams.length > 0 && (
              <div className="relative">
                <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  value={query}
                  onChange={(event) => setQuery(event.target.value)}
                  className="h-9 w-60 pl-9"
                  placeholder="Search teams..."
                />
              </div>
            )}
            {editable && (
              <Button size="sm" onClick={openCreate}>
                <Plus className="h-4 w-4 mr-1" />
                New Team
              </Button>
            )}
          </div>
        </div>

        {teams.length === 0 ? (
          <div className="flex min-h-[320px] flex-col items-center justify-center gap-3 rounded-lg border border-dashed border-border text-center">
            <div className="flex h-12 w-12 items-center justify-center rounded-xl border border-border bg-muted/60">
              <Users className="h-5 w-5 text-muted-foreground" />
            </div>
            <div className="space-y-1">
              <p className="font-medium">No teams yet</p>
              <p className="text-sm text-muted-foreground">
                Create your first team to manage memberships, labels, and workflows.
              </p>
            </div>
            {editable && (
              <Button onClick={openCreate}>
                <Plus className="h-4 w-4 mr-1" />
                Create Team
              </Button>
            )}
          </div>
        ) : (
          <div className="overflow-hidden rounded-lg border border-border">
            <Table>
              <TableHeader>
                <TableRow className="bg-muted/40 hover:bg-muted/40">
                  <TableHead className="w-[280px]">Name</TableHead>
                  <TableHead className="w-[140px]">Handle</TableHead>
                  <TableHead className="w-[100px]">Members</TableHead>
                  <TableHead>Description</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {filteredTeams.map((team) => {
                  const memberCount = getTeamMemberships(team.id).length;
                  return (
                    <TableRow
                      key={team.id}
                      className="cursor-pointer"
                      onClick={() => setSelectedTeamId(team.id)}
                    >
                      <TableCell>
                        <div className="flex items-center gap-3">
                          {(() => {
                            const color = getAvatarColor(team.name);
                            return (
                              <div className={cn('flex h-8 w-8 shrink-0 items-center justify-center rounded-lg text-xs font-semibold', color.bg, color.text)}>
                                {getInitials(team.name)}
                              </div>
                            );
                          })()}
                          <span className="font-medium">{team.name}</span>
                        </div>
                      </TableCell>
                      <TableCell>
                        {team.handle ? (
                          <span className="font-mono text-xs text-muted-foreground">@{team.handle}</span>
                        ) : (
                          <span className="text-xs text-muted-foreground/50">&mdash;</span>
                        )}
                      </TableCell>
                      <TableCell>
                        <span className="text-sm text-muted-foreground">{memberCount}</span>
                      </TableCell>
                      <TableCell>
                        <span className="line-clamp-1 text-sm text-muted-foreground">
                          {team.description || '\u2014'}
                        </span>
                      </TableCell>
                    </TableRow>
                  );
                })}
                {filteredTeams.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={4} className="py-8 text-center text-sm text-muted-foreground">
                      No teams match your search.
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </div>
        )}
      </div>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent>
          <form onSubmit={handleSubmit}>
            <DialogHeader>
              <DialogTitle>Create Team</DialogTitle>
            </DialogHeader>
            <div className="space-y-4 py-4">
              <div className="space-y-2">
                <Label>Name</Label>
                <Input value={name} onChange={e => setName(e.target.value)} required />
              </div>
              <div className="space-y-2">
                <Label>Handle</Label>
                <Input
                  value={handle}
                  onChange={e => setHandle(e.target.value)}
                  placeholder={slugifyTeamHandle(name) || 'growth'}
                />
                <p className="text-xs text-muted-foreground">Used for mentions like @{slugifyTeamHandle(handle || name) || 'team'}.</p>
              </div>
              <div className="space-y-2">
                <Label>Description</Label>
                <Textarea
                  value={description}
                  onChange={e => setDescription(e.target.value)}
                  rows={4}
                  placeholder="Describe what this team owns."
                />
              </div>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setDialogOpen(false)}>Cancel</Button>
              <Button type="submit" disabled={saving}>{saving ? 'Saving...' : 'Save'}</Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={deleteTeamConfirm !== null}
        onOpenChange={(open) => { if (!open) setDeleteTeamConfirm(null); }}
        title="Delete team"
        description="This will permanently delete the team and remove all member assignments. This action cannot be undone."
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={() => { if (deleteTeamConfirm) handleDelete(deleteTeamConfirm); setDeleteTeamConfirm(null); }}
      />
    </>
  );
}

/* ============ People Tab ============ */

function PeopleTab({ people, editable, onRefresh }: {
  people: WorkspacePerson[];
  editable: boolean;
  onRefresh: () => void;
}) {
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editPerson, setEditPerson] = useState<WorkspacePerson | null>(null);
  const [role, setRole] = useState<WorkspacePerson['role']>('employee');
  const [jobRole, setJobRole] = useState('');
  const [salary, setSalary] = useState('0');
  const [saving, setSaving] = useState(false);
  const [deletePersonConfirm, setDeletePersonConfirm] = useState<string | null>(null);

  const openEdit = (p: WorkspacePerson) => {
    setEditPerson(p);
    setRole(p.role); setJobRole(p.job_role); setSalary(String(p.base_salary));
    setDialogOpen(true);
  };

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (!editPerson) return;
    setSaving(true);
    const { error } = await settingsService.updatePerson(editPerson.id, {
      role, job_role: jobRole, base_salary: Number(salary),
    });
    if (error) toast.error(error);
    else { toast.success('Person updated'); setDialogOpen(false); onRefresh(); }
    setSaving(false);
  };

  const handleDelete = async (id: string) => {
    const { error } = await settingsService.deletePerson(id);
    if (error) toast.error(error);
    else { toast.success('Person removed'); onRefresh(); }
  };

  return (
    <Card className={LINEAR_CARD_CLASS}>
      <CardHeader>
        <div className="flex items-center gap-2">
          <CardTitle className="text-base">People</CardTitle>
          <Badge variant="outline" className="text-xs font-normal">{people.length}</Badge>
        </div>
      </CardHeader>
      <CardContent>
        {people.length === 0 ? (
          <p className="text-sm text-muted-foreground text-center py-6">No people yet. Invite members from the Members tab to get started.</p>
        ) : (
          <div className="overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Email</TableHead>
                  <TableHead>Role</TableHead>
                  <TableHead>Job Role</TableHead>
                  <TableHead>Status</TableHead>
                  {editable && <TableHead className="w-24">Actions</TableHead>}
                </TableRow>
              </TableHeader>
              <TableBody>
                {people.map(p => (
                  <TableRow key={p.id}>
                    <TableCell className="font-medium">{p.name}</TableCell>
                    <TableCell className="text-muted-foreground">{p.email}</TableCell>
                    <TableCell><Badge variant="outline" className="text-xs">{p.role}</Badge></TableCell>
                    <TableCell>{p.job_role || '—'}</TableCell>
                    <TableCell>
                      <Badge variant={p.status === 'active' ? 'default' : 'secondary'} className="text-xs">
                        {p.status}
                      </Badge>
                    </TableCell>
                    {editable && (
                      <TableCell>
                        <div className="flex gap-1">
                          <Button size="icon" variant="ghost" onClick={() => openEdit(p)}>
                            <Pencil className="h-3.5 w-3.5" />
                          </Button>
                          <Button size="icon" variant="ghost" className="text-destructive hover:text-destructive" onClick={() => setDeletePersonConfirm(p.id)}>
                            <Trash2 className="h-3.5 w-3.5" />
                          </Button>
                        </div>
                      </TableCell>
                    )}
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}
      </CardContent>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent>
          <form onSubmit={handleSubmit}>
            <DialogHeader>
              <DialogTitle>Edit Person</DialogTitle>
            </DialogHeader>
            <div className="space-y-4 py-4">
              <div className="space-y-2">
                <Label>Name</Label>
                <Input value={editPerson?.name ?? ''} disabled className="bg-muted" />
              </div>
              <div className="space-y-2">
                <Label>Email</Label>
                <Input value={editPerson?.email ?? ''} disabled className="bg-muted" />
              </div>
              <div className="space-y-2">
                <Label>Role</Label>
                <Select value={role} onValueChange={v => setRole(v as WorkspacePerson['role'])}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="executive">Executive</SelectItem>
                    <SelectItem value="manager">Manager</SelectItem>
                    <SelectItem value="employee">Employee</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <Label>Job Role</Label>
                <Input placeholder="e.g. Frontend Developer" value={jobRole} onChange={e => setJobRole(e.target.value)} />
              </div>
              <div className="space-y-2">
                <Label>Base Salary</Label>
                <Input type="number" value={salary} onChange={e => setSalary(e.target.value)} />
              </div>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setDialogOpen(false)}>Cancel</Button>
              <Button type="submit" disabled={saving}>{saving ? 'Saving...' : 'Save'}</Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={deletePersonConfirm !== null}
        onOpenChange={(open) => { if (!open) setDeletePersonConfirm(null); }}
        title="Delete person"
        description="This will permanently remove this person from the workspace. This action cannot be undone."
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={() => { if (deletePersonConfirm) handleDelete(deletePersonConfirm); setDeletePersonConfirm(null); }}
      />
    </Card>
  );
}

/* ============ Job Roles Tab ============ */

function JobRolesTab({ workspaceId, criteria, editable, onRefresh }: {
  workspaceId: string;
  criteria: JobRoleCriteria[];
  editable: boolean;
  onRefresh: () => void;
}) {
  // Group criteria by job_role
  const grouped = criteria.reduce<Record<string, JobRoleCriteria[]>>((acc, c) => {
    if (!acc[c.job_role]) acc[c.job_role] = [];
    acc[c.job_role].push(c);
    return acc;
  }, {});
  const jobRoles = Object.keys(grouped).sort();
  const [deleteRoleConfirm, setDeleteRoleConfirm] = useState<string | null>(null);

  const handleDeleteRole = async (jobRole: string) => {
    const { error } = await settingsService.deleteJobRole(workspaceId, jobRole);
    if (error) toast.error(error);
    else { toast.success(`"${jobRole}" criteria deleted`); onRefresh(); }
  };

  return (
    <Card className={LINEAR_CARD_CLASS}>
      <CardHeader>
        <CardTitle className="text-base">Job Role Criteria</CardTitle>
      </CardHeader>
      <CardContent>
        {jobRoles.length === 0 ? (
          <p className="text-sm text-muted-foreground text-center py-6">No job role criteria configured.</p>
        ) : (
          <div className="space-y-4">
            {jobRoles.map(role => (
              <div key={role} className="border border-border rounded-none p-4">
                <div className="flex items-center justify-between mb-3">
                  <h3 className="font-medium">{role}</h3>
                  <div className="flex items-center gap-2">
                    <Badge variant="outline" className="text-xs">
                      {grouped[role].length} criteria
                    </Badge>
                    {editable && (
                      <Button size="icon" variant="ghost" className="text-destructive hover:text-destructive" onClick={() => setDeleteRoleConfirm(role)}>
                        <Trash2 className="h-3.5 w-3.5" />
                      </Button>
                    )}
                  </div>
                </div>
                <div className="space-y-1">
                  {grouped[role].map(c => (
                    <div key={c.criteria_id} className="flex items-center justify-between text-sm py-1">
                      <div>
                        <span className="font-medium">{c.name}</span>
                        <span className="text-muted-foreground ml-2">- {c.question}</span>
                      </div>
                      <Badge variant={c.enabled ? 'default' : 'secondary'} className="text-xs">
                        {c.enabled ? 'Enabled' : 'Disabled'}
                      </Badge>
                    </div>
                  ))}
                </div>
              </div>
            ))}
          </div>
        )}
      </CardContent>

      <ConfirmDialog
        open={deleteRoleConfirm !== null}
        onOpenChange={(open) => { if (!open) setDeleteRoleConfirm(null); }}
        title="Delete job role criteria"
        description={`This will permanently delete all criteria for "${deleteRoleConfirm}". This action cannot be undone.`}
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={() => { if (deleteRoleConfirm) handleDeleteRole(deleteRoleConfirm); setDeleteRoleConfirm(null); }}
      />
    </Card>
  );
}

/* ============ Bonus Tiers Tab ============ */

function BonusTiersTab({ workspaceId, tiers, editable, onRefresh }: {
  workspaceId: string;
  tiers: BonusTierConfig[];
  editable: boolean;
  onRefresh: () => void;
}) {
  const [editing, setEditing] = useState(false);
  const [localTiers, setLocalTiers] = useState(tiers);
  const [saving, setSaving] = useState(false);

  useEffect(() => { setLocalTiers(tiers); }, [tiers]);

  const handleSave = async () => {
    setSaving(true);
    const { error } = await settingsService.updateBonusTiers(
      workspaceId,
      localTiers.map(t => ({
        tier: t.tier,
        min_score: t.min_score,
        max_score: t.max_score,
        salary_multiplier: t.salary_multiplier,
        description: t.description,
        editable: t.editable,
      })),
    );
    setSaving(false);
    if (error) toast.error(error);
    else { toast.success('Bonus tiers updated'); setEditing(false); onRefresh(); }
  };

  const updateTier = (idx: number, field: string, value: string) => {
    setLocalTiers(prev => prev.map((t, i) => i === idx ? { ...t, [field]: field === 'description' ? value : Number(value) } : t));
  };

  const tierVariant = (tier: string): 'default' | 'secondary' | 'destructive' => {
    switch (tier) {
      case 'A': return 'default';
      case 'B': return 'secondary';
      default: return 'destructive';
    }
  };

  return (
    <Card className={LINEAR_CARD_CLASS}>
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle className="text-base">Bonus Tiers</CardTitle>
          {editable && !editing && (
            <Button size="sm" variant="outline" onClick={() => setEditing(true)}>
              <Pencil className="h-3.5 w-3.5 mr-1" /> Edit
            </Button>
          )}
        </div>
      </CardHeader>
      <CardContent>
        <div className="space-y-4">
          {localTiers.map((tier, idx) => (
            <div key={tier.tier} className="border border-border rounded-none p-4 space-y-3">
              <div className="flex items-center gap-2">
                <Badge variant={tierVariant(tier.tier)}>Tier {tier.tier}</Badge>
                {tier.description && <span className="text-sm text-muted-foreground">{tier.description}</span>}
              </div>
              {editing ? (
                <div className="grid grid-cols-3 gap-3">
                  <div className="space-y-1">
                    <Label className="text-xs">Min Score</Label>
                    <Input type="number" value={tier.min_score} onChange={e => updateTier(idx, 'min_score', e.target.value)} />
                  </div>
                  <div className="space-y-1">
                    <Label className="text-xs">Max Score</Label>
                    <Input type="number" value={tier.max_score} onChange={e => updateTier(idx, 'max_score', e.target.value)} />
                  </div>
                  <div className="space-y-1">
                    <Label className="text-xs">Multiplier</Label>
                    <Input type="number" step="0.1" value={tier.salary_multiplier} onChange={e => updateTier(idx, 'salary_multiplier', e.target.value)} />
                  </div>
                </div>
              ) : (
                <div className="flex gap-6 text-sm">
                  <span>Score: {tier.min_score} - {tier.max_score}</span>
                  <span>Multiplier: {tier.salary_multiplier}x</span>
                </div>
              )}
            </div>
          ))}
        </div>
        {editing && (
          <div className="flex gap-2 mt-4 justify-end">
            <Button variant="outline" onClick={() => { setEditing(false); setLocalTiers(tiers); }}>Cancel</Button>
            <Button onClick={handleSave} disabled={saving}>{saving ? 'Saving...' : 'Save'}</Button>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

/* ============ System Tab ============ */

function SystemTab({ workspaceId, config, editable, onRefresh }: {
  workspaceId: string;
  config: WorkspaceSettings['settings'];
  editable: boolean;
  onRefresh: () => void;
}) {
  const [sprintDuration, setSprintDuration] = useState(config.sprint_duration_weeks);
  const [teamWeight, setTeamWeight] = useState(config.team_weight);
  const [notifications, setNotifications] = useState(config.notifications_enabled);
  const [autoCalc, setAutoCalc] = useState(config.auto_calculate_bonuses);
  useEffect(() => {
    setSprintDuration(config.sprint_duration_weeks);
    setTeamWeight(config.team_weight);
    setNotifications(config.notifications_enabled);
    setAutoCalc(config.auto_calculate_bonuses);
  }, [config]);

  const handleSave = async () => {
    const { error } = await settingsService.updateSystem(workspaceId, {
      sprint_duration_weeks: sprintDuration,
      team_weight: teamWeight,
      notifications_enabled: notifications,
      auto_calculate_bonuses: autoCalc,
    });
    if (error) toast.error(error);
    else { toast.success('System settings updated'); onRefresh(); }
  };

  return (
    <div className="space-y-6">
      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader>
          <CardTitle className="text-base">Reward Defaults</CardTitle>
        </CardHeader>
        <CardContent className="space-y-6">
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-2">
              <Label>Sprint Duration (weeks)</Label>
              <Input
                type="number"
                min="1"
                max="4"
                value={sprintDuration}
                onChange={e => setSprintDuration(Number(e.target.value))}
                disabled={!editable}
              />
            </div>
            <div className="space-y-2">
              <Label>Team Weight (%)</Label>
              <Input
                type="number"
                min="0"
                max="100"
                value={teamWeight}
                onChange={e => setTeamWeight(Number(e.target.value))}
                disabled={!editable}
              />
              <p className="text-xs text-muted-foreground">Individual weight: {100 - teamWeight}%</p>
            </div>
          </div>

          <div className="space-y-4">
            <div className="flex items-center justify-between">
              <div>
                <Label>Notifications</Label>
                <p className="text-xs text-muted-foreground">Send email notifications for sprint events</p>
              </div>
              <Switch checked={notifications} onCheckedChange={setNotifications} disabled={!editable} />
            </div>
            <div className="flex items-center justify-between">
              <div>
                <Label>Auto-calculate Bonuses</Label>
                <p className="text-xs text-muted-foreground">Automatically recalculate bonuses when scores change</p>
              </div>
              <Switch checked={autoCalc} onCheckedChange={setAutoCalc} disabled={!editable} />
            </div>
          </div>
        </CardContent>
      </Card>
      {editable && (
        <div className="flex justify-end">
          <Button onClick={handleSave}>Save</Button>
        </div>
      )}
    </div>
  );
}

function AITab({ workspaceId, config, editable, onRefresh }: {
  workspaceId: string;
  config: WorkspaceSettings['settings'];
  editable: boolean;
  onRefresh: () => void;
}) {
  const [planningMethodology, setPlanningMethodology] = useState(config.planning_methodology);
  const [planningWebSearchEnabled, setPlanningWebSearchEnabled] = useState(config.planning_web_search_enabled);
  const [planningWebSearchProvider, setPlanningWebSearchProvider] = useState(config.planning_web_search_provider);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    setPlanningMethodology(config.planning_methodology);
    setPlanningWebSearchEnabled(config.planning_web_search_enabled);
    setPlanningWebSearchProvider(config.planning_web_search_provider);
  }, [config]);

  const handleSave = async () => {
    setSaving(true);
    const { error } = await settingsService.updateSystem(workspaceId, {
      planning_methodology: planningMethodology,
      planning_web_search_enabled: planningWebSearchEnabled,
      planning_web_search_provider: planningWebSearchProvider,
    });
    setSaving(false);
    if (error) toast.error(error);
    else { toast.success('AI settings updated'); onRefresh(); }
  };

  return (
    <div className="space-y-6">
      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader>
          <CardTitle className="text-base">Planning Methodology</CardTitle>
          <CardDescription>
            This controls the hidden prompt pack used for epic `draft_spec` and `plan_stories` runs. It does not change story execution or support prompts yet.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          <div className="space-y-2">
            <Label>Workspace Planning Mode</Label>
            <Select
              value={planningMethodology}
              onValueChange={(value) => setPlanningMethodology(value as 'structured_v1' | 'basic_v1')}
              disabled={!editable}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="structured_v1">Structured v1</SelectItem>
                <SelectItem value="basic_v1">Basic v1</SelectItem>
              </SelectContent>
            </Select>
            <p className="text-xs text-muted-foreground">
              `Structured v1` uses a stronger internal planning methodology inspired by staged PM and architecture roles. `Basic v1` keeps planning simpler for fallback and comparison.
            </p>
          </div>

          {editable && (
            <div className="flex justify-end">
              <Button onClick={handleSave} disabled={saving}>
                {saving ? 'Saving...' : 'Save Settings'}
              </Button>
            </div>
          )}
        </CardContent>
      </Card>

      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader>
          <CardTitle className="text-base">External Research</CardTitle>
          <CardDescription>
            Product planners can use controlled web research during `draft_spec` to gather market context, standards, and external evidence. Sources are cited back into the saved PRD.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          <div className="flex items-center justify-between gap-4 rounded-md border p-4">
            <div className="space-y-1">
              <Label className="text-sm font-medium">Enable planning web research</Label>
              <p className="text-xs text-muted-foreground">
                This only affects `product_planner` during the PRD drafting stage. Story planning remains repo/spec-driven.
              </p>
            </div>
            <Switch
              checked={planningWebSearchEnabled}
              onCheckedChange={setPlanningWebSearchEnabled}
              disabled={!editable}
            />
          </div>

          <div className="space-y-2">
            <Label>Research Provider</Label>
            <Select
              value={planningWebSearchProvider}
              onValueChange={(value) => setPlanningWebSearchProvider(value as 'brave')}
              disabled={!editable || !planningWebSearchEnabled}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="brave">Brave Search</SelectItem>
              </SelectContent>
            </Select>
            <p className="text-xs text-muted-foreground">
              Brave is the first supported provider. The API server will reject enabling research if Brave credentials are not configured.
            </p>
          </div>

          {editable && (
            <div className="flex justify-end">
              <Button onClick={handleSave} disabled={saving}>
                {saving ? 'Saving...' : 'Save Settings'}
              </Button>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

function ProjectDeliveryTab({ workspaceId, editable }: {
  workspaceId: string;
  editable: boolean;
}) {
  const [integrations, setIntegrations] = useState<GitIntegration[]>([]);
  const [repositories, setRepositories] = useState<GitRepository[]>([]);
  const [runnerHealth, setRunnerHealth] = useState<RunnerHealth | null>(null);
  const [syncingIntegrationId, setSyncingIntegrationId] = useState<string | null>(null);
  const [installingGitHubApp, setInstallingGitHubApp] = useState(false);
  const [installActionError, setInstallActionError] = useState<string | null>(null);
  const [integrationDialogOpen, setIntegrationDialogOpen] = useState(false);
  const [creatingIntegration, setCreatingIntegration] = useState(false);
  const [integrationName, setIntegrationName] = useState('');
  const [accountLogin, setAccountLogin] = useState('');
  const [installationId, setInstallationId] = useState('');
  const [baseUrl, setBaseURL] = useState('');

  const loadGitStatus = useCallback(async () => {
    const [integrationsRes, reposRes, runnerRes] = await Promise.all([
      gitService.listIntegrations(workspaceId),
      gitService.listRepositories(workspaceId, { all: true }),
      agentService.getRunnerHealth(workspaceId),
    ]);
    setIntegrations(integrationsRes.data ?? []);
    setRepositories(reposRes.data ?? []);
    setRunnerHealth(runnerRes.data ?? null);
  }, [workspaceId]);

  useEffect(() => {
    void loadGitStatus();
  }, [loadGitStatus]);

  useEffect(() => {
    const url = new URL(window.location.href);
    const status = url.searchParams.get('github_app');
    const message = url.searchParams.get('github_message');
    if (!status) return;

    if (status === 'connected') {
      setInstallActionError(null);
      toast.success(message || 'GitHub App connected');
    } else {
      toast.error(message || 'GitHub App connection failed');
    }

    url.searchParams.delete('github_app');
    url.searchParams.delete('github_message');
    url.searchParams.delete('integration_id');
    url.searchParams.delete('repo_count');
    const nextQuery = url.searchParams.toString();
    window.history.replaceState({}, '', `${url.pathname}${nextQuery ? `?${nextQuery}` : ''}${url.hash}`);
    void loadGitStatus();
  }, [loadGitStatus]);

  const installGuidance = useMemo(() => {
    if (!installActionError) return null;
    const normalized = installActionError.toLowerCase();
    if (normalized.includes('github app onboarding is not configured')) {
      return {
        title: 'GitHub App server setup required',
        description: 'The API server is missing GitHub App configuration, so it cannot generate the install URL yet.',
        details: ['GITHUB_APP_ID', 'GITHUB_APP_SLUG', 'GITHUB_APP_PRIVATE_KEY (base64 PEM)', 'APP_BASE_URL'],
      };
    }
    if (normalized.includes('workspace_id is required')) {
      return {
        title: 'Workspace context is missing',
        description: 'The request did not include a workspace ID. Refresh the page and try again from the workspace settings route.',
        details: [] as string[],
      };
    }
    return {
      title: 'GitHub App install failed',
      description: installActionError,
      details: [] as string[],
    };
  }, [installActionError]);

  return (
    <div className="space-y-6">
      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader>
          <CardTitle className="text-base">GitHub & Runners</CardTitle>
          <CardDescription>Connected delivery integrations, repository sync, and shared runner queues</CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          {/* Stats overview */}
          <div className="grid gap-4 md:grid-cols-4">
            <div className="rounded-lg border border-border/60 p-4">
              <div className="flex items-center gap-2">
                <GitBranch className="h-3.5 w-3.5 text-muted-foreground" />
                <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">Integrations</p>
              </div>
              <p className="mt-2 text-2xl font-semibold">{integrations.length}</p>
            </div>
            <div className="rounded-lg border border-border/60 p-4">
              <div className="flex items-center gap-2">
                <GitPullRequest className="h-3.5 w-3.5 text-muted-foreground" />
                <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">Repositories</p>
              </div>
              <p className="mt-2 text-2xl font-semibold">{repositories.length}</p>
            </div>
            <div className="rounded-lg border border-border/60 p-4">
              <div className="flex items-center gap-2">
                <Server className="h-3.5 w-3.5 text-muted-foreground" />
                <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">Runner Queues</p>
              </div>
              <p className="mt-2 text-2xl font-semibold">{runnerHealth?.queues?.length ?? 0}</p>
              {runnerHealth?.namespace && (
                <p className="mt-1 text-xs text-muted-foreground">Namespace: {runnerHealth.namespace}</p>
              )}
            </div>
            <div className="rounded-lg border border-border/60 p-4">
              <div className="flex items-center gap-2">
                <Zap className="h-3.5 w-3.5 text-muted-foreground" />
                <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">Active Runs</p>
              </div>
              <p className="mt-2 text-2xl font-semibold">{runnerHealth?.active_runs?.length ?? 0}</p>
              <p className="mt-1 text-xs text-muted-foreground">
                <span className={runnerHealth?.temporal_configured
                  ? 'text-green-600 dark:text-green-400'
                  : 'text-muted-foreground'
                }>
                  {runnerHealth?.temporal_configured ? 'Temporal connected' : 'Temporal not configured'}
                </span>
              </p>
            </div>
          </div>

          <Separator />

          {/* GitHub App Onboarding */}
          <div>
            <div className="mb-3 flex items-center gap-2">
              <GitBranch className="h-4 w-4 text-muted-foreground" />
              <h4 className="text-sm font-semibold">GitHub App</h4>
            </div>
            <div className="rounded-lg border border-border/60 p-4">
              <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
                <div>
                  <p className="font-medium">Install or connect</p>
                  <p className="text-sm text-muted-foreground">
                    Install the workspace GitHub App, then return here for automatic integration creation and repository sync.
                  </p>
                </div>
                {editable ? (
                  <div className="flex flex-shrink-0 flex-wrap gap-2">
                    <Button
                      variant="outline"
                      size="sm"
                      disabled={installingGitHubApp}
                      className="gap-1.5"
                      onClick={async () => {
                        setInstallActionError(null);
                        setInstallingGitHubApp(true);
                        const { data, error } = await gitService.getGitHubInstallURL(workspaceId);
                        setInstallingGitHubApp(false);
                        if (error || !data?.install_url) {
                          const message = error || 'GitHub App install URL is not available';
                          setInstallActionError(message);
                          toast.error(message);
                          return;
                        }
                        window.location.assign(data.install_url);
                      }}
                    >
                      {installingGitHubApp ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : null}
                      {installingGitHubApp ? 'Opening GitHub...' : 'Install GitHub App'}
                    </Button>
                    <Button variant="ghost" size="sm" className="gap-1.5" onClick={() => setIntegrationDialogOpen(true)}>
                      <Plus className="h-3.5 w-3.5" />
                      Manual registration
                    </Button>
                  </div>
                ) : null}
              </div>
            </div>

            {installGuidance ? (
              <div className="mt-3 rounded-lg border border-destructive/30 bg-destructive/5 p-4">
                <div className="flex items-center gap-2">
                  <Badge variant="destructive">Setup required</Badge>
                  <p className="font-medium">{installGuidance.title}</p>
                </div>
                <p className="mt-2 text-sm text-muted-foreground">{installGuidance.description}</p>
                {installGuidance.details.length ? (
                  <p className="mt-2 text-sm text-muted-foreground">
                    Required envs: {installGuidance.details.map((item, index) => (
                      <span key={item}>
                        <code className="rounded bg-background px-1 py-0.5 text-xs">{item}</code>
                        {index < installGuidance.details.length - 1 ? ', ' : ''}
                      </span>
                    ))}
                  </p>
                ) : null}
                {installActionError && installActionError !== installGuidance.description ? (
                  <p className="mt-2 text-xs text-muted-foreground">Backend response: {installActionError}</p>
                ) : null}
              </div>
            ) : null}

            {/* Integrations list */}
            {integrations.length === 0 ? (
              <div className="mt-3 rounded-lg border border-dashed border-border px-4 py-6 text-center text-sm text-muted-foreground">
                No GitHub integrations connected yet.
              </div>
            ) : (
              <div className="mt-3 space-y-2">
                {integrations.map((integration) => (
                  <div key={integration.id} className="flex flex-col gap-3 rounded-lg border border-border/60 p-4 md:flex-row md:items-center md:justify-between">
                    <div>
                      <div className="flex items-center gap-2">
                        <p className="font-medium">{integration.display_name}</p>
                        <Badge
                          variant="outline"
                          className={integration.active
                            ? 'border-green-500/30 bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
                            : ''
                          }
                        >
                          {integration.active ? 'Active' : 'Inactive'}
                        </Badge>
                      </div>
                      <p className="mt-0.5 text-xs text-muted-foreground">
                        {integration.provider}
                        {integration.account_login ? ` · ${integration.account_login}` : ''}
                        {integration.installation_id ? ` · installation ${integration.installation_id}` : ''}
                        {integration.last_synced_at ? ` · synced ${new Date(integration.last_synced_at).toLocaleString()}` : ''}
                      </p>
                      {integration.last_sync_error && (
                        <p className="mt-1 text-xs text-destructive">{integration.last_sync_error}</p>
                      )}
                    </div>
                    <Button
                      variant="outline"
                      size="sm"
                      className="gap-1.5"
                      disabled={syncingIntegrationId === integration.id}
                      onClick={async () => {
                        setSyncingIntegrationId(integration.id);
                        const { error } = await gitService.syncRepositories(workspaceId, integration.id);
                        setSyncingIntegrationId(null);
                        if (error) {
                          toast.error(error);
                          return;
                        }
                        toast.success('Repositories synced');
                        await loadGitStatus();
                      }}
                    >
                      {syncingIntegrationId === integration.id
                        ? <Loader2 className="h-3.5 w-3.5 animate-spin" />
                        : <RefreshCw className="h-3.5 w-3.5" />
                      }
                      {syncingIntegrationId === integration.id ? 'Syncing...' : 'Sync'}
                    </Button>
                  </div>
                ))}
              </div>
            )}
          </div>

          <Separator />

          {/* Repository Catalog */}
          <div>
            <div className="mb-3 flex items-center gap-2">
              <GitPullRequest className="h-4 w-4 text-muted-foreground" />
              <h4 className="text-sm font-semibold">Repository Catalog</h4>
              {repositories.length > 0 && (
                <span className="rounded-full bg-muted px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground">
                  {repositories.length}
                </span>
              )}
            </div>
            <p className="mb-3 text-xs text-muted-foreground">Choose which synced repositories are available to teams and story delivery targets.</p>
            {repositories.length ? (
              <div className="space-y-2">
                {repositories.map((repo) => (
                  <div key={repo.id} className="rounded-lg border border-border/60 p-4">
                    <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
                      <div>
                        <div className="flex flex-wrap items-center gap-2">
                          <p className="font-medium">{repo.full_name}</p>
                          <Badge variant="outline" className="text-[10px]">{repo.private ? 'Private' : 'Public'}</Badge>
                          {repo.archived ? <Badge variant="secondary" className="text-[10px]">Archived</Badge> : null}
                        </div>
                        <p className="mt-0.5 text-xs text-muted-foreground">
                          <span className="font-mono">{repo.default_branch}</span>
                        </p>
                      </div>
                      <div className="flex items-center gap-3">
                        <span className="text-xs text-muted-foreground">Available for delivery</span>
                        <Switch
                          checked={repo.selected}
                          disabled={!editable || repo.archived}
                          onCheckedChange={async (checked) => {
                            const { error } = await gitService.updateRepository(workspaceId, repo.id, { selected: checked });
                            if (error) {
                              toast.error(error);
                              return;
                            }
                            toast.success(`${repo.full_name} ${checked ? 'enabled' : 'hidden'} for delivery`);
                            await loadGitStatus();
                          }}
                        />
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <div className="rounded-lg border border-dashed border-border px-4 py-6 text-center text-sm text-muted-foreground">
                No repositories have been synced yet.
              </div>
            )}
          </div>

          {/* Runner Queues */}
          {runnerHealth?.queues?.length ? (
            <>
              <Separator />
              <div>
                <div className="mb-3 flex items-center gap-2">
                  <Server className="h-4 w-4 text-muted-foreground" />
                  <h4 className="text-sm font-semibold">Runner Queues</h4>
                </div>
                <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
                  {runnerHealth.queues.map((queue) => (
                    <div key={queue.name} className="rounded-lg border border-border/60 p-4">
                      <div className="flex items-center justify-between gap-2">
                        <p className="text-sm font-medium">{queue.name}</p>
                        <Badge variant="outline" className="text-[10px]">x{queue.concurrency}</Badge>
                      </div>
                      <div className="mt-3 grid grid-cols-2 gap-3 text-xs">
                        <div>
                          <p className="text-[10px] font-medium uppercase tracking-wide text-muted-foreground">Queued</p>
                          <p className="mt-0.5 text-sm font-medium">{queue.queued_runs}</p>
                        </div>
                        <div>
                          <p className="text-[10px] font-medium uppercase tracking-wide text-muted-foreground">Running</p>
                          <p className="mt-0.5 text-sm font-medium">{queue.running_runs}</p>
                        </div>
                        <div>
                          <p className="text-[10px] font-medium uppercase tracking-wide text-muted-foreground">Awaiting approval</p>
                          <p className="mt-0.5 text-sm font-medium">{queue.awaiting_approval_runs}</p>
                        </div>
                        <div>
                          <p className="text-[10px] font-medium uppercase tracking-wide text-muted-foreground">Heartbeat</p>
                          <p className="mt-0.5 text-sm font-medium">
                            {queue.latest_heartbeat_at ? new Date(queue.latest_heartbeat_at).toLocaleTimeString() : 'n/a'}
                          </p>
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            </>
          ) : null}

          <Separator />

          {/* In-flight Runs */}
          <div>
            <div className="mb-3 flex items-center gap-2">
              <Zap className="h-4 w-4 text-muted-foreground" />
              <h4 className="text-sm font-semibold">In-flight Runs</h4>
              {(runnerHealth?.active_runs?.length ?? 0) > 0 && (
                <span className="rounded-full bg-blue-100 px-1.5 py-0.5 text-[10px] font-medium text-blue-700 dark:bg-blue-900/30 dark:text-blue-400">
                  {runnerHealth!.active_runs.length}
                </span>
              )}
            </div>
            <p className="mb-3 text-xs text-muted-foreground">Queued, running, and approval-pending runs in this workspace.</p>
            {runnerHealth?.active_runs?.length ? (
              <div className="space-y-2">
                {runnerHealth.active_runs.map((run) => (
                  <div key={run.id} className="rounded-lg border border-border/60 p-4">
                    <div className="flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
                      <div>
                        <div className="flex items-center gap-2">
                          <p className="text-sm font-medium">{run.task_queue}</p>
                          <Badge
                            variant={run.stale ? 'destructive' : 'outline'}
                            className={!run.stale && run.status === 'running'
                              ? 'border-blue-500/30 bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400'
                              : ''
                            }
                          >
                            {run.status}
                          </Badge>
                          {run.execution_stage ? <Badge variant="secondary" className="text-[10px]">{run.execution_stage}</Badge> : null}
                        </div>
                        <p className="mt-0.5 text-xs text-muted-foreground">
                          {run.target_type} · <span className="font-mono">{run.target_id.slice(0, 8)}</span>
                          {run.workflow_id ? ` · ${run.workflow_id}` : ''}
                        </p>
                      </div>
                      <div className="text-xs text-muted-foreground">
                        <p>Started: {run.started_at ? new Date(run.started_at).toLocaleString() : 'Pending'}</p>
                        <p>Heartbeat: {run.last_heartbeat_at ? new Date(run.last_heartbeat_at).toLocaleString() : 'n/a'}</p>
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <div className="rounded-lg border border-dashed border-border px-4 py-8 text-center">
                <Zap className="mx-auto h-8 w-8 text-muted-foreground/30" />
                <p className="mt-2 text-sm text-muted-foreground">No in-flight runs right now.</p>
              </div>
            )}
          </div>
        </CardContent>
      </Card>

      <Dialog open={integrationDialogOpen} onOpenChange={setIntegrationDialogOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>Connect GitHub App</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <p className="text-sm text-muted-foreground">
              Fallback for environments where the install handshake is not available. Register a GitHub App installation manually so shared runners can mint short-lived installation tokens.
            </p>
            <div className="space-y-2">
              <Label>Display name</Label>
              <Input
                value={integrationName}
                onChange={(event) => setIntegrationName(event.target.value)}
                placeholder="GitHub Production"
              />
            </div>
            <div className="space-y-2">
              <Label>Account / org</Label>
              <Input
                value={accountLogin}
                onChange={(event) => setAccountLogin(event.target.value)}
                placeholder="acme-inc"
              />
            </div>
            <div className="space-y-2">
              <Label>Installation ID</Label>
              <Input
                value={installationId}
                onChange={(event) => setInstallationId(event.target.value)}
                placeholder="12345678"
              />
            </div>
            <div className="space-y-2">
              <Label>Base URL</Label>
              <Input
                value={baseUrl}
                onChange={(event) => setBaseURL(event.target.value)}
                placeholder="https://github.com"
              />
            </div>
          </div>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setIntegrationDialogOpen(false)}
              disabled={creatingIntegration}
            >
              Cancel
            </Button>
            <Button
              disabled={creatingIntegration || !integrationName.trim() || !installationId.trim()}
              onClick={async () => {
                setCreatingIntegration(true);
                const { error } = await gitService.createIntegration(workspaceId, {
                  provider: 'github',
                  display_name: integrationName.trim(),
                  credential_mode: 'github_app',
                  account_login: accountLogin.trim() || undefined,
                  installation_id: installationId.trim(),
                  base_url: baseUrl.trim() || undefined,
                });
                setCreatingIntegration(false);
                if (error) {
                  toast.error(error);
                  return;
                }
                toast.success('GitHub App integration connected');
                setIntegrationDialogOpen(false);
                setIntegrationName('');
                setAccountLogin('');
                setInstallationId('');
                setBaseURL('');
                await loadGitStatus();
              }}
            >
              {creatingIntegration ? 'Connecting...' : 'Connect'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

/* ============ Workflows Tab ============ */

function WorkflowsTab({ workspaceId, teams, editable, initialTeamId }: {
  workspaceId: string;
  teams: WorkspaceTeam[];
  editable: boolean;
  initialTeamId?: string;
}) {
  const navigate = useNavigate();
  const { currentWorkspace } = useWorkspaceStore();
  const [workflows, setWorkflows] = useState<WorkflowWithStates[]>([]);
  const [loading, setLoading] = useState(true);
  const [teamFilter, setTeamFilter] = useState<string>(initialTeamId || '__all__');
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editWorkflow, setEditWorkflow] = useState<WorkflowWithStates | null>(null);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [teamID, setTeamID] = useState<string>('none');
  const [autoAssignOwner, setAutoAssignOwner] = useState(false);
  const [saving, setSaving] = useState(false);
  const [deleteWorkflowConfirm, setDeleteWorkflowConfirm] = useState<string | null>(null);

  const loadWorkflows = async () => {
    setLoading(true);
    const { data, error } = await pmWorkflowService.list(workspaceId);
    if (error) {
      toast.error(error);
      setWorkflows([]);
    } else {
      setWorkflows(data ?? []);
    }
    setLoading(false);
  };

  useEffect(() => { loadWorkflows(); }, [workspaceId]);

  const openCreate = () => {
    setEditWorkflow(null);
    setName('');
    setDescription('');
    setTeamID('none');
    setAutoAssignOwner(false);
    setDialogOpen(true);
  };

  const openEdit = (workflow: WorkflowWithStates) => {
    setEditWorkflow(workflow);
    setName(workflow.workflow.name);
    setDescription(workflow.workflow.description ?? '');
    setTeamID(workflow.workflow.team_id ?? 'none');
    setAutoAssignOwner(workflow.workflow.auto_assign_owner);
    setDialogOpen(true);
  };

  const handleSave = async (e: FormEvent) => {
    e.preventDefault();
    if (!name.trim()) return;
    setSaving(true);

    const normalizedTeamID = teamID === 'none' ? undefined : teamID;
    if (editWorkflow) {
      const { error } = await pmWorkflowService.update(workspaceId, editWorkflow.workflow.id, {
        name: name.trim(),
        description: description.trim() || undefined,
        team_id: normalizedTeamID,
        auto_assign_owner: autoAssignOwner,
      });
      if (error) toast.error(error);
      else {
        toast.success('Workflow updated');
        setDialogOpen(false);
        await loadWorkflows();
      }
    } else {
      const { error } = await pmWorkflowService.create({
        workspace_id: workspaceId,
        name: name.trim(),
        description: description.trim() || undefined,
        team_id: normalizedTeamID,
        auto_assign_owner: autoAssignOwner,
      });
      if (error) toast.error(error);
      else {
        toast.success('Workflow created');
        setDialogOpen(false);
        await loadWorkflows();
      }
    }
    setSaving(false);
  };

  const handleDelete = async (workflowID: string) => {
    const { error } = await pmWorkflowService.remove(workspaceId, workflowID);
    if (error) toast.error(error);
    else {
      toast.success('Workflow deleted');
      await loadWorkflows();
    }
  };

  const findTeamName = (id?: string) => {
    if (!id) return 'Workspace Default';
    return teams.find((team) => team.id === id)?.name ?? 'Unknown Team';
  };

  const filteredWorkflows = teamFilter === '__all__'
    ? workflows
    : teamFilter === '__default__'
      ? workflows.filter((w) => !w.workflow.team_id)
      : workflows.filter((w) => w.workflow.team_id === teamFilter);

  return (
    <Card className={LINEAR_CARD_CLASS}>
      <CardHeader>
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <CardTitle className="text-base">Workflows</CardTitle>
            <Badge variant="outline" className="text-xs font-normal">{filteredWorkflows.length}</Badge>
          </div>
          <div className="flex items-center gap-2">
            <Select value={teamFilter} onValueChange={setTeamFilter}>
              <SelectTrigger className="h-8 w-[220px] text-sm">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="__all__">All workflows</SelectItem>
                <SelectItem value="__default__">Workspace default</SelectItem>
                {teams.map((team) => (
                  <SelectItem key={team.id} value={team.id}>
                    {team.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            {editable && (
              <Button size="sm" onClick={openCreate}>
                <Plus className="h-4 w-4 mr-1" /> Create Workflow
              </Button>
            )}
          </div>
        </div>
      </CardHeader>
      <CardContent>
        {loading ? (
          <p className="text-sm text-muted-foreground text-center py-6">Loading workflows...</p>
        ) : filteredWorkflows.length === 0 ? (
          <p className="text-sm text-muted-foreground text-center py-6">No workflows found.</p>
        ) : (
          <div className="space-y-1.5">
            {filteredWorkflows.map((workflow) => (
              <div key={workflow.workflow.id} className="rounded-none border border-border px-3 py-2">
                <div className="flex items-center justify-between gap-2">
                  <div>
                    <div className="flex items-center gap-2">
                      <p className="text-sm font-medium">{workflow.workflow.name}</p>
                      {!workflow.workflow.team_id && (
                        <Badge variant="secondary" className="text-xs">Workspace Default</Badge>
                      )}
                    </div>
                    <p className="text-xs text-muted-foreground">
                      {workflow.workflow.description || 'No description'}
                    </p>
                  </div>
                  <div className="flex items-center gap-2 text-xs text-muted-foreground">
                    <span>{workflow.states.length} states</span>
                    <span>•</span>
                    <span>{findTeamName(workflow.workflow.team_id)}</span>
                    <span>•</span>
                    <span>{workflow.workflow.auto_assign_owner ? 'Auto-assign owner' : 'Manual owner'}</span>
                  </div>
                </div>
                {editable && (
                  <div className="mt-1.5 flex justify-end gap-1">
                    <Button
                      size="sm"
                      variant="ghost"
                      className="h-7 gap-1 text-xs"
                      onClick={() => {
                        const slug = currentWorkspace?.slug;
                        if (slug) {
                          navigate({
                            to: '/w/$slug/settings/$section',
                            params: { slug, section: 'workflowstates' },
                            search: { workflow: workflow.workflow.id },
                          });
                        }
                      }}
                    >
                      <ListTree className="h-3.5 w-3.5" />
                      Modify States
                    </Button>
                    <Button size="icon" variant="ghost" onClick={() => openEdit(workflow)}>
                      <Pencil className="h-3.5 w-3.5" />
                    </Button>
                    <Button size="icon" variant="ghost" className="text-destructive hover:text-destructive" onClick={() => setDeleteWorkflowConfirm(workflow.workflow.id)}>
                      <Trash2 className="h-3.5 w-3.5" />
                    </Button>
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
      </CardContent>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent>
          <form onSubmit={handleSave}>
            <DialogHeader>
              <DialogTitle>{editWorkflow ? 'Edit Workflow' : 'Create Workflow'}</DialogTitle>
            </DialogHeader>
            <div className="space-y-4 py-4">
              <div className="space-y-2">
                <Label>Name</Label>
                <Input value={name} onChange={(e) => setName(e.target.value)} required />
              </div>
              <div className="space-y-2">
                <Label>Description</Label>
                <Input value={description} onChange={(e) => setDescription(e.target.value)} />
              </div>
              <div className="space-y-2">
                <Label>Team</Label>
                <Select value={teamID} onValueChange={setTeamID}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="none">Workspace Default</SelectItem>
                    {teams.map((team) => (
                      <SelectItem key={team.id} value={team.id}>{team.name}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="flex items-center justify-between rounded-none border border-border px-3 py-2">
                <div>
                  <Label>Auto assign owner when moved to started state</Label>
                  <p className="text-xs text-muted-foreground">Assign current user when story enters a started state and has no owner.</p>
                </div>
                <Switch checked={autoAssignOwner} onCheckedChange={setAutoAssignOwner} />
              </div>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setDialogOpen(false)}>Cancel</Button>
              <Button type="submit" disabled={saving}>{saving ? 'Saving...' : 'Save'}</Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={deleteWorkflowConfirm !== null}
        onOpenChange={(open) => { if (!open) setDeleteWorkflowConfirm(null); }}
        title="Delete workflow"
        description="This will permanently delete the workflow and all its states. Stories using this workflow will need to be reassigned. This action cannot be undone."
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={() => { if (deleteWorkflowConfirm) handleDelete(deleteWorkflowConfirm); setDeleteWorkflowConfirm(null); }}
      />
    </Card>
  );
}

/* ============ Workflow States Tab ============ */

const STATE_TYPE_ORDER: StateType[] = ['backlog', 'unstarted', 'started', 'done'];
const STATE_TYPE_LABEL: Record<StateType, string> = {
  backlog: 'Backlog',
  unstarted: 'Unstarted',
  started: 'Started',
  done: 'Done',
};

function WorkflowStatesTab({ workspaceId, editable, initialWorkflowId }: {
  workspaceId: string;
  editable: boolean;
  initialWorkflowId?: string;
}) {
  const [workflows, setWorkflows] = useState<WorkflowWithStates[]>([]);
  const [selectedWorkflowID, setSelectedWorkflowID] = useState<string>('');
  const [loading, setLoading] = useState(true);

  const [dialogOpen, setDialogOpen] = useState(false);
  const [editState, setEditState] = useState<WorkflowState | null>(null);
  const [newStateType, setNewStateType] = useState<StateType>('unstarted');
  const [stateName, setStateName] = useState('');
  const [stateDescription, setStateDescription] = useState('');
  const [stateColor, setStateColor] = useState('');
  const [stateWIP, setStateWIP] = useState('');
  const [stateDefault, setStateDefault] = useState(false);
  const [deleteStateConfirm, setDeleteStateConfirm] = useState(false);
  const [saving, setSaving] = useState(false);

  const loadWorkflows = async () => {
    setLoading(true);
    const { data, error } = await pmWorkflowService.list(workspaceId);
    if (error) {
      toast.error(error);
      setWorkflows([]);
      setLoading(false);
      return;
    }
    const next = data ?? [];
    setWorkflows(next);
    setSelectedWorkflowID((prev) => {
      // Prefer URL param, then keep existing selection, then fall back to first
      if (initialWorkflowId && next.some((workflow) => workflow.workflow.id === initialWorkflowId)) return initialWorkflowId;
      if (prev && next.some((workflow) => workflow.workflow.id === prev)) return prev;
      return next[0]?.workflow.id ?? '';
    });
    setLoading(false);
  };

  useEffect(() => { loadWorkflows(); }, [workspaceId]);

  const selectedWorkflow = workflows.find((workflow) => workflow.workflow.id === selectedWorkflowID) ?? null;
  const sortedStates = selectedWorkflow
    ? [...selectedWorkflow.states].sort((a, b) => a.position - b.position)
    : [];

  const statesByType: Record<StateType, WorkflowState[]> = {
    backlog: sortedStates.filter((state) => state.state_type === 'backlog'),
    unstarted: sortedStates.filter((state) => state.state_type === 'unstarted'),
    started: sortedStates.filter((state) => state.state_type === 'started'),
    done: sortedStates.filter((state) => state.state_type === 'done'),
  };

  const orderedStateIDs = (workflow: WorkflowWithStates) =>
    STATE_TYPE_ORDER.flatMap((type) =>
      [...workflow.states]
        .filter((state) => state.state_type === type)
        .sort((a, b) => a.position - b.position)
        .map((state) => state.id)
    );

  const normalizeOrdering = async (workflow: WorkflowWithStates) => {
    const targetIDs = orderedStateIDs(workflow);
    const currentIDs = [...workflow.states].sort((a, b) => a.position - b.position).map((state) => state.id);
    if (targetIDs.length === currentIDs.length && targetIDs.every((id, idx) => currentIDs[idx] === id)) return;
    await pmWorkflowService.reorderStates(workspaceId, workflow.workflow.id, targetIDs);
  };

  const openCreateForType = (type: StateType) => {
    setEditState(null);
    setNewStateType(type);
    setStateName('');
    setStateDescription('');
    setStateColor('');
    setStateWIP('');
    setStateDefault(false);
    setDialogOpen(true);
  };

  const openEdit = (state: WorkflowState) => {
    setEditState(state);
    setNewStateType(state.state_type);
    setStateName(state.name);
    setStateDescription(state.description ?? '');
    setStateColor(state.color ?? '');
    setStateWIP(state.wip_limit ? String(state.wip_limit) : '');
    setStateDefault(state.is_default);
    setDialogOpen(true);
  };

  const handleSaveState = async (e: FormEvent) => {
    e.preventDefault();
    if (!selectedWorkflow || !stateName.trim()) return;
    setSaving(true);
    const payload = {
      name: stateName.trim(),
      state_type: newStateType,
      description: stateDescription.trim() || undefined,
      color: stateColor.trim() || undefined,
      wip_limit: stateWIP.trim() ? Number(stateWIP) : undefined,
      is_default: stateDefault,
    };

    if (editState) {
      const { error } = await pmWorkflowService.updateState(workspaceId, selectedWorkflow.workflow.id, editState.id, payload);
      if (error) toast.error(error);
      else toast.success('State updated');
    } else {
      const { error } = await pmWorkflowService.createState(workspaceId, selectedWorkflow.workflow.id, payload);
      if (error) toast.error(error);
      else {
        toast.success('State created');
        const refreshed = await pmWorkflowService.get(workspaceId, selectedWorkflow.workflow.id);
        if (refreshed.data) {
          await normalizeOrdering(refreshed.data);
        }
      }
    }
    await loadWorkflows();
    setDialogOpen(false);
    setSaving(false);
  };

  const handleDeleteState = async () => {
    if (!selectedWorkflow || !editState) return;
    const { error } = await pmWorkflowService.removeState(workspaceId, selectedWorkflow.workflow.id, editState.id);
    if (error) toast.error(error);
    else {
      toast.success('State deleted');
      setDialogOpen(false);
      await loadWorkflows();
    }
  };

  const handleMoveWithinType = async (type: StateType, stateID: string, direction: 'up' | 'down') => {
    if (!selectedWorkflow) return;
    const typed = [...statesByType[type]];
    const idx = typed.findIndex((state) => state.id === stateID);
    if (idx < 0) return;
    const swapIdx = direction === 'up' ? idx - 1 : idx + 1;
    if (swapIdx < 0 || swapIdx >= typed.length) return;
    const copy = [...typed];
    const [current] = copy.splice(idx, 1);
    copy.splice(swapIdx, 0, current);

    const idsByType: Record<StateType, string[]> = {
      backlog: statesByType.backlog.map((state) => state.id),
      unstarted: statesByType.unstarted.map((state) => state.id),
      started: statesByType.started.map((state) => state.id),
      done: statesByType.done.map((state) => state.id),
    };
    idsByType[type] = copy.map((state) => state.id);
    const nextIDs = STATE_TYPE_ORDER.flatMap((stateType) => idsByType[stateType]);

    const { error } = await pmWorkflowService.reorderStates(workspaceId, selectedWorkflow.workflow.id, nextIDs);
    if (error) toast.error(error);
    else await loadWorkflows();
  };

  const toggleAutoAssignOwner = async (checked: boolean) => {
    if (!selectedWorkflow) return;
    const { error } = await pmWorkflowService.update(workspaceId, selectedWorkflow.workflow.id, {
      auto_assign_owner: checked,
    });
    if (error) toast.error(error);
    else await loadWorkflows();
  };

  return (
    <Card className={LINEAR_CARD_CLASS}>
      <CardContent className="pt-6 space-y-5">
        {loading ? (
          <p className="text-sm text-muted-foreground">Loading workflows...</p>
        ) : workflows.length === 0 ? (
          <p className="text-sm text-muted-foreground">No workflows found. Create one in the Workflows tab first.</p>
        ) : (
          <>
            <div className="grid gap-3 md:grid-cols-2">
              <div className="space-y-2">
                <Label>Workflow</Label>
                <Select value={selectedWorkflowID} onValueChange={setSelectedWorkflowID}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    {workflows.map((workflow) => (
                      <SelectItem key={workflow.workflow.id} value={workflow.workflow.id}>
                        {workflow.workflow.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              {selectedWorkflow && (
                <div className="flex items-end">
                  <div className="flex w-full items-center justify-between rounded-none border border-border px-2.5 py-2">
                    <div>
                      <Label>Auto assign owner</Label>
                      <p className="text-xs text-muted-foreground">Assign current user when stories move into started state without owner.</p>
                    </div>
                    <Switch
                      checked={selectedWorkflow.workflow.auto_assign_owner}
                      disabled={!editable}
                      onCheckedChange={toggleAutoAssignOwner}
                    />
                  </div>
                </div>
              )}
            </div>

            {selectedWorkflow && (
              <div className="space-y-4">
                {STATE_TYPE_ORDER.map((type) => (
                  <section key={type} className="space-y-1.5">
                    <div className="flex items-center justify-between">
                      <h4 className="flex items-center gap-1.5 text-xs font-semibold uppercase tracking-wide">
                        <StateTypeIcon stateType={type} className="h-3.5 w-3.5" />
                        {STATE_TYPE_LABEL[type]}
                      </h4>
                      {editable && (
                        <Button variant="ghost" size="sm" onClick={() => openCreateForType(type)}>
                          <Plus className="h-3.5 w-3.5 mr-1" /> Add
                        </Button>
                      )}
                    </div>
                    {statesByType[type].length === 0 ? (
                      <div className="rounded-none border border-dashed border-border px-3 py-2 text-xs text-muted-foreground">
                        No states in this group.
                      </div>
                    ) : (
                      <div className="space-y-1.5">
                        {statesByType[type].map((state, idx) => (
                          <div key={state.id} className="rounded-none border border-border px-2.5 py-1.5">
                            <div className="flex items-start justify-between gap-2">
                              <div>
                                <div className="flex items-center gap-1.5">
                                  <StateTypeIcon stateType={state.state_type} className="h-3.5 w-3.5" />
                                  <p className="text-sm font-medium">{state.name}</p>
                                  {state.is_default && (
                                    <Badge variant="secondary" className="text-xs">Default</Badge>
                                  )}
                                  {state.wip_limit ? (
                                    <Badge variant="outline" className="text-xs">WIP {state.wip_limit}</Badge>
                                  ) : null}
                                </div>
                                <p className="text-xs text-muted-foreground">{state.description || 'No description'}</p>
                              </div>
                              {editable && (
                                <div className="flex items-center gap-1">
                                  <Button
                                    size="icon"
                                    variant="ghost"
                                    className="h-7 w-7"
                                    disabled={idx === 0}
                                    onClick={() => handleMoveWithinType(type, state.id, 'up')}
                                  >
                                    <ArrowUp className="h-3.5 w-3.5" />
                                  </Button>
                                  <Button
                                    size="icon"
                                    variant="ghost"
                                    className="h-7 w-7"
                                    disabled={idx === statesByType[type].length - 1}
                                    onClick={() => handleMoveWithinType(type, state.id, 'down')}
                                  >
                                    <ArrowDown className="h-3.5 w-3.5" />
                                  </Button>
                                  <Button size="icon" variant="ghost" className="h-7 w-7" onClick={() => openEdit(state)}>
                                    <Pencil className="h-3.5 w-3.5" />
                                  </Button>
                                </div>
                              )}
                            </div>
                          </div>
                        ))}
                      </div>
                    )}
                  </section>
                ))}
              </div>
            )}
          </>
        )}
      </CardContent>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent>
          <form onSubmit={handleSaveState}>
            <DialogHeader>
              <DialogTitle>{editState ? 'Edit Workflow State' : 'Add Workflow State'}</DialogTitle>
            </DialogHeader>
            <div className="space-y-4 py-4">
              <div className="space-y-2">
                <Label>State Type</Label>
                <Input value={STATE_TYPE_LABEL[newStateType]} disabled />
              </div>
              <div className="space-y-2">
                <Label>State Name</Label>
                <Input value={stateName} onChange={(e) => setStateName(e.target.value)} required />
              </div>
              <div className="space-y-2">
                <Label>Description</Label>
                <Input value={stateDescription} onChange={(e) => setStateDescription(e.target.value)} />
              </div>
              <div className="grid gap-3 sm:grid-cols-2">
                <div className="space-y-2">
                  <Label>Color</Label>
                  <Input value={stateColor} onChange={(e) => setStateColor(e.target.value)} placeholder="#3b82f6" />
                </div>
                <div className="space-y-2">
                  <Label>WIP Limit</Label>
                  <Input type="number" min={0} value={stateWIP} onChange={(e) => setStateWIP(e.target.value)} />
                </div>
              </div>
              <div className="flex items-center justify-between rounded-none border border-border px-3 py-2">
                <div>
                  <Label>Default state</Label>
                  <p className="text-xs text-muted-foreground">Stories are created in this state by default.</p>
                </div>
                <Switch checked={stateDefault} onCheckedChange={setStateDefault} />
              </div>
            </div>
            <DialogFooter className="justify-between">
              <div>
                {editState && editable && (
                  <Button type="button" variant="ghost" className="text-destructive" onClick={() => setDeleteStateConfirm(true)}>
                    <Trash2 className="h-3.5 w-3.5 mr-1" /> Delete State
                  </Button>
                )}
              </div>
              <div className="flex gap-2">
                <Button type="button" variant="outline" onClick={() => setDialogOpen(false)}>Cancel</Button>
                <Button type="submit" disabled={saving || !editable}>{saving ? 'Saving...' : 'Save Changes'}</Button>
              </div>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={deleteStateConfirm}
        onOpenChange={setDeleteStateConfirm}
        title="Delete workflow state"
        description="This will permanently delete this state. Stories in this state will need to be moved to another state. This action cannot be undone."
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={() => { handleDeleteState(); setDeleteStateConfirm(false); }}
      />
    </Card>
  );
}

/* ============ Automations Tab ============ */

function AutomationsTab({ workspaceId, teams, editable = true }: {
  workspaceId: string;
  teams: WorkspaceTeam[];
  editable?: boolean;
}) {
  const [automations, setAutomations] = useState<PMAutomation[]>([]);
  const [epicStates, setEpicStates] = useState<EpicWorkflowState[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      setLoading(true);
      try {
        const [autoRes, statesRes] = await Promise.all([
          pmAutomationService.list(workspaceId),
          pmWorkflowService.listEpicStates(workspaceId),
        ]);
        if (cancelled) return;
        if (autoRes.data) setAutomations(autoRes.data);
        if (statesRes.data) setEpicStates(statesRes.data);
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => { cancelled = true; };
  }, [workspaceId]);

  const getAuto = (type: AutomationType, teamId?: string): PMAutomation | undefined =>
    automations.find((a) => a.automation_type === type && (teamId ? a.team_id === teamId : !a.team_id));

  const upsert = async (type: AutomationType, enabled: boolean, opts?: { teamId?: string; configStateId?: string; configInt?: number; configInt2?: number; configInt3?: number }) => {
    const payload = {
      workspace_id: workspaceId,
      automation_type: type,
      enabled,
      team_id: opts?.teamId,
      config_state_id: opts?.configStateId,
      config_int: opts?.configInt,
      config_int2: opts?.configInt2,
      config_int3: opts?.configInt3,
    };

    // Snapshot for rollback
    const snapshot = automations;

    // Optimistic update
    setAutomations((prev) => {
      const idx = prev.findIndex((a) => a.automation_type === type && (opts?.teamId ? a.team_id === opts.teamId : !a.team_id));
      if (idx >= 0) {
        const updated = [...prev];
        updated[idx] = { ...updated[idx], enabled, config_state_id: opts?.configStateId, config_int: opts?.configInt, config_int2: opts?.configInt2, config_int3: opts?.configInt3 };
        return updated;
      }
      return [...prev, { id: 'temp-' + Date.now(), workspace_id: workspaceId, automation_type: type, enabled, team_id: opts?.teamId, config_state_id: opts?.configStateId, config_int: opts?.configInt, config_int2: opts?.configInt2, config_int3: opts?.configInt3, created_at: '', updated_at: '' }];
    });

    try {
      const res = await pmAutomationService.upsert(workspaceId, payload);
      if (res.error) {
        setAutomations(snapshot);
        toast.error(res.error);
      } else if (res.data) {
        // Replace temp/stale entry with server response
        setAutomations((prev) => {
          const idx = prev.findIndex((a) => a.automation_type === type && (opts?.teamId ? a.team_id === opts.teamId : !a.team_id));
          if (idx >= 0) {
            const updated = [...prev];
            updated[idx] = res.data!;
            return updated;
          }
          return prev;
        });
      }
    } catch {
      setAutomations(snapshot);
      toast.error('Failed to save automation');
    }
  };

  const removeAuto = async (type: AutomationType, teamId?: string) => {
    const snapshot = automations;
    setAutomations((prev) => prev.filter((a) => !(a.automation_type === type && (teamId ? a.team_id === teamId : !a.team_id))));
    const res = await pmAutomationService.remove(workspaceId, type, teamId);
    if (res.error) {
      setAutomations(snapshot);
      toast.error(res.error);
    }
  };

  const updateSprintConfig = (cfg: PMAutomation, patch: { enabled?: boolean; configInt?: number; configInt2?: number; configInt3?: number }) =>
    upsert('sprint_auto_create', patch.enabled ?? cfg.enabled, {
      teamId: cfg.team_id!,
      configInt: patch.configInt ?? cfg.config_int ?? 2,
      configInt2: patch.configInt2 ?? cfg.config_int2 ?? 1,
      configInt3: patch.configInt3 ?? cfg.config_int3 ?? 1,
    });

  const startedStates = epicStates.filter((s) => s.state_type === 'started');
  const doneStates = epicStates.filter((s) => s.state_type === 'done');

  const autoStart = getAuto('epic_auto_start');
  const autoComplete = getAuto('epic_auto_complete');

  const sprintAutoCreateConfigs = automations.filter((a) => a.automation_type === 'sprint_auto_create' && a.team_id);
  const sprintMoveConfigs = automations.filter((a) => a.automation_type === 'sprint_move_unfinished' && a.team_id);

  const sprintAutoCreateTeamIds = new Set(sprintAutoCreateConfigs.map((a) => a.team_id!));
  const sprintMoveTeamIds = new Set(sprintMoveConfigs.map((a) => a.team_id!));

  if (loading) {
    return (
      <div className="space-y-4">
        <Skeleton className="h-48" />
        <Skeleton className="h-48" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* ── Epic Automations ── */}
      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader>
          <CardTitle className="text-base">Epic Automations</CardTitle>
          <CardDescription>Automatically transition epics based on story progress.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center gap-2 rounded-md bg-blue-50 p-3 text-sm text-blue-800">
            <Info className="h-4 w-4 shrink-0" />
            Changes to Epic Automations affect the entire workspace.
          </div>

          {/* Auto Start Epic */}
          <div className="flex items-center justify-between gap-4 rounded-md border p-4">
            <div className="flex-1">
              <p className="text-sm font-medium">Auto Start Epic</p>
              <p className="text-xs text-muted-foreground">
                When any story moves to a started state, auto-transition its parent epic.
              </p>
            </div>
            <div className="flex items-center gap-3">
              {startedStates.length > 1 ? (
                <Select
                  value={autoStart?.config_state_id ?? ''}
                  onValueChange={(val) => upsert('epic_auto_start', autoStart?.enabled ?? true, { configStateId: val })}
                >
                  <SelectTrigger className="w-[180px] h-8 text-xs">
                    <SelectValue placeholder="Target state..." />
                  </SelectTrigger>
                  <SelectContent>
                    {startedStates.map((st) => (
                      <SelectItem key={st.id} value={st.id}>{st.name}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              ) : startedStates.length === 1 ? (
                <Badge variant="secondary" className="text-xs">{startedStates[0].name}</Badge>
              ) : null}
              <Switch
                checked={autoStart?.enabled ?? false}
                disabled={!editable}
                onCheckedChange={(checked) => {
                  const stateId = autoStart?.config_state_id ?? startedStates[0]?.id;
                  if (!stateId) { toast.error('No started epic state available. Please check your epic workflow states.'); return; }
                  upsert('epic_auto_start', checked, { configStateId: stateId });
                }}
              />
            </div>
          </div>

          {/* Auto Complete Epic */}
          <div className="flex items-center justify-between gap-4 rounded-md border p-4">
            <div className="flex-1">
              <p className="text-sm font-medium">Auto Complete Epic</p>
              <p className="text-xs text-muted-foreground">
                When all stories in an epic reach a done state, auto-transition the epic.
              </p>
            </div>
            <div className="flex items-center gap-3">
              {doneStates.length > 1 ? (
                <Select
                  value={autoComplete?.config_state_id ?? ''}
                  onValueChange={(val) => upsert('epic_auto_complete', autoComplete?.enabled ?? true, { configStateId: val })}
                >
                  <SelectTrigger className="w-[180px] h-8 text-xs">
                    <SelectValue placeholder="Target state..." />
                  </SelectTrigger>
                  <SelectContent>
                    {doneStates.map((st) => (
                      <SelectItem key={st.id} value={st.id}>{st.name}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              ) : doneStates.length === 1 ? (
                <Badge variant="secondary" className="text-xs">{doneStates[0].name}</Badge>
              ) : null}
              <Switch
                checked={autoComplete?.enabled ?? false}
                disabled={!editable}
                onCheckedChange={(checked) => {
                  const stateId = autoComplete?.config_state_id ?? doneStates[0]?.id;
                  if (!stateId) { toast.error('No done epic state available. Please check your epic workflow states.'); return; }
                  upsert('epic_auto_complete', checked, { configStateId: stateId });
                }}
              />
            </div>
          </div>
        </CardContent>
      </Card>

      {/* ── Sprint Automations ── */}
      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader>
          <CardTitle className="text-base">Sprint Automations</CardTitle>
          <CardDescription>Automate sprint creation and story rollover per team.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          <div className="flex items-center gap-2 rounded-md bg-blue-50 p-3 text-sm text-blue-800">
            <Info className="h-4 w-4 shrink-0" />
            Changes to Sprint Automations are specific to each Team.
          </div>

          {/* Auto-Create Future Sprints */}
          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium">Auto-Create Future Sprints</p>
                <p className="text-xs text-muted-foreground">
                  Automatically create future sprints when a sprint completes.
                </p>
              </div>
              {editable && (
                <Select
                  value=""
                  onValueChange={(teamId) => upsert('sprint_auto_create', true, { teamId, configInt: 2, configInt2: 1, configInt3: 1 })}
                >
                  <SelectTrigger className="w-[140px] h-8 text-xs">
                    <SelectValue placeholder="Add Team..." />
                  </SelectTrigger>
                  <SelectContent>
                    {teams.filter((t) => !sprintAutoCreateTeamIds.has(t.id)).map((t) => (
                      <SelectItem key={t.id} value={t.id}>{t.name}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              )}
            </div>
            {sprintAutoCreateConfigs.map((cfg) => {
              const team = teams.find((t) => t.id === cfg.team_id);
              return (
                <div key={cfg.id} className="flex items-center gap-3 rounded-md border p-3">
                  <span className="text-sm font-medium min-w-[100px]">{team?.name ?? 'Unknown'}</span>
                  <div className="flex items-center gap-2 text-xs flex-wrap">
                    <Label className="text-xs text-muted-foreground">Sprints:</Label>
                    <Input
                      type="number"
                      min={1}
                      max={10}
                      value={cfg.config_int ?? 2}
                      onChange={(e) => updateSprintConfig(cfg, { configInt: Number(e.target.value) })}
                      className="w-16 h-7 text-xs"
                      disabled={!editable}
                    />
                    <Label className="text-xs text-muted-foreground">Weeks:</Label>
                    <Input
                      type="number"
                      min={1}
                      max={8}
                      value={cfg.config_int2 ?? 1}
                      onChange={(e) => updateSprintConfig(cfg, { configInt2: Number(e.target.value) })}
                      className="w-16 h-7 text-xs"
                      disabled={!editable}
                    />
                    <Label className="text-xs text-muted-foreground">Start day:</Label>
                    <Select
                      value={String(cfg.config_int3 ?? 1)}
                      onValueChange={(val) => updateSprintConfig(cfg, { configInt3: Number(val) })}
                      disabled={!editable}
                    >
                      <SelectTrigger className="w-[100px] h-7 text-xs">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'].map((day, i) => (
                          <SelectItem key={i} value={String(i)}>{day}</SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                  <Switch
                    checked={cfg.enabled}
                    disabled={!editable}
                    onCheckedChange={(checked) => updateSprintConfig(cfg, { enabled: checked })}
                  />
                  {editable && (
                    <button type="button" onClick={() => removeAuto('sprint_auto_create', cfg.team_id!)} className="text-muted-foreground hover:text-destructive">
                      <X className="h-4 w-4" />
                    </button>
                  )}
                </div>
              );
            })}
          </div>

          {/* Move Unfinished Stories */}
          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium">Move Unfinished Stories to Next Sprint</p>
                <p className="text-xs text-muted-foreground">
                  When a sprint ends, move incomplete stories to the next sprint.
                </p>
              </div>
              {editable && (
                <Select
                  value=""
                  onValueChange={(teamId) => upsert('sprint_move_unfinished', true, { teamId })}
                >
                  <SelectTrigger className="w-[140px] h-8 text-xs">
                    <SelectValue placeholder="Add Team..." />
                  </SelectTrigger>
                  <SelectContent>
                    {teams.filter((t) => !sprintMoveTeamIds.has(t.id)).map((t) => (
                      <SelectItem key={t.id} value={t.id}>{t.name}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              )}
            </div>
            {sprintMoveConfigs.map((cfg) => {
              const team = teams.find((t) => t.id === cfg.team_id);
              return (
                <div key={cfg.id} className="flex items-center gap-3 rounded-md border p-3">
                  <span className="text-sm font-medium flex-1">{team?.name ?? 'Unknown'}</span>
                  <Switch
                    checked={cfg.enabled}
                    disabled={!editable}
                    onCheckedChange={(checked) => upsert('sprint_move_unfinished', checked, { teamId: cfg.team_id! })}
                  />
                  {editable && (
                    <button type="button" onClick={() => removeAuto('sprint_move_unfinished', cfg.team_id!)} className="text-muted-foreground hover:text-destructive">
                      <X className="h-4 w-4" />
                    </button>
                  )}
                </div>
              );
            })}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

/* ============ Import Tab ============ */

const IMPORT_SOURCES = [
  {
    key: 'shortcut' as const,
    title: 'Shortcut',
    description: 'Import stories, epics, workflows, and members from Shortcut.',
    icon: Import,
    comingSoon: false,
  },
  {
    key: 'jira' as const,
    title: 'Jira',
    description: 'Import issues, projects, and workflows from Jira.',
    icon: Import,
    comingSoon: true,
  },
  {
    key: 'linear' as const,
    title: 'Linear',
    description: 'Import issues, projects, and cycles from Linear.',
    icon: Import,
    comingSoon: true,
  },
];

function ImportTab({ workspaceId, editable = true }: { workspaceId: string; editable?: boolean }) {
  const [selected, setSelected] = useState<string | null>(null);
  const [members, setMembers] = useState<MemberWithUser[]>([]);

  useEffect(() => {
    workspacesService.listMembers(workspaceId).then(({ data }) => {
      if (data) setMembers(data);
    });
  }, [workspaceId]);

  if (selected === 'shortcut') {
    return (
      <div className="space-y-4">
        <Button variant="ghost" size="sm" className="gap-1.5 text-muted-foreground" onClick={() => setSelected(null)}>
          <ChevronRight className="h-4 w-4 rotate-180" />
          Back to sources
        </Button>
        <ShortcutImportWizard workspaceId={workspaceId} members={members} />
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div>
        <h3 className="text-sm font-medium text-muted-foreground">Select a source to import from</h3>
      </div>
      <div className="overflow-hidden rounded-lg border border-border bg-background">
        {IMPORT_SOURCES.map((source, idx) => (
          <button
            key={source.key}
            type="button"
            disabled={source.comingSoon || !editable}
            onClick={() => setSelected(source.key)}
            className={cn(
              'flex w-full items-center gap-4 px-4 py-4 text-left transition-colors',
              source.comingSoon ? 'cursor-not-allowed opacity-60' : 'hover:bg-muted/40',
              idx > 0 && 'border-t border-border',
            )}
          >
            <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
              <source.icon className="h-4 w-4" />
            </div>
            <div className="min-w-0 flex-1">
              <p className="text-sm font-medium">{source.title}</p>
              <p className="text-sm text-muted-foreground">{source.description}</p>
            </div>
            {source.comingSoon ? (
              <Badge variant="secondary" className="text-xs">Coming Soon</Badge>
            ) : (
              <ChevronRight className="h-4 w-4 shrink-0 text-muted-foreground" />
            )}
          </button>
        ))}
      </div>
    </div>
  );
}

// ── Help Center Settings ────────────────────────────────────────────────────

function HelpcenterTab({ workspaceId }: { workspaceId: string }) {
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [config, setConfig] = useState<{
    subdomain: string;
    custom_domain: string;
    brand_name: string;
    brand_logo_url: string;
    brand_color: string;
    is_published: boolean;
    seo_title: string;
    seo_description: string;
    support_email: string;
  }>({
    subdomain: '',
    custom_domain: '',
    brand_name: '',
    brand_logo_url: '',
    brand_color: '#3b82f6',
    is_published: false,
    seo_title: '',
    seo_description: '',
    support_email: '',
  });

  useEffect(() => {
    const load = async () => {
      setLoading(true);
      const { docsService } = await import('@/lib/services/docsService');
      const res = await docsService.getHelpcenterConfig(workspaceId);
      if (res.data) {
        setConfig({
          subdomain: res.data.subdomain ?? '',
          custom_domain: res.data.custom_domain ?? '',
          brand_name: res.data.brand_name ?? '',
          brand_logo_url: res.data.brand_logo_url ?? '',
          brand_color: res.data.brand_color ?? '#3b82f6',
          is_published: res.data.is_published ?? false,
          seo_title: res.data.seo_title ?? '',
          seo_description: res.data.seo_description ?? '',
          support_email: res.data.support_email ?? '',
        });
      }
      setLoading(false);
    };
    load();
  }, [workspaceId]);

  const handleSave = async (e: FormEvent) => {
    e.preventDefault();
    setSaving(true);
    const { docsService } = await import('@/lib/services/docsService');
    const res = await docsService.updateHelpcenterConfig(workspaceId, {
      subdomain: config.subdomain || undefined,
      custom_domain: config.custom_domain || undefined,
      brand_name: config.brand_name || undefined,
      brand_logo_url: config.brand_logo_url || undefined,
      brand_color: config.brand_color || undefined,
      is_published: config.is_published,
      seo_title: config.seo_title || undefined,
      seo_description: config.seo_description || undefined,
      support_email: config.support_email || undefined,
    });
    setSaving(false);
    if (res.error) {
      toast.error(res.error);
    } else {
      toast.success('Help center settings saved');
    }
  };

  if (loading) {
    return (
      <div className="space-y-4">
        <Skeleton className="h-10 w-full" />
        <Skeleton className="h-10 w-full" />
        <Skeleton className="h-10 w-full" />
      </div>
    );
  }

  return (
    <form onSubmit={handleSave} className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle>Branding</CardTitle>
          <CardDescription>Customize how your public help center looks.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-2">
            <Label htmlFor="hc-brand-name">Brand Name</Label>
            <Input
              id="hc-brand-name"
              value={config.brand_name}
              onChange={(e) => setConfig({ ...config, brand_name: e.target.value })}
              placeholder="Your Company"
            />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="hc-brand-logo">Logo URL</Label>
            <Input
              id="hc-brand-logo"
              value={config.brand_logo_url}
              onChange={(e) => setConfig({ ...config, brand_logo_url: e.target.value })}
              placeholder="https://..."
            />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="hc-brand-color">Brand Color</Label>
            <div className="flex items-center gap-2">
              <input
                type="color"
                id="hc-brand-color"
                value={config.brand_color}
                onChange={(e) => setConfig({ ...config, brand_color: e.target.value })}
                className="h-8 w-12 cursor-pointer rounded border"
              />
              <Input
                value={config.brand_color}
                onChange={(e) => setConfig({ ...config, brand_color: e.target.value })}
                className="flex-1"
                placeholder="#3b82f6"
              />
            </div>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Domain</CardTitle>
          <CardDescription>Set up your help center URL.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-2">
            <Label htmlFor="hc-subdomain">Subdomain</Label>
            <div className="flex items-center gap-1">
              <Input
                id="hc-subdomain"
                value={config.subdomain}
                onChange={(e) => setConfig({ ...config, subdomain: e.target.value })}
                placeholder="yourcompany"
              />
              <span className="shrink-0 text-sm text-muted-foreground">.helpin.ai</span>
            </div>
          </div>
          <div className="grid gap-2">
            <Label htmlFor="hc-custom-domain">Custom Domain (optional)</Label>
            <Input
              id="hc-custom-domain"
              value={config.custom_domain}
              onChange={(e) => setConfig({ ...config, custom_domain: e.target.value })}
              placeholder="help.yourcompany.com"
            />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="hc-support-email">Support Email</Label>
            <Input
              id="hc-support-email"
              type="email"
              value={config.support_email}
              onChange={(e) => setConfig({ ...config, support_email: e.target.value })}
              placeholder="support@yourcompany.com"
            />
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>SEO</CardTitle>
          <CardDescription>Optimize your help center for search engines.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-2">
            <Label htmlFor="hc-seo-title">SEO Title</Label>
            <Input
              id="hc-seo-title"
              value={config.seo_title}
              onChange={(e) => setConfig({ ...config, seo_title: e.target.value })}
              placeholder="Help Center - Your Company"
            />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="hc-seo-desc">SEO Description</Label>
            <Textarea
              id="hc-seo-desc"
              value={config.seo_description}
              onChange={(e) => setConfig({ ...config, seo_description: e.target.value })}
              placeholder="Find answers, guides, and documentation..."
              rows={3}
            />
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Publishing</CardTitle>
          <CardDescription>Control whether your help center is live.</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium">Help Center Published</p>
              <p className="text-xs text-muted-foreground">
                {config.is_published
                  ? 'Your help center is publicly accessible.'
                  : 'Your help center is not visible to the public.'}
              </p>
            </div>
            <Switch
              checked={config.is_published}
              onCheckedChange={(v) => setConfig({ ...config, is_published: v })}
            />
          </div>
        </CardContent>
      </Card>

      <div className="flex justify-end">
        <Button type="submit" disabled={saving}>
          {saving ? 'Saving...' : 'Save Settings'}
        </Button>
      </div>
    </form>
  );
}

/* ============ CRM Email Settings Tab ============ */

function CRMEmailSettingsTab({ workspaceId }: { workspaceId: string }) {
  const user = useAuthStore((s) => s.user);
  return <EmailAccountConnect workspaceId={workspaceId} memberId={user?.id ?? ''} />;
}

/* ============ CRM Autonomy Settings Tab ============ */

function CRMAutonomySettingsTab({ workspaceId }: { workspaceId: string }) {
  const { data: settings, isLoading } = useAutonomySettings(workspaceId);
  const updateSettings = useUpdateAutonomySettings(workspaceId);
  const [saving, setSaving] = useState(false);

  const [enabled, setEnabled] = useState(true);
  const [autoCreateDeals, setAutoCreateDeals] = useState(true);
  const [autoProgressDeals, setAutoProgressDeals] = useState(true);
  const [autoExecuteThreshold, setAutoExecuteThreshold] = useState(0.9);
  const [reviewThreshold, setReviewThreshold] = useState(0.7);

  useEffect(() => {
    if (settings) {
      setEnabled(settings.enabled);
      setAutoCreateDeals(settings.auto_create_deals);
      setAutoProgressDeals(settings.auto_progress_deals);
      setAutoExecuteThreshold(settings.auto_execute_threshold);
      setReviewThreshold(settings.review_threshold);
    }
  }, [settings]);

  const handleSave = async () => {
    setSaving(true);
    try {
      await updateSettings.mutateAsync({
        enabled,
        auto_create_deals: autoCreateDeals,
        auto_progress_deals: autoProgressDeals,
        auto_execute_threshold: autoExecuteThreshold,
        review_threshold: reviewThreshold,
      });
      toast.success('Autonomy settings saved');
    } catch {
      toast.error('Failed to save settings');
    } finally {
      setSaving(false);
    }
  };

  if (isLoading) return <Skeleton className="h-64 w-full" />;

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Self-Driving CRM</CardTitle>
          <CardDescription>
            Configure how aggressively the system auto-creates and advances deals based on detected buyer signals.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          <div className="flex items-center justify-between">
            <div>
              <Label className="text-sm font-medium">Enable Automation</Label>
              <p className="text-xs text-muted-foreground">Master switch for all CRM automation</p>
            </div>
            <Switch checked={enabled} onCheckedChange={setEnabled} />
          </div>

          <Separator />

          <div className="flex items-center justify-between">
            <div>
              <Label className="text-sm font-medium">Auto-Create Deals</Label>
              <p className="text-xs text-muted-foreground">Automatically create deals when buying intent is detected</p>
            </div>
            <Switch checked={autoCreateDeals} onCheckedChange={setAutoCreateDeals} disabled={!enabled} />
          </div>

          <div className="flex items-center justify-between">
            <div>
              <Label className="text-sm font-medium">Auto-Progress Deals</Label>
              <p className="text-xs text-muted-foreground">Automatically advance deal stages based on signals</p>
            </div>
            <Switch checked={autoProgressDeals} onCheckedChange={setAutoProgressDeals} disabled={!enabled} />
          </div>

          <Separator />

          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <Label className="text-sm font-medium">Auto-Execute Threshold</Label>
              <p className="text-xs text-muted-foreground">Signals above this confidence are executed without review</p>
              <Select
                value={String(autoExecuteThreshold)}
                onValueChange={(v) => {
                  const n = Number(v);
                  setAutoExecuteThreshold(n);
                  if (n < reviewThreshold) setReviewThreshold(n);
                }}
                disabled={!enabled}
              >
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  {[50, 55, 60, 65, 70, 75, 80, 85, 90, 95, 100].map((p) => (
                    <SelectItem key={p} value={String(p / 100)}>{p}%</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2">
              <Label className="text-sm font-medium">Review Threshold</Label>
              <p className="text-xs text-muted-foreground">Signals between this and auto-execute create pending suggestions</p>
              <Select
                value={String(reviewThreshold)}
                onValueChange={(v) => {
                  const n = Number(v);
                  setReviewThreshold(n);
                  if (n > autoExecuteThreshold) setAutoExecuteThreshold(n);
                }}
                disabled={!enabled}
              >
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  {[30, 35, 40, 45, 50, 55, 60, 65, 70, 75, 80, 85, 90, 95, 100].map((p) => (
                    <SelectItem key={p} value={String(p / 100)}>{p}%</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>

          <div className="rounded-md border p-3 text-xs text-muted-foreground">
            <p className="font-medium text-foreground">How it works:</p>
            <ul className="mt-1 list-inside list-disc space-y-0.5">
              <li>Confidence &ge; {(autoExecuteThreshold * 100).toFixed(0)}%: auto-executed (no review needed)</li>
              <li>Confidence {(reviewThreshold * 100).toFixed(0)}%&ndash;{(autoExecuteThreshold * 100).toFixed(0)}%: pending review in the Review feed</li>
              <li>Confidence &lt; {(reviewThreshold * 100).toFixed(0)}%: low-priority suggestion</li>
            </ul>
          </div>
        </CardContent>
      </Card>

      <div className="flex justify-end">
        <Button onClick={handleSave} disabled={saving}>
          {saving ? 'Saving...' : 'Save Settings'}
        </Button>
      </div>
    </div>
  );
}

