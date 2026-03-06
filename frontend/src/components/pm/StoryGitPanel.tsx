import { useCallback, useEffect, useState } from 'react';
import { ExternalLink, GitBranch, GitPullRequest, Loader2 } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Separator } from '@/components/ui/separator';
import { gitService } from '@/lib/services/gitService';
import type { StoryGitLink } from '@/lib/pmTypes';

const PR_STATUS_COLORS: Record<string, string> = {
  open: 'bg-green-100 text-green-700 border-green-500/30 dark:bg-green-900/30 dark:text-green-400',
  merged: 'bg-sky-100 text-sky-700 border-sky-500/30 dark:bg-sky-900/30 dark:text-sky-400',
  closed: 'bg-zinc-100 text-zinc-600 border-zinc-500/30 dark:bg-zinc-800/30 dark:text-zinc-400',
};

export function StoryGitPanel({
  storyId,
  workspaceId,
}: {
  storyId: string;
  workspaceId: string;
}) {
  const [links, setLinks] = useState<StoryGitLink[]>([]);
  const [loading, setLoading] = useState(true);

  const loadLinks = useCallback(async () => {
    const res = await gitService.getStoryGitLinks(workspaceId, storyId);
    setLinks(res.data ?? []);
    setLoading(false);
  }, [workspaceId, storyId]);

  useEffect(() => {
    loadLinks();
  }, [loadLinks]);

  if (loading) {
    return (
      <div className="mt-6 flex items-center gap-2 py-4 text-xs text-muted-foreground">
        <Loader2 className="h-3.5 w-3.5 animate-spin" />
        Loading development history...
      </div>
    );
  }

  return (
    <div className="mt-6 rounded-lg border border-border/70 bg-card">
      {/* Header */}
      <div className="flex items-center gap-2 px-4 py-3">
        <GitBranch className="h-4 w-4 text-muted-foreground" />
        <h3 className="text-sm font-semibold">Development History</h3>
        {links.length > 0 && (
          <span className="rounded-full bg-muted px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground">
            {links.length}
          </span>
        )}
      </div>

      <Separator />

      {links.length === 0 ? (
        <div className="px-4 py-6 text-center">
          <GitBranch className="mx-auto h-8 w-8 text-muted-foreground/30" />
          <p className="mt-2 text-xs text-muted-foreground">
            No branch, commit, or PR activity has been recorded yet.
          </p>
        </div>
      ) : (
        <div className="divide-y divide-border/40">
          {links.map((link) => (
            <div key={link.id} className="space-y-1.5 px-4 py-3 text-xs">
              <div className="font-medium">{link.repo}</div>
              {link.branch && (
                <div className="flex items-center gap-1.5 text-muted-foreground">
                  <GitBranch className="h-3 w-3 shrink-0" />
                  <span className="truncate font-mono">{link.branch}</span>
                </div>
              )}
              {link.pr_url && (
                <div className="flex items-center gap-1.5">
                  <GitPullRequest className="h-3 w-3 shrink-0 text-muted-foreground" />
                  <a
                    href={link.pr_url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="inline-flex items-center gap-1 text-primary hover:underline"
                  >
                    <span>#{link.pr_number}</span>
                    <span className="truncate">{link.pr_title ?? 'Pull request'}</span>
                    <ExternalLink className="h-2.5 w-2.5 shrink-0" />
                  </a>
                  {link.pr_status && (
                    <Badge
                      variant="outline"
                      className={`px-1.5 py-0 text-[10px] ${PR_STATUS_COLORS[link.pr_status] ?? ''}`}
                    >
                      {link.pr_status}
                    </Badge>
                  )}
                </div>
              )}
              {link.commit_sha && (
                <div className="font-mono text-muted-foreground">
                  {link.commit_sha.slice(0, 7)}
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
