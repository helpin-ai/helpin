'use client';

import Link from 'next/link';
import { useEffect, useRef } from 'react';

/* ─── Intersection-observer hooks ───────────────── */

function useReveal() {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    const obs = new IntersectionObserver(
      ([e]) => { if (e.isIntersecting) { el.classList.add('visible'); obs.disconnect(); } },
      { threshold: 0.08 }
    );
    obs.observe(el);
    return () => obs.disconnect();
  }, []);
  return ref;
}

function useStagger() {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    const children = Array.from(el.children) as HTMLElement[];
    const obs = new IntersectionObserver(
      ([e]) => {
        if (e.isIntersecting) {
          children.forEach((c, i) => setTimeout(() => c.classList.add('visible'), i * 60));
          obs.disconnect();
        }
      },
      { threshold: 0.04 }
    );
    obs.observe(el);
    return () => obs.disconnect();
  }, []);
  return ref;
}

/* ─── Data ───────────────────────────────────────── */

const STACK = [
  { tool: 'Linear',   desc: 'tasks live here' },
  { tool: 'Notion',   desc: 'docs live here' },
  { tool: 'Intercom', desc: 'support lives here' },
  { tool: 'HubSpot',  desc: 'customers live here' },
  { tool: 'Slack',    desc: 'decisions die here' },
  { tool: 'ChatGPT',  desc: 'AI lives here — alone, without context' },
  { tool: 'You',      desc: 'holding it all together, manually, every day', dim: true },
];

const AGENTS = [
  { n: '01', name: 'Project Agent',    role: 'Turns strategy into epics, sprints, and tasks. Keeps execution connected to why it matters.' },
  { n: '02', name: 'Support Agent',    role: 'Triages conversations, resolves known issues, escalates to the right human — with full context intact.' },
  { n: '03', name: 'Docs Agent',       role: 'Writes, updates, and connects documentation to the work that created it. Docs that stay alive.' },
  { n: '04', name: 'Knowledge Agent',  role: 'Surfaces the right answer at the right moment. Ends the question "where did we write that down?"' },
  { n: '05', name: 'CRM Agent',        role: 'Keeps customer context where work happens. Bridges sales and product without copy-paste.' },
  { n: '06', name: 'Escalation Agent', role: 'Catches what slips through. Routes unresolved issues before they become the Friday fire.' },
  { n: '07', name: 'Operations Agent', role: 'Runs recurring workflows on autopilot. Standups, reports, reviews — no manual kick-off ever again.' },
  { n: '08', name: 'Onboarding Agent', role: 'Personalised activation for every new customer, every time. Zero manual touchpoints from your team.' },
];

const SCENARIOS = [
  {
    before: 'A support ticket gets resolved but the underlying bug never reaches the product team.',
    after:  'Support Agent logs it, Escalation Agent routes it to product, it lands in the sprint — automatically.',
  },
  {
    before: 'Docs go stale the moment the product ships. Nobody has time to update them.',
    after:  'Docs Agent detects the change and updates documentation before anyone notices it\'s wrong.',
  },
  {
    before: '12 Slack messages asking where a decision was made. You wrote it somewhere.',
    after:  'Knowledge Agent surfaces the decision, the context, and who made it — in seconds.',
  },
  {
    before: 'New customers churn in week two because onboarding is a PDF and a prayer.',
    after:  'Onboarding Agent runs a personalised activation flow for every customer, every time.',
  },
  {
    before: 'Your AI tools write summaries. They can\'t actually do anything with them.',
    after:  'Helpin agents don\'t summarise — they move the next step forward inside the system.',
  },
  {
    before: 'Leadership wants a status update. You spend your Friday pulling it together manually.',
    after:  'Operations Agent assembles it from live data and sends it before you open your laptop.',
  },
];

const LAWS = [
  { n: '01', title: 'Knowledge compounds',   body: 'Every decision, doc, and resolved issue becomes shared context. Nothing gets buried or forgotten.' },
  { n: '02', title: 'Agents get sharper',    body: 'Helpin agents operate with full company context. The more they handle, the more accurately they act.' },
  { n: '03', title: 'Teams stay aligned',    body: 'When the system is shared, silos disappear. Every team operates from the same source of truth.' },
  { n: '04', title: 'Execution accelerates', body: 'No more manual handoffs. Agents carry context from step to step without you bridging the gap.' },
];

