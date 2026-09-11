import { useEffect, useState } from 'react';
import { useTheme } from 'next-themes';
import { Alert01Icon, Loading01Icon } from '@/lib/icons';
import { renderNwdiagSvg } from '@/lib/nwdiagRenderer';

type RenderState = { key: string; svg?: string; error?: string };

export function NwdiagBlock({ source }: { source: string }) {
  const { resolvedTheme } = useTheme();
  const theme = resolvedTheme === 'dark' ? 'dark' : 'light';
  const trimmedSource = source.trim();
  const key = `${theme}:${trimmedSource}`;
  const [state, setState] = useState<RenderState>();

  useEffect(() => {
    let cancelled = false;
    if (!trimmedSource) return;
    void renderNwdiagSvg(trimmedSource, theme).then(
      (svg) => { if (!cancelled) setState({ key, svg }); },
      (error: unknown) => {
        if (!cancelled) setState({ key, error: error instanceof Error ? error.message : 'Unable to render nwdiag diagram.' });
      },
    );
    return () => { cancelled = true; };
  }, [key, theme, trimmedSource]);

  const error = !trimmedSource ? 'nwdiag diagram is empty.' : state?.key === key ? state.error : undefined;
  if (error) {
    return (
      <div role="alert" data-nwdiag-status="error" className="flex min-h-40 items-center gap-3 px-4 py-3 text-sm text-quiet-accent">
        <Alert01Icon className="h-4 w-4 shrink-0" />
        <p className="min-w-0 whitespace-pre-wrap break-words">{error}</p>
      </div>
    );
  }
  if (state?.key !== key || !state.svg) {
    return (
      <div role="status" data-nwdiag-status="loading" className="flex min-h-40 items-center justify-center gap-2 text-sm text-muted-foreground">
        <Loading01Icon className="h-4 w-4 animate-spin" />
        Rendering diagram...
      </div>
    );
  }
  return (
    <div
      role="img"
      aria-label="nwdiag network diagram"
      data-nwdiag-status="ready"
      className="min-h-40 overflow-auto px-4 py-5 [overflow-anchor:none] [&_svg]:mx-auto [&_svg]:h-auto [&_svg]:max-w-full"
      dangerouslySetInnerHTML={{ __html: state.svg }}
    />
  );
}
