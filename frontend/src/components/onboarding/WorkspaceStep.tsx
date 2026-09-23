import { useEffect, useId, useRef, useState, type FormEvent } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/design-system/quiet-dropdown-select';
import { useCreateOrganization, useOrganizations } from '@/hooks/queries';
import {
  BookOpen01Icon,
  DollarCircleIcon,
  File01Icon,
  HeadphonesIcon,
  Loading01Icon,
  SourceCodeIcon,
} from '@/lib/icons';
import { workspacesService } from '@/lib/services/workspacesService';
import { generateWorkspaceSlug } from '@/lib/slugUtils';
import type { Workspace } from '@/lib/types';
import { validateWorkspaceOnboardingDetails } from '@/lib/workspaceOnboardingDetails';
import { workspaceDefaultsForCreation } from '@/lib/workspaceOnboardingDefaults';
import { toggleOrderedSelection } from '@/lib/workspaceOnboardingFlow';
import { shouldShowWorkspaceOnboardingOrganizationSelector } from '@/lib/workspaceOnboardingMode';
import {
  ONBOARDING_USE_CASE_OPTIONS,
  mapOnboardingUseCasesToSetupGoals,
  trackWorkspaceOnboardingUseCases,
  type WorkspaceOnboardingUseCase,
} from '@/lib/workspaceOnboardingUseCases';
import { cn } from '@/lib/utils';
import { useAuthStore } from '@/stores/authStore';
import { useOrganizationStore } from '@/stores/organizationStore';
import { OnboardingActions } from './OnboardingShell';

const NEW_ORGANIZATION = '__new_organization__';

const goalIcons = {
  product_engineering: SourceCodeIcon,
  customer_support: HeadphonesIcon,
  help_center_docs: BookOpen01Icon,
  internal_docs: File01Icon,
  sales_crm: DollarCircleIcon,
} satisfies Record<WorkspaceOnboardingUseCase, typeof SourceCodeIcon>;

const goalIconStyles = {
  product_engineering: 'border-sky-200 bg-sky-50 text-sky-700 dark:border-sky-900 dark:bg-sky-950 dark:text-sky-300',
  customer_support: 'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-900 dark:bg-amber-950 dark:text-amber-300',
  help_center_docs: 'border-violet-200 bg-violet-50 text-violet-700 dark:border-violet-900 dark:bg-violet-950 dark:text-violet-300',
  internal_docs: 'border-rose-200 bg-rose-50 text-rose-700 dark:border-rose-900 dark:bg-rose-950 dark:text-rose-300',
  sales_crm: 'border-cyan-200 bg-cyan-50 text-cyan-700 dark:border-cyan-900 dark:bg-cyan-950 dark:text-cyan-300',
} satisfies Record<WorkspaceOnboardingUseCase, string>;

function workspaceKeyFromName(name: string) {
  return (name.replace(/[^a-zA-Z]/g, '').slice(0, 3) || 'WS').toUpperCase();
}

type WorkspaceStepProps = {
  /** Prefill the name from a company email domain (first workspace only). */
  useEmailDefaults: boolean;
  onCreated: (workspace: Workspace) => void;
};

/**
 * Names the workspace and records what the person wants to set up, then
 * creates it. Website and company context are optional and come later.
 */
