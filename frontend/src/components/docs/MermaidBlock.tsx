import { useEffect, useState } from 'react';
import { Alert01Icon, Loading01Icon } from '@/lib/icons';
import { renderMermaidSvg } from '@/lib/mermaidRenderer';

type MermaidBlockProps = {
  source: string;
};

type RenderState =
  | { status: 'idle' | 'loading'; svg?: never; error?: never }
  | { status: 'ready'; svg: string; error?: never }
  | { status: 'error'; svg?: never; error: string };

export function MermaidBlock({ source }: MermaidBlockProps) {
  const [state, setState] = useState<RenderState>({ status: 'idle' });

  useEffect(() => {
    let cancelled = false;
    const trimmed = source.trim();
    if (!trimmed) {
      setState({ status: 'error', error: 'Mermaid diagram is empty.' });
      return;
    }

    setState({ status: 'loading' });
    renderMermaidSvg(trimmed)
      .then((svg) => {
        if (!cancelled) setState({ status: 'ready', svg });
      })
      .catch((err) => {
        if (!cancelled) {
          setState({
            status: 'error',
            error: err instanceof Error ? err.message : 'Unable to render Mermaid diagram.',
          });
        }
      });

    return () => {
      cancelled = true;
    };
  }, [source]);

  if (state.status === 'loading' || state.status === 'idle') {
    return (
      <div className="flex min-h-40 items-center justify-center gap-2 text-sm text-muted-foreground">
        <Loading01Icon className="h-4 w-4 animate-spin" />
        Rendering diagram...
      </div>
    );
  }

  if (state.status === 'error') {
    return (
      <div className="flex min-h-32 items-center gap-3 rounded-md border border-amber-500/30 bg-amber-500/5 px-4 py-3 text-sm text-amber-700 dark:text-amber-300">
        <Alert01Icon className="h-4 w-4 shrink-0" />
        <p className="min-w-0 break-words">{state.error}</p>
      </div>
    );
  }

  return (
    <div
      className="docs-mermaid-preview overflow-auto px-4 py-5 [&_svg]:mx-auto [&_svg]:h-auto [&_svg]:max-w-full"
      dangerouslySetInnerHTML={{ __html: state.svg ?? '' }}
    />
  );
}
