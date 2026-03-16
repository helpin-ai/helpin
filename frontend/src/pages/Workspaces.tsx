import { useEffect, useMemo, useState, type FormEvent } from 'react';
import { useNavigate, useSearch } from '@tanstack/react-router';
import { useTitle } from '@/hooks/useTitle';
import { useOrganizationStore } from '@/stores/organizationStore';
import { useOrganizations, useCreateOrganization, useWorkspaces } from '@/hooks/queries';
import { useQueryClient } from '@tanstack/react-query';
import { workspacesService } from '@/lib/services/workspacesService';
import { settingsService } from '@/lib/services/settingsService';
import { generateWorkspaceSlug } from '@/lib/slugUtils';
import { WorkspaceSelector } from '@/components/workspace/WorkspaceSelector';
import type { OrganizationWithRole } from '@/lib/types';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Textarea } from '@/components/ui/textarea';
import { Skeleton } from '@/components/ui/skeleton';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { Plus, X } from 'lucide-react';
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
  teamType: TeamType;
  selected: boolean;
  isCustom: boolean;
};

const createInitialTeamDrafts = (): TeamDraft[] =>
  WORKSPACE_TEAM_SUGGESTIONS.map((team, index) => ({
    id: `preset-${index}`,
    name: team.name,
    teamType: team.teamType,
    selected: team.selected,
    isCustom: false,
  }));

