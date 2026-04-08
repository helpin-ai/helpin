'use client';

import { useState } from 'react';
import Link from 'next/link';
import { Check, ArrowRight, Zap, Users, Building2, Crown, Layers, MessageCircle, BarChart3, FileText, Search, Code2, CheckCircle, Send, HelpCircle } from 'lucide-react';

const PLANS = [
  {
    name: 'Free',
    price: 0,
    annual: 0,
    description: 'For solo founders and small projects getting started.',
    Icon: Zap,
    color: 'oklch(0.48 0.15 155)',
    seats: '2',
    credits: '50',
    cta: 'Get started free',
    popular: false,
    features: [
      'All modules — PM, Support, Sales, Docs',
      'Tasks, epics & board views',
      'Live chat widget & shared inbox',
      'CRM with contacts & deals',
      'Internal docs & knowledge base',
      'Built-in AI agents (limited)',
      'Import from other tools',
    ],
  },
  {
    name: 'Starter',
    price: 99,
    annual: 79,
    description: 'For teams that need the full platform without limits on work.',
    Icon: Users,
    color: 'oklch(0.52 0.16 250)',
    seats: 'Unlimited',
    credits: '500',
    cta: 'Start free trial',
    popular: false,
    features: [
      'Everything in Free, plus:',
      'Unlimited tasks & epics',
      'Built-in AI agents',
      'GitHub integration',
      'Public help center + custom domain',
      'Basic automations',
      'Analytics & reporting',
      'Unlimited conversation history',
    ],
  },
  {
    name: 'Growth',
    price: 299,
    annual: 249,
    description: 'For growing teams that want custom AI agents and full control.',
    Icon: Building2,
    color: 'oklch(0.55 0.18 310)',
    seats: 'Unlimited',
    credits: '2,500',
    cta: 'Start free trial',
    popular: true,
    features: [
      'Everything in Starter, plus:',
      'Custom AI agents',
      'Advanced automations',
      'Agent scheduling & cron',
      'API access & webhooks',
      'Remove Helpin branding',
      'Advanced RBAC',
      'Advanced analytics',
      'Priority support',
    ],
  },
];

