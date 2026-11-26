import Link from 'next/link';
import { Building2, CalendarDays, Check, DoorOpen, FolderKanban } from 'lucide-react';
import { CustomerLogos } from './_components/CustomerLogos';
import { PreviewNav } from './_components/PreviewNav';
import { ConnectedWorkspace } from './_components/ConnectedWorkspace';
import { PreviewFooter } from './_components/PreviewFooter';
import { HeroHeadline } from './_components/HeroHeadline';
import { HeroVortex } from './_components/HeroVortex';
import { CoverageGapsScene, MergedPrDocsScene } from './_components/docs-paths/DocsPaths';
import { SwitchFlow } from './_components/switch-flow/SwitchFlow';
import { SupportStory } from './_components/support-story/SupportStory';
import { AgentControlArt } from './_components/AgentControlArt';
import { DeskStack } from './_components/desk-stack/DeskStack';
import { GithubIcon, SectionHead, StartForm, GITHUB_URL, SIGNUP_URL, START_URL, LinkArrow } from './_components/ui';
import { createPageMetadata, PAGE_SEO, SITE_URL } from '@/lib/metadata';
import { JsonLd, organization, softwareApplication, website } from '@/lib/structured-data';
import { PLANS } from '../pricing/pricing-data';

export const metadata = createPageMetadata(PAGE_SEO.home);
const HOME_JSON_LD = {
  '@context': 'https://schema.org',
  '@graph': [organization, website, softwareApplication(PLANS)],
};

