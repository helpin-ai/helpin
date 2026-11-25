import { useTheme } from 'next-themes';
import { WidgetPreview } from '@/components/settings/WidgetPreview';
import type { SupportQuickstartDetection } from '@/lib/services/supportQuickstartService';
import { cn } from '@/lib/utils';
import type { AgentPreview, PreviewConversation } from './useAgentPreview';

/** The widget's color when setup finds no brand color to give it. */
const DEFAULT_WIDGET_COLOR = '#6366f1';

/** The questions the preview can show, grouped by where each came from. */
export function PreviewQuestions({ preview, articleCount }: {
  preview: AgentPreview;
  /** How many article titles the gaps were checked against. */
  articleCount?: number;
}) {
  const { conversations, current, pending, select } = preview;
  if (conversations.length === 0) return null;
  const groups: { label: string; from: PreviewConversation['from'] }[] = [
    { label: articleCount ? 'Answered from your articles' : 'Answered from your pages', from: 'agent' },
    { label: 'No article yet', from: 'gap' },
    { label: 'Your questions', from: 'visitor' },
  ];
  return (
    <nav aria-label="Example questions" className="space-y-4">
      {groups.map((group) => {
        const items = conversations.filter((item) => item.from === group.from);
        if (items.length === 0) return null;
        return (
          <div key={group.from}>
            <p className="pb-1 text-[11.5px] font-medium text-muted-foreground">{group.label}</p>
            <ul className="border-t">
              {items.map((item) => {
                const showing = !pending && item === current;
                return (
                  <li key={item.question} className="border-b">
                    <button
                      type="button"
                      aria-pressed={showing}
                      onClick={() => select(item.question)}
                      className={cn(
                        'relative w-full py-2 pl-3 pr-1 text-left text-[13px] leading-5 transition-colors hover:bg-muted/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
                        showing ? 'font-medium text-foreground' : 'text-muted-foreground',
                      )}
                    >
                      {showing && <span aria-hidden="true" className="absolute inset-y-2 left-0 w-[3px] rounded-full bg-foreground" />}
                      <span className="line-clamp-3">{item.question}</span>
                    </button>
                  </li>
                );
              })}
            </ul>
          </div>
        );
      })}
      <p className="text-[12.5px] leading-5 text-muted-foreground">Pick a question, or ask your own in the chat.</p>
      {conversations.some((item) => item.from === 'gap') && (
        <p className="text-[12.5px] leading-5 text-muted-foreground">
          “No article yet” lists common questions we didn’t find among your {articleCount ? `${articleCount} ` : ''}article titles.
          Once you’re live, Helpin groups the questions your agent couldn’t answer and can draft the missing article for your review.
        </p>
      )}
    </nav>
  );
}

/**
 * The company's chat widget as their customers will see it: the real widget,
 * with the icon and color setup will give it.
 */
export function PreviewWidget({ site, companyName, host, preview }: {
  site: SupportQuickstartDetection;
  companyName: string;
  host: string;
  preview: AgentPreview;
}) {
  const { resolvedTheme } = useTheme();
  // Setup gives the widget the site's icon and its brand color, unless that is plain black.
  const mark = site.favicon_url || undefined;
  const brandColor = site.brand_color && site.brand_color !== '#000000' ? site.brand_color : DEFAULT_WIDGET_COLOR;
  return (
    <figure className="space-y-3 duration-500 animate-in fade-in slide-in-from-bottom-2 motion-reduce:animate-none">
      <div className="h-[min(40rem,calc(100svh-9rem))] min-h-[26rem]">
        <WidgetPreview
          bare
          initialView="conversation"
          colorScheme={resolvedTheme === 'dark' ? 'dark' : 'light'}
          brandColor={brandColor}
          logoUrl={mark}
          avatarUrl={mark}
          workspaceName={companyName}
          showBranding
          launcherPosition="bottom_right"
          launcherIcon="chat_bubble"
          welcomeMessage=""
          aiFirst
          messages={preview.messages}
          isAIThinking={preview.thinking}
          onSendMessage={preview.send}
        />
      </div>
      <figcaption className="text-center text-[12px] leading-5 text-muted-foreground">Your chat widget, as customers will see it on {host}.</figcaption>
      {preview.error && <p role="alert" className="text-center text-[12.5px] text-destructive">{preview.error}</p>}
    </figure>
  );
}

/** The company's icon in a square, or its initial on the brand color. */
export function BrandMark({ site, name, className }: { site: SupportQuickstartDetection; name: string; className?: string }) {
  // The square holds an icon; a wordmark logo only when there is no icon.
  const mark = site.favicon_url || site.logo_url;
  return (
    <span
      aria-hidden="true"
      className={cn('flex shrink-0 items-center justify-center overflow-hidden rounded-[10px] border font-semibold', mark ? 'bg-white' : 'bg-foreground text-background', className)}
      style={!mark && site.brand_color ? { background: site.brand_color, color: readableTextOn(site.brand_color) } : undefined}
    >
      {mark ? <img src={mark} alt="" className="max-h-[75%] max-w-[75%] object-contain" /> : (name.trim()[0] ?? '?').toUpperCase()}
    </span>
  );
}

/** Black or white, whichever reads better on a brand color such as #ff5500. */
function readableTextOn(hex: string) {
  const match = /^#?([0-9a-f]{6})$/i.exec(hex.trim());
  if (!match) return '#fff';
  const value = parseInt(match[1], 16);
  const [r, g, b] = [(value >> 16) & 255, (value >> 8) & 255, value & 255];
  return (0.299 * r + 0.587 * g + 0.114 * b) / 255 > 0.6 ? '#111' : '#fff';
}
