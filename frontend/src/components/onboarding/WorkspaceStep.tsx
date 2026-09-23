import { useEffect, useId, useRef, useState, type FormEvent } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/design-system/quiet-dropdown-select';
import { useCreateOrganization, useOrganizations } from '@/hooks/queries';
import { Loading01Icon } from '@/lib/icons';
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
          autoFocus
          required
        />
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
      <ul className="divide-y divide-border border-y border-border">
        {ONBOARDING_USE_CASE_OPTIONS.map((option) => {
          const position = selected.indexOf(option.value);
          const isSelected = position >= 0;
          return (
            <li key={option.value}>
              <button
                type="button"
                aria-pressed={isSelected}
                onClick={() => onToggle(option.value)}
                className="flex w-full items-start gap-3 px-1 py-3 text-left transition-colors hover:bg-muted/60"
              >
                <span
                  aria-hidden="true"
                  className={cn(
                    'mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-full border text-[11px] font-semibold tabular-nums',
                    isSelected ? 'border-foreground bg-foreground text-background' : 'border-input text-transparent',
                  )}
                >
                  {isSelected ? position + 1 : ''}
                </span>
                <span className="min-w-0 flex-1">
                  <span className="block text-[13.5px] font-semibold leading-5 tracking-[-0.008em]">{option.label}</span>
                  <span className="mt-0.5 block text-[12.5px] leading-5 text-muted-foreground">{option.description}</span>
                  {isSelected && <span className="sr-only">Selected, priority {position + 1}</span>}
                </span>
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