const COMPARISON_FEATURES = [
  { name: 'Users & Access', category: true },
  { name: 'Users', free: '2', starter: 'Unlimited', growth: 'Unlimited', enterprise: 'Unlimited' },
  { name: 'Mobile access', free: true, starter: true, growth: true, enterprise: true },

  { name: 'Project Management', category: true },
  { name: 'Tasks & stories', free: '50 active', starter: 'Unlimited', growth: 'Unlimited', enterprise: 'Unlimited' },
  { name: 'Epics', free: 'Unlimited', starter: 'Unlimited', growth: 'Unlimited', enterprise: 'Unlimited' },
  { name: 'Sprints', free: false, starter: true, growth: true, enterprise: true },
  { name: 'Board & list views', free: true, starter: true, growth: true, enterprise: true },
  { name: 'Custom fields', free: false, starter: true, growth: true, enterprise: true },
  { name: 'Roadmap', free: false, starter: true, growth: true, enterprise: true },

  { name: 'Support', category: true },
  { name: 'Live chat widget', free: true, starter: true, growth: true, enterprise: true },
  { name: 'Shared inbox', free: true, starter: true, growth: true, enterprise: true },
  { name: 'Conversations/month', free: '50', starter: '500', growth: 'Unlimited', enterprise: 'Unlimited' },
  { name: 'Conversation history', free: '30 days', starter: 'Unlimited', growth: 'Unlimited', enterprise: 'Unlimited' },
  { name: 'SLA management', free: false, starter: true, growth: true, enterprise: true },
  { name: 'CSAT surveys', free: false, starter: true, growth: true, enterprise: true },

  { name: 'Sales / CRM', category: true },
  { name: 'Contacts', free: '100', starter: '1,000', growth: '10,000', enterprise: 'Unlimited' },
  { name: 'Deals', free: '10', starter: '100', growth: 'Unlimited', enterprise: 'Unlimited' },
  { name: 'Pipeline & activity', free: true, starter: true, growth: true, enterprise: true },
  { name: 'Email integration', free: false, starter: true, growth: true, enterprise: true },
  { name: 'Deal automation', free: false, starter: false, growth: true, enterprise: true },
  { name: 'Forecasting', free: false, starter: false, growth: true, enterprise: true },

  { name: 'Docs / Knowledge', category: true },
  { name: 'Documents', free: '20', starter: '200', growth: 'Unlimited', enterprise: 'Unlimited' },
  { name: 'Internal docs', free: true, starter: true, growth: true, enterprise: true },
  { name: 'Public help center', free: false, starter: true, growth: true, enterprise: true },
  { name: 'Custom domain', free: false, starter: true, growth: true, enterprise: true },

  { name: 'AI Agents', category: true },
  { name: 'AI credits/month', free: '50', starter: '500', growth: '2,500', enterprise: 'Custom' },
  { name: 'Built-in agents', free: 'Limited', starter: true, growth: true, enterprise: true },
  { name: 'Custom agents', free: false, starter: false, growth: true, enterprise: true },
  { name: 'Agent scheduling', free: false, starter: false, growth: true, enterprise: true },

  { name: 'Platform', category: true },
  { name: 'Storage', free: '500 MB', starter: '5 GB', growth: '50 GB', enterprise: 'Custom' },
  { name: 'GitHub integration', free: false, starter: true, growth: true, enterprise: true },
  { name: 'Import tools', free: true, starter: true, growth: true, enterprise: true },
  { name: 'Basic automations', free: false, starter: true, growth: true, enterprise: true },
  { name: 'Advanced automations', free: false, starter: false, growth: true, enterprise: true },
  { name: 'API access & webhooks', free: false, starter: false, growth: true, enterprise: true },
  { name: 'Analytics', free: false, starter: true, growth: true, enterprise: true },
  { name: 'Advanced analytics', free: false, starter: false, growth: true, enterprise: true },

  { name: 'Branding', category: true },
  { name: 'Widget branding', free: 'Helpin', starter: 'Helpin', growth: 'Removed', enterprise: 'Removed' },

  { name: 'Security & Compliance', category: true },
  { name: 'Advanced RBAC', free: false, starter: false, growth: true, enterprise: true },
  { name: 'SSO / SAML', free: false, starter: false, growth: false, enterprise: true },
  { name: 'Audit logs', free: false, starter: false, growth: false, enterprise: true },
  { name: 'Custom SLA', free: false, starter: false, growth: false, enterprise: true },
  { name: 'Dedicated infrastructure', free: false, starter: false, growth: false, enterprise: true },

  { name: 'Support', category: true },
  { name: 'Community support', free: true, starter: true, growth: true, enterprise: true },
  { name: 'Priority support', free: false, starter: false, growth: true, enterprise: true },
  { name: 'Dedicated success manager', free: false, starter: false, growth: false, enterprise: true },
  { name: 'Onboarding & migration', free: false, starter: false, growth: false, enterprise: true },
];

