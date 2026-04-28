import type { ComponentPropsWithoutRef } from 'react';
import { Streamdown, defaultRehypePlugins, type Components } from 'streamdown';

import { cn } from '@/lib/utils';

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
        {content}
      </Streamdown>
    </div>
  );
}
