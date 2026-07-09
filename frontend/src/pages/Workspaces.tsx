import { useEffect, useMemo, useRef, useState, type FormEvent } from 'react';
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
import { validateWorkspaceOnboardingDetails } from '@/lib/workspaceOnboardingDetails';
import { workspaceDefaultsForCreation } from '@/lib/workspaceOnboardingDefaults';
import {
  type WorkspaceOnboardingStep,
  shouldClearWorkspaceCreateSearchAfterDialogOpen,
  shouldContinueToInviteStepAfterWorkspaceCreate,
  shouldShowWorkspaceOnboardingOrganizationSelector,
  shouldUseFullPageWorkspaceOnboarding,
} from '@/lib/workspaceOnboardingMode';
import {
  ONBOARDING_USE_CASE_OPTIONS,
  trackWorkspaceOnboardingUseCases,
  type WorkspaceOnboardingUseCase,
} from '@/lib/workspaceOnboardingUseCases';
import { WorkspaceSelector } from '@/components/workspace/WorkspaceSelector';
import { HelpinLogo } from '@/components/layout/HelpinLogo';
import type { OrganizationWithRole } from '@/lib/types';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { EmailChipInput, classifyEmailChipInput, mergeEmailChips } from '@/components/ui/email-chip-input';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Skeleton } from '@/components/ui/skeleton';
import { Textarea } from '@/components/ui/textarea';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { cn } from '@/lib/utils';
import {
  BookOpen01Icon,
  DollarCircleIcon,
  File01Icon,
  FolderKanbanIcon,
  HeadphonesIcon,
  PlusSignIcon,
  Cancel01Icon,
  Logout01Icon,
  SourceCodeIcon,
  Tick01Icon,
  type IconComponent,
} from '@/lib/icons';
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

const workspaceOnboardingProgressSteps: Exclude<WorkspaceOnboardingStep, 'learning'>[] = [
  'details',
  'use-cases',
  'context',
  'teams',
  'invite',
];

const createInitialTeamDrafts = (): TeamDraft[] =>
  WORKSPACE_TEAM_SUGGESTIONS.map((team, index) => ({
    id: `preset-${index}`,
    name: team.name,
    handle: slugifyTeamHandle(team.name),
    teamType: team.teamType,
    selected: team.selected,
    isCustom: false,
  }));

const useCaseIcons = {
  product_engineering: SourceCodeIcon,
  team_project_management: FolderKanbanIcon,
  customer_support: HeadphonesIcon,
  help_center_docs: BookOpen01Icon,
  internal_docs: File01Icon,
  sales_crm: DollarCircleIcon,
} satisfies Record<WorkspaceOnboardingUseCase, IconComponent>;

const useCaseIconStyles = {
  product_engineering: 'border-sky-200 bg-sky-50 text-sky-700',
  team_project_management: 'border-emerald-200 bg-emerald-50 text-emerald-700',
  customer_support: 'border-amber-200 bg-amber-50 text-amber-700',
  help_center_docs: 'border-violet-200 bg-violet-50 text-violet-700',
  internal_docs: 'border-rose-200 bg-rose-50 text-rose-700',
  sales_crm: 'border-cyan-200 bg-cyan-50 text-cyan-700',
} satisfies Record<WorkspaceOnboardingUseCase, string>;

