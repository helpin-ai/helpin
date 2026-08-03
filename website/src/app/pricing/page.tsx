'use client';

import { useState } from 'react';
import Link from 'next/link';
import { Check, ArrowRight, Zap, Users, Building2, Layers, MessageCircle, BarChart3, FileText, Search, Code2, CheckCircle, Send } from 'lucide-react';
import { HelpinBrand } from '@/components/HelpinBrand';

const SIGNUP_URL = 'https://app.helpin.ai/register';

const PLANS = [
  {
    name: 'Starter',
    price: 99,
    annual: 79,
    description: 'For teams that want the full workspace after trial.',
    Icon: Users,
    color: 'oklch(0.52 0.16 250)',
    seats: 'Unlimited',
    credits: '5,000',
    cta: 'Start Starter trial',
    href: `${SIGNUP_URL}?plan=starter`,
    popular: false,
    features: [
      'All modules — PM, Support, Sales, Docs',
      'Unlimited users',
      '10 teams',
      '500 documents',
      '5,000 contacts',
      'Built-in AI agents and workflows',
      'GitHub integration',
      'Public help center + custom domain',
    ],
  },
  {
    name: 'Growth',
    price: 299,
    annual: 239,
    description: 'For growing teams that want custom AI agents and full control.',
    Icon: Building2,
    color: 'oklch(0.55 0.18 310)',
    seats: 'Unlimited',
    credits: '25,000',
    cta: 'Start Growth trial',
    href: `${SIGNUP_URL}?plan=growth`,
    popular: true,
    features: [
      'Everything in Starter, plus:',
      'Unlimited teams',
      'Unlimited documents',
      'Unlimited contacts',
      'Custom AI agents',
      'Agent scheduling & cron',
      'Advanced automation flows',
      'AI conversation routing',
      'Remove Helpin branding',
      'Priority support',
    ],
  },
];

const COMPARISON_FEATURES = [
  { name: 'Users & Access', category: true },
  { name: 'Users', starter: 'Unlimited', growth: 'Unlimited' },
  { name: 'Teams', starter: '10', growth: 'Unlimited' },
  { name: 'Mobile access', starter: true, growth: true },

  { name: 'Project Management', category: true },
  { name: 'Tasks & stories', starter: 'Unlimited', growth: 'Unlimited' },
  { name: 'Epics', starter: 'Unlimited', growth: 'Unlimited' },
  { name: 'Sprints', starter: true, growth: true },
  { name: 'Board, list, and roadmap views', starter: true, growth: true },
  { name: 'Custom fields', starter: true, growth: true },

  { name: 'Support', category: true },
  { name: 'Live chat widget', starter: true, growth: true },
  { name: 'Shared inbox', starter: true, growth: true },
  { name: 'Team inboxes', starter: true, growth: true },
  { name: 'Saved replies', starter: true, growth: true },
  { name: 'Email forwarding', starter: true, growth: true },
  { name: 'Round-robin assignment', starter: false, growth: true },
  { name: 'SLA management', starter: false, growth: true },
  { name: 'AI conversation routing', starter: false, growth: true },

  { name: 'Sales / CRM', category: true },
  { name: 'Contacts', starter: '5,000', growth: 'Unlimited' },
  { name: 'Deals and pipelines', starter: true, growth: true },
  { name: 'Gmail sync', starter: true, growth: true },
  { name: 'Buyer signal detection', starter: true, growth: true },
  { name: 'Deal automation', starter: false, growth: true },

  { name: 'Docs / Knowledge', category: true },
  { name: 'Documents', starter: '500', growth: 'Unlimited' },
  { name: 'Internal docs', starter: true, growth: true },
  { name: 'Public help center', starter: true, growth: true },
  { name: 'Custom domain', starter: true, growth: true },
  { name: 'AI article translation', starter: false, growth: true },

  { name: 'AI Agents', category: true },
  { name: 'Monthly AI credits', starter: '5,000', growth: '25,000' },
  { name: 'Built-in agents', starter: true, growth: true },
  { name: 'Custom agents', starter: false, growth: true },
  { name: 'Agent scheduling', starter: false, growth: true },
  { name: 'Extra usage packs', starter: '$50 / 5,000', growth: '$50 / 5,000' },

  { name: 'Platform', category: true },
  { name: 'GitHub integration', starter: true, growth: true },
  { name: 'Import tools', starter: true, growth: true },
  { name: 'Module access controls', starter: true, growth: true },
  { name: 'Automation flows', starter: false, growth: true },

  { name: 'Branding', category: true },
  { name: 'Widget branding', starter: 'Helpin', growth: 'Removed' },

  { name: 'Support', category: true },
  { name: 'Standard support', starter: true, growth: true },
  { name: 'Priority support', starter: false, growth: true },
];

