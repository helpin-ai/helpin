import { HeroVortex } from '../_components/HeroVortex';
import { CtaRow, CtaNote } from '../_components/ui';
import { previewMetadata } from '../_components/preview-metadata';
import Link from 'next/link';
import { ArrowRight, BookOpen, Bot, Braces, Code2, KeyRound, MessagesSquare, Plug, Terminal, Webhook } from 'lucide-react';
import { PreviewNav } from '../_components/PreviewNav';
import { PreviewFooter } from '../_components/PreviewFooter';
import { GITHUB_URL, GithubIcon, SectionHead } from '../_components/ui';
import { DeveloperHeroScene, EventScene, MCPScene, SDKExplorer } from '../_components/platform/PlatformScenes';
import { DocLink, PlatformBreadcrumb, PlatformClosing, PlatformFAQ, REPO } from '../_components/platform/PlatformParts';
import { WorkspaceAPIExample } from '../_components/platform/PlatformProof';
import { DeveloperInterfaces } from '../_components/platform/DeveloperInterfaces';
import { SupportIdentity } from '../products/customer-support/support-identity';
import '../_components/platform/platform.css';
import '../_components/platform/platform-polish.css';

export const metadata = previewMetadata("Developers \u2014 Helpin", "/new/developers");
const FAQS = [
  [
    "Where should I start?",
    "Use the SDK for customer chat, public MCP for AI clients working with Helpin, and external MCP for Helpin agents using your tools.",
    "/new/developers#developer-interfaces"
  ],
  [
    "Can I use the SDK with self-hosted Helpin?",
    "Yes. Use your installation’s public widget URL and allow your website origin in workspace settings.",
    "https://github.com/helpin-ai/helpin/blob/develop/docs/community/widget-identity.md"
  ],
  [
    "What is the difference between the two MCP connections?",
    "Public MCP lets an external AI client access selected Helpin capabilities. External MCP gives a Helpin agent selected tools from another server.",
    "https://github.com/helpin-ai/helpin/blob/develop/docs/public-mcp-server.md"
  ],
  [
    "Can a webhook start agent work?",
    "Yes, where Automation is on. Events are checked against your rules; matching ones start the agent you chose.",
    "https://github.com/helpin-ai/helpin/blob/develop/docs/agents-and-automation.md"
  ],
  ["Which SDKs are available in the codebase?", "JavaScript, React, Next.js and Vue packages are available. Check each package’s README for setup and published versions.", "https://github.com/helpin-ai/helpin/tree/develop/packages"],
  ["Is the widget key also a workspace API credential?", "No. The public widget key identifies your widget; workspace APIs and MCP connections use separate authentication and permissions.", "https://github.com/helpin-ai/helpin/blob/develop/docs/community/widget-identity.md"],
  ["How is customer identity verified?", "After login, your server signs a short-lived identity proof for the SDK. Helpin checks it separately from the public widget key.", "https://github.com/helpin-ai/helpin/blob/develop/docs/community/widget-identity.md"],
  ["Is public MCP enabled everywhere?", "Public MCP is in controlled beta and must be enabled for the environment and workspace. Tools are limited by granted scopes, user permissions and enabled modules.", "https://github.com/helpin-ai/helpin/blob/develop/docs/public-mcp-server.md"]
] as const;
export default function DevelopersPage(){return <><PreviewNav/><div className="platform-page developers-page">
 <section className="platform-hero motion-hero"><HeroVortex variant="connections" tone="dark" /><div className="wrap"><PlatformBreadcrumb label="Developers"/><div className="platform-hero-grid"><div className="platform-hero-copy"><span className="eyebrow">Support SDKs, workspace APIs and AI tools</span><h1>Add Helpin to your app. <span>Connect it to your AI tools.</span></h1><p className="lede">Drop support chat into your product with the SDK. Use Helpin from Claude or Cursor over MCP. Give Helpin agents tools from your own systems. Build on the workspace API.</p><CtaRow /><CtaNote /><div className="platform-hero-points"><span><Braces size={14}/>APIs & SDKs</span><span><Plug size={14}/>MCP</span><span><Webhook size={14}/>Events & workflows</span></div></div><div><DeveloperHeroScene/></div></div></div></section>
 <nav className="platform-page-nav" aria-label="On this page"><div className="wrap"><strong>Developers</strong><a href="#sdk">SDKs</a><a href="#mcp">MCP</a><a href="#api">Workspace API</a><a href="#webhooks">Events & workflows</a><a href="#developer-resources">Resources</a></div></nav>

 <section id="developer-interfaces"><div className="wrap"><SectionHead eyebrow="Build with Helpin" title="Four ways to connect the work." /><div className="platform-resource-grid">{[
  {Icon:Terminal,title:'Helpin CLI',body:'Install, inspect and manage your own instance.',href:'/new/self-hosting#cli'},
  {Icon:Braces,title:'APIs & SDKs',body:'Add support chat and connect your customer data.',href:'#sdk'},
  {Icon:Plug,title:'MCP, both ways',body:'Use Helpin from an AI client or connect external tools to agents.',href:'#mcp'},
  {Icon:Webhook,title:'Webhooks & triggers',body:'Start configured workflows from product and engineering events.',href:'#webhooks'},
 ].map(({Icon,title,body,href})=><a key={title} href={href}><Icon size={22}/><h3>{title}</h3><p>{body}</p><ArrowRight size={16}/></a>)}</div></div></section>
 <section id="identity" className="platform-soft"><div className="wrap platform-split"><div><SectionHead eyebrow="Identity & tools" title="Verified customers, scoped access." lede="Sign customer identities on your backend with HMAC, then let agents use selected logs and account tools over MCP. Identity verification and tool authorization are separate controls." />
 <div className="platform-faqs"><details><summary>How does HMAC identity verification work?</summary><p>Your backend signs a short-lived customer identity proof with a server-side secret. Helpin validates it when the widget identifies the customer. <a href={`${REPO}/docs/community/widget-identity.md`}>Identity setup →</a></p></details>
 <details><summary>Does verified identity grant access to connected tools?</summary><p>No. Select agent tools and approvals separately, and enforce customer access in your tool server. <a href={`${REPO}/docs/external-mcp-servers.md`}>Tool access guide →</a></p></details></div>
 </div><SupportIdentity /></div></section>
 <section id="sdk"><div className="wrap"><div className="platform-centered"><SectionHead eyebrow="Meet customers inside your product" title="Bring the conversation into your app." lede="Open a conversation from your own UI. Identify the customer, carry their company context, and connect them to your support team."/></div><SDKExplorer/><div className="platform-sdk-links"><a href={`${GITHUB_URL}/tree/develop/packages/sdk-js`} target="_blank" rel="noopener noreferrer">JavaScript<ArrowRight size={13}/></a><a href={`${GITHUB_URL}/tree/develop/packages/react`} target="_blank" rel="noopener noreferrer">React<ArrowRight size={13}/></a><a href={`${GITHUB_URL}/tree/develop/packages/nextjs`} target="_blank" rel="noopener noreferrer">Next.js<ArrowRight size={13}/></a><a href={`${GITHUB_URL}/tree/develop/packages/vue`} target="_blank" rel="noopener noreferrer">Vue<ArrowRight size={13}/></a></div><div className="platform-feature-grid platform-sdk-features">{[
  {Icon:MessagesSquare,title:'Choose where the conversation starts.',body:'Use your own launcher, start a conversation with an opening message, or bring an existing conversation back into view.'},
  {Icon:KeyRound,title:'Connect the person behind the session.',body:'Identify the customer and associate company context. Use a server-signed proof when upgrading to a verified identity.'},
  {Icon:BookOpen,title:'Put an answer within reach.',body:'Open help articles in the widget and connect support with your product’s existing navigation and help entry points.'},
 ].map(({Icon,title,body})=><article key={title}><Icon size={23}/><h3>{title}</h3><p>{body}</p></article>)}</div></div></section>
 <section id="mcp" className="platform-dark"><div className="wrap"><div className="platform-centered"><SectionHead eyebrow="Connect your AI tools" title="MCP in both directions." lede="Use Helpin context in an external AI client, or bring another system’s tools to a Helpin agent. Each connection gets the access you choose."/></div><div className="platform-mcp-grid"><article><div className="platform-mcp-copy"><span className="platform-micro">YOUR AI CLIENT → HELPIN</span><h3>Bring Helpin into your AI client.</h3><p>Search permitted workspace context, read docs, and perform supported actions through a workspace-bound MCP connection.</p></div><MCPScene direction="inbound"/><DocLink href={`${REPO}/docs/public-mcp-server.md`}>Public MCP setup</DocLink><p className="platform-beta-note">Controlled beta · Requires environment and workspace enablement.</p></article><article><div className="platform-mcp-copy"><span className="platform-micro">HELPIN AGENT → YOUR TOOLS</span><h3>Bring external tools to your agents.</h3><p>Connect an external server, review its tools, and select what each agent can use. Installing a server doesn’t grant every agent access.</p></div><MCPScene direction="outbound"/><DocLink href={`${REPO}/docs/external-mcp-servers.md`}>External MCP setup</DocLink><p className="platform-beta-note">Selected tools · Agent permissions · Action approval policy.</p></article></div></div></section>
 <section id="api"><div className="wrap platform-split"><div><SectionHead eyebrow="Build on the workspace" title="The workspace API." lede="Read and work with the records behind your customer workflows. Requests use Helpin authentication and the same resource permissions as the product." secondaryLede="Start with the authenticated workspace API. Inspect the routes and handlers for the operations your integration needs."/><DocLink href={`${REPO}/server/internal/router/router.go`}>Explore the API routes</DocLink></div><WorkspaceAPIExample/></div></section>
 <section id="webhooks"><div className="wrap platform-split"><div><SectionHead eyebrow="Connect events to the next action" title="Turn an event into agent work." lede="A repository update can start a review. A matching rule can launch an agent. Keep the trigger, the work, and the result connected." secondaryLede="Inspect trigger activity separately from agent runs, including events that were skipped."/><DocLink href={`${REPO}/docs/agents-and-automation.md`}>Read about agents and automation</DocLink><p className="platform-availability">Requires Automation and the relevant event integration to be enabled.</p></div><EventScene/></div></section>
 <section id="developer-resources" className="developer-resources-green"><div className="wrap"><div className="developer-resources-intro"><SectionHead eyebrow="Go straight to the implementation" title="Pick your starting point." lede="Bring support into your app, connect your AI tools, or run Helpin yourself. Find the code and guides for what you want to build."/><DeveloperInterfaces/></div><div className="platform-resource-grid">{[
  {Icon:Code2,title:'JavaScript SDK',body:'Installation, identity, widget commands, and configuration.',href:`${GITHUB_URL}/tree/develop/packages/sdk-js`},
  {Icon:KeyRound,title:'Widget identity',body:'Server-signed customer identity and verification.',href:`${REPO}/docs/community/widget-identity.md`},
  {Icon:Plug,title:'MCP server',body:'Client setup, scopes, toolsets, and service accounts.',href:`${REPO}/docs/public-mcp-server.md`},
  {Icon:Bot,title:'External MCP',body:'Tool discovery, agent access, and connection management.',href:`${REPO}/docs/external-mcp-servers.md`},
  {Icon:Terminal,title:'Helpin CLI',body:'Install, configure, inspect, and operate an instance.',href:`${REPO}/docs/community/cli.md`},
  {Icon:GitIcon,title:'Contributing',body:'Explore the source and contribute to the product.',href:`${REPO}/CONTRIBUTING.md`},
 ].map(({Icon,title,body,href})=><a key={title} href={href} target="_blank" rel="noopener noreferrer"><Icon size={22}/><h3>{title}</h3><p>{body}</p><ArrowRight size={16}/></a>)}</div><div className="platform-self-host-link" role="region" aria-labelledby="developer-self-host-title"><Terminal size={22} aria-hidden="true"/><div><h3 id="developer-self-host-title">Run the workspace you’re building on.</h3><p>Explore Docker Compose deployment, the CLI, and self-hosting configuration.</p></div><Link className="platform-text-link" href="/new/self-hosting">Explore self-hosting<ArrowRight size={15}/></Link></div></div></section>
 <section id="developer-faq"><div className="wrap platform-faq-grid"><SectionHead eyebrow="Questions" title="Choose the right interface for the job."/><PlatformFAQ items={FAQS}/></div></section>
 <PlatformClosing id="developers-final-title" title="Build on the customer history your team already uses." description="Connect your application, your tools, and the work your team does next."/>
 </div><PreviewFooter/></>;}
function GitIcon({size}:{size?:number}){return <GithubIcon size={size}/>}
