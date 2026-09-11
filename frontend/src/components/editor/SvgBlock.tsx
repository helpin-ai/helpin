import { useMemo } from 'react';
import { Alert01Icon } from '@/lib/icons';
import { sanitizeSvg, svgDataUrl } from '@/lib/svgRenderer';

export function SvgBlock({ source }: { source: string }) {
  const result = useMemo(() => {
    try {
      const svg = sanitizeSvg(source);
      const title = new DOMParser().parseFromString(svg, 'image/svg+xml').querySelector('title')?.textContent;
      return { src: svgDataUrl(svg), title: title || 'SVG diagram' };
    } catch (error) {
      return { error: error instanceof Error ? error.message : 'Unable to render SVG diagram.' };
    }
  }, [source]);
  if (result.error) {
    return <div role="alert" className="flex min-h-40 items-center gap-3 px-4 py-3 text-sm text-quiet-accent">
      <Alert01Icon className="h-4 w-4 shrink-0" />
      <p className="min-w-0 whitespace-pre-wrap break-words">{result.error}</p>
    </div>;
  }
  return <div className="min-h-40 overflow-auto bg-white px-4 py-5 [overflow-anchor:none]">
    <img src={result.src} alt={result.title} className="mx-auto h-auto max-w-full" />
  </div>;
}
