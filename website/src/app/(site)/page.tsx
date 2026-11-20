import Link from 'next/link';
import { Braces, Building2, CalendarDays, Database, DoorOpen, FolderKanban, Inbox, Plug, Reply, Route, SlidersHorizontal, SquareTerminal, Timer } from 'lucide-react';
import { CustomerLogos } from './_components/CustomerLogos';
import { PreviewNav } from './_components/PreviewNav';
import { ConnectedWorkspace } from './_components/ConnectedWorkspace';
import { PreviewFooter } from './_components/PreviewFooter';
import { HeroHeadline } from './_components/HeroHeadline';
import { HeroVortex } from './_components/HeroVortex';
import { CoverageGapsScene, MergedPrDocsScene } from './_components/docs-paths/DocsPaths';
import { ImportScene } from './_components/import-scene/ImportScene';
import { SupportStory } from './_components/support-story/SupportStory';
import { HostingDiagram } from './_components/HostingDiagram';
import { AgentControlArt } from './_components/AgentControlArt';
import { CustomerRecordBento } from './_components/CustomerRecordBento';
import { CtaRow, GithubIcon, SectionHead, GITHUB_URL, SIGNUP_URL } from './_components/ui';
import { DOCS } from './_components/docsLinks';
import { createPageMetadata, PAGE_SEO, SITE_URL } from '@/lib/metadata';
import { JsonLd, organization, softwareApplication, website } from '@/lib/structured-data';
import { PLANS } from '../pricing/pricing-data';

export const metadata = createPageMetadata(PAGE_SEO.home);
const HOME_JSON_LD = {
  '@context': 'https://schema.org',
  '@graph': [organization, website, softwareApplication(PLANS)],
};

