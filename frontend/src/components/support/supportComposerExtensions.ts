import { Extension } from '@tiptap/core';
import StarterKit from '@tiptap/starter-kit';
import { Markdown } from 'tiptap-markdown';

export function normalizeSupportLinkInput(raw: string): string {
  const value = raw.trim();
  if (value.startsWith('//')) return `https:${value}`;
  if (!value || /^(?:[a-z][a-z0-9+.-]*:|[/?#]|\.\.?\/)/i.test(value)) return value;
  return `https://${value}`;
}

type LinkMatch = { schema: string; url: string };
type Linkifier = { normalize: (match: LinkMatch) => void };

// Markdown paste/draft parsing uses linkify-it, separately from TipTap's links.
const HttpsMarkdownLinks = Extension.create({
  name: 'supportHttpsMarkdownLinks',
  addStorage() {
    const configured = new WeakSet<Linkifier>();
    return {
      markdown: {
        parse: {
          setup({ linkify }: { linkify: Linkifier }) {
            if (configured.has(linkify)) return;
            configured.add(linkify);
            const normalize = linkify.normalize;
            linkify.normalize = (match: LinkMatch) => {
              normalize.call(linkify, match);
              if (!match.schema) match.url = match.url.replace(/^http:\/\//i, 'https://');
            };
          },
        },
      },
    };
  },
});

export function createSupportComposerExtensions() {
  return [
    StarterKit.configure({
      heading: false,
      codeBlock: false,
      horizontalRule: false,
      link: {
        openOnClick: false,
        autolink: true,
        linkOnPaste: true,
        defaultProtocol: 'https',
        HTMLAttributes: { target: '_blank', rel: 'noopener noreferrer nofollow' },
      },
    }),
    HttpsMarkdownLinks,
    Markdown.configure({
      html: false,
      linkify: true,
      transformPastedText: true,
      transformCopiedText: true,
    }),
  ];
}
