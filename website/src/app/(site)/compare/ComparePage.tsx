import Link from 'next/link';
import { ArrowRight, Check, ChevronRight } from 'lucide-react';
import { HeroVortex } from '../_components/HeroVortex';
import { PreviewNav } from '../_components/PreviewNav';
import { PreviewFooter } from '../_components/PreviewFooter';
import { PlatformClosing } from '../_components/platform/PlatformParts';
import { CtaRow, DEMO_URL, FAQList, SectionHead, SIGNUP_URL } from '../_components/ui';
import { COMPETITORS, formatChecked, type Cell, type Competitor } from './compare-data';

function Value({ value }: { value: Cell }) {
  if (value === true) return <span className="compare-yes"><Check size={17} strokeWidth={2} aria-hidden="true" /><span className="sr-only">Yes</span></span>;
  if (value === false) return <span className="compare-no"><span aria-hidden="true">—</span><span className="sr-only">No</span></span>;
  return <>{value}</>;
}

export function ComparisonTable({ competitor }: { competitor: Competitor }) {
  return (
    <table className="compare-table">
      <caption className="sr-only">Helpin and {competitor.name} compared. Checkmarks mean available and dashes mean not available.</caption>
      <colgroup><col className="compare-feature-column" /><col /><col /></colgroup>
      <thead><tr><th scope="col"><span className="sr-only">Feature</span></th><th scope="col">Helpin</th><th scope="col">{competitor.name}</th></tr></thead>
      {competitor.table.map(group => (
        <tbody key={group.group}>
          <tr className="compare-table-group"><th scope="rowgroup" colSpan={3}>{group.group}</th></tr>
          {group.rows.map(row => (
            <tr key={row.label}>
              <th scope="row">{row.label}</th>
              <td data-product="helpin"><Value value={row.helpin} /></td>
              <td><Value value={row.competitor} /></td>
            </tr>
          ))}
        </tbody>
      ))}
    </table>
  );
}

export function ComparePage({ competitor }: { competitor: Competitor }) {
  const others = COMPETITORS.filter(item => item.slug !== competitor.slug);
  return (
    <>
      <PreviewNav />
      <div className="compare-page">
        <section className="compare-hero">
          <HeroVortex variant="orbit" tone="dark" />
          <div className="wrap">
            <div className="compare-breadcrumb"><Link href="/">Helpin</Link><ChevronRight size={12} aria-hidden="true" /><Link href="/compare">Compare</Link><ChevronRight size={12} aria-hidden="true" /><span>{competitor.name}</span></div>
            <span className="eyebrow">Compare · {competitor.category}</span>
            <h1>{competitor.hero.title}</h1>
            <p className="lede">{competitor.hero.lede}</p>
            <CtaRow primaryLabel="Start free trial" primaryHref={SIGNUP_URL} secondaryHref={DEMO_URL} secondaryLabel="Book a demo" />
            <p className="compare-checked">Comparison checked {formatChecked(competitor.checked)} against {competitor.name}’s public pages. <a href="#sources">Sources</a></p>
          </div>
        </section>

        <section id="short-answer" className="compare-short">
          <div className="wrap">
            <SectionHead eyebrow="The short answer" title={competitor.summary.title} lede={competitor.summary.lede} />
            <div className="compare-choose">
              <article>
                <h3>Choose {competitor.name} if</h3>
                <ul>{competitor.summary.competitor.map(item => <li key={item}>{item}</li>)}</ul>
              </article>
              <article data-product="helpin">
                <h3>Choose Helpin if</h3>
                <ul>{competitor.summary.helpin.map(item => <li key={item}>{item}</li>)}</ul>
              </article>
            </div>
          </div>
        </section>

        <section id="at-a-glance">
          <div className="wrap">
            <SectionHead eyebrow="At a glance" title={`Helpin and ${competitor.name}, side by side.`} lede={competitor.tableLede} />
            <ComparisonTable competitor={competitor} />
          </div>
        </section>

        <section id="differences" className="compare-soft">
          <div className="wrap">
            <SectionHead eyebrow="Where they differ" title={competitor.differencesTitle} />
            <div className="compare-differences">
              {competitor.differences.map(item => <article key={item.title}><h3>{item.title}</h3><p>{item.body}</p></article>)}
            </div>
          </div>
        </section>

        <section id="strengths">
          <div className="wrap compare-split">
            <SectionHead eyebrow="Fair is fair" title={`Where ${competitor.name} is stronger.`} lede={`${competitor.name} is a good product. These are the areas where it is ahead today.`} />
            <ul className="compare-strengths">{competitor.strengths.map(item => <li key={item.title}><strong>{item.title}</strong> {item.body}</li>)}</ul>
          </div>
        </section>

        {competitor.cost ? (
          <section id="cost" className="compare-soft">
            <div className="wrap">
              <SectionHead eyebrow="Pricing" title={competitor.cost.title} lede={competitor.cost.lede} />
              <div className="compare-cost">
                <article data-product="helpin"><span>Helpin</span><strong>{competitor.cost.helpin.amount}</strong><p>{competitor.cost.helpin.detail}</p></article>
                <article><span>{competitor.name}</span><strong>{competitor.cost.competitor.amount}</strong><p>{competitor.cost.competitor.detail}</p></article>
              </div>
              <p className="compare-note">{competitor.cost.note} <Link href="/pricing">See Helpin pricing</Link></p>
            </div>
          </section>
        ) : null}

        <section id="switching">
          <div className="wrap compare-split">
            <SectionHead eyebrow="Switching" title={competitor.switching.title} />
            <div className="compare-switching">{competitor.switching.body.map(paragraph => <p key={paragraph}>{paragraph}</p>)}</div>
          </div>
        </section>

        <section id="questions" className="compare-soft">
          <div className="wrap compare-faq">
            <SectionHead eyebrow="Questions" title={`Helpin vs ${competitor.name}: common questions.`} />
            <FAQList items={competitor.faqs} className="compare-faq-items" />
          </div>
        </section>

        <section id="more-comparisons">
          <div className="wrap">
            <SectionHead eyebrow="More comparisons" title="See how Helpin compares with other tools." />
            <div className="compare-cards">
              {others.map(item => (
                <Link key={item.slug} className="compare-card" href={`/compare/${item.slug}`}>
                  <span>{item.category}</span>
                  <strong>Helpin vs {item.name}</strong>
                  <ArrowRight size={16} aria-hidden="true" />
                </Link>
              ))}
            </div>
          </div>
        </section>

        <section id="sources" className="compare-sources">
          <div className="wrap">
            <h2>Sources</h2>
            <p>Checked {formatChecked(competitor.checked)}. Prices are in US dollars and exclude tax. Products change, so check {competitor.name}’s site for current details. {competitor.name} is a trademark of its owner; Helpin is not affiliated with it. Spot something out of date? Email <a href="mailto:hello@helpin.ai">hello@helpin.ai</a>.</p>
            <ul>{competitor.sources.map(source => <li key={source.url}><a href={source.url} target="_blank" rel="noopener noreferrer nofollow">{source.label}</a></li>)}</ul>
          </div>
        </section>

        <PlatformClosing id="compare-final-title" eyebrow="Try it yourself" title={competitor.closing.title} description={competitor.closing.description} primaryLabel="Start free trial" primaryHref={SIGNUP_URL} />
      </div>
      <PreviewFooter />
    </>
  );
}
