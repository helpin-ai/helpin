'use client';

import Link from 'next/link';
import { useEffect, useRef, useState } from 'react';
import {
  LayoutGrid, Users, MessageCircle, BookOpen, ArrowRight,
  Check, Bot, Code, Eye, Headphones, Shield, TrendingUp, Layers,
  Workflow, Sparkles, Clock, Target, Brain, Activity,
  ChevronRight, BarChart3, Cpu, FileText, GitPullRequest,
  Search, Zap,
} from 'lucide-react';

// ─── Hooks ───

function useReveal(threshold = 0.12) {
  const ref = useRef<HTMLDivElement>(null);
  const [cls, setCls] = useState('');
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    const obs = new IntersectionObserver(
      ([e]) => { if (e.isIntersecting) { setCls('visible'); obs.disconnect(); } },
      { threshold, rootMargin: '0px 0px -40px 0px' },
    );
    obs.observe(el);
    return () => obs.disconnect();
  }, [threshold]);
  return { ref, cls };
}

function useCounter(target: number, duration: number, active: boolean) {
  const [n, setN] = useState(0);
  useEffect(() => {
    if (!active) return;
    const t0 = performance.now();
    let raf: number;
    const tick = (now: number) => {
      const p = Math.min((now - t0) / duration, 1);
      setN(Math.round(target * (1 - Math.pow(1 - p, 3))));
      if (p < 1) raf = requestAnimationFrame(tick);
    };
    raf = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(raf);
  }, [target, duration, active]);
  return n;
}

// ─── Data ───

const AGENTS = [
  {
    key: 'epic_planner', role: 'Epic Planner', persona: 'Atlas',
    desc: 'Write "Add SSO support" as an epic. Atlas breaks it into 14 stories — with acceptance criteria, priority, estimates, and dependencies. Sprint-ready before standup.',
    caps: ['Spec writing', 'Story breakdown', 'Dependency mapping'],
  },
  {
    key: 'story_planner', role: 'Story Planner', persona: 'Scribe',
    desc: 'Takes a story and decomposes it into clear execution steps, sub-tasks, and implementation notes. Refines acceptance criteria so nothing is ambiguous.',
    caps: ['Task decomposition', 'Acceptance criteria', 'Execution plans'],
  },
  {
    key: 'code_builder', role: 'Code Builder', persona: 'Forge',
    desc: 'A bug gets assigned. Forge reads the codebase, writes a fix, opens a PR, and links it back. You review — you don\'t write.',
    caps: ['Code generation', 'PR creation', 'Bug fixes'],
  },
  {
    key: 'review_agent', role: 'Review Agent', persona: 'Lens',
    desc: 'Every PR gets reviewed before a human touches it. Lens checks for regressions, runs tests, flags issues. Obvious problems are caught before you look.',
    caps: ['Code review', 'Test validation', 'Regression checks'],
  },
  {
    key: 'support_agent', role: 'Support Agent', persona: 'Echo',
    desc: 'A ticket at 2 AM. Echo drafts a reply, matches to CRM, flags churn risk. By morning your team has context — not a backlog.',
    caps: ['Draft replies', 'Ticket routing', 'Knowledge search'],
  },
  {
    key: 'crm_operator', role: 'CRM Operator', persona: 'Operator',
    desc: 'Syncs emails, detects buying signals, creates deals automatically. Manages contacts across support and sales so nothing falls through.',
    caps: ['Signal detection', 'Deal creation', 'Contact sync'],
  },
];

const MODULES = [
  {
    icon: LayoutGrid, label: 'Project Management',
    title: 'Stories, sprints, epics, objectives — the whole system.',
    desc: 'Kanban boards. Sprint planning with velocity. Custom workflows per team. Estimate scales. Roadmap objectives. The full PM system your engineering team actually needs.',
    features: ['Kanban & Sprint boards', 'Velocity tracking', 'Custom workflows', 'Roadmap objectives', 'Estimate scales', 'Dependency mapping'],
    mockup: 'kanban',
  },
  {
    icon: Users, label: 'CRM',
    title: 'A sales pipeline that reads your email and moves itself.',
    desc: 'Syncs with Gmail, detects 7 types of buying signals via AI, and auto-creates deals when confidence is high. You close — it handles the pipeline.',
    features: ['Gmail auto-sync', 'AI signal detection', 'Auto-deal creation', 'Pipeline automation', 'Contact enrichment', 'Churn prediction'],
    mockup: 'pipeline',
  },
  {
    icon: MessageCircle, label: 'Support',
    title: 'Customer support where AI writes the first draft.',
    desc: 'Conversation threads, ticket routing, AI-drafted replies. Every ticket auto-matches to a CRM contact so you see the full history before typing a word.',
    features: ['AI draft replies', 'CRM auto-matching', 'Smart routing', 'Risk scoring', 'SLA tracking', 'Knowledge search'],
    mockup: 'inbox',
  },
  {
    icon: BookOpen, label: 'Knowledge Base',
    title: 'A knowledge base your AI agents actually read.',
    desc: 'Specs, decisions, runbooks — organized in spaces. When a planner agent breaks down an epic, it reads your docs. When support drafts a reply, it checks here first.',
    features: ['Spaces & collections', 'Linked to stories', 'Agent-readable', 'Version history', 'Search & embed', 'Access controls'],
    mockup: 'docs',
  },
];