export function WorkspaceStep({ useEmailDefaults, onCreated }: WorkspaceStepProps) {
  const id = useId();
  const queryClient = useQueryClient();
  const user = useAuthStore((state) => state.user);
  const { data: organizations = [], isLoading: organizationsLoading } = useOrganizations();
  const createOrganization = useCreateOrganization();
  const { currentOrganization, setCurrentOrganization } = useOrganizationStore();
  const [name, setName] = useState(() => workspaceDefaultsForCreation({ email: user?.email, useEmailDefaults }).name);
  const [goals, setGoals] = useState<WorkspaceOnboardingUseCase[]>([]);
  const [organizationChoice, setOrganizationChoice] = useState<string | null>(null);
  const [organizationName, setOrganizationName] = useState(() => {
    const first = user?.full_name?.split(' ')[0];
    return first ? `${first}'s Organization` : '';
  });
  const autoCreateAttempted = useRef(false);
  const [autoCreateFailed, setAutoCreateFailed] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);

  // Signup creates an organization; this covers accounts where it didn't.
  useEffect(() => {
    if (organizationsLoading || organizations.length > 0 || autoCreateAttempted.current || !user) return;
    autoCreateAttempted.current = true;
    const first = user.full_name?.split(' ')[0] || 'My';
    const orgName = `${first}'s Organization`;
    createOrganization.mutateAsync({ name: orgName, slug: generateWorkspaceSlug(orgName) })
      .then((organization) => setCurrentOrganization(organization))
      .catch(() => setAutoCreateFailed(true));
  }, [createOrganization, organizations.length, organizationsLoading, setCurrentOrganization, user]);

  const selectedOrganization = organizationChoice && organizationChoice !== NEW_ORGANIZATION
    ? organizations.find((organization) => organization.id === organizationChoice) ?? null
    : organizationChoice === NEW_ORGANIZATION
      ? null
      : currentOrganization ?? organizations[0] ?? null;
  const needsOrganizationName = organizationChoice === NEW_ORGANIZATION || (autoCreateFailed && organizations.length === 0);
  // Without any organization, one is being created automatically (or is about to be).
  const preparing = organizationsLoading || (organizations.length === 0 && !autoCreateFailed && !selectedOrganization);
  const showOrganizationSelector = shouldShowWorkspaceOnboardingOrganizationSelector({ organizationCount: organizations.length });

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    const slug = generateWorkspaceSlug(name);
    const validationError = validateWorkspaceOnboardingDetails({
      hasOrganization: Boolean(selectedOrganization) || (needsOrganizationName && organizationName.trim().length > 0),
      name,
      slug,
      goalCount: goals.length,
    });
    if (validationError) {
      setError(validationError);
      return;
    }
    setError(null);
    setCreating(true);
    try {
      let organization = selectedOrganization;
      if (needsOrganizationName) {
        const orgName = organizationName.trim();
        organization = await createOrganization.mutateAsync({ name: orgName, slug: generateWorkspaceSlug(orgName) });
        setOrganizationChoice(organization.id);
      }
      if (!organization) {
        setError('Enter an organization name to continue.');
        return;
      }
      setCurrentOrganization(organization);
      const { data: workspace, error: createError } = await workspacesService.create({
        name: name.trim(),
        slug,
        workspace_key: workspaceKeyFromName(name),
        organization_id: organization.id,
        timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
        setup_goals: mapOnboardingUseCasesToSetupGoals(goals),
      });
      if (createError || !workspace) {
        setError(createError ?? 'The workspace couldn’t be created. Try again.');
        return;
      }
      trackWorkspaceOnboardingUseCases(goals);
      void queryClient.invalidateQueries({ queryKey: ['workspaces'] });
      onCreated(workspace);
    } catch (caught) {
      setError(caught instanceof Error && caught.message ? caught.message : 'The workspace couldn’t be created. Try again.');
    } finally {
      setCreating(false);
    }
  };

  const errorId = `${id}-error`;

  return (
    <form onSubmit={(event) => void submit(event)} noValidate aria-describedby={error ? errorId : undefined} className="space-y-7">
      {showOrganizationSelector && (
        <div className="space-y-2">
          <Label htmlFor={`${id}-organization`}>Organization</Label>
          <Select
            value={organizationChoice ?? selectedOrganization?.id ?? ''}
            onValueChange={setOrganizationChoice}
          >
            <SelectTrigger id={`${id}-organization`} variant="underline" className="w-full px-0.5">
              <SelectValue placeholder="Select organization" />
            </SelectTrigger>
            <SelectContent>
              {organizations.map((organization) => (
                <SelectItem key={organization.id} value={organization.id}>{organization.name}</SelectItem>
              ))}
              <SelectItem value={NEW_ORGANIZATION}>New organization…</SelectItem>
            </SelectContent>
          </Select>
        </div>
      )}
      {needsOrganizationName && (
        <div className="space-y-2">
          <Label htmlFor={`${id}-organization-name`}>Organization name</Label>
          <Input
            id={`${id}-organization-name`}
            value={organizationName}
            onChange={(event) => setOrganizationName(event.target.value)}
            placeholder="Acme Inc."
            autoComplete="organization"
          />
          {autoCreateFailed && organizations.length === 0 && (
            <p className="text-[12.5px] text-muted-foreground">Workspaces belong to an organization. We’ll create it with your workspace.</p>
          )}
        </div>
      )}
      <div className="space-y-2">
        <Label htmlFor={`${id}-name`}>Workspace name</Label>
        <Input
          id={`${id}-name`}
          value={name}
          onChange={(event) => setName(event.target.value)}
          placeholder="Acme"
          autoComplete="organization"
          aria-describedby={`${id}-name-hint`}
          autoFocus
          required
        />
        <p id={`${id}-name-hint`} className="text-[12.5px] text-muted-foreground">Usually your company or team name.</p>
      </div>
      <GoalPicker selected={goals} onToggle={(goal) => setGoals((current) => toggleOrderedSelection(current, goal))} />
      <p id={errorId} role="alert" className="text-sm text-destructive empty:hidden">{error ?? ''}</p>
      <OnboardingActions>
        <Button type="submit" className="w-full sm:w-auto sm:min-w-40" disabled={creating || preparing}>
          {(creating || preparing) && <Loading01Icon className="mr-2 h-4 w-4 animate-spin" aria-hidden="true" />}
          {preparing ? 'Preparing…' : creating ? 'Creating workspace…' : 'Create workspace'}
        </Button>
      </OnboardingActions>
    </form>
  );
}

