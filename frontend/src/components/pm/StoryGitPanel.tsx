import { useCallback, useEffect, useState } from 'react';
import { ExternalLink, GitBranch, GitPullRequest } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { gitService } from '@/lib/services/gitService';
import type { StoryGitLink } from '@/lib/pmTypes';

const PR_STATUS_COLORS: Record<string, string> = {
  open: 'bg-green-500/15 text-green-700 border-green-500/30',
  merged: 'bg-sky-500/15 text-sky-700 border-sky-500/30',
  closed: 'bg-zinc-500/15 text-zinc-700 border-zinc-500/30',
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
    return null;
  }

  return (
    <div className="mt-6">
      <div className="mb-2">
        <h4 className="flex items-center gap-1.5 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
          <GitBranch className="h-3 w-3" />
          Development History
        </h4>
      </div>

      {links.length === 0 ? (
        <p className="text-xs text-muted-foreground">No branch, commit, or PR activity has been recorded yet.</p>
      ) : (
        <div className="space-y-2">
          {links.map((link) => (
            <div
              key={link.id}
              className="space-y-1 rounded-md border border-border/60 px-3 py-2 text-xs"
            >
              <div className="font-medium">{link.repo}</div>
              {link.branch && (
                <div className="flex items-center gap-1 text-muted-foreground">
                  <GitBranch className="h-3 w-3" />
                  <span className="font-mono">{link.branch}</span>
                </div>
              )}
              {link.pr_url && (
                <div className="flex items-center gap-1.5">
                  <GitPullRequest className="h-3 w-3 text-muted-foreground" />
                  <a
                    href={link.pr_url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="inline-flex items-center gap-1 text-primary hover:underline"
                  >
                    <span>#{link.pr_number}</span>
                    <span>{link.pr_title ?? 'Pull request'}</span>
                    <ExternalLink className="h-2.5 w-2.5" />
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
