import { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { queryKeys } from '@/lib/queryKeys';
import { unwrap } from '@/lib/queryUtils';
import { mcpService } from '@/lib/services/mcpService';
import { setupService } from '@/lib/services/setupService';
import { buildSupportSetupPrompt, supportSetupConnectionIssue } from '@/lib/supportSetup';

export function SupportSetupAssistant({ workspaceId, slug }: { workspaceId: string; slug: string }) {
  const [open, setOpen] = useState(false);
  const queryClient = useQueryClient();
  return <>
    <Dialog open={open} onOpenChange={(next) => {
      setOpen(next);
      if (!next) void queryClient.invalidateQueries({ queryKey: queryKeys.workspaces.setup(workspaceId), exact: true });
    }}>
      <DialogTrigger asChild><Button size="sm" variant="outline">Set up with AI</Button></DialogTrigger>
      <DialogContent className="max-h-[85vh] overflow-y-auto sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>Set up support with your AI assistant</DialogTitle>
          <DialogDescription>Use your assistant’s MCP connection and browser, or follow its links yourself. You can continue in Helpin at any time.</DialogDescription>
        </DialogHeader>
        {open && <SupportSetupAssistantContent key={workspaceId} workspaceId={workspaceId} slug={slug} />}
      </DialogContent>
    </Dialog>
  </>;
}

function SupportSetupAssistantContent({ workspaceId, slug }: { workspaceId: string; slug: string }) {
  const queryClient = useQueryClient();
  const setup = useQuery({
    queryKey: queryKeys.workspaces.supportSetup(workspaceId),
    queryFn: async () => unwrap(await setupService.supportSetup(workspaceId)),
    staleTime: 0, retry: false, refetchOnWindowFocus: 'always', refetchInterval: 15_000,
  });
  const mcp = useQuery({
    queryKey: queryKeys.mcp.dashboard(workspaceId),
    queryFn: async () => unwrap(await mcpService.getDashboard(workspaceId)),
    staleTime: 0, retry: false, refetchOnWindowFocus: 'always',
  });
  const refresh = () => {
    void setup.refetch();
    void mcp.refetch();
    void queryClient.invalidateQueries({ queryKey: queryKeys.workspaces.setup(workspaceId), exact: true });
  };
  const copy = async (text: string, message: string) => {
    try { await navigator.clipboard.writeText(text); toast.success(message); }
    catch { toast.error('Couldn’t copy. Select and copy the text below.'); }
  };
  if (setup.isPending) return <p role="status" className="py-6 text-sm text-muted-foreground">Checking support setup…</p>;
  if (setup.isError || !setup.data) return <div role="alert" className="space-y-3">
    <p className="text-sm">Couldn’t load setup. Your existing settings haven’t changed.</p>
    <Button variant="outline" size="sm" onClick={refresh}>Try again</Button>
  </div>;
  const guide = setup.data;
  const issue = mcp.isError ? 'Couldn’t check MCP access. Refresh to try again.' : mcp.data ? supportSetupConnectionIssue(mcp.data) : 'Checking MCP access…';
  const prompt = buildSupportSetupPrompt(guide, slug, window.location.origin);
  return <div className="space-y-5">
    <section className="space-y-2" aria-label="Connect your assistant">
      <div className="flex items-center justify-between gap-3">
        <h3 className="text-sm font-medium">1. Connect your assistant</h3>
        <a href={`/w/${slug}/settings/mcp`} className="text-xs text-muted-foreground underline underline-offset-4">MCP access</a>
      </div>
      {issue ? <p role="status" className="text-sm text-muted-foreground">{issue}</p> : <>
        <div className="flex items-center gap-2">
          <input aria-label="MCP server URL" readOnly value={mcp.data!.mcp_url} className="h-9 min-w-0 flex-1 rounded-md border bg-muted/30 px-3 text-xs" onFocus={(event) => event.target.select()} />
          <Button size="sm" variant="outline" onClick={() => copy(mcp.data!.mcp_url, 'MCP URL copied')}>Copy URL</Button>
        </div>
        <p className="text-xs text-muted-foreground">Sign in to this workspace and allow Support read access. Add Docs access if you want help with articles.</p>
      </>}
    </section>
    <section className="space-y-2" aria-label="Start setup">
      <h3 className="text-sm font-medium">2. Start or resume setup</h3>
      <Button onClick={() => copy(prompt, 'Setup prompt copied')} disabled={Boolean(issue)}>Copy setup prompt</Button>
      <p className="text-xs text-muted-foreground">Paste it into your assistant. You’ll handle sign-in and credentials, and approve publishing or live AI replies.</p>
      <details className="text-xs text-muted-foreground">
        <summary className="cursor-pointer">View prompt</summary>
        <textarea aria-label="Setup prompt" readOnly value={prompt} className="mt-2 h-44 w-full resize-y rounded-md border bg-muted/30 p-3 text-xs" />
      </details>
    </section>
    <section className="border-t pt-4" aria-label="Current support setup">
      <div className="mb-2 flex items-center justify-between">
        <h3 className="text-sm font-medium">Current setup</h3>
        <Button size="sm" variant="ghost" onClick={refresh} disabled={setup.isFetching || mcp.isFetching}>Refresh</Button>
      </div>
      <p className="mb-2 text-xs text-muted-foreground">Choose the channels you need. AI is optional. Checks update as settings change.</p>
      <ul className="divide-y">
        {guide.steps.map((step) => <li key={step.key} className="py-2">
          <div className="flex items-center justify-between gap-3 text-sm">
            {step.path ? <a href={`/w/${slug}${step.path}`} className="underline-offset-4 hover:underline">{step.title}</a> : <span>{step.title}</span>}
            <span className="shrink-0 text-xs text-muted-foreground">{step.status === 'completed' ? 'Configured' : step.status === 'blocked' ? 'Access needed' : step.status === 'unable_to_verify' ? 'Test needed' : 'Not configured'}</span>
          </div>
          {step.blocked_reason && <p className="mt-1 text-xs text-muted-foreground">{step.blocked_reason}</p>}
        </li>)}
      </ul>
      <p className="mt-2 text-xs text-muted-foreground">Configured settings don’t confirm message delivery or answer quality. Finish with a test conversation.</p>
    </section>
  </div>;
}
