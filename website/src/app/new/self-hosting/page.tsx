import { HeroVortex } from '../_components/HeroVortex';
import Link from 'next/link';
import { IncludedTable } from '../_components/IncludedTable';
import { CtaRow } from '../_components/ui';
import { previewMetadata } from '../_components/preview-metadata';
import { Database, GitBranch, KeyRound, Server, Settings2, Terminal } from 'lucide-react';
import { PreviewNav } from '../_components/PreviewNav';
import { PreviewFooter } from '../_components/PreviewFooter';

import { SectionHead } from '../_components/ui';
import { DeploymentExplorer, InfrastructureScene } from '../_components/platform/PlatformScenes';
import { DocLink, PlatformBreadcrumb, PlatformClosing, PlatformFAQ, REPO } from '../_components/platform/PlatformParts';
import { CommunityShowcase } from '../_components/platform/PlatformProof';
import '../_components/platform/platform.css';
import '../_components/platform/platform-polish.css';
import './self-hosting-copy.css';

export const metadata = previewMetadata("Self-hosting \u2014 Helpin", "/new/self-hosting");
const INSTALL_GUIDE = `${REPO}/community/README.md`;
const FAQS = [
  [
    "Which parts of Helpin can we run ourselves?",
    "The product modules, AI agents, and automation are available for self-hosting. Enterprise capabilities are separately licensed."
  ],
  [
    "What does a local evaluation require?",
    "Docker Engine, Compose v2, Bash, and OpenSSL. Allow 8 GiB RAM and 20 GiB free disk for evaluation; size production for your workload. The CLI bootstrap also requires curl and sha256sum or shasum."
  ],
  [
    "Where should we start the installation?",
    "Follow the release-specific guide for the CLI or manual bundle."
  ],
  [
    "Can we choose our AI providers?",
    "Yes. Use supported provider connections and select agent models. Review provider terms, usage charges, and data handling before connecting customer workflows."
  ],
  [
    "Who handles maintenance and upgrades?",
    "Your team. Validated cross-version upgrade procedures are not yet available."
  ],
  [
    "Does server setup manage HTTPS for us?",
    "No. It prepares a Caddy configuration; your team manages DNS, the proxy, and certificates."
  ],
  [
    "How is Helpin licensed?",
    "Application: AGPL-3.0. SDK: Apache-2.0. Enterprise code has a separate license."
  ],
  [
    "Are Kubernetes and fully offline deployments supported?",
    "Docker Compose is the documented route. Kubernetes and fully air-gapped installations are not yet validated."
  ]
] as const;
export default function SelfHostingPage(){return <><PreviewNav/><div className="platform-page self-hosting-page">
 <section className="platform-hero motion-hero"><HeroVortex variant="connections" tone="dark" /><div className="wrap"><PlatformBreadcrumb label="Open source & self-hosting"/><div className="platform-hero-grid"><div className="platform-hero-copy"><span className="eyebrow">Open source. Self-hosted.</span><h1>The whole product. <span>Your infrastructure.</span></h1><p className="lede">Run support, projects, CRM, meetings, docs, and AI agents in one connected platform. Keep the customer’s history together while your team controls the deployment and the services it connects to.</p><CtaRow primaryLabel="Self-host Helpin" primaryHref={INSTALL_GUIDE}/><p className="self-hosting-supporting-note">Looking for managed hosting? <Link href="/pricing">Explore Helpin Cloud →</Link></p><div className="platform-hero-points"><span><GitBranch size={14}/>Open source</span><span><Server size={14}/>Docker Compose</span><span><Terminal size={14}/>Product modules included</span></div></div><div><InfrastructureScene/><p className="self-hosting-caption">Know what runs in your environment—and what connects outside it.</p></div></div></div></section>
 <nav className="platform-page-nav" aria-label="On this page"><div className="wrap"><strong>Self-hosting</strong><a href="#whats-included">What’s included</a><a href="#cli">Helpin CLI</a><a href="#deployment">Deployment</a><a href="#ownership">Data & connections</a><a href="#self-hosting-faq">Questions</a></div></nav>
 <IncludedTable />
 <section id="community"><div className="wrap"><div className="platform-centered"><SectionHead eyebrow="The work stays connected" title="More than an inbox. Built for the whole team." lede="Answer a question, plan the resulting work, and keep the customer informed. Bring the people doing the work and the agents helping them into the same customer history."/></div><CommunityShowcase/><p className="self-hosting-caption">Different teams. Connected work. An installation you operate.</p></div></section>
 <section id="cli" className="platform-soft"><div className="wrap"><div className="platform-split platform-cli-layout"><figure className="platform-cli-art"><h3 className="self-hosting-art-heading">Your installation, within reach.</h3><div className="self-hosting-terminal-labels"><span>Install</span><span>Check</span><span>Inspect</span></div><img src="/new/product/helpin-cli-tilted-1920-v1.webp" srcSet="/new/product/helpin-cli-tilted-1920-v1.webp 1920w, /new/product/helpin-cli-tilted-4k-v1.webp 3840w" sizes="(max-width: 960px) calc(100vw - 48px), 680px" width={3840} height={2160} loading="lazy" alt="Tilted Helpin CLI terminal showing installation checks, diagnostics, status, and logs."/><figcaption className="self-hosting-caption">Start with the status. Follow the evidence.</figcaption></figure><div><SectionHead eyebrow="From the terminal" title="Set it up. See how it’s running." lede="Use the Helpin CLI to install Helpin, inspect its status, and investigate services when something needs attention."/><DocLink href={`${REPO}/docs/community/cli.md`}>Read the CLI guide</DocLink><p className="self-hosting-supporting-note">Use the installation instructions for the release you intend to run.</p></div></div><div className="platform-command-grid">{[
  ['helpin install','Get Helpin running.','Follow the installation checks for your machine or server.'],
  ['helpin doctor','Check before you troubleshoot.','Inspect the installation and identify what needs attention.'],
  ['helpin logs','Look into the service.','Read the logs behind an issue instead of guessing what happened.'],
 ].map(([command,title,body])=><article key={command}><code><span>$</span> {command}</code><h3>{title}</h3><p>{body}</p></article>)}</div></div></section>
 <section id="deployment" className="platform-dark section-motion"><HeroVortex variant="converge" tone="dark" /><div className="wrap"><div className="platform-centered"><SectionHead eyebrow="From evaluation to everyday use" title="Try the workflow locally. Plan the rollout from there." lede="Start with a local installation you can explore. Before bringing in your team and customers, review the connections it needs and who will operate it."/></div><DeploymentExplorer/><DocLink href={`${REPO}/docs/community/deployment.md`}>Read the deployment guide</DocLink></div></section>
 <section id="ownership"><div className="wrap"><div className="platform-section-intro"><SectionHead eyebrow="Your team runs Helpin" title={<>Keep the history.<br/>Choose the connections.</>} lede="Manage customer records and files on infrastructure you operate. Decide which services Helpin uses—and understand where those services process information."/></div><div className="platform-feature-grid">{[
  {Icon:Database,title:'Make the history recoverable.',body:'Back up records, files, configuration, and encryption keys. Test a restore before you depend on it.',href:`${REPO}/docs/community/backups.md`,label:'Read the backup and restore guide'},
  {Icon:KeyRound,title:'Choose the models behind the agents.',body:'Connect supported providers and select models for the work. Set each agent’s tools and approval rules separately.',href:`${REPO}/docs/community/configuration.md`,label:'Configure AI connections'},
  {Icon:Settings2,title:'Connect the experience your customers use.',body:'Configure your widget, published help center, and connected tools for the environment you run. Check access before enabling a new workflow.',href:`${REPO}/docs/community/deployment.md`,label:'Configure your deployment'},
 ].map(({Icon,title,body,href,label})=><article key={title}><Icon size={24}/><h3>{title}</h3><p>{body}</p><DocLink href={href}>{label}</DocLink></article>)}</div></div></section>
 <section id="self-hosting-faq"><div className="wrap platform-faq-grid"><SectionHead eyebrow="Before you deploy" title="Know what you’ll run. Know what you’ll maintain."/><div><PlatformFAQ items={FAQS}/><DocLink href={INSTALL_GUIDE}>Read the self-hosting documentation</DocLink></div></div></section>
 <PlatformClosing id="self-hosting-final-title" eyebrow="Run Helpin on your terms" title="Bring the customer history together. Run it on your terms." description="Give your team and AI agents a shared place to understand the request, do the work, and follow through—on infrastructure your team operates." primaryLabel="Self-host Helpin" primaryHref={INSTALL_GUIDE} supportingLine={<>Prefer managed hosting? <Link href="/pricing">Explore Helpin Cloud →</Link></>}/>
 </div><PreviewFooter homepage/></>;}
