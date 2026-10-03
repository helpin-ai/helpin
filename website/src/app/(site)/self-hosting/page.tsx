import { HeroVortex } from '../_components/HeroVortex';
import Link from 'next/link';
import { IncludedTable } from '../_components/IncludedTable';
import { CtaRow } from '../_components/ui';
import { createPageMetadata, PAGE_SEO } from '@/lib/metadata';
import { Database, GitBranch, KeyRound, Server, Settings2, Users } from 'lucide-react';
import { PreviewNav } from '../_components/PreviewNav';
import { PreviewFooter } from '../_components/PreviewFooter';

import { SectionHead } from '../_components/ui';
import { DeploymentExplorer, InfrastructureScene } from '../_components/platform/PlatformScenes';
import { DocLink, PlatformClosing, PlatformFAQ } from '../_components/platform/PlatformParts';
import { CommunityShowcase } from '../_components/platform/PlatformProof';
import '../_components/platform/platform.css';
import '../_components/platform/platform-polish.css';
import './self-hosting-copy.css';
import { DOCS } from '../_components/docsLinks';

export const metadata = createPageMetadata(PAGE_SEO.selfHosting);
const INSTALL_GUIDE = DOCS.selfHosting;
const FAQS = [
  [
    "Which parts of Helpin can we run ourselves?",
    "All of it: support, docs and help center, AI agents including coding agents, projects, CRM with meetings, and automation. Only Cloud billing and hosted AI are left out."
  ],
  [
    "What does a local evaluation require?",
    "Linux or macOS on amd64 or arm64, with Docker Engine, Compose v2, Bash, OpenSSL, curl, and sha256sum or shasum. Allow 8 GiB RAM and 20 GiB free disk for evaluation; size production for your workload. Native Windows is not supported."
  ],
  [
    "Where should we start the installation?",
    "Run curl -fsSL https://helpin.ai/install.sh | bash, then helpin install. A manual bundle installation is also documented."
  ],
  [
    "Can we choose our AI providers?",
    "Yes. Self-hosted Helpin always uses your own providers. Connect Anthropic, OpenAI, or OpenRouter with your keys and choose the model for each agent. Knowledge search uses a separate OpenAI-compatible embeddings setting."
  ],
  [
    "Who handles maintenance and upgrades?",
    "Your team. The CLI includes backup, restore, and upgrade. Upgrades run only between releases that declare a tested path."
  ],
  [
    "Who helps us if something breaks?",
    "Community support is on GitHub and Discord. Enterprise adds deployment help and support terms."
  ],
  [
    "Can we move from Zendesk or Intercom?",
    "Import from Zendesk and Intercom is coming soon.",
    "/compare",
    "See how Helpin compares"
  ],
  [
    "Does server setup manage HTTPS for us?",
    "No. It prepares a Caddy configuration; your team manages DNS, the proxy, and certificates."
  ],
  [
    "How is Helpin licensed?",
    "Application: AGPL-3.0-only. SDKs and widget: Apache-2.0. If your company can’t use AGPL, Enterprise offers a commercial license."
  ],
  [
    "Are Kubernetes and fully offline deployments supported?",
    "Docker Compose is the documented route. Kubernetes and fully air-gapped installations are not yet validated."
  ]
] as const;
export default function SelfHostingPage(){return <><PreviewNav tone="dark"/><div className="platform-page self-hosting-page">
 <section className="platform-hero motion-hero"><HeroVortex variant="connections" tone="dark" /><div className="wrap"><div className="platform-hero-grid"><div className="platform-hero-copy"><span className="eyebrow">Open source · Community 0.2 beta</span><h1>Your team. Your agents.<br /><span>Your own servers.</span></h1><p className="lede">Run support, docs, projects, CRM, meetings, and AI agents on infrastructure you control. Helpin Community is open source under AGPL-3.0, with no license fee or plan limits.</p><CtaRow primaryLabel="Self-host Helpin" primaryHref={INSTALL_GUIDE}/><p className="self-hosting-supporting-note">Looking for managed hosting? <Link prefetch={false} href="/pricing">Explore Helpin Cloud →</Link></p><div className="platform-hero-points"><span><GitBranch size={14}/>Open source</span><span><Server size={14}/>Docker Compose</span><span><Users size={14}/>Unlimited users</span></div></div><div><InfrastructureScene/><p className="self-hosting-caption">No telemetry by default. Nothing leaves your servers unless you connect it.</p></div></div></div></section>

 <IncludedTable />
 <section id="community"><div className="wrap"><div className="platform-centered"><SectionHead eyebrow="The work stays connected" title="The same agent workflows. Run by your team." lede="Use the Echo agent for support, the Quill agent for docs, and planning and coding agents for product work. Connect your own AI providers and choose the tools each agent can use."/></div><CommunityShowcase/></div></section>
 <section id="cli" className="platform-soft"><div className="wrap"><div className="platform-split platform-cli-layout"><figure className="platform-cli-art"><h3 className="self-hosting-art-heading">One CLI to install and operate.</h3><div className="self-hosting-terminal-labels"><span>Install</span><span>Check</span><span>Inspect</span></div><img src="/new/product/helpin-cli-tilted-1920-v1.webp" srcSet="/new/product/helpin-cli-tilted-1920-v1.webp 1920w, /new/product/helpin-cli-tilted-4k-v1.webp 3840w" sizes="(max-width: 960px) calc(100vw - 48px), 680px" width={3840} height={2160} loading="lazy" alt="Tilted Helpin CLI terminal showing installation checks, diagnostics, status, and logs."/></figure><div><SectionHead eyebrow="From the terminal" title="Set it up. See how it’s running." lede="Install Helpin, check its health, read service logs, and run backups, restores, and upgrades from one command-line tool."/><DocLink href={DOCS.selfHostingInstall}>Read the CLI guide</DocLink></div></div><div className="platform-command-grid">{[
  ['helpin install','Get Helpin running.','Follow the installation checks for your machine or server.'],
  ['helpin doctor','Check before you troubleshoot.','Inspect the installation and identify what needs attention.'],
  ['helpin logs','Look into the service.','Read the logs behind an issue instead of guessing what happened.'],
 ].map(([command,title,body])=><article key={command}><code><span>$</span> {command}</code><h3>{title}</h3><p>{body}</p></article>)}</div></div></section>
 <section id="deployment" className="platform-dark section-motion"><HeroVortex variant="converge" tone="dark" /><div className="wrap"><div className="platform-centered"><SectionHead eyebrow="From evaluation to everyday use" title="Try it locally. Move it to your server." lede="Evaluate on your laptop, then move to a server with your own domains. The CLI generates the Caddy configuration for the public setup."/></div><DeploymentExplorer/><DocLink href={DOCS.selfHostingDeploy}>Read the deployment guide</DocLink></div></section>
 <section id="ownership"><div className="wrap"><div className="platform-section-intro"><SectionHead eyebrow="Your team runs Helpin" title={<>Keep the history.<br/>Choose the connections.</>} lede="Manage your records, files, and backups. Choose the services your agents connect to and check where those providers process information."/></div><div className="platform-feature-grid">{[
  {Icon:Database,title:'Keep a backup you can restore.',body:'Back up records, files, configuration, and encryption keys. Test a restore before you depend on it.',href:DOCS.selfHostingBackups,label:'Read the backup and restore guide'},
  {Icon:KeyRound,title:'Choose the models behind the agents.',body:'Connect Anthropic, OpenAI, or OpenRouter with your own keys and select models for the work. Set each agent’s tools and approval rules separately.',href:DOCS.selfHostingAI,label:'Configure AI connections'},
  {Icon:Settings2,title:'Connect the experience your customers use.',body:'Configure your widget, published help center, and connected tools for the environment you run. Check access before enabling a new workflow.',href:DOCS.selfHostingConfigure,label:'Configure your deployment'},
 ].map(({Icon,title,body,href,label})=><article key={title}><Icon size={24}/><h3>{title}</h3><p>{body}</p><DocLink href={href}>{label}</DocLink></article>)}</div></div></section>
 <section id="self-hosting-faq"><div className="wrap platform-faq-grid"><SectionHead eyebrow="Questions, answered" title="FAQs about Helpin’s self-hosting"/><div><PlatformFAQ items={FAQS}/><DocLink href={INSTALL_GUIDE}>Read the self-hosting documentation</DocLink></div></div></section>
 <PlatformClosing id="self-hosting-final-title" eyebrow="Run Helpin on your terms" title="Run your agents on your own Helpin instance." description="Self-host the open-source product and connect your AI providers. Prefer a managed service? Helpin Cloud includes AI usage." primaryLabel="Self-host Helpin" primaryHref={INSTALL_GUIDE} supportingLine={<>Prefer managed hosting? <Link prefetch={false} href="/pricing">Explore Helpin Cloud →</Link></>}/>
 </div><PreviewFooter homepage/></>;}
