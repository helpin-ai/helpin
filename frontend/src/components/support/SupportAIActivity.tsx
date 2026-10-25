import { AskAgentAvatar } from '@/components/agents/AskAgentAvatar';
import { useState, type ReactNode } from 'react';
import { getSupportHandoffCard, type AIActivity, type SupportHandoffCard } from './supportAIActivityModel';
import type { SupportMessage } from '@/lib/pmTypes';
import { ArrowTurnBackwardIcon } from '@/lib/icons';
import { useSupportInboxStore } from '@/stores/supportInboxStore';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { formatTimestamp, getAvatarColor, getInitial } from './helpers';
import { CardList, CardRow, SupportAICard } from './SupportAICardParts';
import { cardAction, restoreReplyDraft } from './supportAICardActions';

function handoffSections(content: string) {
  const match = content.match(/^AI handoff\n\nIssue\n([\s\S]*?)\n\nAlready tried \/ suggested\n([\s\S]*?)\n\nStill unresolved\n([\s\S]*?)\n\nReason for handoff\n([\s\S]*)$/);
  return match ? { issue: match[1], checked: match[2], next: match[3], reason: match[4] } : null;
}

/** Structured AI handoff: each fact once, rows only when they have content. */
function SupportHandoffCardBody({ card, conversationId, readOnly }: { card: SupportHandoffCard; conversationId: string; readOnly: boolean }) {
  const askAgentWithDraft = useSupportInboxStore((state) => state.askAgentWithDraft);
  const checked = (card.checked ?? []).filter((check) => check.source || check.finding);
  const openQuestions = card.open_questions ?? [];
  const unavailable = card.unavailable_data ?? [];
  const heldReply = card.held_reply?.trim() ?? '';
  const docsGap = card.help_article_gap?.trim() ?? '';
  const applyReply = () => restoreReplyDraft(conversationId, heldReply);
  const askToUpdateDocs = () => askAgentWithDraft(conversationId, `Update our help articles so they cover this: ${docsGap}. Use this conversation, including our reply to the customer, as the source.`);
  return <div className="text-sm [overflow-wrap:anywhere]">
    {card.issue && <p className="mb-2 whitespace-pre-wrap">{card.issue}</p>}
    <dl>
      <CardRow label="Reason">{card.reason}</CardRow>
      {checked.length > 0 ? <CardRow label="AI checked">
        <ul className="space-y-1">{checked.map((check, index) => <li key={index}>
          {check.source && <span className="text-muted-foreground">{check.source}{check.finding ? ':' : ''}</span>}{check.source && check.finding ? ' ' : ''}{check.finding}
        </li>)}</ul>
      </CardRow> : card.nothing_checked ? <CardRow label="AI checked">Nothing.</CardRow> : null}
      {openQuestions.length > 0 && <CardRow label="Open question"><CardList items={openQuestions} /></CardRow>}
      {unavailable.length > 0 && <CardRow label="Couldn't check"><CardList items={unavailable} /></CardRow>}
    </dl>
    {(heldReply || docsGap) && <div className="mt-3">
      <h3 className="mb-1 text-sm font-semibold">Next steps</h3>
      <dl>
        {heldReply && <CardRow label="Reply">
          <p className="line-clamp-2">{heldReply.replace(/\s+/g, ' ')}</p>
          {!readOnly && <button type="button" onClick={applyReply} className={cardAction}><ArrowTurnBackwardIcon className="h-3.5 w-3.5" />Use this reply</button>}
        </CardRow>}
        {docsGap && <CardRow label="Docs">
          <p>{docsGap}</p>
          {!readOnly && <button type="button" onClick={askToUpdateDocs} className={cardAction}><AskAgentAvatar plateStyle="solid" radius={50} className="h-3.5 w-3.5 shrink-0" />Ask Agent to update docs</button>}
        </CardRow>}
      </dl>
    </div>}
  </div>;
}

