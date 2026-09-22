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
import { WorkspaceAPIExample } from '../_components/platform/PlatformProof';
import { SupportIdentity } from '../products/customer-support/support-identity';
import '../_components/platform/platform.css';
import '../_components/platform/platform-polish.css';
import './developers-copy.css';

export const metadata = previewMetadata("Developers \u2014 Helpin", "/new/developers");
const FAQS = [
  [
    "Where should I start?",
    "Use an SDK for in-product support. Choose MCP for AI-tool connections. For an application integration, inspect the authenticated workspace API and the operations it supports."
  ],
  [
    "Can I use the SDK with self-hosted Helpin?",
    "Yes. Configure it for your installation and check the permitted website origins."
  ],
  [
    "What is the difference between the two MCP connections?",
    "Public MCP lets an external AI client work with Helpin. External MCP lets a Helpin agent use tools from another system."
  ],
  [
    "Can a webhook start agent work?",
    "Yes, when the relevant integration and automation are configured. Matching events start the selected workflow; agent actions follow their configured permissions and approvals."
  ],
  [
    "Which SDK should I use?",
    "Choose the package for your application: JavaScript, React, Next.js, or Vue. Follow that package’s setup instructions and check its published version."
  ],
  [
    "Is my widget key also an API credential?",
    "No. Widget identification, workspace authentication, and MCP access are separate."
  ],
  [
    "How does Helpin verify a customer?",
    "Your backend signs the identity proof. Helpin checks it when the widget identifies the customer. Verification does not grant access to every connected tool."
  ],
  [
    "Is public MCP available in every workspace?",
    "No. It is a controlled beta requiring enablement. Available capabilities also depend on granted access, user permissions, and enabled modules."
  ]
] as const;
export default function DevelopersPage(){return <><PreviewNav/><div className="platform-page developers-page">
 <section className="platform-hero motion-hero"><HeroVortex variant="connections" tone="dark" /><div className="wrap"><PlatformBreadcrumb label="Developers"/><div className="platform-hero-grid"><div className="platform-hero-copy"><span className="eyebrow">SDKs, APIs, and tools for AI agents</span><h1>Connect your product. <span>Give AI agents the tools to act.</span></h1><p className="lede">Bring support into your app, connect your customer data, and give agents access to the tools behind the work. Build on Helpin without leaving the customer’s history behind.</p><CtaRow primaryLabel="Start free trial" /><p className="developer-supporting-note">Open source · Self-host or use Helpin Cloud</p></div><div><DeveloperHeroScene/><p className="developer-demo-caption">Start in your product. Follow through with the history attached.</p></div></div></div></section>
 <nav className="platform-page-nav" aria-label="On this page"><div className="wrap"><strong>Developers</strong><a href="#sdk">SDKs</a><a href="#mcp">MCP</a><a href="#api">Workspace API</a><a href="#webhooks">Events & workflows</a><a href="#developer-resources">Resources</a></div></nav>

 <section id="developer-interfaces"><div className="wrap"><SectionHead eyebrow="Choose your starting point" title="Connect what you need. Build from there." /><div className="platform-resource-grid">{[
  {Icon:Terminal,label:'Helpin CLI',title:'Run your own workspace.',body:'Install Helpin, check the services, and investigate your deployment from the terminal.',href:'/new/self-hosting#cli',link:'Explore the CLI'},
  {Icon:Braces,label:'APIs & SDKs',title:'Bring Helpin into your product.',body:'Add support chat and connect the customer behind the conversation.',href:'#sdk',link:'Explore the SDKs'},
  {Icon:Plug,label:'MCP connections',title:'Connect the tools around the work.',body:'Use Helpin from an AI client, or give Helpin agents tools from another system.',href:'#mcp',link:'Explore MCP'},
  {Icon:Webhook,label:'Events & workflows',title:'Start work when something changes.',body:'Connect supported events to the agent workflows your team has configured.',href:'#webhooks',link:'Explore events and automation'},
 ].map(({Icon,label,title,body,href,link})=><a key={title} href={href}><Icon size={22}/><span className="platform-micro">{label}</span><h3>{title}</h3><p>{body}</p><span className="developer-card-link">{link}<ArrowRight size={16}/></span></a>)}</div>
 <div id="developer-resources" className="developer-resource-links" role="navigation" aria-label="Developer resources">
  <a href={`${REPO}/docs/community/cli.md`} target="_blank" rel="noopener noreferrer">CLI setup guide<ArrowRight size={15}/></a>
  <a href={`${REPO}/CONTRIBUTING.md`} target="_blank" rel="noopener noreferrer">Contributing<ArrowRight size={15}/></a>
  <Link href="/new/self-hosting">Explore self-hosting<ArrowRight size={15}/></Link>
 </div></div></section>
 <section id="identity" className="platform-soft"><div className="wrap platform-split"><div><SectionHead eyebrow="Know who’s asking" title="Identify the customer. Control what agents can access." lede="Verify the person behind the conversation, then let agents investigate through selected tools. Customer identity and permission to access data are separate controls." />
 <div className="platform-faqs"><details><summary>How does identity verification work?</summary><p>Your backend signs an identity proof using HMAC. Helpin validates the proof when the widget identifies the customer. Keep the signing secret on your server. <a href={`${REPO}/docs/community/widget-identity.md`}>Set up customer identity →</a></p></details>
 <details><summary>Does verification unlock access to customer data?</summary><p>No. You choose the agent’s tools and approval rules separately. Your connected tool server must also enforce which customer’s data it can return. <a href={`${REPO}/docs/external-mcp-servers.md`}>Configure tool access →</a></p></details></div>
 </div><div><h3 className="developer-demo-heading">An investigation with defined access.</h3><SupportIdentity variant="developer" /></div></div></section>
 <section id="sdk"><div className="wrap"><div className="platform-centered"><SectionHead eyebrow="Support inside your product" title="Put help where customers need it." lede="Start a conversation from your own interface. Associate it with the signed-in customer and their company, then keep the exchange in Helpin."/></div><SDKExplorer/><div className="platform-sdk-links"><a href="https://www.npmjs.com/package/@helpin-ai/sdk-js" target="_blank" rel="noopener noreferrer">JavaScript<ArrowRight size={13}/></a><a href="https://www.npmjs.com/package/@helpin-ai/react" target="_blank" rel="noopener noreferrer">React<ArrowRight size={13}/></a><a href="https://www.npmjs.com/package/@helpin-ai/nextjs" target="_blank" rel="noopener noreferrer">Next.js<ArrowRight size={13}/></a><a href="https://www.npmjs.com/package/@helpin-ai/vue" target="_blank" rel="noopener noreferrer">Vue<ArrowRight size={13}/></a></div><div className="platform-feature-grid platform-sdk-features">{[
  {Icon:MessagesSquare,title:'Start from your own interface.',body:'Use a custom launcher, prepare an opening message, or reopen an existing conversation.'},
  {Icon:KeyRound,title:'Connect the person and company.',body:'Identify signed-in customers so the conversation has a clear account context.'},
  {Icon:BookOpen,title:'Open the relevant guide.',body:'Bring an article into the widget from your product’s help menu or interface.'},
 ].map(({Icon,title,body})=><article key={title}><Icon size={23}/><h3>{title}</h3><p>{body}</p></article>)}</div></div></section>
 <section id="mcp" className="platform-dark section-motion"><HeroVortex variant="converge" tone="dark" /><div className="wrap"><div className="platform-centered"><SectionHead eyebrow="Connect your AI tools" title="Bring Helpin to your AI tools. Bring your tools to Helpin." lede="Make the direction of access explicit: an AI client can work with Helpin, or a Helpin agent can use another system."/></div><div className="platform-mcp-grid"><article><div className="platform-mcp-copy"><span className="platform-micro">YOUR AI CLIENT → HELPIN</span><h3>Keep useful context within reach.</h3><p>Let an authorized AI client retrieve permitted Helpin information and use supported workspace capabilities.</p></div><MCPScene direction="inbound"/><DocLink href={`${REPO}/docs/public-mcp-server.md`}>Set up public MCP</DocLink><p className="platform-beta-note">Controlled beta · Requires environment and workspace enablement.</p></article><article><div className="platform-mcp-copy"><span className="platform-micro">HELPIN AGENT → YOUR TOOLS</span><h3>Give agents a way to check the facts.</h3><p>Connect an external MCP server and select the tools each agent can use. Keep access focused on the job, with approvals where required.</p></div><MCPScene direction="outbound"/><DocLink href={`${REPO}/docs/external-mcp-servers.md`}>Connect an external tool server</DocLink><p className="platform-beta-note">Selected tools · Agent permissions · Action approval policy.</p></article></div></div></section>
 <section id="api"><div className="wrap platform-split"><div><SectionHead eyebrow="Build on the records behind the work" title="Connect your workflows to the workspace." lede="Use authenticated workspace routes for supported operations. Access follows Helpin’s resource permissions and enabled modules." secondaryLede="Check the routes and handlers for the operation your integration needs."/><DocLink href={`${REPO}/server/internal/router/router.go`}>Explore the API routes</DocLink></div><WorkspaceAPIExample/></div></section>
 <section id="webhooks"><div className="wrap platform-split"><div><SectionHead eyebrow="From an event to the next step" title="Start the right work when something changes." lede="Connect supported events to configured agent workflows. Let a matching rule start the job, using the agent’s selected tools, instructions, and approvals." secondaryLede="Inspect trigger activity, including skipped events, separately from agent runs."/><Link className="platform-text-link" href="/new/products/ai-agents#agent-controls">Explore agents and automation<ArrowRight size={15}/></Link><p className="platform-availability">Requires Automation and the relevant event integration to be enabled.</p></div><EventScene/></div></section>
 <section id="developer-faq"><div className="wrap platform-faq-grid"><SectionHead eyebrow="Before you connect" title="Choose the right interface."/><PlatformFAQ items={FAQS}/></div></section>
 <PlatformClosing id="developers-final-title" eyebrow="Build with the history attached" title="Connect your tools. Keep the customer in the picture." description="Bring your product, customer history, and AI agents together. Build the connections that help your team understand the request and carry the work forward." primaryLabel="Start free trial" supportingLine="Open source · Self-host or use Helpin Cloud"/>
 </div><PreviewFooter homepage/></>;}
