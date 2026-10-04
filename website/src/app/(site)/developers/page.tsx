import { HeroVortex } from '../_components/HeroVortex';
import { CtaRow } from '../_components/ui';
import { createPageMetadata, PAGE_SEO } from '@/lib/metadata';
import Link from 'next/link';
import { ArrowRight, BookOpen, Braces, KeyRound, MessagesSquare, Plug, Terminal, Webhook } from 'lucide-react';
import { PreviewNav } from '../_components/PreviewNav';
import { PreviewFooter } from '../_components/PreviewFooter';
import { SectionHead } from '../_components/ui';
import { DeveloperHeroScene, EventScene, MCPScene, SDKExplorer } from '../_components/platform/PlatformScenes';
import { DocLink, PlatformClosing, PlatformFAQ, REPO } from '../_components/platform/PlatformParts';
import { SupportIdentity } from '../products/customer-support/support-identity';
import '../_components/platform/platform.css';
import '../_components/platform/platform-polish.css';
import './developers-copy.css';
import { DOCS } from '../_components/docsLinks';

export const metadata = createPageMetadata(PAGE_SEO.developers);
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
export default function DevelopersPage(){return <><PreviewNav tone="dark"/><div className="platform-page developers-page">
 <section className="platform-hero motion-hero"><HeroVortex variant="connections" tone="dark" /><div className="wrap"><div className="platform-hero-grid"><div className="platform-hero-copy"><span className="eyebrow">SDKs, MCP, and events for AI agents</span><h1>Give your agents <span>the facts behind the answer.</span></h1><p className="lede">Add support to your app, verify who is asking, and connect the tools an agent needs. The Echo agent can check permitted account facts, while coding agents work from real reports.</p><CtaRow primaryLabel="Start free trial" /><p className="developer-supporting-note">Open source · Self-host free, or let us run it</p></div><div><DeveloperHeroScene/><p className="developer-demo-caption">Your app starts the conversation. Agents work with the tools you connect.</p></div></div></div></section>


 <section id="developer-interfaces"><div className="wrap"><SectionHead eyebrow="Choose your starting point" title="Connect the part you need." /><div className="platform-resource-grid">{[
  {Icon:Terminal,label:'Helpin CLI',title:'Run your own workspace.',body:'Install Helpin, check the services, and inspect logs from the terminal.',href:'/self-hosting#cli',link:'Explore the CLI'},
  {Icon:Braces,label:'SDKs',title:'Bring Helpin into your product.',body:'Add support chat to your app and identify the customer behind each conversation.',href:'#sdk',link:'Explore the SDKs'},
  {Icon:Plug,label:'MCP connections',title:'Give agents useful tools.',body:'Connect AI clients and integrations to Helpin, or give Helpin agents tools from another system.',href:'#mcp',link:'Explore MCP'},
  {Icon:Webhook,label:'Events & automation',title:'Start work when something changes.',body:'Start agent work from GitHub and GitLab events, workspace changes, or a schedule.',href:'#webhooks',link:'Explore events and automation'},
 ].map(({Icon,label,title,body,href,link})=><a key={title} href={href}><Icon size={22}/><span className="platform-micro">{label}</span><h3>{title}</h3><p>{body}</p><span className="developer-card-link">{link}<ArrowRight size={16}/></span></a>)}</div>
 <div id="developer-resources" className="developer-resource-links" role="navigation" aria-label="Developer resources">
  <a href={DOCS.selfHostingInstall}>CLI setup guide<ArrowRight size={15}/></a>
  <a href={`${REPO}/CONTRIBUTING.md`} target="_blank" rel="noopener noreferrer">Contributing<ArrowRight size={15}/></a>
  <Link prefetch={false} href="/self-hosting">Explore self-hosting<ArrowRight size={15}/></Link>
 </div></div></section>
 <section id="sdk"><div className="wrap"><div className="platform-centered"><SectionHead eyebrow="Support inside your product" title="Put the Echo agent inside your product." lede="Open chat from your own interface and link it to the signed-in customer and company. The Echo agent and your team can pick up the conversation in Helpin."/></div><SDKExplorer/><div className="platform-sdk-links"><a href="https://www.npmjs.com/package/@helpin-ai/sdk-js" target="_blank" rel="noopener noreferrer">JavaScript<ArrowRight size={13}/></a><a href="https://www.npmjs.com/package/@helpin-ai/react" target="_blank" rel="noopener noreferrer">React<ArrowRight size={13}/></a><a href="https://www.npmjs.com/package/@helpin-ai/nextjs" target="_blank" rel="noopener noreferrer">Next.js<ArrowRight size={13}/></a><a href="https://www.npmjs.com/package/@helpin-ai/vue" target="_blank" rel="noopener noreferrer">Vue<ArrowRight size={13}/></a></div><div className="platform-feature-grid platform-sdk-features">{[
  {Icon:MessagesSquare,title:'Start from your own interface.',body:'Use a custom launcher, prepare an opening message, or reopen an existing conversation.'},
  {Icon:KeyRound,title:'Connect the person and company.',body:'Identify signed-in customers so the conversation has a clear account context.'},
  {Icon:BookOpen,title:'Open the relevant guide.',body:'Bring an article into the widget from your product’s help menu or interface.'},
 ].map(({Icon,title,body})=><article key={title}><Icon size={23}/><h3>{title}</h3><p>{body}</p></article>)}</div></div></section>
 <section id="identity" className="platform-soft"><div className="wrap platform-split"><div><SectionHead eyebrow="Know who’s asking" title="Verify the customer. Limit the lookup." lede="Verify the person behind the conversation and choose which tools agents can use to investigate. Customer identity and permission to access data are separate controls." />
 <div className="platform-faqs"><details><summary>How does identity verification work?</summary><p>Your backend signs an identity proof with HMAC-SHA256. Helpin validates the proof when the widget identifies the customer. Keep the signing secret on your server, and turn on “Require server-signed identities” to reject unsigned ones. <a href={DOCS.widgetSdk}>Set up customer identity →</a></p></details>
 <details><summary>Does verification unlock access to customer data?</summary><p>No. You choose the agent’s tools and approval rules separately. Your connected tool server must also enforce which customer’s data it can return. <a href={DOCS.externalMcp}>Configure tool access →</a></p></details></div>
 </div><div><SupportIdentity variant="developer" /></div></div></section>
 <section id="mcp" className="platform-dark section-motion"><HeroVortex variant="converge" tone="dark" /><div className="wrap"><div className="platform-centered"><SectionHead eyebrow="Connect your AI tools · Controlled beta" title="Connect your AI tools in either direction." lede="AI clients and integrations work with Helpin through OAuth or a scoped service token. Helpin agents can use tools from other MCP servers."/></div><div className="platform-mcp-grid"><article><div className="platform-mcp-copy"><h3>Use Helpin from your AI client.</h3><p>Let an authorized AI client or integration search your docs and use the workspace capabilities you grant. A workspace manager turns it on.</p></div><MCPScene direction="inbound"/><DocLink href={DOCS.mcpServer}>Set up public MCP</DocLink></article><article><div className="platform-mcp-copy"><h3>Give agents a way to check the facts.</h3><p>Connect an external MCP server and select the tools each agent can use, with approvals where required. The deployment must enable external servers first.</p></div><MCPScene direction="outbound"/><DocLink href={DOCS.externalMcp}>Connect an external tool server</DocLink></article></div></div></section>
 <section id="webhooks"><div className="wrap platform-split"><div><SectionHead eyebrow="From an event to the next step" title="Start an agent when work changes." lede="A pull request opens. A task changes stage. A scheduled check is due. Use a matching automation rule to start the agent with the tools and approvals you selected." secondaryLede="Inspect trigger activity, including skipped events, separately from agent runs."/><Link prefetch={false} className="platform-text-link" href="/products/ai-agents#agent-controls">Explore agents and automation<ArrowRight size={15}/></Link></div><EventScene/></div></section>
 <section id="developer-faq"><div className="wrap platform-faq-grid"><SectionHead eyebrow="Questions, answered" title="FAQs about Helpin’s developer tools"/><PlatformFAQ items={FAQS}/></div></section>
 <PlatformClosing id="developers-final-title" eyebrow="Start building" title="Connect one useful tool. Give an agent a real job." description="Start a free Cloud trial, or self-host the open-source edition and build against your own installation." primaryLabel="Start free trial" supportingLine="Open source · Self-host free, or let us run it"/>
 </div><PreviewFooter homepage/></>;}
