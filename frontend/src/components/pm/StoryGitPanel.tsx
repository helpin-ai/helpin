import { useCallback, useEffect, useState } from 'react';
import { ExternalLink, GitBranch, GitPullRequest, Plus } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { gitService } from '@/lib/services/gitService';
import type { GitIntegration, StoryGitLink } from '@/lib/pmTypes';

const PR_STATUS_COLORS: Record<string, string> = {
  open: 'bg-green-500/15 text-green-700 border-green-500/30',
  merged: 'bg-purple-500/15 text-purple-700 border-purple-500/30',
  closed: 'bg-red-500/15 text-red-700 border-red-500/30',
};

export function StoryGitPanel({
  storyId,
  workspaceId,
}: {
  storyId: string;
  workspaceId: string;
}) {
  const [links, setLinks] = useState<StoryGitLink[]>([]);
  const [integrations, setIntegrations] = useState<GitIntegration[]>([]);
  const [loading, setLoading] = useState(true);
  const [dialogOpen, setDialogOpen] = useState(false);

  const [selectedIntegration, setSelectedIntegration] = useState('');
  const [repo, setRepo] = useState('');
  const [branchName, setBranchName] = useState('');
  const [creating, setCreating] = useState(false);

  const loadLinks = useCallback(async () => {
    const res = await gitService.getStoryGitLinks(workspaceId, storyId);
    setLinks(res.data ?? []);
    setLoading(false);
  }, [workspaceId, storyId]);

  useEffect(() => {
    loadLinks();
  }, [loadLinks]);

  useEffect(() => {
    if (dialogOpen && integrations.length === 0) {
      gitService.listIntegrations(workspaceId).then((res) => {
        const list = res.data ?? [];
        setIntegrations(list);
        if (list.length === 1) setSelectedIntegration(list[0].id);
      });
    }
  }, [dialogOpen, workspaceId, integrations.length]);

  const handleCreateBranch = async () => {
    if (!selectedIntegration || !repo.trim() || !branchName.trim()) return;
    setCreating(true);
    const { data } = await gitService.createBranch(workspaceId, storyId, {
      integration_id: selectedIntegration,
      repo: repo.trim(),
      branch_name: branchName.trim(),
    });
    setCreating(false);
    if (data) {
      setLinks((prev) => [...prev, data]);
      setDialogOpen(false);
      setRepo('');
      setBranchName('');
    }
  };

  if (loading) return null;

  return (
    <div className="mt-4">
      <div className="flex items-center justify-between mb-2">
        <h4 className="text-xs font-semibold text-muted-foreground uppercase tracking-wide flex items-center gap-1.5">
          <GitBranch className="h-3 w-3" />
          Git
        </h4>
        <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
          <DialogTrigger asChild>
            <Button variant="ghost" size="sm" className="h-6 px-2 text-xs">
              <Plus className="h-3 w-3 mr-1" />
              Create Branch
            </Button>
          </DialogTrigger>
          <DialogContent className="sm:max-w-md">
            <DialogHeader>
              <DialogTitle>Create Branch</DialogTitle>
            </DialogHeader>
            <div className="space-y-3 pt-2">
              <div>
                <label className="text-xs font-medium text-muted-foreground mb-1 block">
                  Integration
                </label>
                <Select value={selectedIntegration} onValueChange={setSelectedIntegration}>
                  <SelectTrigger className="h-8 text-sm">
                    <SelectValue placeholder="Select integration" />
                  </SelectTrigger>
                  <SelectContent>
                    {integrations.map((i) => (
                      <SelectItem key={i.id} value={i.id}>
                        {i.display_name} ({i.provider})
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div>
                <label className="text-xs font-medium text-muted-foreground mb-1 block">
                  Repository
                </label>
                <Input
                  className="h-8 text-sm"
                  placeholder="org/repo-name"
                  value={repo}
                  onChange={(e) => setRepo(e.target.value)}
                />
              </div>
              <div>
                <label className="text-xs font-medium text-muted-foreground mb-1 block">
                  Branch Name
                </label>
                <Input
                  className="h-8 text-sm"
                  placeholder="feature/my-branch"
                  value={branchName}
                  onChange={(e) => setBranchName(e.target.value)}
                />
              </div>
              <div className="flex justify-end">
                <Button
                  size="sm"
                  disabled={creating || !selectedIntegration || !repo.trim() || !branchName.trim()}
                  onClick={handleCreateBranch}
                >
                  {creating ? 'Creating...' : 'Create'}
                </Button>
              </div>
            </div>
          </DialogContent>
        </Dialog>
      </div>

      {links.length === 0 ? (
        <p className="text-xs text-muted-foreground">No git connections</p>
      ) : (
        <div className="space-y-2">
          {links.map((link) => (
            <div
              key={link.id}
              className="rounded-md border border-border/60 px-3 py-2 text-xs space-y-1"
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
                    className="text-primary hover:underline inline-flex items-center gap-1"
                  >
                    #{link.pr_number}
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
                <div className="text-muted-foreground font-mono">
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