function ProductLearningAnimation({ domain }: { domain: string }) {
  return (
    <div className="relative h-40 w-full overflow-hidden rounded-lg border border-border/70 bg-background">
      <svg viewBox="0 0 520 220" className="h-full w-full" role="img" aria-label={`Learning from ${domain}`}>
        <defs>
          <linearGradient id="onboarding-scan" x1="0" y1="0" x2="1" y2="0">
            <stop offset="0%" stopColor="currentColor" stopOpacity="0" />
            <stop offset="50%" stopColor="currentColor" stopOpacity="0.28" />
            <stop offset="100%" stopColor="currentColor" stopOpacity="0" />
          </linearGradient>
        </defs>
        <rect x="0" y="0" width="520" height="220" rx="16" className="fill-muted/20" />
        <g className="text-muted-foreground">
          <path d="M160 112 C210 76 300 76 360 112" fill="none" stroke="currentColor" strokeOpacity="0.18" strokeWidth="2" strokeDasharray="5 9">
            <animate attributeName="stroke-dashoffset" from="0" to="-28" dur="1.8s" repeatCount="indefinite" />
          </path>
          <path d="M160 132 C218 160 296 160 360 132" fill="none" stroke="currentColor" strokeOpacity="0.14" strokeWidth="2" strokeDasharray="5 9">
            <animate attributeName="stroke-dashoffset" from="0" to="28" dur="2.2s" repeatCount="indefinite" />
          </path>
        </g>
        <g>
          <rect x="54" y="58" width="124" height="104" rx="10" className="fill-background stroke-border" />
          <rect x="72" y="78" width="64" height="7" rx="3.5" className="fill-foreground/30" />
          <rect x="72" y="98" width="86" height="6" rx="3" className="fill-muted-foreground/25" />
          <rect x="72" y="114" width="74" height="6" rx="3" className="fill-muted-foreground/20" />
          <rect x="72" y="134" width="42" height="10" rx="5" className="fill-primary/20" />
          <rect x="54" y="58" width="28" height="104" rx="10" className="fill-muted/45" />
          <rect x="95" y="46" width="58" height="8" rx="4" className="fill-primary/30">
            <animate attributeName="opacity" values="0.35;0.9;0.35" dur="1.7s" repeatCount="indefinite" />
          </rect>
        </g>
        <g>
          <rect x="348" y="54" width="122" height="112" rx="12" className="fill-background stroke-border" />
          <rect x="374" y="78" width="70" height="8" rx="4" className="fill-foreground/25" />
          <rect x="374" y="102" width="48" height="6" rx="3" className="fill-muted-foreground/25" />
          <rect x="374" y="118" width="76" height="6" rx="3" className="fill-muted-foreground/20" />
          <rect x="374" y="134" width="58" height="6" rx="3" className="fill-muted-foreground/20" />
          <path d="M348 96 H470" className="stroke-border" strokeWidth="1" />
          <rect x="348" y="54" width="122" height="112" rx="12" fill="url(#onboarding-scan)" className="text-primary">
            <animate attributeName="x" values="300;470;300" dur="2.4s" repeatCount="indefinite" />
          </rect>
        </g>
        <g>
          <rect x="222" y="72" width="78" height="78" rx="18" className="fill-primary/10 stroke-primary/30" />
          <path d="M242 118 h38 M242 102 h30 M242 134 h22" className="stroke-primary" strokeWidth="4" strokeLinecap="round" opacity="0.65" />
          <path d="M288 82 l12 12 -12 12" fill="none" className="stroke-primary" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round">
            <animate attributeName="opacity" values="0.35;1;0.35" dur="1.4s" repeatCount="indefinite" />
          </path>
          <animateTransform attributeName="transform" type="translate" values="0 0; 0 -4; 0 0" dur="2.2s" repeatCount="indefinite" />
        </g>
      </svg>
    </div>
  );
}

