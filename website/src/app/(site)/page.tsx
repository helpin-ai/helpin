import Link from 'next/link';
import { Braces, Database, Plug, Server, SlidersHorizontal, Webhook } from 'lucide-react';
import { CustomerLogos } from './_components/CustomerLogos';
import { PreviewNav } from './_components/PreviewNav';
import { ConnectedWorkspace } from './_components/ConnectedWorkspace';
import { PreviewFooter } from './_components/PreviewFooter';
import { HeroHeadline } from './_components/HeroHeadline';
import { HeroVortex } from './_components/HeroVortex';
import { ProductPreview } from './_components/product-previews';
import { ProductExplorer } from './_components/ProductExplorer';
import { HostingDiagram } from './_components/HostingDiagram';
import { AgentDirectoryPreview } from './_components/agent-directory/AgentDirectoryPreview';
import { AgentControlArt } from './_components/AgentControlArt';
import { AskAgentBento } from './_components/AskAgentBento';
import { CustomerRecordBento } from './_components/CustomerRecordBento';
import { CtaRow, GithubIcon, SectionHead, GITHUB_URL } from './_components/ui';
import { DOCS } from './_components/docsLinks';
import { createPageMetadata, PAGE_SEO, SITE_URL } from '@/lib/metadata';
import { JsonLd, organization, softwareApplication, website } from '@/lib/structured-data';
import { PLANS } from '../pricing/pricing-data';

export const metadata = createPageMetadata(PAGE_SEO.home);
const HERO_VIDEO = '/new/home/helpin-launch-1080p-v1.mp4';
const HERO_POSTER = '/new/home/helpin-launch-poster-1600-v1.webp';
const HOME_JSON_LD = {
  '@context': 'https://schema.org',
  '@graph': [organization, website, softwareApplication(PLANS), {
    '@type': 'VideoObject',
    '@id': `${SITE_URL}/#intro-video`,
    name: 'Introducing Helpin',
    description: 'An introduction to Helpin, bringing customer support, projects, and customer records into one workspace.',
    thumbnailUrl: `${SITE_URL}${HERO_POSTER}`,
    contentUrl: `${SITE_URL}${HERO_VIDEO}`,
    uploadDate: '2026-10-02T16:50:29Z',
    duration: 'PT1M2.4S',
    inLanguage: 'en',
  }],
};

