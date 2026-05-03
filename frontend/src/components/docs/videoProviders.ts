export interface VideoInfo {
  provider: 'youtube' | 'vimeo' | 'loom' | 'wistia';
  sourceUrl: string;
  embedUrl: string;
}

interface ProviderParser {
  provider: VideoInfo['provider'];
  hosts: string[];
  match: (pathname: string, searchParams: URLSearchParams) => string | null;
}

const PARSERS: ProviderParser[] = [
  {
    provider: 'youtube',
    hosts: ['www.youtube.com', 'youtube.com', 'm.youtube.com', 'youtu.be'],
    match: (pathname, searchParams) => {
      // youtu.be/ID
      if (pathname.match(/^\/([a-zA-Z0-9_-]{11})$/)) {
        return `https://www.youtube.com/embed/${pathname.slice(1)}`;
      }
      // /watch?v=ID
      const v = searchParams.get('v');
      if (v && /^[a-zA-Z0-9_-]{11}$/.test(v)) {
        return `https://www.youtube.com/embed/${v}`;
      }
      // /embed/ID, /shorts/ID, /live/ID
      const m = pathname.match(/^\/(embed|shorts|live)\/([a-zA-Z0-9_-]{11})/);
      if (m) return `https://www.youtube.com/embed/${m[2]}`;
      return null;
    },
  },
  {
    provider: 'vimeo',
    hosts: ['vimeo.com', 'www.vimeo.com', 'player.vimeo.com'],
    match: (pathname) => {
      // player.vimeo.com/video/ID
      let m = pathname.match(/^\/video\/(\d+)/);
      if (m) return `https://player.vimeo.com/video/${m[1]}`;
      // vimeo.com/ID or vimeo.com/channels/*/ID or vimeo.com/ID/hash
      m = pathname.match(/^\/(?:channels\/[^/]+\/|groups\/[^/]+\/videos\/)?(\d+)(?:\/([a-f0-9]+))?$/);
      if (m) {
        const base = `https://player.vimeo.com/video/${m[1]}`;
        return m[2] ? `${base}?h=${m[2]}` : base;
      }
      return null;
    },
  },
  {
    provider: 'loom',
    hosts: ['www.loom.com', 'loom.com'],
    match: (pathname) => {
      const m = pathname.match(/^\/(share|embed)\/([a-zA-Z0-9]+)/);
      return m ? `https://www.loom.com/embed/${m[2]}` : null;
    },
  },
  {
    provider: 'wistia',
    hosts: ['fast.wistia.net', 'wi.st'],
    // Also match *.wistia.com
    match: (pathname) => {
      // /medias/ID or /embed/iframe/ID
      let m = pathname.match(/^\/medias\/([a-zA-Z0-9]+)/);
      if (!m) m = pathname.match(/^\/embed\/iframe\/([a-zA-Z0-9]+)/);
      return m ? `https://fast.wistia.net/embed/iframe/${m[1]}` : null;
    },
  },
];

export function parseVideoUrl(raw: string): VideoInfo | null {
  const s = raw.trim();
  if (!s) return null;

  // Normalize: add https if missing
  const normalized = /^https?:\/\//i.test(s) ? s : `https://${s}`;

  let u: URL;
  try {
    u = new URL(normalized);
  } catch {
    return null;
  }

  if (u.protocol !== 'https:' && u.protocol !== 'http:') return null;

  for (const parser of PARSERS) {
    // Check host — exact match or *.wistia.com for custom Wistia domains
    const hostMatch = parser.hosts.includes(u.host) ||
      (parser.provider === 'wistia' && u.host.endsWith('.wistia.com'));
    if (!hostMatch) continue;

    const embedUrl = parser.match(u.pathname, u.searchParams);
    if (embedUrl) {
      return { provider: parser.provider, sourceUrl: canonicalSourceUrl(parser.provider, normalized, embedUrl), embedUrl };
    }
  }
  return null;
}

function canonicalSourceUrl(provider: VideoInfo['provider'], fallback: string, embedUrl: string): string {
  if (provider !== 'youtube') {
    return fallback;
  }
  try {
    const u = new URL(embedUrl);
    const match = u.pathname.match(/^\/embed\/([a-zA-Z0-9_-]{11})$/);
    if (u.host === 'www.youtube.com' && match) {
      return `https://www.youtube.com/watch?v=${match[1]}`;
    }
  } catch {
    return fallback;
  }
  return fallback;
}

export const SUPPORTED_PROVIDERS = 'YouTube, Vimeo, Loom, or Wistia';
