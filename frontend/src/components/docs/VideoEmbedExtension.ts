import { Node, mergeAttributes } from '@tiptap/core';
import { ReactNodeViewRenderer } from '@tiptap/react';
import { VideoEmbedNodeView } from './VideoEmbedNodeView';

declare module '@tiptap/core' {
  interface Commands<ReturnType> {
    videoEmbed: {
      setVideoEmbed: (attrs: { provider: string; sourceUrl: string; embedUrl: string }) => ReturnType;
    };
  }
}

export const VideoEmbedExtension = Node.create({
  name: 'videoEmbed',
  group: 'block',
  atom: true,
  draggable: true,

  addAttributes() {
    return {
      provider: { default: null },
      sourceUrl: { default: null },
      embedUrl: { default: null },
    };
  },

  parseHTML() {
    return [
      {
        tag: 'div[data-video-embed]',
        getAttrs: (el) => {
          const dom = el as HTMLElement;
          const embedUrl = dom.getAttribute('data-video-embed') ?? '';

          // Provider allowlist: embed host → path prefix + allowed source hosts
          const providers: Record<string, { pathPrefix: string; sourceHosts: string[] }> = {
            'www.youtube.com': { pathPrefix: '/embed/', sourceHosts: ['youtube.com', 'www.youtube.com', 'm.youtube.com', 'youtu.be'] },
            'player.vimeo.com': { pathPrefix: '/video/', sourceHosts: ['vimeo.com', 'www.vimeo.com', 'player.vimeo.com'] },
            'www.loom.com': { pathPrefix: '/embed/', sourceHosts: ['loom.com', 'www.loom.com'] },
            'fast.wistia.net': { pathPrefix: '/embed/iframe/', sourceHosts: ['fast.wistia.net', 'wi.st'] },
          };

          // Validate embed URL
          let embedHost: string;
          try {
            const u = new URL(embedUrl);
            if (u.protocol !== 'https:') return false;
            const provider = providers[u.host];
            if (!provider || !u.pathname.startsWith(provider.pathPrefix)) return false;
            embedHost = u.host;
          } catch { return false; }

          // Validate sourceUrl — must belong to the same provider's known hosts
          const allowedSourceHosts = providers[embedHost].sourceHosts;
          let validatedSourceUrl = embedUrl;
          const rawSource = dom.getAttribute('data-video-source') ?? '';
          if (rawSource) {
            try {
              const su = new URL(rawSource);
              if ((su.protocol === 'https:' || su.protocol === 'http:') &&
                  allowedSourceHosts.includes(su.host)) {
                validatedSourceUrl = rawSource;
              }
            } catch { /* invalid URL, use fallback */ }
          }
          return {
            provider: dom.getAttribute('data-video-provider'),
            sourceUrl: validatedSourceUrl,
            embedUrl,
          };
        },
      },
      {
        // Parse iframe embeds from imported content — validate host, not substring
        tag: 'iframe',
        getAttrs: (el) => {
          const dom = el as HTMLIFrameElement;
          const src = dom.getAttribute('src') ?? '';
          try {
            const u = new URL(src);
            if (u.protocol !== 'https:') return false;
            const hostMap: Record<string, { provider: string; pathPrefix: string }> = {
              'www.youtube.com': { provider: 'youtube', pathPrefix: '/embed/' },
              'player.vimeo.com': { provider: 'vimeo', pathPrefix: '/video/' },
              'www.loom.com': { provider: 'loom', pathPrefix: '/embed/' },
              'fast.wistia.net': { provider: 'wistia', pathPrefix: '/embed/iframe/' },
            };
            const match = hostMap[u.host];
            if (match && u.pathname.startsWith(match.pathPrefix)) {
              return { provider: match.provider, sourceUrl: src, embedUrl: src };
            }
          } catch { /* invalid URL */ }
          return false;
        },
      },
    ];
  },

  renderHTML({ HTMLAttributes }) {
    // Include a fallback link so .doc/.md export preserves the video URL
    return ['div', mergeAttributes(HTMLAttributes, {
      'data-video-embed': HTMLAttributes.embedUrl,
      'data-video-provider': HTMLAttributes.provider,
      'data-video-source': HTMLAttributes.sourceUrl,
    }), ['a', { href: HTMLAttributes.sourceUrl || HTMLAttributes.embedUrl, target: '_blank', rel: 'noopener noreferrer' }, `Video (${HTMLAttributes.provider || 'embed'})`]];
  },

  addNodeView() {
    return ReactNodeViewRenderer(VideoEmbedNodeView);
  },

  // Markdown serialization — serialize as HTML div so it round-trips via parseHTML.
  // Falls back to a markdown link when html mode is off.
  addStorage() {
    return {
      markdown: {
        serialize(state: any, node: any) {
          const url = node.attrs.sourceUrl || node.attrs.embedUrl || '';
          const provider = node.attrs.provider || 'video';
          const embedUrl = node.attrs.embedUrl || '';
          if ((this as any).editor?.storage?.markdown?.options?.html) {
            state.write(
              `<div data-video-embed="${embedUrl}" data-video-provider="${provider}" data-video-source="${url}">` +
              `<a href="${url}" target="_blank" rel="noopener noreferrer">Video (${provider})</a></div>`
            );
            state.closeBlock(node);
          } else {
            state.write(`[Video (${provider})](${url})\n\n`);
          }
        },
        parse: {},
      },
    };
  },

  addCommands() {
    return {
      setVideoEmbed:
        (attrs) =>
        ({ commands }) =>
          commands.insertContent({ type: this.name, attrs }),
    };
  },
});