const LOGOS = ['Acme Corp', 'Buildfast', 'Launchpad', 'Nexus Studio', 'Orbit HQ', 'Skyline'];

/* ─── Mockup component ───────────────────────────── */

function AppMockup() {
  return (
    <div className="w-full rounded-xl overflow-hidden border border-[oklch(27%_0.046_265)] bg-[oklch(17%_0.043_265)] shadow-[0_0_0_1px_oklch(100%_0_0/0.04),0_32px_80px_oklch(0%_0_0/0.6)]">
      {/* Title bar */}
      <div className="flex items-center gap-2 px-4 h-10 border-b border-[oklch(18%_0.015_250)] bg-[oklch(14%_0.040_265)]">
        <div className="flex gap-1.5">
          {['bg-red-500/40', 'bg-yellow-500/40', 'bg-green-500/40'].map((c, i) => (
            <div key={i} className={`w-2.5 h-2.5 rounded-full ${c}`} />
          ))}
        </div>
        <div className="flex-1 flex justify-center">
          <div className="bg-[oklch(20%_0.045_265)] rounded-md px-3 h-5 flex items-center">
            <span className="text-[var(--fs-label)] text-[oklch(38%_0.018_250)]">app.helpin.ai/workspace</span>
          </div>
        </div>
      </div>

      {/* App layout */}
      <div className="flex" style={{ height: '420px' }}>
        {/* Sidebar */}
        <div className="w-48 shrink-0 border-r border-[oklch(22%_0.046_265)] bg-[oklch(15%_0.041_265)] flex flex-col p-3 hidden sm:flex">
          <div className="flex items-center gap-2 px-2 py-2 mb-3">
            <div className="w-5 h-5 rounded-md bg-[var(--color-pop)] flex items-center justify-center shrink-0">
              <svg width="10" height="10" viewBox="0 0 10 10" fill="none"><path d="M1 5h8M5 1.5l3.5 3.5L5 8.5" stroke="white" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/></svg>
            </div>
            <span className="text-[var(--fs-secondary)] font-semibold text-[oklch(88%_0.006_80)]">Helpin</span>
          </div>

          <div className="space-y-0.5 flex-1">
            {[
              { label: 'Projects',  color: 'bg-sky-400',    active: false },
              { label: 'Support',   color: 'bg-[var(--color-pop)]', active: true  },
              { label: 'CRM',       color: 'bg-emerald-400', active: false },
              { label: 'Docs',      color: 'bg-violet-400',  active: false },
              { label: 'Knowledge', color: 'bg-amber-400',   active: false },
            ].map((item) => (
              <div
                key={item.label}
                className={`flex items-center gap-2.5 px-2 py-[7px] rounded-md text-[var(--fs-secondary)] ${item.active ? 'bg-[oklch(16%_0.014_250)] text-[oklch(90%_0.006_80)]' : 'text-[oklch(62%_0.018_260)] hover:text-[oklch(70%_0.01_80)]'}`}
              >
                <div className={`w-1.5 h-1.5 rounded-full ${item.color} opacity-80`} />
                {item.label}
              </div>
            ))}
          </div>

          <div className="border-t border-[oklch(22%_0.046_265)] pt-3 mt-3">
            <div className="text-[var(--fs-label)] font-semibold tracking-[0.14em] uppercase text-[oklch(38%_0.022_265)] px-2 mb-2">Agents live</div>
            {['Support', 'Docs', 'Onboarding'].map((a) => (
              <div key={a} className="flex items-center gap-2 px-2 py-1">
                <div className="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-pulse" />
                <span className="text-[var(--fs-sm)] text-[oklch(40%_0.018_250)]">{a}</span>
              </div>
            ))}
          </div>
        </div>

        {/* Main panel */}
        <div className="flex-1 overflow-hidden flex flex-col">
          {/* Panel header */}
          <div className="flex items-center justify-between px-5 py-3 border-b border-[oklch(22%_0.046_265)] shrink-0">
            <div>
              <h3 className="text-[var(--fs-base)] font-semibold text-[oklch(90%_0.006_80)]">Support Queue</h3>
              <p className="text-[var(--fs-sm)] text-[oklch(42%_0.018_250)] mt-0.5">6 open · 4 handled by agent</p>
            </div>
            <div className="flex items-center gap-2">
              <div className="flex items-center gap-1.5 px-2.5 py-1 rounded-full bg-[var(--color-pop-10)] border border-[var(--color-pop-20)]">
                <div className="w-1.5 h-1.5 rounded-full bg-[var(--color-pop)] animate-pulse" />
                <span className="text-[var(--fs-sm)] font-medium text-[var(--color-pop-light)]">Support Agent active</span>
              </div>
            </div>
          </div>

          {/* Ticket list */}
          <div className="flex-1 overflow-y-auto p-4 space-y-2">
            {[
              { title: 'Billing issue — charged twice',   status: 'Resolved',    statusColor: 'text-emerald-400 bg-emerald-400/10',     ai: true,  time: '2m ago'  },
              { title: 'How to export reports?',          status: 'In progress', statusColor: 'text-sky-400 bg-sky-400/10',             ai: true,  time: '7m ago'  },
              { title: 'API rate limit hit',              status: 'Escalated',   statusColor: 'text-[var(--color-pop-light)] bg-[var(--color-pop-12)]', ai: false, time: '14m ago' },
              { title: 'SSO setup not working',          status: 'Open',        statusColor: 'text-[oklch(58%_0.018_260)] bg-[oklch(22%_0.046_265)]',  ai: false, time: '22m ago' },
              { title: 'Data export via API endpoint?',  status: 'Resolved',    statusColor: 'text-emerald-400 bg-emerald-400/10',     ai: true,  time: '38m ago' },
              { title: 'Webhook events not firing',      status: 'In progress', statusColor: 'text-sky-400 bg-sky-400/10',             ai: true,  time: '1h ago'  },
            ].map((t, i) => (
              <div
                key={i}
                className="group flex items-center gap-3 px-3 py-2.5 rounded-lg bg-[oklch(19%_0.044_265)] border border-[oklch(18%_0.014_250/0.6)] hover:border-[oklch(28%_0.046_265)] transition-colors cursor-default"
              >
                <div className="flex-1 min-w-0">
                  <p className="text-[var(--fs-secondary)] font-medium text-[oklch(82%_0.006_80)] truncate">{t.title}</p>
                  <p className="text-[var(--fs-sm)] text-[oklch(48%_0.020_262)] mt-0.5">{t.time}{t.ai ? ' · AI handled' : ''}</p>
                </div>
                <span className={`shrink-0 text-[var(--fs-sm)] font-medium px-2.5 py-1 rounded-full ${t.statusColor}`}>
                  {t.status}
                </span>
              </div>
            ))}
          </div>
        </div>

        {/* Right panel — agent activity */}
        <div className="w-56 shrink-0 border-l border-[oklch(22%_0.046_265)] p-4 hidden lg:block overflow-y-auto">
          <div className="text-[var(--fs-label)] font-semibold tracking-[0.14em] uppercase text-[oklch(38%_0.022_265)] mb-3">Agent activity</div>
          <div className="space-y-3">
            {[
              { agent: 'Support Agent',    action: 'Resolved billing issue #4821',    time: '2m' },
              { agent: 'Docs Agent',       action: 'Updated API rate limits article',  time: '8m' },
              { agent: 'Escalation Agent', action: 'Routed #4819 → Engineering',       time: '14m' },
              { agent: 'CRM Agent',        action: 'Linked ticket to Acme account',    time: '22m' },
              { agent: 'Support Agent',    action: 'Resolved export question #4817',   time: '38m' },
              { agent: 'Onboarding Agent', action: 'Sent day-3 activation to 3 users', time: '1h' },
            ].map((item, i) => (
              <div key={i} className="flex gap-2.5">
                <div className="w-1.5 h-1.5 rounded-full bg-[var(--color-pop)] shrink-0 mt-1.5" />
                <div>
                  <p className="text-[var(--fs-sm)] font-medium text-[oklch(55%_0.018_250)]">{item.agent}</p>
                  <p className="text-[var(--fs-label)] text-[oklch(38%_0.015_250)] leading-tight mt-0.5">{item.action}</p>
                  <p className="text-[var(--fs-label)] text-[oklch(28%_0.013_250)] mt-1">{item.time} ago</p>
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}

/* ─── Page ───────────────────────────────────────── */

export default function HomePage() {
  const problemRef  = useReveal();
  const stackRef    = useStagger();
  const catRef      = useReveal();
  const agentHdRef  = useReveal();
  const agentsRef   = useStagger();
  const baHdRef     = useReveal();
  const baRef       = useReveal();
  const lawsHdRef   = useReveal();
  const lawsRef     = useStagger();
  const nowRef      = useReveal();
  const ctaRef      = useReveal();

  return (
    <>
      {/* ═══════════════════════════════════════════
          HERO
      ═══════════════════════════════════════════ */}
      <section className="relative bg-[oklch(13%_0.038_265)] overflow-hidden">
        {/* Ambient glow */}
        <div
          aria-hidden
          className="accent-glow w-[700px] h-[400px] top-[-80px] left-1/2 -translate-x-1/2"
          style={{ background: 'radial-gradient(ellipse, oklch(0.65 0.17 42 / 0.08) 0%, transparent 70%)' }}
        />

        {/* Subtle grid */}
        <div aria-hidden className="absolute inset-0 grid-lines opacity-30" />

        {/* Content */}
        <div className="relative z-10 flex flex-col items-center text-center px-6 pt-20 pb-0">

          {/* Badge */}
          <div
            className="hero-fade inline-flex items-center gap-2 rounded-full border border-[oklch(28%_0.046_265)] bg-[oklch(17%_0.043_265)] px-3.5 py-1.5 mb-7"
            style={{ animationDelay: '80ms' }}
          >
            <span className="w-1.5 h-1.5 rounded-full bg-[var(--color-pop)]" />
            <span className="text-[var(--fs-sm)] font-semibold text-[oklch(58%_0.018_250)] tracking-wide">
              AI Work Operating System · Early Access
            </span>
          </div>

          {/* Headline */}
          <h1
            className="hero-fade text-[clamp(2.4rem,5.6vw,4.8rem)] font-bold leading-[1.06] tracking-[-0.03em] text-[oklch(96%_0.005_80)] max-w-[820px] mb-5"
            style={{ animationDelay: '200ms' }}
          >
            Stop managing tools.
            <br />
            <span className="text-[oklch(48%_0.02_250)]">Start running a company.</span>
          </h1>

          {/* Sub */}
          <p
            className="hero-fade text-[oklch(68%_0.018_260)] text-[var(--fs-lg)] max-w-[480px] leading-[1.7] mb-9"
            style={{ animationDelay: '340ms' }}
          >
            Helpin agents handle your projects, docs, support, and customers — connected, automated, always on. One system that actually runs the company.
          </p>

          {/* CTAs */}
          <div
            className="hero-fade flex items-center gap-3 flex-wrap justify-center mb-16"
            style={{ animationDelay: '460ms' }}
          >
            <Link
              href="https://helpin.ai"
              className="inline-flex items-center gap-2 rounded-lg bg-[oklch(96%_0.005_80)] text-[oklch(15%_0.041_265)] px-5 py-2.5 text-[var(--fs-base)] font-semibold tracking-tight transition-all duration-200 hover:bg-white hover:shadow-[0_0_20px_oklch(96%_0.005_80/0.15)] hover:-translate-y-px"
            >
              Get started free
              <svg width="12" height="12" viewBox="0 0 12 12" fill="none" aria-hidden>
                <path d="M1 6h10M6.5 1.5L11 6l-4.5 4.5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
              </svg>
            </Link>
            <Link
              href="https://helpin.ai"
              className="inline-flex items-center gap-2 rounded-lg border border-[oklch(28%_0.046_265)] bg-transparent text-[oklch(66%_0.018_260)] px-5 py-2.5 text-[var(--fs-base)] font-medium tracking-tight transition-all duration-200 hover:border-[oklch(30%_0.018_250)] hover:text-[oklch(78%_0.010_80)]"
            >
              See how it works
            </Link>
          </div>

          {/* App mockup */}
          <div
            className="hero-fade w-full max-w-6xl mx-auto"
            style={{ animationDelay: '580ms' }}
          >
            <AppMockup />
          </div>
        </div>

        {/* Bottom fade */}
        <div aria-hidden className="absolute bottom-0 left-0 right-0 h-1 bg-[oklch(12%_0.013_250)]" />
      </section>

      {/* ═══════════════════════════════════════════
          LOGO BAR
      ═══════════════════════════════════════════ */}
      <div className="bg-[oklch(15%_0.041_265)] border-b border-[oklch(22%_0.046_265)] py-5 px-6">
        <div className="max-w-5xl mx-auto flex flex-col sm:flex-row items-center justify-center gap-4 sm:gap-8">
          <span className="text-[var(--fs-sm)] font-semibold tracking-[0.12em] uppercase text-[oklch(38%_0.022_265)] whitespace-nowrap">
            Trusted by teams at
          </span>
          <div className="w-px h-4 bg-[oklch(27%_0.046_265)] hidden sm:block" />
          <div className="flex items-center gap-6 flex-wrap justify-center">
            {LOGOS.map((name) => (
              <span key={name} className="text-[var(--fs-secondary)] font-semibold text-[oklch(34%_0.016_250)] tracking-tight">
                {name}
              </span>
            ))}
          </div>
        </div>
      </div>

      {/* ═══════════════════════════════════════════
          THE PROBLEM
      ═══════════════════════════════════════════ */}
      <section className="max-w-6xl mx-auto px-6 py-24 lg:py-36">
        <div ref={problemRef} className="reveal grid grid-cols-1 lg:grid-cols-2 gap-14 lg:gap-20">
          <div>
            <div className="inline-flex items-center gap-2 rounded-full border border-[oklch(28%_0.046_265)] px-3 py-1 mb-6">
              <span className="text-[var(--fs-sm)] font-semibold text-[var(--color-pop)] tracking-wide uppercase">The Problem</span>
            </div>
            <h2 className="text-[clamp(1.8rem,3.2vw,2.8rem)] font-bold leading-[1.1] tracking-[-0.025em] text-[oklch(92%_0.006_80)] mb-5">
              Your company is paying rent on ten empty buildings
            </h2>
            <p className="text-[oklch(68%_0.018_260)] text-[var(--fs-md)] leading-[1.75] mb-4">
              Every tool you use was built to solve one problem. None of them were built to run a company. So you become the human API — copying, pasting, bridging the gap that no one else closes.
            </p>
            <p className="text-[oklch(68%_0.018_260)] text-[var(--fs-md)] leading-[1.75]">
              And when you add AI on top of disconnected tools, you don't get a smarter company. You get smarter chaos.
            </p>
          </div>

          <div ref={stackRef} className="stagger self-center">
            {STACK.map((item) => (
              <div
                key={item.tool}
                className="reveal flex items-baseline justify-between py-3.5 border-b border-[oklch(22%_0.046_265)] first:border-t"
              >
                <span className={`text-[var(--fs-base)] font-semibold ${item.dim ? 'text-[oklch(52%_0.018_260)]' : 'text-[oklch(80%_0.006_80)]'}`}>
                  {item.tool}
                </span>
                <span className="text-[oklch(52%_0.018_260)] text-[var(--fs-secondary)] text-right ml-4">{item.desc}</span>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* ═══════════════════════════════════════════
          MARQUEE
      ═══════════════════════════════════════════ */}
      <div className="overflow-hidden border-y border-[oklch(22%_0.046_265)] py-3 bg-[oklch(12%_0.036_265)]" aria-hidden>
        <div className="marquee-track">
          {[0, 1].map((outer) => (
            <div key={outer} className="flex shrink-0">
              {AGENTS.map((a) => (
                <span key={a.n} className="flex items-center gap-4 px-6 whitespace-nowrap">
                  <span className="text-[var(--color-pop)] text-[var(--fs-label)] font-bold tracking-[0.16em] uppercase">{a.n}</span>
                  <span className="text-[oklch(32%_0.016_250)] text-[var(--fs-sm)] font-semibold tracking-[0.06em] uppercase">{a.name}</span>
                </span>
              ))}
            </div>
          ))}
        </div>
      </div>

      {/* ═══════════════════════════════════════════
          CATEGORY
      ═══════════════════════════════════════════ */}
      <section className="px-6 py-8">
        <div ref={catRef} className="reveal max-w-6xl mx-auto rounded-2xl bg-[oklch(16%_0.042_265)] border border-[oklch(17%_0.014_250)] px-8 py-14 lg:px-16 lg:py-20 overflow-hidden relative">
          <div aria-hidden className="absolute top-0 right-0 w-[500px] h-[300px] opacity-50" style={{background: 'radial-gradient(ellipse at top right, oklch(0.65 0.17 42 / 0.06), transparent 70%)'}} />
          <div className="relative z-10 grid grid-cols-1 lg:grid-cols-[3fr_2fr] gap-12 lg:gap-16 items-center">
            <div>
              <div className="inline-flex items-center gap-2 rounded-full border border-[oklch(28%_0.046_265)] px-3 py-1 mb-6">
                <span className="text-[var(--fs-sm)] font-semibold text-[var(--color-pop)] tracking-wide uppercase">A New Category</span>
              </div>
              <h2 className="text-[clamp(1.8rem,3.4vw,3rem)] font-bold leading-[1.08] tracking-[-0.025em] text-[oklch(93%_0.006_80)] mb-5">
                Your tools manage work.
                <br />
                <span className="text-[oklch(58%_0.018_260)]">Helpin runs the company.</span>
              </h2>
              <p className="text-[oklch(66%_0.018_260)] text-[var(--fs-md)] leading-[1.75] max-w-md">
                One system where agents connect projects, docs, support, and customer context — so nothing falls through the cracks and AI can actually move work forward.
              </p>
            </div>
            <div>
              <p className="text-[var(--fs-label)] font-bold tracking-[0.18em] uppercase text-[oklch(36%_0.022_265)] mb-4">Replaces your entire stack</p>
              <div className="flex flex-wrap gap-2 mb-5">
                {['project management', 'help desk', 'knowledge base', 'CRM', 'AI chatbot', 'wiki', 'automation tool'].map((item) => (
                  <span key={item} className="border border-[oklch(24%_0.046_265)] text-[oklch(38%_0.022_265)] px-2.5 py-1 text-[var(--fs-sm)] rounded line-through decoration-[oklch(24%_0.014_250)]">
                    {item}
                  </span>
                ))}
              </div>
              <div className="inline-flex items-center gap-2.5 bg-[var(--color-pop-10)] border border-[var(--color-pop-25)] rounded-lg px-4 py-2.5">
                <div className="w-2 h-2 rounded-full bg-[var(--color-pop)]" />
                <span className="text-[var(--fs-secondary)] font-semibold text-[var(--color-pop-light)]">AI Work Operating System</span>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* ═══════════════════════════════════════════
          AGENTS
      ═══════════════════════════════════════════ */}
      <section className="max-w-6xl mx-auto px-6 py-24 lg:py-36">
        <div ref={agentHdRef} className="reveal mb-12">
          <div className="inline-flex items-center gap-2 rounded-full border border-[oklch(28%_0.046_265)] px-3 py-1 mb-6">
            <span className="text-[var(--fs-sm)] font-semibold text-[var(--color-pop)] tracking-wide uppercase">Meet the Team</span>
          </div>
          <h2 className="text-[clamp(1.8rem,3.2vw,2.8rem)] font-bold leading-[1.1] tracking-[-0.025em] text-[oklch(92%_0.006_80)] max-w-2xl mb-4">
            Hire a Helpin agent for every job your company needs done
          </h2>
          <p className="text-[oklch(68%_0.018_260)] text-[var(--fs-md)] leading-[1.75] max-w-xl">
            Helpin agents aren't features. They're workers. Each one owns a job, shares context with the others, and operates across the full system — not inside a single tool.
          </p>
        </div>

        <div ref={agentsRef} className="stagger border-t border-[oklch(22%_0.046_265)]">
          {AGENTS.map((agent) => (
            <div
              key={agent.n}
              className="reveal group flex items-start gap-6 lg:gap-10 py-4 border-b border-[oklch(22%_0.046_265)] -mx-3 px-3 rounded-lg transition-colors duration-150 hover:bg-[oklch(17%_0.043_265)] cursor-default"
            >
              <span className="text-[var(--color-pop)] text-[var(--fs-label)] font-bold font-mono tracking-widest pt-[3px] shrink-0 w-7 text-right">
                {agent.n}
              </span>
              <div className="flex-1 flex flex-col sm:flex-row sm:items-baseline gap-1 sm:gap-8">
                <span className="text-[oklch(82%_0.006_80)] font-semibold text-[var(--fs-base)] shrink-0 min-w-[164px]">
                  {agent.name}
                </span>
                <span className="text-[oklch(62%_0.018_260)] text-[var(--fs-base)] leading-[1.6]">
                  {agent.role}
                </span>
              </div>
            </div>
          ))}
        </div>
      </section>

      {/* ═══════════════════════════════════════════
          BEFORE / AFTER
      ═══════════════════════════════════════════ */}
      <section className="bg-[oklch(14%_0.040_265)] border-y border-[oklch(22%_0.046_265)] py-24 lg:py-36">
        <div className="max-w-6xl mx-auto px-6">
          <div ref={baHdRef} className="reveal mb-12">
            <div className="inline-flex items-center gap-2 rounded-full border border-[oklch(28%_0.046_265)] px-3 py-1 mb-6">
              <span className="text-[var(--fs-sm)] font-semibold text-[var(--color-pop)] tracking-wide uppercase">Life with Helpin</span>
            </div>
            <h2 className="text-[clamp(1.8rem,3.2vw,2.8rem)] font-bold leading-[1.1] tracking-[-0.025em] text-[oklch(92%_0.006_80)] max-w-2xl">
              What changes when agents run the work
            </h2>
          </div>

          <div ref={baRef} className="reveal grid grid-cols-1 sm:grid-cols-2 rounded-2xl overflow-hidden border border-[oklch(24%_0.046_265)]">
            <div className="bg-[oklch(15%_0.041_265)] p-7 lg:p-10 border-b sm:border-b-0 sm:border-r border-[oklch(24%_0.046_265)]">
              <div className="text-[var(--fs-label)] font-bold tracking-[0.16em] uppercase text-[oklch(36%_0.022_265)] mb-6 pb-4 border-b border-[oklch(22%_0.046_265)]">
                Without Helpin
              </div>
              <div className="divide-y divide-[oklch(20%_0.045_265)]">
                {SCENARIOS.map((s, i) => (
                  <p key={i} className="py-3.5 text-[var(--fs-base)] leading-[1.65] text-[oklch(56%_0.018_260)]">
                    {s.before}
                  </p>
                ))}
              </div>
            </div>
            <div className="bg-[oklch(17%_0.043_265)] p-7 lg:p-10">
              <div className="text-[var(--fs-label)] font-bold tracking-[0.16em] uppercase text-[var(--color-pop-muted)] mb-6 pb-4 border-b border-[var(--color-pop-15)]">
                With Helpin Agents
              </div>
              <div className="divide-y divide-[oklch(20%_0.045_265)]">
                {SCENARIOS.map((s, i) => (
                  <p key={i} className="py-3.5 text-[var(--fs-base)] leading-[1.65] text-[oklch(78%_0.008_80)]">
                    {s.after}
                  </p>
                ))}
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* ═══════════════════════════════════════════
          FOUR LAWS
      ═══════════════════════════════════════════ */}
      <section className="max-w-6xl mx-auto px-6 py-24 lg:py-36">
        <div ref={lawsHdRef} className="reveal mb-14">
          <div className="inline-flex items-center gap-2 rounded-full border border-[oklch(28%_0.046_265)] px-3 py-1 mb-6">
            <span className="text-[var(--fs-sm)] font-semibold text-[var(--color-pop)] tracking-wide uppercase">The Compounding Advantage</span>
          </div>
          <h2 className="text-[clamp(1.8rem,3.2vw,2.8rem)] font-bold leading-[1.1] tracking-[-0.025em] text-[oklch(92%_0.006_80)] max-w-2xl mb-4">
            Most tools make you more dependent as you scale.
            <br />
            <span className="text-[oklch(58%_0.018_260)]">Helpin makes you smarter.</span>
          </h2>
          <p className="text-[oklch(68%_0.018_260)] text-[var(--fs-md)] leading-[1.75] max-w-xl">
            Every Helpin agent operates inside the same connected system. As your company grows, the context grows with it. Agents improve because they know more — not because you configure more.
          </p>
        </div>

        <div ref={lawsRef} className="stagger grid grid-cols-1 sm:grid-cols-2 gap-px bg-[oklch(22%_0.046_265)] rounded-2xl overflow-hidden border border-[oklch(22%_0.046_265)]">
          {LAWS.map((law) => (
            <div key={law.n} className="reveal bg-[oklch(14%_0.040_265)] p-8 hover:bg-[oklch(10.5%_0.012_250)] transition-colors">
              <div className="text-[3.5rem] font-bold text-[oklch(16%_0.014_250)] leading-none mb-5 select-none tabular-nums">
                {law.n}
              </div>
              <div className="text-[oklch(82%_0.006_80)] font-semibold text-[var(--fs-md)] mb-2">{law.title}</div>
              <div className="text-[oklch(62%_0.018_260)] text-[var(--fs-base)] leading-[1.65]">{law.body}</div>
            </div>
          ))}
        </div>
      </section>

      {/* ═══════════════════════════════════════════
          WHY NOW
      ═══════════════════════════════════════════ */}
      <section className="bg-[oklch(14%_0.040_265)] border-y border-[oklch(22%_0.046_265)] py-24 lg:py-36 overflow-hidden relative">
        <div aria-hidden className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[800px] h-[400px] opacity-60" style={{background: 'radial-gradient(ellipse, oklch(0.65 0.17 42 / 0.05), transparent 70%)'}} />
        <div ref={nowRef} className="reveal max-w-6xl mx-auto px-6 relative z-10">
          <div className="inline-flex items-center gap-2 rounded-full border border-[oklch(28%_0.046_265)] px-3 py-1 mb-8">
            <span className="text-[var(--fs-sm)] font-semibold text-[var(--color-pop)] tracking-wide uppercase">Why Now</span>
          </div>
          <div className="grid grid-cols-1 lg:grid-cols-[3fr_1fr] gap-12 items-end">
            <div>
              <h2 className="text-[clamp(2rem,4vw,3.6rem)] font-bold leading-[1.06] tracking-[-0.03em] text-[oklch(93%_0.006_80)] mb-6 max-w-3xl">
                The companies that run on connected systems today will be{' '}
                <span className="text-[var(--color-pop)]">untouchable</span> in three years.
              </h2>
              <p className="text-[oklch(66%_0.018_260)] text-[var(--fs-md)] leading-[1.75] max-w-xl mb-4">
                AI is not a feature you add to a broken stack. It's a reason to replace it. Helpin was built from scratch for a world where people and agents operate together — not in separate apps that don't talk.
              </p>
              <p className="text-[oklch(52%_0.018_260)] text-[var(--fs-md)] leading-[1.75] max-w-xl">
                The companies adopting this model now are not just moving faster. They are building an operational advantage that is very difficult to undo.
              </p>
            </div>
            <div className="hidden lg:block pb-1 text-right" aria-hidden>
              <div className="text-[6rem] font-black text-[oklch(20%_0.045_265)] leading-none select-none tabular-nums tracking-tight">
                2026
              </div>
              <p className="text-[oklch(26%_0.014_250)] text-[var(--fs-label)] font-bold tracking-[0.16em] uppercase mt-1">
                The window is open.
              </p>
            </div>
          </div>
        </div>
      </section>

      {/* ═══════════════════════════════════════════
          FINAL CTA
      ═══════════════════════════════════════════ */}
      <section className="max-w-6xl mx-auto px-6 py-28 lg:py-44">
        <div ref={ctaRef} className="reveal text-center max-w-3xl mx-auto">
          <h2 className="text-[clamp(2.2rem,4.8vw,4.2rem)] font-bold leading-[1.06] tracking-[-0.03em] text-[oklch(93%_0.006_80)] mb-5">
            Your first agent is one click away.
            <br />
            <span className="text-[oklch(52%_0.018_260)]">The rest of your team can wait.</span>
          </h2>
          <p className="text-[oklch(58%_0.018_260)] text-[var(--fs-md)] leading-[1.75] mb-10">
            Free during early access. No credit card required.
          </p>
          <div className="flex items-center gap-3 justify-center flex-wrap">
            <Link
              href="https://helpin.ai"
              className="inline-flex items-center gap-2 rounded-lg bg-[oklch(96%_0.005_80)] text-[oklch(15%_0.041_265)] px-6 py-3 text-[var(--fs-md)] font-semibold tracking-tight transition-all duration-200 hover:bg-white hover:shadow-[0_0_30px_oklch(96%_0.005_80/0.12)] hover:-translate-y-px"
            >
              Start free
              <svg width="12" height="12" viewBox="0 0 12 12" fill="none" aria-hidden>
                <path d="M1 6h10M6.5 1.5L11 6l-4.5 4.5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
              </svg>
            </Link>
            <Link
              href="https://helpin.ai"
              className="inline-flex items-center rounded-lg border border-[oklch(28%_0.046_265)] text-[oklch(56%_0.018_250)] px-6 py-3 text-[var(--fs-md)] font-medium tracking-tight transition-all duration-200 hover:border-[oklch(30%_0.018_250)] hover:text-[oklch(72%_0.012_80)]"
            >
              See the demo
            </Link>
          </div>
          <p className="text-[oklch(28%_0.014_250)] text-[var(--fs-secondary)] mt-6">
            Free during early access — pricing comes later. Get in now.
          </p>
        </div>
      </section>
    </>
  );
}
