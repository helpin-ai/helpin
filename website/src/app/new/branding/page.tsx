import type { Metadata } from 'next';
import Link from 'next/link';
import { ArrowDown, ArrowRight, ArrowUpRight, Check, Download, X } from 'lucide-react';
import { HelpinBrand } from '@/components/HelpinBrand';
import { PreviewNav } from '../_components/PreviewNav';
import { PreviewFooter } from '../_components/PreviewFooter';
import { ColorPalette, BrandMotion } from './brand-tools';
import './branding.css';

export const metadata: Metadata = {
  title: 'Branding — Helpin',
  description: 'The Helpin brand: logos, colors, typography, and guidance for telling our story. Download brand assets for your next article, integration, or presentation.',
  alternates: { canonical: '/new/branding' },
  openGraph: { title: 'The Helpin brand', description: 'Logos, colors, typography, and a shared visual language.', url: '/new/branding' },
};

const NAV = [['brand-logo', 'Logo'], ['brand-colors', 'Colors'], ['brand-type', 'Typography'], ['brand-style', 'Visual style'], ['brand-voice', 'Voice'], ['brand-downloads', 'Downloads']];
function SectionIntro({ number, title, children }: { number: string; title: string; children: React.ReactNode }) {
  return <div className="brand-section-intro"><span className="brand-index">{number}</span><div><h2>{title}</h2><p>{children}</p></div></div>;
}
function DownloadLink({ href, children }: { href: string; children: React.ReactNode }) {
  return <a href={href} download className="brand-download-link">{children}<Download size={15} aria-hidden="true" /></a>;
}

