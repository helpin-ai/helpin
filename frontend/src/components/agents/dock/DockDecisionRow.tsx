import { Cancel01Icon, ClipboardIcon, Tick01Icon } from '@/lib/icons';
import type { CodingSessionActor, CodingSessionTranscriptMessage } from '@/lib/pmTypes';
import { MarkdownContent } from '@/components/pm/CodingSession/MarkdownContent';
import { cn } from '@/lib/utils';

/** Decisions belong to the activity history, not to human chat bubbles. */
export function DockDecisionRow({ message, actor, timeline = false }: {
  message: CodingSessionTranscriptMessage;
  actor: CodingSessionActor | null;
  timeline?: boolean;
}) {
  const content = message.content.trim();
  const approved = message.message_type === 'approval' || /^approved\b/i.test(content);
  const declined = /^(declined|rejected|denied|skipped)\b/i.test(content);
  const label = approved ? 'approved' : declined ? 'declined'
    : /^requested changes\b/i.test(content) ? 'requested changes' : 'reviewed';
  const name = actor?.full_name || actor?.email;
  const summary = name ? `${name} ${label}` : label.charAt(0).toUpperCase() + label.slice(1);
  const details = /^(approved\.?\s*(continue\.?)?|approve)$/i.test(content) ? '' : content;
  const Icon = approved ? Tick01Icon : declined ? Cancel01Icon : ClipboardIcon;
  const marker = <span className={`grid h-5 w-5 shrink-0 place-items-center ${approved ? 'text-quiet-positive' : ''}`}><Icon className="h-3.5 w-3.5" aria-hidden="true" /></span>;

  return (
    <div className={cn('text-xs text-muted-foreground', timeline ? 'py-1.5' : 'py-2')} data-dock-decision={label}>
      {details ? (
        <details>
          <summary className={cn('flex cursor-pointer items-center rounded-sm focus-visible:outline-2 focus-visible:outline-ring', timeline ? 'min-h-5 gap-2.5' : 'min-h-8 gap-2')}>
            {marker}<span>{summary}</span><span className="ml-auto text-[11px]">Details</span>
          </summary>
          <div className="mt-1 border-l border-border pl-5">
            <MarkdownContent content={details} className="text-xs" />
          </div>
        </details>
      ) : <div className={cn('flex items-center', timeline ? 'min-h-5 gap-2.5' : 'min-h-8 gap-2')}>{marker}<span>{summary}</span></div>}
    </div>
  );
}
