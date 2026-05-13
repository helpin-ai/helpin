import { useQuery } from '@tanstack/react-query';
import { formatDistanceToNow, parseISO } from 'date-fns';
import { LinkSquare01Icon, GitBranchIcon, GitCommitIcon, GitPullRequestIcon, Loading01Icon } from '@/lib/icons';
import { Badge } from '@/components/ui/badge';
import { CollapsibleSection } from '@/components/ui/collapsible-section';
import { gitService } from '@/lib/services/gitService';
import { queryKeys } from '@/lib/queryKeys';
import type { TaskDeliveryTarget, TaskGitLink } from '@/lib/pmTypes';

const PR_STATUS_COLORS: Record<string, string> = {
  open: 'bg-green-100 text-green-700 border-green-500/30 dark:bg-green-900/30 dark:text-green-400',
  merged: 'bg-sky-100 text-sky-700 border-sky-500/30 dark:bg-sky-900/30 dark:text-sky-400',
  closed: 'bg-zinc-100 text-zinc-600 border-zinc-500/30 dark:bg-zinc-800/30 dark:text-zinc-400',
};

function gitLinkKind(link: TaskGitLink) {
  if (link.pr_url || link.pr_number) return 'Pull request';
  if (link.commit_sha) return 'Commit';
  return 'Branch';
}

function gitLinkIcon(link: TaskGitLink) {
  if (link.pr_url || link.pr_number) return GitPullRequestIcon;
  if (link.commit_sha) return GitCommitIcon;
  return GitBranchIcon;
}

function shortSha(sha?: string) {
  return sha ? sha.slice(0, 7) : '';
}

function branchFlow(link: TaskGitLink, deliveryTarget?: TaskDeliveryTarget | null) {
  const workingBranch = link.branch ?? deliveryTarget?.working_branch;
  const baseBranch = deliveryTarget?.base_branch;
  return { workingBranch, baseBranch };
}

export function TaskGitPanel({
  taskId,
  workspaceId,
}: {
  taskId: string;
  workspaceId: string;
}) {
  const { data: links = [], isPending } = useQuery({
    queryKey: queryKeys.git.taskLinks(workspaceId, taskId),
    queryFn: async () => {
      const res = await gitService.getTaskGitLinks(workspaceId, taskId);
      return res.data ?? [];
    },
  });
  const { data: deliveryTarget } = useQuery({
    queryKey: queryKeys.git.taskDeliveryTarget(workspaceId, taskId),
    queryFn: async () => {
      const res = await gitService.getTaskDeliveryTarget(workspaceId, taskId);
      return res.data ?? null;
    },
  });

  if (isPending) {
    return (
      <div className="mt-6 flex items-center gap-2 py-4 text-xs text-muted-foreground">
        <Loading01Icon className="h-3.5 w-3.5 animate-spin" />
        Loading development history...
      </div>
    );
  }

  if (links.length === 0) {
    return null;
  }

  return (
    <div className="mt-6">
      <CollapsibleSection
        title="Development History"
        icon={GitBranchIcon}
        count={links.length}
        defaultOpen
      >
        <div className="overflow-hidden rounded-md border border-border/60 bg-card">
          {links.map((link, index) => {
            const kind = gitLinkKind(link);
            const Icon = gitLinkIcon(link);
            const updatedLabel = formatDistanceToNow(parseISO(link.updated_at), { addSuffix: true });
            const { workingBranch, baseBranch } = branchFlow(link, deliveryTarget);
            const commitSha = link.commit_sha ?? deliveryTarget?.last_commit_sha;
            const commitLabel = shortSha(commitSha);
            return (
              <div key={link.id} className="group relative flex gap-3 px-3 py-3 text-xs hover:bg-muted/30">
                <div className="relative flex shrink-0 justify-center">
                  {index < links.length - 1 ? (
                    <span className="absolute left-1/2 top-7 h-[calc(100%+0.75rem)] w-px -translate-x-1/2 bg-border/60" />
                  ) : null}
                  <span className="relative z-[1] flex h-7 w-7 items-center justify-center rounded-full border border-border/70 bg-card text-muted-foreground shadow-sm group-hover:border-primary/30 group-hover:text-primary">
                    <Icon className="h-3.5 w-3.5" />
                  </span>
                </div>

                <div className="min-w-0 flex-1 space-y-1">
                  <div className="flex min-w-0 items-center gap-2">
                    <span className="font-medium text-foreground">{kind}</span>
                    {link.pr_status ? (
                      <Badge
                        variant="outline"
                        className={`h-5 rounded-full px-1.5 py-0 text-[10px] uppercase ${PR_STATUS_COLORS[link.pr_status] ?? ''}`}
                      >
                        {link.pr_status}
                      </Badge>
                    ) : null}
                    <span className="ml-auto shrink-0 text-[11px] text-muted-foreground">{updatedLabel}</span>
                  </div>

                  {link.pr_url ? (
                    <a
                      href={link.pr_url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="inline-flex max-w-full items-center gap-1 font-medium text-foreground transition-colors hover:text-primary"
                    >
                      {link.pr_number ? <span className="shrink-0">#{link.pr_number}</span> : null}
                      <span className="truncate">{link.pr_title ?? 'Pull request'}</span>
                      <LinkSquare01Icon className="h-2.5 w-2.5 shrink-0 text-muted-foreground" />
                    </a>
                  ) : null}

                  <div className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 text-muted-foreground">
                    <a
                      href={`https://github.com/${link.repo}`}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="inline-flex min-w-0 items-center gap-1 transition-colors hover:text-primary"
                    >
                      <span className="truncate">{link.repo}</span>
                      <LinkSquare01Icon className="h-2.5 w-2.5 shrink-0" />
                    </a>
                    {workingBranch ? (
                      <>
                        <span className="text-border">·</span>
                        <a
                          href={`https://github.com/${link.repo}/tree/${workingBranch}`}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="inline-flex min-w-0 items-center gap-1 transition-colors hover:text-primary"
                        >
                          <span className="truncate font-mono text-[11px]">{workingBranch}</span>
                          <LinkSquare01Icon className="h-2.5 w-2.5 shrink-0" />
                        </a>
                        {baseBranch ? (
                          <>
                            <span className="text-muted-foreground/60">→</span>
                            <span className="font-mono text-[11px] text-muted-foreground">{baseBranch}</span>
                          </>
                        ) : null}
                      </>
                    ) : null}
                  </div>

                  {commitSha ? (
                    <a
                      href={`https://github.com/${link.repo}/commit/${commitSha}`}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="inline-flex max-w-full items-center gap-1 text-[11px] text-muted-foreground transition-colors hover:text-primary"
                    >
                      <GitCommitIcon className="h-3 w-3 shrink-0" />
                      <code className="font-mono">{commitLabel}</code>
                      <LinkSquare01Icon className="h-2.5 w-2.5 shrink-0" />
                    </a>
                  ) : null}
                </div>
              </div>
            );
          })}
        </div>
      </CollapsibleSection>
    </div>
  );
}