const RECORD_FACTS = [
  {
    title: 'AI agents answer with the full customer history.',
    description: 'Past questions, replies, and attempted fixes help AI agents understand each customer’s issue, so customers get relevant help without explaining the same problem again.',
    image: 'conversations',
  },
  {
    title: 'AI agents turn customer reports into fixes.',
    description: 'AI agents can plan, code, and review fixes using the customer’s report and the linked task. Your team checks the proposed changes before approving them.',
    image: 'coding',
  },
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
            <span className="eyebrow">AI-first, open-source alternative to <Link className="eyebrow-link" href="/compare/intercom">Intercom</Link></span>
            <HeroHeadline />
            <p className="lede">Customers get answers from your docs right away. Questions that need a person reach your team with the AI’s findings attached, so nobody starts over.</p>
            <CtaRow primaryLabel="Start free trial" />
            <p className="cta-note">14-day free trial, no card · Open source, self-host free</p>

          </div>
          <figure className="hero-workflow hero-film">
            <SupportStory />
          </figure>
          <div className="hero-evaluation">
            <CustomerLogos />
          </div>
        </div>
      </section>

      {/* The support desk: what a support team gets on day one. */}
      <section id="support-desk" className="developer-section">
        <div className="wrap">
          <SectionHead eyebrow="The support desk" title="Everything your support team runs on, in one inbox."
            lede="Chat, email, and Telegram arrive in the same place, with response targets, a customer portal, and AI agents that keep a conversation moving after the first reply." />
          <div className="developer-features developer-features-six">
            <article>
              <span className="developer-feature-icon"><Inbox size={22} aria-hidden="true" /></span>
              <h3>One inbox for every channel.</h3>
              <p>Chat, email, and Telegram conversations land in a shared inbox, with team inboxes, assignment, and saved views.</p>
            </article>
            <article>
              <span className="developer-feature-icon"><Route size={22} aria-hidden="true" /></span>
              <h3>Routed to the right team.</h3>
              <p>The AI classifies each new conversation, tags it, and moves it to the team inbox that handles it.</p>
            </article>
            <article>
              <span className="developer-feature-icon"><Timer size={22} aria-hidden="true" /></span>
              <h3>SLAs your team can act on.</h3>
              <p>Set first-response and next-response targets by policy. Helpin warns your team before a target is missed.</p>
            </article>
            <article>
              <span className="developer-feature-icon"><Reply size={22} aria-hidden="true" /></span>
              <h3>AI agents that follow up.</h3>
              <p>The Echo agent answers from your docs, checks whether the customer still needs help, and hands over with what it found.</p>
            </article>
            <article>
              <span className="developer-feature-icon"><Plug size={22} aria-hidden="true" /></span>
              <h3>Check account details before you reply.</h3>
              <p>Connect tools for billing, error logs, or issue tracking so agents can check the facts behind a question. You choose what they can use and change.</p>
            </article>
            <article>
              <span className="developer-feature-icon"><DoorOpen size={22} aria-hidden="true" /></span>
              <h3>A portal for your customers.</h3>
              <p>Customers sign in to follow their requests, search their history, and read help written for them.</p>
              <Link prefetch={false} className="inline-link" href="/products/customer-portal">Explore the portal →</Link>
            </article>
          </div>
          <div className="home-section-actions">
            <Link prefetch={false} className="btn btn-primary" href={SIGNUP_URL}>Start free trial →</Link>
            <Link prefetch={false} className="btn-link" href="/products/customer-support">Explore customer support →</Link>
          </div>
        </div>
      </section>

      {/* When a question is a bug: history in, fix out, and the rest of the workspace behind it. */}
      <section id="record" className="dark record-motion">
        <HeroVortex variant="converge" tone="dark" />
        <div className="wrap">
          <div className="rs-copy record-intro">
            <span className="rs-eyebrow">When it’s a bug</span>
            <h2>When a question turns out to be a bug, the fix stays attached.</h2>
            <p>AI agents answer with the customer’s full history. When the answer is a bug, the task, the fix, and the update to the customer stay on the same conversation.</p>
          </div>
          <div className="record-bento">
            {RECORD_FACTS.map(({ title, description, image }) => (
              <article className="record-bento-card record-bento-wide" key={title}>
                <div className="record-bento-copy">
                  <h3>{title}</h3>
                  <p>{description}</p>
                </div>
                <CustomerRecordBento variant={image} />
              </article>
            ))}
          </div>
          <div className="built-in">
            <p className="built-in-title">Built in, for when support needs it</p>
            <div className="built-in-items">
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
          <SectionHead eyebrow="Docs that keep up" title="Your help center learns from support and from every merged PR."
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
            <Link prefetch={false} className="btn-link" href="/products/knowledge">Explore knowledge and docs →</Link>
          </div>
        </div>
      </section>

      {/* Agent controls */}
      <section id="control">
        <div className="wrap">
          <SectionHead eyebrow="Approvals built in" title="You decide what agents can do on their own."
            lede="Choose what runs automatically and what needs your approval. You can see each agent’s activity, check its results, and step in when a conversation needs you." />
          <div className="ctrl">
            <article>
              <AgentControlArt variant="tools" />
              <div className="ctrl-copy">
                <h3>Choose what your support agent can use.</h3>
                <p className="ctrl-description">Pick the docs, Helpin features, and connected tools it can use. Add <span className="home-feature">built-in skills</span> or <span className="home-feature">your own</span> to guide how it answers.</p>
              </div>
            </article>
            <article id="approval-demo">
              <AgentControlArt variant="approvals" />
              <div className="ctrl-copy">
                <h3>Review the replies that matter before they go out.</h3>
                <p className="ctrl-description">Set <span className="home-feature">approval rules</span> for each agent, so a teammate checks the replies and actions you care about before they happen.</p>
              </div>
            </article>
            <article>
              <AgentControlArt variant="triggers" />
              <div className="ctrl-copy">
                <h3>Run checks on a schedule.</h3>
                <p className="ctrl-description"><span className="home-feature">Schedule a weekly review</span> of your public help center, or start an agent when a <span className="home-feature">pull request merges</span>, so nobody has to start each job.</p>
              </div>
            </article>
          </div>
          <div className="home-section-actions">
            <Link prefetch={false} className="btn-link" href="/products/ai-agents">Explore agents and approvals →</Link>
          </div>
        </div>
      </section>

      {/* Importers: switching without losing history. */}
      <section id="switch" className="switch-section">
        <div className="wrap switch-grid">
          <div className="switch-copy">
            <SectionHead eyebrow="Switching" title="Move from Intercom without starting over." tight
              lede="Bring your help center and your past conversations, with their replies, notes, and tags. Your AI agents answer with that history from the first day." />
            <div className="switch-sources">
              <article><h3>Intercom</h3><p>Conversations and the Help Center, from the data region you use and from the date you choose. Articles keep their Intercom status or come in as drafts.</p></article>
              <article><h3>Other help centers</h3><p>Copy a public Intercom help center from its address, with no token. Import HelpScout Docs or a Nextra site with images and redirects.</p></article>
              <article><h3>The rest of your stack</h3><p>Contacts from HubSpot or a CSV file, and tasks, epics, and workflows from Shortcut.</p></article>
            </div>
            <Link prefetch={false} className="btn-link" href="/compare/intercom">See how Helpin compares with Intercom →</Link>
          </div>
          <ImportScene />
        </div>
      </section>

      {/* Open source, self-hosting, and building on Helpin. */}
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
            <article><SquareTerminal size={23} aria-hidden="true" /><h3>Install and manage it from your terminal.</h3><p>The Helpin CLI installs the self-hosted version, checks that its services are running, and shows the logs when something needs attention. <a className="inline-link" href={DOCS.selfHostingInstall}>Explore the CLI →</a></p></article>
            <article><Database size={23} aria-hidden="true" /><h3>Control your data and backups.</h3><p>Keep your customer records and files on the servers you manage, and choose how you back them up.</p></article>
            <article><SlidersHorizontal size={23} aria-hidden="true" /><h3>Choose your AI provider.</h3><p>When you self-host, you can connect a supported AI provider and choose the models your agents use.</p></article>
          </div>
          <div className="self-host-build">
            <p className="self-host-build-title">Build with Helpin</p>
            <div className="self-host-features self-host-build-grid">
              <article><Braces size={23} aria-hidden="true" /><h3>Add support chat with customer context.</h3><p>Use Helpin’s APIs and SDKs to add support chat and share customer details from your app, so customers don’t have to explain who they are each time. <a className="inline-link" href={DOCS.widgetSdk}>Explore the SDK →</a></p></article>
              <article><Plug size={23} aria-hidden="true" /><h3>Connect your AI tools through MCP.</h3><p>Claude Code, Codex, Cursor, and other compatible clients can read conversations, create tasks, and update docs with the permissions you grant. <a className="inline-link" href={DOCS.mcpServer}>Set up Helpin MCP →</a></p></article>
            </div>
          </div>
        </div>
      </section>

      {/* 13 Final CTA */}
      <section className="final-cta final-cta-connected" aria-labelledby="final-cta-title">
        <div className="wrap">
          <ConnectedWorkspace focus="support" />
          <div className="final">
            <span className="eyebrow">14-day free trial</span>
            <h2 id="final-cta-title">Spend no time on repeat questions, follow-ups, and updates.</h2>
            <p className="lede">Connect your docs, add support chat to your site, and let an agent take the repeat questions. AI usage is included in the trial, and you don’t need a card to start.</p>
            <CtaRow primaryLabel="Start free trial" />
            <div className="assure"><span>Open source</span><span>Self-host free, or let us run it</span><span>Built for SaaS teams</span></div>
          </div>
        </div>
      </section>

      <PreviewFooter homepage />
    </>
  );
}
