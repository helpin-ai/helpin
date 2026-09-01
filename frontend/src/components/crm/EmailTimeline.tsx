import { useEffect, useMemo, useState } from 'react';
import { format, isThisMonth } from 'date-fns';
import { toast } from 'sonner';
import {
  ArrowDown02Icon, ArrowLeft02Icon, AttachmentIcon, CheckListIcon,
  DollarCircleIcon, Forward01Icon, Link01Icon, Loading01Icon, Mail01Icon,
  MailReply01Icon, PlusSignIcon, SentIcon, Setting07Icon, Tick01Icon,
} from '@/lib/icons';
import {
  useEmailAccounts, useEmailThread, useInfiniteEmailThreads, useLinkEmailThreadDeal,
  useReplyToEmailThread, useSetEmailThreadDismissed,
} from '@/hooks/queries/useCRM';
import { useCreateTask } from '@/hooks/queries';
import { useAccessibleTeams } from '@/hooks/useAccessibleTeams';
import { useAuthStore } from '@/stores/authStore';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { CRMEmailComposerDialog, type EmailDraft } from './CRMEmailComposerDialog';
import { CreateTaskModal } from '@/components/pm/CreateTaskModal';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { CRMEmailReplyComposer } from '@/components/crm/CRMEmailReplyComposer';
import { EmailBodyRenderer } from '@/components/support/EmailBodyRenderer';
import { Button } from '@/components/ui/button';
import { Dialog, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { QuietRelationshipDialogContent, QuietRelationshipResults, QuietSearchInput, quietRelationshipResultRowClassName } from '@/components/design-system/quiet';
import { crmEmailService, crmSearchService } from '@/lib/services/crmService';
import { unwrap } from '@/lib/queryUtils';
import { associationsService } from '@/lib/services/associationsService';
import { pmWorkflowService } from '@/lib/services/pmWorkflowService';
import { cn } from '@/lib/utils';
import type { CRMEmailMessage, CRMEmailParticipant, CRMEmailThread, CRMObjectType } from '@/lib/crmTypes';
import type { CreateTaskRequest, WorkflowWithStates } from '@/lib/pmTypes';

interface EmailTimelineProps {
  workspaceId: string;
  contactId?: string;
  companyId?: string;
  dealId?: string;
  defaultRecipient?: string;
  showComposeAction?: boolean;
  selectedThreadId?: string;
  onSelectedThreadChange?: (threadId?: string) => void;
  focusThreadOnly?: boolean;
}

type Scope = 'all' | 'direct' | 'needs_reply';
type Sort = 'newest' | 'oldest';

function messageText(message?: CRMEmailMessage) {
  if (!message) return '';
  if (message.body_text?.trim()) return message.body_text.replace(/\s+/g, ' ').trim();
  if (!message.body_html?.trim()) return '';
  if (typeof window === 'undefined') return message.body_html.replace(/<[^>]+>/g, ' ').replace(/\s+/g, ' ').trim();
  const doc = new window.DOMParser().parseFromString(message.body_html, 'text/html');
  return doc.body.textContent?.replace(/\s+/g, ' ').trim() ?? '';
}

function addressName(value?: string) {
  if (!value) return 'Unknown sender';
  const local = value.split('@')[0] ?? value;
  return local.replace(/[._-]+/g, ' ').replace(/\b\w/g, (character) => character.toUpperCase());
}

function relativeDate(value: string) {
  const date = new Date(value);
  return date.toDateString() === new Date().toDateString() ? format(date, 'h:mm a') : format(date, 'MMM d');
}

function dateRange(messages: CRMEmailMessage[]) {
  if (messages.length === 0) return '';
  const timestamps = messages.map((message) => new Date(message.sent_at).getTime()).sort((a, b) => a - b);
  const first = new Date(timestamps[0]);
  const last = new Date(timestamps[timestamps.length - 1]);
  return first.toDateString() === last.toDateString()
    ? format(last, 'MMM d, yyyy')
    : `${format(first, 'MMM d')} – ${format(last, 'MMM d, yyyy')}`;
}

function participantLabel(participant: CRMEmailParticipant) {
  return participant.contact_name || participant.name || addressName(participant.email);
}

function ThreadRow({ thread, selected, onSelect }: { thread: CRMEmailThread; selected: boolean; onSelect: () => void }) {
  const latest = thread.latest_message;
  const sender = latest?.from_name || addressName(latest?.from_address);
  return (
    <button type="button" onClick={onSelect} className={cn(
      'group relative flex w-full items-start gap-3 border-t border-border/45 px-5 py-3 text-left transition-colors hover:bg-muted/30',
      selected && 'bg-muted/35',
    )}>
      <span className={cn('absolute inset-y-0 left-0 w-[3px]', selected ? 'bg-foreground' : thread.needs_reply ? 'bg-orange-600' : 'bg-transparent')} />
      <UserAvatar name={sender} className="mt-0.5 h-7 w-7 shrink-0" fallbackClassName="text-[10px]" />
      <span className="min-w-0 flex-1">
        <span className="flex items-center gap-2 text-xs text-muted-foreground">
          <span className={cn('truncate text-foreground/75', thread.needs_reply ? 'font-semibold' : 'font-medium')}>{sender}</span>
          {thread.contact_ids.length > 0 ? <Link01Icon className="h-3 w-3 shrink-0 text-teal-700 dark:text-teal-400" /> : null}
          <span className="ml-auto shrink-0 text-[11px]">{thread.message_count > 1 ? thread.message_count : ''}</span>
          <span className="shrink-0 text-[11px]">{relativeDate(thread.last_message_at)}</span>
        </span>
        <span className={cn('pm-rich-text mt-0.5 block truncate', thread.needs_reply && 'font-semibold')}>{thread.subject || '(no subject)'}</span>
        <span className="mt-0.5 block truncate text-xs leading-5 text-muted-foreground">{messageText(latest) || 'No message preview'}</span>
        <span className="mt-1.5 flex items-center gap-2 text-[11px] text-muted-foreground">
          {latest?.direction === 'inbound' ? <ArrowDown02Icon className="h-3 w-3 text-teal-700 dark:text-teal-400" /> : <SentIcon className="h-3 w-3" />}
          <span>{latest?.direction === 'inbound' ? 'Received' : 'Sent'}</span>
          <span className="h-2.5 w-px bg-border" /><Mail01Icon className="h-3 w-3" />
          <span className="capitalize">{thread.mailbox_provider || 'mail'}</span>
        </span>
      </span>
    </button>
  );
}

function MessageBlock({ message, workspaceId }: { message: CRMEmailMessage; workspaceId: string }) {
  const sender = message.from_name || addressName(message.from_address);
  const recipients = [...message.to_addresses, ...message.cc_addresses].map(addressName).join(', ');
  return (
    <article className="flex gap-3 border-b border-border/45 px-5 py-5 sm:px-7">
      <UserAvatar name={sender} className="h-8 w-8 shrink-0" fallbackClassName="text-[10px]" />
      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 items-start gap-2">
          <div className="min-w-0 flex-1"><span className="text-[13px] font-semibold text-foreground">{sender}</span><span className="ml-2 truncate text-xs text-muted-foreground">to {recipients || 'undisclosed recipients'}</span></div>
          <time className="shrink-0 text-[11px] text-muted-foreground">{format(new Date(message.sent_at), 'MMM d, yyyy · h:mm a')}</time>
        </div>
        <div className="pm-rich-text mt-2 max-w-[680px]">
          {message.body_html ? <EmailBodyRenderer html={message.body_html} collapsedByDefault constrainHeight={false} /> : <p className="whitespace-pre-wrap break-words">{message.body_text || 'No message body'}</p>}
        </div>
        {message.attachments?.length ? (
          <div className="mt-3 flex flex-wrap gap-1.5">
            {message.attachments.map((attachment) => (
              <button key={attachment.id} type="button" className="inline-flex min-w-0 items-center gap-1.5 rounded-md border border-border/60 px-2 py-1 text-xs text-muted-foreground hover:bg-muted hover:text-foreground" onClick={async () => {
                const popup = window.open('', '_blank');
                try {
                  const result = unwrap(await crmEmailService.getAttachmentDownload(workspaceId, attachment.id));
                  if (popup) popup.location.href = result.url;
                  else window.location.assign(result.url);
                } catch (error) { popup?.close(); toast.error(error instanceof Error ? error.message : 'Attachment could not be downloaded'); }
              }}>
                <AttachmentIcon className="h-3.5 w-3.5" /><span className="max-w-48 truncate">{attachment.file_name}</span>
              </button>
            ))}
          </div>
        ) : null}
      </div>
    </article>
  );
}

export function EmailTimeline({
  workspaceId, contactId, companyId, dealId, defaultRecipient, showComposeAction = true,
  selectedThreadId: controlledThreadId, onSelectedThreadChange, focusThreadOnly = false,
}: EmailTimelineProps) {
  const { currentWorkspace } = useWorkspaceStore();
  const userId = useAuthStore((state) => state.user?.id);
  const [scope, setScope] = useState<Scope>('all');
  const [sort, setSort] = useState<Sort>('newest');
  const [searchInput, setSearchInput] = useState('');
  const [search, setSearch] = useState('');
  const [internalThreadId, setInternalThreadId] = useState<string>();
  const [mobileReading, setMobileReading] = useState(false);
  const [composeDraft, setComposeDraft] = useState<EmailDraft | null>(null);
  const [replyMode, setReplyMode] = useState<'reply' | 'reply_all'>('reply');
  const [replyHTML, setReplyHTML] = useState('');
  const [taskOpen, setTaskOpen] = useState(false);
  const [taskWorkflow, setTaskWorkflow] = useState<WorkflowWithStates | null>(null);
  const [taskTeamId, setTaskTeamId] = useState('');
  const [dealOpen, setDealOpen] = useState(false);
  const [dealQuery, setDealQuery] = useState('');
  const [dealResults, setDealResults] = useState<Array<{ id: string; name: string }>>([]);
  const accounts = useEmailAccounts(workspaceId);
  const sendableAccounts = (accounts.data ?? []).filter((account) => account.can_send === true || (account.member_id === userId && account.is_active && account.status === 'connected' && account.provider === 'gmail'));
  const { teams } = useAccessibleTeams(workspaceId);
  const createTask = useCreateTask(workspaceId);
  const reply = useReplyToEmailThread(workspaceId);
  const dismiss = useSetEmailThreadDismissed(workspaceId);
  const linkDeal = useLinkEmailThreadDeal(workspaceId);

  useEffect(() => {
    const timer = window.setTimeout(() => setSearch(searchInput.trim()), 250);
    return () => window.clearTimeout(timer);
  }, [searchInput]);

  const filters = useMemo(() => ({ contact_id: contactId, company_id: companyId, deal_id: dealId, search, scope, sort }), [companyId, contactId, dealId, scope, search, sort]);
  const threadsQuery = useInfiniteEmailThreads(workspaceId, filters);
  const threads = useMemo(() => threadsQuery.data?.pages.flatMap((page) => page?.data ?? []) ?? [], [threadsQuery.data]);
  const selectedThreadId = controlledThreadId ?? internalThreadId ?? threads[0]?.id;
  const selectedThread = useEmailThread(workspaceId, selectedThreadId);

  useEffect(() => {
    if (!dealOpen || dealQuery.trim().length < 2) return;
    const timer = window.setTimeout(async () => {
      const response = await crmSearchService.search(workspaceId, dealQuery.trim());
      setDealResults((response.data ?? []).filter((result) => result.type === 'deal').map((result) => ({ id: result.id, name: result.name })));
    }, 250);
    return () => window.clearTimeout(timer);
  }, [dealOpen, dealQuery, workspaceId]);

  const selectThread = (threadId: string) => {
    setInternalThreadId(threadId); onSelectedThreadChange?.(threadId); setMobileReading(true); setReplyHTML('');
  };
  const openSettings = () => { if (currentWorkspace?.slug) window.location.href = `/w/${currentWorkspace.slug}/settings/crm-email`; };
  const sendReply = async (attachments?: { draftId: string; attachmentIds: string[] }) => {
    if (!selectedThreadId || !replyHTML.replace(/<[^>]+>/g, '').trim()) return;
    try { await reply.mutateAsync({ threadId: selectedThreadId, mode: replyMode, body_html: replyHTML, draft_id: attachments?.draftId, attachment_ids: attachments?.attachmentIds }); setReplyHTML(''); toast.success('Reply sent'); }
    catch (error) { toast.error(error instanceof Error ? error.message : 'Reply could not be sent'); throw error; }
  };
  const openCreateTask = async () => {
    const team = teams[0];
    if (!team) { toast.error('Join a team before creating a task'); return; }
    const workflow = await pmWorkflowService.resolveTeamWorkflow(workspaceId, team.id);
    if (workflow.error || !workflow.data) { toast.error(workflow.error || 'Could not resolve team workflow'); return; }
    setTaskTeamId(team.id); setTaskWorkflow(workflow.data); setTaskOpen(true);
  };
  const createAndLinkTask = async (payload: CreateTaskRequest) => {
    const result = await createTask.mutateAsync(payload);
    const target: { type: CRMObjectType; id: string } | null = contactId ? { type: 'contact', id: contactId } : companyId ? { type: 'company', id: companyId } : dealId ? { type: 'deal', id: dealId } : null;
    if (target && result.task?.id) {
      const association = await associationsService.createAssociation({ workspace_id: workspaceId, from_object_type: 'task', from_object_id: result.task.id, to_object_type: target.type, to_object_id: target.id });
      if (association.error) throw new Error(association.error);
    }
    return result.task ? { id: result.task.id, task: result.task } : undefined;
  };

  const detail = selectedThread.data;
  const selected = detail?.thread;
  const visibleDealResults = dealQuery.trim().length >= 2 ? dealResults : [];
  const messageGroups = useMemo(() => {
    const needsReply = threads.filter((thread) => thread.needs_reply);
    const remaining = threads.filter((thread) => !thread.needs_reply);
    return [
      { label: 'Needs reply', items: needsReply },
      { label: 'This month', items: remaining.filter((thread) => isThisMonth(new Date(thread.last_message_at))) },
      { label: 'Earlier', items: remaining.filter((thread) => !isThisMonth(new Date(thread.last_message_at))) },
    ].filter((group) => group.items.length > 0);
  }, [threads]);

  return (
    <div className="@container/email flex h-full min-h-0 overflow-hidden bg-transparent">
      <section className={cn(
        'flex min-h-0 w-full shrink-0 flex-col border-r border-border/60 @[820px]/email:w-[clamp(320px,38%,456px)]',
        mobileReading && 'hidden @[820px]/email:flex',
        focusThreadOnly && '!hidden',
      )}>
        <div className="flex items-center gap-2 border-b border-border/50 px-4 py-2.5">
          <QuietSearchInput containerClassName="min-w-0 flex-1" value={searchInput} onChange={(event) => setSearchInput(event.target.value)} placeholder="Search conversations…" />
          {showComposeAction ? <Button size="sm" className="h-7 gap-1.5 px-2.5 text-xs" onClick={() => setComposeDraft({ to: defaultRecipient ? [defaultRecipient] : [] })} disabled={sendableAccounts.length === 0}><PlusSignIcon className="h-3.5 w-3.5" /> Compose</Button> : null}
        </div>
        <div className="flex items-center gap-4 border-b border-border/60 px-5 py-2.5 text-xs">
          {(['all', 'direct', 'needs_reply'] as Scope[]).map((value) => <button key={value} type="button" onClick={() => setScope(value)} className={cn('capitalize text-muted-foreground transition-colors hover:text-foreground', scope === value && 'font-semibold text-foreground')}>{value === 'needs_reply' ? 'Needs reply' : value}</button>)}
          <button type="button" className="ml-auto flex items-center gap-1 text-muted-foreground hover:text-foreground" onClick={() => setSort((current) => current === 'newest' ? 'oldest' : 'newest')}>{sort === 'newest' ? 'Newest' : 'Oldest'} <ArrowDown02Icon className={cn('h-3 w-3 transition-transform', sort === 'oldest' && 'rotate-180')} /></button>
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto">
          {threadsQuery.isLoading ? <div className="space-y-px p-4">{Array.from({ length: 6 }).map((_, index) => <div key={index} className="h-[92px] animate-pulse rounded bg-muted/45" />)}</div>
            : threadsQuery.isError ? <div className="flex h-full flex-col items-center justify-center px-6 text-center"><Mail01Icon className="h-5 w-5 text-muted-foreground" /><p className="mt-3 text-sm font-medium">Unable to load emails</p><Button variant="ghost" size="sm" className="mt-2" onClick={() => void threadsQuery.refetch()}>Try again</Button></div>
              : threads.length === 0 ? <div className="flex h-full flex-col items-center justify-center px-6 text-center"><Mail01Icon className="h-5 w-5 text-muted-foreground/60" /><p className="mt-3 text-sm font-medium">No conversations found</p><p className="mt-1 max-w-xs text-xs text-muted-foreground">Synced conversations for this record will appear here.</p></div>
                : <>{messageGroups.map((group) => <div key={group.label}><div className="px-5 pb-1.5 pt-3 text-[11px] font-semibold uppercase tracking-[0.06em] text-muted-foreground">{group.label}</div>{group.items.map((thread) => <ThreadRow key={thread.id} thread={thread} selected={thread.id === selectedThreadId} onSelect={() => selectThread(thread.id)} />)}</div>)}{threadsQuery.hasNextPage ? <div className="flex justify-center border-t border-border/50 p-3"><Button variant="ghost" size="sm" disabled={threadsQuery.isFetchingNextPage} onClick={() => void threadsQuery.fetchNextPage()}>{threadsQuery.isFetchingNextPage ? <Loading01Icon className="h-3.5 w-3.5 animate-spin" /> : null}Load more</Button></div> : null}</>}
        </div>
        <div className="flex items-center gap-2 border-t border-border/60 px-5 py-3 text-xs text-muted-foreground"><Mail01Icon className="h-3.5 w-3.5" /><span>{selected?.mailbox_last_synced_at ? `Synced ${format(new Date(selected.mailbox_last_synced_at), 'MMM d, h:mm a')}` : 'Mailbox sync status'}</span><button type="button" onClick={openSettings} className="ml-auto inline-flex items-center gap-1 hover:text-foreground"><Setting07Icon className="h-3.5 w-3.5" /> Settings</button></div>
      </section>

      <section className={cn(
        'min-h-0 min-w-0 flex-1 flex-col',
        focusThreadOnly ? 'flex' : mobileReading ? 'flex' : 'hidden @[820px]/email:flex',
      )}>
        {!selectedThreadId ? <div className="flex h-full items-center justify-center text-sm text-muted-foreground">Select a conversation</div>
          : selectedThread.isLoading ? <div className="flex h-full items-center justify-center"><Loading01Icon className="h-5 w-5 animate-spin text-muted-foreground" /></div>
            : !detail ? <div className="flex h-full items-center justify-center text-sm text-muted-foreground">Conversation unavailable</div>
              : <>
                <header className="border-b border-border/50 px-5 py-2.5 sm:px-7 @[820px]/email:py-4">
                  {!focusThreadOnly ? <button type="button" onClick={() => setMobileReading(false)} className="mb-1 inline-flex items-center gap-1 text-xs text-muted-foreground @[820px]/email:hidden"><ArrowLeft02Icon className="h-3.5 w-3.5" /> Conversations</button> : null}
                  <div className="flex flex-wrap items-start gap-2 @[820px]/email:gap-3"><div className="min-w-0 flex-1"><h2 className="text-base font-semibold tracking-tight text-foreground @[820px]/email:text-lg">{detail.thread.subject || '(no subject)'}</h2><div className="mt-1 flex min-w-0 items-center gap-2 overflow-hidden text-xs text-muted-foreground @[820px]/email:mt-1.5"><span className="shrink-0">{detail.thread.message_count} messages</span><span className="h-2.5 w-px shrink-0 bg-border" /><span className="shrink-0">{dateRange(detail.messages)}</span><span className="h-2.5 w-px shrink-0 bg-border" /><Mail01Icon className="h-3 w-3 shrink-0" /><span className="shrink-0 capitalize">{detail.thread.mailbox_provider}</span><span className="truncate">{detail.thread.mailbox_email}</span></div></div>
                    <div className="flex flex-wrap items-center gap-3 text-xs">{detail.thread.can_reply ? <button type="button" className="inline-flex items-center gap-1.5 text-muted-foreground hover:text-foreground" onClick={() => document.getElementById('crm-thread-reply')?.scrollIntoView({ behavior: 'smooth' })}><MailReply01Icon className="h-3.5 w-3.5" /> Reply</button> : null}<button type="button" className="inline-flex items-center gap-1.5 text-muted-foreground hover:text-foreground" onClick={() => void openCreateTask()}><CheckListIcon className="h-3.5 w-3.5" /> Create task</button>{!dealId ? <button type="button" className="inline-flex items-center gap-1.5 text-muted-foreground hover:text-foreground" onClick={() => setDealOpen(true)}><DollarCircleIcon className="h-3.5 w-3.5" /> Link deal</button> : null}{detail.thread.needs_reply ? <button type="button" className="inline-flex items-center gap-1.5 text-muted-foreground hover:text-foreground" onClick={() => void dismiss.mutateAsync({ threadId: detail.thread.id, dismissed: true })}><Tick01Icon className="h-3.5 w-3.5" /> Dismiss</button> : null}</div>
                  </div>
                </header>
                <div className="flex flex-wrap items-start gap-x-5 gap-y-1.5 border-b border-border/50 px-5 py-2 sm:px-7 @[820px]/email:py-3"><span className="w-14 shrink-0 pt-1 text-[10px] font-semibold uppercase tracking-[0.06em] text-muted-foreground">People</span><div className="flex min-w-0 flex-1 flex-wrap gap-x-5 gap-y-1.5">{detail.participants.map((participant) => <div key={participant.email} className="flex min-w-0 items-center gap-2"><UserAvatar name={participantLabel(participant)} className="h-5 w-5" fallbackClassName="text-[8px]" /><span className="text-xs font-medium">{participantLabel(participant)}</span><span className="text-[10px] uppercase tracking-wide text-muted-foreground">{participant.role}</span>{participant.contact_id ? <Link01Icon className="h-3 w-3 text-teal-700 dark:text-teal-400" /> : <span className="max-w-44 truncate text-[11px] text-muted-foreground">{participant.email}</span>}</div>)}</div></div>
                <div className="min-h-0 flex-1 overflow-y-auto">
                  {detail.messages.map((message) => <MessageBlock key={message.id} message={message} workspaceId={workspaceId} />)}
                </div>
                <div id="crm-thread-reply" className="z-10 max-h-[48%] shrink-0 overflow-y-auto border-t border-border/60 bg-transparent px-5 py-2.5 sm:px-7 @[820px]/email:py-4">
                  {detail.thread.can_reply ? (
                    <div className="flex gap-3">
                      <UserAvatar name={detail.thread.mailbox_email} className="h-7 w-7 shrink-0" fallbackClassName="text-[9px]" />
                      <div className="min-w-0 flex-1">
                        <div className="mb-2 flex items-center gap-4 text-xs">
                          <button type="button" onClick={() => setReplyMode('reply')} className={cn('text-muted-foreground', replyMode === 'reply' && 'font-semibold text-foreground')}>Reply</button>
                          <button type="button" onClick={() => setReplyMode('reply_all')} className={cn('text-muted-foreground', replyMode === 'reply_all' && 'font-semibold text-foreground')}>Reply all</button>
                          <button type="button" onClick={() => setComposeDraft({ title: 'Forward email', subject: `Fwd: ${detail.thread.subject}`, body: messageText(detail.messages[0]) })} className="inline-flex items-center gap-1 text-muted-foreground hover:text-foreground"><Forward01Icon className="h-3 w-3" /> Forward</button>
                          <span className="ml-auto truncate text-[11px] text-muted-foreground">Sends from {detail.thread.mailbox_email}</span>
                        </div>
                        <CRMEmailReplyComposer key={selectedThreadId} workspaceId={workspaceId} content={replyHTML} onChange={setReplyHTML} sending={reply.isPending} onSubmit={sendReply} />
                      </div>
                    </div>
                  ) : (
                    <div className="flex items-start gap-3 py-2"><Mail01Icon className="mt-0.5 h-4 w-4 text-muted-foreground" /><div><p className="text-sm font-medium">Read-only conversation</p><p className="mt-1 text-xs leading-5 text-muted-foreground">This mailbox belongs to another workspace member. You can view the thread, but only the person who connected {detail.thread.mailbox_email || 'this mailbox'} can reply.</p></div></div>
                  )}
                </div>
              </>}
      </section>

      <CRMEmailComposerDialog workspaceId={workspaceId} accounts={sendableAccounts} open={Boolean(composeDraft)} draft={composeDraft ?? undefined} onOpenChange={(open) => !open && setComposeDraft(null)} />
      <CreateTaskModal open={taskOpen} onOpenChange={setTaskOpen} workspaceId={workspaceId} workflow={taskWorkflow ?? undefined} initialTeamId={taskTeamId} initialName={selected ? `Follow up: ${selected.subject}` : 'Email follow-up'} onCreate={createAndLinkTask} />
      <Dialog open={dealOpen} onOpenChange={setDealOpen}>
        <QuietRelationshipDialogContent>
          <DialogHeader><DialogTitle>Link deal to conversation</DialogTitle></DialogHeader>
          <QuietSearchInput value={dealQuery} onChange={(event) => setDealQuery(event.target.value)} placeholder="Search deals" />
          <QuietRelationshipResults className="max-h-64 overflow-y-auto border-t border-border/50 pt-2">
            {visibleDealResults.map((deal) => (
              <button
                key={deal.id}
                type="button"
                className={cn(quietRelationshipResultRowClassName, 'flex w-full items-center gap-2 px-2 py-2 text-left text-sm hover:bg-muted/40')}
                onClick={async () => {
                  if (!selectedThreadId) return;
                  await linkDeal.mutateAsync({ threadId: selectedThreadId, dealId: deal.id });
                  setDealOpen(false);
                  setDealQuery('');
                  toast.success('Deal linked');
                }}
              >
                <DollarCircleIcon className="h-4 w-4 shrink-0 text-muted-foreground" />
                <span className="min-w-0 flex-1 truncate">{deal.name}</span>
              </button>
            ))}
            {dealQuery.trim().length < 2 ? (
              <p className="py-5 text-center text-xs text-muted-foreground">Type at least two characters</p>
            ) : visibleDealResults.length === 0 ? (
              <p className="py-5 text-center text-xs text-muted-foreground">No deals found</p>
            ) : null}
          </QuietRelationshipResults>
        </QuietRelationshipDialogContent>
      </Dialog>
    </div>
  );
}
