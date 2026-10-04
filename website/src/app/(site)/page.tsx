import Link from 'next/link';
import { Braces, Database, Plug, Server, SlidersHorizontal, Webhook } from 'lucide-react';
import { CustomerLogos } from './_components/CustomerLogos';
import { PreviewNav } from './_components/PreviewNav';
import { ConnectedWorkspace } from './_components/ConnectedWorkspace';
import { PreviewFooter } from './_components/PreviewFooter';
import { HeroHeadline } from './_components/HeroHeadline';
import { HeroVideo } from './_components/HeroVideo';
import { HeroVortex } from './_components/HeroVortex';
import { ProductPreview } from './_components/product-previews';
import { ProductExplorer } from './_components/ProductExplorer';
import { HostingDiagram } from './_components/HostingDiagram';
import { AgentDirectoryPreview } from './_components/agent-directory/AgentDirectoryPreview';
import { AgentControlArt } from './_components/AgentControlArt';
import { AskAgentBento } from './_components/AskAgentBento';
import { CustomerRecordBento } from './_components/CustomerRecordBento';
import { CtaRow, GithubIcon, SectionHead, GITHUB_URL, SIGNUP_URL } from './_components/ui';
import { DOCS } from './_components/docsLinks';
import { createPageMetadata, PAGE_SEO, SITE_URL } from '@/lib/metadata';
import { JsonLd, organization, softwareApplication, website } from '@/lib/structured-data';
import { PLANS } from '../pricing/pricing-data';