export default function Workspaces({ dedicatedOnboarding = false }: { dedicatedOnboarding?: boolean }) {
  useTitle(dedicatedOnboarding ? 'Onboarding' : 'Workspaces');
  const { data: organizations = [], isLoading: orgsLoading } = useOrganizations();
  const navigate = useNavigate();
  const { user, signOut } = useAuthStore();
  const { currentOrganization, setCurrentOrganization } = useOrganizationStore();
  const { data: allWorkspaces = [], isLoading: wsLoading } = useWorkspaces();
  const createOrgMutation = useCreateOrganization();
  const queryClient = useQueryClient();
  const { create: createSearchParam } = useSearch({ strict: false }) as { create?: boolean };
  const create = dedicatedOnboarding || createSearchParam;
  const [dialogOpen, setDialogOpen] = useState(false);
  const [orgDialogOpen, setOrgDialogOpen] = useState(false);
  const [name, setName] = useState('');
  const [slug, setSlug] = useState('');
  const [workspaceKey, setWorkspaceKey] = useState('');
  const [websiteUrl, setWebsiteUrl] = useState('');
  const [creating, setCreating] = useState(false);
  const [workspaceStep, setWorkspaceStep] = useState<WorkspaceOnboardingStep>('details');
  const [createdWorkspace, setCreatedWorkspace] = useState<{ id: string; slug: string; name: string; website_url?: string } | null>(null);
  const [companyProductDescription, setCompanyProductDescription] = useState('');
  const [generatingDescription, setGeneratingDescription] = useState(false);
  const [createdTeamIds, setCreatedTeamIds] = useState<string[]>([]);
  const [inviteEmails, setInviteEmails] = useState<string[]>([]);
  const [inviteEmailInput, setInviteEmailInput] = useState('');
  const [inviteRole, setInviteRole] = useState('member');
  const [sendingInvites, setSendingInvites] = useState(false);
  const [selectedUseCases, setSelectedUseCases] = useState<WorkspaceOnboardingUseCase[]>([]);
  const [teamDrafts, setTeamDrafts] = useState<TeamDraft[]>(createInitialTeamDrafts);
  const [fullPageOnboardingInitialized, setFullPageOnboardingInitialized] = useState(false);
  const [autoOrgCreateAttempted, setAutoOrgCreateAttempted] = useState(false);
  const [autoOrgCreateFailed, setAutoOrgCreateFailed] = useState(false);
  const blankWorkspaceDefaults = workspaceDefaultsForCreation({
    email: user?.email,
    useEmailDefaults: false,
  });
  const firstWorkspaceDefaults = workspaceDefaultsForCreation({
    email: user?.email,
    useEmailDefaults: true,
  });
  const companyContextGenerationRef = useRef<{ key: string; promise: Promise<void> } | null>(null);

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

  // Auto-open create dialog when existing users navigate with ?create=true.
  // First-time signup uses the full-page onboarding below.
  useEffect(() => {
    if (!dedicatedOnboarding && create && currentOrganization && !orgsLoading && !wsLoading && allWorkspaces.length > 0 && !createdWorkspace && !dialogOpen) {
      openWorkspaceDialog();
      if (shouldClearWorkspaceCreateSearchAfterDialogOpen({ create: Boolean(create), isFullPageOnboarding: false })) {
        void navigate({
          to: '/workspaces',
          search: { create: undefined, step: undefined },
          replace: true,
        });
      }
    }
  }, [create, currentOrganization, dedicatedOnboarding, orgsLoading, wsLoading, allWorkspaces.length, createdWorkspace, dialogOpen, navigate]);

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

  const resetWorkspaceDialog = ({ useEmailDefaults = false }: { useEmailDefaults?: boolean } = {}) => {
    const workspaceDefaults = useEmailDefaults ? firstWorkspaceDefaults : blankWorkspaceDefaults;
    setWorkspaceStep('details');
    setName(workspaceDefaults.name);
    setSlug(workspaceDefaults.slug);
    setWorkspaceKey(workspaceDefaults.workspaceKey);
    setWebsiteUrl(workspaceDefaults.websiteUrl);
    setCompanyProductDescription('');
    setGeneratingDescription(false);
    setTeamDrafts(createInitialTeamDrafts());
    setCreatedWorkspace(null);
    setCreatedTeamIds([]);
    setInviteEmails([]);
    setInviteEmailInput('');
    setInviteRole('member');
    setSelectedUseCases([]);
    companyContextGenerationRef.current = null;
  };

  const openWorkspaceDialog = () => {
    resetWorkspaceDialog({ useEmailDefaults: false });
    setDialogOpen(true);
  };

  const handleWorkspaceDialogChange = (open: boolean) => {
    if (!open && creating) return;
    setDialogOpen(open);
    if (!open) setTimeout(resetWorkspaceDialog, 300);
  };

  const handleCreateOrg = async (e: FormEvent) => {
    e.preventDefault();
    try {
      const data = await createOrgMutation.mutateAsync({ name: orgName, slug: orgSlug });
      toast.success('Organization created');
      setOrgDialogOpen(false);
      setOrgName('');
      setOrgSlug('');
      if (data) {
        setAutoOrgCreateFailed(false);
        setAutoOrgCreateAttempted(true);
        setCurrentOrganization(data);
      }
    } catch {
      toast.error('Failed to create organization');
    }
  };

  const [checkingSlug] = useState(false);

  const startCompanyProductDescriptionGeneration = (nextName = name, nextWebsiteUrl = websiteUrl) => {
    const trimmedWebsiteUrl = nextWebsiteUrl.trim();
    if (!trimmedWebsiteUrl) {
      return Promise.resolve();
    }

    const key = `${nextName.trim()}::${trimmedWebsiteUrl}`;
    if (companyContextGenerationRef.current?.key === key) {
      return companyContextGenerationRef.current.promise;
    }

    setGeneratingDescription(true);
    const promise = workspacesService.generateCompanyProductDescription({
      workspace_name: nextName.trim(),
      website_url: trimmedWebsiteUrl,
    }).then(({ data, error }) => {
      if (companyContextGenerationRef.current?.key !== key) {
        return;
      }
      if (data?.company_product_context || data?.description) {
        setCompanyProductDescription(data.company_product_context || data.description);
      } else if (error) {
        toast.warning('Could not generate company/product context', { description: error });
      }
    }).finally(() => {
      if (companyContextGenerationRef.current?.key === key) {
        setGeneratingDescription(false);
      }
    });

    companyContextGenerationRef.current = { key, promise };
    return promise;
  };

  const handleContinueFromDetails = async (e: FormEvent) => {
    e.preventDefault();
    const validationError = validateWorkspaceOnboardingDetails({
      hasOrganization: Boolean(activeOrganization),
      name,
      slug,
      websiteUrl,
    });
    if (validationError) {
      toast.error(validationError);
      return;
    }
    if (!currentOrganization && activeOrganization) {
      setCurrentOrganization(activeOrganization);
    }
    setWorkspaceStep('use-cases');
    void startCompanyProductDescriptionGeneration();
  };

  const toggleUseCase = (useCase: WorkspaceOnboardingUseCase) => {
    setSelectedUseCases((current) => (
      current.includes(useCase)
        ? current.filter((value) => value !== useCase)
        : [...current, useCase]
    ));
  };

  const continueFromUseCases = async () => {
    if (selectedUseCases.length === 0) {
      toast.error('Select at least one use case');
      return;
    }
    trackWorkspaceOnboardingUseCases(selectedUseCases);
    if (generatingDescription || !companyProductDescription.trim()) {
      setWorkspaceStep('learning');
      await startCompanyProductDescriptionGeneration();
    }
    setWorkspaceStep('context');
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

  const createWorkspaceOnly = async ({ showCreatedToast = true }: { showCreatedToast?: boolean } = {}) => {
    if (createdWorkspace) {
      return createdWorkspace;
    }
    const organization = activeOrganization;
    if (!organization) {
      toast.error('Please select an organization first');
      return null;
    }
    if (!currentOrganization) {
      setCurrentOrganization(organization);
    }
    setCreating(true);
    const { data: workspace, error } = await workspacesService.create({
      name,
      slug,
      workspace_key: (workspaceKey || name.replace(/[^a-zA-Z]/g, '').slice(0, 3) || 'WS').toUpperCase(),
      organization_id: organization.id,
      company_product_context: companyProductDescription.trim() || undefined,
      website_url: websiteUrl.trim() || undefined,
      timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
    });
    if (error) {
      setCreating(false);
      toast.error(error);
      return null;
    }
    setCreating(false);
    queryClient.invalidateQueries({ queryKey: ['workspaces'] });
    if (!workspace) return null;
    const created = {
      id: workspace.id,
      slug: workspace.slug,
      name: workspace.name,
      website_url: workspace.website_url,
    };
    setCreatedWorkspace(created);
    if (showCreatedToast) {
      toast.success('Workspace created');
    }
    return created;
  };

  const handleSaveContext = async () => {
    const workspace = await createWorkspaceOnly({ showCreatedToast: false });
    if (workspace) {
      setWorkspaceStep('teams');
    }
  };

  const completeWorkspaceSetup = async (skipTeams = false) => {
    const workspace = await createWorkspaceOnly();
    if (!workspace) {
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

    if (selectedTeams.length > 0) {
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

    queryClient.invalidateQueries({ queryKey: ['workspaces'] });

    if (failedTeamCount > 0) {
      toast.warning(`Workspace created. Added ${createdTeamCount} team${createdTeamCount === 1 ? '' : 's'}, but ${failedTeamCount} failed.`);
    } else if (createdTeamCount > 0) {
      toast.success(`Workspace created with ${createdTeamCount} team${createdTeamCount === 1 ? '' : 's'}.`);
    } else {
      toast.success('Workspace created');
    }

    if (shouldContinueToInviteStepAfterWorkspaceCreate({ isFullPageOnboarding })) {
      setWorkspaceStep('invite');
      return;
    }

    finishWorkspaceSetup(workspace);
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
    const failures: { email: string; error: string }[] = [];
    await Promise.all(
      emails.map(async (email) => {
        const { data, error } = await inviteService.send({ workspace_id: createdWorkspace.id, email, role: inviteRole });
        if (error) {
          failures.push({ email, error });
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
    const failureLines = failures.map((f) => `${f.email}: ${f.error}`);
    if (sent > 0 && failures.length > 0) {
      toast.warning(`${sent} of ${emails.length} invitations sent`, {
        description: failureLines.join('\n'),
      });
    } else if (sent > 0) {
      toast.success(`${sent} invitation${sent === 1 ? '' : 's'} sent`);
    } else if (failures.length === 1) {
      toast.error(`Failed to invite ${failures[0].email}`, {
        description: failures[0].error,
      });
    } else {
      toast.error('Failed to send invitations', {
        description: failureLines.join('\n'),
      });
    }
    finishWorkspaceSetup();
  };

  const finishWorkspaceSetup = (workspaceOverride?: { slug: string }) => {
    const ws = workspaceOverride ?? createdWorkspace;
    setDialogOpen(false);
    if (ws) {
      void navigate({ to: `/w/${ws.slug}/pm/my-work` });
    }
    setTimeout(resetWorkspaceDialog, 300);
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

  const activeOrganization = currentOrganization ?? organizations[0] ?? null;
  const isLoading = wsLoading || orgsLoading;
  const isFullPageOnboarding = shouldUseFullPageWorkspaceOnboarding({
    create,
    forceFullPage: dedicatedOnboarding,
    isLoading,
    workspaceCount: allWorkspaces.length,
    hasStartedOnboarding: fullPageOnboardingInitialized,
  });
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
  const websiteDomain = websiteUrl.replace(/^https?:\/\//, '').replace(/\/.*$/, '') || 'your website';
  const preparingOrganization = orgsLoading || createOrgMutation.isPending || (!activeOrganization && !autoOrgCreateAttempted);
  const needsOrganization = !preparingOrganization && !activeOrganization;

  useEffect(() => {
    if (!isFullPageOnboarding || fullPageOnboardingInitialized) {
      return;
    }
    resetWorkspaceDialog({ useEmailDefaults: true });
    setFullPageOnboardingInitialized(true);
  }, [isFullPageOnboarding, fullPageOnboardingInitialized]);

  useEffect(() => {
    if (!isFullPageOnboarding) {
      return;
    }
    void navigate({
      to: '/onboarding',
      search: { step: workspaceStep },
      replace: true,
    });
  }, [dedicatedOnboarding, isFullPageOnboarding, navigate, workspaceStep]);

  const handleWorkspaceOnboardingSubmit = (event: FormEvent) => {
    event.preventDefault();
    if (workspaceStep === 'details') {
      void handleContinueFromDetails(event);
      return;
    }
    if (workspaceStep === 'use-cases') {
      void continueFromUseCases();
      return;
    }
    if (workspaceStep === 'context') {
      void handleSaveContext();
      return;
    }
    if (workspaceStep === 'teams') {
      void completeWorkspaceSetup(false);
      return;
    }
    if (workspaceStep === 'invite') {
      void handleSendInvites();
    }
  };

  const workspaceOnboardingTitle =
    workspaceStep === 'details' ? 'Create workspace'
      : workspaceStep === 'use-cases' ? 'What will you use Helpin AI agents for?'
      : workspaceStep === 'learning' ? `Learning about ${websiteDomain}`
      : workspaceStep === 'context' ? 'Company/product context'
      : workspaceStep === 'teams' ? 'Set up teams'
      : 'Invite members';

  const workspaceOnboardingDescription =
    workspaceStep === 'details'
      ? 'Set up a new workspace.'
      : workspaceStep === 'use-cases'
      ? 'Select where you want AI agents to help.'
      : workspaceStep === 'learning'
      ? 'Reading a few public pages and drafting a description you can review.'
      : workspaceStep === 'context'
      ? 'Review the context Helpin agents should use to understand your company and product.'
      : workspaceStep === 'teams'
      ? 'Pick the teams you need. You can always add more later.'
      : 'Invite your team to collaborate. You can always do this later.';

  const workspaceOnboardingDescriptionLines =
    workspaceStep === 'context'
      ? [workspaceOnboardingDescription, 'You can also edit it later in Settings > Knowledge.']
      : [workspaceOnboardingDescription];

  const renderWorkspaceOnboardingBody = (fullPage = false) => (
    <>
      {workspaceStep === 'details' && (
        <div className="space-y-4 py-4">
          {needsOrganization && (
            <div className="rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-950">
              <p className="font-medium">
                {autoOrgCreateFailed ? 'Organization setup needs your attention.' : 'Create an organization to continue.'}
              </p>
              <p className="mt-1 text-amber-900">
                Workspaces belong to an organization. Create one now, then continue setting up your workspace.
              </p>
              <Button
                type="button"
                size="sm"
                variant="outline"
                className="mt-3 border-amber-300 bg-white text-amber-950 hover:bg-amber-100"
                onClick={() => setOrgDialogOpen(true)}
              >
                Create organization
              </Button>
            </div>
          )}
          {shouldShowWorkspaceOnboardingOrganizationSelector({
            isFullPageOnboarding: fullPage,
            organizationCount: organizations.length,
          }) && (
            <div className="space-y-2">
              <Label>Organization</Label>
              <Select
                value={activeOrganization?.id ?? ''}
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
                    <span className="flex items-center gap-1.5"><PlusSignIcon className="h-3.5 w-3.5" /> New Organization</span>
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>
          )}
          <div className="space-y-2">
            <Label htmlFor="ws-name">Workspace Name</Label>
            <Input id="ws-name" placeholder="Acme Corporation" value={name} onChange={e => handleNameChange(e.target.value)} required />
          </div>
          <input type="hidden" value={slug} />
          <input type="hidden" value={workspaceKey} />
          <div className="space-y-2">
            <Label htmlFor="ws-website">Company/product website</Label>
            <Input
              id="ws-website"
              type="url"
              placeholder="https://acme.com"
              value={websiteUrl}
              onChange={e => setWebsiteUrl(e.target.value)}
              required
            />
            <p className="text-xs text-muted-foreground">Used to generate company/product context for Helpin AI agents.</p>
          </div>
        </div>
      )}
      {workspaceStep === 'learning' && (
        <div className="py-10">
          <div className="mx-auto flex max-w-md flex-col items-center text-center">
            <ProductLearningAnimation domain={websiteDomain} />
            <p className="mt-4 text-sm font-medium">Learning about your product</p>
            <p className="mt-2 text-sm leading-6 text-muted-foreground">
              We are reading public website pages and drafting company/product context for your agents.
            </p>
          </div>
        </div>
      )}
      {workspaceStep === 'use-cases' && (
        <div className="grid gap-3 py-4 sm:grid-cols-2">
          {ONBOARDING_USE_CASE_OPTIONS.map((option) => {
            const selected = selectedUseCases.includes(option.value);
            const Icon = useCaseIcons[option.value];
            const iconStyle = useCaseIconStyles[option.value];
            return (
              <button
                key={option.value}
                type="button"
                aria-pressed={selected}
                onClick={() => toggleUseCase(option.value)}
                className={cn(
                  'relative flex min-h-[156px] w-full flex-col rounded-lg border p-4 text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2',
                  selected
                    ? 'border-primary/50 bg-primary/5 shadow-sm'
                    : 'border-border/70 bg-background hover:border-foreground/20 hover:bg-muted/30',
                )}
              >
                <span className="flex items-start gap-3">
                  <span className={cn(
                    'flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border',
                    iconStyle,
                    selected && 'ring-2 ring-current/15',
                  )}>
                    <Icon className="h-4 w-4" />
                  </span>
                  <span className="min-w-0 flex-1 pt-0.5 text-sm font-medium leading-5">{option.label}</span>
                  <span className={cn(
                    'flex h-5 w-5 shrink-0 items-center justify-center rounded-full border',
                    selected
                      ? 'border-primary bg-primary text-primary-foreground'
                      : 'border-border bg-background text-transparent',
                  )}>
                    <Tick01Icon className="h-3.5 w-3.5" />
                  </span>
                </span>
                <span className="mt-3 block min-w-0">
                  <span className="block text-xs leading-5 text-muted-foreground">{option.description}</span>
                </span>
                <span className="mt-auto block border-t border-border/60 pt-3 text-[11px] leading-4 text-muted-foreground/80">
                  {option.replaces}
                </span>
              </button>
            );
          })}
        </div>
      )}
      {workspaceStep === 'context' && (
        <div className="space-y-2 py-4">
          <div className="space-y-2">
            <Textarea
              id="company-product-description"
              value={companyProductDescription}
              onChange={(event) => setCompanyProductDescription(event.target.value)}
              placeholder="Plain text about what your company or product does, who it serves, and what problems it solves."
              rows={12}
              className="max-h-[min(42vh,420px)] min-h-[220px] resize-none overflow-y-auto"
            />
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
                            className={cn(
                              'flex items-center gap-3 rounded-lg border px-4 py-3 transition-colors',
                              team.selected
                                ? 'border-foreground/15 bg-card shadow-sm'
                                : 'border-border/70 bg-background hover:border-foreground/20 hover:bg-muted/30',
                            )}
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
                      <Cancel01Icon className="h-3.5 w-3.5" />
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
            <PlusSignIcon className="h-3.5 w-3.5" />
            Add custom team
          </button>
        </div>
      )}
      {workspaceStep === 'invite' && (
        <div className="space-y-4 py-4">
          <div className="space-y-2">
            <Label>Emails</Label>
            <EmailChipInput
              placeholder="name@example.com, name2@example.com"
              value={inviteEmails}
              onValueChange={setInviteEmails}
              inputValue={inviteEmailInput}
              onInputValueChange={setInviteEmailInput}
              autoFocus
            />
            <p className="text-xs text-muted-foreground">Press comma, Enter, or Tab to turn each email into a chip. You can also paste a list.</p>
          </div>
          <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
            <div className="space-y-1">
              <Label htmlFor="invite-role">Role</Label>
              <p className="text-xs text-muted-foreground">Applies to all invites. You can change it later in settings.</p>
            </div>
            <Select value={inviteRole} onValueChange={setInviteRole}>
              <SelectTrigger id="invite-role" className="w-full sm:w-44">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="member">Member</SelectItem>
                <SelectItem value="admin">Admin</SelectItem>
                <SelectItem value="viewer">Viewer</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>
      )}
    </>
  );

  const renderWorkspaceOnboardingFooter = (fullPage = false) => (
    <>
      {workspaceStep === 'details' && (
        <>
          {!fullPage && <Button type="button" variant="outline" onClick={() => handleWorkspaceDialogChange(false)}>Cancel</Button>}
          {needsOrganization ? (
            <Button type="button" onClick={() => setOrgDialogOpen(true)}>Create organization</Button>
          ) : (
            <Button type="submit" disabled={checkingSlug || preparingOrganization}>
              {preparingOrganization ? 'Preparing...' : checkingSlug ? 'Checking...' : 'Continue'}
            </Button>
          )}
        </>
      )}
      {workspaceStep === 'use-cases' && (
        <>
          <Button type="button" variant="outline" onClick={() => setWorkspaceStep('details')}>Back</Button>
          <div className="flex-1" />
          <Button type="submit" disabled={selectedUseCases.length === 0}>Continue</Button>
        </>
      )}
      {workspaceStep === 'learning' && (
        <Button type="button" disabled>
          {generatingDescription ? 'Learning...' : 'Preparing...'}
        </Button>
      )}
      {workspaceStep === 'context' && (
        <>
          <Button type="button" variant="outline" onClick={() => setWorkspaceStep('use-cases')} disabled={creating}>Back</Button>
          <div className="flex-1" />
          <Button type="submit" disabled={creating}>{creating ? 'Saving...' : 'Save and continue'}</Button>
        </>
      )}
      {workspaceStep === 'teams' && (
        <>
          <Button type="button" variant="outline" onClick={() => setWorkspaceStep('context')} disabled={creating}>Back</Button>
          <div className="flex-1" />
          <Button type="submit" disabled={creating || hasSelectedTeamWithoutName}>
            {creating ? 'Creating...' : `Create${selectedTeamCount > 0 ? ` with ${selectedTeamCount} team${selectedTeamCount === 1 ? '' : 's'}` : ''}`}
          </Button>
        </>
      )}
      {workspaceStep === 'invite' && (
        <>
          <Button type="button" variant="outline" onClick={() => setWorkspaceStep('teams')} disabled={sendingInvites}>Back</Button>
          <div className="flex-1" />
          <Button type="button" variant="ghost" onClick={() => finishWorkspaceSetup()} disabled={sendingInvites}>
            Skip
          </Button>
          <Button type="submit" disabled={sendingInvites || !hasInviteRecipients || hasInvalidInviteInput}>
            {sendingInvites ? 'Sending...' : 'Send Invites'}
          </Button>
        </>
      )}
    </>
  );

  // Auto-create org if user has none (edge case — signup normally handles this).
  useEffect(() => {
    if (!orgsLoading && organizations.length === 0 && !createOrgMutation.isPending && user && !autoOrgCreateAttempted) {
      setAutoOrgCreateAttempted(true);
      setAutoOrgCreateFailed(false);
      const first = user.full_name?.split(' ')[0] || 'My';
      const name = `${first}'s Organization`;
      createOrgMutation.mutateAsync({ name, slug: generateWorkspaceSlug(name) }).then((org) => {
        setCurrentOrganization(org);
      }).catch(() => {
        setAutoOrgCreateFailed(true);
        toast.error('Could not prepare your organization. Create one to continue.');
      });
    }
  }, [orgsLoading, organizations.length, createOrgMutation.isPending, user, autoOrgCreateAttempted, createOrgMutation, setCurrentOrganization]);

  const onboardingProgressStep = workspaceStep === 'learning' ? 'context' : workspaceStep;
  const onboardingProgressIndex = Math.max(workspaceOnboardingProgressSteps.indexOf(onboardingProgressStep), 0);
  const onboardingContentWidth = workspaceStep === 'use-cases' ? 'max-w-4xl' : 'max-w-2xl';
  const organizationCreateDialog = (
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
  );

  if (isFullPageOnboarding) {
    return (
      <div className="min-h-screen bg-background">
        {organizationCreateDialog}
        <div className="mx-auto flex w-full max-w-md items-center gap-1 px-4 pt-3 sm:px-0">
          {workspaceOnboardingProgressSteps.map((step, index) => (
            <div
              key={step}
              className={cn(
                'h-1 flex-1 rounded-full',
                index <= onboardingProgressIndex ? 'bg-primary' : 'bg-muted',
              )}
            />
          ))}
        </div>
        <div className="mx-auto flex min-h-screen w-full max-w-6xl flex-col px-4 py-5 sm:px-6 lg:px-8">
          <header className="flex items-center justify-between">
            <HelpinLogo className="justify-start" imageClassName="h-8" />
            <Button type="button" variant="ghost" onClick={() => void signOut()}>
              <Logout01Icon className="h-4 w-4 mr-2" />
              Sign out
            </Button>
          </header>

          <main className="flex flex-1 items-center justify-center py-8">
            <div className={cn('w-full', onboardingContentWidth)}>
              <div className="mb-7 text-center">
                <h1 className="text-balance text-2xl font-semibold tracking-tight sm:text-3xl">
                  {workspaceOnboardingTitle}
                </h1>
                <p className="mx-auto mt-3 max-w-2xl text-sm leading-6 text-muted-foreground">
                  {workspaceOnboardingDescriptionLines.map((line) => (
                    <span key={line} className="block">{line}</span>
                  ))}
                </p>
              </div>

              <form onSubmit={handleWorkspaceOnboardingSubmit} className="rounded-xl bg-background/95">
                {renderWorkspaceOnboardingBody(true)}
                <div className="mt-2 flex items-center justify-end gap-2">
                  {renderWorkspaceOnboardingFooter(true)}
                </div>
              </form>
            </div>
          </main>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-background">
      <div className="max-w-6xl mx-auto px-4 py-12">
        {organizationCreateDialog}

        <div className="flex items-center justify-between mb-8">
          <div>
            <h1 className="text-3xl font-bold">Workspaces</h1>
            <p className="text-muted-foreground mt-1">Select a workspace or create a new one</p>
          </div>
          <div className="flex flex-col-reverse items-stretch gap-2 sm:flex-row sm:items-center">
            <Dialog open={dialogOpen} onOpenChange={handleWorkspaceDialogChange}>
              <DialogTrigger asChild>
                <Button disabled={!currentOrganization} onClick={openWorkspaceDialog}>
                  <PlusSignIcon className="h-4 w-4 mr-2" />
                  Create workspace
                </Button>
              </DialogTrigger>
              <DialogContent className={cn(workspaceStep === 'use-cases' ? 'sm:max-w-3xl' : 'sm:max-w-2xl')}>
                <form onSubmit={handleWorkspaceOnboardingSubmit}>
                  <DialogHeader>
                    <DialogTitle>{workspaceOnboardingTitle}</DialogTitle>
                    <DialogDescription>
                      {workspaceOnboardingDescriptionLines.map((line) => (
                        <span key={line} className="block">{line}</span>
                      ))}
                    </DialogDescription>
                  </DialogHeader>
                  {renderWorkspaceOnboardingBody()}
                  <DialogFooter>{renderWorkspaceOnboardingFooter()}</DialogFooter>
                </form>
              </DialogContent>
            </Dialog>
            <Button type="button" variant="outline" onClick={() => void signOut()}>
              <Logout01Icon className="h-4 w-4 mr-2" />
              Sign out
            </Button>
          </div>
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
              <PlusSignIcon className="h-4 w-4 mr-2" />
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
                  <h2 className="text-sm font-medium text-muted-foreground">{org.name}</h2>
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
