import Link from 'next/link';
import { ArrowRight, BookOpen, Bot, Building2, CircleUserRound, Kanban, MessagesSquare, Video } from 'lucide-react';
import './products-menu.css';

export const PRODUCTS = [
  { label: 'Customer support', description: 'Resolve questions with context.', href: '/new/products/customer-support', icon: MessagesSquare },
  { label: 'Meetings', description: 'Turn conversations into next steps.', href: '/new/products/meetings', icon: Video },
  { label: 'Projects', description: 'Ship what your customers need.', href: '/new/products/projects', icon: Kanban },
  { label: 'CRM', description: 'Move customer relationships forward.', href: '/new/products/crm', icon: Building2 },
  { label: 'Knowledge', description: 'Trusted answers, in one place.', href: '/new/products/knowledge', icon: BookOpen },
  { label: 'Customer records', description: 'See the full customer story.', href: '/new#record', icon: CircleUserRound },
];

export function ProductLink({ item }: { item: typeof PRODUCTS[number] }) {
  const Icon = item.icon;
  return (
    <Link className="nav-product" href={item.href}>
      <span className="nav-product-art" aria-hidden="true"><Icon size={23} strokeWidth={1.5} /></span>
      <span className="nav-product-copy">
        <span className="nav-product-title">{item.label}<ArrowRight size={14} aria-hidden="true" /></span>
        <span className="nav-product-value">{item.description}</span>
      </span>
    </Link>
  );
}

export function ProductsMenu() {
  return (
    <>
      <div className="nav-products-grid">{PRODUCTS.map(item => <ProductLink item={item} key={item.label} />)}</div>
      <div className="nav-products-footer">
        <Link href="/new/products/ai-agents#agent-ask" className="nav-products-agent-link">
          <span className="nav-products-agent-icon"><Bot size={21} strokeWidth={1.5} aria-hidden="true" /></span>
          <span><b>Ask Agent</b><small>AI across your workspace.</small></span>
          <ArrowRight size={14} aria-hidden="true" />
        </Link>
        <Link className="nav-products-explore" href="/new/product">Explore the platform<ArrowRight size={14} aria-hidden="true" /></Link>
      </div>
    </>
  );
}