const TIMELINE = [
  { time: '10:14', mod: 'Support', title: 'Customer email received', desc: 'AI reads the message, categorizes as billing bug, drafts a reply.' },
  { time: '10:14', mod: 'PM', title: 'Bug story auto-created', desc: 'Story added to backlog — severity high, linked to support conversation.' },
  { time: '10:15', mod: 'CRM', title: 'Churn risk elevated', desc: 'Third ticket in two weeks. Health score drops. Account owner notified.' },
  { time: '10:16', mod: 'Forge', title: 'Code Builder investigating', desc: 'Reads codebase, identifies billing calc error, opens fix PR.' },
  { time: '10:22', mod: 'Resolved', title: 'Customer notified', desc: 'Fix deployed. AI drafts follow-up. Resolution: 8 minutes, not 8 hours.' },
];

const STATS = [
  { value: 10, suffix: 'x', label: 'Faster execution', icon: Zap },
  { value: 85, suffix: '%', label: 'Less context switching', icon: Target },
  { value: 5, suffix: '', label: 'Tools replaced', icon: Layers },
  { value: 8, suffix: 'min', label: 'Avg. resolution', icon: Clock },
];

const COMPARE = [
  'PM + CRM + Support + Docs in one',
  'Six AI agents that execute real work',
  'Cross-module context flow',
  'Self-driving CRM pipeline',
  'AI-drafted support replies',
  'Knowledge base agents read',
  'Configurable autonomy levels',
  'Single subscription',
];

// ─── Agent Avatars (from app) ───

function AgentAvatar({ persona, className = '' }: { persona: string; className?: string }) {
  const svgs: Record<string, React.ReactNode> = {
    Atlas: (
      <svg viewBox="0 0 80 80" fill="none">
        <rect x="10" y="14" width="60" height="52" rx="14" fill="#D85A30" opacity=".12" />
        <rect x="14" y="18" width="52" height="44" rx="12" fill="#D85A30" />
        <rect x="22" y="28" width="14" height="10" rx="4" fill="#FAECE7" />
        <rect x="44" y="28" width="14" height="10" rx="4" fill="#FAECE7" />
        <circle cx="28" cy="33" r="3" fill="#4A1B0C" />
        <circle cx="50" cy="33" r="3" fill="#4A1B0C" />
        <rect x="30" y="44" width="20" height="4" rx="2" fill="#FAECE7" />
        <rect x="24" y="8" width="8" height="14" rx="4" fill="#D85A30" />
        <rect x="48" y="8" width="8" height="14" rx="4" fill="#D85A30" />
        <line x1="28" y1="8" x2="28" y2="2" stroke="#993C1D" strokeWidth="2" strokeLinecap="round" />
        <line x1="52" y1="8" x2="52" y2="2" stroke="#993C1D" strokeWidth="2" strokeLinecap="round" />
        <circle cx="28" cy="2" r="2" fill="#EF9F27" />
        <circle cx="52" cy="2" r="2" fill="#EF9F27" />
      </svg>
    ),
    Scribe: (
      <svg viewBox="0 0 80 80" fill="none">
        <rect x="10" y="16" width="60" height="48" rx="12" fill="#1D9E75" opacity=".12" />
        <rect x="14" y="20" width="52" height="40" rx="10" fill="#1D9E75" />
        <rect x="22" y="30" width="12" height="8" rx="3" fill="#E1F5EE" />
        <rect x="46" y="30" width="12" height="8" rx="3" fill="#E1F5EE" />
        <circle cx="28" cy="34" r="2.5" fill="#04342C" />
        <circle cx="52" cy="34" r="2.5" fill="#04342C" />
        <path d="M32 46Q40 52 48 46" stroke="#E1F5EE" strokeWidth="2.5" strokeLinecap="round" />
        <rect x="30" y="6" width="20" height="16" rx="6" fill="#1D9E75" />
        <rect x="34" y="2" width="12" height="8" rx="4" fill="#0F6E56" />
        <rect x="36" y="10" width="8" height="4" rx="2" fill="#5DCAA5" />
      </svg>
    ),
    Forge: (
      <svg viewBox="0 0 80 80" fill="none">
        <rect x="10" y="16" width="60" height="50" rx="10" fill="#534AB7" opacity=".12" />
        <rect x="14" y="20" width="52" height="42" rx="8" fill="#534AB7" />
        <rect x="20" y="28" width="16" height="12" rx="4" fill="#EEEDFE" />
        <rect x="44" y="28" width="16" height="12" rx="4" fill="#EEEDFE" />
        <rect x="24" y="32" width="4" height="4" rx="1" fill="#26215C" />
        <rect x="28" y="32" width="4" height="4" rx="1" fill="#7F77DD" />
        <rect x="48" y="32" width="4" height="4" rx="1" fill="#26215C" />
        <rect x="52" y="32" width="4" height="4" rx="1" fill="#7F77DD" />
        <rect x="28" y="48" width="24" height="6" rx="3" fill="#AFA9EC" />
        <rect x="30" y="49.5" width="4" height="3" rx="1" fill="#26215C" />
        <rect x="36" y="49.5" width="4" height="3" rx="1" fill="#26215C" />
        <rect x="42" y="49.5" width="4" height="3" rx="1" fill="#26215C" />
        <polygon points="40,4 48,16 32,16" fill="#534AB7" />
        <rect x="37" y="10" width="6" height="3" rx="1" fill="#EEEDFE" />
      </svg>
    ),
    Lens: (
      <svg viewBox="0 0 80 80" fill="none">
        <rect x="10" y="16" width="60" height="50" rx="14" fill="#BA7517" opacity=".12" />
        <rect x="14" y="20" width="52" height="42" rx="12" fill="#BA7517" />
        <circle cx="30" cy="36" r="9" fill="#FAEEDA" />
        <circle cx="50" cy="36" r="9" fill="#FAEEDA" />
        <circle cx="30" cy="36" r="5" fill="#412402" />
        <circle cx="50" cy="36" r="5" fill="#412402" />
        <circle cx="32" cy="34" r="2" fill="#FAEEDA" />
        <circle cx="52" cy="34" r="2" fill="#FAEEDA" />
        <rect x="34" y="50" width="12" height="4" rx="2" fill="#FAEEDA" />
        <path d="M10 30Q6 20 14 14L22 20" stroke="#BA7517" strokeWidth="3" strokeLinecap="round" />
        <path d="M70 30Q74 20 66 14L58 20" stroke="#BA7517" strokeWidth="3" strokeLinecap="round" />
      </svg>
    ),
    Echo: (
      <svg viewBox="0 0 80 80" fill="none">
        <circle cx="40" cy="42" r="28" fill="#D4537E" opacity=".12" />
        <circle cx="40" cy="42" r="24" fill="#D4537E" />
        <circle cx="32" cy="36" r="6" fill="#FBEAF0" />
        <circle cx="48" cy="36" r="6" fill="#FBEAF0" />
        <circle cx="32" cy="36" r="3" fill="#4B1528" />
        <circle cx="48" cy="36" r="3" fill="#4B1528" />
        <circle cx="33.5" cy="35" r="1" fill="#FBEAF0" />
        <circle cx="49.5" cy="35" r="1" fill="#FBEAF0" />
        <path d="M34 50Q40 56 46 50" stroke="#FBEAF0" strokeWidth="2.5" strokeLinecap="round" />
        <ellipse cx="40" cy="10" rx="10" ry="6" fill="#D4537E" />
        <circle cx="40" cy="6" r="3" fill="#ED93B1" />
      </svg>
    ),
    Operator: (
      <svg viewBox="0 0 80 80" fill="none">
        <rect x="12" y="16" width="56" height="50" rx="14" fill="#8B5CF6" opacity=".12" />
        <rect x="16" y="20" width="48" height="42" rx="12" fill="#8B5CF6" />
        <rect x="24" y="30" width="12" height="8" rx="3" fill="#EDE9FE" />
        <rect x="44" y="30" width="12" height="8" rx="3" fill="#EDE9FE" />
        <circle cx="30" cy="34" r="2.5" fill="#2E1065" />
        <circle cx="50" cy="34" r="2.5" fill="#2E1065" />
        <path d="M32 46Q40 52 48 46" stroke="#EDE9FE" strokeWidth="2.5" strokeLinecap="round" />
        <rect x="28" y="6" width="24" height="16" rx="6" fill="#8B5CF6" />
        <circle cx="40" cy="10" r="4" fill="#C4B5FD" />
        <rect x="36" y="16" width="8" height="4" rx="2" fill="#A78BFA" />
      </svg>
    ),
  };

  return (
    <span className={`inline-flex shrink-0 items-center justify-center overflow-hidden rounded-2xl border border-white/10 bg-white/5 shadow-sm ${className}`}>
      {svgs[persona] || <Bot className="h-1/2 w-1/2" />}
    </span>
  );
}