const FAQS = [
  {
    q: 'Do I need to buy modules separately?',
    a: 'No. Every plan includes all modules — PM, Support, Sales, and Docs. We don\'t sell features separately. Your whole team gets access to everything from day one.',
  },
  {
    q: 'What happens if I exceed my AI credit limit?',
    a: 'Paid workspaces can enable on-demand credits, billed at $50 per 5,000-credit block. You can also move from Starter to Growth when your team needs more included usage.',
  },
  {
    q: 'What counts as an AI credit?',
    a: 'Credits are consumed based on the type of work. A support AI reply uses 5 credits, a CRM/deal action uses 10, document generation uses 20, planning uses 50, and coding or review runs use 100.',
  },
  {
    q: 'Can I switch plans anytime?',
    a: 'Yes. Upgrade instantly, downgrade at the end of your billing cycle. No contracts, no penalties.',
  },
  {
    q: 'Do you offer annual billing?',
    a: 'Yes. Starter is $99/month or $948/year. Growth is $299/month or $2,868/year.',
  },
  {
    q: 'Is a credit card required for the trial?',
    a: 'No. New workspaces start on a no-card 14-day Growth trial. Add billing only when you are ready to keep using the workspace.',
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
    a: 'Each workspace gets its own plan and billing. Create a workspace, start with a 14-day Growth trial, then choose Starter or Growth. If you do not upgrade, workspace access is limited until you choose a paid plan.',
  },
  {
    q: 'Are seats really unlimited on paid plans?',
    a: 'Yes. Starter and Growth both include unlimited seats. We do not charge per seat — your whole team gets access.',
  },
  {
    q: 'Is my data safe?',
    a: 'Yes. Your data is encrypted at rest and in transit. It is never shared across workspaces and never used to train AI models. Growth includes advanced RBAC.',
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
            Focus on Growth,<br />not the seat count.
          </h1>
          <p className="text-[17px] text-muted-foreground leading-relaxed max-w-2xl mx-auto mb-14">
            Bring projects, support, sales, and docs into one workspace—with AI agents that help every team get more done.
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
        <div className="mx-auto max-w-5xl px-6 lg:px-8">
          <div className="grid gap-6 md:grid-cols-2 lg:gap-8">
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
                  href={plan.href}
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
                <HelpinBrand iconClassName="h-7 w-7" />
                <span className="text-[16px] font-medium text-muted-foreground">(replaces all of the above)</span>
              </div>
              <span className="text-[16px] font-bold text-pop">14-day Growth trial — paid plans from $99/mo</span>
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
              { action: 'Support AI reply', credits: '5', Icon: MessageCircle, color: 'oklch(0.58 0.15 55)', bg: 'oklch(0.58 0.15 55 / 0.08)' },
              { action: 'CRM/deal action', credits: '10', Icon: Send, color: 'oklch(0.52 0.14 28)', bg: 'oklch(0.52 0.14 28 / 0.08)' },
              { action: 'Document generation', credits: '20', Icon: FileText, color: 'oklch(0.55 0.16 160)', bg: 'oklch(0.55 0.16 160 / 0.08)' },
              { action: 'Planning run', credits: '50', Icon: Layers, color: 'oklch(0.55 0.18 310)', bg: 'oklch(0.55 0.18 310 / 0.08)' },
              { action: 'Coding run', credits: '100', Icon: Code2, color: 'oklch(0.50 0.14 200)', bg: 'oklch(0.50 0.14 200 / 0.08)' },
              { action: 'Review run', credits: '100', Icon: CheckCircle, color: 'oklch(0.48 0.15 155)', bg: 'oklch(0.48 0.15 155 / 0.08)' },
              { action: 'Knowledge search', credits: 'Included in action', Icon: Search, color: 'oklch(0.55 0.15 130)', bg: 'oklch(0.55 0.15 130 / 0.08)' },
              { action: 'On-demand block', credits: '5,000 for $50', Icon: Zap, color: 'oklch(0.52 0.16 250)', bg: 'oklch(0.52 0.16 250 / 0.08)' },
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
              <strong className="text-foreground">Need more?</strong> Paid workspaces can enable on-demand 5,000-credit blocks.
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
                  <th className="py-4 px-4 text-[15px] font-bold text-foreground text-center">Starter</th>
                  <th className="py-4 px-4 text-[15px] font-bold text-pop text-center">Growth</th>
                </tr>
              </thead>
              <tbody>
                {COMPARISON_FEATURES.map((row, i) => {
                  if ('category' in row && row.category) {
                    return (
                      <tr key={i}>
                        <td colSpan={3} className="pt-8 pb-3 text-sm font-bold uppercase tracking-widest text-muted-foreground/60">
                          {row.name}
                        </td>
                      </tr>
                    );
                  }
                  return (
                    <tr key={i} className="border-b border-border/30 hover:bg-muted/30 transition-colors">
                      <td className="py-4 pr-4 text-[15px] text-foreground">{row.name}</td>
                      <td className="py-4 px-4 text-center"><CellValue value={row.starter} /></td>
                      <td className="py-4 px-4 text-center"><CellValue value={row.growth} /></td>
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
