import { useCallback, useEffect, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { gitService } from '@/lib/services/gitService';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import type { TeamRepoDefault, WorkspaceTeam } from '@/lib/types';
import type { GitRepository } from '@/lib/pmTypes';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { GitBranchIcon, GitPullRequestIcon } from '@/lib/icons';
import { LINEAR_CARD_CLASS } from './settingsConstants';

export function ProjectDeliveryTab({ workspaceId, editable, teams = [], teamRepoDefaults = [] }: {
  workspaceId: string;
  editable: boolean;
  teams?: WorkspaceTeam[];
  teamRepoDefaults?: TeamRepoDefault[];
}) {
  const navigate = useNavigate();
  const workspaceSlug = useWorkspaceStore((state) => state.currentWorkspace?.slug);
  const [repositories, setRepositories] = useState<GitRepository[]>([]);

  const loadRepositories = useCallback(async () => {
    const { data } = await gitService.listRepositories(workspaceId);
    setRepositories(data ?? []);
  }, [workspaceId]);

  useEffect(() => {
    void loadRepositories();
  }, [loadRepositories]);

  const selectedRepositories = repositories.filter((repo) => repo.selected && repo.active && !repo.archived);

  return (
    <div className="space-y-6">
      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader>
          <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
            <div>
              <div className="flex items-center gap-2">
                <GitPullRequestIcon className="h-4 w-4 text-muted-foreground" />
                <CardTitle className="text-base">Repository source</CardTitle>
              </div>
              <CardDescription className="mt-1.5">
                Project delivery uses repositories from the shared workspace catalog.
              </CardDescription>
            </div>
            {workspaceSlug ? (
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => void navigate({ to: '/w/$slug/settings/repositories', params: { slug: workspaceSlug } })}
              >
                {selectedRepositories.length > 0 ? 'Manage repositories' : 'Add repositories'}
              </Button>
            ) : null}
          </div>
        </CardHeader>
        <CardContent>
          {selectedRepositories.length > 0 ? (
            <div className="flex flex-wrap gap-2">
              {selectedRepositories.slice(0, 8).map((repo) => (
                <Badge key={repo.id} variant="outline" className="max-w-full truncate">
                  {repo.full_name}
                </Badge>
              ))}
              {selectedRepositories.length > 8 ? (
                <Badge variant="secondary">+{selectedRepositories.length - 8} more</Badge>
              ) : null}
            </div>
          ) : (
            <div className="rounded-lg border border-dashed border-border px-4 py-6 text-center text-sm text-muted-foreground">
              No repositories are available for delivery yet. Add repositories to this workspace before configuring team defaults.
            </div>
          )}
        </CardContent>
      </Card>

      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader>
          <div className="flex items-center gap-2">
            <GitBranchIcon className="h-4 w-4 text-muted-foreground" />
            <CardTitle className="text-base">Team delivery defaults</CardTitle>
          </div>
          <CardDescription>
            Each team can pin a default repository, base branch, and branch template for new tasks. Task-level overrides still take precedence.
          </CardDescription>
        </CardHeader>
        <CardContent>
          {teams.length === 0 ? (
            <div className="rounded-lg border border-dashed border-border px-4 py-6 text-center text-sm text-muted-foreground">
              No teams in this workspace yet. Create a team in Workspace Settings - Teams to assign delivery defaults.
            </div>
          ) : (
            <ul className="divide-y divide-border/60 rounded-lg border border-border/60">
              {teams.map((team) => {
                const teamDefault = teamRepoDefaults.find((entry) => entry.team_id === team.id);
                const repo = teamDefault
                  ? repositories.find((entry) => entry.id === teamDefault.repository_id)
                  : null;
                const meta = teamDefault
                  ? `${repo?.full_name ?? 'Repository selected'} · base ${teamDefault.base_branch}`
                  : 'Not configured';
                return (
                  <li key={team.id} className="flex items-center justify-between gap-3 px-4 py-3">
                    <div className="min-w-0">
                      <p className="truncate text-sm font-medium">{team.name}</p>
                      <p className="truncate text-xs text-muted-foreground">{meta}</p>
                    </div>
                    <div className="flex shrink-0 items-center gap-2">
                      {teamDefault ? (
                        <Badge variant="outline" className="border-emerald-500/40 bg-emerald-500/10 text-[10px] uppercase tracking-wide text-emerald-700 dark:text-emerald-400">
                          Configured
                        </Badge>
                      ) : null}
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        disabled={!editable || !workspaceSlug}
                        onClick={() => {
                          if (!workspaceSlug) return;
                          void navigate({
                            to: '/w/$slug/settings/teams',
                            params: { slug: workspaceSlug },
                            search: { team: team.id, section: 'delivery' },
                          });
                        }}
                      >
                        {teamDefault ? 'Edit' : 'Configure'}
                      </Button>
                    </div>
                  </li>
                );
              })}
            </ul>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