// ─── Grid wrapper ───

function GridSection({ children, className = '', dark = false }: { children: React.ReactNode; className?: string; dark?: boolean }) {
  return (
    <section className={`grid-section relative ${className}`}>
      <div className="grid-lines-inner" />
      <div className="relative z-10">{children}</div>
    </section>
  );
}

function GridSeparator() {
  return <div className="grid-separator mx-auto max-w-7xl" />;
}

// ─── Page ───

export default function Home() {
  useEffect(() => {
    const obs = new IntersectionObserver(
      (entries) => entries.forEach(e => { if (e.isIntersecting) { e.target.classList.add('visible'); obs.unobserve(e.target); } }),
      { threshold: 0.1, rootMargin: '0px 0px -30px 0px' },
    );
    document.querySelectorAll('.reveal, .reveal-scale, .reveal-left, .reveal-right, .reveal-blur').forEach(el => obs.observe(el));
    return () => obs.disconnect();
  }, []);

  return (
    <>
      <Hero />
      <GridSeparator />
      <MarqueeStrip />
      <GridSeparator />
      <Stats />
      <GridSeparator />
      <Modules />
      <AgentsSection />
      <GridSeparator />
      <Timeline />
      <GridSeparator />
      <Comparison />
      <CTA />
    </>
  );
}

// ═══════════════════════════════════════════
// HERO
// ═══════════════════════════════════════════

