'use client';

import { useState } from 'react';
import Link from 'next/link';
import { ArrowRight, BookOpen, Bot, Braces, Check, LockKeyhole, MessagesSquare } from 'lucide-react';

const PRODUCTS = [
  { name: 'Support', Icon: MessagesSquare, title: 'Answer with the customer history beside you.', body: 'Bring conversations, teammates, and AI assistance into one inbox.', href: '/new/products/customer-support', image: '/new/support/inbox-orbitdesk-sep20-1920.webp', srcSet: '/new/support/inbox-orbitdesk-sep20-960.webp 960w, /new/support/inbox-orbitdesk-sep20-1920.webp 1920w, /new/support/inbox-orbitdesk-sep20-3840.webp 3840w', alt: 'OrbitDesk inbox showing a customer conversation, Helpin AI assistance, and the customer record.' },
  { name: 'Knowledge', Icon: BookOpen, title: 'Publish the answers your customers need.', body: 'Give guides, search, and product knowledge a home under your brand.', href: '/new/products/knowledge', image: '/new/knowledge/help-center-1920-v1.webp', srcSet: '/new/knowledge/help-center-960-v1.webp 960w, /new/knowledge/help-center-1920-v1.webp 1920w, /new/knowledge/help-center-3840-v1.webp 3840w', alt: 'OrbitDesk help center showing integration documentation, article navigation, and search.' },
  { name: 'Agents', Icon: Bot, title: 'Choose the agents that work alongside your team.', body: 'Give each agent a purpose, selected tools, and a model profile.', href: '/new/products/ai-agents', image: '', srcSet: '', alt: '' },
];

export function CommunityShowcase() {
  const [active, setActive] = useState(0);
  const product = PRODUCTS[active];
  return <div className="community-showcase">
    <div className="community-switcher" role="group" aria-label="Explore Community products">
      {PRODUCTS.map(({ name, Icon }, index) => <button type="button" key={name} aria-pressed={index === active} aria-controls="community-product-view" onClick={() => setActive(index)}><Icon size={18}/><span>{name}</span><ArrowRight size={14}/></button>)}
    </div>
    <div id="community-product-view">
      <div className="community-product-stage">{active===2?<CommunityAgents/>:<img key={product.image} src={product.image} srcSet={product.srcSet} sizes="(max-width: 1280px) calc(100vw - 64px), 1200px" width={3840} height={2160} loading="lazy" alt={product.alt}/>}</div>
      <div className="community-product-caption"><div><h3>{product.title}</h3><p>{product.body}</p></div><Link className="platform-text-link" href={product.href}>Explore {product.name.toLowerCase()}<ArrowRight size={15}/></Link></div>
    </div>
  </div>;
}

function CommunityAgents() {
 return <div className="community-agents-art" role="img" aria-label="Support agent uses selected knowledge to help answer customers. Docs agent prepares documentation updates for review. Each agent works with the tools and model profile selected for it."><div aria-hidden="true"><span className="platform-micro">YOUR KNOWLEDGE, IN THE HANDS OF YOUR AGENTS</span><div className="community-agent-source"><BookOpen size={20}/><span>OrbitDesk Help Center</span><small>Selected knowledge</small></div><div className="community-agent-branches"><i/><i/></div><div className="community-agent-pair">{[{name:'Support agent',id:'echo',job:'Help answer the customer',detail:'Work from customer history and selected knowledge.',outcome:'An answer grounded in your docs'},{name:'Docs agent',id:'quill',job:'Keep the knowledge useful',detail:'Prepare focused documentation changes for review.',outcome:'A draft ready for your team'}].map(agent=><div key={agent.name}><img src={`/new/agents/${agent.id}.svg`} width={55} height={55} alt=""/><strong>{agent.name}</strong><span>{agent.job}</span><p>{agent.detail}</p><div><Check size={14}/>{agent.outcome}</div></div>)}</div><div className="community-agent-settings"><span>Selected tools</span><span>Model profiles</span><span>Approval controls</span></div></div></div>;
}

export function CodeContent({code}:{code:string}) {
  // Local presentation only: preserve exact whitespace and copied source.
  const expression = /(\/\/[^\n]*|'[^'\n]*'|"[^"\n]*"|\b(?:import|from|const|export|function|return|true|false|await)\b)/g;
  return <>{code.split('\n').map((line, index) => <span className="platform-code-line" key={index}><span className="platform-line-number" aria-hidden="true">{index + 1}</span><span>{line.split(expression).map((part, token) => <span key={token} className={part.startsWith('//') ? 'code-comment' : /^['"]/.test(part) ? 'code-string' : /^(import|from|const|export|function|return|true|false|await)$/.test(part) ? 'code-keyword' : undefined}>{part}</span>)}{'\n'}</span></span>)}</>;
}

export function WorkspaceAPIExample() {
  return <div className="workspace-api-example">
    <div className="workspace-api-heading"><Braces size={17}/><strong>Workspace API</strong><span><LockKeyhole size={12}/>Authenticated</span></div>
    <div className="workspace-api-endpoint"><span>GET</span><code>/api/workspaces</code></div>
    <div className="workspace-api-auth"><span>Authorization</span><code>Bearer &lt;session_access_token&gt;</code></div>
    <div className="workspace-api-response"><span className="platform-micro"><Check size={12}/>RESPONSE SHAPE · SELECTED FIELDS</span><pre tabIndex={0} aria-label="Example workspace response"><code><CodeContent code={'[\n  {\n    "name": "OrbitDesk",\n    "slug": "orbitdesk",\n    "role": "member"\n  }\n]'}/></code></pre></div>
    <div className="workspace-api-note"><LockKeyhole size={14}/><p>Only workspaces the signed-in user can access. Product operations also check resource permissions and enabled modules.</p></div>
  </div>;
}