export const metadata = createPageMetadata(PAGE_SEO.home);
const HERO_VIDEO = '/new/home/helpin-launch-1080p-v1.mp4';
const HERO_POSTER = '/new/home/helpin-launch-poster-1600-v3.webp';
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
    "title": "AI agents answer with the full customer history.",
    "description": "Past questions, replies, and attempted fixes help AI agents understand each customer’s issue, so customers get relevant help without explaining the same problem again.",
    "image": "conversations",
    "wide": true
  },
  {
    "title": "Keep meeting decisions from getting lost.",
    "description": "AI agents can use meeting notes and agreed next steps to help your team follow through, without replaying recordings or asking everyone what was decided.",
    "image": "meetings",
    "wide": false
  },
  {
    "title": "Help AI agents solve the right problem.",
    "description": "Linked customer requests help AI agents understand why a task matters, who is waiting, and what needs to change before they plan the work.",
    "image": "projects",
    "wide": false
  },
  {
    "title": "AI agents turn customer reports into fixes.",
    "description": "AI agents can plan, code, and review fixes using customer requests and linked tasks. Your team can check the proposed changes before approving them.",
    "image": "coding",
    "wide": true
  },
  {
    "title": "Know what buyers need before following up.",
    "description": "AI agents can use buyers’ questions, concerns, and past commitments to suggest relevant follow-ups, helping your team address what is holding up each deal.",
    "image": "deals",
    "wide": false
  },
  {
    "title": "Put your company’s knowledge to work with AI.",
    "description": "Product guides and internal docs spaces give AI agents the facts and instructions to answer questions and complete tasks without repeated explanations from your team.",
    "image": "docs",
    "wide": false
  },
  {
    "title": "Get a meeting brief from your AI agents.",
    "description": "AI agents can use emails and calendar events to help you prepare for meetings, so you know what was agreed and what still needs attention.",
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
            <span className="eyebrow">AI-first, open-source alternative to <Link className="eyebrow-link" href="/compare/intercom">Intercom</Link> and <Link className="eyebrow-link" href="/compare/linear">Linear</Link></span>
            <HeroHeadline />
            <p className="lede">Let AI agents answer customers, keep help docs current, follow up with leads, update customer records, and turn requests into code, all in one workspace. You decide what needs approval; your agents take care of the rest.</p>
            <CtaRow primaryLabel="Start free trial" />
            <p className="cta-note">Open source · Self-host free, or let us run it</p>

          </div>
          <HeroVideo src={HERO_VIDEO} poster={HERO_POSTER} />
          <div className="hero-evaluation">
            <CustomerLogos />
          </div>
        </div>
      </section>

      {/* Shared customer history: the context behind the work. */}
      <section id="record" className="dark record-motion">
        <HeroVortex variant="converge" tone="dark" />
        <div className="wrap">
          <div className="rs-copy record-intro">
            <span className="rs-eyebrow">Your company’s brain</span>
            <h2>Your team and AI agents can find what they need in one place.</h2>
            <p>Helpin brings your customer conversations, meeting notes, docs, and project updates together. Your team and AI agents can use this information to answer questions without searching through separate tools or asking customers to repeat themselves.</p>
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
          <div className="home-section-actions">
            <Link prefetch={false} className="btn btn-primary" href={SIGNUP_URL}>Start free trial →</Link>
            <Link prefetch={false} className="btn-link" href="/products/knowledge">Explore knowledge and docs →</Link>
          </div>
        </div>
      </section>

      {/* Ask Agent: answers, execution, and connected tools. */}
      <section id="ask-agent" className="ask-agent-section section-motion" aria-labelledby="ask-agent-title"><HeroVortex variant="converge" tone="dark" />
        <div className="wrap">
          <div className="ask-agent-intro">
            <span className="eyebrow">Helpin AI</span>
            <h2 id="ask-agent-title">Imagine what you could get done with Helpin AI.</h2>
            <p className="lede">You can ask about a customer, investigate a bug, or prepare a follow-up in a conversation. Helpin AI uses your company’s information and brings in specialist agents when the task needs them.</p>
          </div>
          <figure className="ask-agent-preview">
            <ProductPreview product="ask-agent" theme="light" />
            <figcaption>See how Helpin AI checks a reported bug and prepares a plan and customer reply.</figcaption>
          </figure>
          <div className="ask-agent-capabilities">
            <article>
              <span className="ask-agent-label">Your workspace</span>
              <h3>Get answers from your company’s information.</h3>
              <p>Helpin AI can look through conversations, meeting notes, docs, and tasks. Ask what a customer needs or why a project is delayed without searching each tool yourself.</p>
              <AskAgentBento variant="answers" />
            </article>
            <article>
              <span className="ask-agent-label">Your agents</span>
              <h3>Finish complex tasks with specialist agents.</h3>
              <p>Helpin AI can ask Scribe to <span className="home-feature">plan a fix</span>, Forge to <span className="home-feature">write code</span>, and Quill to <span className="home-feature">update the guide</span>. You can review their work in Helpin.</p>
              <AskAgentBento variant="coordination" />
            </article>
            <article>
              <span className="ask-agent-label">Connected tools · Beta</span>
              <h3>Check account details before you reply.</h3>
              <p>Connect tools for <span className="home-feature">billing, error logs, or issue tracking</span> so agents can check the facts behind a customer’s question. You choose which tools they can use and what they can change.</p>
              <AskAgentBento variant="mcp" />
            </article>
          </div>
          <div className="ask-agent-links home-section-actions">
            <Link prefetch={false} className="btn btn-primary" href={SIGNUP_URL}>Start free trial →</Link>
            <Link prefetch={false} className="btn-link" href="/products/ai-agents">Meet the AI agents →</Link>
          </div>
        </div>
      </section>

      {/* Explore each product area with the shared interactive previews. */}
      <section id="product">
        <div className="wrap">
          <SectionHead eyebrow="What’s inside" title="You can manage support, development, sales and marketing in the same place." lede="Your team gets the inboxes, task boards, CRM, docs, and meeting notes it needs for day-to-day work. AI agents help with replies, updates, and follow-ups inside those tools." />
          <ProductExplorer />
          <div className="home-section-actions">
            <Link prefetch={false} className="btn btn-primary" href={SIGNUP_URL}>Start free trial →</Link>
            <Link prefetch={false} className="btn-link" href="/pricing">See pricing →</Link>
          </div>
        </div>
      </section>

      {/* Agent controls */}
      <section id="control">
        <div className="wrap">
          <SectionHead eyebrow="Approvals built in" title="You decide what agents can do on their own."
            lede="Choose what runs automatically and what needs your approval. You can see each agent’s activity, check its results, and step in when a task needs your attention." />
          <figure className="control-screenshot">
            <AgentDirectoryPreview showFlows />
            <figcaption>Explore your agents and flow templates, with your team in control of approvals.</figcaption>
          </figure>
          <div className="ctrl">
            <article>
              <AgentControlArt variant="tools" />
              <div className="ctrl-copy">
                <span className="k">Tools &amp; skills</span>
                <h3>Give agents the tools and skills they need.</h3>
                <p className="ctrl-description">Choose which docs, Helpin features, and connected services each agent can use. Add <span className="home-feature">built-in skills</span> or <span className="home-feature">bring your own skills</span> to guide how it does the work.</p>
              </div>
            </article>
            <article id="approval-demo">
              <AgentControlArt variant="approvals" />
              <div className="ctrl-copy">
                <span className="k">Approvals</span>
                <h3>Choose which actions need your approval.</h3>
                <p className="ctrl-description">Set <span className="home-feature">approval rules</span> for each agent so you can review the actions that matter to you before they happen.</p>
              </div>
            </article>
            <article>
              <AgentControlArt variant="triggers" />
              <div className="ctrl-copy">
                <span className="k">Triggers</span>
                <h3>Run agents automatically when work needs attention.</h3>
                <p className="ctrl-description"><span className="home-feature">Schedule a regular check</span> or start an agent when a <span className="home-feature">task changes stage</span> or a <span className="home-feature">pull request needs review</span>, so you don’t have to start every job yourself.</p>
              </div>
            </article>
          </div>
          <div className="home-section-actions">
            <Link prefetch={false} className="btn btn-primary" href={SIGNUP_URL}>Start free trial →</Link>
            <Link prefetch={false} className="btn-link" href="/products/ai-agents">Explore agents and automation →</Link>
          </div>
        </div>
      </section>

      {/* Self-hosting */}
      <section id="open-source" className="dark self-host-section section-motion"><HeroVortex variant="converge" tone="dark" />
        <div className="wrap">
          <div className="self-host-intro">
            <div>
              <SectionHead eyebrow="Open source" title="We can host Helpin for you, or you can run it yourself."
                lede="Choose Helpin Cloud if you want us to manage the hosting, with AI usage included. Choose the open-source Community edition if you want to run Helpin on your own servers and connect your own AI providers." />
              <div className="links"><a className="btn btn-primary" href={DOCS.selfHosting}>Self-host Helpin →</a><a className="btn-link" href={GITHUB_URL} target="_blank" rel="noopener noreferrer"><GithubIcon />View the code →</a></div>
              <p className="self-host-license">AGPL-3.0 · Community 0.2 beta</p>
            </div>
            <HostingDiagram />
          </div>
          <div className="self-host-features">
            <article><Server size={23} aria-hidden="true" /><h3>Deploy with Docker Compose.</h3><p>Use the installation guide to set up Helpin on your servers. Your team manages the installation and covers hosting and AI provider costs.</p></article>
            <article><Database size={23} aria-hidden="true" /><h3>Control your data and backups.</h3><p>Keep your customer records and files on the servers you manage, and choose how you back them up.</p></article>
            <article><SlidersHorizontal size={23} aria-hidden="true" /><h3>Choose your AI provider.</h3><p>When you self-host, you can connect a supported AI provider and choose the models your agents use.</p></article>
          </div>
        </div>
      </section>

      <section id="developers" className="developer-section">
        <div className="wrap">
          <SectionHead eyebrow="For developers" title="Connect Helpin to your AI agents and development tools."
            lede="Use your preferred AI agents with Helpin, add support chat to your app, and automate work from task and code changes." />
          <article className="developer-cli-feature">
            <figure className="developer-visual">
              <a href="/new/product/helpin-cli-tilted-4k-v1.webp" target="_blank" rel="noopener noreferrer" aria-label="View the Helpin CLI illustration at full size (opens in a new tab)">
                <img src="/new/product/helpin-cli-tilted-1920-v1.webp" srcSet="/new/product/helpin-cli-tilted-1920-v1.webp 1920w, /new/product/helpin-cli-tilted-4k-v1.webp 3840w" sizes="(max-width: 700px) calc(100vw - 78px), (max-width: 1120px) 55vw, 590px" width={3840} height={2160} alt="An angled Helpin CLI terminal showing installation checks, diagnostics, status, and logs, with a softly blurred right edge." loading="lazy" decoding="async" />
              </a>
            </figure>
            <div className="developer-cli-copy">
              <span className="eyebrow">Helpin CLI</span>
              <h3>Install and manage Helpin from your terminal.</h3>
              <p>Use the Helpin command-line tool to install the self-hosted version, check that its services are running, and read logs when something needs attention.</p>
              <a className="btn-link" href={DOCS.selfHostingInstall}>Explore the Helpin CLI →</a>
            </div>
          </article>
          <div className="developer-features">
            <article>
              <span className="developer-feature-icon"><Braces size={22} aria-hidden="true" /></span>
              <h3>Add support chat with customer context.</h3>
              <p>Use Helpin’s <span className="home-feature">APIs and SDKs</span> to add support chat and share customer details from your app. Your team and AI agents can answer with the right account information, so customers don’t have to explain who they are each time.</p>
              <a className="inline-link" href={DOCS.developer}>Developer docs →</a>
              <a className="inline-link" href={DOCS.widgetSdk}>Explore the SDK →</a>
            </article>
            <article>
              <span className="developer-feature-icon"><Plug size={22} aria-hidden="true" /></span>
              <h3>Connect external AI agents through MCP.</h3>
              <p>Connect <span className="home-feature">Claude Code</span>, <span className="home-feature">Codex</span>, <span className="home-feature">Cursor</span>, <span className="home-feature">Hermes</span>, and other compatible agents through Helpin’s <span className="home-feature">MCP server</span>. They can read conversations, create tasks, update docs, and run Helpin agents with your chosen permissions. Helpin agents can also use tools from <span className="home-feature">external MCP servers</span>.</p>
              <a className="inline-link" href={DOCS.mcpServer}>Set up Helpin MCP →</a>
              <a className="inline-link" href={DOCS.externalMcp}>Connect external MCP servers →</a>
            </article>
            <article>
              <span className="developer-feature-icon"><Webhook size={22} aria-hidden="true" /></span>
              <h3>Run AI agents when tasks or code change.</h3>
              <p>Start AI agents when tasks change or supported <span className="home-feature">GitHub and GitLab events</span> occur. For example, have an agent <span className="home-feature">review a new pull request</span> or work on a task that enters a chosen stage, without starting each run yourself.</p>
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
            <h2 id="final-cta-title">Spend no time on repeat questions, follow-ups, and updates.</h2>
            <p className="lede">Build your company’s brain, then ask an agent to handle a support question, documentation update, or sales follow-up. AI usage is included in the trial, and you don’t need a card to start.</p>
            <CtaRow primaryLabel="Start free trial" />
            <div className="assure"><span>Open source</span><span>Self-host free, or let us run it</span><span>Built for SaaS teams</span></div>
          </div>
        </div>
      </section>

      <PreviewFooter homepage />
    </>
  );
}