/**
 * Multi-select list of goals with no maximum. Selection order is shown as a
 * number because the first goal leads the Setup guide.
 */
export function GoalPicker({
  selected,
  onToggle,
}: {
  selected: WorkspaceOnboardingUseCase[];
  onToggle: (goal: WorkspaceOnboardingUseCase) => void;
}) {
  const id = useId();
  return (
    <fieldset className="space-y-3" aria-describedby={`${id}-hint`}>
      <legend className="text-sm font-medium">What do you want to set up?</legend>
      <p id={`${id}-hint`} className="-mt-1 text-[12.5px] leading-5 text-muted-foreground">
        Select everything you want to set up. Your first choice leads your setup guide.
      </p>
      <ul className="grid gap-3 sm:grid-cols-2">
        {ONBOARDING_USE_CASE_OPTIONS.map((option) => {
          const position = selected.indexOf(option.value);
          const isSelected = position >= 0;
          const Icon = goalIcons[option.value];
          return (
            <li key={option.value} className="flex">
              <button
                type="button"
                aria-pressed={isSelected}
                onClick={() => onToggle(option.value)}
                className={cn(
                  'flex w-full flex-col rounded-lg border p-4 text-left transition-colors',
                  isSelected
                    ? 'border-foreground/60 bg-muted/50'
                    : 'border-border hover:border-foreground/25 hover:bg-muted/40',
                )}
              >
                <span className="flex items-start gap-3">
                  <span aria-hidden="true" className={cn('flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border', goalIconStyles[option.value])}>
                    <Icon className="h-4 w-4" />
                  </span>
                  <span className="min-w-0 flex-1 pt-0.5 text-[13.5px] font-semibold leading-5 tracking-[-0.008em]">{option.label}</span>
                  {/* The number shows priority: the first pick leads the setup guide. */}
                  <span
                    aria-hidden="true"
                    className={cn(
                      'flex h-5 w-5 shrink-0 items-center justify-center rounded-full border text-[11px] font-semibold tabular-nums',
                      isSelected ? 'border-foreground bg-foreground text-background' : 'border-input text-transparent',
                    )}
                  >
                    {isSelected ? position + 1 : ''}
                  </span>
                </span>
                <span className="mt-3 block flex-1 text-[12.5px] leading-5 text-muted-foreground">{option.description}</span>
                <span className="mt-3 block border-t border-border pt-3 text-[11.5px] leading-4 text-muted-foreground/90">
                  {option.replaces}
                </span>
                {isSelected && <span className="sr-only">Selected, priority {position + 1}</span>}
              </button>
            </li>
          );
        })}
      </ul>
      <p className="text-[12px] text-muted-foreground" aria-live="polite">
        {selected.length === 0 ? 'Nothing selected yet.' : `${selected.length} selected`}
      </p>
    </fieldset>
  );
}