const FAQS = [
  {
    q: 'Do I need to buy modules separately?',
    a: 'No. Every plan includes all modules — PM, Support, Sales, and Docs. We don\'t sell features separately. Your whole team gets access to everything from day one.',
  },
  {
    q: 'What happens if I exceed my AI credit limit?',
    a: 'Your agents keep working. We\'ll notify you when you\'re approaching your limit, and any overage is billed at a simple per-credit rate. You can also add credit packs anytime.',
  },
  {
    q: 'What counts as an AI credit?',
    a: 'Credits are consumed based on the type of work. Light tasks like support triage use very few credits. Heavier tasks like coding runs or epic planning use more. For example, 100 support triages cost ~20 credits, while 10 coding runs cost ~80 credits. Most teams on Growth never exceed their 2,500 monthly credits.',
  },
  {
    q: 'Can I switch plans anytime?',
    a: 'Yes. Upgrade instantly, downgrade at the end of your billing cycle. No contracts, no penalties.',
  },
  {
    q: 'Do you offer annual billing?',
    a: 'Yes. Annual plans save 20% compared to monthly billing. You can switch from monthly to annual anytime.',
  },
  {
    q: 'Is there a free plan?',
    a: 'Yes. The Free plan includes all modules with 2 seats and 50 AI credits/month — no credit card required. Paid plans include a 14-day free trial.',
  },
  {
    q: 'What AI models does Helpin support?',
    a: 'Helpin supports multiple AI providers including Claude and GPT. AI is built into the platform — no separate setup required.',
  },
  {
    q: 'How does pricing compare to my current tools?',
    a: 'A typical 20-person team pays $1,000-2,000/month across Jira, Intercom, HubSpot, Notion, and ChatGPT. Helpin replaces all of them starting at $99/month on Starter — with AI agents included.',
  },
  {
    q: 'Do unused credits roll over?',
    a: 'Credits reset monthly. Your included credits refresh at the start of each billing cycle. We keep it simple.',
  },
  {
    q: 'How does per-workspace pricing work?',
    a: 'Each workspace (product or brand) gets its own plan and billing. Create a workspace, start with a 14-day free trial, then choose a plan. You can have different plans for different workspaces.',
  },
  {
    q: 'Are seats really unlimited on paid plans?',
    a: 'Yes. Starter and Growth include unlimited seats. The Free plan supports up to 2 seats. We don\'t charge per seat — your whole team gets access.',
  },
  {
    q: 'Is my data safe?',
    a: 'Yes. Your data is encrypted at rest and in transit. It\'s never shared across workspaces and never used to train AI models. Growth includes advanced RBAC. Enterprise adds SSO, audit logs, and dedicated infrastructure.',
  },
];

const COST_COMPARISON = [
  { tool: 'Jira / Linear / ClickUp', cost: '$7-19/seat/mo', domain: 'linear.app' },
  { tool: 'Intercom / Zendesk / Freshdesk', cost: '$15-139/seat/mo', domain: 'intercom.com' },
  { tool: 'HubSpot / Salesforce / Pipedrive', cost: '$14-300/seat/mo', domain: 'hubspot.com' },
  { tool: 'Notion / Confluence / Slite', cost: '$10-20/seat/mo', domain: 'notion.so' },
  { tool: 'ChatGPT / Claude / Copilot', cost: '$20-200/seat/mo', domain: 'openai.com' },
];

function CellValue({ value }: { value: boolean | string | undefined }) {
  if (value === true) return <Check className="w-4 h-4 text-pop mx-auto" />;
  if (value === false || value === undefined) return <span className="text-muted-foreground/30">—</span>;
  return <span className="text-sm font-medium text-foreground">{value}</span>;
}

