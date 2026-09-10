import DOMPurify from 'dompurify';

export const NWDIAG_EXAMPLE = `nwdiag {
  network dmz {
    address = "10.0.0.0/24";
    web01 [address = ".10"];
    web02 [address = ".11"];
  }
}`;

export async function renderNwdiagSvg(
  source: string,
  theme: 'light' | 'dark' = 'light',
  options: { brandColor?: string } = {},
): Promise<string> {
  if (!source.trim()) throw new Error('nwdiag diagram is empty.');
  const { renderFromSource } = await import('simplediag');
  const brand = /^#[0-9a-f]{6}$/i.test(options.brandColor ?? '') ? options.brandColor : undefined;
  const dark = theme === 'dark';
  const result = renderFromSource(source.trim(), {
    errorMode: 'null',
    theme: {
      colors: {
        background: 'transparent',
        text: dark ? '#f8fafc' : '#1c1a17',
        mutedText: dark ? '#cbd5e1' : '#57534e',
        nodeFill: dark ? '#1f2937' : '#ffffff',
        nodeStroke: dark ? '#cbd5e1' : '#57534e',
        railFill: dark ? '#334155' : '#e9e8e5',
        railStroke: brand ?? (dark ? '#94a3b8' : '#78716c'),
        groupFill: dark ? '#334155' : '#f0efec',
        groupStroke: dark ? '#64748b' : '#a8a5a0',
        linkStroke: dark ? '#cbd5e1' : '#57534e',
      },
    },
  });
  if (!result.svg || result.diagnostics.some((diagnostic) => diagnostic.severity === 'error')) {
    throw new Error(result.diagnostics.filter((diagnostic) => diagnostic.severity === 'error')
      .map((diagnostic) => diagnostic.loc
        ? `Line ${diagnostic.loc.start.line}: ${diagnostic.message}`
        : diagnostic.message).join('\n') || 'Unable to render nwdiag diagram.');
  }
  return DOMPurify.sanitize(result.svg, { USE_PROFILES: { svg: true, svgFilters: true } });
}
