let renderCounter = 0;
let renderQueue: Promise<void> = Promise.resolve();

export type MermaidExportTheme = 'light' | 'dark';
export type MermaidRenderOptions = {
  brandColor?: string;
};

function normalizeHexColor(value: string | undefined): string | null {
  if (!value) return null;
  const trimmed = value.trim();
  const match = /^#?([0-9a-f]{6})$/i.exec(trimmed);
  return match ? `#${match[1].toLowerCase()}` : null;
}

function mixHex(color: string, target: string, amount: number): string {
  const c = normalizeHexColor(color) ?? '#3b82f6';
  const t = normalizeHexColor(target) ?? '#ffffff';
  const parts = [c, t].map((hex) => ({
    r: parseInt(hex.slice(1, 3), 16),
    g: parseInt(hex.slice(3, 5), 16),
    b: parseInt(hex.slice(5, 7), 16),
  }));
  const [source, destination] = parts;
  const channel = (from: number, to: number) => Math.round(from + (to - from) * amount).toString(16).padStart(2, '0');
  return `#${channel(source.r, destination.r)}${channel(source.g, destination.g)}${channel(source.b, destination.b)}`;
}

function themeVariables(theme: MermaidExportTheme, options: MermaidRenderOptions = {}): Record<string, string> {
  const brand = normalizeHexColor(options.brandColor) ?? '#3b82f6';
  const darkBrand = mixHex(brand, '#ffffff', 0.35);

  if (theme === 'dark') {
    return {
      background: 'transparent',
      mainBkg: '#111827',
      primaryColor: '#111827',
      primaryBorderColor: darkBrand,
      primaryTextColor: '#f8fafc',
      secondaryColor: '#1f2937',
      secondaryTextColor: '#f8fafc',
      tertiaryColor: '#0f172a',
      lineColor: darkBrand,
      textColor: '#f8fafc',
      edgeLabelBackground: 'transparent',
      clusterBkg: 'transparent',
      clusterBorder: '#475569',
    };
  }

  return {
    background: 'transparent',
    mainBkg: mixHex(brand, '#ffffff', 0.94),
    primaryColor: mixHex(brand, '#ffffff', 0.94),
    primaryBorderColor: brand,
    primaryTextColor: '#0f172a',
    secondaryColor: '#f8fafc',
    secondaryTextColor: '#0f172a',
    tertiaryColor: '#f1f5f9',
    lineColor: mixHex(brand, '#0f172a', 0.2),
    textColor: '#0f172a',
    edgeLabelBackground: 'transparent',
    clusterBkg: 'transparent',
    clusterBorder: '#cbd5e1',
  };
}

async function getMermaid(theme: MermaidExportTheme, options?: MermaidRenderOptions) {
  const mod = await import('mermaid');
  const mermaid = mod.default;
  mermaid.initialize({
    startOnLoad: false,
    securityLevel: 'strict',
    theme: 'base',
    fontFamily: 'Inter, ui-sans-serif, system-ui, sans-serif',
    themeVariables: themeVariables(theme, options),
  });
  return mermaid;
}

async function queuedRender<T>(render: () => Promise<T>): Promise<T> {
  const previous = renderQueue;
  let release: () => void = () => {};
  renderQueue = new Promise<void>((resolve) => {
    release = resolve;
  });
  await previous.catch(() => undefined);
  try {
    return await render();
  } finally {
    release();
  }
}

function createRenderHost(): HTMLDivElement {
  const host = document.createElement('div');
  host.dataset.mermaidRenderHost = '';
  host.setAttribute('aria-hidden', 'true');
  Object.assign(host.style, {
    position: 'fixed',
    inset: '0 auto auto 0',
    width: '1024px',
    height: '1024px',
    overflow: 'hidden',
    visibility: 'hidden',
    pointerEvents: 'none',
    contain: 'strict',
    zIndex: '-1',
  });
  document.body.appendChild(host);
  return host;
}

export async function renderMermaidSvg(
  source: string,
  theme: MermaidExportTheme = 'light',
  options?: MermaidRenderOptions,
): Promise<string> {
  const trimmed = source.trim();
  if (!trimmed) {
    throw new Error('Mermaid diagram is empty');
  }

  return queuedRender(async () => {
    const mermaid = await getMermaid(theme, options);
    const id = `docs-mermaid-${Date.now()}-${renderCounter++}`;
    const renderHost = createRenderHost();
    try {
      // Mermaid otherwise appends its temporary measuring SVG directly to
      // document.body. Large diagrams then toggle the root scrollbar while
      // they render, shifting the entire application left and right.
      const result = await mermaid.render(id, trimmed, renderHost);
      return result.svg.replace(/background-color:\s*[^;"}]+;?/gi, 'background-color: transparent;');
    } finally {
      renderHost.remove();
    }
  });
}

export function mermaidSvgFile(svg: string, filename = 'mermaid-diagram.svg'): File {
  return new File([svg], filename, { type: 'image/svg+xml' });
}
