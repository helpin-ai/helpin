import DOMPurify from 'dompurify';

export const SVG_EXAMPLE = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 480 160">
  <title>Request flow</title>
  <rect width="480" height="160" fill="white"/>
  <g fill="#f5f5f5" stroke="#1a1a1a">
    <rect x="20" y="40" width="160" height="80" rx="8"/>
    <rect x="300" y="40" width="160" height="80" rx="8"/>
    <path d="M180 80H290M280 72L290 80L280 88" fill="none"/>
  </g>
  <g font-family="Arial, sans-serif" font-size="20" text-anchor="middle" fill="#1a1a1a">
    <text x="100" y="87">Request</text>
    <text x="380" y="87">Response</text>
  </g>
</svg>`;

/** Static SVG only: the same portable asset is used in previews, publishing and exports. */
export function sanitizeSvg(source: string): string {
  if (!source.trim()) throw new Error('SVG diagram is empty.');
  if (/<!DOCTYPE|<!ENTITY/i.test(source)) throw new Error('SVG diagrams cannot contain document types or entities.');
  const parsed = new DOMParser().parseFromString(source, 'image/svg+xml');
  const root = parsed.documentElement;
  if (parsed.querySelector('parsererror') || root.localName !== 'svg' || (root.namespaceURI && root.namespaceURI !== 'http://www.w3.org/2000/svg')) {
    throw new Error('Enter valid SVG with an <svg> root element.');
  }
  if (root.querySelector('script, foreignObject, animate, animateMotion, animateTransform, set')) {
    throw new Error('Use static SVG shapes and text. Scripts, animation and embedded HTML are not supported.');
  }
  for (const element of [root, ...root.querySelectorAll('*')]) {
    for (const attr of Array.from(element.attributes)) {
      if ((attr.localName === 'href' || attr.localName === 'src') && attr.value && !attr.value.startsWith('#')) {
        throw new Error('SVG diagrams must be self-contained. External images and links are not supported.');
      }
    }
  }
  // CSS remains scoped inside an image. Only local references (markers, gradients,
  // filters) are portable; escaped CSS can conceal external URLs, so reject it.
  const css = [...root.querySelectorAll('style')].map((el) => el.textContent ?? '').join('\n')
    + [root, ...root.querySelectorAll('*')].flatMap((el) => Array.from(el.attributes).map((attr) => attr.value)).join('\n');
  if (/\\|@import|@font-face|expression\s*\(/i.test(css) || [...css.matchAll(/url\s*\(([^)]*)\)/gi)].some((match) => !/^['"]?#[\w:.-]+['"]?$/.test(match[1].trim()))) {
    throw new Error('SVG diagrams must use self-contained styles and local references.');
  }
  root.setAttribute('xmlns', 'http://www.w3.org/2000/svg');
  const safe = DOMPurify.sanitize(new XMLSerializer().serializeToString(root), {
    USE_PROFILES: { svg: true, svgFilters: true },
    FORBID_TAGS: ['foreignObject', 'script', 'animate', 'animateMotion', 'animateTransform', 'set'],
  });
  return safe;
}

export function svgDataUrl(svg: string): string {
  const bytes = new TextEncoder().encode(svg);
  let binary = '';
  for (const byte of bytes) binary += String.fromCharCode(byte);
  return `data:image/svg+xml;base64,${btoa(binary)}`;
}
