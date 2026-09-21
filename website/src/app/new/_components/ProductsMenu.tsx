'use client';

import { useState } from 'react';
import Link from 'next/link';
import { ArrowRight, BookOpen, Bot, Building2, Check, CircleUserRound, FileText, GitBranch, History, Kanban, MessagesSquare, Network, Search, TrendingUp, Video } from 'lucide-react';
import './products-menu.css';

export const PRODUCTS = [
  { label: 'Customer support', description: 'Answer faster, with every conversation in context.', details: 'AI answers · Shared inbox · Human handoffs', href: '/new/products/customer-support', icon: MessagesSquare, accent: Check, prompt: 'Help me resolve this customer’s issue.', context: ['Conversation', 'Customer history'], outcome: 'Prepare a reply and link the work still needed.' },
  { label: 'Meetings', description: 'Leave every call with a clear way forward.', details: 'Recordings · Summaries · Follow-ups', href: '/new/products/meetings', icon: Video, accent: FileText, prompt: 'What did we agree to do on this call?', context: ['Meeting notes', 'Customer record'], outcome: 'Pull out decisions, owners, and next steps.' },
  { label: 'Projects', description: 'Ship the work your customers are waiting for.', details: 'Customer-linked tasks · Sprints · Coding agents', href: '/new/products/projects', icon: Kanban, accent: GitBranch, prompt: 'Which customer requests should we tackle next?', context: ['Customer requests', 'Project tasks'], outcome: 'Review the context and propose priorities.' },
  { label: 'CRM', description: 'See what moves each relationship forward.', details: 'Pipelines · Customer signals · Playbooks', href: '/new/products/crm', icon: Building2, accent: TrendingUp, prompt: 'Help me prepare for this account’s renewal.', context: ['Account history', 'Open issues'], outcome: 'Surface concerns and prepare a follow-up.' },
  { label: 'Knowledge', description: 'Give customers and agents answers they can trust.', details: 'Help centers · Internal docs · Agent knowledge', href: '/new/products/knowledge', icon: BookOpen, accent: Search, prompt: 'What do our docs say about this question?', context: ['Published docs', 'Internal knowledge'], outcome: 'Find relevant guidance to inform the answer.' },
  { label: 'Customer records', description: 'Pick up the relationship with the full story.', details: 'Contacts · Companies · Shared history', href: '/new#record', icon: CircleUserRound, accent: History, prompt: 'Catch me up before I speak to this customer.', context: ['Conversations', 'Account activity'], outcome: 'Bring the customer’s recent history together.' },
];
type Product = typeof PRODUCTS[number];

const WORKSPACE_EXAMPLE = {
  label: 'Across your workspace',
  prompt: 'What does this customer need from us next?',
  context: ['Conversations', 'Connected work'],
  outcome: 'Find the context. Prepare the next step for review.',
};

export function ProductLink({ item, onPreview, previewed = false }: { item: Product; onPreview?: (item: Product) => void; previewed?: boolean }) {
  const Icon = item.icon;
  const Accent = item.accent;
  return (
    <Link className="nav-product" href={item.href} data-previewed={previewed || undefined}
      onPointerEnter={event => { if (event.pointerType === 'mouse') onPreview?.(item); }} onFocus={() => onPreview?.(item)}>
      <span className="nav-product-art" aria-hidden="true">
        <Icon className="nav-product-symbol" size={25} strokeWidth={1.5} />
        <span className="nav-product-accent"><Accent size={11} strokeWidth={1.8} /></span>
      </span>
      <span className="nav-product-copy">
        <span className="nav-product-title">{item.label}<ArrowRight size={14} aria-hidden="true" /></span>
        <span className="nav-product-value">{item.description}</span>
        <span className="nav-product-details">{item.details}</span>
      </span>
    </Link>
  );
}

export function ProductsMenu() {
  const [preview, setPreview] = useState<Product | null>(null);
  const example = preview ?? WORKSPACE_EXAMPLE;
  return (
    <>
      <div className="nav-products-main">
        <div className="nav-products-heading">
          <p className="nav-section-label">Explore the products</p>
          <p>Start with a customer. Keep the work connected.</p>
        </div>
        <div className="nav-products-grid">{PRODUCTS.map(item => <ProductLink item={item} key={item.label} onPreview={setPreview} previewed={preview?.label === item.label} />)}</div>
        <Link className="nav-products-footer" href="/new/product"><span className="nav-products-platform-icon"><Network size={16} aria-hidden="true" /></span><span>Built to work together.</span><span className="nav-products-explore">Explore the platform<ArrowRight size={14} aria-hidden="true" /></span></Link>
      </div>
      <aside className="nav-products-agent" aria-label="How Ask Agent connects your products">
        <div className="nav-products-agent-label"><span><Bot size={23} strokeWidth={1.5} aria-hidden="true" /></span><div>Ask Agent<small>Your context, put to work</small></div></div>
        <h2>One question.<br />A workspace of context.</h2>
        <div className="nav-products-example">
          <p className="nav-products-example-label">{example.label}</p>
          <p className="nav-products-prompt"><span>Try asking</span>“{example.prompt}”</p>
          <div className="nav-products-context" aria-label="Connected context">{example.context.map(source => <span key={source}>{source}</span>)}</div>
          <p className="nav-products-outcome"><Check size={14} aria-hidden="true" /><span>{example.outcome}</span></p>
        </div>
        <Link href="/new/products/ai-agents#agent-ask" className="nav-products-agent-cta">Explore Ask Agent<ArrowRight size={15} aria-hidden="true" /></Link>
      </aside>
    </>
  );
}