export default function BrandingPage() {
  return <>
    <PreviewNav />
    <div className="brand-page">
      <section className="brand-hero">
        <div className="wrap brand-hero-grid">
          <div className="brand-hero-copy"><span className="eyebrow">The Helpin brand</span><h1>Connected by design.<br /><span>Recognizably Helpin.</span></h1><p className="lede">A shared identity for a connected workspace. Everything you need to bring Helpin into your story, your product, or your next presentation.</p><div className="brand-actions"><a className="btn btn-primary" href="/brand/kit/helpin-brand-kit.zip" download><Download size={16} aria-hidden="true" />Download brand kit</a><a className="btn-link" href="#brand-logo">Explore the guidelines <ArrowDown size={15} aria-hidden="true" /></a></div><span className="brand-file-note">SVG + PNG symbols · Color palette</span></div>
          <div className="brand-cover" aria-label="Helpin identity: forest green, a white logo, and a restrained green palette">
            <div className="brand-cover-top"><span>Helpin / Brand essentials</span><span>01—06</span></div>
            <div className="brand-cover-logo"><HelpinBrand variant="light-on-dark" /></div>
            <p>One customer history.<br />A workspace for your team and agents.</p>
            <div className="brand-cover-palette" aria-hidden="true"><i /><i /><i /><i /></div>
          </div>
        </div>
      </section>

      <nav className="brand-page-nav" aria-label="Brand guidelines sections"><div className="wrap"><span>Brand essentials</span>{NAV.map(([id, label]) => <a key={id} href={`#${id}`}>{label}</a>)}</div></nav>

      <section id="brand-logo"><div className="wrap">
        <SectionIntro number="01" title="A familiar mark. Wherever we meet.">Use the Helpin logo to introduce us clearly. Keep its shape intact, give it room, and choose the version that stands out against your background.</SectionIntro>
        <div className="brand-logo-grid">
          <article className="brand-logo-card"><div className="brand-logo-stage brand-logo-light"><HelpinBrand /></div><div className="brand-logo-caption"><div><h3>Dark on light</h3><p>The everyday choice for light backgrounds.</p></div><div><DownloadLink href="/brand/helpin-icon-ink.svg">Symbol SVG</DownloadLink><DownloadLink href="/brand/kit/helpin-symbol-ink-512.png">PNG</DownloadLink></div></div></article>
          <article className="brand-logo-card"><div className="brand-logo-stage brand-logo-dark"><HelpinBrand variant="light-on-dark" /></div><div className="brand-logo-caption"><div><h3>Light on dark</h3><p>A white mark for forest and dark surfaces.</p></div><div><DownloadLink href="/brand/helpin-icon-white.svg">Symbol SVG</DownloadLink><DownloadLink href="/brand/kit/helpin-symbol-white-512.png">PNG</DownloadLink></div></div></article>
        </div>
        <div className="brand-logo-guides">
          <article><div className="brand-clearspace" aria-label="Leave clear space of at least one quarter of the symbol width on every side"><div><img src="/brand/helpin-icon-ink.svg" width={80} height={80} alt="Helpin symbol" /></div><span>¼ mark width</span></div><h3>Give the mark room.</h3><p>Leave at least a quarter of the symbol’s width clear on every side. Keep text, edges, and other logos outside that space.</p></article>
          <article><div className="brand-small-marks"><img src="/brand/helpin-icon-ink.svg" width={24} height={24} alt="Helpin symbol at 24 pixels" /><img src="/brand/helpin-icon-ink.svg" width={40} height={40} alt="Helpin symbol at 40 pixels" /><img src="/brand/helpin-icon-ink.svg" width={64} height={64} alt="Helpin symbol at 64 pixels" /></div><h3>Keep it recognizable.</h3><p>Use the symbol at 24 pixels or larger in your layouts. Use SVG for crisp scaling; PNG is available when you need a raster file.</p></article>
          <article><div className="brand-name-sample">Helpin<span>Capital H. One word.</span></div><h3>Keep the name simple.</h3><p>Write Helpin in running text. Use the symbol on its own when the name is already nearby, such as an integration listing.</p></article>
        </div>
        <div className="brand-usage"><p><Check size={17} aria-hidden="true" /><span>Use the original files on a clean, high-contrast background.</span></p><p><X size={17} aria-hidden="true" /><span>Avoid stretching, rotating, adding effects, or rebuilding the symbol.</span></p></div>
      </div></section>

      <section id="brand-colors" className="brand-soft-section"><div className="wrap">
        <SectionIntro number="02" title="Grounded in green. Balanced with space.">Forest gives Helpin depth. Green brings focus. Light neutrals keep the work readable. These are the colors behind our website and customer support experience.</SectionIntro>
        <ColorPalette />
        <div className="brand-color-guidance"><div><h3>Use contrast with purpose.</h3><p>Pair white or sage with forest, ink with white, and green with white. Reserve pale tones for backgrounds and hairline dividers, rather than small text.</p></div><div className="brand-pairings" aria-label="Example text and background pairings"><span>Aa<small>White / Forest</small></span><span>Aa<small>Ink / White</small></span><span>Aa<small>Green / White</small></span></div></div>
      </div></section>

      <section id="brand-type"><div className="wrap">
        <SectionIntro number="03" title="Clear words. Quiet confidence.">Our typography makes complex work feel approachable. Use a clear hierarchy, comfortable spacing, and short lines that are easy to follow.</SectionIntro>
        <div className="brand-type-grid"><article className="brand-type-primary"><div className="brand-type-meta"><span>Primary typeface</span><span>400 / 500 / 600 / 700</span></div><h3>Instrument Sans</h3><div className="brand-type-large" aria-hidden="true">Aa</div><p className="brand-alphabet">ABCDEFGHIJKLMNOPQRSTUVWXYZ<br />abcdefghijklmnopqrstuvwxyz<br />0123456789 &amp; @ →</p><p className="brand-type-note">Headlines, paragraphs, navigation, and interface labels. Use regular weight for reading and semibold for emphasis.</p></article><div className="brand-type-stack"><article className="brand-type-mono"><div className="brand-type-meta"><span>Supporting typeface</span><span>400 / 500</span></div><h3>JetBrains Mono</h3><p className="brand-mono-specimen">Hear → Decide<br />Ship → Tell</p><p className="brand-type-note">Small labels, sequences, code, and technical details. A useful accent, used sparingly.</p></article><article className="brand-hierarchy"><span className="eyebrow">A simple hierarchy</span><h3>Every conversation<br />has a next step.</h3><p>Bring customer context, your team, and AI agents together to get work done.</p><span className="brand-sample-link">Turn it into action <ArrowRight size={15} aria-hidden="true" /></span></article></div></div>
      </div></section>

      <section id="brand-style" className="brand-style-section"><div className="wrap">
        <SectionIntro number="04" title="Show how the work connects.">Our visual language follows the product: people, conversations, and work sharing context. Use structure and motion to make those connections easy to understand.</SectionIntro>
        <div className="brand-style-grid"><article><BrandMotion /><div className="brand-style-copy"><h3>Motion with a reason.</h3><p>Follow a connection, reveal a handoff, or show the next step. Keep the pace calm and the story readable when motion is paused.</p></div></article><article><div className="brand-texture" aria-hidden="true"><div className="brand-texture-card"><span className="brand-texture-dot" />Customer context<span className="brand-texture-line" /><span className="brand-texture-line" /></div><span className="brand-texture-label">A little structure. Room to breathe.</span></div><div className="brand-style-copy"><h3>Quiet structure.</h3><p>Dotted backgrounds, fine grids, and soft corners create a sense of order. Leave enough space for the message to lead.</p></div></article></div>
        <div className="brand-product-example"><figure><img src="/new/support/inbox-orbitdesk-sep20-960.webp" width={960} height={504} loading="lazy" decoding="async" alt="Illustrative Helpin inbox showing a customer conversation alongside customer details and connected work." /><figcaption>Illustrative product scene from Customer Support.</figcaption></figure><div><span className="eyebrow">The product is the story</span><h3>Show real context.<br />Make the next step visible.</h3><p>Use focused product scenes with believable conversations and readable details. Keep customer names, workspace identity, and the story consistent across images.</p><Link href="/new/products/customer-support" className="btn-link">See the support experience <ArrowUpRight size={15} aria-hidden="true" /></Link></div></div>
      </div></section>

      <section id="brand-voice"><div className="wrap">
        <SectionIntro number="05" title="Helpful by name. Human by nature.">Write like a capable teammate. Be specific about the work, clear about the next step, and honest about what people and agents do.</SectionIntro>
        <div className="brand-voice-grid"><article><span className="brand-index">Clear</span><h3>Start with the outcome.</h3><p>Say what someone can get done. Lead with the customer’s need before explaining how the system works.</p><blockquote>“Turn customer conversations into work that gets done.”</blockquote></article><article><span className="brand-index">Connected</span><h3>Keep the context.</h3><p>Show how one step leads to another. Talk about the people and history behind a task, not just the feature.</p><blockquote>“One customer. One record.”</blockquote></article><article><span className="brand-index">Considered</span><h3>Keep people in control.</h3><p>Be direct about what agents handle and where the team reviews, decides, or takes over.</p><blockquote>“Give agents work. Keep control.”</blockquote></article></div>
      </div></section>

      <section id="brand-downloads" className="brand-downloads"><div className="wrap"><div className="brand-download-heading"><span className="eyebrow">06 / Ready to use</span><h2>A little Helpin.<br />For your next big thing.</h2><p>Writing about Helpin, building an integration, or putting together a presentation? Start with the original assets.</p><a className="btn" href="/brand/kit/helpin-brand-kit.zip" download><Download size={17} aria-hidden="true" />Download brand kit</a><span className="brand-file-note">ZIP · Dark and white symbols in SVG + PNG · JSON + CSS palette</span></div><div className="brand-download-list"><DownloadLink href="/brand/helpin-icon-ink.svg">Dark symbol <span>SVG</span></DownloadLink><DownloadLink href="/brand/helpin-icon-white.svg">White symbol <span>SVG</span></DownloadLink><DownloadLink href="/brand/kit/helpin-palette.json">Color palette <span>JSON</span></DownloadLink><DownloadLink href="/brand/kit/helpin-palette.css">Color palette <span>CSS</span></DownloadLink><p>Need something else?<br /><a href="mailto:hello@helpin.ai">Talk to us <ArrowUpRight size={14} aria-hidden="true" /></a></p></div></div></section>
    </div>
    <PreviewFooter />
  </>;
}
