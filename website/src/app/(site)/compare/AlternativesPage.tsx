import Link from 'next/link';
import { ArrowRight, CalendarCheck, Check, ChevronRight, Minus, PenLine, Scale } from 'lucide-react';
import { SITE_URL } from '@/lib/metadata';
import { article, JsonLd, organization } from '@/lib/structured-data';
import { HeroVortex } from '../_components/HeroVortex';
import { PreviewNav } from '../_components/PreviewNav';
import { PreviewFooter } from '../_components/PreviewFooter';
import { PlatformClosing } from '../_components/platform/PlatformParts';
import { CtaNote, CtaRow, DEMO_URL, FAQList, SectionHead, SIGNUP_URL } from '../_components/ui';
import { alternativesSeo, TOOLS, type AlternativesPage as Page } from './alternatives-data';
import { CompareCard, HelpinMark, Monogram } from './ComparePage';
import { COMPETITORS, formatChecked } from './compare-data';

function ToolMark({ name, size = 24 }: { name: string; size?: number }) {
  return name === 'Helpin' ? <HelpinMark size={size} /> : <Monogram name={name} />;
}

export function AlternativesPage({ page }: { page: Page }) {
  const tools = page.tools.map(key => TOOLS[key]);
  const checked = formatChecked(page.checked);
  const year = page.checked.slice(0, 4);
  const related = COMPETITORS.filter(item => item.name !== page.competitor && tools.some(tool => tool.compare === `/compare/${item.slug}`)).slice(0, 3);
  const seo = alternativesSeo(page);
  const structured = {
    '@context': 'https://schema.org',
    '@graph': [
      {
        '@type': 'BreadcrumbList',
        itemListElement: [
          { '@type': 'ListItem', position: 1, name: 'Helpin', item: SITE_URL },
          { '@type': 'ListItem', position: 2, name: 'Compare', item: `${SITE_URL}/compare` },
          { '@type': 'ListItem', position: 3, name: `${page.competitor} alternatives`, item: `${SITE_URL}/compare/${page.slug}` },
        ],
      },
      { ...article({ headline: seo.title, description: seo.description, path: seo.canonicalPath, date: page.checked }), image: `${SITE_URL}${seo.imagePath}` },
      organization,
      {
        '@type': 'ItemList',
        name: `${page.competitor} alternatives`,
        itemListElement: tools.map((tool, index) => ({ '@type': 'ListItem', position: index + 1, name: tool.name })),
      },
    ],
  };

  return (
    <>
      <PreviewNav />
      <div className="platform-page compare-page">
        <JsonLd data={structured} />
        <section className="platform-hero motion-hero">
          <HeroVortex variant="connections" tone="dark" />
          <div className="wrap">
            <div className="platform-breadcrumb"><Link href="/">Helpin</Link><ChevronRight size={12} aria-hidden="true" /><Link href="/compare">Compare</Link><ChevronRight size={12} aria-hidden="true" /><span>{page.competitor} alternatives</span></div>
            <div className="platform-hero-grid">
              <div className="platform-hero-copy">
                <span className="eyebrow">{page.competitor} alternatives · {year}</span>
                <h1>{page.competitor} alternatives: <span>six tools worth a look.</span></h1>
                <p className="lede">{page.lede}</p>
                <CtaRow primaryLabel="Start free trial" primaryHref={SIGNUP_URL} secondaryHref={DEMO_URL} secondaryLabel="Talk to us about switching" />
                <CtaNote trial support />
                <div className="platform-hero-points">
                  <span><PenLine size={14} aria-hidden="true" />By the Helpin team</span>
                  <span><CalendarCheck size={14} aria-hidden="true" />Verified {checked}</span>
                  <span><Scale size={14} aria-hidden="true" />Limits listed for every tool</span>
                </div>
              </div>
              <nav className="cmp-hub-board" aria-label={`${page.competitor} alternatives on this page`}>
                <div className="cmp-matchup-bar"><span><Scale size={13} aria-hidden="true" />The shortlist</span><span>{tools.length} tools</span></div>
                <ul>
                  {tools.map((tool, index) => (
                    <li key={tool.key}>
                      <a href={`#tool-${tool.key}`}>
                        <span className="cmp-hub-rank">{String(index + 1).padStart(2, '0')}</span>
                        <span className="cmp-hub-name"><strong>{tool.name}</strong><span>{tool.bestFor}</span></span>
                        <ArrowRight size={15} aria-hidden="true" />
                      </a>
                    </li>
                  ))}
                </ul>
              </nav>
            </div>
          </div>
        </section>

        <nav className="platform-page-nav" aria-label="On this page">
          <div className="wrap">
            <strong>{page.competitor} alternatives</strong>
            <a href="#why-look">Why teams look</a>
            <a href="#what-to-compare">What to compare</a>
            <a href="#at-a-glance">At a glance</a>
            <a href="#the-alternatives">The alternatives</a>
            <a href="#questions">Questions</a>
          </div>
        </nav>

        <section id="why-look">
          <div className="wrap">
            <div className="platform-section-intro"><SectionHead eyebrow="Why teams look" title={`Why teams look for a ${page.competitor} alternative.`} lede={`${page.competitor} is a strong product. These are the reasons teams most often give for looking elsewhere.`} /></div>
            <div className="cmp-reason-grid">
              {page.reasons.map((reason, index) => <article key={reason.title}><span>{String(index + 1).padStart(2, '0')}</span><h3>{reason.title}</h3><p>{reason.body}</p></article>)}
            </div>
          </div>
        </section>

        <section id="what-to-compare" className="platform-soft">
          <div className="wrap">
            <div className="platform-section-intro"><SectionHead eyebrow="What to compare" title="Six questions to ask each tool." lede="The same criteria we use for every tool on this page." /></div>
            <div className="platform-feature-grid cmp-criteria-grid">
              {page.criteria.map(item => <article key={item.title}><h3>{item.title}</h3><p>{item.body}</p></article>)}
            </div>
          </div>
        </section>

        <section id="at-a-glance">
          <div className="wrap">
            <div className="platform-section-intro"><SectionHead eyebrow="At a glance" title={`${tools.length} ${page.competitor} alternatives, side by side.`} lede="Starting prices are list prices billed annually, unless noted." /></div>
            <div className="included-table-scroll cmp-table-scroll" role="region" aria-label={`${page.competitor} alternatives compared`} tabIndex={0}>
              <table className="included-table cmp-table cmp-glance-table">
                <caption className="sr-only">{page.competitor} alternatives compared by best fit, starting price, AI pricing and self-hosting.</caption>
                <thead><tr><th scope="col">Tool</th><th scope="col">Best for</th><th scope="col">Starting price</th><th scope="col">AI pricing</th><th scope="col">Self-host</th></tr></thead>
                <tbody>
                  {tools.map(tool => (
                    <tr key={tool.key} className={tool.key === 'helpin' ? 'cmp-glance-helpin' : undefined}>
                      <th scope="row"><span className="cmp-table-name"><ToolMark name={tool.name} size={18} />{tool.name}</span></th>
                      <td>{tool.bestFor}</td>
                      <td>{tool.startingPrice}</td>
                      <td>{tool.aiPricing}</td>
                      <td>{tool.selfHost}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        </section>

        <section id="the-alternatives" className="platform-soft">
          <div className="wrap">
            <div className="platform-section-intro"><SectionHead eyebrow="The alternatives" title="Each tool, with its strengths and limits." lede="We make Helpin, so it’s first. Every other tool gets the same treatment, including where it beats us." /></div>
            <div className="cmp-tool-list">
              {tools.map((tool, index) => (
                <article key={tool.key} id={`tool-${tool.key}`} className={tool.key === 'helpin' ? 'cmp-tool cmp-tool-helpin' : 'cmp-tool'}>
                  <header>
                    <span className="cmp-tool-rank">{String(index + 1).padStart(2, '0')}</span>
                    <ToolMark name={tool.name} size={30} />
                    <div>
                      <h3>{tool.name}{tool.key === 'helpin' ? <span className="cmp-tool-badge">Our product</span> : null}</h3>
                      <p>Best for {tool.bestFor.charAt(0).toLowerCase()}{tool.bestFor.slice(1)}.</p>
                    </div>
                  </header>
                  <p className="cmp-tool-summary">{tool.summary}</p>
                  <div className="cmp-tool-lists">
                    <div><h4>Strengths</h4><ul>{tool.strengths.map(item => <li key={item}><Check size={14} strokeWidth={2.4} aria-hidden="true" />{item}</li>)}</ul></div>
                    <div><h4>Limits</h4><ul>{tool.limits.map(item => <li key={item}><Minus size={14} strokeWidth={2.4} aria-hidden="true" />{item}</li>)}</ul></div>
                  </div>
                  <footer>
                    <dl>
                      <div><dt>Starting price</dt><dd>{tool.startingPrice}</dd></div>
                      <div><dt>Price model</dt><dd>{tool.priceModel}</dd></div>
                      <div><dt>AI pricing</dt><dd>{tool.aiPricing}</dd></div>
                    </dl>
                    {tool.key === 'helpin'
                      ? <a className="btn btn-primary" href={SIGNUP_URL}>Start free trial →</a>
                      : tool.compare ? <Link className="platform-text-link" href={tool.compare}>Helpin vs {tool.name}<ArrowRight size={15} aria-hidden="true" /></Link> : null}
                  </footer>
                </article>
              ))}
            </div>
          </div>
        </section>

        <section id="questions">
          <div className="wrap platform-faq-grid">
            <SectionHead eyebrow="Questions" title={`${page.competitor} alternatives: common questions.`} />
            <div><FAQList items={page.faqs} className="platform-faqs" /></div>
          </div>
        </section>

        {related.length ? (
          <section id="more-comparisons" className="platform-soft">
            <div className="wrap">
              <div className="platform-section-intro">
                <SectionHead eyebrow="Detailed comparisons" title="Compare Helpin one to one." />
                <Link className="platform-text-link" href="/compare">All comparisons<ArrowRight size={15} aria-hidden="true" /></Link>
              </div>
              <div className="cmp-card-grid">{related.map(item => <CompareCard key={item.slug} competitor={item} />)}</div>
            </div>
          </section>
        ) : null}

        <section id="about-this-comparison" className="cmp-sources">
          <div className="wrap">
            <h2>About this list</h2>
            <p>Written by the Helpin team from each product’s public pricing and documentation, verified {checked}. We make Helpin, which is why it’s listed first. Prices are in US dollars, exclude tax and change often, so check each vendor’s site. All product names are trademarks of their owners; Helpin is not affiliated with them. Spot something out of date? Email <a href="mailto:hello@helpin.ai">hello@helpin.ai</a>.</p>
          </div>
        </section>

        <PlatformClosing id="alternatives-final-title" eyebrow="14-day free trial" title="See if Helpin fits your team." description="Start a 14-day trial of Helpin Cloud with no card, or self-host the open-source edition for free." primaryLabel="Start free trial" primaryHref={SIGNUP_URL} />
      </div>
      <PreviewFooter />
    </>
  );
}
