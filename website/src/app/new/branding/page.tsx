import type { Metadata } from 'next';
import { Download } from 'lucide-react';
import { HelpinBrand } from '@/components/HelpinBrand';
import { PreviewNav } from '../_components/PreviewNav';
import { PreviewFooter } from '../_components/PreviewFooter';
import { ColorPalette } from './brand-tools';
import './branding.css';

export const metadata: Metadata = {
  title: 'Branding — Helpin',
  description: 'Helpin logos, colors, and typography. Download the brand kit for your next article, integration, or presentation.',
  alternates: { canonical: '/new/branding' },
  openGraph: { title: 'The Helpin brand', description: 'Logos, colors, and typography. The Helpin brand essentials.', url: '/new/branding' },
};

export default function BrandingPage() {
  return <>
    <PreviewNav />
    <div className="brand-page">
      <header className="brand-header wrap">
        <div><span className="eyebrow">Branding</span><h1>The Helpin essentials.</h1><p>Our logo, colors, and type. Ready for your next article, integration, or presentation.</p></div>
        <div className="brand-download"><a className="btn btn-primary" href="/brand/kit/helpin-brand-kit.zip" download><Download size={16} aria-hidden="true" />Download brand kit</a><span>SVG + PNG symbols · Color palette</span></div>
      </header>

      <section id="brand-logo"><div className="wrap">
        <div className="brand-section-heading"><h2>Logo</h2><p>Use the original mark, give it space, and keep its shape and colors intact.</p></div>
        <div className="brand-logo-grid">
          {(['ink', 'white'] as const).map(color => <article className="brand-logo-card" key={color}>
            <div className={`brand-logo-stage brand-logo-${color}`}><HelpinBrand variant={color === 'white' ? 'light-on-dark' : 'dark-on-light'} /></div>
            <div className="brand-logo-caption"><h3>{color === 'ink' ? 'Dark on light' : 'Light on dark'}</h3><div><a href={`/brand/helpin-icon-${color}.svg`} download aria-label={`Download ${color === 'ink' ? 'dark' : 'white'} symbol as SVG`}>Symbol SVG <Download size={14} aria-hidden="true" /></a><a href={`/brand/kit/helpin-symbol-${color}-512.png`} download aria-label={`Download ${color === 'ink' ? 'dark' : 'white'} symbol as PNG`}>PNG <Download size={14} aria-hidden="true" /></a></div></div>
          </article>)}
        </div>
      </div></section>

      <section id="brand-colors"><div className="wrap">
        <div className="brand-section-heading"><h2>Colors</h2><p>Forest and green, balanced with light neutrals. <a href="/brand/kit/helpin-palette.json" download>Download the full palette.</a></p></div>
        <ColorPalette />
      </div></section>

      <section id="brand-type"><div className="wrap">
        <div className="brand-section-heading"><h2>Typography</h2><p>Two typefaces. Clear, readable, and consistent.</p></div>
        <div className="brand-type-grid">
          <article><span className="brand-type-label">Primary · Regular to bold</span><h3>Instrument Sans</h3><p className="brand-type-specimen">Every conversation has a next step.</p><p>For headlines, body copy, and interface text.</p></article>
          <article className="brand-type-mono"><span className="brand-type-label">Supporting · Regular &amp; medium</span><h3>JetBrains Mono</h3><p className="brand-type-specimen">Hear → Decide → Ship → Tell</p><p>For small labels, code, and technical details.</p></article>
        </div>
        <p className="brand-contact">Need another format? <a href="mailto:hello@helpin.ai">Get in touch.</a></p>
      </div></section>
    </div>
    <PreviewFooter />
  </>;
}