export default function HomePage() {
  return (
    <>
      <JsonLd data={HOME_JSON_LD} />
      <PreviewNav />

      {/* 01 Hero */}
      <section className="hero homepage-hero">
        <HeroVortex />
        <div className="wrap hero-wrap">
          <div className="hero-inner">
            <span className="eyebrow">AI-first, open-source alternative to <Link className="eyebrow-link" href="/compare/intercom">Intercom</Link></span>
            <HeroHeadline />
            <p className="lede">Answers from your docs right away. Specialist agents and your team take on the hard ones. No <span>per-seat</span> or <span>per-resolution</span> fees.</p>
            <StartForm />
            <p className="start-note">Watch your AI agent answer a customer question from your own docs, before you sign up.</p>
            <p className="cta-note">Or <Link prefetch={false} href={SIGNUP_URL}>start a free trial</Link> · <span>14 days, no card</span> · <span>Open source, self-host free</span></p>

          </div>
          <figure className="hero-workflow hero-film">
            <SupportStory />
          </figure>
          <div className="hero-evaluation">
            <CustomerLogos />
          </div>
        </div>
      </section>

      {/* The support desk: what a support team gets on day one, and the other products behind it. */}
      <section id="support-desk" className="developer-section">
        <div className="wrap">
          <SectionHead eyebrow="The support desk" title="Everything your support team runs on, in one inbox" />
          <DeskStack />
          <div className="home-section-actions">
            <Link prefetch={false} className="btn btn-primary" href={START_URL}>See your agent</Link>
            <Link prefetch={false} className="btn-link" href="/products/customer-support">Explore customer support<LinkArrow /></Link>
          </div>
          <div className="built-in">
            <p className="built-in-title">Built in, for when support needs it</p>
            <div className="built-in-items">
              <Link prefetch={false} href="/products/customer-portal"><DoorOpen size={18} aria-hidden="true" /><span><b>Customer portal</b>Customers sign in to follow their requests and search their history.</span></Link>
              <Link prefetch={false} href="/products/projects"><FolderKanban size={18} aria-hidden="true" /><span><b>Projects</b>The bug behind a conversation becomes a task your team can plan and ship.</span></Link>
              <Link prefetch={false} href="/products/crm"><Building2 size={18} aria-hidden="true" /><span><b>CRM</b>Contacts, companies, and deals next to the conversations behind them.</span></Link>
              <Link prefetch={false} href="/products/meetings"><CalendarDays size={18} aria-hidden="true" /><span><b>Meetings</b>Decisions and next steps from customer calls, linked to the work.</span></Link>
            </div>
          </div>
        </div>
      </section>

      {/* Docs that keep up: the two paths into a help-center update. */}
      <section id="docs-paths" className="docs-paths-section">
        <div className="wrap">
          <SectionHead eyebrow="Docs that keep up" title="Your help center learns from support and from every merged PR"
            lede="Quill, the documentation agent, turns the questions customers keep asking and the changes your team ships into help-center updates. Changes wait for your review unless you let Quill edit directly." />
          <div className="docs-paths">
            <article className="docs-path">
              <div className="docs-path-copy">
                <h3>Turn repeated questions into help articles</h3>
                <p>Helpin groups the questions your AI couldn’t answer well into coverage gaps and shows how many conversations hit each one in the last 30 days. Similar gaps merge, so one article covers all of them.</p>
                <p>Open a gap and click <b>Prepare a fix</b>. Quill reads the conversations, checks your existing guides, and saves a draft for your team to review.</p>
              </div>
              <CoverageGapsScene />
            </article>
            <article className="docs-path docs-path-flip">
              <div className="docs-path-copy">
                <h3>Check the docs whenever a pull request merges</h3>
                <p>Quill reads the changes and the linked tasks, ignores what readers never see, like tests and CI, and proposes edits to the articles the change made wrong. A new workflow gets a new article.</p>
                <p>Each run ends with a report of the articles it changed and the ones it left alone. You can also backfill past pull requests in one run.</p>
              </div>
              <MergedPrDocsScene />
            </article>
          </div>
          <div className="home-section-actions">
            <Link prefetch={false} className="btn-link" href="/products/knowledge">Explore knowledge and docs<LinkArrow /></Link>
          </div>
        </div>
      </section>

      {/* Agent controls */}
      <section id="control">
        <div className="wrap">
          <SectionHead eyebrow="Approvals built in" title="You decide what agents can do on their own"
            lede="Choose what runs automatically and what needs your approval. You can see each agent’s activity, check its results, and step in when a conversation needs you." />
          <div className="ctrl">
            <article>
              <AgentControlArt variant="tools" />
              <div className="ctrl-copy">
                <h3>Choose what your support agent can use</h3>
                <p className="ctrl-description">Pick the docs, Helpin features, and connected tools it can use. Add <span className="home-feature">built-in skills</span> or <span className="home-feature">your own</span> to guide how it answers.</p>
              </div>
            </article>
            <article id="approval-demo">
              <AgentControlArt variant="approvals" />
              <div className="ctrl-copy">
                <h3>Review the replies that matter before they go out</h3>
                <p className="ctrl-description">Set <span className="home-feature">approval rules</span> for each agent, so a teammate checks the replies and actions you care about before they happen.</p>
              </div>
            </article>
            <article>
              <AgentControlArt variant="triggers" />
              <div className="ctrl-copy">
                <h3>Run checks on a schedule</h3>
                <p className="ctrl-description"><span className="home-feature">Schedule a weekly review</span> of your public help center, or start an agent when a <span className="home-feature">pull request merges</span>, so nobody has to start each job.</p>
              </div>
            </article>
          </div>
          <div className="home-section-actions">
            <Link prefetch={false} className="btn-link" href="/products/ai-agents">Explore agents and approvals<LinkArrow /></Link>
          </div>
        </div>
      </section>

      {/* Price and hosting: the page's one dark band, kept away from the dark footer. */}
      <section id="open-source" className="dark self-host-section section-motion"><HeroVortex variant="converge" tone="dark" />
        <div className="wrap">
          <SectionHead eyebrow="Open source" title="No per-seat or per-resolution fees, on Helpin Cloud or your own servers" />
          <div className="run-options">
            <article>
              <h3>Helpin Cloud</h3>
              <p className="run-figure">From ${PLANS[0].annual} a month <small>billed annually</small></p>
              <ul>
                <li><Check size={15} aria-hidden="true" />One price per workspace</li>
                <li><Check size={15} aria-hidden="true" />Unlimited teammates</li>
                <li><Check size={15} aria-hidden="true" />AI usage allowance in every plan</li>
              </ul>
              <div className="run-actions"><Link prefetch={false} className="btn btn-primary" href="/pricing">See pricing</Link></div>
            </article>
            <article>
              <h3>Community edition</h3>
              <p className="run-figure">No license fee</p>
              <ul>
                <li><Check size={15} aria-hidden="true" />Runs on your own servers</li>
                <li><Check size={15} aria-hidden="true" />Your own AI provider and models</li>
                <li><Check size={15} aria-hidden="true" />Every product included</li>
              </ul>
              <div className="run-actions"><Link prefetch={false} className="btn btn-secondary" href="/self-hosting">Self-host Helpin</Link><a className="btn-link" href={GITHUB_URL} target="_blank" rel="noopener noreferrer"><GithubIcon />View the code<LinkArrow external /></a></div>
            </article>
          </div>
          <div className="run-foot">
            <p>AGPL-3.0 · Community 0.2 beta</p>
            <Link prefetch={false} className="btn-link" href="/developers">Building on Helpin? SDK and MCP<LinkArrow /></Link>
          </div>
        </div>
      </section>

      {/* Importers: the help center moves from any tool, conversations from Intercom. */}
      <section id="switch" className="switch-section">
        <div className="wrap switch-grid switch-grid-flow">
          <div className="switch-copy">
            <SectionHead eyebrow="Switching" title="Switch without starting over" tight
              lede="Helpin copies your help center from the tool you use today, by its address. Coming from Intercom, your past conversations move too, so your AI agents answer with that history from the first day." />
            <p className="switch-also">You can also import contacts from HubSpot or a CSV file, and tasks, epics, and workflows from Shortcut.</p>
            <Link prefetch={false} className="btn-link" href="/compare/intercom">See how Helpin compares with Intercom<LinkArrow /></Link>
          </div>
          <SwitchFlow />
        </div>
      </section>

      {/* Final CTA: the same website box the page opened with. */}
      <section className="final-cta final-cta-connected final-cta-start" aria-labelledby="final-cta-title">
        <div className="wrap">
          <ConnectedWorkspace focus="support" />
          <div className="final">
            <h2 id="final-cta-title">See your AI agent answer from your own docs</h2>
            <StartForm />
            <p className="cta-note">Or <Link prefetch={false} href={SIGNUP_URL}>start a free trial</Link> · <span>14 days, no card</span></p>
          </div>
        </div>
      </section>

      <PreviewFooter homepage />
    </>
  );
}