export function SupportAIActivity({ message, activity, teammateName, avatarUrl, detailsContent, readOnly = false, folded = false }: {
  message: SupportMessage;
  activity: AIActivity;
  teammateName?: string;
  avatarUrl?: string;
  detailsContent?: ReactNode;
  readOnly?: boolean;
  /** Older handoffs fold to one line once a newer one exists. */
  folded?: boolean;
}) {
  const [expanded, setExpanded] = useState(false);
  const storedName = message.sender_display_name === 'AI control' ? undefined : message.sender_display_name;
  const name = (storedName || teammateName)?.trim();
  const actor = !name || name === 'A teammate' ? 'A teammate' : name.split(/\s+/)[0];
  const details = (label: string) => (
    <details className="mt-2 text-xs text-muted-foreground">
      <summary className="w-fit cursor-pointer rounded-sm font-medium hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">{label}</summary>
      <div className="prose-chat mt-2 text-sm leading-relaxed [overflow-wrap:anywhere]">{detailsContent ?? <p className="whitespace-pre-wrap">{message.content}</p>}</div>
    </details>
  );

  if (activity !== 'handoff') {
    return <div className="my-5 text-xs text-muted-foreground" data-support-ai-activity>
      <div className="flex items-center justify-center">
        <Tooltip>
          <TooltipTrigger asChild>
            <div data-support-system-callout tabIndex={0} className="flex min-w-0 max-w-full items-center gap-2 rounded-full px-3 py-1 [overflow-wrap:anywhere] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
              {avatarUrl ? (
                <img src={avatarUrl} alt={name || actor} className="h-5 w-5 shrink-0 rounded-full object-cover" />
              ) : (
                <div aria-label={name || actor} className={`flex h-5 w-5 shrink-0 items-center justify-center rounded-full text-[9px] font-semibold leading-none ${getAvatarColor(message.sender_user_id || name || actor)}`}>
                  {getInitial(name || actor)}
                </div>
              )}
              <span className="min-w-0">{actor} {activity === 'pause' ? <strong className="font-semibold">Paused AI</strong> : 'returned the conversation to AI'}.</span>
            </div>
          </TooltipTrigger>
          <TooltipContent side="top"><div className="text-xs">{formatTimestamp(message.created_at)}</div></TooltipContent>
        </Tooltip>
      </div>
      {activity === 'return' && <p className="mt-1 text-center">AI will respond to the next customer message.</p>}
      {message.message_type === 'note' && activity === 'pause' && <div className="mx-auto max-w-lg">{details('View original note')}</div>}
      {activity === 'return' && message.content.includes('customer requested a human') && <p className="mt-1 text-center">Confirmed despite the customer’s request for a human.</p>}
    </div>;
  }

  const card = getSupportHandoffCard(message);
  const sections = card ? null : handoffSections(message.content);
  return <SupportAICard kind="handoff" title="AI handoff" timestamp={message.created_at} folded={folded} summary={card ? card.issue || card.reason : sections?.issue}>
    {card ? <SupportHandoffCardBody card={card} conversationId={message.conversation_id} readOnly={readOnly} /> : sections ? <div className="text-sm [overflow-wrap:anywhere]">
      <p className={`whitespace-pre-wrap ${expanded ? '' : 'line-clamp-2'}`}>{sections.issue}</p>
      <details className="mt-2 text-xs text-muted-foreground" onToggle={event => setExpanded(event.currentTarget.open)}>
        <summary className="w-fit cursor-pointer rounded-sm font-medium hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">View details</summary>
        <dl className="mt-2 space-y-2 text-sm text-foreground">
          {sections.checked !== 'No attempted steps recorded.' && <div><dt className="text-xs font-medium text-muted-foreground">What AI checked / suggested</dt><dd className="whitespace-pre-wrap">{sections.checked}</dd></div>}
          <div><dt className="text-xs font-medium text-muted-foreground">Still unresolved</dt><dd className="whitespace-pre-wrap">{sections.next}</dd></div>
          <div><dt className="text-xs font-medium text-muted-foreground">Reason for handoff</dt><dd className="whitespace-pre-wrap">{sections.reason}</dd></div>
        </dl>
      </details>
    </div> : <div className="prose-chat text-sm [overflow-wrap:anywhere]">{detailsContent ?? <p className="whitespace-pre-wrap">{message.content}</p>}</div>}
  </SupportAICard>;
}
