import { useId, useState, type FormEvent, type ReactNode } from 'react';
import { Link } from '@tanstack/react-router';
import { useMutation, useQuery } from '@tanstack/react-query';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Progress } from '@/components/ui/progress';
import { useChatSettings } from '@/hooks/queries';
import { Alert01Icon, ArrowRight01Icon, BubbleChatIcon, CheckmarkCircle02Icon, Loading01Icon, Mail01Icon, UserAdd01Icon } from '@/lib/icons';
import { queryKeys } from '@/lib/queryKeys';
import { unwrap } from '@/lib/queryUtils';
import { agentService } from '@/lib/services/agentService';
import { supportQuickstartService, type SupportQuickstartStatus } from '@/lib/services/supportQuickstartService';
import type { SupportAIPreviewResponse } from '@/lib/pmTypes';
import { siteLabel } from '@/lib/workspaceOnboardingFlow';
import { cn } from '@/lib/utils';

type RowState = 'working' | 'done' | 'failed';

/** Polls the support setup, shows each part as it finishes, and lets the person try the AI. */
export function SupportSetupProgress({ workspaceId, slug }: { workspaceId: string; slug: string }) {
  const status = useQuery({
    queryKey: queryKeys.workspaces.supportQuickstart(workspaceId),
    queryFn: async () => unwrap(await supportQuickstartService.status(workspaceId)),
    refetchInterval: (query) => {
      const latest = query.state.data;
      if (latest?.done && latest.agent_ready) return false;
      // Articles copy several a second; follow them closely enough to see it.
      return latest?.articles?.status === 'importing' ? 1500 : 4000;
    },
  });
  const data = status.data;
  if (!data) {
    return <p role="status" className="flex items-center gap-2 text-sm text-muted-foreground"><Loading01Icon className="h-4 w-4 animate-spin" aria-hidden="true" />Loading your setup…</p>;
  }
  return (
    <div className="space-y-8">
      <ul className="divide-y rounded-xl border" aria-live="polite">
        <ProgressRow state="done" title="Workspace, inbox and chat widget" detail="Named and branded for your company." />
        {data.articles && <ProgressRow {...articlesRow(data.articles)} />}
        {data.sources.map((source) => <ProgressRow key={source.url} {...sourceRow(source, Boolean(data.articles))} />)}
        <ProgressRow {...agentRow(data)} />
      </ul>

      {data.agent_ready && data.ai_mode !== 'off' && <AskAgent workspaceId={workspaceId} />}

      <section className="space-y-3" aria-label="Next steps">
        <h2 className="text-sm font-medium">When you’re ready</h2>
        <div className="grid gap-3 sm:grid-cols-3">
          <NextStep icon={BubbleChatIcon} to={`/w/${slug}/settings/chat-general`} title="Add chat to your site">One snippet, or send it to your developer.</NextStep>
          <NextStep icon={Mail01Icon} to={`/w/${slug}/settings/support-channels`} title="Connect support email">Forward your support address.</NextStep>
          <NextStep icon={UserAdd01Icon} to={`/w/${slug}/settings/members`} title="Invite your team">Teammates answer from the same inbox.</NextStep>
        </div>
      </section>
    </div>
  );
}

function articlesRow(articles: NonNullable<SupportQuickstartStatus['articles']>): RowProps {
  if (articles.status === 'failed') {
    return { state: 'failed', title: 'Help articles', detail: 'The help center couldn’t be copied. You can import it later from Settings → Import.' };
  }
  if (articles.status === 'done') {
    const failed = articles.failed > 0 ? `, ${articles.failed} skipped` : '';
    return { state: 'done', title: `${articles.imported} help articles copied${failed}`, detail: 'Published in your help center.' };
  }
  return {
    state: 'working',
    title: 'Copying your help articles',
    detail: articles.total > 0 ? `${articles.imported} of ${articles.total}` : 'Reading your help center…',
    progress: articles.total > 0 ? Math.round((articles.imported / articles.total) * 100) : undefined,
    current: articles.current,
  };
}

function sourceRow(source: SupportQuickstartStatus['sources'][number], hasArticles: boolean): RowProps {
  const host = siteLabel(source.url);
  if (source.status === 'ready' && source.indexed_pages > 0) {
    return { state: 'done', title: `Read ${host}`, detail: `${source.indexed_pages} ${source.indexed_pages === 1 ? 'page' : 'pages'} for the AI to answer from.` };
  }
  if (source.status === 'failed' || source.status === 'disabled' || source.status === 'ready') {
    // The site may have been read and then not indexed, so this doesn't say which.
    return {
      state: 'failed',
      title: `Couldn’t learn from ${host}`,
      detail: `${hasArticles ? 'Your agent still answers from your help articles. ' : ''}Try the site again from Settings → Knowledge sources.`,
    };
  }
  return { state: 'working', title: `Reading ${host}`, detail: source.indexed_pages > 0 ? `${source.indexed_pages} pages so far` : 'This takes a few minutes.' };
}

function agentRow(data: SupportQuickstartStatus): RowProps {
  if (data.agent_ready) {
    return {
      state: 'done',
      title: data.ai_mode === 'draft' ? 'AI agent ready to draft replies' : data.ai_mode === 'answer' ? 'AI agent ready to answer' : 'Knowledge ready',
      detail: data.ai_mode === 'off' ? 'Turn on AI replies in settings when you’re ready.' : 'It answers from your articles and site, and hands off when unsure.',
    };
  }
  if (data.done) {
    return { state: 'working', title: 'Preparing your AI agent', detail: 'Indexing what was copied. Usually a few minutes.' };
  }
  return { state: 'working', title: 'Preparing your AI agent', detail: 'Starts as soon as the first pages are in.' };
}

