import { useState, type ReactNode } from 'react';
import type { AIActivity } from './supportAIActivity';
import type { SupportMessage } from '@/lib/pmTypes';
import { BotIcon } from '@/lib/icons';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { formatMessageTime, formatTimestamp, getAvatarColor, getInitial } from './helpers';

function handoffSections(content: string) {
  const match = content.match(/^AI handoff\n\nIssue\n([\s\S]*?)\n\nAlready tried \/ suggested\n([\s\S]*?)\n\nStill unresolved\n([\s\S]*?)\n\nReason for handoff\n([\s\S]*)$/);
  return match ? { issue: match[1], checked: match[2], next: match[3], reason: match[4] } : null;
}

export function SupportAIActivity({ message, activity, teammateName, avatarUrl, detailsContent }: {
  message: SupportMessage;
  activity: AIActivity;
  teammateName?: string;
  avatarUrl?: string;
  detailsContent?: ReactNode;
}) {
  const [expanded, setExpanded] = useState(false);
  const storedName = message.sender_display_name === 'AI control' ? undefined : message.sender_display_name;
  const name = (storedName || teammateName)?.trim();
  const actor = !name || name === 'A teammate' ? 'A teammate' : name.split(/\s+/)[0];
  const time = <time dateTime={message.created_at} title={formatTimestamp(message.created_at)} className="shrink-0 text-[11px] text-muted-foreground">{formatMessageTime(message.created_at)}</time>;
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

  const sections = handoffSections(message.content);
  return <div className="my-4 flex justify-end" data-support-ai-handoff>
    <section aria-label="AI handoff summary" className="w-full max-w-lg rounded-lg border border-amber-200/70 bg-amber-50/60 p-3 dark:border-amber-900/50 dark:bg-amber-950/20">
      <div className="mb-2 flex items-center gap-2">
        <BotIcon className="size-3.5 text-amber-600 dark:text-amber-400" aria-hidden="true" />
        <span className="text-xs font-semibold">AI handoff</span>
        <span className="text-[11px] text-muted-foreground">Team only</span>
        <span className="ml-auto">{time}</span>
      </div>
      {sections ? <div className="text-sm [overflow-wrap:anywhere]">
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

    </section>
  </div>;
}
