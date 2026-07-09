import { useEffect, useRef, useState, type ComponentPropsWithoutRef } from 'react';
import { Streamdown, defaultRehypePlugins, type Components } from 'streamdown';

import { cn } from '@/lib/utils';

/**
 * Reveals streamed text at a steady per-frame rate so large network deltas
 * don't pop in as chunks — the smooth "typing" feel of ChatGPT/Claude.
 *
 * The reveal decouples the on-screen cadence from the delta cadence: it eases
 * toward the target (faster when it's far behind so it never lags noticeably),
 * and snaps instantly when streaming ends or the text is replaced (not appended,
 * e.g. a completion that rewrites the message). Streamdown's incomplete-markdown
 * handling keeps partial prefixes from flickering formatting.
 */
function useSmoothText(target: string, enabled: boolean): string {
  const [revealed, setRevealed] = useState(enabled ? '' : target);
  const revealedRef = useRef(revealed);
  const rafRef = useRef<number | null>(null);
  revealedRef.current = revealed;

  useEffect(() => {
    const cancel = () => {
      if (rafRef.current != null) {
        cancelAnimationFrame(rafRef.current);
        rafRef.current = null;
      }
    };

    // Not streaming, or the target diverged from what we've shown (a completion
    // that replaced the text rather than extended it): show it all immediately.
    if (!enabled || !target.startsWith(revealedRef.current)) {
      cancel();
      if (revealedRef.current !== target) setRevealed(target);
      return cancel;
    }

    const step = () => {
      const current = revealedRef.current;
      if (current.length >= target.length) {
        rafRef.current = null;
        return;
      }
      const remaining = target.length - current.length;
      // Ease-out: reveal ~1/8th of the backlog per frame (min 2 chars) so a big
      // burst catches up quickly, a trickle reveals one-at-a-time smoothly.
      const chars = Math.max(2, Math.ceil(remaining / 8));
      setRevealed(target.slice(0, current.length + chars));
      rafRef.current = requestAnimationFrame(step);
    };
    if (rafRef.current == null) rafRef.current = requestAnimationFrame(step);
    return cancel;
  }, [target, enabled]);

  return enabled ? revealed : target;
}

const markdownComponents: Components = {
  p: ({ children }) => <p className="mb-3 last:mb-0">{children}</p>,
  ul: ({ children }) => <ul className="mb-3 list-disc space-y-1 pl-5 last:mb-0">{children}</ul>,
  ol: ({ children }) => <ol className="mb-3 list-decimal space-y-1 pl-5 last:mb-0">{children}</ol>,
  li: ({ children }) => <li>{children}</li>,
  strong: ({ children }) => <strong>{children}</strong>,
  em: ({ children }) => <em>{children}</em>,
  h1: ({ children }) => <h1 className="mb-3 text-base font-semibold last:mb-0">{children}</h1>,
  h2: ({ children }) => <h2 className="mb-3 text-sm font-semibold last:mb-0">{children}</h2>,
  h3: ({ children }) => <h3 className="mb-2 text-sm font-semibold last:mb-0">{children}</h3>,
  table: ({ children }) => (
    <div className="mb-3 overflow-x-auto rounded-md border border-border/60 last:mb-0">
      <table className="min-w-full border-collapse">{children}</table>
    </div>
  ),
  thead: ({ children }) => <thead className="bg-muted/50">{children}</thead>,
  tbody: ({ children }) => <tbody>{children}</tbody>,
  tr: ({ children }) => <tr className="border-b border-border/60 last:border-b-0">{children}</tr>,
  th: ({ children }) => <th className="px-3 py-2 text-left text-[12px] font-semibold text-foreground">{children}</th>,
  td: ({ children }) => <td className="px-3 py-2 align-top text-sm leading-5 text-foreground">{children}</td>,
  blockquote: ({ children }) => (
    <blockquote className="mb-3 border-l-2 border-border pl-3 text-muted-foreground last:mb-0">
      {children}
    </blockquote>
  ),
  a: ({ children, href }) => (
    <a
      href={href}
      target="_blank"
      rel="noreferrer"
      className="text-primary underline underline-offset-2"
    >
      {children}
    </a>
  ),
  pre: ({ children }) => (
    <pre className="mb-3 overflow-x-auto rounded-md bg-zinc-950 px-3 py-2 text-[12px] leading-5 text-zinc-50 last:mb-0">
      {children}
    </pre>
  ),
  code: ({ className: codeClassName, children, node: _node, ...props }: ComponentPropsWithoutRef<'code'> & { node?: unknown }) => (
    <code className={cn('text-[12px]', codeClassName)} {...props}>
      {children}
    </code>
  ),
  inlineCode: ({ children, node: _node, ...props }: ComponentPropsWithoutRef<'code'> & { node?: unknown }) => (
    <code className="rounded bg-muted px-1 py-0.5 text-[12px]" {...props}>
      {children}
    </code>
  ),
};

export function MarkdownContent({
  content,
  className,
  streaming = false,
}: {
  content: string;
  className?: string;
  streaming?: boolean;
}) {
  const shown = useSmoothText(content, streaming);
  return (
    <div className={cn('text-sm leading-6 text-foreground', className)}>
      <Streamdown
        className="[&>*+*]:!mt-0"
        components={markdownComponents}
        controls={false}
        lineNumbers={false}
        mode={streaming ? 'streaming' : 'static'}
        isAnimating={streaming}
        parseIncompleteMarkdown={streaming}
        rehypePlugins={[
          defaultRehypePlugins.sanitize,
          defaultRehypePlugins.harden,
        ]}
      >
        {shown}
      </Streamdown>
    </div>
  );
}