export default function Workspaces() {
  useTitle('Workspaces');
  const { data: organizations = [], isLoading: orgsLoading } = useOrganizations();
  const navigate = useNavigate();
  const { currentOrganization, setCurrentOrganization } = useOrganizationStore();
  const { data: workspaces = [], isLoading: wsLoading } = useWorkspaces(currentOrganization?.id);
  const createOrgMutation = useCreateOrganization();
  const queryClient = useQueryClient();
  const { create } = useSearch({ from: '/_authenticated/workspaces' });
  const [dialogOpen, setDialogOpen] = useState(false);
  const [orgDialogOpen, setOrgDialogOpen] = useState(false);
  const [name, setName] = useState('');
  const [slug, setSlug] = useState('');
  const [description, setDescription] = useState('');
  const [creating, setCreating] = useState(false);
  const [workspaceStep, setWorkspaceStep] = useState<'details' | 'teams'>('details');
  const [teamDrafts, setTeamDrafts] = useState<TeamDraft[]>(createInitialTeamDrafts);

  // Org creation state
  const [orgName, setOrgName] = useState('');
  const [orgSlug, setOrgSlug] = useState('');

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
  };

  const handleOrgNameChange = (val: string) => {
    setOrgName(val);
    setOrgSlug(generateWorkspaceSlug(val));
  };

  const resetWorkspaceDialog = () => {
    setWorkspaceStep('details');
    setName('');
    setSlug('');
    setDescription('');
    setTeamDrafts(createInitialTeamDrafts());
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

  const handleContinueToTeams = (e: FormEvent) => {
    e.preventDefault();
    if (!currentOrganization) {
      toast.error('Please select an organization first');
      return;
    }
    if (!name.trim() || !slug.trim()) {
      toast.error('Enter a workspace name and slug');
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
      organization_id: currentOrganization.id,
      description: description || undefined,
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
            handle: slugifyTeamHandle(team.name),
            team_type: team.teamType,
            default_story_type: TEAM_TYPE_PRESETS[team.teamType].defaultStoryType,
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
        }),
      );

      createdTeamCount = results.filter((result) => result.status === 'fulfilled').length;
      failedTeamCount = results.length - createdTeamCount;
    }

    setCreating(false);
    setDialogOpen(false);
    resetWorkspaceDialog();
    queryClient.invalidateQueries({ queryKey: ['workspaces'] });

    if (failedTeamCount > 0) {
      toast.warning(`Workspace created. Added ${createdTeamCount} team${createdTeamCount === 1 ? '' : 's'}, but ${failedTeamCount} failed.`);
    } else if (createdTeamCount > 0) {
      toast.success(`Workspace created with ${createdTeamCount} team${createdTeamCount === 1 ? '' : 's'}.`);
    } else {
      toast.success('Workspace created');
    }

    // Auto-navigate to the new workspace
    if (workspace) {
      void navigate({ to: `/w/${workspace.slug}/pm/my-work` });
    }
  };

  const handleOrgSwitch = (org: OrganizationWithRole) => {
    setCurrentOrganization(org);
  };

  const isLoading = wsLoading || orgsLoading;
  const selectedTeamCount = useMemo(
    () => teamDrafts.filter((team) => team.selected && team.name.trim()).length,
    [teamDrafts],
  );
  const hasSelectedTeamWithoutName = teamDrafts.some((team) => team.selected && !team.name.trim());

  // Show create org screen if user has no organizations
  if (!orgsLoading && organizations.length === 0) {
    return (
      <div className="min-h-screen bg-background">
        <div className="max-w-md mx-auto px-4 py-24">
          <div className="text-center mb-8">
            <div className="mx-auto mb-4">
              <UserAvatar name="Organization" className="h-14 w-14 rounded-full" fallbackClassName="text-xl rounded-full" />
            </div>
            <h1 className="text-2xl font-bold">Create your Organization</h1>
            <p className="text-muted-foreground mt-2">
              Organizations group your workspaces and team members together.
            </p>
          </div>
          <form onSubmit={handleCreateOrg} className="space-y-4">
            <OrgFormFields
              name={orgName} slug={orgSlug}
              onNameChange={handleOrgNameChange} onSlugChange={setOrgSlug}
              nameId="org-name" slugId="org-slug"
            />
            <Button type="submit" className="w-full" disabled={createOrgMutation.isPending}>
              {createOrgMutation.isPending ? 'Creating...' : 'Create Organization'}
            </Button>
          </form>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-background">
      <div className="max-w-6xl mx-auto px-4 py-12">
        {/* Organization switcher */}
        {organizations.length > 0 && (
          <div className="flex items-center gap-3 mb-6">
            <UserAvatar name={currentOrganization?.name} avatarUrl={currentOrganization?.logo_url} className="h-6 w-6 rounded" fallbackClassName="text-[9px] rounded" />
            {organizations.length === 1 ? (
              <span className="text-sm font-medium">{currentOrganization?.name}</span>
            ) : (
              <Select
                value={currentOrganization?.id ?? ''}
                onValueChange={(id) => {
                  const org = organizations.find(o => o.id === id);
                  if (org) handleOrgSwitch(org);
                }}
              >
                <SelectTrigger className="w-[240px] h-8">
                  <SelectValue placeholder="Select organization" />
                </SelectTrigger>
                <SelectContent>
                  {organizations.map(org => (
                    <SelectItem key={org.id} value={org.id}>{org.name}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
            <Dialog open={orgDialogOpen} onOpenChange={setOrgDialogOpen}>
              <DialogTrigger asChild>
                <Button variant="outline" size="sm">
                  <Plus className="h-3.5 w-3.5 mr-1" />
                  New Org
                </Button>
              </DialogTrigger>
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
          </div>
        )}

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
              <form onSubmit={workspaceStep === 'details' ? handleContinueToTeams : (e) => { e.preventDefault(); void completeWorkspaceSetup(false); }}>
                <DialogHeader>
                  <DialogTitle>{workspaceStep === 'details' ? 'Create Workspace' : 'Set Up Teams'}</DialogTitle>
                  <DialogDescription>
                    {workspaceStep === 'details'
                      ? `Set up a new workspace in ${currentOrganization?.name}.`
                      : 'Pick the teams you need. You can always add more later.'}
                  </DialogDescription>
                </DialogHeader>
                {workspaceStep === 'details' ? (
                  <div className="space-y-4 py-4">
                    <div className="space-y-2">
                      <Label htmlFor="ws-name">Name</Label>
                      <Input id="ws-name" placeholder="Acme Corporation" value={name} onChange={e => handleNameChange(e.target.value)} required />
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="ws-slug">Slug</Label>
                      <Input id="ws-slug" placeholder="acme-corporation" value={slug} onChange={e => setSlug(e.target.value)} required />
                      <p className="text-xs text-muted-foreground">Used in the workspace URL: /w/{slug || '...'}</p>
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="ws-desc">Description (optional)</Label>
                      <Textarea id="ws-desc" placeholder="A brief description of this workspace" value={description} onChange={e => setDescription(e.target.value)} />
                    </div>
                  </div>
                ) : (
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
                                onChange={(event) => updateTeamDraft(team.id, { name: event.target.value })}
                                placeholder="Team name"
                                className="h-8 min-w-0 flex-1 text-sm"
                              />
                            ) : (
                              <button
                                type="button"
                                className="min-w-0 flex-1 text-left text-sm font-medium"
                                onClick={() => updateTeamDraft(team.id, { selected: !team.selected })}
                              >
                                {team.name}
                              </button>
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
                <DialogFooter>
                  {workspaceStep === 'details' ? (
                    <>
                      <Button type="button" variant="outline" onClick={() => handleWorkspaceDialogChange(false)}>Cancel</Button>
                      <Button type="submit">Continue</Button>
                    </>
                  ) : (
                    <>
                      <Button type="button" variant="outline" onClick={() => setWorkspaceStep('details')} disabled={creating}>Back</Button>
                      <div className="flex-1" />
                      <button
                        type="button"
                        className="text-xs text-muted-foreground hover:text-foreground transition-colors disabled:opacity-50"
                        onClick={() => void completeWorkspaceSetup(true)}
                        disabled={creating}
                      >
                        Skip
                      </button>
                      <Button type="submit" disabled={creating || hasSelectedTeamWithoutName}>
                        {creating ? 'Creating...' : `Create${selectedTeamCount > 0 ? ` with ${selectedTeamCount} team${selectedTeamCount === 1 ? '' : 's'}` : ''}`}
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
        ) : workspaces.length === 0 ? (
          <div className="text-center py-16">
            <p className="text-muted-foreground mb-4">No workspaces in this organization yet.</p>
            <Button onClick={openWorkspaceDialog} disabled={!currentOrganization}>
              <Plus className="h-4 w-4 mr-2" />
              Create your first workspace
            </Button>
          </div>
        ) : (
          <WorkspaceSelector workspaces={workspaces} />
        )}
      </div>
    </div>
  );
}
