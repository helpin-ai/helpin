// Render the canonical blueprint to stdout; publish the returned HTML separately.
import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { pathToFileURL } from 'node:url';

const requireFrontend = createRequire(new URL('../frontend/package.json', import.meta.url));
const React = requireFrontend('react');
const { renderToStaticMarkup } = requireFrontend('react-dom/server');
const { default: Markdown } = await import(pathToFileURL(requireFrontend.resolve('react-markdown')).href);
const { default: remarkGfm } = await import(pathToFileURL(requireFrontend.resolve('remark-gfm')).href);
const source = readFileSync(new URL('../docs/crm-customer-work-blueprint.md', import.meta.url), 'utf8');
const appStyles = readFileSync(new URL('../frontend/src/index.css', import.meta.url), 'utf8');
const h = React.createElement;
const slug = text => String(text).toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '');
const headings = [...source.matchAll(/^## (.+)$/gm)].map(([, title]) => ({ title, id: slug(title) }));
const tokens = ['text-primary', 'text-secondary', 'text-tertiary', 'surface', 'hover', 'row-hover', 'divider-strong', 'accent']
  .map(name => {
    const match = appStyles.match(new RegExp(`--quiet-${name}:\\s*([^;]+);`));
    if (!match) throw new Error(`Missing Quiet token: ${name}`);
    return `--quiet-${name}: ${match[1]};`;
  }).join('\n');

const styles = `
:root { ${tokens} color-scheme: light; }
* { box-sizing: border-box; }
html { scroll-padding-top: 90px; }
body { margin: 0; background: var(--quiet-surface); color: var(--quiet-text-primary); font: 15px/1.75 Inter, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; }
a { color: var(--quiet-accent); text-decoration-thickness: 1px; text-underline-offset: 3px; }
a:hover { text-decoration: underline; }
a:focus-visible { outline: 2px solid var(--quiet-accent); outline-offset: 4px; }
.topbar { position: sticky; top: 0; z-index: 1; background: var(--quiet-surface); border-bottom: 1px solid var(--quiet-divider-strong); }
.topbar-inner { max-width: 1380px; margin: auto; padding: 16px 36px; display: flex; align-items: center; justify-content: space-between; gap: 24px; }
.brand { font-size: 14px; font-weight: 600; }
.brand span { margin-left: 12px; font-weight: 400; color: var(--quiet-text-tertiary); }
.topbar a { font-size: 13px; }
.layout { max-width: 1380px; margin: auto; display: grid; grid-template-columns: 245px minmax(0, 1fr); gap: 58px; padding: 44px 36px 100px; }
aside { position: sticky; top: 104px; align-self: start; }
.eyebrow { margin: 0 0 14px; font-size: 11px; font-weight: 600; letter-spacing: .1em; text-transform: uppercase; color: var(--quiet-text-tertiary); }
nav a { display: block; margin: 0 0 6px; padding: 5px 0; color: var(--quiet-text-secondary); text-decoration: none; font-size: 13px; line-height: 1.55; }
nav a:hover { color: var(--quiet-accent); }
.aside-note { border-top: 1px solid var(--quiet-divider-strong); margin-top: 22px; padding-top: 18px; color: var(--quiet-text-tertiary); font-size: 12px; line-height: 1.65; }
article { min-width: 0; max-width: 950px; }
h1 { margin: 0 0 26px; font-size: 36px; line-height: 1.2; font-weight: 600; letter-spacing: -1.15px; }
h2 { margin: 48px 0 18px; padding-top: 28px; border-top: 1px solid var(--quiet-divider-strong); font-size: 23px; line-height: 1.35; font-weight: 600; letter-spacing: -.45px; }
h3 { margin: 30px 0 12px; font-size: 17px; line-height: 1.5; font-weight: 600; }
p { margin: 0 0 17px; }
strong { font-weight: 600; }
ul, ol { padding-left: 23px; margin: 14px 0 22px; }
li { padding-left: 4px; margin: 9px 0; }
li p { margin-bottom: 9px; }
code { font-size: .87em; background: var(--quiet-hover); padding: 2px 5px; border-radius: 3px; overflow-wrap: anywhere; }
.table-wrap { overflow-x: auto; margin: 22px 0 28px; }
table { width: 100%; border-collapse: collapse; font-size: 13px; line-height: 1.6; }
th { text-align: left; background: var(--quiet-row-hover); color: var(--quiet-text-secondary); font-weight: 600; }
th, td { padding: 13px 15px; vertical-align: top; border-bottom: 1px solid var(--quiet-divider-strong); }
th:first-child, td:first-child { min-width: 150px; }
.repo-reference { text-decoration: underline dotted; text-underline-offset: 3px; }
.footer { margin-top: 40px; padding-top: 22px; border-top: 1px solid var(--quiet-divider-strong); color: var(--quiet-text-tertiary); font-size: 12px; }
@media (max-width: 950px) { .layout { grid-template-columns: 1fr; gap: 26px; padding: 28px 24px 64px; } aside { position: static; } nav { display: flex; flex-wrap: wrap; gap: 4px 20px; } .aside-note { display: none; } .topbar-inner { padding: 14px 24px; } h1 { font-size: 30px; } }
`;

const components = {
  h2: ({ children }) => h('h2', { id: slug(children) }, children),
  a: ({ href, children }) => /^https?:\/\//.test(href || '')
    ? h('a', { href, target: '_blank', rel: 'noopener noreferrer' }, children)
    : h('span', { className: 'repo-reference', title: 'Repository reference — source files are not published on this review page.' }, children),
  table: ({ children }) => h('div', { className: 'table-wrap' }, h('table', null, children)),
};

const page = h('html', { lang: 'en' },
  h('head', null,
    h('meta', { charSet: 'utf-8' }),
    h('meta', { name: 'viewport', content: 'width=device-width, initial-scale=1' }),
    h('meta', { name: 'robots', content: 'noindex, nofollow, noarchive' }),
    h('meta', { name: 'referrer', content: 'no-referrer' }),
    h('title', null, 'CRM customer work · Helpin product blueprint'),
    h('style', null, styles)),
  h('body', null,
    h('header', { className: 'topbar' }, h('div', { className: 'topbar-inner' },
      h('div', { className: 'brand' }, 'Helpin', h('span', null, 'Product review')),
      h('a', { href: '#9-agreed-decisions-and-implementation-boundary' }, 'Jump to decisions →'))),
    h('div', { className: 'layout' },
      h('aside', null,
        h('p', { className: 'eyebrow' }, 'In this blueprint'),
        h('nav', { 'aria-label': 'Blueprint sections' }, headings.map(item => h('a', { key: item.id, href: '#' + item.id }, item.title))),
        h('p', { className: 'aside-note' }, 'Read-only plan. Playbooks → Flows → Beacon + skills is the agreed direction. This page does not activate automation.')),
      h('article', { 'aria-label': 'CRM customer-work blueprint' },
        h(Markdown, { remarkPlugins: [remarkGfm], components, skipHtml: true }, source),
        h('footer', { className: 'footer' }, 'Source: docs/crm-customer-work-blueprint.md · Repository references are shown as labels; source files are not published here.')))));

process.stdout.write('<!doctype html>\n' + renderToStaticMarkup(page) + '\n');