function Hero() {
  return (
    <GridSection className="pt-24 pb-20 lg:pt-32 lg:pb-28 overflow-hidden">
      <div className="hero-orb hero-orb-1" />
      <div className="hero-orb hero-orb-2" />

      <div className="mx-auto max-w-7xl px-6 lg:px-8">
        <div className="grid lg:grid-cols-[1.1fr_0.9fr] gap-12 items-center">
          <div>
            <div className="reveal inline-flex items-center gap-2 rounded-full border border-[var(--color-pop-medium)] bg-[var(--color-pop-light)] px-4 py-1.5">
              <Sparkles className="h-3.5 w-3.5 text-pop" />
              <span className="text-sm font-semibold text-pop">The stack replacement</span>
            </div>

            <h1 className="reveal mt-8 text-[clamp(2.25rem,4.5vw,3.75rem)] font-extrabold leading-[1.15] tracking-tight" style={{ transitionDelay: '100ms' }}>
              Your PM, CRM, support, and docs
              — replaced.{' '}
              <br className="hidden lg:block" />
              <span className="font-display italic font-normal pop-gradient">AI&nbsp;agents</span>{' '}
              handle the rest.
            </h1>

            <p className="reveal mt-7 max-w-xl text-[19px] leading-[1.65] text-muted-foreground" style={{ transitionDelay: '200ms' }}>
              Stop duct-taping five tools together. Helpin is one connected system
              for everything your team runs on. AI agents — planners, engineers,
              reviewers, support reps — execute the work automatically.
            </p>

            <div className="reveal mt-10 flex flex-wrap items-center gap-4" style={{ transitionDelay: '500ms' }}>
              <Link href="https://helpin.ai/login" className="btn-pop inline-flex items-center gap-2.5 rounded-xl px-7 py-3.5 text-[15px] font-semibold text-white">
                Start free <ArrowRight className="h-4 w-4" />
              </Link>
              <Link href="/features" className="btn-secondary inline-flex items-center gap-2 rounded-xl border border-border bg-background px-7 py-3.5 text-[15px] font-semibold text-foreground">
                See how it works
              </Link>
            </div>

            <div className="reveal mt-14 flex flex-wrap items-center gap-2" style={{ transitionDelay: '600ms' }}>
              <span className="text-[11px] font-bold tracking-[0.15em] uppercase text-foreground/20 mr-1">Replaces</span>
              {['PM tool', 'CRM', 'Support desk', 'Wiki', 'Integrations'].map(t => (
                <span key={t} className="rounded-md bg-foreground/[0.03] border border-border/50 px-2.5 py-1 text-[13px] text-muted-foreground/50 line-through decoration-[var(--color-pop)] decoration-[1.5px]">{t}</span>
              ))}
            </div>
          </div>

          <div className="reveal-scale hidden lg:block" style={{ transitionDelay: '300ms' }}>
            <ProductMockup />
          </div>
        </div>
      </div>
    </GridSection>
  );
}

// ─── Product Mockup ───

function ProductMockup() {
  const [tab, setTab] = useState(0);
  const tabs = ['Projects', 'CRM', 'Support', 'Docs'];

  useEffect(() => {
    const t = setInterval(() => setTab(i => (i + 1) % 4), 3000);
    return () => clearInterval(t);
  }, []);

  return (
    <div className="rounded-2xl border border-border bg-background shadow-2xl shadow-foreground/[0.05] overflow-hidden">
      <div className="flex items-center gap-2 border-b border-border/50 bg-muted/40 px-4 py-2.5">
        <div className="flex gap-1.5">
          <div className="h-2.5 w-2.5 rounded-full bg-zinc-300" />
          <div className="h-2.5 w-2.5 rounded-full bg-zinc-300" />
          <div className="h-2.5 w-2.5 rounded-full bg-zinc-300" />
        </div>
        <div className="mx-auto rounded-lg bg-background/80 border border-border/40 px-8 py-1 text-[11px] text-muted-foreground/60 font-medium">helpin.ai</div>
        <div className="w-14" />
      </div>
      <div className="flex min-h-[340px]">
        <div className="w-36 shrink-0 border-r border-border/30 bg-muted/20 p-3 flex flex-col">
          <div className="px-2 mb-5">
            <img src="https://assets.helpin.ai/logos/helpin-light-mode-logo.svg" alt="Helpin" className="h-4 opacity-60" />
          </div>
          <nav className="space-y-0.5">
            {tabs.map((t, i) => (
              <button key={t} onClick={() => setTab(i)} className={`w-full flex items-center gap-2 rounded-lg px-2.5 py-2 text-[11px] font-medium transition-all duration-300 ${tab === i ? 'bg-[var(--color-pop-light)] text-pop' : 'text-muted-foreground/60 hover:text-muted-foreground'}`}>
                <div className={`h-1.5 w-1.5 rounded-full transition-colors duration-300 ${tab === i ? 'bg-pop' : 'bg-foreground/10'}`} />
                {t}
              </button>
            ))}
          </nav>
          <div className="mt-auto"><div className="flex items-center gap-2 rounded-lg px-2.5 py-2 text-[11px] text-muted-foreground/40"><Bot className="h-3 w-3" /> Agents</div></div>
        </div>
        <div className="flex-1 p-4 relative overflow-hidden">
          {[<MockKanban key={0} />, <MockPipeline key={1} />, <MockInbox key={2} />, <MockDocs key={3} />].map((v, i) => (
            <div key={i} className={`absolute inset-0 p-4 transition-all duration-500 ease-[cubic-bezier(0.16,1,0.3,1)] ${tab === i ? 'opacity-100 translate-y-0' : 'opacity-0 translate-y-3 pointer-events-none'}`}>{v}</div>
          ))}
        </div>
      </div>
    </div>
  );
}

