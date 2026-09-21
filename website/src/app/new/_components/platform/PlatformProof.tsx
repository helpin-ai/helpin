'use client';

import { useId, useState } from 'react';
import Link from 'next/link';
import { ProductPreview } from '../product-previews';
import './platform-product-previews.css';
import { ArrowRight, BookOpen, Bot, Braces, Building2, Check, FolderKanban, LockKeyhole, MessagesSquare, Video } from 'lucide-react';

const PRODUCTS = [
  { name: 'Support', preview: 'inbox', Icon: MessagesSquare, title: 'Answer with the customer history beside you.', body: 'Bring conversations, teammates, and AI assistance into one inbox.', href: '/new/products/customer-support' },
  { name: 'Meetings', preview: 'meetings', Icon: Video, title: 'Turn customer calls into next steps.', body: 'Keep meeting notes, decisions and follow-up tasks attached to the customer.', href: '/new/products/meetings' },
  { name: 'Projects', preview: 'projects', Icon: FolderKanban, title: 'Move customer requests through to release.', body: 'Plan the work, follow progress and keep the original request attached.', href: '/new/products/projects' },
  { name: 'CRM', preview: 'crm', Icon: Building2, title: 'Know the history behind every relationship.', body: 'Manage contacts, companies and deals alongside their conversations and work.', href: '/new/products/crm' },
  { name: 'Knowledge', preview: 'knowledge', Icon: BookOpen, title: 'Publish the answers your customers need.', body: 'Give guides, search, and product knowledge a home under your brand.', href: '/new/products/knowledge' },
  { name: 'Agents', preview: 'agents', Icon: Bot, title: 'Choose the agents that work alongside your team.', body: 'Give each agent a purpose, selected tools, and a model profile.', href: '/new/products/ai-agents' },
] as const;

export function CommunityShowcase() {
  const [active, setActive] = useState(0);
  const [visited, setVisited] = useState(() => new Set([0]));
  const id = useId();
  function select(index: number) {
    setVisited(previous => new Set(previous).add(index));
    setActive(index);
  }
  return <div className="community-showcase">
    <div className="community-switcher" role="group" aria-label="Explore self-hosted products">
      {PRODUCTS.map(({ name, Icon }, index) => <button type="button" key={name} aria-pressed={index === active} aria-controls={`${id}-${index}`} onClick={() => select(index)}><Icon size={18} aria-hidden="true"/><span>{name}</span><ArrowRight size={14} aria-hidden="true"/></button>)}
    </div>
    {PRODUCTS.map((product, index) => <div id={`${id}-${index}`} key={product.preview} hidden={active !== index}>
      {visited.has(index) && <>
        <div className="community-product-stage"><ProductPreview product={product.preview} theme="light" /></div>
        <div className="community-product-caption"><div><h3>{product.title}</h3><p>{product.body}</p></div><Link className="platform-text-link" href={product.href}>Explore {product.name.toLowerCase()}<ArrowRight size={15}/></Link></div>
      </>}
    </div>)}
  </div>;
}

export function CodeContent({code}:{code:string}) {
  // Local presentation only: preserve exact whitespace and copied source.
  const expression = /(\/\/[^\n]*|'[^'\n]*'|"[^"\n]*"|\b(?:import|from|const|export|function|return|true|false|await)\b)/g;
  return <>{code.split('\n').map((line, index) => <span className="platform-code-line" key={index}><span className="platform-line-number" aria-hidden="true">{index + 1}</span><span>{line.split(expression).map((part, token) => <span key={token} className={part.startsWith('//') ? 'code-comment' : /^['"]/.test(part) ? 'code-string' : /^(import|from|const|export|function|return|true|false|await)$/.test(part) ? 'code-keyword' : undefined}>{part}</span>)}{'\n'}</span></span>)}</>;
}

export function WorkspaceAPIExample() {
  return <div className="workspace-api-example">
    <div className="workspace-api-heading"><Braces size={17}/><strong>Work with authorized workspace data.</strong><span><LockKeyhole size={12}/>Authenticated</span></div>
    <div className="workspace-api-endpoint"><span>GET</span><code>/api/workspaces</code></div>
    <div className="workspace-api-auth"><span>Authorization</span><code>Bearer &lt;session_access_token&gt;</code></div>
    <div className="workspace-api-response"><span className="platform-micro"><Check size={12}/>EXAMPLE RESPONSE · SELECTED FIELDS</span><pre tabIndex={0} aria-label="Example workspace response"><code><CodeContent code={'[\n  {\n    "name": "OrbitDesk",\n    "slug": "orbitdesk",\n    "role": "member"\n  }\n]'}/></code></pre></div>
    <div className="workspace-api-note"><LockKeyhole size={14}/><p>Results depend on the authenticated user’s access.</p></div>
    <p className="developer-supporting-note">The public widget key is not a workspace API credential.</p>
  </div>;
}
