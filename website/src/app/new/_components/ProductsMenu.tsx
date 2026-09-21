import Link from 'next/link';
import { ArrowRight, BookOpen, Bot, Building2, Check, CircleUserRound, FileText, Kanban, MessageSquareText, MessagesSquare, Network, Video } from 'lucide-react';
import './products-menu.css';

export const PRODUCTS = [
  { label: 'Customer support', description: 'Resolve questions with the full customer story.', details: 'AI answers · Shared inbox · Human handoffs', href: '/new/products/customer-support', icon: MessagesSquare, accent: Check },
  { label: 'Meetings', description: 'Turn every conversation into a clear next step.', details: 'Recordings · Summaries · Follow-ups', href: '/new/products/meetings', icon: Video, accent: FileText },
  { label: 'Projects', description: 'Ship the work your customers are waiting for.', details: 'Customer-linked tasks · Sprints · Coding agents', href: '/new/products/projects', icon: Kanban, accent: Check },
  { label: 'CRM', description: 'Know what moves each relationship forward.', details: 'Pipelines · Customer signals · Playbooks', href: '/new/products/crm', icon: Building2, accent: Network },
  { label: 'Knowledge', description: 'Turn what your team knows into trusted answers.', details: 'Help centers · Internal docs · Agent knowledge', href: '/new/products/knowledge', icon: BookOpen, accent: FileText },
  { label: 'Customer records', description: 'Pick up where the last conversation left off.', details: 'Contacts · Companies · Shared history', href: '/new#record', icon: CircleUserRound, accent: MessagesSquare },
];

export function ProductLink({ item }: { item: typeof PRODUCTS[number] }) {
  const Icon = item.icon;
  const Accent = item.accent;
  return (
    <Link className="nav-product" href={item.href}>
      <span className="nav-product-art" aria-hidden="true">
        <Icon className="nav-product-symbol" size={27} strokeWidth={1.5} />
        <span className="nav-product-accent"><Accent size={12} strokeWidth={1.8} /></span>
      </span>
      <span className="nav-product-copy">
        <span className="nav-product-title">{item.label}<ArrowRight size={15} aria-hidden="true" /></span>
        <span className="nav-product-value">{item.description}</span>
        <span className="nav-product-details">{item.details}</span>
      </span>
    </Link>
  );
}

export function ProductsMenu() {
  return (
    <>
      <div className="nav-products-main">
        <div className="nav-products-heading">
          <p className="nav-section-label">The Helpin platform</p>
          <p>Every conversation. A way forward.</p>
        </div>
        <div className="nav-products-grid">{PRODUCTS.map(item => <ProductLink item={item} key={item.label} />)}</div>
        <Link className="nav-products-footer" href="/new/product"><Network size={17} aria-hidden="true" /><span>One workspace. Shared customer context.</span><span className="nav-products-explore">Explore the platform<ArrowRight size={14} aria-hidden="true" /></span></Link>
      </div>
      <Link href="/new/products/ai-agents#agent-ask" className="nav-products-agent">
        <span className="nav-products-agent-label"><Bot size={20} strokeWidth={1.5} aria-hidden="true" />Ask Agent</span>
        <strong>Bring the context.<br />Move the work forward.</strong>
        <p>Find answers across your workspace and prepare the next step for your team.</p>
        <span className="nav-products-workflow" aria-hidden="true">
          <span><MessageSquareText size={16} /><span>A customer needs help</span></span>
          <span><Network size={16} /><span>The context comes together</span></span>
          <span><Check size={16} /><span>A next step, ready to review</span></span>
        </span>
        <span className="nav-products-agent-cta">Meet Ask Agent<ArrowRight size={16} aria-hidden="true" /></span>
      </Link>
    </>
  );
}
