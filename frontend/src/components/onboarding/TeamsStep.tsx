import { useId, useMemo, useState, type FormEvent } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { Cancel01Icon, Loading01Icon, PlusSignIcon } from '@/lib/icons';
import { settingsService } from '@/lib/services/settingsService';
import {
  buildPresetFieldVisibility,
  slugifyTeamHandle,
  TEAM_TYPE_PRESETS,
  WORKSPACE_TEAM_SUGGESTIONS,
  type TeamType,
} from '@/lib/teamPresets';
import { cn } from '@/lib/utils';
import { OnboardingActions, OnboardingTextButton } from './OnboardingShell';

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

type TeamsStepProps = {
  workspaceId: string;
  /** Handles of teams already created in this session, so going back never duplicates them. */
  createdHandles: string[];
  onDone: (created: { id: string; handle: string }[]) => void;
};

/** Creates the selected teams with their presets (estimates, field visibility). */
export function TeamsStep({ workspaceId, createdHandles, onDone }: TeamsStepProps) {
  const id = useId();
  const queryClient = useQueryClient();
  const [drafts, setDrafts] = useState<TeamDraft[]>(createInitialTeamDrafts);
  const [creating, setCreating] = useState(false);

  const selected = useMemo(() => drafts.filter((team) => team.selected && team.name.trim()), [drafts]);
  const hasUnnamedSelection = drafts.some((team) => team.selected && !team.name.trim());

  const update = (teamId: string, changes: Partial<TeamDraft>) => {
    setDrafts((current) => current.map((team) => (team.id === teamId ? { ...team, ...changes } : team)));
  };

  const addCustomTeam = () => {
    setDrafts((current) => [
      ...current,
      { id: `custom-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`, name: '', teamType: 'custom', selected: true, isCustom: true },
    ]);
  };

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    const pending = selected
      .map((team) => ({ name: team.name.trim(), handle: slugifyTeamHandle(team.name), teamType: team.teamType }))
      .filter((team) => !createdHandles.includes(team.handle));
    if (pending.length === 0) {
      onDone([]);
      return;
    }
    setCreating(true);
    const results = await Promise.allSettled(
      pending.map(async (team) => {
        const teamRes = await settingsService.createTeam({
          workspace_id: workspaceId,
          name: team.name,
          handle: team.handle,
          team_type: team.teamType,
          default_task_type: TEAM_TYPE_PRESETS[team.teamType].defaultTaskType,
        });
        if (!teamRes.data || teamRes.error) {
          throw new Error(teamRes.error ?? `Failed to create ${team.name}`);
        }
        const [estimateRes, visibilityRes] = await Promise.all([
          settingsService.updateTeamEstimateSettings(workspaceId, teamRes.data.id, TEAM_TYPE_PRESETS[team.teamType].estimate),
          settingsService.updateTeamFieldVisibility(workspaceId, teamRes.data.id, buildPresetFieldVisibility(team.teamType)),
        ]);
        if (estimateRes.error || visibilityRes.error) {
          throw new Error(estimateRes.error ?? visibilityRes.error ?? `Failed to finish setup for ${team.name}`);
        }
        return { id: teamRes.data.id, handle: team.handle };
      }),
    );
    setCreating(false);
    const created = results
      .filter((result): result is PromiseFulfilledResult<{ id: string; handle: string }> => result.status === 'fulfilled')
      .map((result) => result.value);
    const failed = results.length - created.length;
    void queryClient.invalidateQueries({ queryKey: ['workspaces'] });
    if (failed > 0) {
      toast.warning(`Added ${created.length} team${created.length === 1 ? '' : 's'}, but ${failed} failed. You can add teams later in Settings → Teams.`);
    }
    onDone(created);
  };

  return (
    <form onSubmit={(event) => void submit(event)} className="space-y-7">
      <ul className="divide-y divide-border border-y border-border">
        {drafts.map((team) => {
          const checkboxId = `${id}-${team.id}`;
          const alreadyCreated = !team.isCustom && createdHandles.includes(slugifyTeamHandle(team.name));
          return (
            <li key={team.id} className="flex min-h-12 items-center gap-3 px-1 py-2">
              <Checkbox
                id={checkboxId}
                checked={team.selected || alreadyCreated}
                disabled={alreadyCreated}
                aria-label={team.isCustom ? 'Include this team' : undefined}
                onCheckedChange={(checked) => update(team.id, { selected: checked === true })}
              />
              {team.isCustom ? (
                <Input
                  value={team.name}
                  onChange={(event) => update(team.id, { name: event.target.value })}
                  placeholder="Team name"
                  aria-label="Custom team name"
                  className="min-w-0 flex-1"
                  autoFocus
                />
              ) : (
                <Label htmlFor={checkboxId} className="min-w-0 flex-1 cursor-pointer truncate text-sm font-medium">
                  {team.name}
                  {alreadyCreated && <span className="ml-2 font-normal text-muted-foreground">Created</span>}
                </Label>
              )}
              {team.selected && !alreadyCreated && (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <div className="ml-auto flex shrink-0 items-center gap-1.5">
                      <Checkbox
                        id={`${checkboxId}-eng`}
                        checked={team.teamType === 'engineering'}
                        onCheckedChange={(checked) => update(team.id, { teamType: checked ? 'engineering' : 'custom' })}
                        className="h-3.5 w-3.5"
                      />
                      <Label htmlFor={`${checkboxId}-eng`} className="cursor-pointer whitespace-nowrap text-xs font-normal text-muted-foreground">
                        Eng / dev
                      </Label>
                    </div>
                  </TooltipTrigger>
                  <TooltipContent side="top" className="max-w-[260px] text-xs">
                    Engineering teams get development workflows, GitHub integration, and preset fields. Other teams start with a simpler setup.
                  </TooltipContent>
                </Tooltip>
              )}
              {team.isCustom && (
                <button
                  type="button"
                  aria-label="Remove team"
                  className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground"
                  onClick={() => setDrafts((current) => current.filter((draft) => draft.id !== team.id))}
                >
                  <Cancel01Icon className="h-3.5 w-3.5" aria-hidden="true" />
                </button>
              )}
            </li>
          );
        })}
      </ul>
      <button
        type="button"
        onClick={addCustomTeam}
        className={cn('flex min-h-11 items-center gap-2 text-sm text-muted-foreground underline-offset-4 hover:text-foreground hover:underline')}
      >
        <PlusSignIcon className="h-3.5 w-3.5" aria-hidden="true" />
        Add a team
      </button>
      <OnboardingActions>
        <OnboardingTextButton onClick={() => onDone([])} disabled={creating}>Skip</OnboardingTextButton>
        <Button type="submit" className="w-full sm:w-auto sm:min-w-32" disabled={creating || hasUnnamedSelection}>
          {creating && <Loading01Icon className="mr-2 h-4 w-4 animate-spin" aria-hidden="true" />}
          {creating
            ? 'Creating teams…'
            : selected.length > 0
              ? `Continue with ${selected.length} team${selected.length === 1 ? '' : 's'}`
              : 'Continue'}
        </Button>
      </OnboardingActions>
    </form>
  );
}
