import { useRef, useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { queryKeys } from '@/lib/queryKeys';
import { unwrap } from '@/lib/queryUtils';
import { mcpService } from '@/lib/services/mcpService';
import { setupService } from '@/lib/services/setupService';
import { buildWorkspaceSetupPrompt, workspaceSetupAccessLabels, workspaceSetupConnectionIssue, workspaceSetupPath } from '@/lib/workspaceSetup';

type Props = { workspaceId: string; slug: string };

/** Same handoff in the Setup guide and the signup shell. */
export function WorkspaceSetupAssistant({ workspaceId, slug }: Props) {
  const [open, setOpen] = useState(false);
  const heading = useRef<HTMLHeadingElement>(null);
  const queryClient = useQueryClient();
  return <Dialog open={open} onOpenChange={(next) => {
    setOpen(next);
    if (!next) void queryClient.invalidateQueries({ queryKey: queryKeys.workspaces.setup(workspaceId), exact: true });
  }}>
    <DialogTrigger asChild><Button size="sm" variant="outline">Set up with AI</Button></DialogTrigger>
    <DialogContent className="max-h-[85vh] overflow-y-auto sm:max-w-xl" onOpenAutoFocus={(event) => { event.preventDefault(); heading.current?.focus(); }}>
      <DialogHeader>
        <DialogTitle ref={heading} tabIndex={-1} className="outline-none">Set up your workspace with your AI assistant</DialogTitle>
        <DialogDescription>Connect your assistant, then give it your workspace setup prompt.</DialogDescription>
      </DialogHeader>
      {open && <WorkspaceSetupAssistantContent key={workspaceId} workspaceId={workspaceId} slug={slug} />}
    </DialogContent>
  </Dialog>;
}

export function WorkspaceSetupAssistantContent({ workspaceId, slug }: Props) {
  const queryClient = useQueryClient();
  const setup = useQuery({
    queryKey: queryKeys.workspaces.workspaceSetup(workspaceId),
    queryFn: async () => {
      const guide = unwrap(await setupService.workspaceSetup(workspaceId));
      if (guide.workspace_id !== workspaceId) throw new Error('Workspace mismatch');
      return guide;
    },
    staleTime: 0, retry: false, refetchOnWindowFocus: 'always', refetchInterval: 15_000,
  });
  const mcp = useQuery({
    queryKey: queryKeys.mcp.dashboard(workspaceId),
    queryFn: async () => unwrap(await mcpService.getDashboard(workspaceId)),
    staleTime: 0, retry: false, refetchOnWindowFocus: 'always', refetchInterval: 15_000,
  });
  const refresh = () => {
    void setup.refetch();
    void mcp.refetch();
    void queryClient.invalidateQueries({ queryKey: queryKeys.workspaces.setup(workspaceId), exact: true });
  };
  const copy = async (value: string, message: string) => {
    try { await navigator.clipboard.writeText(value); toast.success(message); }
    catch { toast.error('Couldn’t copy. Select and copy the text below.'); }
  };
  if (setup.isPending) return <p role="status" className="py-6 text-sm text-muted-foreground">Checking workspace setup…</p>;
  if (setup.isError || !setup.data) return <div role="alert" className="space-y-3">
    <p className="text-sm">Couldn’t load setup for this workspace. Try again or continue in Helpin.</p>
    <Button variant="outline" size="sm" onClick={refresh}>Try again</Button>
  </div>;

  const guide = setup.data;
  const issue = mcp.isError ? 'Couldn’t check MCP access. Refresh to try again.' : mcp.data ? workspaceSetupConnectionIssue(mcp.data) : 'Checking MCP access…';
  const prompt = buildWorkspaceSetupPrompt(guide, slug, window.location.origin);
  const busy = setup.isFetching || mcp.isFetching;

  return <div className="space-y-6">
    <section className="space-y-2" aria-label="Connect your assistant">
      <div className="flex items-center justify-between gap-3">
        <h2 className="text-sm font-medium">1. Connect to Helpin</h2>
        <a href={`/w/${encodeURIComponent(slug)}/settings/mcp`} target="_blank" rel="noreferrer" className="text-xs text-muted-foreground underline underline-offset-4">MCP access</a>
      </div>
      {issue ? <p role="status" className="text-sm text-muted-foreground">{issue}</p> : <>
        <div className="flex items-center gap-2">
          <input aria-label="MCP server URL" readOnly value={mcp.data!.mcp_url} className="h-10 min-w-0 flex-1 rounded-md border bg-muted/30 px-3 text-xs" onFocus={(event) => event.target.select()} />
          <Button size="sm" variant="outline" onClick={() => void copy(mcp.data!.mcp_url, 'MCP URL copied')}>Copy URL</Button>
        </div>
        <p className="text-xs leading-relaxed text-muted-foreground">Add this server in your assistant’s MCP settings and sign in to this workspace. Allow read access for {workspaceSetupAccessLabels(guide.goals).join(', ')}.</p>
      </>}
    </section>
    <section className="space-y-2" aria-label="Start workspace setup">
      <div className="flex items-center gap-2">
        <h2 className="text-sm font-medium">2. Start or resume setup</h2>
        <QuickTooltip label="Your assistant needs its own browser tools to use Helpin settings, or it can give you the links. You handle sign-in and credentials, and approve invitations, publishing and live activation. Connecting a model to Helpin is only needed for Helpin’s own AI features.">
          <button type="button" aria-label="How assistant setup works" className="flex h-4 w-4 items-center justify-center rounded-full border text-[10px] text-muted-foreground">i</button>
        </QuickTooltip>
      </div>
      <Button onClick={() => void copy(prompt, 'Setup prompt copied')} disabled={Boolean(issue) || busy}>Copy setup prompt</Button>
      <p className="text-xs leading-relaxed text-muted-foreground">Paste it into your assistant. It includes your workspace, selected goals, current checks and setup instructions.</p>
      <details className="text-xs text-muted-foreground">
        <summary className="cursor-pointer">View prompt</summary>
        <textarea aria-label="Workspace setup prompt" readOnly value={prompt} className="mt-2 h-48 w-full resize-y rounded-md border bg-muted/30 p-3 text-xs" />
      </details>
    </section>
    <section className="border-t pt-4" aria-label="Current workspace setup">
      <div className="mb-2 flex items-center justify-between">
        <h2 className="text-sm font-medium">Current setup</h2>
        <Button size="sm" variant="ghost" onClick={refresh} disabled={busy}>Refresh</Button>
      </div>
      {guide.sections.map(section => <details key={section.key} className="group border-b py-2" open={Boolean(issue)}>
        <summary className="cursor-pointer text-sm">{section.title}</summary>
        <ul className="mt-2 divide-y">
          {section.steps.map(step => {
            const path = step.status === 'blocked' ? undefined : workspaceSetupPath(step.path, slug);
            return <li key={step.key} className="py-2">
              <div className="flex items-start justify-between gap-3 text-xs">
                {path ? <a href={path} target="_blank" rel="noreferrer" className="underline-offset-4 hover:underline">{step.title}</a> : <span>{step.title}</span>}
                <QuickTooltip label={step.blocked_reason || step.verification}>
                  <span tabIndex={0} className="shrink-0 text-muted-foreground">{step.status === 'completed' ? 'Check passed' : step.status === 'blocked' ? 'Access needed' : step.status === 'unable_to_verify' ? 'Test needed' : 'Not yet'}</span>
                </QuickTooltip>
              </div>
            </li>;
          })}
        </ul>
      </details>)}
    </section>
  </div>;
}
