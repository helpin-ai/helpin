import { HeroVortex } from '../_components/HeroVortex';
import { CtaRow } from '../_components/ui';
import { previewMetadata } from '../_components/preview-metadata';
import Link from 'next/link';
import { ArrowRight, BookOpen, Braces, KeyRound, MessagesSquare, Plug, Terminal, Webhook } from 'lucide-react';
import { PreviewNav } from '../_components/PreviewNav';
import { PreviewFooter } from '../_components/PreviewFooter';
import { SectionHead } from '../_components/ui';
import { DeveloperHeroScene, EventScene, MCPScene, SDKExplorer } from '../_components/platform/PlatformScenes';
import { DocLink, PlatformBreadcrumb, PlatformClosing, PlatformFAQ, REPO } from '../_components/platform/PlatformParts';
import { SupportIdentity } from '../products/customer-support/support-identity';
import '../_components/platform/platform.css';
import '../_components/platform/platform-polish.css';
import './developers-copy.css';

export const metadata = previewMetadata("Developers \u2014 Helpin", "/new/developers");
const FAQS = [
  [
    "Where should I start?",
    "Use an SDK to put support inside your product. Use MCP to connect AI clients and integrations to your workspace, or to give Helpin agents tools from other systems."
  ],
  [
    "Which SDK should I use?",
    "@helpin-ai/sdk-js for any web app, @helpin-ai/react for React, @helpin-ai/nextjs for Next.js, and @helpin-ai/vue for Vue 3 and Nuxt. Install the framework package alongside @helpin-ai/sdk-js."
  ],
  [
    "Can I use the SDK with self-hosted Helpin?",
    "Yes. Set host to your installation’s public widget URL and add your site to the permitted website origins."
  ],
  [
    "Is my widget key also an API credential?",
    "No. The widget key is public and only identifies your widget. Integrations reach your workspace through MCP, with OAuth or a scoped service token, and identity signing uses a separate server secret."
  ],
  [
    "How does Helpin verify a customer?",
    "Your backend signs an identity proof with HMAC-SHA256, and Helpin checks it when the widget identifies the customer. Turn on “Require server-signed identities” to reject unsigned identities; until then, they’re accepted as unverified. Verification doesn’t grant access to connected tools."
  ],
  [
    "What is the difference between the two MCP connections?",
    "Public MCP lets an external AI client or integration work with Helpin. External MCP lets a Helpin agent use tools from another system."
  ],
  [
    "Is MCP available in every workspace?",
    "Both connections are in controlled beta. A workspace manager turns on public MCP; external MCP servers must first be enabled for the deployment. What each connection can do depends on granted scopes, user permissions, and enabled modules."
  ],
  [
    "Which events can start agent work?",
    "GitHub and GitLab repository events, such as a pull request opened, merged, or closed, or a review requested, plus workspace events and schedules. A matching automation rule starts the selected agent with its tools and approvals. Outbound webhooks aren’t available yet."
  ]
] as const;
export default function DevelopersPage(){return <><PreviewNav/><div className="platform-page developers-page">
 <section className="platform-hero motion-hero"><HeroVortex variant="connections" tone="dark" /><div className="wrap"><PlatformBreadcrumb label="Developers"/><div className="platform-hero-grid"><div className="platform-hero-copy"><span className="eyebrow">SDKs, MCP, and events for AI agents</span><h1>Connect your product. <span>Give AI agents the tools to act.</span></h1><p className="lede">Bring support into your app, identify the customer behind each conversation, and give agents access to the tools behind the work.</p><CtaRow primaryLabel="Start free trial" /><p className="developer-supporting-note">Open source · Self-host free, or let us run it</p></div><div><DeveloperHeroScene/><p className="developer-demo-caption">Your app starts the conversation. Agents work with the tools you connect.</p></div></div></div></section>
 <nav className="platform-page-nav" aria-label="On this page"><div className="wrap"><strong>Developers</strong><a href="#sdk">SDKs</a><a href="#identity">Identity</a><a href="#mcp">MCP</a><a href="#webhooks">Events & automation</a><a href="#developer-resources">Resources</a></div></nav>

 <section id="developer-interfaces"><div className="wrap"><SectionHead eyebrow="Choose your starting point" title="Four ways to build on Helpin." /><div className="platform-resource-grid">{[
  {Icon:Terminal,label:'Helpin CLI',title:'Run your own workspace.',body:'Install Helpin, check the services, and inspect logs from the terminal.',href:'/new/self-hosting#cli',link:'Explore the CLI'},
  {Icon:Braces,label:'SDKs',title:'Bring Helpin into your product.',body:'Add support chat to your app and identify the customer behind each conversation.',href:'#sdk',link:'Explore the SDKs'},
  {Icon:Plug,label:'MCP connections',title:'Connect the tools around the work.',body:'Connect AI clients and integrations to Helpin, or give Helpin agents tools from another system.',href:'#mcp',link:'Explore MCP'},
  {Icon:Webhook,label:'Events & automation',title:'Start work when something changes.',body:'Start agent work from GitHub and GitLab events, workspace changes, or a schedule.',href:'#webhooks',link:'Explore events and automation'},
 ].map(({Icon,label,title,body,href,link})=><a key={title} href={href}><Icon size={22}/><span className="platform-micro">{label}</span><h3>{title}</h3><p>{body}</p><span className="developer-card-link">{link}<ArrowRight size={16}/></span></a>)}</div>
 <div id="developer-resources" className="developer-resource-links" role="navigation" aria-label="Developer resources">
  <a href={`${REPO}/docs/community/cli.md`} target="_blank" rel="noopener noreferrer">CLI setup guide<ArrowRight size={15}/></a>
  <a href={`${REPO}/CONTRIBUTING.md`} target="_blank" rel="noopener noreferrer">Contributing<ArrowRight size={15}/></a>
  <Link href="/new/self-hosting">Explore self-hosting<ArrowRight size={15}/></Link>
 </div></div></section>
 <section id="sdk"><div className="wrap"><div className="platform-centered"><SectionHead eyebrow="Support inside your product" title="Put help where customers need it." lede="Start a conversation from your own interface. Associate it with the signed-in customer and their company, then keep the exchange in Helpin."/></div><SDKExplorer/><div className="platform-sdk-links"><a href="https://www.npmjs.com/package/@helpin-ai/sdk-js" target="_blank" rel="noopener noreferrer">JavaScript<ArrowRight size={13}/></a><a href="https://www.npmjs.com/package/@helpin-ai/react" target="_blank" rel="noopener noreferrer">React<ArrowRight size={13}/></a><a href="https://www.npmjs.com/package/@helpin-ai/nextjs" target="_blank" rel="noopener noreferrer">Next.js<ArrowRight size={13}/></a><a href="https://www.npmjs.com/package/@helpin-ai/vue" target="_blank" rel="noopener noreferrer">Vue<ArrowRight size={13}/></a></div><div className="platform-feature-grid platform-sdk-features">{[
  {Icon:MessagesSquare,title:'Start from your own interface.',body:'Use a custom launcher, prepare an opening message, or reopen an existing conversation.'},
  {Icon:KeyRound,title:'Connect the person and company.',body:'Identify signed-in customers so the conversation has a clear account context.'},
  {Icon:BookOpen,title:'Open the relevant guide.',body:'Bring an article into the widget from your product’s help menu or interface.'},
 ].map(({Icon,title,body})=><article key={title}><Icon size={23}/><h3>{title}</h3><p>{body}</p></article>)}</div></div></section>
 <section id="identity" className="platform-soft"><div className="wrap platform-split"><div><SectionHead eyebrow="Know who’s asking" title="Identify the customer. Control what agents can access." lede="Verify the person behind the conversation, then let agents investigate through selected tools. Customer identity and permission to access data are separate controls." />
 <div className="platform-faqs"><details><summary>How does identity verification work?</summary><p>Your backend signs an identity proof with HMAC-SHA256. Helpin validates the proof when the widget identifies the customer. Keep the signing secret on your server, and turn on “Require server-signed identities” to reject unsigned ones. <a href={`${REPO}/docs/community/widget-identity.md`}>Set up customer identity →</a></p></details>
 <details><summary>Does verification unlock access to customer data?</summary><p>No. You choose the agent’s tools and approval rules separately. Your connected tool server must also enforce which customer’s data it can return. <a href={`${REPO}/docs/external-mcp-servers.md`}>Configure tool access →</a></p></details></div>
 </div><div><h3 className="developer-demo-heading">An investigation with defined access.</h3><SupportIdentity variant="developer" /></div></div></section>
 <section id="mcp" className="platform-dark section-motion"><HeroVortex variant="converge" tone="dark" /><div className="wrap"><div className="platform-centered"><SectionHead eyebrow="Connect your AI tools · Controlled beta" title="Bring Helpin to your AI tools. Bring your tools to Helpin." lede="AI clients and integrations work with Helpin through OAuth or a scoped service token. Helpin agents can use tools from other MCP servers."/></div><div className="platform-mcp-grid"><article><div className="platform-mcp-copy"><h3>Use Helpin from your AI client.</h3><p>Let an authorized AI client or integration search your docs and use the workspace capabilities you grant. A workspace manager turns it on.</p></div><MCPScene direction="inbound"/><DocLink href={`${REPO}/docs/public-mcp-server.md`}>Set up public MCP</DocLink></article><article><div className="platform-mcp-copy"><h3>Give agents a way to check the facts.</h3><p>Connect an external MCP server and select the tools each agent can use, with approvals where required. The deployment must enable external servers first.</p></div><MCPScene direction="outbound"/><DocLink href={`${REPO}/docs/external-mcp-servers.md`}>Connect an external tool server</DocLink></article></div></div></section>
 <section id="webhooks"><div className="wrap platform-split"><div><SectionHead eyebrow="From an event to the next step" title="Start the right work when something changes." lede="Connect GitHub and GitLab events, workspace changes, and schedules to agent workflows. A matching rule starts the job with the agent’s selected tools, instructions, and approvals." secondaryLede="Inspect trigger activity, including skipped events, separately from agent runs."/><Link className="platform-text-link" href="/new/products/ai-agents#agent-controls">Explore agents and automation<ArrowRight size={15}/></Link></div><EventScene/></div></section>
 <section id="developer-faq"><div className="wrap platform-faq-grid"><SectionHead eyebrow="Before you connect" title="Choose the right interface."/><PlatformFAQ items={FAQS}/></div></section>
 <PlatformClosing id="developers-final-title" eyebrow="Start building" title="Put support in your product. Give your agents the right tools." description="Start a free Cloud trial, or self-host the open-source edition and build against your own installation." primaryLabel="Start free trial" supportingLine="Open source · Self-host free, or let us run it"/>
 </div><PreviewFooter homepage/></>;}
