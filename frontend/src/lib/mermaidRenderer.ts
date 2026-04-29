let renderCounter = 0;
let initialized = false;

async function getMermaid() {
  const mod = await import('mermaid');
  const mermaid = mod.default;
  if (!initialized) {
    mermaid.initialize({
      startOnLoad: false,
      securityLevel: 'strict',
      theme: 'default',
      fontFamily: 'Inter, ui-sans-serif, system-ui, sans-serif',
    });
    initialized = true;
  }
  return mermaid;
}

export async function renderMermaidSvg(source: string): Promise<string> {
  const trimmed = source.trim();
  if (!trimmed) {
    throw new Error('Mermaid diagram is empty');
  }

  const mermaid = await getMermaid();
  const id = `docs-mermaid-${Date.now()}-${renderCounter++}`;
  const result = await mermaid.render(id, trimmed);
  return result.svg;
}

export function mermaidSvgFile(svg: string, filename = 'mermaid-diagram.svg'): File {
  return new File([svg], filename, { type: 'image/svg+xml' });
}
