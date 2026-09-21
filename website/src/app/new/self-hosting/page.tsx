import { IncludedTable } from '../_components/IncludedTable';
import { CtaRow, CtaNote, ExampleLabel } from '../_components/ui';
import { previewMetadata } from '../_components/preview-metadata';
import { Database, GitBranch, KeyRound, Server, Settings2, ShieldCheck, Terminal } from 'lucide-react';
import { PreviewNav } from '../_components/PreviewNav';
import { PreviewFooter } from '../_components/PreviewFooter';

import { SectionHead } from '../_components/ui';
import { DeploymentExplorer, InfrastructureScene } from '../_components/platform/PlatformScenes';
import { DocLink, PlatformBreadcrumb, PlatformClosing, PlatformFAQ, REPO } from '../_components/platform/PlatformParts';
import { CommunityShowcase } from '../_components/platform/PlatformProof';
import '../_components/platform/platform.css';
import '../_components/platform/platform-polish.css';

export const metadata = previewMetadata("Self-hosting \u2014 Helpin", "/new/self-hosting");
const FAQS = [
  [
    "What is included in Helpin Community?",
    "Support, docs and agents. Projects, CRM, meetings, coding workflows and automation are outside the supported Community bundle.",
    "#whats-included",
    "See the comparison"
  ],
  [
    "What do I need to run it?",
    "Docker Engine, Docker Compose v2, Bash and OpenSSL. Start with 8 GiB of RAM and 20 GiB of free disk for evaluation.",
    "https://github.com/helpin-ai/helpin/blob/develop/community/README.md"
  ],
  [
    "How do I get the installer?",
    "Follow the current installation guide for the CLI or manual bundle, including release availability and setup requirements.",
    "https://github.com/helpin-ai/helpin/blob/develop/docs/community/cli.md"
  ],
  [
    "Can we use our own models?",
    "Yes. Add supported provider keys and choose a model per agent; requests go to that provider, so self-hosting does not make inference local.",
    "https://github.com/helpin-ai/helpin/blob/develop/docs/community/configuration.md"
  ],
  [
    "Does Helpin manage our backups or upgrades?",
    "No — your team backs up the databases, files, configuration and keys. Tested cross-version upgrades are planned for a later Community release.",
    "https://github.com/helpin-ai/helpin/blob/develop/docs/community/backups.md"
  ],
  [
    "Does the CLI configure public HTTPS for us?",
    "Server mode prepares public URLs and a Caddy configuration. Your team manages DNS, the proxy and certificates.",
    "https://github.com/helpin-ai/helpin/blob/develop/docs/community/deployment.md"
  ],
  [
    "What is the open-source license?",
    "The application is AGPL-3.0 and the SDK is Apache-2.0. You can inspect and modify the source under their license terms.",
    "https://github.com/helpin-ai/helpin/blob/develop/LICENSE"
  ]
] as const;
export default function SelfHostingPage(){return <><PreviewNav/><div className="platform-page self-hosting-page">
 <section className="platform-hero"><div className="wrap"><PlatformBreadcrumb label="Open source & self-hosting"/><div className="platform-hero-grid"><div className="platform-hero-copy"><span className="eyebrow">Open source · Community edition</span><h1>Run your customer workspace <span>on your infrastructure.</span></h1><p className="lede">Bring support, docs, and agents to the servers you control. Own the customer history, choose your model connections, and manage the installation with the Helpin CLI.</p><CtaRow /><CtaNote /><div className="platform-hero-points"><span><GitBranch size={14}/>AGPL-3.0</span><span><Server size={14}/>Docker Compose</span><span><Terminal size={14}/>Support, docs & agents</span></div></div><InfrastructureScene/></div></div></section>
 <nav className="platform-page-nav" aria-label="On this page"><div className="wrap"><strong>Self-hosting</strong><a href="#whats-included">What’s included</a><a href="#cli">Helpin CLI</a><a href="#deployment">Deployment</a><a href="#ownership">Data & connections</a><a href="#self-hosting-faq">Questions</a></div></nav>
 <IncludedTable />
 <section id="community"><div className="wrap"><div className="platform-centered"><SectionHead eyebrow="Start with the customer" title="What’s in Community: support, docs and agents." lede="Answer customers, publish the knowledge behind each reply, and put agents to work—all in the installation your team runs."/></div><ExampleLabel /><CommunityShowcase/></div></section>
 <section id="cli" className="platform-soft"><div className="wrap"><div className="platform-split platform-cli-layout"><figure className="platform-cli-art"><img src="/new/product/helpin-cli-tilted-1920-v1.webp" srcSet="/new/product/helpin-cli-tilted-1920-v1.webp 1920w, /new/product/helpin-cli-tilted-4k-v1.webp 3840w" sizes="(max-width: 960px) calc(100vw - 48px), 680px" width={3840} height={2160} loading="lazy" alt="Tilted Helpin CLI terminal showing installation checks, diagnostics, status, and logs."/></figure><div><SectionHead eyebrow="Helpin CLI" title="Run your instance from one terminal." lede="Install the bundle, check the services, and follow the logs. Use the CLI to handle the routine work of running Helpin."/><DocLink href={`${REPO}/docs/community/cli.md`}>Read the CLI guide</DocLink></div></div><div className="platform-command-grid">{[
  ['helpin install','Get the installation ready.','Choose local or server mode. Verify the bundle, generate secrets, and check application readiness.'],
  ['helpin doctor','See what needs attention.','Check Docker, configuration, services, API readiness, HTTPS, and secret-file permissions.'],
  ['helpin logs','Look into the services.','Follow logs for the installation or selected services. Use status to inspect containers and image identities.'],
 ].map(([command,title,body])=><article key={command}><code><span>$</span> {command}</code><h3>{title}</h3><p>{body}</p></article>)}</div></div></section>
 <section id="deployment" className="platform-dark"><div className="wrap"><div className="platform-centered"><SectionHead eyebrow="From your machine to your server" title="Start locally. Put it on your domains when you’re ready." lede="Use local defaults to explore Helpin, or configure a public server with your own dashboard, help-center, and attachment URLs."/></div><DeploymentExplorer/><DocLink href={`${REPO}/docs/community/deployment.md`}>Read the public deployment guide</DocLink></div></section>
 <section id="ownership"><div className="wrap"><div className="platform-section-intro"><SectionHead eyebrow="You operate the workspace" title="You own the data, the models and the backups." lede="Choose how the installation connects, protect the customer history, and keep the operating details in reach."/><span className="platform-side-note"><ShieldCheck size={23}/>Infrastructure and operating choices you control.</span></div><div className="platform-feature-grid">{[
  {Icon:Database,title:'Keep the history and files together.',body:'Operate the application databases and S3-compatible file storage. Back up the data, configuration, and encryption keys as one installation.',href:`${REPO}/docs/community/backups.md`,label:'Backup & restore guide'},
  {Icon:KeyRound,title:'Choose how AI connects.',body:'Set up supported provider connections and agent profiles. Configure embeddings and other server AI features separately for the capabilities you use.',href:`${REPO}/docs/community/configuration.md`,label:'Configuration reference'},
  {Icon:Settings2,title:'Configure the customer-facing services.',body:'Set public URLs, widget origins, and application mail. Enable optional integrations when your team needs them.',href:`${REPO}/docs/community/deployment.md`,label:'Deployment reference'},
 ].map(({Icon,title,body,href,label})=><article key={title}><Icon size={24}/><h3>{title}</h3><p>{body}</p><DocLink href={href}>{label}</DocLink></article>)}</div></div></section>
 <section id="self-hosting-faq"><div className="wrap platform-faq-grid"><SectionHead eyebrow="Questions" title="Know what you’re running."/><PlatformFAQ items={FAQS}/></div></section>
 <PlatformClosing id="self-hosting-final-title" title="Make the workspace yours." description="Start with the Community installation guide. Keep the customer workflow close to your team and your infrastructure."/>
 </div><PreviewFooter/></>;}