function MockKanban() {
  return (<div>
    <div className="text-[10px] font-bold mb-3 text-foreground/60">Sprint 14 — <span className="text-pop">Active</span></div>
    <div className="grid grid-cols-3 gap-2">
      {['Backlog', 'In Progress', 'Done'].map((col, ci) => (
        <div key={col}>
          <div className="text-[8px] font-bold uppercase tracking-wider text-muted-foreground/30 mb-1.5">{col}</div>
          {Array.from({ length: 2 + ci }).map((_, j) => (
            <div key={j} className="mock-element mb-2 rounded-lg border border-border/20 bg-background p-2.5 shadow-sm kanban-enter" style={{ animationDelay: `${(ci * 3 + j) * 80}ms` }}>
              <div className="flex items-center gap-1.5 mb-1.5"><div className="h-2 w-2 rounded-full bg-zinc-300" /><div className="h-[5px] rounded bg-foreground/8" style={{ width: `${55 + j * 15}%` }} /></div>
              <div className="h-[4px] rounded bg-foreground/[0.03]" style={{ width: `${65 + j * 10}%` }} />
            </div>
          ))}
        </div>
      ))}
    </div>
  </div>);
}

function MockPipeline() {
  return (<div>
    <div className="text-[10px] font-bold mb-3 text-foreground/60">Pipeline — <span className="text-pop">Q1 2026</span></div>
    <div className="space-y-3">
      {[{ l: 'Lead', v: '$89k', w: 100 }, { l: 'Qualified', v: '$64k', w: 72 }, { l: 'Proposal', v: '$42k', w: 48 }, { l: 'Closed Won', v: '$28k', w: 32 }].map(s => (
        <div key={s.l} className="mock-element">
          <div className="flex justify-between text-[10px] mb-1"><span className="font-medium text-foreground/50">{s.l}</span><span className="text-muted-foreground/40">{s.v}</span></div>
          <div className="h-3 rounded-full bg-muted/50 overflow-hidden"><div className="h-full rounded-full bg-[var(--color-pop)] opacity-30 progress-animated" style={{ '--target-width': `${s.w}%` } as React.CSSProperties} /></div>
        </div>
      ))}
    </div>
    <div className="mt-4 p-2.5 rounded-lg bg-[var(--color-pop-light)] border border-[var(--color-pop-medium)] flex items-center gap-2">
      <Activity className="h-3.5 w-3.5 text-pop" /><span className="text-[10px] text-pop font-medium">3 signals detected today</span>
    </div>
  </div>);
}

function MockInbox() {
  return (<div>
    <div className="text-[10px] font-bold mb-3 text-foreground/60">Inbox — <span className="text-pop">3 open</span></div>
    {[{ from: 'SK', subj: 'Billing issue — charged twice', tag: 'Urgent' }, { from: 'MR', subj: 'How to export reports?', tag: 'Question' }, { from: 'LW', subj: 'API rate limit hit', tag: 'Bug' }].map((t, i) => (
      <div key={i} className="mock-element flex items-center gap-2.5 rounded-lg border border-border/20 bg-background p-2.5 mb-2 shadow-sm kanban-enter" style={{ animationDelay: `${i * 100}ms` }}>
        <div className="h-7 w-7 rounded-full bg-gradient-to-br from-zinc-100 to-zinc-200 flex items-center justify-center text-[9px] font-bold text-zinc-600">{t.from}</div>
        <div className="flex-1 min-w-0"><div className="text-[11px] font-semibold truncate">{t.subj}</div></div>
        <span className="text-[8px] font-bold rounded px-1.5 py-0.5 bg-foreground/5 text-foreground/40">{t.tag}</span>
      </div>
    ))}
    <div className="mt-2 p-2.5 rounded-lg bg-[var(--color-pop-light)] border border-[var(--color-pop-medium)] flex items-center gap-2">
      <Bot className="h-3.5 w-3.5 text-pop" /><span className="text-[10px] text-pop font-medium">Echo drafted 2 replies</span>
      <span className="typing-cursor text-pop text-sm ml-auto">|</span>
    </div>
  </div>);
}

function MockDocs() {
  return (<div>
    <div className="text-[10px] font-bold mb-3 text-foreground/60">Knowledge Base — <span className="text-pop">47 docs</span></div>
    {[{ title: 'Authentication Spec', icon: '📄', space: 'Engineering' }, { title: 'API Rate Limiting', icon: '⚡', space: 'Infrastructure' }, { title: 'Deployment Runbook', icon: '🚀', space: 'DevOps' }, { title: 'Onboarding Guide', icon: '📘', space: 'Product' }].map((d, i) => (
      <div key={i} className="mock-element flex items-center gap-2.5 rounded-lg border border-border/20 bg-background p-2.5 mb-2 shadow-sm kanban-enter" style={{ animationDelay: `${i * 100}ms` }}>
        <span className="text-sm">{d.icon}</span>
        <div className="flex-1"><div className="text-[11px] font-semibold">{d.title}</div><div className="text-[9px] text-muted-foreground/40">{d.space}</div></div>
      </div>
    ))}
    <div className="mt-2 p-2.5 rounded-lg bg-[var(--color-pop-light)] border border-[var(--color-pop-medium)] flex items-center gap-2">
      <Brain className="h-3.5 w-3.5 text-pop" /><span className="text-[10px] text-pop font-medium">Atlas indexed all docs</span>
    </div>
  </div>);
}

