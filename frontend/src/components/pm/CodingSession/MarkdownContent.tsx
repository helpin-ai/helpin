import type { ComponentPropsWithoutRef, MouseEvent } from 'react';
import { Streamdown, defaultRehypePlugins, defaultRemarkPlugins, type Components, type ExtraProps } from 'streamdown';
import { toast } from 'sonner';

import { automationService } from '@/lib/services/automationService';
import {
  helpinReferenceRoute,
  parseHelpinReferenceMarker,
  remarkHelpinReferences,
} from '@/lib/helpinReferences';
import { cn } from '@/lib/utils';
import { useWorkspaceStore } from '@/stores/workspaceStore';

function MarkdownLink(props: (ComponentPropsWithoutRef<'a'> | Record<string, unknown>) & ExtraProps) {
  const { children, href } = props as ComponentPropsWithoutRef<'a'>;
  const workspace = useWorkspaceStore((state) => state.currentWorkspace);
  const reference = parseHelpinReferenceMarker(href);
  if (!reference) {
    return (
      <a href={href} target="_blank" rel="noreferrer" className="text-primary underline underline-offset-2">
        {children}
      </a>
    );
  }

  if (reference.type === 'artifacts') {
    const openArtifact = async (event: MouseEvent<HTMLAnchorElement>) => {
      event.preventDefault();
      if (!workspace?.id) return;
      const preview = window.open('about:blank', '_blank');
      if (preview) preview.opener = null;
      const response = await automationService.getArtifactContentURL(workspace.id, reference.id);
      if (response.error || !response.data?.url) {
        preview?.close();
        toast.error(response.error || 'Unable to open artifact');
        return;
      }
      if (preview) {
        preview.location.replace(response.data.url);
      } else {
        window.open(response.data.url, '_blank', 'noopener,noreferrer');
      }
    };
    return (
      <a
        href={href}
        data-helpin-reference="artifacts"
        onClick={(event) => void openArtifact(event)}
        className="text-primary underline underline-offset-2"
      >
        {children}
      </a>
    );
  }

  const route = workspace?.slug ? helpinReferenceRoute(reference, workspace.slug) : null;
  if (!route) return <span className="text-muted-foreground">{children}</span>;
  return (
    <a
      href={route}
      data-helpin-reference={reference.type}
      className="text-primary underline underline-offset-2"
    >
      {children}
    </a>
  );
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
  a: MarkdownLink,
  inlineCode: ({ children, node, ...props }: ComponentPropsWithoutRef<'code'> & { node?: unknown }) => {
    void node;
    return (
      <code className="rounded bg-muted px-1 py-0.5 text-[12px]" {...props}>
        {children}
      </code>
    );
  },
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
  const markdownRemarkPlugins = [
    ...Object.values(defaultRemarkPlugins),
    remarkHelpinReferences,
  ];
  return (
    <div className={cn('text-sm leading-6 text-foreground', className)}>
      <Streamdown
        className="[&>*+*]:!mt-0"
        components={markdownComponents}
        controls={{ code: { copy: true, download: false } }}
        shikiTheme={['github-light', 'github-dark']}
        lineNumbers={false}
        mode={streaming ? 'streaming' : 'static'}
        isAnimating={streaming}
        animated={{ animation: 'fadeIn', sep: 'word', duration: 300, stagger: 12 }}
        parseIncompleteMarkdown={streaming}
        rehypePlugins={[
          defaultRehypePlugins.sanitize,
          defaultRehypePlugins.harden,
        ]}
        remarkPlugins={markdownRemarkPlugins}
      >
        {content}
      </Streamdown>
    </div>
  );
}
