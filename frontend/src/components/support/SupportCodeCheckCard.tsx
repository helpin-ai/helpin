import { useState, type ReactNode } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { AskAgentAvatar } from '@/components/agents/AskAgentAvatar';
import { ArrowTurnBackwardIcon, ClipboardIcon, Link01Icon, Loading03Icon } from '@/lib/icons';
import type { SupportMessage } from '@/lib/pmTypes';
import { queryKeys } from '@/lib/queryKeys';
import { unwrap } from '@/lib/queryUtils';
import { supportService } from '@/lib/services/supportService';
import { useSupportInboxStore } from '@/stores/supportInboxStore';
import { CardList, CardRow, SupportAICard } from './SupportAICardParts';
import { CREATE_SUPPORT_TASK_EVENT, cardAction, restoreReplyDraft } from './supportAICardActions';
import type { SupportCodeCheck, SupportCodeCheckTask } from './supportAIActivityModel';

function TaskStep({ task, workspaceId, conversationId, linkedTaskId, readOnly }: {
  task: SupportCodeCheckTask;
  workspaceId: string;
  conversationId: string;
  linkedTaskId?: string | null;
  readOnly: boolean;
}) {
  const queryClient = useQueryClient();
  const [linking, setLinking] = useState(false);
  const existing = task.kind === 'existing' && task.key;
  const link = async () => {
    if (!task.id) return;
    setLinking(true);
    try {
      unwrap(await supportService.linkConversationTask(workspaceId, conversationId, { task_id: task.id }));
      toast.success(`Linked ${task.key}`);
      void queryClient.invalidateQueries({ queryKey: queryKeys.support.conversation(workspaceId, conversationId) });
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Couldn’t link the task');
    } finally {
      setLinking(false);
    }
  };
  const createTask = () => window.dispatchEvent(new CustomEvent(CREATE_SUPPORT_TASK_EVENT, { detail: { conversationId } }));
  return <CardRow label="Task">
    <p>{existing ? <>Already covered by {task.key}{task.text ? `: ${task.text}` : ''}</> : task.text}</p>
    {!readOnly && (existing
      ? task.id && (linkedTaskId === task.id
        ? <p className="mt-1 text-xs text-muted-foreground">Linked to this conversation</p>
        : <button type="button" onClick={link} disabled={linking} className={cardAction}><Link01Icon className="h-3.5 w-3.5" />Link {task.key}</button>)
      : <button type="button" onClick={createTask} className={cardAction}><ClipboardIcon className="h-3.5 w-3.5" />Create task</button>)}
  </CardRow>;
}

function runningText(check: SupportCodeCheck) {
  if (check.phase === 'before_reply') return check.focus ? `Checking the code before replying: ${check.focus}` : 'Checking the code before replying.';
  return check.focus ? `Checking the code: ${check.focus}` : 'Checking the code for the cause.';
}

/**
 * Code check card: a running placeholder posted at handoff that the findings
 * fill in place. Rows appear only for what the check found.
 */
export function SupportCodeCheckCard({ message, check, linkedTaskId, readOnly = false, folded = false, renderMarkdown }: {
  message: SupportMessage;
  check: SupportCodeCheck;
  linkedTaskId?: string | null;
  readOnly?: boolean;
  folded?: boolean;
  renderMarkdown: (markdown: string) => ReactNode;
}) {
  const askAgentWithDraft = useSupportInboxStore((state) => state.askAgentWithDraft);
  const conversationId = message.conversation_id;
  const running = check.status === 'running';
  const unavailable = check.unavailable_data ?? [];
  const hasSteps = Boolean(check.reply || check.docs || check.task || check.investigate);
  let body: ReactNode;
  if (running) {
    body = <div className="text-sm">
      <p className="flex items-center gap-2"><Loading03Icon className="h-3.5 w-3.5 shrink-0 animate-spin text-amber-600 motion-reduce:animate-none" />{runningText(check)}</p>
      <p className="mt-0.5 pl-[1.375rem] text-xs text-muted-foreground">Details will appear here when done.</p>
    </div>;
  } else if (check.status === 'failed') {
    body = <p className="text-sm">{check.message}</p>;
  } else {
    body = <div className="text-sm [overflow-wrap:anywhere]">
      {check.found && <><h3 className="mb-1 text-sm font-semibold">What AI found</h3><p className="whitespace-pre-wrap">{check.found}</p></>}
      {unavailable.length > 0 && <dl className="mt-2"><CardRow label="Couldn't check"><CardList items={unavailable} /></CardRow></dl>}
      {hasSteps && <div className="mt-3">
        <h3 className="mb-1 text-sm font-semibold">Next steps</h3>
        <dl>
          {check.reply && <CardRow label="Reply">
            <p className="line-clamp-2">{check.reply.replace(/\s+/g, ' ')}</p>
            {!readOnly && <button type="button" onClick={() => restoreReplyDraft(conversationId, check.reply ?? '')} className={cardAction}><ArrowTurnBackwardIcon className="h-3.5 w-3.5" />Use this reply</button>}
          </CardRow>}
          {check.docs && <CardRow label="Docs">
            <p>{check.docs}</p>
            {!readOnly && <button type="button" onClick={() => askAgentWithDraft(conversationId, `Update our help articles so they cover this: ${check.docs}. Use this conversation and the code check findings as the source.`)} className={cardAction}><AskAgentAvatar plateStyle="solid" radius={50} className="h-3.5 w-3.5 shrink-0" />Ask Agent to update docs</button>}
          </CardRow>}
          {check.task && <TaskStep task={check.task} workspaceId={message.workspace_id} conversationId={conversationId} linkedTaskId={linkedTaskId} readOnly={readOnly} />}
          {check.investigate && <CardRow label="Investigate">
            <p>{check.investigate}</p>
            {!readOnly && <button type="button" onClick={() => askAgentWithDraft(conversationId, `Look into this for the conversation: ${check.investigate}`)} className={cardAction}><AskAgentAvatar plateStyle="solid" radius={50} className="h-3.5 w-3.5 shrink-0" />Ask Agent</button>}
          </CardRow>}
        </dl>
      </div>}
      {check.engineering && <details className="mt-2 text-xs text-muted-foreground">
        <summary className="w-fit cursor-pointer rounded-sm font-medium hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">Engineering details</summary>
        <div className="prose-chat mt-2 text-sm leading-relaxed text-foreground [overflow-wrap:anywhere]">{renderMarkdown(check.engineering)}</div>
      </details>}
    </div>;
  }
  const summary = running ? 'Checking the code' : check.status === 'failed' ? check.message : check.found;
  return <SupportAICard kind="code-check" title="Code check" timestamp={running ? message.created_at : message.updated_at || message.created_at} folded={folded} summary={summary}>
    {body}
  </SupportAICard>;
}