// ═══════════════════════════════════════════
// MARQUEE
// ═══════════════════════════════════════════

function MarqueeStrip() {
  const items = ['SaaS Teams', 'Engineering Orgs', 'Product Teams', 'Sales Teams', 'Support Teams', 'Startups', 'Growth Companies'];
  return (
    <GridSection className="py-6 overflow-hidden">
      <div className="marquee-track">
        {[...items, ...items].map((item, i) => (
          <span key={i} className="flex items-center gap-6 px-6">
            <span className="text-[15px] font-semibold text-foreground/10 tracking-wide whitespace-nowrap">{item}</span>
            <span className="h-1 w-1 rounded-full bg-pop opacity-40" />
          </span>
        ))}
      </div>
    </GridSection>
  );
}

// ═══════════════════════════════════════════
// STATS
// ═══════════════════════════════════════════

function Stats() {
  const { ref, cls } = useReveal();
  return (
    <GridSection className="py-20 lg:py-24">
      <div ref={ref} className={`mx-auto max-w-7xl px-6 lg:px-8 stagger ${cls}`}>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          {STATS.map((s, i) => <StatCard key={s.label} stat={s} active={cls === 'visible'} i={i} />)}
        </div>
      </div>
    </GridSection>
  );
}

function StatCard({ stat, active, i }: { stat: typeof STATS[number]; active: boolean; i: number }) {
  const n = useCounter(stat.value, 1800, active);
  const Icon = stat.icon;
  return (
    <div className="reveal glass-card rounded-2xl p-7" style={{ transitionDelay: `${i * 100}ms` }}>
      <Icon className="h-5 w-5 text-pop mb-4" />
      <div className="text-4xl font-extrabold tracking-tight lg:text-5xl">
        {n}<span className="text-pop">{stat.suffix}</span>
      </div>
      <p className="mt-2 text-sm text-muted-foreground/70">{stat.label}</p>
      <div className="mt-4 h-1.5 w-full rounded-full bg-muted overflow-hidden">
        <div className={`stat-bar h-full rounded-full ${active ? 'active' : ''}`} style={{ animationDelay: `${i * 150 + 400}ms` }} />
      </div>
    </div>
  );
}

// ═══════════════════════════════════════════
// MODULES
// ═══════════════════════════════════════════

function Modules() {
  return (
    <GridSection className="py-24 lg:py-32">
      <div className="mx-auto max-w-7xl px-6 lg:px-8">
        <div className="reveal-blur max-w-2xl mb-20">
          <span className="inline-flex items-center gap-2 rounded-full border border-[var(--color-pop-medium)] bg-[var(--color-pop-light)] px-4 py-1.5 text-sm font-semibold text-pop">
            <Layers className="h-3.5 w-3.5" /> Platform
          </span>
          <h2 className="mt-6 text-[clamp(2rem,4vw,3.25rem)] font-extrabold tracking-tight leading-[1.1]">
            Four tools replaced.<br /><span className="text-muted-foreground/40">Connected by default.</span>
          </h2>
        </div>
        <div className="space-y-32">
          {MODULES.map((mod, i) => <ModuleBlock key={mod.label} mod={mod} index={i} />)}
        </div>
      </div>
    </GridSection>
  );
}

function ModuleBlock({ mod, index }: { mod: typeof MODULES[number]; index: number }) {
  const reversed = index % 2 === 1;
  const Icon = mod.icon;
  return (
    <div className="grid gap-12 lg:grid-cols-2 lg:gap-20 items-center">
      <div className={reversed ? 'lg:order-2' : ''}>
        <div className={reversed ? 'reveal-right' : 'reveal-left'}>
          <span className="inline-flex items-center gap-2 rounded-full border border-foreground/10 bg-foreground/[0.03] px-3.5 py-1.5 text-xs font-bold text-foreground/60">
            <Icon className="h-3.5 w-3.5" /> {mod.label}
          </span>
        </div>
        <h3 className={`${reversed ? 'reveal-right' : 'reveal-left'} mt-5 text-[clamp(1.5rem,3vw,2rem)] font-bold leading-snug tracking-tight`} style={{ transitionDelay: '80ms' }}>{mod.title}</h3>
        <p className={`${reversed ? 'reveal-right' : 'reveal-left'} mt-4 text-base leading-relaxed text-muted-foreground lg:text-[17px]`} style={{ transitionDelay: '160ms' }}>{mod.desc}</p>
        <div className={`${reversed ? 'reveal-right' : 'reveal-left'} mt-7 grid grid-cols-2 gap-x-4 gap-y-2.5`} style={{ transitionDelay: '240ms' }}>
          {mod.features.map(f => (
            <div key={f} className="flex items-center gap-2 text-[14px] text-muted-foreground/80"><Check className="h-4 w-4 shrink-0 text-pop" /> {f}</div>
          ))}
        </div>
        <div className={`${reversed ? 'reveal-right' : 'reveal-left'} mt-8`} style={{ transitionDelay: '320ms' }}>
          <Link href="/features" className="group inline-flex items-center gap-1.5 text-[15px] font-semibold text-pop transition-colors hover:opacity-80">
            Learn more <ChevronRight className="h-4 w-4 transition-transform group-hover:translate-x-0.5" />
          </Link>
        </div>
      </div>
      <div className={`${reversed ? 'lg:order-1 reveal-left' : 'reveal-right'}`} style={{ transitionDelay: '200ms' }}>
        <div className="rounded-2xl bg-gradient-to-br from-zinc-500/10 via-stone-500/5 to-transparent border border-zinc-200/40 p-1">
          <div className="rounded-xl bg-background/80 backdrop-blur-sm p-6 min-h-[320px] flex flex-col justify-center">
            <ModuleMockUI type={mod.mockup} />
          </div>
        </div>
      </div>
    </div>
  );
}

