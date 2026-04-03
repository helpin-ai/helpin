import { useEffect, useMemo, useState, type FormEvent } from 'react';
import { useNavigate, useSearch } from '@tanstack/react-router';
import { useTitle } from '@/hooks/useTitle';
import { useOrganizationStore } from '@/stores/organizationStore';
import { useAuthStore } from '@/stores/authStore';
import { useOrganizations, useCreateOrganization, useWorkspaces } from '@/hooks/queries';
import { useQueryClient } from '@tanstack/react-query';
import { workspacesService } from '@/lib/services/workspacesService';
import { settingsService } from '@/lib/services/settingsService';
import { inviteService } from '@/lib/services/inviteService';
import { generateWorkspaceSlug } from '@/lib/slugUtils';
import { buildWorkspaceWebsiteContentSourcePayload } from '@/lib/workspaceWebsiteSource';
import { WorkspaceSelector } from '@/components/workspace/WorkspaceSelector';
import type { OrganizationWithRole } from '@/lib/types';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { EmailChipInput, classifyEmailChipInput, mergeEmailChips } from '@/components/ui/email-chip-input';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Skeleton } from '@/components/ui/skeleton';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { cn } from '@/lib/utils';
import { useCreateSupportContentSource } from '@/hooks/queries/useSupport';
import { Favicon } from '@/components/ui/favicon';
import { CheckCircle2, Loader2, Plus, X } from 'lucide-react';
import { UserAvatar } from '@/components/pm/UserAvatar';
import {
  buildPresetFieldVisibility,
  slugifyTeamHandle,
  TEAM_TYPE_PRESETS,
  WORKSPACE_TEAM_SUGGESTIONS,
  type TeamType,
} from '@/lib/teamPresets';
import { toast } from 'sonner';

function OrgFormFields({
  name, slug, onNameChange, onSlugChange, nameId, slugId,
}: {
  name: string; slug: string;
  onNameChange: (val: string) => void; onSlugChange: (val: string) => void;
  nameId: string; slugId: string;
}) {
  return (
    <>
      <div className="space-y-2">
        <Label htmlFor={nameId}>Organization Name</Label>
        <Input id={nameId} placeholder="Acme Inc." value={name} onChange={e => onNameChange(e.target.value)} required />
      </div>
      <div className="space-y-2">
        <Label htmlFor={slugId}>Slug</Label>
        <Input id={slugId} placeholder="acme-inc" value={slug} onChange={e => onSlugChange(e.target.value)} required />
        <p className="text-xs text-muted-foreground">A URL-friendly identifier for your organization</p>
      </div>
    </>
  );
}

type TeamDraft = {
  id: string;
  name: string;
  handle: string;
  teamType: TeamType;
  selected: boolean;
  isCustom: boolean;
};

const createInitialTeamDrafts = (): TeamDraft[] =>
  WORKSPACE_TEAM_SUGGESTIONS.map((team, index) => ({
    id: `preset-${index}`,
    name: team.name,
    handle: slugifyTeamHandle(team.name),
    teamType: team.teamType,
    selected: team.selected,
    isCustom: false,
  }));

