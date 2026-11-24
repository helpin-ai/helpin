import { useState, type FormEvent, type ReactNode } from 'react';
import { useMutation } from '@tanstack/react-query';
import { Input } from '@/components/ui/input';
import { ArrowUp02Icon, ArrowUpRight01Icon } from '@/lib/icons';
import { unwrap } from '@/lib/queryUtils';
import {
  supportQuickstartService,
  type SupportPreviewAnswer,
  type SupportQuickstartDetection,
} from '@/lib/services/supportQuickstartService';
import { cn } from '@/lib/utils';

type Conversation = SupportPreviewAnswer & { from: 'agent' | 'gap' | 'visitor' };

/**
 * The company's chat widget as their customers would see it, with the
 * questions the agent answered from their articles, the ones it found no
 * article for, and a box to ask it anything else.
 */
export function SupportAgentPreview({ site, companyName, helpUrl, answers, gaps, articleCount }: {
  site: SupportQuickstartDetection;
  companyName: string;
  helpUrl: string;
  answers: SupportPreviewAnswer[];
  gaps: string[];
  /** How many article titles the gaps were checked against. */
  articleCount?: number;
}) {
  const [asked, setAsked] = useState<Conversation[]>([]);
  const [selected, setSelected] = useState(0);
  const [question, setQuestion] = useState('');
  const ask = useMutation({
    mutationFn: async (text: string) => unwrap(await supportQuickstartService.previewAnswer({
      website_url: site.website_url, help_center_url: helpUrl || undefined, question: text,
    })),
    onSuccess: (result) => {
      setSelected(answers.length + gaps.length + asked.length);
      setAsked((previous) => [...previous, { ...result, from: 'visitor' }]);
      setQuestion('');
    },
  });

  const conversations: Conversation[] = [
    ...answers.map((answer) => ({ ...answer, from: 'agent' as const })),
    ...gaps.map((gap) => ({ available: true, question: gap, can_answer: false, sources: [], from: 'gap' as const })),
    ...asked,
  ];
  const groups: { label: string; from: Conversation['from'] }[] = [
    { label: articleCount ? 'Answered from your articles' : 'Answered from your pages', from: 'agent' },
    { label: 'No article yet', from: 'gap' },
    { label: 'Your questions', from: 'visitor' },
  ];
  // While a typed question is being answered, the widget shows it waiting.
  const pending = ask.isPending ? ask.variables : undefined;
  const current = pending ? undefined : conversations[selected];
  const brand = site.brand_color || undefined;
  const onBrand = brand ? { background: brand, color: readableTextOn(brand) } : undefined;

  const submit = (event: FormEvent) => {
    event.preventDefault();
    if (question.trim() && !ask.isPending) ask.mutate(question.trim());
  };

  return (
    <div className="space-y-4">
      <div className={cn('grid gap-5', conversations.length > 0 && 'sm:grid-cols-[minmax(0,12.5rem)_minmax(0,1fr)]')}>
        {conversations.length > 0 && (
          <nav aria-label="Example questions" className="space-y-4">
            {groups.map((group) => {
              const items = conversations.map((item, index) => ({ item, index })).filter(({ item }) => item.from === group.from);
              if (items.length === 0) return null;
              return (
                <div key={group.from}>
                  <p className="pb-1 text-[11.5px] font-medium text-muted-foreground">{group.label}</p>
                  <ul className="border-t">
                    {items.map(({ item, index }) => (
                      <li key={`${group.from}-${item.question}`} className="border-b">
                        <button
                          type="button"
                          aria-pressed={!pending && index === selected}
                          onClick={() => setSelected(index)}
                          className={cn(
                            'relative w-full py-2 pl-3 pr-1 text-left text-[13px] leading-5 transition-colors hover:bg-muted/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
                            !pending && index === selected ? 'font-medium text-foreground' : 'text-muted-foreground',
                          )}
                        >
                          {!pending && index === selected && <span aria-hidden="true" className="absolute inset-y-2 left-0 w-[3px] rounded-full bg-foreground" />}
                          <span className="line-clamp-3">{item.question}</span>
                        </button>
                      </li>
                    ))}
                  </ul>
                </div>
              );
            })}
          </nav>
        )}

        <div className="flex flex-col overflow-hidden rounded-[14px] border bg-background shadow-sm">
          <div className="flex items-center gap-2.5 bg-foreground px-4 py-3 text-background" style={onBrand}>
            <BrandMark site={site} name={companyName} className="h-7 w-7 rounded-md text-xs" />
            <div className="min-w-0">
              <p className="truncate text-[13px] font-semibold leading-4">{companyName}</p>
              <p className="text-[11.5px] leading-4 opacity-80">AI agent</p>
            </div>
          </div>
          <div className="min-h-44 flex-1 space-y-3 p-4" aria-live="polite">
            {pending && (
              <>
                <Bubble side="visitor" style={onBrand}>{pending}</Bubble>
                <Bubble side="agent"><TypingDots /></Bubble>
              </>
            )}
            {current && (
              // Keyed so each conversation fades in when it is picked.
              <div key={selected} className="space-y-3 duration-300 animate-in fade-in slide-in-from-bottom-1 motion-reduce:animate-none">
                <Bubble side="visitor" style={onBrand}>{current.question}</Bubble>
                <Bubble side="agent">
                  {current.can_answer && current.answer
                    ? <AnswerText text={current.answer} />
                    : 'I don’t see that in your help center yet, so I’d pass this to your team with the conversation so far.'}
                </Bubble>
                {current.can_answer && current.sources.length > 0 && (
                  <p className="text-[12px] leading-5 text-muted-foreground">
                    Based on{' '}
                    {current.sources.map((source, index) => (
                      <span key={source.url}>
                        {index > 0 && ', '}
                        <a href={source.url} target="_blank" rel="noreferrer noopener" className="underline underline-offset-2 hover:text-foreground">
                          {source.title}
                          <ArrowUpRight01Icon className="ml-0.5 inline h-3 w-3 align-[-1px]" aria-hidden="true" />
                        </a>
                      </span>
                    ))}
                  </p>
                )}
              </div>
            )}
            {!pending && !current && (
              <p className="text-[12.5px] leading-5 text-muted-foreground">Ask the kind of question your customers send you.</p>
            )}
          </div>
          <form onSubmit={submit} className="flex items-center gap-2 border-t px-3 py-2.5">
            <Input
              value={question}
              onChange={(event) => setQuestion(event.target.value)}
              placeholder="Ask it something a customer would…"
              aria-label="Customer question"
              maxLength={300}
              className="h-9 border-transparent px-1 shadow-none focus-visible:border-transparent focus-visible:ring-0"
            />
            {/* A plain button: the page shell makes every Button a 44px bar, which a widget's send button is not. */}
            <button
              type="submit"
              aria-label="Ask"
              disabled={ask.isPending || !question.trim()}
              className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-foreground text-background transition-opacity disabled:opacity-40"
            >
              <ArrowUp02Icon className="h-4 w-4" aria-hidden="true" />
            </button>
          </form>
          {ask.error && <p role="alert" className="px-4 pb-3 text-[12.5px] text-destructive">{ask.error.message}</p>}
        </div>
      </div>

      {gaps.length > 0 && (
        <p className="text-[12.5px] leading-5 text-muted-foreground">
          “No article yet” lists common questions we didn’t find among your {articleCount ? `${articleCount} ` : ''}article titles.
          Once you’re live, Helpin groups the questions your agent couldn’t answer and can draft the missing article for your review.
        </p>
      )}
    </div>
  );
}

