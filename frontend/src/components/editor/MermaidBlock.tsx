import { useEffect, useState } from 'react';
import { Alert01Icon, Loading01Icon } from '@/lib/icons';
import { getCachedMermaidDiagram, renderMermaidDiagram } from './mermaidPreviewCache';

type MermaidBlockProps = {
  source: string;
};

type RenderState =
  | { status: 'idle' | 'loading'; source: string; svg?: never; error?: never }
  | { status: 'ready'; source: string; svg: string; error?: never }
  | { status: 'error'; source: string; svg?: never; error: string };

export function MermaidBlock({ source }: MermaidBlockProps) {
  const trimmedSource = source.trim();
  const [state, setState] = useState<RenderState>(() => {
    const cached = getCachedMermaidDiagram(trimmedSource);
    return cached
      ? { status: 'ready', source: trimmedSource, svg: cached }
      : { status: 'idle', source: trimmedSource };
  });
  const hasCurrentDiagram = state.status === 'ready' && state.source === trimmedSource;

  useEffect(() => {
    let cancelled = false;
    if (!trimmedSource || hasCurrentDiagram) return;

    renderMermaidDiagram(trimmedSource)
      .then((svg) => {
        if (!cancelled) setState({ status: 'ready', source: trimmedSource, svg });
      })
      .catch((err) => {
        if (!cancelled) {
          setState({
            status: 'error',
            source: trimmedSource,
            error: err instanceof Error ? err.message : 'Unable to render Mermaid diagram.',
          });
        }
      });

    return () => {
      cancelled = true;
    };
  }, [hasCurrentDiagram, trimmedSource]);

  if (!trimmedSource) {
    return (
      <div
        className="flex min-h-40 items-center gap-3 rounded-md border border-amber-500/30 bg-amber-500/5 px-4 py-3 text-sm text-amber-700 dark:text-amber-300"
        data-mermaid-status="error"
      >
        <Alert01Icon className="h-4 w-4 shrink-0" />
        <p className="min-w-0 break-words">Mermaid diagram is empty.</p>
      </div>
    );
  }

  if (state.source !== trimmedSource || state.status === 'loading' || state.status === 'idle') {
    return (
      <div
        className="flex min-h-40 items-center justify-center gap-2 text-sm text-muted-foreground"
        data-mermaid-status="loading"
      >
        <Loading01Icon className="h-4 w-4 animate-spin" />
        Rendering diagram...
      </div>
    );
  }

  if (state.status === 'error') {
    return (
      <div
        className="flex min-h-40 items-center gap-3 rounded-md border border-amber-500/30 bg-amber-500/5 px-4 py-3 text-sm text-amber-700 dark:text-amber-300"
        data-mermaid-status="error"
      >
        <Alert01Icon className="h-4 w-4 shrink-0" />
        <p className="min-w-0 break-words">{state.error}</p>
      </div>
    );
  }

  return (
    <div
      className="editor-mermaid-preview min-h-40 overflow-auto px-4 py-5 [overflow-anchor:none] [&_svg]:mx-auto [&_svg]:h-auto [&_svg]:max-w-full"
      data-mermaid-status="ready"
      dangerouslySetInnerHTML={{ __html: state.svg ?? '' }}
    />
  );
}