type RowProps = {
  state: RowState;
  title: string;
  detail: string;
  /** How far along the row is, 0 to 100, when that is known. */
  progress?: number;
  /** What the row is working on right now. */
  current?: string;
};

function ProgressRow({ state, title, detail, progress, current }: RowProps) {
  return (
    <li className="flex gap-3 p-4">
      <span className="mt-0.5 shrink-0" aria-hidden="true">
        {state === 'done' && <CheckmarkCircle02Icon className="h-[18px] w-[18px] text-emerald-600" />}
        {state === 'working' && <Loading01Icon className="h-[18px] w-[18px] animate-spin text-muted-foreground" />}
        {state === 'failed' && <Alert01Icon className="h-[18px] w-[18px] text-amber-600" />}
      </span>
      <div className="min-w-0 flex-1">
        <p className={cn('text-[13.5px] font-medium leading-5', state === 'working' && 'text-foreground/80')}>{title}</p>
        <p className="mt-0.5 text-[12.5px] leading-5 tabular-nums text-muted-foreground">{detail}</p>
        {progress !== undefined && (
          <Progress value={progress} aria-label={title} className="mt-2 h-1.5" indicatorClassName="bg-emerald-600 duration-700 ease-out" />
        )}
        {current && (
          // Keyed so each article fades in as the copy moves on; the count above is what screen readers follow.
          <p key={current} aria-hidden="true" className="mt-2 truncate font-mono text-[12px] leading-5 text-muted-foreground duration-300 animate-in fade-in slide-in-from-bottom-1 motion-reduce:animate-none">
            <span className="text-muted-foreground/60">› </span>
            {current}
          </p>
        )}
      </div>
      <span className="sr-only">{state === 'done' ? 'Done' : state === 'failed' ? 'Needs attention' : 'In progress'}</span>
    </li>
  );
}

function NextStep({ icon: Icon, to, title, children }: { icon: typeof Mail01Icon; to: string; title: string; children: ReactNode }) {
  return (
    <Link to={to} className="group flex flex-col rounded-xl border p-4 no-underline transition-colors hover:border-foreground/25 hover:bg-muted/40 hover:no-underline">
      <Icon className="h-[18px] w-[18px] text-muted-foreground" aria-hidden="true" />
      <span className="mt-3 flex items-center gap-1 text-[13.5px] font-medium">
        {title}
        <ArrowRight01Icon className="h-3.5 w-3.5 opacity-0 transition-opacity group-hover:opacity-100" aria-hidden="true" />
      </span>
      <span className="mt-1 text-[12.5px] leading-5 text-muted-foreground">{children}</span>
    </Link>
  );
}

/** One question to the support agent, answered from the knowledge just set up, without contacting anyone. */
function AskAgent({ workspaceId }: { workspaceId: string }) {
  const id = useId();
  const settings = useChatSettings(workspaceId);
  const agentId = settings.data?.settings?.ai_agent_id ?? '';
  const [question, setQuestion] = useState('');
  const [run, setRun] = useState<SupportAIPreviewResponse | null>(null);
  const ask = useMutation({
    mutationFn: async () => unwrap(await agentService.previewSupportReply(workspaceId, agentId, { message: question.trim() })),
    onSuccess: setRun,
  });
  const result = useQuery({
    queryKey: ['support-preview', workspaceId, agentId, run?.run_id],
    enabled: Boolean(run?.run_id),
    queryFn: async () => unwrap(await agentService.getSupportPreview(workspaceId, agentId, run!.run_id!)),
    refetchInterval: (query) => (query.state.data && query.state.data.final_decision !== 'pending' ? false : 1000),
  });
  if (!agentId) return null;
  const response = result.data ?? run;
  const thinking = ask.isPending || response?.final_decision === 'pending';
  const error = ask.error ?? result.error;
  const submit = (event: FormEvent) => {
    event.preventDefault();
    if (question.trim() && !thinking) ask.mutate();
  };
  return (
    <section aria-labelledby={`${id}-title`} className="space-y-3 rounded-xl border bg-muted/30 p-4">
      <h2 id={`${id}-title`} className="text-sm font-medium">Ask your AI agent something a customer would</h2>
      <form onSubmit={submit} className="flex gap-2">
        <Input value={question} onChange={(event) => setQuestion(event.target.value)} placeholder="How do I reset my password?" aria-label="Customer question" maxLength={2000} className="bg-background" />
        <Button type="submit" disabled={thinking || !question.trim()}>{thinking ? 'Thinking…' : 'Ask'}</Button>
      </form>
      <div aria-live="polite" className="empty:hidden">
        {error && <p role="alert" className="text-sm text-destructive">{error.message}</p>}
        {response && response.final_decision !== 'pending' && (response.answer?.content && response.answer.can_answer
          ? <p className="whitespace-pre-wrap rounded-lg bg-background p-3 text-[13.5px] leading-6">{response.answer.content}</p>
          : <p className="rounded-lg bg-background p-3 text-[13px] text-muted-foreground">Your docs don’t cover this yet, so the agent would hand it to your team.</p>)}
      </div>
    </section>
  );
}
