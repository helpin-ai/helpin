import { HeroVortex } from '../_components/HeroVortex';
import { createPageMetadata, PAGE_SEO } from '@/lib/metadata';
import { Download } from 'lucide-react';
import { HelpinBrand } from '@/components/HelpinBrand';
import { PreviewNav } from '../_components/PreviewNav';
import { PreviewFooter } from '../_components/PreviewFooter';
import { ColorPalette } from './brand-tools';
import './branding.css';

export const metadata = createPageMetadata(PAGE_SEO.branding);

export default function BrandingPage() {
  return <>
    <PreviewNav />
    <div className="brand-page">
      <header className="brand-header wrap">
        <div className="brand-banner motion-hero"><HeroVortex variant="connections" tone="dark" />
          <div className="brand-banner-copy"><span className="eyebrow">Helpin / Brand resources</span><h1>The Helpin<br />essentials.</h1><p>Use the Helpin logo, colors, and type in your article, integration, or presentation.</p><div className="brand-download"><a className="btn btn-primary" href="/brand/kit/helpin-brand-kit.zip" download><Download size={16} aria-hidden="true" />Download brand kit</a><span>SVG + PNG symbols · Color palette</span></div></div>
          <div className="brand-banner-art" aria-hidden="true">
            <svg viewBox="0 0 360 260" fill="none"><path d="M0 50 H72 Q90 50 90 68 V112 Q90 130 108 130 H148 M0 210 H72 Q90 210 90 192 V148 Q90 130 108 130 H148 M360 50 H288 Q270 50 270 68 V112 Q270 130 252 130 H212 M360 210 H288 Q270 210 270 192 V148 Q270 130 252 130 H212" stroke="#7CAC8B" strokeOpacity=".45" /><circle cx="34" cy="50" r="4" fill="#B8DDBB" /><circle cx="326" cy="210" r="4" fill="#B8DDBB" /><rect x="100" y="50" width="160" height="160" rx="36" fill="#173D2A" stroke="#4B715A" /><image href="/brand/helpin-icon-white.svg" x="128" y="78" width="104" height="104" /></svg>
            <span>One customer history.<br />A workspace for your team and agents.</span>
          </div>
          <div className="brand-banner-strip" aria-hidden="true"><i /><i /><i /><i /><i /><i /></div>
        </div>
      </header>

      <section id="brand-logo"><div className="wrap">
        <div className="brand-section-heading"><div><span>01 / Identity</span><h2>Our logo.</h2></div><p>Use the original mark, give it space, and keep its shape and colors intact.</p></div>
        <div className="brand-logo-grid">
          {(['ink', 'white'] as const).map(color => <article className="brand-logo-card" key={color}>
            <div className={`brand-logo-stage brand-logo-${color}`}><span className="brand-stage-label">{color === 'ink' ? 'The everyday signature' : 'Made for darker backgrounds'}</span><HelpinBrand variant={color === 'white' ? 'light-on-dark' : 'dark-on-light'} /></div>
            <div className="brand-logo-caption"><h3>{color === 'ink' ? 'Dark on light' : 'Light on dark'}</h3><div><a href={`/brand/helpin-icon-${color}.svg`} download aria-label={`Download ${color === 'ink' ? 'dark' : 'white'} symbol as SVG`}>Symbol SVG <Download size={14} aria-hidden="true" /></a><a href={`/brand/kit/helpin-symbol-${color}-512.png`} download aria-label={`Download ${color === 'ink' ? 'dark' : 'white'} symbol as PNG`}>PNG <Download size={14} aria-hidden="true" /></a></div></div>
          </article>)}
        </div>
      </div></section>

      <section id="brand-colors"><div className="wrap">
        <div className="brand-section-heading"><div><span>02 / Palette</span><h2>Our colors.</h2></div><p>Forest and green, balanced with light neutrals. <a href="/brand/kit/helpin-palette.json" download>Download the full palette.</a></p></div>
        <ColorPalette />
      </div></section>

      <section id="brand-type"><div className="wrap">
        <div className="brand-section-heading"><div><span>03 / Typography</span><h2>Our type.</h2></div><p>Two typefaces. Clear, readable, and consistent.</p></div>
        <div className="brand-type-grid">
          <article><div className="brand-type-top"><span className="brand-type-label">Primary typeface</span><span className="brand-type-weight">400 to 700</span></div><h3>Instrument Sans</h3><p className="brand-type-specimen">Every agent<br />has a useful job.</p><div className="brand-type-bottom"><span aria-hidden="true">Aa Bb Cc 0123456789</span><p>Headlines, body copy, and interface text.</p></div></article>
          <article className="brand-type-mono"><div className="brand-type-top"><span className="brand-type-label">Supporting typeface</span><span className="brand-type-weight">400 / 500</span></div><h3>JetBrains Mono</h3><p className="brand-type-specimen">Hear → Decide<br />Ship → Tell</p><div className="brand-type-bottom"><span aria-hidden="true">Aa Bb Cc 0123456789</span><p>Small labels, code, and technical details.</p></div></article>
        </div>
        <p className="brand-contact">Need another format? <a href="mailto:hello@helpin.ai">Get in touch.</a></p>
      </div></section>
    </div>
    <PreviewFooter />
  </>;
}
