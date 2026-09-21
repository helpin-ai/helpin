import Link from 'next/link';
import { ArrowRight, BookOpen, Bot, Building2, Check, ChevronRight, FileText, Kanban, MessagesSquare, UserRound, Video } from 'lucide-react';
import './products-menu.css';

export const PRODUCTS = [
  { label: 'Support', description: 'Answer customer questions with full context.', href: '/new/products/customer-support', icon: MessagesSquare },
  { label: 'Meetings', description: 'Turn customer calls into clear next steps.', href: '/new/products/meetings', icon: Video },
  { label: 'Projects', description: 'Plan, build, and ship what customers need.', href: '/new/products/projects', icon: Kanban },
  { label: 'CRM', description: 'Manage relationships with the work behind them.', href: '/new/products/crm', icon: Building2 },
  { label: 'AI Agents', description: 'Specialist agents that answer, plan, and act.', href: '/new/products/ai-agents', icon: Bot },
  { label: 'Knowledge', description: 'Docs and guides your team and agents can trust.', href: '/new/products/knowledge', icon: BookOpen },
];

export function ProductLink({ item }: { item: typeof PRODUCTS[number] }) {
  const Icon = item.icon;
  return (
    <Link className="nav-product" href={item.href}>
      <span className="nav-product-art" aria-hidden="true"><Icon size={23} strokeWidth={1.5} /></span>
      <span className="nav-product-copy">
        <span className="nav-product-title">{item.label}<ChevronRight size={14} aria-hidden="true" /></span>
        <span className="nav-product-value">{item.description}</span>
      </span>
    </Link>
  );
}

export function AskAgentMenuCard() {
  return <Link href="/new/products/ai-agents#agent-ask" className="nav-products-agent-link">
    <div className="nav-agent-context-art" aria-hidden="true">
      <svg className="nav-agent-context-path" viewBox="0 0 180 250" fill="none"><path d="M42 12C170 36 186 198 6 242"/><path d="M55 25C143 64 169 157 46 214"/></svg>
      <span className="nav-agent-context-card"><MessagesSquare size={16}/><i/><i/></span>
      <span className="nav-agent-context-card"><FileText size={16}/><i/><i/></span>
      <span className="nav-agent-context-card"><UserRound size={16}/><i/><i/></span>
    </div>
    <span className="nav-products-agent-icon"><Bot size={29} strokeWidth={1.5} aria-hidden="true" /></span>
    <span className="nav-products-agent-copy"><small>Built on your customer history</small><b>Ask Agent</b><span>Your workspace assistant. Ask questions, investigate issues, and coordinate specialist agents using customer history.</span></span>
    <span className="nav-products-agent-cta">See Ask Agent in action<ArrowRight size={15} aria-hidden="true" /></span>
    <span className="nav-agent-benefits">{['Faster answers', 'Less context switching', 'Happier customers'].map(item => <span key={item}><Check size={11} aria-hidden="true"/>{item}</span>)}</span>
  </Link>;
}

export function ProductsMenu() {
  return <>
    <div className="nav-products-main">
      <div className="nav-products-intro"><span>Explore the platform</span><h2>Everything you need to build exceptional customer experiences</h2><p>An integrated platform powered by AI and your customer history.</p></div>
      <div className="nav-products-grid">{PRODUCTS.map(item => <ProductLink item={item} key={item.label} />)}</div>
      <div className="nav-products-footer"><Link className="nav-products-explore" href="/new/product"><span>Explore the platform<ArrowRight size={14} aria-hidden="true" /></span><small>See how it all works together.</small></Link></div>
    </div>
    <AskAgentMenuCard/>
  </>;
}