function Bubble({ side, style, children }: { side: 'visitor' | 'agent'; style?: { background: string; color: string }; children: ReactNode }) {
  return (
    <div
      className={cn(
        'w-fit max-w-[88%] rounded-xl px-3.5 py-2 text-[13.5px] leading-[1.55]',
        side === 'visitor' ? 'ml-auto bg-foreground text-background' : 'bg-muted',
      )}
      style={side === 'visitor' ? style : undefined}
    >
      {children}
    </div>
  );
}

function TypingDots() {
  return (
    <span role="status" aria-label="Your agent is answering" className="flex h-5 items-center gap-1">
      {[0, 150, 300].map((delay) => (
        <span key={delay} className="h-1.5 w-1.5 animate-bounce rounded-full bg-muted-foreground/70 motion-reduce:animate-none" style={{ animationDelay: `${delay}ms` }} />
      ))}
    </span>
  );
}

/** The agent's answer: paragraphs, short lists and bold, which is all it is allowed to write. */
function AnswerText({ text }: { text: string }) {
  const blocks: ReactNode[] = [];
  let list: { ordered: boolean; items: string[] } | null = null;
  const endList = () => {
    if (!list) return;
    const items = list.items.map((item, index) => <li key={index}>{withBold(item)}</li>);
    blocks.push(list.ordered
      ? <ol key={blocks.length} className="list-decimal space-y-1 pl-5">{items}</ol>
      : <ul key={blocks.length} className="list-disc space-y-1 pl-5">{items}</ul>);
    list = null;
  };
  for (const raw of text.split('\n')) {
    const line = raw.trim();
    const item = /^(?:[-*]|(\d+)[.)])\s+(.*)$/.exec(line);
    if (item) {
      const ordered = item[1] !== undefined;
      if (!list || list.ordered !== ordered) {
        endList();
        list = { ordered, items: [] };
      }
      list.items.push(item[2]);
      continue;
    }
    endList();
    if (line) blocks.push(<p key={blocks.length}>{withBold(line)}</p>);
  }
  endList();
  return <div className="space-y-2">{blocks}</div>;
}

function withBold(text: string) {
  return text.split(/\*\*(.+?)\*\*/g).map((part, index) => (index % 2 ? <strong key={index} className="font-semibold">{part}</strong> : part));
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