function ModuleMockUI({ type }: { type: string }) {
  if (type === 'kanban') return <MockKanban />;
  if (type === 'pipeline') return <MockPipeline />;
  if (type === 'inbox') return <MockInbox />;
  return <MockDocs />;
}

// ═══════════════════════════════════════════
// AGENTS — 6 real agents, dark section with grid
// ═══════════════════════════════════════════

function AgentsSection() {
  return (
    <section className="py-28 lg:py-36 bg-foreground text-white overflow-hidden relative">
      {/* Faint grid on dark */}
      <div className="absolute inset-0 pointer-events-none" style={{ backgroundImage: 'linear-gradient(rgba(255,255,255,0.03) 1px, transparent 1px), linear-gradient(90deg, rgba(255,255,255,0.03) 1px, transparent 1px)', backgroundSize: '80px 80px' }} />

      <div className="relative mx-auto max-w-7xl px-6 lg:px-8">
        <div className="reveal-blur max-w-2xl">
          <span className="inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/[0.04] px-4 py-1.5 text-sm font-semibold text-pop">
            <Cpu className="h-3.5 w-3.5" /> 6 AI Agents
          </span>
          <h2 className="mt-6 text-[clamp(2rem,4vw,3.25rem)] font-extrabold tracking-tight leading-[1.1]">
            Not assistants.{' '}
            <span className="font-display italic font-normal text-pop">Workers.</span>
          </h2>
          <p className="mt-5 text-lg leading-relaxed text-white/40">
            Six specialized agents with defined roles, codebase access, and configurable autonomy. Assign work like you would a teammate.
          </p>
        </div>

        <div className="mt-16 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {AGENTS.map((agent, i) => (
            <div key={agent.key} className="reveal agent-card rounded-2xl border border-white/[0.06] bg-white/[0.02] p-6" style={{ transitionDelay: `${i * 80}ms` }}>
              <div className="flex items-center gap-3 mb-4">
                <AgentAvatar persona={agent.persona} className="h-11 w-11" />
                <div>
                  <h3 className="text-[15px] font-bold">{agent.persona}</h3>
                  <span className="text-[11px] text-white/30">{agent.role}</span>
                </div>
              </div>
              <p className="text-[14px] leading-relaxed text-white/35">{agent.desc}</p>
              <div className="mt-5 flex flex-wrap gap-1.5">
                {agent.caps.map(c => (
                  <span key={c} className="rounded-md border border-white/8 px-2.5 py-1 text-[11px] font-medium text-white/25">{c}</span>
                ))}
              </div>
            </div>
          ))}
        </div>

        <div className="reveal mt-12 rounded-2xl border border-white/8 bg-white/[0.02] px-6 py-5 lg:px-8" style={{ transitionDelay: '500ms' }}>
          <div className="flex flex-col sm:flex-row sm:items-center gap-4 sm:gap-8 text-sm text-white/30">
            {[
              { icon: Shield, label: 'Trigger modes', desc: 'manual · on assignment · on event' },
              { icon: TrendingUp, label: 'Autonomy levels', desc: 'require review · auto-execute' },
              { icon: Layers, label: 'Tool access', desc: 'configurable per agent' },
            ].map(({ icon: I, label, desc }) => (
              <div key={label} className="flex items-center gap-2.5">
                <I className="h-4 w-4 text-pop/50 shrink-0" />
                <span><strong className="text-white/50">{label}</strong> — {desc}</span>
              </div>
            ))}
          </div>
        </div>
      </div>
    </section>
  );
}

// ═══════════════════════════════════════════
// TIMELINE
// ═══════════════════════════════════════════