const RECORD_FACTS = [
  {
    "title": "Conversations",
    "description": "What they asked, what you answered, and what still needs attention.",
    "image": "conversations",
    "wide": true
  },
  {
    "title": "Meetings",
    "description": "The decisions and commitments behind the next step—not just a recording.",
    "image": "meetings",
    "wide": false
  },
  {
    "title": "Projects",
    "description": "The priorities, owners, and progress behind the work a customer is waiting for.",
    "image": "projects",
    "wide": false
  },
  {
    "title": "Engineering",
    "description": "The original request, proposed changes, and review status—linked to the work.",
    "image": "coding",
    "wide": true
  },
  {
    "title": "Deals",
    "description": "The conversations, concerns, and open work behind a sale or renewal.",
    "image": "deals",
    "wide": false
  },
  {
    "title": "Knowledge",
    "description": "The guidance you’ve shared and the questions your docs still need to answer.",
    "image": "docs",
    "wide": false
  },
  {
    "title": "Email & calendar",
    "description": "Customer emails and meetings alongside the account—not in a separate history.",
    "image": "email-calendar",
    "wide": true
  }
] as const;


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
            <span className="eyebrow">AI-FIRST, OPEN-SOURCE ALTERNATIVE TO <Link className="eyebrow-link" href="/compare/intercom">INTERCOM</Link> AND <Link className="eyebrow-link" href="/compare/linear">LINEAR</Link></span>
            <HeroHeadline />
            <p className="lede">Let AI agents answer customers, keep help docs current, follow up with leads, update customer records, and turn requests into code, all in one workspace. You decide what needs approval; your agents take care of the rest.</p>
            <CtaRow primaryLabel="Start free trial" />
            <p className="cta-note">Open source · Self-host free, or let us run it</p>

          </div>
          <figure className="hero-workflow hero-film" id="intro-video">
            <video
              controls
              playsInline
              preload="none"
              width={1920}
              height={1080}
              poster={HERO_POSTER}
              aria-label="Introducing Helpin"
              aria-describedby="intro-video-caption"
            >
              <source src={HERO_VIDEO} type="video/mp4" />
              <a href={HERO_VIDEO}>Watch the Helpin introduction</a>
            </video>
            <figcaption id="intro-video-caption">Introducing Helpin · 1 minute</figcaption>
          </figure>
          <div className="hero-evaluation">
            <p className="hero-replaces">Move off separate support, project, and CRM tools, or connect the ones you keep.</p>
            <CustomerLogos />
          </div>
        </div>
      </section>

      {/* Shared customer history: the context behind the work. */}
      <section id="record" className="dark record-motion">
        <HeroVortex variant="converge" tone="dark" />
        <div className="wrap">
          <div className="rs-copy record-intro">
            <span className="rs-eyebrow">One customer. One record.</span>
            <h2>More than the last message.</h2>
            <p>Keep conversations, meeting decisions, deals, and linked work together—so your team and agents can see what was asked, what was promised, and what happened next.</p>
          </div>
          <div className="record-bento">
            {RECORD_FACTS.map(({ title, description, image, wide }) => (
              <article className={`record-bento-card${wide ? ' record-bento-wide' : ''}`} key={title}>
                <div className="record-bento-copy">
                  <h3>{title}</h3>
                  <p>{description}</p>
                </div>
                <CustomerRecordBento variant={image} />
              </article>
            ))}
          </div>
          <p className="rs-close">So the next answer builds on what happened before.</p>
        </div>
      </section>

      {/* Ask Agent: answers, execution, and connected tools. */}
      <section id="ask-agent" className="ask-agent-section section-motion" aria-labelledby="ask-agent-title"><HeroVortex variant="converge" tone="dark" />
        <div className="wrap">
          <div className="ask-agent-intro">
            <span className="eyebrow">Ask Agent</span>
            <h2 id="ask-agent-title">Ask a question.<br />Put the answer to work.</h2>
            <p className="lede">Ask Agent answers from your customer history, or coordinates specialist agents to investigate and plan the next step.</p>
          </div>
          <figure className="ask-agent-preview">
            <ProductPreview product="ask-agent" theme="light" />
            <figcaption>An example rollout review: specialist findings, a work plan, and a customer update ready for review.</figcaption>
          </figure>
          <div className="ask-agent-capabilities">
            <article>
              <span className="ask-agent-label">Your workspace</span>
              <h3>Understand what’s happening.</h3>
              <p>Bring the customer’s question, the meeting decision, and the linked task into one answer—so you can see the situation before deciding what to do.</p>
              <AskAgentBento variant="answers" />
              <div className="ask-agent-example"><span>Try asking</span><blockquote>What’s blocking Northstar Labs’ rollout, and what have we already tried?</blockquote></div>
            </article>
            <article>
              <span className="ask-agent-label">Your agents</span>
              <h3>Turn the findings into a plan.</h3>
              <p>Let specialist agents investigate different parts of a request, run independent steps in parallel, and bring their findings back together.</p>
              <AskAgentBento variant="coordination" />
              <div className="ask-agent-example"><span>Try asking</span><blockquote>Review the blockers, propose the next steps, and draft an update for my review.</blockquote></div>
            </article>
            <article>
              <span className="ask-agent-label">Connected tools</span>
              <h3>Check the systems behind the answer.</h3>
              <p>Give agents access to selected tools outside Helpin, so they can check the relevant details without leaving the conversation.</p>
              <AskAgentBento variant="mcp" />
              <div className="ask-agent-example"><span>With a connected agent</span><blockquote>Check the linked issue in our connected tracker before drafting the update.</blockquote></div>
            </article>
          </div>
          <div className="ask-agent-links"><a className="btn-link" href="/products/ai-agents">Explore agents →</a></div>
        </div>
      </section>

      {/* Explore each product area with the shared interactive previews. */}
      <section id="product">
        <div className="wrap">
          <SectionHead eyebrow="What’s inside" title="Run support, projects, and sales together." lede="From the shared inbox to sprint planning and sales pipelines, give each team a place to work—and a shared view of the customer." />
          <ProductExplorer />
        </div>
      </section>

      {/* Agent controls */}
      <section id="control">
        <div className="wrap">
          <SectionHead eyebrow="Approvals built in" title="Agents take action. You set the limits."
            lede="Choose each agent’s tools, decide when it needs approval, and set what starts a run. Delegate the task without giving up control of the process." />
          <figure className="control-screenshot">
            <AgentDirectoryPreview />
            <figcaption>See your agents, follow their runs, and review actions waiting for approval.</figcaption>
          </figure>
          <div className="ctrl">
            <article>
              <AgentControlArt variant="tools" />
              <div className="ctrl-copy">
                <span className="k">Tool access</span>
                <h3>Give each agent the tools it needs.</h3>
                <p className="ctrl-description">Select the workspace tools and connected services it can use. Keep access scoped to the job.</p>
              </div>
            </article>
            <article>
              <AgentControlArt variant="approvals" />
              <div className="ctrl-copy">
                <span className="k">Approvals</span>
                <h3>Choose when it asks first.</h3>
                <p className="ctrl-description">Set the approval mode for each agent. Review actions that require permission before the agent proceeds.</p>
              </div>
            </article>
            <article>
              <AgentControlArt variant="triggers" />
              <div className="ctrl-copy">
                <span className="k">Triggers</span>
                <h3>Start work at the right moment.</h3>
                <p className="ctrl-description">Run agents from supported events or a schedule, using the tools and approval settings you’ve chosen.</p>
              </div>
            </article>
          </div>
          <p className="section-close"><a className="inline-link" href="/products/ai-agents">Explore agents and automation →</a></p>
        </div>
      </section>

      {/* Self-hosting */}
      <section id="open-source" className="dark self-host-section section-motion"><HeroVortex variant="converge" tone="dark" />
        <div className="wrap">
          <div className="self-host-intro">
            <div>
              <SectionHead eyebrow="Open source" title="Your customer history stays yours."
                lede="Self-host the complete product for free, with every module and no plan limits. Or let Helpin Cloud run it, with AI included." />
              <div className="links"><a className="btn btn-primary" href={DOCS.selfHosting}>Self-host Helpin →</a><a className="btn-link" href={GITHUB_URL} target="_blank" rel="noopener noreferrer"><GithubIcon />View the code →</a></div>
              <p className="self-host-license">AGPL-3.0 · Community 0.2 beta</p>
            </div>
            <HostingDiagram />
          </div>
          <div className="self-host-features">
            <article><Server size={23} aria-hidden="true" /><h3>Deploy with Docker Compose.</h3><p>Install Helpin on your infrastructure using the documented self-hosting setup.</p></article>
            <article><Database size={23} aria-hidden="true" /><h3>Control your data and backups.</h3><p>Manage your customer records, files, and backup process on the infrastructure you run.</p></article>
            <article><SlidersHorizontal size={23} aria-hidden="true" /><h3>Choose your AI provider.</h3><p>Connect supported providers and select the models your agents use.</p></article>
          </div>
        </div>
      </section>

      <section id="developers" className="developer-section">
        <div className="wrap">
          <SectionHead eyebrow="For developers" title="Connect the tools you keep."
            lede="Embed support in your product, connect customer data, and let Helpin work with your existing AI tools and services." />
          <article className="developer-cli-feature">
            <figure className="developer-visual">
              <a href="/new/product/helpin-cli-tilted-4k-v1.webp" target="_blank" rel="noopener noreferrer" aria-label="View the Helpin CLI illustration at full size (opens in a new tab)">
                <img src="/new/product/helpin-cli-tilted-1920-v1.webp" srcSet="/new/product/helpin-cli-tilted-1920-v1.webp 1920w, /new/product/helpin-cli-tilted-4k-v1.webp 3840w" sizes="(max-width: 700px) calc(100vw - 78px), (max-width: 1120px) 55vw, 590px" width={3840} height={2160} alt="An angled Helpin CLI terminal showing installation checks, diagnostics, status, and logs, with a softly blurred right edge." loading="lazy" decoding="async" />
              </a>
            </figure>
            <div className="developer-cli-copy">
              <span className="eyebrow">Helpin CLI</span>
              <h3>Run your instance from the terminal.</h3>
              <p>Install Helpin, check your services, and inspect logs with the Helpin CLI.</p>
              <a className="btn-link" href={DOCS.selfHostingInstall}>Explore the Helpin CLI →</a>
            </div>
          </article>
          <div className="developer-features">
            <article>
              <span className="developer-feature-icon"><Braces size={22} aria-hidden="true" /></span>
              <h3>APIs &amp; SDKs</h3><p>Bring customer data into Helpin and add support chat to your app.</p>
              <a className="inline-link" href={DOCS.developer}>Developer docs →</a>
              <a className="inline-link" href={DOCS.widgetSdk}>Explore the SDK →</a>
            </article>
            <article>
              <span className="developer-feature-icon"><Plug size={22} aria-hidden="true" /></span>
              <h3>Connect your AI tools</h3><p>Give Helpin agents selected tools from external systems. Use public MCP to make permitted Helpin context available to connected AI clients.</p><p className="developer-availability">Public MCP is in controlled beta.</p>
              <a className="inline-link" href={DOCS.mcpServer}>Connect AI tools →</a>
              <a className="inline-link" href={DOCS.externalMcp}>Connect external tools →</a>
            </article>
            <article>
              <span className="developer-feature-icon"><Webhook size={22} aria-hidden="true" /></span>
              <h3>Start work from an event</h3><p>Connect supported product and engineering events to agent workflows. Set the rules that determine when a run begins.</p>
              <a className="inline-link" href={DOCS.automation}>Explore automation →</a>
            </article>
          </div>
        </div>
      </section>

      {/* 13 Final CTA */}
      <section className="final-cta final-cta-connected" aria-labelledby="final-cta-title">
        <div className="wrap">
          <ConnectedWorkspace />
          <div className="final">
            <span className="eyebrow">14-day free trial</span>
            <h2 id="final-cta-title">Put your customer history to work.</h2>
            <p className="lede">No card required. Every module, unlimited teammates, and AI usage included.</p>
            <CtaRow primaryLabel="Start free trial" />
            <div className="assure"><span>Open source</span><span>Self-host free, or let us run it</span><span>Built for SaaS teams</span></div>
          </div>
        </div>
      </section>

      <PreviewFooter homepage />
    </>
  );
}