export default function Workspaces() {
  useTitle('Workspaces');
  const { data: organizations = [], isLoading: orgsLoading } = useOrganizations();
  const navigate = useNavigate();
  const { user } = useAuthStore();
  const { currentOrganization, setCurrentOrganization } = useOrganizationStore();
  const { data: allWorkspaces = [], isLoading: wsLoading } = useWorkspaces();
  const createOrgMutation = useCreateOrganization();
  const queryClient = useQueryClient();
  const { create } = useSearch({ from: '/_authenticated/workspaces' });
  const [dialogOpen, setDialogOpen] = useState(false);
  const [orgDialogOpen, setOrgDialogOpen] = useState(false);
  const [name, setName] = useState('');
  const [slug, setSlug] = useState('');
  const [workspaceKey, setWorkspaceKey] = useState('');
  const [websiteUrl, setWebsiteUrl] = useState('');
  const [creating, setCreating] = useState(false);
  const [workspaceStep, setWorkspaceStep] = useState<'details' | 'teams' | 'invite'>('details');
  const [createdWorkspace, setCreatedWorkspace] = useState<{ id: string; slug: string; name: string; website_url?: string } | null>(null);
  const [createdTeamIds, setCreatedTeamIds] = useState<string[]>([]);
  const [inviteEmails, setInviteEmails] = useState<string[]>([]);
  const [inviteEmailInput, setInviteEmailInput] = useState('');
  const [inviteRole, setInviteRole] = useState('member');
  const [sendingInvites, setSendingInvites] = useState(false);
  const [websiteSourceAdded, setWebsiteSourceAdded] = useState(false);
  const [teamDrafts, setTeamDrafts] = useState<TeamDraft[]>(createInitialTeamDrafts);
  const createWebsiteSourceMutation = useCreateSupportContentSource(createdWorkspace?.id ?? '');

  // Org creation state — pre-fill from user's first name for first-time users
  const firstName = user?.full_name?.split(' ')[0] ?? '';
  const defaultOrgName = firstName ? `${firstName}'s Organization` : '';
  const [orgName, setOrgName] = useState(defaultOrgName);
  const [orgSlug, setOrgSlug] = useState(defaultOrgName ? generateWorkspaceSlug(defaultOrgName) : '');

  // Pre-fill org name from user's name when it becomes available
  useEffect(() => {
    if (user?.full_name && !orgName && organizations.length === 0) {
      const first = user.full_name.split(' ')[0];
      if (first) {
        const name = `${first}'s Organization`;
        setOrgName(name);
        setOrgSlug(generateWorkspaceSlug(name));
      }
    }
  }, [user?.full_name, orgName, organizations.length]);

  // Auto-select first org when organizations load and none is selected
  useEffect(() => {
    if (organizations.length > 0 && !currentOrganization) {
      const savedId = localStorage.getItem('current_organization_id');
      const org = (savedId ? organizations.find((o) => o.id === savedId) : null) ?? organizations[0];
      setCurrentOrganization(org);
    }
  }, [organizations, currentOrganization, setCurrentOrganization]);

  // Auto-open create dialog when navigated with ?create=true
  useEffect(() => {
    if (create && currentOrganization && !orgsLoading) {
      openWorkspaceDialog();
    }
  }, [create, currentOrganization, orgsLoading]);

  const handleNameChange = (val: string) => {
    setName(val);
    setSlug(generateWorkspaceSlug(val));
    // Auto-suggest workspace key from name (first 3 alpha chars, uppercase)
    const alpha = val.replace(/[^a-zA-Z]/g, '').toUpperCase();
    setWorkspaceKey(alpha.slice(0, 3));
  };

  const handleOrgNameChange = (val: string) => {
    setOrgName(val);
    setOrgSlug(generateWorkspaceSlug(val));
  };

  const resetWorkspaceDialog = () => {
    setWorkspaceStep('details');
    setName('');
    setSlug('');
    setWebsiteUrl('');
    setTeamDrafts(createInitialTeamDrafts());
    setCreatedWorkspace(null);
    setCreatedTeamIds([]);
    setInviteEmails([]);
    setInviteEmailInput('');
    setInviteRole('member');
    setWebsiteSourceAdded(false);
  };

  const openWorkspaceDialog = () => {
    resetWorkspaceDialog();
    setDialogOpen(true);
  };

  const handleWorkspaceDialogChange = (open: boolean) => {
    if (!open && creating) return;
    setDialogOpen(open);
    if (!open) resetWorkspaceDialog();
  };

  const handleCreateOrg = async (e: FormEvent) => {
    e.preventDefault();
    try {
      const data = await createOrgMutation.mutateAsync({ name: orgName, slug: orgSlug });
      toast.success('Organization created');
      setOrgDialogOpen(false);
      setOrgName('');
      setOrgSlug('');
      if (data) setCurrentOrganization(data);
    } catch {
      toast.error('Failed to create organization');
    }
  };

  const [checkingSlug, setCheckingSlug] = useState(false);

  const handleContinueToTeams = async (e: FormEvent) => {
    e.preventDefault();
    if (!currentOrganization) {
      toast.error('Please select an organization first');
      return;
    }
    if (!name.trim() || !slug.trim()) {
      toast.error('Enter a workspace name and slug');
      return;
    }
    // Check slug availability
    setCheckingSlug(true);
    const { data: existing } = await workspacesService.getBySlug(slug.trim());
    setCheckingSlug(false);
    if (existing) {
      toast.error(`The slug "${slug.trim()}" is already taken. Please choose a different one.`);
      return;
    }
    setWorkspaceStep('teams');
  };

  const updateTeamDraft = (id: string, updates: Partial<TeamDraft>) => {
    setTeamDrafts((current) => current.map((team) => (team.id === id ? { ...team, ...updates } : team)));
  };

  const addCustomTeam = () => {
    const nextId = `custom-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
    setTeamDrafts((current) => [
      ...current,
      {
        id: nextId,
        handle: '',
        name: '',
        teamType: 'custom',
        selected: true,
        isCustom: true,
      },
    ]);
  };

  const removeCustomTeam = (id: string) => {
    setTeamDrafts((current) => current.filter((team) => team.id !== id));
  };

  const completeWorkspaceSetup = async (skipTeams = false) => {
    if (!currentOrganization) {
      toast.error('Please select an organization first');
      return;
    }
    setCreating(true);
    const { data: workspace, error } = await workspacesService.create({
      name,
      slug,
      workspace_key: workspaceKey.toUpperCase(),
      organization_id: currentOrganization.id,
      website_url: websiteUrl.trim() || undefined,
      timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
    });
    if (error) {
      setCreating(false);
      toast.error(error);
      return;
    }

    const selectedTeams = skipTeams
      ? []
      : teamDrafts
        .filter((team) => team.selected && team.name.trim())
        .map((team) => ({
          name: team.name.trim(),
          handle: team.handle.trim(),
          teamType: team.teamType,
        }));

    let createdTeamCount = 0;
    let failedTeamCount = 0;

    if (workspace && selectedTeams.length > 0) {
      const results = await Promise.allSettled(
        selectedTeams.map(async (team) => {
          const teamRes = await settingsService.createTeam({
            workspace_id: workspace.id,
            name: team.name,
            handle: team.handle ? slugifyTeamHandle(team.handle) : slugifyTeamHandle(team.name),
            team_type: team.teamType,
            default_task_type: TEAM_TYPE_PRESETS[team.teamType].defaultTaskType,
          });

          if (!teamRes.data || teamRes.error) {
            throw new Error(teamRes.error ?? `Failed to create ${team.name}`);
          }

          const [estimateRes, visibilityRes] = await Promise.all([
            settingsService.updateTeamEstimateSettings(workspace.id, teamRes.data.id, TEAM_TYPE_PRESETS[team.teamType].estimate),
            settingsService.updateTeamFieldVisibility(workspace.id, teamRes.data.id, buildPresetFieldVisibility(team.teamType)),
          ]);

          if (estimateRes.error || visibilityRes.error) {
            throw new Error(estimateRes.error ?? visibilityRes.error ?? `Failed to finish setup for ${team.name}`);
          }
          return teamRes.data.id;
        }),
      );

      createdTeamCount = results.filter((result) => result.status === 'fulfilled').length;
      failedTeamCount = results.length - createdTeamCount;
      setCreatedTeamIds(results.filter((r): r is PromiseFulfilledResult<string> => r.status === 'fulfilled').map((r) => r.value));
    }

    setCreating(false);
    queryClient.invalidateQueries({ queryKey: ['workspaces'] });

    if (failedTeamCount > 0) {
      toast.warning(`Workspace created. Added ${createdTeamCount} team${createdTeamCount === 1 ? '' : 's'}, but ${failedTeamCount} failed.`);
    } else if (createdTeamCount > 0) {
      toast.success(`Workspace created with ${createdTeamCount} team${createdTeamCount === 1 ? '' : 's'}.`);
    } else {
      toast.success('Workspace created');
    }

    if (workspace) {
      setCreatedWorkspace({
        id: workspace.id,
        slug: workspace.slug,
        name: workspace.name,
        website_url: workspace.website_url,
      });
      setWorkspaceStep('invite');
    }
  };

  const handleAddWebsiteSource = async () => {
    if (!createdWorkspace || !createdWorkspace.website_url || websiteSourceAdded || createWebsiteSourceMutation.isPending) {
      return;
    }

    await createWebsiteSourceMutation.mutateAsync(
      buildWorkspaceWebsiteContentSourcePayload(createdWorkspace.name, createdWorkspace.website_url),
    );
    setWebsiteSourceAdded(true);
    toast.success('Website source added and syncing');
  };
  const handleSendInvites = async () => {
    if (!createdWorkspace) return;
    const emails = mergeEmailChips(inviteEmails, inviteEmailInput);
    if (emails.length === 0) {
      finishWorkspaceSetup();
      return;
    }
    setInviteEmails(emails);
    setInviteEmailInput('');
    setSendingInvites(true);
    let sent = 0;
    const failedEmails: string[] = [];
    await Promise.all(
      emails.map(async (email) => {
        const { data, error } = await inviteService.send({ workspace_id: createdWorkspace.id, email, role: inviteRole });
        if (error) {
          failedEmails.push(email);
        } else {
          sent++;
          // Preassign non-admin invitees to all created teams
          if (data?.id && inviteRole !== 'admin' && createdTeamIds.length > 0) {
            await Promise.all(
              createdTeamIds.map((teamId) => settingsService.addTeamInvitation(createdWorkspace.id, teamId, data.id)),
            );
          }
        }
      }),
    );
    setSendingInvites(false);
    if (sent > 0 && failedEmails.length > 0) {
      toast.warning(`${sent} of ${emails.length} invitations sent. Failed: ${failedEmails.join(', ')}`);
    } else if (sent > 0) {
      toast.success(`${sent} invitation${sent === 1 ? '' : 's'} sent`);
    } else {
      toast.error(`Failed to send invitations: ${failedEmails.join(', ')}`);
    }
    finishWorkspaceSetup();
  };

  const finishWorkspaceSetup = () => {
    const ws = createdWorkspace;
    setDialogOpen(false);
    resetWorkspaceDialog();
    if (ws) {
      void navigate({ to: `/w/${ws.slug}/pm/my-work` });
    }
  };

  const handleOrgSwitch = (org: OrganizationWithRole) => {
    setCurrentOrganization(org);
  };

  // Group workspaces by organization for display
  const workspacesByOrg = useMemo(() => {
    const groups: { org: OrganizationWithRole; workspaces: typeof allWorkspaces }[] = [];
    for (const org of organizations) {
      const orgWorkspaces = allWorkspaces.filter((ws) => ws.organization_id === org.id);
      if (orgWorkspaces.length > 0) {
        groups.push({ org, workspaces: orgWorkspaces });
      }
    }
    // Include workspaces with no matching org (edge case)
    const ungrouped = allWorkspaces.filter((ws) => !organizations.some((o) => o.id === ws.organization_id));
    if (ungrouped.length > 0) {
      groups.push({ org: { id: '', name: 'Other', slug: '', owner_id: '', created_at: '', updated_at: '', role: 'member' as const }, workspaces: ungrouped });
    }
    return groups;
  }, [allWorkspaces, organizations]);

  const isLoading = wsLoading || orgsLoading;
  const selectedTeamCount = useMemo(
    () => teamDrafts.filter((team) => team.selected && team.name.trim()).length,
    [teamDrafts],
  );
  const hasSelectedTeamWithoutName = teamDrafts.some((team) => team.selected && !team.name.trim());
  const hasInviteRecipients = useMemo(
    () => mergeEmailChips(inviteEmails, inviteEmailInput).length > 0,
    [inviteEmailInput, inviteEmails],
  );
  const hasInvalidInviteInput = useMemo(
    () => classifyEmailChipInput(inviteEmailInput).invalid.length > 0,
    [inviteEmailInput],
  );

  // Auto-create org if user has none (edge case — signup normally handles this).
  useEffect(() => {
    if (!orgsLoading && organizations.length === 0 && !createOrgMutation.isPending && user) {
      const first = user.full_name?.split(' ')[0] || 'My';
      const name = `${first}'s Organization`;
      createOrgMutation.mutateAsync({ name, slug: generateWorkspaceSlug(name) }).then((org) => {
        setCurrentOrganization(org);
      }).catch(() => {});
    }
  }, [orgsLoading, organizations.length, createOrgMutation.isPending, user]);

  return (
    <div className="min-h-screen bg-background">
      <div className="max-w-6xl mx-auto px-4 py-12">
        <Dialog open={orgDialogOpen} onOpenChange={setOrgDialogOpen}>
          <DialogContent>
            <form onSubmit={handleCreateOrg}>
              <DialogHeader>
                <DialogTitle>Create Organization</DialogTitle>
                <DialogDescription>Create a new organization to group workspaces.</DialogDescription>
              </DialogHeader>
              <div className="space-y-4 py-4">
                <OrgFormFields
                  name={orgName} slug={orgSlug}
                  onNameChange={handleOrgNameChange} onSlugChange={setOrgSlug}
                  nameId="new-org-name" slugId="new-org-slug"
                />
              </div>
              <DialogFooter>
                <Button type="button" variant="outline" onClick={() => setOrgDialogOpen(false)}>Cancel</Button>
                <Button type="submit" disabled={createOrgMutation.isPending}>{createOrgMutation.isPending ? 'Creating...' : 'Create'}</Button>
              </DialogFooter>
            </form>
          </DialogContent>
        </Dialog>

        <div className="flex items-center justify-between mb-8">
          <div>
            <h1 className="text-3xl font-bold">Workspaces</h1>
            <p className="text-muted-foreground mt-1">Select a workspace or create a new one</p>
          </div>
          <Dialog open={dialogOpen} onOpenChange={handleWorkspaceDialogChange}>
            <DialogTrigger asChild>
              <Button disabled={!currentOrganization} onClick={openWorkspaceDialog}>
                <Plus className="h-4 w-4 mr-2" />
                Create Workspace
              </Button>
            </DialogTrigger>
            <DialogContent className="sm:max-w-2xl">
              <form onSubmit={
                workspaceStep === 'details' ? handleContinueToTeams
                : workspaceStep === 'teams' ? (e) => { e.preventDefault(); void completeWorkspaceSetup(false); }
                : (e) => { e.preventDefault(); void handleSendInvites(); }
              }>
                <DialogHeader>
                  <DialogTitle>
                    {workspaceStep === 'details' ? 'Create Workspace' : workspaceStep === 'teams' ? 'Set Up Teams' : 'Invite Members'}
                  </DialogTitle>
                  <DialogDescription>
                    {workspaceStep === 'details'
                      ? 'Set up a new workspace.'
                      : workspaceStep === 'teams'
                      ? 'Pick the teams you need. You can always add more later.'
                      : 'Invite your team to collaborate. You can always do this later.'}
                  </DialogDescription>
                </DialogHeader>
                {workspaceStep === 'details' && (
                  <div className="space-y-4 py-4">
                    {organizations.length > 0 && (
                      <div className="space-y-2">
                        <Label>Organization</Label>
                        <Select
                          value={currentOrganization?.id ?? ''}
                          onValueChange={(id) => {
                            if (id === '__new_org__') {
                              setOrgDialogOpen(true);
                              return;
                            }
                            const org = organizations.find(o => o.id === id);
                            if (org) handleOrgSwitch(org);
                          }}
                        >
                          <SelectTrigger>
                            <SelectValue placeholder="Select organization" />
                          </SelectTrigger>
                          <SelectContent>
                            {organizations.map(org => (
                              <SelectItem key={org.id} value={org.id}>{org.name}</SelectItem>
                            ))}
                            <SelectItem value="__new_org__" className="text-primary">
                              <span className="flex items-center gap-1.5"><Plus className="h-3.5 w-3.5" /> New Organization</span>
                            </SelectItem>
                          </SelectContent>
                        </Select>
                      </div>
                    )}
                    <div className="space-y-2">
                      <Label htmlFor="ws-name">Workspace Name</Label>
                      <Input id="ws-name" placeholder="Acme Corporation" value={name} onChange={e => handleNameChange(e.target.value)} required />
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="ws-slug">Slug</Label>
                      <Input id="ws-slug" placeholder="acme-corporation" value={slug} onChange={e => setSlug(e.target.value)} required />
                      <p className="text-xs text-muted-foreground">Used in the workspace URL: /w/{slug || '...'}</p>
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="ws-key">Task Key Prefix</Label>
                      <Input
                        id="ws-key"
                        placeholder="ACM"
                        value={workspaceKey}
                        onChange={e => setWorkspaceKey(e.target.value.replace(/[^a-zA-Z]/g, '').toUpperCase().slice(0, 5))}
                        maxLength={5}
                        required
                      />
                      <p className="text-xs text-muted-foreground">
                        2-5 uppercase letters. Task IDs will look like: {workspaceKey || '...'}-1, {workspaceKey || '...'}-2, etc.
                      </p>
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="ws-website">Website (optional)</Label>
                      <Input
                        id="ws-website"
                        type="url"
                        placeholder="https://acme.com"
                        value={websiteUrl}
                        onChange={e => setWebsiteUrl(e.target.value)}
                      />
                      <p className="text-xs text-muted-foreground">Used for workspace identity and future website-aware features.</p>
                    </div>
                  </div>
                )}
                {workspaceStep === 'teams' && (
                  <div className="space-y-4 py-4">
                    <ScrollArea className="max-h-[min(380px,50vh)] pr-4">
                      <div className="space-y-2">
                        {teamDrafts.map((team) => (
                          <div
                            key={team.id}
                            className={`flex items-center gap-3 rounded-lg border px-4 py-3 transition-colors ${team.selected ? 'border-foreground/15 bg-card shadow-sm' : 'border-border/60 bg-muted/20 opacity-60'}`}
                          >
                            <Checkbox
                              checked={team.selected}
                              onCheckedChange={(checked) => updateTeamDraft(team.id, { selected: checked === true })}
                            />
                            {team.isCustom ? (
                              <Input
                                value={team.name}
                                onChange={(event) => {
                                  const newName = event.target.value;
                                  updateTeamDraft(team.id, { name: newName, handle: slugifyTeamHandle(newName) });
                                }}
                                placeholder="Team name"
                                className="h-8 min-w-0 flex-1 text-sm"
                              />
                            ) : (
                              <button
                                type="button"
                                className="min-w-0 text-left text-sm font-medium truncate"
                                onClick={() => updateTeamDraft(team.id, { selected: !team.selected })}
                              >
                                {team.name}
                              </button>
                            )}
                            {team.selected && (
                              <Tooltip>
                                <TooltipTrigger asChild>
                                  <div className="flex items-center gap-1.5 shrink-0 ml-auto">
                                    <Checkbox
                                      id={`eng-${team.id}`}
                                      checked={team.teamType === 'engineering'}
                                      onCheckedChange={(checked) => updateTeamDraft(team.id, { teamType: checked ? 'engineering' : 'custom' })}
                                      className="h-3.5 w-3.5"
                                    />
                                    <Label htmlFor={`eng-${team.id}`} className="text-xs text-muted-foreground cursor-pointer whitespace-nowrap">
                                      Eng / dev
                                    </Label>
                                  </div>
                                </TooltipTrigger>
                                <TooltipContent side="top" className="max-w-[260px] text-xs">
                                  Engineering teams get development workflows, GitHub integration, and pre-defined settings. Non-engineering teams start with a simpler setup.
                                </TooltipContent>
                              </Tooltip>
                            )}
                            {team.isCustom && (
                              <Button type="button" variant="ghost" size="icon" className="h-7 w-7 shrink-0" onClick={() => removeCustomTeam(team.id)}>
                                <X className="h-3.5 w-3.5" />
                              </Button>
                            )}
                          </div>
                        ))}
                      </div>
                    </ScrollArea>
                    <button
                      type="button"
                      onClick={addCustomTeam}
                      className="flex w-full items-center gap-2 rounded-lg border border-dashed border-border/60 px-4 py-2.5 text-sm text-muted-foreground transition-colors hover:border-foreground/20 hover:text-foreground"
                    >
                      <Plus className="h-3.5 w-3.5" />
                      Add custom team
                    </button>
                  </div>
                )}
                {workspaceStep === 'invite' && (
                  <div className="space-y-4 py-4">
                    {createdWorkspace?.website_url && (
                      <div className="rounded-lg border border-border/70 bg-muted/20 p-4">
                        <div className="flex items-start gap-3">
                          <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-background text-muted-foreground">
                            {websiteSourceAdded ? (
                              <CheckCircle2 className="h-4 w-4 text-emerald-600" />
                            ) : createWebsiteSourceMutation.isPending ? (
                              <Loader2 className="h-4 w-4 animate-spin" />
                            ) : (
                              <Favicon
                                url={createdWorkspace.website_url}
                                name={createdWorkspace.name}
                                size={32}
                                className="h-5 w-5 rounded-md border-none bg-transparent"
                                fallbackClassName="text-[8px]"
                              />
                            )}
                          </div>
                          <div className="min-w-0 flex-1 space-y-1">
                            <p className="text-sm font-medium">Use your website for Support AI?</p>
                            <p className="text-xs text-muted-foreground">
                              Add <span className="font-medium text-foreground">{createdWorkspace.website_url}</span> as a Website Content Source.
                              We&apos;ll save it at the workspace level and start syncing now. You can manage agent access later in Settings → Knowledge.
                            </p>
                            <div className="flex flex-wrap items-center gap-2 pt-1">
                              <Button
                                type="button"
                                size="sm"
                                variant={websiteSourceAdded ? 'outline' : 'default'}
                                onClick={() => void handleAddWebsiteSource()}
                                disabled={websiteSourceAdded || createWebsiteSourceMutation.isPending}
                              >
                                {createWebsiteSourceMutation.isPending
                                  ? 'Adding...'
                                  : websiteSourceAdded
                                  ? 'Added'
                                  : 'Add and sync'}
                              </Button>
                              {websiteSourceAdded && (
                                <span className="text-xs text-muted-foreground">The first sync is now queued.</span>
                              )}
                            </div>
                          </div>
                        </div>
                      </div>
                    )}
                    <div className="space-y-2">
                      <Label>Emails</Label>
                      <EmailChipInput
                        placeholder="name@example.com, name2@example.com"
                        value={inviteEmails}
                        onValueChange={setInviteEmails}
                        inputValue={inviteEmailInput}
                        onInputValueChange={setInviteEmailInput}
                      />
                      <p className="text-xs text-muted-foreground">Press comma, Enter, or Tab to turn each email into a chip. You can also paste a list.</p>
                    </div>
                    <div className="space-y-2">
                      <Label>Role</Label>
                      <div className="space-y-2">
                        {([
                          { value: 'admin', label: 'Admin', description: 'Full access across all teams. Can manage settings, workflows, labels, and members.' },
                          { value: 'member', label: 'Member', description: 'Can create and edit tasks in their teams. Can be promoted to team manager.' },
                          { value: 'viewer', label: 'Viewer', description: 'Read-only access to tasks, epics, and sprints in their assigned teams only.' },
                        ] as const).map((role) => (
                          <button
                            key={role.value}
                            type="button"
                            onClick={() => setInviteRole(role.value)}
                            className={cn(
                              'flex w-full items-start gap-3 rounded-md border p-3 text-left transition-colors',
                              inviteRole === role.value
                                ? 'border-primary bg-primary/5'
                                : 'border-border hover:bg-muted/50'
                            )}
                          >
                            <div className={cn(
                              'mt-0.5 h-4 w-4 shrink-0 rounded-full border-2',
                              inviteRole === role.value
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
                )}
                <DialogFooter>
                  {workspaceStep === 'details' && (
                    <>
                      <Button type="button" variant="outline" onClick={() => handleWorkspaceDialogChange(false)}>Cancel</Button>
                      <Button type="submit" disabled={checkingSlug}>{checkingSlug ? 'Checking...' : 'Continue'}</Button>
                    </>
                  )}
                  {workspaceStep === 'teams' && (
                    <>
                      <Button type="button" variant="outline" onClick={() => setWorkspaceStep('details')} disabled={creating}>Back</Button>
                      <div className="flex-1" />
                      <Button
                        type="button"
                        variant="ghost"
                        onClick={() => void completeWorkspaceSetup(true)}
                        disabled={creating}
                      >
                        Skip
                      </Button>
                      <Button type="submit" disabled={creating || hasSelectedTeamWithoutName}>
                        {creating ? 'Creating...' : `Create${selectedTeamCount > 0 ? ` with ${selectedTeamCount} team${selectedTeamCount === 1 ? '' : 's'}` : ''}`}
                      </Button>
                    </>
                  )}
                  {workspaceStep === 'invite' && (
                    <>
                      <div className="flex-1" />
                      <Button type="button" variant="ghost" onClick={finishWorkspaceSetup} disabled={sendingInvites || createWebsiteSourceMutation.isPending}>
                        Skip
                      </Button>
                      <Button type="submit" disabled={sendingInvites || !hasInviteRecipients || hasInvalidInviteInput}>
                        {sendingInvites ? 'Sending...' : 'Send Invites'}
                      </Button>
                    </>
                  )}
                </DialogFooter>
              </form>
            </DialogContent>
          </Dialog>
        </div>

        {isLoading ? (
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {[1, 2, 3].map(i => (
              <Skeleton key={i} className="h-24 rounded-lg" />
            ))}
          </div>
        ) : allWorkspaces.length === 0 ? (
          <div className="text-center py-16">
            <p className="text-muted-foreground mb-4">No workspaces yet.</p>
            <Button onClick={openWorkspaceDialog} disabled={!currentOrganization}>
              <Plus className="h-4 w-4 mr-2" />
              Create your first workspace
            </Button>
          </div>
        ) : organizations.length <= 1 ? (
          <WorkspaceSelector workspaces={allWorkspaces} />
        ) : (
          <div className="space-y-8">
            {workspacesByOrg.map(({ org, workspaces: orgWs }) => (
              <div key={org.id}>
                <div className="flex items-center gap-2 mb-4">
                  <UserAvatar name={org.name} avatarUrl={org.logo_url} className="h-5 w-5 rounded" fallbackClassName="text-[8px] rounded" />
                  <h2 className="text-sm font-medium text-muted-foreground">{org.name} Organization</h2>
                </div>
                <WorkspaceSelector workspaces={orgWs} />
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