function Timeline() {
  return (
    <GridSection className="py-28 lg:py-36">
      <div className="mx-auto max-w-7xl px-6 lg:px-8">
        <div className="reveal-blur max-w-2xl mb-16">
          <span className="inline-flex items-center gap-2 rounded-full border border-[var(--color-pop-medium)] bg-[var(--color-pop-light)] px-4 py-1.5 text-sm font-semibold text-pop">
            <Workflow className="h-3.5 w-3.5" /> Connected Workflows
          </span>
          <h2 className="mt-6 text-[clamp(2rem,4vw,3.25rem)] font-extrabold tracking-tight leading-[1.1]">
            What this looks like<br /><span className="text-muted-foreground/40">in practice.</span>
          </h2>
          <p className="mt-5 text-lg leading-relaxed text-muted-foreground">
            A customer emails about a billing bug. Here&apos;s what happens across four modules — zero human handoffs.
          </p>
        </div>

        <div className="max-w-3xl mx-auto">
          {TIMELINE.map((ev, i) => (
            <div key={i} className="reveal relative grid grid-cols-[70px_1fr] gap-5 pb-8 lg:grid-cols-[85px_1fr] lg:gap-7" style={{ transitionDelay: `${i * 100}ms` }}>
              {i < TIMELINE.length - 1 && <div className="absolute left-[34px] top-11 bottom-0 w-[2px] timeline-connector lg:left-[41px]" />}
              <div className="pt-1.5 text-right relative">
                <div className="timeline-dot absolute right-[-17px] top-2.5 h-3 w-3 rounded-full bg-pop ring-4 ring-background lg:right-[-21px]" />
                <span className="text-sm font-bold tabular-nums text-foreground/25">{ev.time}</span>
              </div>
              <div className="timeline-card rounded-xl border border-border/40 bg-background p-5 shadow-sm">
                <div className="flex items-center gap-2.5 mb-2">
                  <span className="rounded-md px-2.5 py-1 text-[11px] font-bold bg-foreground/5 text-foreground/60">{ev.mod}</span>
                  <span className="text-[15px] font-semibold">{ev.title}</span>
                </div>
                <p className="text-[14px] leading-relaxed text-muted-foreground">{ev.desc}</p>
              </div>
            </div>
          ))}
          <div className="reveal mt-8 text-center rounded-2xl bg-[var(--color-pop-light)] border border-[var(--color-pop-medium)] p-7" style={{ transitionDelay: '600ms' }}>
            <p className="text-xl font-extrabold tracking-tight">8 minutes. Four modules. Zero tab-switching.</p>
            <p className="mt-2 text-sm text-muted-foreground">No human copied data between tools. The system handled it.</p>
          </div>
        </div>
      </div>
    </GridSection>
  );
}

// ═══════════════════════════════════════════
// COMPARISON
// ═══════════════════════════════════════════

function Comparison() {
  return (
    <GridSection className="py-24 lg:py-32 bg-muted/30">
      <div className="mx-auto max-w-4xl px-6 lg:px-8">
        <div className="reveal-blur text-center mb-14">
          <span className="inline-flex items-center gap-2 rounded-full border border-[var(--color-pop-medium)] bg-[var(--color-pop-light)] px-4 py-1.5 text-sm font-semibold text-pop">
            <BarChart3 className="h-3.5 w-3.5" /> Comparison
          </span>
          <h2 className="mt-6 text-[clamp(2rem,4vw,3rem)] font-extrabold tracking-tight">It&apos;s not even competition.</h2>
        </div>
        <div className="reveal-scale overflow-hidden rounded-2xl border border-border/50 bg-background shadow-lg shadow-foreground/[0.02]" style={{ transitionDelay: '150ms' }}>
          <div className="grid grid-cols-[1fr_90px_90px] items-center border-b border-border/40 px-6 py-4 bg-muted/20">
            <span className="text-xs font-bold uppercase tracking-wider text-foreground/30">Feature</span>
            <span className="text-center"><img src="https://assets.helpin.ai/logos/helpin-light-mode-logo.svg" alt="Helpin" className="h-4 mx-auto" /></span>
            <span className="text-center text-[10px] font-bold uppercase tracking-wider text-foreground/20">5 tools</span>
          </div>
          {COMPARE.map((f, i) => (
            <div key={f} className={`comparison-row grid grid-cols-[1fr_90px_90px] items-center px-6 py-3.5 ${i < COMPARE.length - 1 ? 'border-b border-border/15' : ''}`}>
              <span className="text-[14px] text-foreground/65">{f}</span>
              <div className="flex justify-center"><div className="h-6 w-6 rounded-full bg-[var(--color-pop-light)] flex items-center justify-center"><Check className="h-3.5 w-3.5 text-pop" /></div></div>
              <div className="flex justify-center"><div className="h-[2px] w-4 rounded bg-foreground/10" /></div>
            </div>
          ))}
        </div>
      </div>
    </GridSection>
  );
}

// ═══════════════════════════════════════════
// CTA
// ═══════════════════════════════════════════

function CTA() {
  return (
    <GridSection className="relative py-28 lg:py-36 overflow-hidden">
      <div className="cta-glow absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[500px] h-[500px] rounded-full bg-[var(--color-pop)] opacity-[0.03] blur-[100px]" />
      <div className="reveal-scale relative mx-auto max-w-3xl px-6 text-center lg:px-8">
        <h2 className="text-[clamp(2rem,5vw,3.75rem)] font-extrabold tracking-tight leading-[1.1]">
          Five tools. Five bills.<br /><span className="text-muted-foreground/35">Five places where context dies.</span>
        </h2>
        <p className="mx-auto mt-6 max-w-md text-lg leading-relaxed text-muted-foreground">
          Or one platform where everything&apos;s connected and AI agents handle the execution.
        </p>
        <div className="mt-10 flex flex-wrap items-center justify-center gap-4">
          <Link href="https://helpin.ai/login" className="btn-pop inline-flex items-center gap-2.5 rounded-xl px-8 py-4 text-[15px] font-semibold text-white">
            Get started free <ArrowRight className="h-4 w-4" />
          </Link>
          <Link href="/contact" className="btn-secondary inline-flex items-center rounded-xl border border-border bg-background px-8 py-4 text-[15px] font-semibold text-foreground">
            Talk to us
          </Link>
        </div>
        <p className="mt-6 text-[13px] text-muted-foreground/40">Free forever for small teams. No credit card required.</p>
      </div>
    </GridSection>
  );
}