export default function PricingPage() {
  const [annual, setAnnual] = useState(false);

  return (
    <div className="min-h-screen">

      {/* ══════════════════════════════════
          HERO
          ══════════════════════════════════ */}
      <section className="pt-20 md:pt-28 pb-16 text-center">
        <div className="mx-auto max-w-4xl px-6 lg:px-8">
          <h1 className="text-[clamp(2.25rem,5vw,4.75rem)] font-bold tracking-[-0.04em] leading-[1.0] text-foreground mb-6">
            Pay for output,<br />not headcount.
          </h1>
          <p className="text-[17px] text-muted-foreground leading-relaxed max-w-2xl mx-auto mb-14">
            Your whole team gets PM, Support, Sales, and Docs — with AI agents that plan, build, triage, and follow up. No per-seat pricing.
          </p>

          {/* What's included strip */}
          <div className="rounded-2xl border border-border bg-muted/30 p-8 max-w-3xl mx-auto mb-14">
            <p className="text-[17px] font-semibold text-foreground text-center mb-6">
              Every plan includes all modules and AI agents
            </p>
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
              {[
                { label: 'Project Management', sub: 'Plan, build, ship', Icon: Layers, color: 'oklch(0.52 0.16 250)', bg: 'oklch(0.52 0.16 250 / 0.08)' },
                { label: 'Support', sub: 'Triage, resolve, notify', Icon: MessageCircle, color: 'oklch(0.58 0.15 55)', bg: 'oklch(0.58 0.15 55 / 0.08)' },
                { label: 'Sales / CRM', sub: 'Track, follow up, close', Icon: BarChart3, color: 'oklch(0.52 0.14 28)', bg: 'oklch(0.52 0.14 28 / 0.08)' },
                { label: 'Docs', sub: 'Internal, help center', Icon: FileText, color: 'oklch(0.55 0.16 160)', bg: 'oklch(0.55 0.16 160 / 0.08)' },
              ].map((m) => (
                <div key={m.label} className="text-center">
                  <div className="w-10 h-10 rounded-xl flex items-center justify-center mx-auto mb-3" style={{ background: m.bg }}>
                    <m.Icon className="w-[18px] h-[18px]" style={{ color: m.color }} />
                  </div>
                  <p className="text-[14px] font-semibold text-foreground">{m.label}</p>
                  <p className="text-[12px] text-muted-foreground">{m.sub}</p>
                </div>
              ))}
            </div>
          </div>

          {/* Billing toggle */}
          <div className="flex items-center justify-center gap-3 mb-4">
            <span className={`text-sm font-medium ${!annual ? 'text-foreground' : 'text-muted-foreground'}`}>Monthly</span>
            <button
              onClick={() => setAnnual(!annual)}
              className="relative w-14 h-7 rounded-full transition-colors duration-200"
              style={{ background: annual ? 'var(--color-pop)' : 'var(--color-border)' }}
            >
              <div className="absolute top-0.5 left-0.5 w-6 h-6 rounded-full bg-white shadow transition-transform duration-200"
                style={{ transform: annual ? 'translateX(28px)' : 'translateX(0)' }} />
            </button>
            <span className={`text-sm font-medium ${annual ? 'text-foreground' : 'text-muted-foreground'}`}>
              Annual <span className="text-pop font-semibold">Save 20%</span>
            </span>
          </div>
        </div>
      </section>

      {/* ══════════════════════════════════
          PLAN CARDS
          ══════════════════════════════════ */}
      <section className="pb-24">
        <div className="mx-auto max-w-6xl px-6 lg:px-8">
          <div className="grid md:grid-cols-3 gap-6 lg:gap-8">
            {PLANS.map((plan) => (
              <div
                key={plan.name}
                className="relative rounded-2xl p-8 flex flex-col"
                style={{
                  border: plan.popular ? `2px solid var(--color-pop)` : '1px solid var(--color-border)',
                  background: plan.popular ? 'var(--color-pop-light)' : 'var(--color-background)',
                  boxShadow: plan.popular ? '0 8px 32px oklch(0.48 0.15 155 / 0.08)' : 'none',
                }}
              >
                {plan.popular && (
                  <div className="absolute -top-3 left-1/2 -translate-x-1/2 bg-pop text-white text-xs font-bold uppercase tracking-wider px-4 py-1 rounded-full">
                    Most Popular
                  </div>
                )}

                <div className="mb-6">
                  <div className="flex items-center gap-2 mb-3">
                    <div className="w-8 h-8 rounded-lg flex items-center justify-center" style={{ background: `${plan.color}15` }}>
                      <plan.Icon className="w-4 h-4" style={{ color: plan.color }} />
                    </div>
                    <h3 className="text-lg font-bold text-foreground">{plan.name}</h3>
                  </div>
                  <p className="text-[15px] text-muted-foreground leading-relaxed">{plan.description}</p>
                </div>

                <div className="mb-6">
                  <div className="flex items-baseline gap-1">
                    <span className="text-[clamp(2rem,4vw,3rem)] font-bold text-foreground">
                      ${annual ? plan.annual : plan.price}
                    </span>
                    <span className="text-sm text-muted-foreground">/month</span>
                  </div>
                  {annual && (
                    <p className="text-xs text-pop font-medium mt-1">
                      ${(plan.price - plan.annual) * 12}/year saved
                    </p>
                  )}
                </div>

                {/* Key metrics */}
                <div className="grid grid-cols-2 gap-3 mb-6 py-4 border-y border-border/50">
                  <div className="text-center">
                    <p className="text-lg font-bold text-foreground leading-tight">{plan.seats}</p>
                    <p className="text-[10px] text-muted-foreground uppercase tracking-wide leading-tight mt-1">Seats</p>
                  </div>
                  <div className="text-center">
                    <p className="text-lg font-bold text-foreground leading-tight">{plan.credits}<span className="text-xs font-normal text-muted-foreground">/mo</span></p>
                    <a href="#credits" className="text-[10px] text-muted-foreground uppercase tracking-wide hover:text-pop cursor-pointer leading-tight mt-1 block"
                      onClick={(e) => { e.preventDefault(); document.getElementById('credits')?.scrollIntoView({ behavior: 'smooth' }); }}>
                      AI Credits ↓
                    </a>
                  </div>
                </div>

                <Link
                  href="https://app.helpin.ai"
                  className="block w-full text-center rounded-xl py-3 text-[15px] font-semibold transition-all hover:-translate-y-0.5 mb-6"
                  style={{
                    background: plan.popular ? 'var(--color-foreground)' : 'transparent',
                    color: plan.popular ? 'var(--color-background)' : 'var(--color-foreground)',
                    border: plan.popular ? 'none' : '1px solid var(--color-border)',
                  }}
                >
                  {plan.cta} <ArrowRight className="inline w-4 h-4 ml-1" />
                </Link>

                <ul className="space-y-3 flex-1">
                  {plan.features.map((f, i) => (
                    <li key={i} className="flex items-start gap-2.5">
                      {i === 0 && f.startsWith('Everything') ? (
                        <span className="text-[14px] text-foreground font-semibold">{f}</span>
                      ) : (
                        <>
                          <Check className="w-4 h-4 text-pop flex-shrink-0 mt-0.5" />
                          <span className="text-[14px] text-muted-foreground">{f}</span>
                        </>
                      )}
                    </li>
                  ))}
                </ul>
              </div>
            ))}
          </div>

          {/* Enterprise */}
          <div className="mt-8 rounded-2xl border border-border p-8 md:p-10 flex flex-col md:flex-row items-center justify-between gap-8">
            <div className="flex items-start gap-4 flex-1">
              <div className="w-10 h-10 rounded-lg flex items-center justify-center bg-foreground">
                <Crown className="w-5 h-5 text-background" />
              </div>
              <div>
                <h3 className="text-lg font-bold text-foreground mb-1">Enterprise</h3>
                <p className="text-muted-foreground max-w-lg">
                  Custom users per workspace, custom AI credits, dedicated infrastructure, SSO, custom SLAs, onboarding & migration support. For organizations with complex requirements.
                </p>
              </div>
            </div>
            <Link href="mailto:sales@helpin.ai" className="btn-primary whitespace-nowrap">
              Talk to sales <ArrowRight className="inline w-4 h-4 ml-1" />
            </Link>
          </div>
        </div>
      </section>

      {/* ══════════════════════════════════
          COST COMPARISON
          ══════════════════════════════════ */}
      <section className="py-16 md:py-24 border-t border-border">
        <div className="mx-auto max-w-4xl px-6 lg:px-8">
          <h2 className="text-[clamp(1.875rem,3.5vw,3rem)] font-bold leading-[1.08] tracking-tight text-foreground text-center mb-6">
            Replace 5 tools with one.
          </h2>
          <p className="text-center text-muted-foreground mb-12 max-w-xl mx-auto">
            A typical 10-person team spends $800-2,000/month on disconnected tools. Helpin replaces them all.
          </p>

          <div className="rounded-2xl border border-border overflow-hidden">
            {COST_COMPARISON.map((item, i) => (
              <div key={item.tool} className={`flex items-center justify-between px-6 py-4 ${i < COST_COMPARISON.length - 1 ? 'border-b border-border/50' : ''}`}>
                <div className="flex items-center gap-3">
                  <img src={`/favicons/${item.domain}.png`} className="w-5 h-5" alt={item.tool} />
                  <span className="text-[16px] text-foreground">{item.tool}</span>
                </div>
                <span className="text-[16px] text-muted-foreground">{item.cost}</span>
              </div>
            ))}
            <div className="flex items-center justify-between px-6 py-5 bg-pop-light border-t-2 border-pop">
              <div className="flex items-center gap-3">
                <img src="/logos/helpin-light-mode-logo.svg" className="h-7" alt="Helpin" />
                <span className="text-[16px] font-medium text-muted-foreground">(replaces all of the above)</span>
              </div>
              <span className="text-[16px] font-bold text-pop">Free forever — paid from $99/mo</span>
            </div>
          </div>
        </div>
      </section>

      {/* ══════════════════════════════════
          AI CREDITS EXPLAINED
          ══════════════════════════════════ */}
      <section id="credits" className="py-16 md:py-24 border-t border-border scroll-mt-20">
        <div className="mx-auto max-w-4xl px-6 lg:px-8">
          <h2 className="text-[clamp(1.875rem,3.5vw,3rem)] font-bold leading-[1.08] tracking-tight text-foreground text-center mb-6">
            AI credits, explained simply.
          </h2>
          <p className="text-center text-muted-foreground mb-12 max-w-xl mx-auto">
            Every plan includes AI credits that power your agents. Credits are consumed when agents work.
          </p>

          <div className="grid sm:grid-cols-2 lg:grid-cols-4 gap-4">
            {[
              { action: '100 support triages', credits: '~20', Icon: MessageCircle, color: 'oklch(0.58 0.15 55)', bg: 'oklch(0.58 0.15 55 / 0.08)' },
              { action: '20 story drafts', credits: '~10', Icon: Layers, color: 'oklch(0.52 0.16 250)', bg: 'oklch(0.52 0.16 250 / 0.08)' },
              { action: '10 epic planner runs', credits: '~30', Icon: Layers, color: 'oklch(0.55 0.18 310)', bg: 'oklch(0.55 0.18 310 / 0.08)' },
              { action: '10 coding runs', credits: '~80', Icon: Code2, color: 'oklch(0.50 0.14 200)', bg: 'oklch(0.50 0.14 200 / 0.08)' },
              { action: '50 doc generations', credits: '~25', Icon: FileText, color: 'oklch(0.55 0.16 160)', bg: 'oklch(0.55 0.16 160 / 0.08)' },
              { action: '200 KB searches', credits: '~20', Icon: Search, color: 'oklch(0.55 0.15 130)', bg: 'oklch(0.55 0.15 130 / 0.08)' },
              { action: '50 CRM follow-ups', credits: '~15', Icon: Send, color: 'oklch(0.52 0.14 28)', bg: 'oklch(0.52 0.14 28 / 0.08)' },
              { action: '20 code reviews', credits: '~30', Icon: CheckCircle, color: 'oklch(0.48 0.15 155)', bg: 'oklch(0.48 0.15 155 / 0.08)' },
            ].map((item) => (
              <div key={item.action} className="rounded-xl border border-border p-4 text-center">
                <div className="w-9 h-9 rounded-lg flex items-center justify-center mx-auto mb-3" style={{ background: item.bg }}>
                  <item.Icon className="w-[16px] h-[16px]" style={{ color: item.color }} />
                </div>
                <p className="text-sm font-medium text-foreground mb-1">{item.action}</p>
                <p className="text-xs text-muted-foreground">{item.credits} credits</p>
              </div>
            ))}
          </div>

          <div className="mt-10 rounded-xl bg-muted/50 p-6 text-center">
            <p className="text-sm text-muted-foreground leading-relaxed max-w-lg mx-auto">
              <strong className="text-foreground">Need more?</strong> Add credit packs anytime. Your agents never stop working.
            </p>
          </div>
        </div>
      </section>

      {/* ══════════════════════════════════
          FEATURE COMPARISON TABLE
          ══════════════════════════════════ */}
      <section className="py-16 md:py-24 border-t border-border">
        <div className="mx-auto max-w-5xl px-6 lg:px-8">
          <h2 className="text-[clamp(1.875rem,3.5vw,3rem)] font-bold leading-[1.08] tracking-tight text-foreground text-center mb-6">
            Compare plans at a glance.
          </h2>
          <p className="text-center text-muted-foreground mb-12">
            All plans include PM, Support, Sales, and Docs. The difference is scale and control.
          </p>

          <div className="overflow-x-auto">
            <table className="w-full text-left">
              <thead>
                <tr className="border-b border-border">
                  <th className="py-4 pr-4 text-[15px] font-medium text-muted-foreground w-[30%]">Feature</th>
                  <th className="py-4 px-4 text-[15px] font-bold text-foreground text-center">Free</th>
                  <th className="py-4 px-4 text-[15px] font-bold text-foreground text-center">Starter</th>
                  <th className="py-4 px-4 text-[15px] font-bold text-pop text-center">Growth</th>
                  <th className="py-4 pl-4 text-[15px] font-bold text-foreground text-center">Enterprise</th>
                </tr>
              </thead>
              <tbody>
                {COMPARISON_FEATURES.map((row, i) => {
                  if ('category' in row && row.category) {
                    return (
                      <tr key={i}>
                        <td colSpan={5} className="pt-8 pb-3 text-sm font-bold uppercase tracking-widest text-muted-foreground/60">
                          {row.name}
                        </td>
                      </tr>
                    );
                  }
                  return (
                    <tr key={i} className="border-b border-border/30 hover:bg-muted/30 transition-colors">
                      <td className="py-4 pr-4 text-[15px] text-foreground">{row.name}</td>
                      <td className="py-4 px-4 text-center"><CellValue value={row.free} /></td>
                      <td className="py-4 px-4 text-center"><CellValue value={row.starter} /></td>
                      <td className="py-4 px-4 text-center"><CellValue value={row.growth} /></td>
                      <td className="py-4 pl-4 text-center"><CellValue value={row.enterprise} /></td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </div>
      </section>

      {/* ══════════════════════════════════
          FAQ
          ══════════════════════════════════ */}
      <section className="py-16 md:py-24 border-t border-border">
        <div className="mx-auto max-w-3xl px-6 lg:px-8">
          <h2 className="text-[clamp(1.875rem,3.5vw,3rem)] font-bold leading-[1.08] tracking-tight text-foreground text-center mb-12">
            Frequently asked questions
          </h2>

          <div className="space-y-0">
            {FAQS.map((faq, i) => (
              <details key={i} className="group border-b border-border/50 py-5">
                <summary className="flex items-center justify-between cursor-pointer list-none">
                  <h3 className="text-[18px] font-semibold text-foreground pr-8">{faq.q}</h3>
                  <span className="text-muted-foreground/40 text-xl flex-shrink-0 transition-transform duration-200 group-open:rotate-45">+</span>
                </summary>
                <p className="text-[17px] text-muted-foreground leading-relaxed mt-4 pr-12">{faq.a}</p>
              </details>
            ))}
          </div>
        </div>
      </section>

    </div>
  );
}
