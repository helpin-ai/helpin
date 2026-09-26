'use client';

import { useId, useState } from 'react';
import Link from 'next/link';
import { ProductPreview } from '../product-previews';
import './platform-product-previews.css';
import { ArrowRight, BookOpen, Bot, Building2, FolderKanban, MessagesSquare, Video } from 'lucide-react';

const PRODUCTS = [
  { name: 'Support', preview: 'inbox', Icon: MessagesSquare, title: 'Answer with the earlier conversation in view.', body: 'Handle chat and email together. Give teammates and agents the history to investigate the question and hand it over without starting again.', href: '/products/customer-support' },
  { name: 'Meetings', preview: 'meetings', Icon: Video, title: 'Keep the commitments after the call.', body: 'Capture the discussion, review decisions, and turn agreed next steps into linked work.', href: '/products/meetings' },
  { name: 'Projects', preview: 'projects', Icon: FolderKanban, title: 'Manage the plan—not just the requests.', body: 'Organize roadmaps, sprints, dependencies, and objectives. Keep relevant customer needs attached while your team manages product development, maintenance, and internal work.', href: '/products/projects' },
  { name: 'CRM', preview: 'crm', Icon: Building2, title: 'See the relationship behind the deal.', body: 'Manage contacts, companies, and pipelines alongside the conversations and work that explain the next move.', href: '/products/crm' },
  { name: 'Knowledge', preview: 'knowledge', Icon: BookOpen, title: 'Give people and agents a useful place to look.', body: 'Publish customer guides, maintain internal docs, and select the knowledge your agents can use.', href: '/products/knowledge' },
  { name: 'Agents', preview: 'agents', Icon: Bot, title: 'Put the history to work.', body: 'Use specialists to answer questions, plan tasks, and follow up after the release. Choose their tools and the actions that need review.', href: '/products/ai-agents' },
] as const;

export function CommunityShowcase({ initial = 'Support' }: { initial?: (typeof PRODUCTS)[number]['name'] } = {}) {
  const start = Math.max(0, PRODUCTS.findIndex(product => product.name === initial));
  const [active, setActive] = useState(start);
  const [visited, setVisited] = useState(() => new Set([start]));
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
        <div className="community-product-caption"><div><h3>{product.title}</h3><p>{product.body}</p></div><Link className="platform-text-link" href={product.href}>Explore {product.name === 'Agents' ? 'AI Agents' : product.name}<ArrowRight size={15}/></Link></div>
      </>}
    </div>)}
  </div>;
}

export function CodeContent({code}:{code:string}) {
  // Local presentation only: preserve exact whitespace and copied source.
  const expression = /(\/\/[^\n]*|'[^'\n]*'|"[^"\n]*"|\b(?:import|from|const|export|function|return|true|false|await)\b)/g;
  return <>{code.split('\n').map((line, index) => <span className="platform-code-line" key={index}><span className="platform-line-number" aria-hidden="true">{index + 1}</span><span>{line.split(expression).map((part, token) => <span key={token} className={part.startsWith('//') ? 'code-comment' : /^['"]/.test(part) ? 'code-string' : /^(import|from|const|export|function|return|true|false|await)$/.test(part) ? 'code-keyword' : undefined}>{part}</span>)}{'\n'}</span></span>)}</>;
}
