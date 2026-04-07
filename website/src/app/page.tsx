'use client';

import Link from 'next/link';
import { useEffect, useRef, useState } from 'react';
import { ArrowRight, User, Layers, BookOpen, MessageCircle, Users, Zap, FileText, Search, AlertCircle, RefreshCw, UserPlus, Code2, CheckCircle, Rocket, Globe, Clock, Send, Target, TrendingUp, BarChart3, PenTool, Calendar, Shield, Share2 } from 'lucide-react';

// ─── Early Access Form ───

function EarlyAccessForm({ dark = false, id = 'early-access', bg }: { dark?: boolean; id?: string; bg?: string }) {
  const [email, setEmail] = useState('');
  const [status, setStatus] = useState<'idle' | 'submitting' | 'success' | 'error'>('idle');

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!email || !email.includes('@')) return;
    setStatus('submitting');

    try {
      const w = window as unknown as {
        usermaven?: (cmd: string, ...args: unknown[]) => void;
        _cio?: { identify: (obj: Record<string, unknown>) => void; track: (event: string, obj?: Record<string, unknown>) => void };
      };
      // Usermaven
      if (w.usermaven) {
        w.usermaven('lead', { email });
        w.usermaven('track', 'early_access_signup', { form_id: id, email });
      }
      // Customer.io
      if (w._cio) {
        w._cio.identify({ id: email, email, created_at: Math.floor(Date.now() / 1000) });
        w._cio.track('early_access_signup', { form_id: id });
      }
      setStatus('success');
      setEmail('');
    } catch {
      setStatus('error');
    }
  };

  if (status === 'success') {
    return (
      <div className={`text-[15px] font-medium ${dark ? 'text-white/70' : 'text-foreground'}`}>
        You're on the list. We'll be in touch soon.
      </div>
    );
  }

  return (
    <div className="email-glow-wrapper w-full max-w-lg">
      <div className="email-glow-wrapper-glow" />
      <form onSubmit={handleSubmit} className="relative z-10 flex flex-col sm:flex-row gap-3 w-full rounded-[14px] p-2" style={{ background: bg || (dark ? 'var(--color-foreground)' : 'var(--color-background)'), border: '1px solid oklch(0.12 0.02 55 / 0.08)' }}>
        <input
          type="email"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          placeholder="Enter your work email"
          required
          className="flex-1 px-4 py-3 rounded-lg text-[15px] outline-none"
          style={{
            color: dark ? 'white' : 'var(--color-foreground)',
            background: dark ? 'oklch(1 0 0 / 0.05)' : 'white',
            border: dark ? '1px solid oklch(1 0 0 / 0.08)' : '1px solid oklch(0.12 0.02 55 / 0.12)',
          }}
        />
        <button
          type="submit"
          disabled={status === 'submitting'}
          className="btn-primary whitespace-nowrap justify-center w-full sm:w-auto"
          style={dark ? { background: 'white', color: 'var(--color-foreground)' } : {}}
        >
          {status === 'submitting' ? 'Submitting...' : 'Get early access'}
          {status === 'idle' && <ArrowRight className="h-4 w-4" />}
        </button>
      </form>
    </div>
  );
}

// ─── Reveal hook ───

function useReveal(threshold = 0.1) {
  const ref = useRef<HTMLDivElement>(null);
  const [cls, setCls] = useState('');
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    const obs = new IntersectionObserver(
      ([e]) => { if (e.isIntersecting) { setCls('visible'); obs.disconnect(); } },
      { threshold, rootMargin: '0px 0px -32px 0px' },
    );
    obs.observe(el);
    return () => obs.disconnect();
  }, [threshold]);
  return { ref, cls };
}

function Reveal({ children, className = '' }: { children: React.ReactNode; className?: string }) {
  const { ref, cls } = useReveal();
  return <div ref={ref} className={`reveal ${cls} ${className}`}>{children}</div>;
}

function RevealLeft({ children, className = '' }: { children: React.ReactNode; className?: string }) {
  const { ref, cls } = useReveal();
  return <div ref={ref} className={`reveal-left ${cls} ${className}`}>{children}</div>;
}

function RevealRight({ children, className = '' }: { children: React.ReactNode; className?: string }) {
  const { ref, cls } = useReveal();
  return <div ref={ref} className={`reveal-right ${cls} ${className}`}>{children}</div>;
}

// ─── Data ───

const COMPANIES = [
  { name: 'ContentStudio', domain: 'contentstudio.io' },
  { name: 'Replug',        domain: 'replug.io' },
  { name: 'Usermaven',     domain: 'usermaven.com' },
  { name: 'ContentPen',    domain: 'contentpen.ai' },
  { name: 'Hyperengage',   domain: 'hyperengage.io' },
];

const OLD_STACK = [
  { tool: 'Linear',   role: 'Work is tracked here',          domain: 'linear.app' },
  { tool: 'Notion',   role: 'Knowledge is buried here',      domain: 'notion.so' },
  { tool: 'Intercom', role: 'Support starts here',           domain: 'intercom.com' },
  { tool: 'HubSpot',  role: 'Customer context sits here',    domain: 'hubspot.com' },
  { tool: 'Slack',    role: 'Decisions disappear here',      domain: 'slack.com' },
  { tool: 'ChatGPT',  role: 'AI works here, alone',          domain: 'openai.com' },
];

const TEAM_AGENTS = [
  {
    team: 'Engineering & Product',
    agents: [
      { name: 'Epic Planner',              Icon: Layers,       color: 'oklch(0.52 0.16 250)', bg: 'oklch(0.52 0.16 250 / 0.08)', desc: 'Breaks strategy into epics, sprints, and tasks' },
      { name: 'Code Builder',              Icon: Code2,        color: 'oklch(0.50 0.14 200)', bg: 'oklch(0.50 0.14 200 / 0.08)', desc: 'Writes and ships code with full project context' },
      { name: 'Review Agent',              Icon: CheckCircle,  color: 'oklch(0.55 0.16 160)', bg: 'oklch(0.55 0.16 160 / 0.08)', desc: 'Validates changes and runs checks before merge' },
      { name: 'Bug Triage Agent',           Icon: AlertCircle,  color: 'oklch(0.55 0.18 15)',  bg: 'oklch(0.55 0.18 15 / 0.08)',  desc: 'Classifies bugs, assigns priority, links related issues' },
      { name: 'Release Agent',             Icon: Rocket,       color: 'oklch(0.55 0.15 280)', bg: 'oklch(0.55 0.15 280 / 0.08)', desc: 'Manages release notes, coordinates deploys, notifies stakeholders' },
      { name: 'Dependency Agent',           Icon: Shield,       color: 'oklch(0.50 0.12 220)', bg: 'oklch(0.50 0.12 220 / 0.08)', desc: 'Monitors outdated packages and security vulnerabilities' },
      { name: 'Incident Agent',            Icon: AlertCircle,  color: 'oklch(0.55 0.18 25)',  bg: 'oklch(0.55 0.18 25 / 0.08)',  desc: 'Detects production issues, pages the right people, creates postmortems' },
      { name: 'Feedback Synthesis Agent',   Icon: MessageCircle,color: 'oklch(0.58 0.15 55)',  bg: 'oklch(0.58 0.15 55 / 0.08)',  desc: 'Aggregates user feedback into themes from support, sales, and surveys' },
      { name: 'Roadmap Agent',             Icon: Target,       color: 'oklch(0.52 0.14 28)',  bg: 'oklch(0.52 0.14 28 / 0.08)',  desc: 'Connects customer requests to roadmap items, tracks demand' },
      { name: 'Competitor Agent',          Icon: Search,       color: 'oklch(0.55 0.18 310)', bg: 'oklch(0.55 0.18 310 / 0.08)', desc: 'Monitors competitor launches, pricing, and feature updates' },
      { name: 'Research & Planning Agent',  Icon: BookOpen,     color: 'oklch(0.55 0.14 160)', bg: 'oklch(0.55 0.14 160 / 0.08)', desc: 'Investigates approaches and evaluates tradeoffs' },
      { name: 'Docs Agent',               Icon: FileText,     color: 'oklch(0.50 0.14 200)', bg: 'oklch(0.50 0.14 200 / 0.08)', desc: 'Keeps technical documentation in sync with changes' },
    ],
  },
  {
    team: 'Support',
    agents: [
      { name: 'Support Agent',     Icon: MessageCircle, color: 'oklch(0.58 0.15 55)',  bg: 'oklch(0.58 0.15 55 / 0.08)',  desc: 'Triages, drafts replies, escalates with full context' },
      { name: 'Escalation Agent',  Icon: AlertCircle,   color: 'oklch(0.55 0.18 15)',  bg: 'oklch(0.55 0.18 15 / 0.08)',  desc: 'Catches unresolved issues before they become fires' },
      { name: 'SLA Agent',         Icon: Clock,         color: 'oklch(0.52 0.16 250)', bg: 'oklch(0.52 0.16 250 / 0.08)', desc: 'Tracks response time commitments, warns before breaches' },
      { name: 'Feedback Agent',    Icon: MessageCircle, color: 'oklch(0.55 0.15 130)', bg: 'oklch(0.55 0.15 130 / 0.08)', desc: 'Extracts product insights from support conversations' },
      { name: 'Translation Agent', Icon: Globe,         color: 'oklch(0.55 0.14 200)', bg: 'oklch(0.55 0.14 200 / 0.08)', desc: 'Handles multilingual support, translates tickets and articles' },
      { name: 'Knowledge Agent',   Icon: Search,        color: 'oklch(0.55 0.18 310)', bg: 'oklch(0.55 0.18 310 / 0.08)', desc: 'Surfaces answers instantly from all company knowledge' },
      { name: 'Docs Agent',        Icon: FileText,      color: 'oklch(0.50 0.14 200)', bg: 'oklch(0.50 0.14 200 / 0.08)', desc: 'Maintains help center and support articles' },
      { name: 'Onboarding Agent',  Icon: UserPlus,      color: 'oklch(0.55 0.15 130)', bg: 'oklch(0.55 0.15 130 / 0.08)', desc: 'Guides new users through activation' },
    ],
  },
  {
    team: 'Sales & CRM',
    agents: [
      { name: 'CRM Operator',              Icon: Users,        color: 'oklch(0.52 0.14 28)',  bg: 'oklch(0.52 0.14 28 / 0.08)',  desc: 'Keeps deals moving with full customer context' },
      { name: 'Outreach Agent',            Icon: Send,         color: 'oklch(0.52 0.16 250)', bg: 'oklch(0.52 0.16 250 / 0.08)', desc: 'Drafts personalized follow-ups from deal activity' },
      { name: 'Lead Scoring Agent',         Icon: Target,       color: 'oklch(0.55 0.18 15)',  bg: 'oklch(0.55 0.18 15 / 0.08)',  desc: 'Qualifies inbound leads based on behavior and fit' },
      { name: 'Renewal Agent',             Icon: RefreshCw,    color: 'oklch(0.55 0.16 160)', bg: 'oklch(0.55 0.16 160 / 0.08)', desc: 'Tracks contract timelines, flags churn risk' },
      { name: 'Proposal Agent',            Icon: FileText,     color: 'oklch(0.58 0.15 55)',  bg: 'oklch(0.58 0.15 55 / 0.08)',  desc: 'Generates proposals and quotes from deal context' },
      { name: 'Forecast Agent',            Icon: TrendingUp,   color: 'oklch(0.55 0.15 280)', bg: 'oklch(0.55 0.15 280 / 0.08)', desc: 'Projects revenue based on pipeline, churn, and trends' },
      { name: 'Health Score Agent',         Icon: BarChart3,    color: 'oklch(0.55 0.14 160)', bg: 'oklch(0.55 0.14 160 / 0.08)', desc: 'Monitors usage patterns, flags at-risk accounts' },
      { name: 'Expansion Agent',           Icon: TrendingUp,   color: 'oklch(0.55 0.18 310)', bg: 'oklch(0.55 0.18 310 / 0.08)', desc: 'Identifies upsell opportunities from usage data' },
      { name: 'Research & Planning Agent',  Icon: BookOpen,     color: 'oklch(0.55 0.14 160)', bg: 'oklch(0.55 0.14 160 / 0.08)', desc: 'Researches prospects, prepares for calls' },
      { name: 'Onboarding Agent',          Icon: UserPlus,     color: 'oklch(0.55 0.15 130)', bg: 'oklch(0.55 0.15 130 / 0.08)', desc: 'Runs post-sale customer activation' },
      { name: 'Reporting Agent',           Icon: BarChart3,    color: 'oklch(0.50 0.14 200)', bg: 'oklch(0.50 0.14 200 / 0.08)', desc: 'Generates pipeline and forecast reports' },
    ],
  },
  {
    team: 'Marketing',
    agents: [
      { name: 'Content Agent',             Icon: PenTool,      color: 'oklch(0.55 0.18 310)', bg: 'oklch(0.55 0.18 310 / 0.08)', desc: 'Drafts posts, changelogs, and announcements from product activity' },
      { name: 'SEO Agent',                Icon: Search,       color: 'oklch(0.55 0.16 160)', bg: 'oklch(0.55 0.16 160 / 0.08)', desc: 'Monitors rankings, suggests optimizations' },
      { name: 'Campaign Agent',            Icon: Rocket,       color: 'oklch(0.52 0.16 250)', bg: 'oklch(0.52 0.16 250 / 0.08)', desc: 'Plans and coordinates multi-channel campaigns' },
      { name: 'Social Agent',             Icon: Share2,       color: 'oklch(0.58 0.15 55)',  bg: 'oklch(0.58 0.15 55 / 0.08)',  desc: 'Drafts and schedules social posts, monitors engagement' },
      { name: 'Analytics Agent',           Icon: BarChart3,    color: 'oklch(0.55 0.15 280)', bg: 'oklch(0.55 0.15 280 / 0.08)', desc: 'Tracks campaign performance, surfaces what\'s working' },
      { name: 'Brand Agent',              Icon: Shield,       color: 'oklch(0.52 0.14 28)',  bg: 'oklch(0.52 0.14 28 / 0.08)',  desc: 'Enforces voice, tone, and style guidelines across content' },
      { name: 'Research & Planning Agent',  Icon: BookOpen,     color: 'oklch(0.55 0.14 160)', bg: 'oklch(0.55 0.14 160 / 0.08)', desc: 'Analyzes market, competitors, and positioning' },
    ],
  },
  {
    team: 'Operations',
    agents: [
      { name: 'Task Planner',      Icon: Layers,     color: 'oklch(0.52 0.16 250)', bg: 'oklch(0.52 0.16 250 / 0.08)', desc: 'Decomposes and refines work across teams' },
      { name: 'Meeting Agent',     Icon: Calendar,   color: 'oklch(0.55 0.15 280)', bg: 'oklch(0.55 0.15 280 / 0.08)', desc: 'Prepares agendas, captures action items, follows up' },
      { name: 'Hiring Agent',      Icon: UserPlus,   color: 'oklch(0.55 0.15 130)', bg: 'oklch(0.55 0.15 130 / 0.08)', desc: 'Screens resumes, schedules interviews, coordinates hiring' },
      { name: 'Compliance Agent',  Icon: Shield,     color: 'oklch(0.55 0.18 15)',  bg: 'oklch(0.55 0.18 15 / 0.08)',  desc: 'Monitors policy adherence, flags violations, maintains audit trails' },
      { name: 'Reporting Agent',   Icon: BarChart3,  color: 'oklch(0.50 0.14 200)', bg: 'oklch(0.50 0.14 200 / 0.08)', desc: 'Generates standups, summaries, and performance reports' },
      { name: 'Knowledge Agent',   Icon: Search,     color: 'oklch(0.55 0.18 310)', bg: 'oklch(0.55 0.18 310 / 0.08)', desc: 'Company-wide context search' },
      { name: 'Docs Agent',        Icon: FileText,   color: 'oklch(0.50 0.14 200)', bg: 'oklch(0.50 0.14 200 / 0.08)', desc: 'Maintains internal documentation' },
    ],
  },
];

const PROBLEMS_SOLUTIONS = [
  {
    problem: 'Product planning is still manual',
    problemDesc: 'Your team copies requirements into ChatGPT, pastes the output into Jira, then manually breaks it down into tasks. Every sprint.',
    solutions: [
      'Planning Agent turns strategy into epics with full context',
      'Task Agent decomposes epics into stories and tasks',
      'Code Agent starts building, Review Agent validates',
      'Human approves, it ships — changelog generated automatically',
    ],
  },
  {
    problem: 'Development is slow despite having AI',
    problemDesc: 'Your developers use Copilot and ChatGPT, but the workflow is still manual — read the ticket, understand context, write code, create PR, wait for review, deploy. Every step is a context switch.',
    solutions: [
      'Code Agent picks up the task with full project context — no briefing needed',
      'Writes the implementation, opens a PR automatically',
      'Review Agent validates the code, runs checks, flags issues',
      'Human approves with one click — deployed to production',
    ],
  },
  {
    problem: 'Support is reactive, not intelligent',
    problemDesc: 'Every ticket starts from scratch. Agents don\'t know what\'s already documented. Bugs get reported but never routed to engineering.',
    solutions: [
      'Incoming requests triaged by AI, relevant docs surfaced instantly',
      'Unresolved issues become tickets — AI agent picks them up and fixes the bug',
      'Human reviews, approves, and it goes live. Customer notified automatically',
      'Docs Agent updates help articles after every fix and release',
    ],
  },
  {
    problem: 'Documentation is always outdated',
    problemDesc: 'Nobody maintains docs. They go stale after every release. Support answers questions that should be in the knowledge base. New hires learn from outdated information.',
    solutions: [
      'Docs Agent auto-updates documentation after every release',
      'Support conversations feed back into the knowledge base',
      'Missing docs flagged and created automatically from product changes',
      'Internal and external knowledge stays accurate without anyone maintaining it',
    ],
  },
  {
    problem: 'Sales prep is scattered and manual',
    problemDesc: 'Reps walk into meetings unprepared. Notes get lost. Follow-ups depend on memory. Deals go quiet because nobody nudged.',
    solutions: [
      'AI prepares a brief before every meeting — deal context, history, signals',
      'Meeting notes captured automatically, action items created',
      'Follow-up nudges sent when deals go quiet',
      'Pipeline reports generated without anyone asking',
    ],
  },
  {
    problem: 'Marketing depends on developers',
    problemDesc: 'Every landing page change, headline test, or new page requires a developer. Marketing moves at engineering\'s pace.',
    solutions: [
      'Marketing team edits pages directly — no developer needed',
      'A/B test headlines, CTAs, and layouts autonomously',
      'Create and publish new pages with AI assistance',
      'Review, approve, and go live — all within Helpin',
    ],
  },
];

const COMPOUNDS = [
  {
    title: 'Knowledge compounds',
    desc: 'Every decision, doc, and resolved issue becomes shared context. Nothing gets buried or forgotten.',
  },
  {
    title: 'Agents get sharper',
    desc: 'Helpin agents operate with full company context. The more they handle, the more accurately they act.',
  },
  {
    title: 'Teams stay aligned',
    desc: 'When the system is shared, silos disappear. Every team operates from the same source of truth.',
  },
  {
    title: 'Execution accelerates',
    desc: 'No more manual handoffs. Agents carry context from step to step without you bridging the gap.',
  },
];

// ─── Visual Comparison Section ───

const BEFORE_TOOLS = [
  { domain: 'linear.app' }, { domain: 'notion.so' }, { domain: 'intercom.com' },
  { domain: 'hubspot.com' }, { domain: 'zendesk.com' }, { domain: 'atlassian.com' },
  { domain: 'openai.com' }, { domain: 'asana.com' },
];

// ─── Agent Roster (tabbed by team) ───

function AgentRoster() {
  const [activeTeam, setActiveTeam] = useState(0);
  const team = TEAM_AGENTS[activeTeam];

  return (
    <div>
      {/* Team selector pills */}
      <div className="sticky top-16 z-30 bg-background/90 backdrop-blur-md py-3 -mx-6 px-6 md:static md:bg-transparent md:backdrop-blur-none md:py-0 md:mx-0 md:px-0 flex flex-wrap justify-center gap-2 mb-12">
        {TEAM_AGENTS.map((t, i) => (
          <button
            key={t.team}
            onClick={() => setActiveTeam(i)}
            className="px-5 py-2.5 rounded-full text-[13px] font-semibold transition-all duration-200"
            style={{
              background: i === activeTeam ? 'var(--color-foreground)' : 'transparent',
              color: i === activeTeam ? 'var(--color-background)' : 'var(--color-muted-foreground)',
              border: i === activeTeam ? '1px solid var(--color-foreground)' : '1px solid var(--color-border)',
            }}
          >
            {t.team}
          </button>
        ))}
      </div>

      {/* Agent grid */}
      <div className="grid md:grid-cols-2 lg:grid-cols-3 gap-x-4 md:gap-x-10 gap-y-0 max-w-5xl mx-auto" key={activeTeam}>
        {team.agents.map((agent) => (
          <div
            key={agent.name}
            className="py-6 flex gap-4 border-b border-border/40"
          >
            <div className="w-9 h-9 rounded-lg flex items-center justify-center flex-shrink-0 mt-0.5" style={{ background: agent.bg }}>
              <agent.Icon className="w-[16px] h-[16px]" style={{ color: agent.color }} />
            </div>
            <div>
              <h3 className="text-[16px] font-semibold text-foreground mb-1">{agent.name}</h3>
              <p className="text-[14px] text-muted-foreground leading-relaxed">{agent.desc}</p>
            </div>
          </div>
        ))}
      </div>

      {/* Custom agent callout */}
      <div className="mt-10 text-center px-4">
        <p className="text-[14px] text-muted-foreground">
          Need a custom agent?
        </p>
        <p className="text-[14px] text-muted-foreground mt-1">
          <span className="font-semibold text-foreground">Build your own</span> — choose the tools, targets, schedule, and approval mode.
        </p>
      </div>
    </div>
  );
}

// ─── Stack Transition Visual (scroll-scrubbed) ───

const TOOL_CATEGORIES = [
  {
    label: 'Work tracking',
    tools: [
      { name: 'Linear', domain: 'linear.app' },
      { name: 'Jira', domain: 'atlassian.com' },
      { name: 'Asana', domain: 'asana.com' },
      { name: 'Monday', domain: 'monday.com' },
      { name: 'ClickUp', domain: 'clickup.com' },
    ],
  },
  {
    label: 'Docs',
    tools: [
      { name: 'Notion', domain: 'notion.so' },
      { name: 'Confluence', domain: 'atlassian.com' },
      { name: 'Coda', domain: 'coda.io' },
      { name: 'Slite', domain: 'slite.com' },
    ],
  },
  {
    label: 'Support',
    tools: [
      { name: 'Intercom', domain: 'intercom.com' },
      { name: 'Zendesk', domain: 'zendesk.com' },
      { name: 'Freshdesk', domain: 'freshdesk.com' },
    ],
  },
  {
    label: 'Sales',
    tools: [
      { name: 'HubSpot', domain: 'hubspot.com' },
      { name: 'Salesforce', domain: 'salesforce.com' },
    ],
  },
  {
    label: 'AI',
    tools: [
      { name: 'ChatGPT', domain: 'openai.com' },
      { name: 'Claude', domain: 'anthropic.com' },
    ],
  },
];

const HELPIN_MODULES = [
  { label: 'PM Work',           Icon: Layers,         color: 'oklch(0.52 0.16 250)', bg: 'oklch(0.52 0.16 250 / 0.08)' },
  { label: 'Knowledge',        Icon: BookOpen,        color: 'oklch(0.55 0.16 160)', bg: 'oklch(0.55 0.16 160 / 0.08)' },
  { label: 'Support',          Icon: MessageCircle,   color: 'oklch(0.58 0.15 55)',  bg: 'oklch(0.58 0.15 55 / 0.08)' },
  { label: 'Customer Context', Icon: Users,           color: 'oklch(0.55 0.18 310)', bg: 'oklch(0.55 0.18 310 / 0.08)' },
  { label: 'Sales CRM',        Icon: Users,           color: 'oklch(0.58 0.16 30)',  bg: 'oklch(0.58 0.16 30 / 0.08)' },
];

const remap = (v: number, lo: number, hi: number) => Math.max(0, Math.min(1, (v - lo) / (hi - lo)));

function ToolStackTransition({ scrollZoneRef }: { scrollZoneRef: React.RefObject<HTMLDivElement | null> }) {
  const [progress, setProgress] = useState(0);
  const containerRef = useRef<HTMLDivElement>(null);
  const pillRefs = useRef<Map<string, HTMLDivElement>>(new Map());
  const [pillPositions, setPillPositions] = useState<Map<string, { x: number; y: number }>>(new Map());
  const [measured, setMeasured] = useState(false);

  useEffect(() => {
    const onScroll = () => {
      const zone = scrollZoneRef.current;
      if (!zone) return;
      const rect = zone.getBoundingClientRect();
      const stickyTopPx = window.innerHeight * 0.30;
      const scrolled = stickyTopPx - rect.top;
      const p = scrolled / (window.innerHeight * 3.5);
      setProgress(Math.max(0, Math.min(1, p)));
    };
    window.addEventListener('scroll', onScroll, { passive: true });
    onScroll();
    return () => window.removeEventListener('scroll', onScroll);
  }, [scrollZoneRef]);

  // ── Measure pill positions relative to container center ──
  useEffect(() => {
    const measure = () => {
      const container = containerRef.current;
      if (!container) return;
      const cRect = container.getBoundingClientRect();
      const cx = cRect.left + cRect.width / 2;
      const cy = cRect.top + cRect.height / 2;
      const positions = new Map<string, { x: number; y: number }>();
      pillRefs.current.forEach((el, key) => {
        const r = el.getBoundingClientRect();
        positions.set(key, {
          x: (r.left + r.width / 2) - cx,
          y: (r.top + r.height / 2) - cy,
        });
      });
      if (positions.size > 0) {
        setPillPositions(positions);
        setMeasured(true);
      }
    };

    // Measure after layout settles
    const frame = requestAnimationFrame(() => {
      requestAnimationFrame(measure);
    });

    const observer = new ResizeObserver(measure);
    if (containerRef.current) observer.observe(containerRef.current);

    return () => {
      cancelAnimationFrame(frame);
      observer.disconnect();
    };
  }, []);

  // ── Phases ──
  const convergeT    = remap(progress, 0.04, 0.22);  // pills converge toward category nodes
  const mergeT       = remap(progress, 0.16, 0.26);  // pills fully merge into category nodes
  const arrangeT     = remap(progress, 0.26, 0.40);  // pentagon → orbital positions
  const centerHintT  = remap(progress, 0.20, 0.30);  // faint center disk
  const centerT      = remap(progress, 0.30, 0.38);  // solid center
  const linesT       = remap(progress, 0.36, 0.46);  // orbital lines + ring
  const aliveT       = remap(progress, 0.46, 0.56);  // living system
  const dotT         = remap(progress, 0.30, 0.36);  // traveling dot — starts with center

  // Orbital geometry
  const SIZE = 460;
  const CX = SIZE / 2;
  const CY = SIZE / 2;
  const R = 135;
  const CENTER_SIZE = 92;
  const NODE_SIZE = 52;
  const ANGLES_DEG = [-90, -18, 54, 126, 198];

  const easeInOutCubic = (t: number) => t < 0.5 ? 4 * t * t * t : 1 - Math.pow(-2 * t + 2, 3) / 2;

  // Pentagon positions (pre-arrangement) — nearly at orbital radius, same angles
  const PENT_R = R * 0.92;
  const pentPositions = ANGLES_DEG.map(deg => {
    const rad = deg * Math.PI / 180;
    return { x: PENT_R * Math.cos(rad), y: PENT_R * Math.sin(rad) };
  });

  // Orbital positions (final)
  const orbPositions = ANGLES_DEG.map(deg => {
    const rad = deg * Math.PI / 180;
    return { x: R * Math.cos(rad), y: R * Math.sin(rad) };
  });

  // Category target positions: pills converge to pentagon, then pentagon → orbit
  const at = easeInOutCubic(arrangeT);
  const nodePositions = pentPositions.map((pent, i) => ({
    x: pent.x + (orbPositions[i].x - pent.x) * at,
    y: pent.y + (orbPositions[i].y - pent.y) * at,
  }));

  // ── Build flat pill list with category index ──
  const allPills: { name: string; domain: string; catIndex: number; pillIndex: number; catSize: number }[] = [];
  TOOL_CATEGORIES.forEach((cat, catIndex) => {
    cat.tools.forEach((tool, pillIndex) => {
      allPills.push({ ...tool, catIndex, pillIndex, catSize: cat.tools.length });
    });
  });

  // How far pills have converged (0 = original pos, 1 = at category target)
  const ec = easeInOutCubic(convergeT);

  // Pill text fades out faster than favicon
  const textOpacity = Math.max(0, 1 - convergeT * 2.5);
  const faviconExtraOpacity = Math.max(0, 1 - convergeT * 1.6);

  // Pill scale shrinks slightly during convergence
  const pillScale = 1 - convergeT * 0.35;

  // Once merge is complete, pills are gone
  const pillsVisible = mergeT < 1;

  // Category nodes fade in as pills converge
  const categoryNodeOpacity = Math.min(1, convergeT * 1.5);

  return (
    <div
      ref={containerRef}
      style={{
        position: 'relative', width: '100%', minHeight: 380,
        display: 'flex', justifyContent: 'center', alignItems: 'center',
      }}
    >

      {/* ── LAYER 1: Tool pills — converge toward category nodes ── */}
      {pillsVisible && (
        <div style={{
          width: '100%',
          opacity: convergeT > 0.01 && !measured ? Math.max(0, 1 - convergeT * 3) : 1,
          pointerEvents: convergeT > 0.5 ? 'none' : 'auto',
        }}>
          {TOOL_CATEGORIES.map((cat, catIndex) => (
            <div key={cat.label} style={{ marginBottom: 10 }}>
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: 7, justifyContent: 'center' }}>
                {cat.tools.map((tool, pillIndex) => {
                  const key = `${catIndex}-${pillIndex}`;
                  const measuredPos = pillPositions.get(key);

                  // Target: the pentagon position for this category (in container-center-relative coords)
                  const target = pentPositions[catIndex];

                  // Stagger within category: first pills move slightly before later ones
                  const stagger = pillIndex / (cat.tools.length) * 0.08;
                  const staggeredEc = easeInOutCubic(Math.max(0, Math.min(1, (convergeT - stagger) / (1 - stagger))));

                  // Compute transform if we have measured positions
                  let tx = 0, ty = 0;
                  if (measured && measuredPos && convergeT > 0) {
                    // Move from measured position toward target
                    tx = (target.x - measuredPos.x) * staggeredEc;
                    ty = (target.y - measuredPos.y) * staggeredEc;
                  }

                  // Overall pill opacity: fade out during merge phase
                  const pillOpacity = Math.max(0, 1 - mergeT * 1.5);

                  return (
                    <div
                      key={tool.name}
                      ref={(el) => {
                        if (el) pillRefs.current.set(key, el);
                        else pillRefs.current.delete(key);
                      }}
                      style={{
                        display: 'inline-flex', alignItems: 'center', gap: 8,
                        padding: '8px 14px', borderRadius: 10,
                        border: '1px solid oklch(0.91 0.008 75)',
                        background: 'var(--color-background)',
                        fontSize: 13, fontWeight: 500, color: 'var(--color-foreground)',
                        transform: `translate(${tx}px, ${ty}px) scale(${pillScale})`,
                        opacity: pillOpacity,
                        transition: 'none',
                        willChange: 'transform, opacity',
                      }}
                    >
                      <img
                        src={`https://www.google.com/s2/favicons?domain=${tool.domain}&sz=32`}
                        style={{
                          width: 16, height: 16, flexShrink: 0,
                          opacity: 0.72 * faviconExtraOpacity,
                        }}
                        alt=""
                      />
                      <span style={{ whiteSpace: 'nowrap', opacity: textOpacity }}>
                        {tool.name}
                      </span>
                    </div>
                  );
                })}
              </div>
            </div>
          ))}
        </div>
      )}

      {/* ── LAYER 2: Category nodes + orbital system ── */}
      <div style={{
        position: 'absolute', inset: 0,
        opacity: categoryNodeOpacity,
        display: 'flex', justifyContent: 'center', alignItems: 'center',
        pointerEvents: categoryNodeOpacity < 0.3 ? 'none' : 'auto',
      }}>
        <div style={{ position: 'relative', width: SIZE, height: SIZE, maxWidth: '100%' }}>

          {/* Layer 1: Soft radial background halo */}
          <div style={{
            position: 'absolute',
            left: CX - R - 40, top: CY - R - 40,
            width: (R + 40) * 2, height: (R + 40) * 2,
            borderRadius: '50%',
            background: 'radial-gradient(circle, oklch(0.92 0.005 250 / 0.5) 0%, transparent 70%)',
            opacity: linesT,
            pointerEvents: 'none',
          }} />

          {/* Layer 2: SVG — orbit ring + radial connectors */}
          <svg viewBox={`0 0 ${SIZE} ${SIZE}`} style={{ position: 'absolute', inset: 0, overflow: 'visible' }}>
            {/* Solid orbit ring */}
            <circle cx={CX} cy={CY} r={R} fill="none"
              stroke={`oklch(0.88 0.01 250 / ${linesT * 0.35})`}
              strokeWidth="1"
            />
            {/* Dotted orbit ring */}
            <circle cx={CX} cy={CY} r={R} fill="none"
              stroke={`oklch(0.82 0.01 250 / ${linesT * 0.2})`}
              strokeWidth="1"
              strokeDasharray="2 8"
            />
            {/* Radial connectors — center to each node */}
            {nodePositions.map((pos, i) => (
              <line key={i}
                x1={CX} y1={CY}
                x2={CX + pos.x} y2={CY + pos.y}
                stroke={`oklch(0.88 0.005 75 / ${linesT * 0.35})`}
                strokeWidth="1"
              />
            ))}
          </svg>

          {/* Pulse rings */}
          {aliveT > 0 && [0, 1].map((i) => (
            <div key={i} style={{
              position: 'absolute',
              left: CX - 20, top: CY - 20,
              width: 40, height: 40,
              borderRadius: '50%',
              border: '1.5px solid oklch(0.48 0.15 155 / 0.18)',
              transformOrigin: 'center',
              animation: aliveT >= 1 ? `orbital-pulse-ring 4s ease-out ${i * 2}s infinite` : 'none',
              opacity: aliveT >= 1 ? 1 : 0,
              pointerEvents: 'none',
            }} />
          ))}

          {/* Traveling dot — z-index 0 so it passes behind nodes */}
          {dotT > 0 && (
            <div style={{
              position: 'absolute',
              left: CX - 4, top: CY - 4,
              width: 8, height: 8,
              transformOrigin: '4px 4px',
              animation: dotT >= 1 ? 'orbital-travel 8s linear infinite' : 'none',
              opacity: dotT >= 1 ? 0.8 : 0,
              pointerEvents: 'none',
              zIndex: 0,
            }}>
              <div style={{
                width: 8, height: 8, borderRadius: '50%',
                background: 'var(--color-pop)',
                boxShadow: '0 0 12px oklch(0.48 0.15 155 / 0.8), 0 0 24px oklch(0.48 0.15 155 / 0.4)',
              }} />
            </div>
          )}

          {/* Center node — refined with gradient, glow ring, depth */}
          <div style={{
            position: 'absolute',
            left: '50%', top: '50%',
            transform: 'translate(-50%, -50%)',
            opacity: Math.min(1, centerHintT * 0.3 + centerT * 0.7),
            pointerEvents: 'none',
          }}>
            {/* Outer glow ring */}
            <div style={{
              position: 'absolute',
              inset: -10, borderRadius: '50%',
              border: '1px solid oklch(0.12 0.02 55 / 0.05)',
              boxShadow: '0 0 40px oklch(0.12 0.02 55 / 0.06)',
              opacity: centerT,
            }} />
            {/* Core */}
            <div style={{
              width: CENTER_SIZE, height: CENTER_SIZE, borderRadius: '50%',
              background: 'linear-gradient(145deg, oklch(0.18 0.02 55), oklch(0.10 0.02 55))',
              display: 'flex', alignItems: 'center', justifyContent: 'center',
              boxShadow: `0 12px 48px oklch(0.12 0.02 55 / ${0.22 * centerT}), 0 4px 16px oklch(0.12 0.02 55 / 0.12), inset 0 1px 0 oklch(1 0 0 / 0.04)`,
              transform: aliveT >= 1 ? undefined : `scale(${0.7 + Math.min(1, centerHintT + centerT) * 0.3})`,
              transformOrigin: 'center',
              animation: aliveT >= 1 ? 'orbital-center-breathe 4s ease-in-out infinite' : 'none',
            }}>
              <span style={{
                fontSize: 14, fontWeight: 700, letterSpacing: '0.08em',
                textTransform: 'uppercase', color: 'oklch(0.98 0.003 75)',
              }}>
                Helpin
              </span>
            </div>
          </div>

          {/* Module nodes — refined with halo + inner circle */}
          {HELPIN_MODULES.map(({ label, Icon, color, bg }, i) => {
            const pos = nodePositions[i];
            const nodeAppear = Math.min(1, convergeT * 2);
            const iconScale = 0.6 + mergeT * 0.4;

            return (
              <div key={label} style={{
                position: 'absolute',
                left: CX + pos.x, top: CY + pos.y,
                transform: 'translate(-50%, -50%)',
                display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 10,
                zIndex: 2,
                opacity: nodeAppear,
              }}>
                {/* Outer halo */}
                <div style={{
                  position: 'relative',
                  width: NODE_SIZE + 18, height: NODE_SIZE + 18,
                  borderRadius: '50%',
                  background: `linear-gradient(145deg, ${color.replace(')', ' / 0.2)')}, ${color.replace(')', ' / 0.06)')})`,
                  display: 'flex', alignItems: 'center', justifyContent: 'center',
                  boxShadow: `0 6px 24px ${color.replace(')', ' / 0.2)')}, 0 0 0 1px ${color.replace(')', ' / 0.08)')}`,
                  transform: `scale(${iconScale})`,
                }}>
                  {/* Inner circle */}
                  <div style={{
                    width: NODE_SIZE, height: NODE_SIZE, borderRadius: '50%',
                    background: `linear-gradient(180deg, white, ${color.replace(')', ' / 0.06)')})`,
                    border: `1.5px solid ${color.replace(')', ' / 0.25)')}`,
                    display: 'flex', alignItems: 'center', justifyContent: 'center',
                    boxShadow: `inset 0 2px 4px ${color.replace(')', ' / 0.08)')}, 0 1px 0 oklch(1 0 0 / 0.5)`,
                  }}>
                    <Icon style={{ width: 22, height: 22, color }} />
                  </div>
                </div>
                {/* Label */}
                <span style={{
                  fontSize: 11, fontWeight: 600, letterSpacing: '0.04em',
                  color: 'var(--color-foreground)', whiteSpace: 'nowrap',
                  opacity: mergeT,
                }}>
                  {label}
                </span>
              </div>
            );
          })}

        </div>
      </div>

    </div>
  );
}

// ─── Problem → Solution combined scroll section ───

function ProblemSolutionSection() {
  const scrollZoneRef = useRef<HTMLDivElement>(null);

  return (
    <>
      <div ref={scrollZoneRef} className="lg:min-h-[230vh]" style={{ position: 'relative' }}>
        <div className="grid lg:grid-cols-2 gap-8 lg:gap-20 lg:min-h-[230vh]" style={{ maxWidth: '80rem', margin: '0 auto', padding: '0 2rem', alignItems: 'start' }}>

          {/* LEFT: 3 text sections — fixed gap between them, no flex spacer */}
          <div className="flex flex-col pt-16 pb-3 lg:pt-48 lg:pb-3 lg:self-stretch">

            {/* 1. Problem */}
            <div>
              <h2 className="text-[clamp(1.5rem,2.6vw,2.25rem)] font-bold leading-[1.12] tracking-tight text-foreground mb-7">
                Your current stack isn't built to work together.
              </h2>
              <p className="text-[17px] text-muted-foreground leading-relaxed">
                Engineering, sales, support, and operations all use different tools. Important context gets lost between them, and teams waste time chasing updates across systems.
              </p>
            </div>

            {/* 2. Bridge */}
            <div className="mt-12 lg:mt-[40vh]">
              <RevealLeft>
                <p className="text-[clamp(1.5rem,2.6vw,2.25rem)] font-bold leading-[1.12] tracking-tight text-foreground mb-3">
                  AI can't fix disconnected systems.
                </p>
                <p className="text-[17px] text-muted-foreground leading-relaxed">
                  AI is only useful when it has the full picture. If your projects, customers, support, and docs live in separate tools, AI can only work with part of the context.
                </p>
              </RevealLeft>
            </div>

            {/* 3. Solution — matches right column sticky exactly so they unstick together */}
            <div className="lg:sticky mt-12 lg:mt-[25vh] flex items-center lg:pt-[10vh] lg:h-[70vh]" style={{ top: '15vh' }}>
              <div>
                <h2 className="text-[clamp(1.5rem,2.6vw,2.25rem)] font-bold leading-[1.12] tracking-tight text-foreground mb-7">
                  <span style={{ position: 'relative', display: 'inline-block' }}>Helpin<svg style={{ position: 'absolute', bottom: -4, left: -2, width: 'calc(100% + 4px)', height: 10, overflow: 'visible' }} viewBox="0 0 100 10" preserveAspectRatio="none"><path d="M2 8C12 3 20 9 30 4C40 9 50 2 60 8C70 3 80 9 90 4C95 2 98 5 98 5" stroke="var(--color-pop)" strokeWidth="2.5" strokeLinecap="round" fill="none" /></svg></span> is built differently.
                </h2>
                <p className="text-[17px] text-muted-foreground leading-relaxed">
                  It brings project management, support, sales, and docs into one connected system. That gives humans and AI agents the full context they need to move work forward seamlessly.
                </p>
              </div>
            </div>

          </div>

          {/* RIGHT: sticky — plays full motion story as user scrolls (hidden on mobile) */}
          <div className="hidden lg:flex lg:sticky" style={{ top: '15vh', height: '70vh', alignItems: 'center' }}>
            <ToolStackTransition scrollZoneRef={scrollZoneRef} />
          </div>

        </div>
      </div>

      {/* Fixed breathing room below solution */}
      <div style={{ height: '8vh' }} />
    </>
  );
}

// ─── Main Page ───

export default function HomePage() {
  return (
    <main>

      {/* ══════════════════════════════════
          HERO
          ══════════════════════════════════ */}
      <section className="relative">
        <div className="mx-auto w-full max-w-7xl px-6 lg:px-8 pt-16 md:pt-24 pb-0">

          {/* Centered text block */}
          <div className="text-center mb-14">
            <Reveal>
              <h1 className="text-[clamp(2.25rem,5vw,4.75rem)] font-bold tracking-[-0.04em] leading-[1.0] text-foreground mb-6">
                <span className="block">The AI operating system</span>
                <span className="block">for modern work</span>
              </h1>
            </Reveal>
            <Reveal>
              <p className="text-[1.125rem] text-muted-foreground leading-relaxed max-w-2xl mx-auto mb-8">
                Helpin brings project management, support, sales, and docs into one connected system — so teams and AI agents can move work forward without silos.
              </p>
            </Reveal>
            <Reveal>
              <div className="flex flex-col items-center gap-3">
                <EarlyAccessForm id="hero" />
              </div>
            </Reveal>
          </div>

          {/* AI Workflow Visual — full on desktop, simplified on mobile */}
          <div className="mt-16" />
          <Reveal>
            <div className="hidden md:block">
              <AIWorkflowVisual />
            </div>
            {/* Mobile: simplified module list */}
            <div className="md:hidden flex flex-col items-center gap-3 py-8">
              <div className="w-14 h-14 rounded-full bg-foreground flex items-center justify-center mb-2">
                <span className="text-[10px] font-bold uppercase tracking-wider text-background">Helpin</span>
              </div>
              <div className="flex flex-wrap justify-center gap-2">
                {['Support', 'PM', 'Docs', 'Knowledge', 'CRM', 'Customer'].map((m) => (
                  <span key={m} className="text-[12px] font-medium text-muted-foreground px-3 py-1.5 rounded-full border border-border bg-background">
                    {m}
                  </span>
                ))}
              </div>
              <p className="text-[13px] text-muted-foreground/60 mt-2">Agents connect every module automatically</p>
            </div>
          </Reveal>
        </div>
      </section>

      {/* ══════════════════════════════════
          TRUSTED BY
          ══════════════════════════════════ */}
      <section className="border-y border-border py-8 md:py-12 mt-10 md:mt-16">
        <p className="text-center text-[11px] font-semibold tracking-[0.18em] text-muted-foreground/60 uppercase mb-10">
          Trusted by teams at
        </p>
        <div className="mx-auto max-w-5xl px-8 flex items-center justify-center gap-8 flex-nowrap overflow-x-auto">
          {COMPANIES.map((c, i) => (
            <div key={c.domain} className="flex items-center gap-8">
              <div className="flex items-center gap-2 opacity-75 hover:opacity-100 transition-opacity duration-200 shrink-0">
                <img
                  src={`https://www.google.com/s2/favicons?domain=${c.domain}&sz=64`}
                  alt=""
                  className="h-5 w-5 object-contain"
                />
                <span className="text-[15px] font-semibold text-foreground/90 whitespace-nowrap">{c.name}</span>
              </div>
              {i < COMPANIES.length - 1 && (
                <span className="text-border text-lg shrink-0 select-none">·</span>
              )}
            </div>
          ))}
        </div>
      </section>

      {/* ══════════════════════════════════
          PROBLEM → SOLUTION
          Right side sticky, collapses as you scroll.
          Left: problem text (top) → solution text (bottom, rises naturally).
          ══════════════════════════════════ */}
      <ProblemSolutionSection />

      {/* ══════════════════════════════════
          AGENT ROSTER
          ══════════════════════════════════ */}
      <section className="relative border-t border-border" id="agents">
        <div className="mx-auto max-w-7xl px-6 lg:px-8 py-16 md:py-32">

          <Reveal className="mb-6 text-center max-w-3xl mx-auto">
            <h2 className="text-[clamp(1.875rem,3.5vw,3rem)] font-bold leading-[1.08] tracking-tight text-foreground">
              An agent for every team, every workflow.
            </h2>
          </Reveal>
          <Reveal className="mb-12 text-center max-w-2xl mx-auto">
            <p className="text-[17px] text-muted-foreground leading-relaxed">
              Each agent works autonomously or with your approval. They share context, escalate when needed, and get smarter over time.
            </p>
          </Reveal>

          {/* Team selector */}
          <AgentRoster />
        </div>
      </section>

      {/* ══════════════════════════════════
          BEFORE / AFTER  (dark)
          ══════════════════════════════════ */}
      <section className="relative border-t border-border" id="how">
        <div className="mx-auto max-w-6xl px-6 lg:px-8 py-16 md:py-32">

          {/* Heading */}
          <Reveal className="mb-6 text-center max-w-3xl mx-auto">
            <h2 className="text-[clamp(1.875rem,3.5vw,3rem)] font-bold leading-[1.08] tracking-tight text-foreground">
              The real problems you're dealing with
            </h2>
          </Reveal>
          <Reveal className="mb-16 text-center max-w-2xl mx-auto">
            <p className="text-[17px] text-muted-foreground leading-relaxed">
              Helpin replaces disconnected tools with one connected system — so context flows, agents act, and nothing falls through.
            </p>
          </Reveal>

          {/* Visual: Before → After — aligned to problem/solution columns */}
          <Reveal className="mb-24">
            <div className="grid md:grid-cols-2 gap-8 md:gap-16 max-w-5xl mx-auto items-center">
              {/* Before: scattered tool icons — centered in left column */}
              <div className="flex flex-col items-center relative">
                <div className="relative w-[200px] sm:w-[250px] h-[95px] sm:h-[115px] mb-5">
                  {BEFORE_TOOLS.map((t, i) => {
                    const positions = [
                      { top: 0, left: 5, rotate: -5 },   { top: 3, left: 65, rotate: 3 },
                      { top: 0, left: 125, rotate: -3 }, { top: 5, left: 185, rotate: 4 },
                      { top: 58, left: 25, rotate: 4 },  { top: 55, left: 85, rotate: -4 },
                      { top: 60, left: 145, rotate: 5 }, { top: 56, left: 195, rotate: -3 },
                    ];
                    const p = positions[i];
                    return (
                      <div key={t.domain} className="absolute" style={{ top: p.top, left: p.left, transform: `rotate(${p.rotate}deg)` }}>
                        <div className="w-[40px] h-[40px] sm:w-[52px] sm:h-[52px] rounded-2xl bg-background border border-border flex items-center justify-center"
                          style={{ boxShadow: '0 2px 8px oklch(0.12 0.02 55 / 0.06), 0 0 0 1px oklch(0.12 0.02 55 / 0.03)' }}>
                          <img src={`https://www.google.com/s2/favicons?domain=${t.domain}&sz=64`} className="w-5 h-5 sm:w-7 sm:h-7" alt="" />
                        </div>
                      </div>
                    );
                  })}
                </div>
                <span className="inline-block text-[12px] font-bold uppercase tracking-widest text-foreground bg-muted px-5 py-2 rounded-full">Before</span>
                {/* Down arrow — mobile only */}
                <div className="md:hidden flex justify-center py-4"><ArrowRight className="w-6 h-6 text-muted-foreground/30 rotate-90" /></div>
                {/* Swoosh arrow — positioned to bridge the gap */}
                <div className="absolute right-[-80px] top-1/2 -translate-y-1/2 hidden md:block">
                  <svg width="140" height="56" viewBox="0 0 140 56" fill="none">
                    <path d="M4 20C35 20 45 38 70 38C95 38 105 26 128 26" stroke="oklch(0.12 0.02 55 / 0.22)" strokeWidth="4.5" strokeLinecap="round" fill="none" />
                    <path d="M118 16L130 26L118 36" stroke="oklch(0.12 0.02 55 / 0.22)" strokeWidth="4.5" strokeLinecap="round" strokeLinejoin="round" fill="none" />
                  </svg>
                </div>
              </div>

              {/* After: Helpin — slightly left of center in right column */}
              <div className="flex flex-col items-center md:items-center md:mr-auto md:ml-16">
                <div className="w-[100px] h-[100px] rounded-3xl bg-foreground flex items-center justify-center mb-5"
                  style={{ boxShadow: '0 12px 40px oklch(0.12 0.02 55 / 0.25), 0 4px 12px oklch(0.12 0.02 55 / 0.1)' }}>
                  <span className="text-[14px] font-bold uppercase tracking-wider text-background">Helpin</span>
                </div>
                <span className="inline-block text-[12px] font-bold uppercase tracking-widest rounded-full px-5 py-2" style={{ color: 'oklch(0.45 0.15 155)', background: 'oklch(0.45 0.15 155 / 0.08)' }}>After</span>
              </div>
            </div>
          </Reveal>

          {/* Problem / Solution rows */}
          <div className="max-w-5xl mx-auto">
            {PROBLEMS_SOLUTIONS.map((row, i) => (
              <Reveal key={i}>
                <div className="grid md:grid-cols-2 gap-8 md:gap-16 py-8 border-b border-border/50">
                  {/* Problem */}
                  <div>
                    <span className="md:hidden inline-block text-[10px] font-bold uppercase tracking-widest text-muted-foreground/40 mb-2">Without Helpin</span>
                    <div className="flex gap-4">
                    <span className="text-[18px] font-black flex-shrink-0 mt-0.5" style={{ color: 'oklch(0.55 0.2 25)' }}>✕</span>
                    <div>
                      <h3 className="text-[15px] font-bold text-foreground mb-2">{row.problem}</h3>
                      <p className="text-[14px] text-muted-foreground leading-relaxed">{row.problemDesc}</p>
                    </div>
                    </div>
                  </div>
                  {/* Solutions */}
                  <div>
                    <span className="md:hidden inline-block text-[10px] font-bold uppercase tracking-widest text-pop/60 mb-2">With Helpin</span>
                    <div className="flex flex-col gap-3">
                      {row.solutions.map((s, j) => (
                        <div key={j} className="flex gap-3">
                          <span className="text-[16px] font-black flex-shrink-0 mt-0.5" style={{ color: 'oklch(0.45 0.15 155)' }}>✓</span>
                          <p className="text-[14px] font-medium text-foreground leading-relaxed">{s}</p>
                        </div>
                      ))}
                    </div>
                  </div>
                </div>
              </Reveal>
            ))}
          </div>
        </div>
      </section>

      {/* ══════════════════════════════════
          HOW IT WORKS
          ══════════════════════════════════ */}
      <section className="relative border-t border-border">
        <div className="mx-auto max-w-6xl px-6 lg:px-8 py-16 md:py-32">
          <Reveal className="mb-6 text-center max-w-3xl mx-auto">
            <h2 className="text-[clamp(1.875rem,3.5vw,3rem)] font-bold leading-[1.08] tracking-tight text-foreground">
              How it works
            </h2>
          </Reveal>
          <Reveal className="mb-20 text-center max-w-2xl mx-auto">
            <p className="text-[17px] text-muted-foreground leading-relaxed">
              Your team stays lean. Your output doesn't. Helpin agents handle the repetitive work across every department — so your people focus on what actually moves the needle.
            </p>
          </Reveal>

          {/* Step cards — progressive elevation */}
          <div className="flex flex-col lg:flex-row gap-3 max-w-5xl mx-auto items-stretch">
            {[
              {
                num: '01',
                Icon: Users,
                title: 'Set up your workspace',
                desc: 'Create your workspace, invite your team, and import your data.',
                intensity: 0,
              },
              {
                num: '02',
                Icon: Shield,
                title: 'Configure your agents',
                desc: 'Pick built-in agents or build your own. Set tools, targets, and approval mode.',
                intensity: 1,
              },
              {
                num: '03',
                Icon: Zap,
                title: 'Agents go to work',
                desc: 'They plan, build, triage, and follow up — using your real company context.',
                intensity: 2,
              },
              {
                num: '04',
                Icon: TrendingUp,
                title: 'Increase autonomy',
                desc: 'Start supervised. Increase trust over time. From approval-required to fully autonomous.',
                intensity: 3,
                highlight: true,
              },
            ].map((step, i, arr) => {
              const bgTint = step.highlight ? 'var(--color-pop-light)' : 'var(--color-background)';
              const borderStyle = step.highlight ? '1.5px solid var(--color-pop)' : '1px solid var(--color-border)';
              const shadow = step.highlight ? '0 8px 32px oklch(0.12 0.02 55 / 0.08)' : '0 2px 8px oklch(0.12 0.02 55 / 0.04)';
              return (
                <div key={step.num} className="contents">
                  <Reveal className="flex-1">
                    <div
                      className="relative rounded-2xl p-7 h-full transition-all duration-300"
                      style={{
                        background: bgTint,
                        border: borderStyle,
                        boxShadow: shadow,
                      }}
                    >
                      {/* Number — large faint background */}
                      <span className="absolute top-4 right-5 text-[48px] font-bold leading-none select-none"
                        style={{ color: step.highlight ? 'oklch(0.52 0.14 28 / 0.12)' : 'oklch(0.12 0.02 55 / 0.04)' }}>
                        {step.num}
                      </span>
                      {/* Icon */}
                      <div className="w-11 h-11 rounded-xl flex items-center justify-center mb-5"
                        style={{
                          background: step.highlight ? 'var(--color-pop-medium)' : 'var(--color-muted)',
                        }}>
                        <step.Icon className="w-5 h-5" style={{ color: step.highlight ? 'var(--color-pop)' : 'var(--color-foreground)' }} />
                      </div>
                      {/* Content */}
                      <h3 className="text-[16px] font-bold text-foreground mb-2">{step.title}</h3>
                      <p className="text-[14px] text-muted-foreground leading-relaxed">{step.desc}</p>
                    </div>
                  </Reveal>
                  {/* Arrow between cards */}
                  {i < arr.length - 1 && (
                    <div className="hidden lg:flex items-center justify-center flex-shrink-0 px-1">
                      <ArrowRight className="w-6 h-6 text-muted-foreground/50" strokeWidth={2.5} />
                    </div>
                  )}
                </div>
              );
            })}
          </div>

          {/* CTA */}
          <Reveal className="mt-14 flex justify-center px-4">
            <EarlyAccessForm id="how-it-works" />
          </Reveal>
        </div>
      </section>

      {/* ══════════════════════════════════
          FAQ
          ══════════════════════════════════ */}
      <section className="relative border-t border-border">
        <div className="mx-auto max-w-3xl px-6 lg:px-8 py-16 md:py-32">
          <Reveal className="mb-16 text-center">
            <h2 className="text-[clamp(1.875rem,3.5vw,3rem)] font-bold leading-[1.08] tracking-tight text-foreground">
              Frequently asked questions
            </h2>
          </Reveal>

          <div className="space-y-0">
            {[
              {
                q: 'What happens to my existing tools?',
                a: 'You keep GitHub — Helpin integrates directly. For everything else (Jira, Notion, Intercom, HubSpot), you can import your data. Don\'t see your tool? We\'ll add any importer you need.',
              },
              {
                q: 'How do agents work under the hood?',
                a: 'Helpin runs its own CLI runtime — similar to Claude Code or OpenAI Codex, but built into the platform. You can watch agents work in real-time, interact with them mid-run, or let them run fully autonomously. No black box.',
              },
              {
                q: 'Which AI models does Helpin use?',
                a: 'You bring your own API keys. Helpin supports Claude, GPT, and other providers — you choose the model per agent. Switch anytime. Your keys, your cost control.',
              },
              {
                q: 'I already use Claude Code / Cursor / Copilot. How is this different?',
                a: 'Those tools help individual developers write code. Helpin orchestrates work across your entire company — planning, building, reviewing, supporting, and selling. One agent writes the code, another reviews it, another updates the docs. They share context. That\'s something a code editor can\'t do.',
              },
              {
                q: 'Can agents act without my approval?',
                a: 'You control the dial. Every agent can be set to require human approval, or run fully autonomously. Most teams start supervised and increase trust over time.',
              },
              {
                q: 'Is my data safe?',
                a: 'Yes. Your data is encrypted at rest and in transit. It\'s never shared across workspaces and never used to train AI models.',
              },
              {
                q: 'How is this different from using ChatGPT + my current stack?',
                a: 'ChatGPT doesn\'t know your company. It can\'t read your tickets, check your roadmap, or update your docs. Helpin agents operate inside your system with full context — they don\'t just answer questions, they do the work.',
              },
              {
                q: 'What does it cost?',
                a: 'Free during early access. No credit card required. Start with your full team today.',
              },
              {
                q: 'How long does setup take?',
                a: 'Most teams are up and running in under 10 minutes. Import your data, configure your first agent, and it starts working immediately.',
              },
            ].map((faq, i) => (
              <Reveal key={i}>
                <details className="group border-b border-border/50 py-6">
                  <summary className="flex items-center justify-between cursor-pointer list-none">
                    <h3 className="text-[18px] font-semibold text-foreground pr-8">{faq.q}</h3>
                    <span className="text-muted-foreground/40 text-xl flex-shrink-0 transition-transform duration-200 group-open:rotate-45">+</span>
                  </summary>
                  <p className="text-[17px] text-muted-foreground leading-relaxed mt-4 pr-12">{faq.a}</p>
                </details>
              </Reveal>
            ))}
          </div>
        </div>
      </section>


    </main>
  );
}

// ─── AI Workflow Visual (10X) ───
// Hybrid HTML cards + SVG constellation with triple-layer comet signals

const WF_DATA = [
  { id: 'support',   module: 'Support',    action: 'Ticket triaged',       dot: '#e11d48', svg: { x: 120, y: 75 },  css: { left: '13.3%', top: '15.6%' },  activateAt: 1, signalAt: 2, signalDir: 'in'  as const, packet: 'Login bug' },
  { id: 'pm',        module: 'PM',         action: 'Task created',         dot: '#2563eb', svg: { x: 450, y: 32 },  css: { left: '50%',   top: '6.7%' },   activateAt: 4, signalAt: 4, signalDir: 'out' as const, packet: 'Create task' },
  { id: 'crm',       module: 'Sales',      action: 'Account flagged',      dot: '#ea580c', svg: { x: 450, y: 448 }, css: { left: '50%',   top: '93.3%' },  activateAt: 5, signalAt: 5, signalDir: 'out' as const, packet: 'Flag risk' },
  { id: 'docs',      module: 'Docs',       action: 'Guide updated',        dot: '#16a34a', svg: { x: 780, y: 75 },  css: { left: '86.7%', top: '15.6%' },  activateAt: 6, signalAt: 6, signalDir: 'out' as const, packet: 'Update doc' },
  { id: 'knowledge', module: 'Knowledge',  action: '3 matches found',      dot: '#9333ea', svg: { x: 120, y: 405 }, css: { left: '13.3%', top: '84.4%' },  activateAt: 7, signalAt: 7, signalDir: 'out' as const, packet: 'Find related' },
  { id: 'customer',  module: 'Customer',   action: 'Update sent',           dot: '#0891b2', svg: { x: 780, y: 405 }, css: { left: '86.7%', top: '84.4%' },  activateAt: 8, signalAt: 8, signalDir: 'out' as const, packet: 'Investigating' },
];

const WF_CTR = { x: 450, y: 240 };

function wfCurve(x1: number, y1: number, x2: number, y2: number) {
  const mx = (x1 + x2) / 2;
  const my = (y1 + y2) / 2;
  const dx = x2 - x1;
  const dy = y2 - y1;
  return `M ${x1} ${y1} Q ${mx - dy * 0.12} ${my + dx * 0.12} ${x2} ${y2}`;
}

function wfCurveReversed(x1: number, y1: number, x2: number, y2: number) {
  const mx = (x1 + x2) / 2;
  const my = (y1 + y2) / 2;
  const dx = x2 - x1;
  const dy = y2 - y1;
  const qx = mx - dy * 0.12;
  const qy = my + dx * 0.12;
  return `M ${x2} ${y2} Q ${qx} ${qy} ${x1} ${y1}`;
}

function TravelingPacket({ pathD, label, color, duration = 2200 }: { pathD: string; label: string; color?: string; duration?: number }) {
  const pathRef = useRef<SVGPathElement>(null);
  const [pos, setPos] = useState<{ x: number; y: number } | null>(null);

  useEffect(() => {
    const el = pathRef.current;
    if (!el) return;
    const totalLen = el.getTotalLength();
    const endPt = el.getPointAtLength(totalLen);
    const startPt = el.getPointAtLength(0);
    const cx = 450, cy = 240;
    const endDist = Math.sqrt((endPt.x - cx) ** 2 + (endPt.y - cy) ** 2);
    const startDist = Math.sqrt((startPt.x - cx) ** 2 + (startPt.y - cy) ** 2);
    const trimStart = startDist < endDist ? 36 : 0;
    const trimEnd = endDist < startDist ? 36 : 0;
    const usableLen = totalLen - trimStart - trimEnd;

    const start = performance.now();
    let raf: number;

    const tick = (now: number) => {
      const t = Math.min((now - start) / duration, 1);
      const eased = t < 0.5 ? 2 * t * t : 1 - Math.pow(-2 * t + 2, 2) / 2;
      const pt = el.getPointAtLength(trimStart + eased * usableLen);
      setPos({ x: pt.x, y: pt.y });
      if (t < 1) raf = requestAnimationFrame(tick);
    };
    raf = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(raf);
  }, [pathD, duration]);

  return (
    <>
      <path ref={pathRef} d={pathD} fill="none" stroke="none" />
      {pos && (() => {
        const w = Math.max(48, label.length * 6.5 + 20);
        const hw = w / 2;
        return (
          <g transform={`translate(${pos.x}, ${pos.y})`}>
            {/* Shadow */}
            <rect x={-hw + 1} y="-8" width={w - 2} height="18" rx="9"
              fill={color ? `${color}20` : 'oklch(0.12 0.02 55 / 0.1)'} />
            {/* Pastel background */}
            <rect x={-hw} y="-10" width={w} height="20" rx="10"
              fill={color ? `${color}30` : 'var(--color-pop-light)'}
              stroke={color ? `${color}50` : 'var(--color-pop)'}
              strokeWidth="1.5" />
            {/* Text */}
            <text x="0" y="3.5" textAnchor="middle"
              fill={color || 'var(--color-pop)'} fontSize="8.5" fontWeight="800" fontFamily="var(--font-sans)"
              style={{ letterSpacing: '0.02em' }}>
              {label}
            </text>
          </g>
        );
      })()}
    </>
  );
}

function getNodeActions(id: string, step: number): { text: string; done: boolean }[] {
  const actions: { text: string; done: boolean }[] = [];
  if (id === 'support') {
    if (step >= 1) actions.push({ text: 'Ticket received', done: step >= 3 });
    if (step >= 4) actions.push({ text: 'Triaged by AI', done: step >= 4 });
    if (step >= 12) actions.push({ text: 'Resolved', done: true });
  } else if (id === 'pm') {
    if (step >= 5) actions.push({ text: 'Task created', done: step >= 6 });
    if (step >= 6) actions.push({ text: 'Assigned to sprint', done: step >= 9 });
    if (step >= 9) actions.push({ text: 'Bug fixed', done: true });
  } else if (id === 'crm') {
    if (step >= 6) actions.push({ text: 'Account flagged', done: step >= 7 });
    if (step >= 7) actions.push({ text: 'Risk alert sent', done: true });
  } else if (id === 'docs') {
    if (step >= 7) actions.push({ text: 'Guide updated', done: true });
  } else if (id === 'knowledge') {
    if (step >= 8) actions.push({ text: '3 matches found', done: true });
  } else if (id === 'customer') {
    if (step >= 9) actions.push({ text: 'Update sent', done: step >= 12 });
    if (step >= 12) actions.push({ text: 'Issue resolved', done: true });
  }
  return actions;
}

function WfMicroUI({ id, on, step }: { id: string; on: boolean; step: number }) {
  const t = (v: string, off: string) => ({ color: on ? v : off, transition: 'color 0.4s ease' });

  /* ── Support: chat bubble ── */
  if (id === 'support') return (
    <div>
      <div className="rounded-lg rounded-tl-sm px-2.5 py-2" style={{
        background: on ? 'oklch(0.95 0.01 75)' : 'oklch(0.96 0.005 75 / 0.5)',
        transition: 'background 0.4s ease',
      }}>
        <p className="text-[11px] font-medium leading-snug" style={t('oklch(0.15 0.02 55)', 'oklch(0.15 0.02 55 / 0.18)')}>
          Login not working on mobile
        </p>
      </div>
      <p className="text-[9px] mt-1" style={t('oklch(0.12 0.02 55 / 0.4)', 'oklch(0.12 0.02 55 / 0.1)')}>
        Sarah K. · Acme Corp
      </p>
    </div>
  );

  /* ── PM: task card with priority bar ── */
  if (id === 'pm') return (
    <div className="flex rounded-md overflow-hidden" style={{
      border: `1px solid ${on ? 'oklch(0.9 0.008 75)' : 'oklch(0.94 0.005 75 / 0.4)'}`,
      background: on ? 'oklch(1 0 0 / 0.8)' : 'oklch(1 0 0 / 0.25)',
      transition: 'all 0.4s ease',
    }}>
      <div className="w-1 shrink-0" style={{
        background: on ? '#ef4444' : 'oklch(0.85 0.005 75)',
        transition: 'background 0.4s ease',
      }} />
      <div className="px-2 py-1.5 min-w-0">
        <p className="text-[11px] font-medium truncate" style={t('oklch(0.15 0.02 55)', 'oklch(0.15 0.02 55 / 0.18)')}>
          Fix mobile auth token refresh
        </p>
        <p className="text-[9px]" style={t('oklch(0.12 0.02 55 / 0.4)', 'oklch(0.12 0.02 55 / 0.1)')}>
          Sprint 14 · Urgent
        </p>
      </div>
    </div>
  );

  /* ── Docs: mini document page ── */
  if (id === 'docs') return (
    <div className="rounded-md overflow-hidden" style={{
      border: `1px solid ${on ? 'oklch(0.88 0.04 155)' : 'oklch(0.94 0.005 75 / 0.4)'}`,
      background: on ? 'oklch(1 0 0 / 0.8)' : 'oklch(1 0 0 / 0.25)',
      transition: 'all 0.4s ease',
    }}>
      <div className="px-2 py-0.5" style={{
        background: on ? 'oklch(0.95 0.03 155)' : 'oklch(0.96 0.005 75 / 0.5)',
        transition: 'background 0.4s ease',
      }}>
        <p className="text-[8px] font-medium" style={t('oklch(0.35 0.1 155)', 'oklch(0.12 0.02 55 / 0.12)')}>DOC</p>
      </div>
      <div className="px-2 py-1.5">
        <p className="text-[11px] font-medium truncate" style={t('oklch(0.15 0.02 55)', 'oklch(0.15 0.02 55 / 0.18)')}>
          Mobile auth guide
        </p>
        <p className="text-[9px]" style={t('oklch(0.12 0.02 55 / 0.4)', 'oklch(0.12 0.02 55 / 0.1)')}>
          Updated automatically
        </p>
      </div>
    </div>
  );

  /* ── CRM: account card with health ── */
  if (id === 'crm') return (
    <div className="rounded-md px-2.5 py-2" style={{
      border: `1px solid ${on ? 'oklch(0.9 0.008 75)' : 'oklch(0.94 0.005 75 / 0.4)'}`,
      background: on ? 'oklch(1 0 0 / 0.8)' : 'oklch(1 0 0 / 0.25)',
      transition: 'all 0.4s ease',
    }}>
      <div className="flex items-center gap-1.5">
        <div className="w-5 h-5 rounded flex items-center justify-center shrink-0" style={{
          background: on ? 'oklch(0.94 0.03 50)' : 'oklch(0.95 0.005 75)',
          transition: 'background 0.4s ease',
        }}>
          <span className="text-[8px] font-bold" style={t('oklch(0.4 0.08 50)', 'oklch(0.12 0.02 55 / 0.12)')}>AC</span>
        </div>
        <div className="min-w-0">
          <p className="text-[11px] font-medium truncate" style={t('oklch(0.15 0.02 55)', 'oklch(0.15 0.02 55 / 0.18)')}>Acme Corp · $48k</p>
          <div className="flex items-center gap-1">
            <span className="w-1.5 h-1.5 rounded-full shrink-0" style={{
              background: on ? '#f59e0b' : 'oklch(0.88 0.005 75)',
              transition: 'background 0.4s ease',
            }} />
            <span className="text-[9px]" style={t('oklch(0.55 0.12 70)', 'oklch(0.12 0.02 55 / 0.1)')}>At risk</span>
          </div>
        </div>
      </div>
    </div>
  );

  /* ── Knowledge: search result with match ── */
  if (id === 'knowledge') return (
    <div className="rounded-md px-2.5 py-2" style={{
      border: `1px solid ${on ? 'oklch(0.9 0.008 75)' : 'oklch(0.94 0.005 75 / 0.4)'}`,
      background: on ? 'oklch(1 0 0 / 0.8)' : 'oklch(1 0 0 / 0.25)',
      transition: 'all 0.4s ease',
    }}>
      <p className="text-[11px] font-medium truncate" style={t('oklch(0.15 0.02 55)', 'oklch(0.15 0.02 55 / 0.18)')}>
        Mobile login issues
      </p>
      <div className="flex items-center gap-1.5 mt-1">
        <div className="h-1 flex-1 rounded-full overflow-hidden" style={{
          background: on ? 'oklch(0.93 0.01 75)' : 'oklch(0.95 0.005 75)',
          transition: 'background 0.4s ease',
        }}>
          <div className="h-full rounded-full" style={{
            width: on ? '92%' : '0%',
            background: 'oklch(0.55 0.15 300)',
            transition: 'width 0.9s cubic-bezier(0.16,1,0.3,1) 0.15s',
          }} />
        </div>
        <span className="text-[9px] font-semibold shrink-0" style={t('oklch(0.5 0.15 300)', 'transparent')}>92%</span>
      </div>
    </div>
  );

  /* ── Customer: first = investigating, after return = resolved ── */
  if (id === 'customer') {
    const resolved = step >= 11;
    return (
      <div className="rounded-md px-2.5 py-2 flex items-center gap-2" style={{
        background: on ? (resolved ? 'oklch(0.95 0.04 160)' : 'oklch(0.95 0.02 55)') : 'oklch(0.96 0.005 75 / 0.4)',
        border: `1px solid ${on ? (resolved ? 'oklch(0.88 0.06 160)' : 'oklch(0.90 0.01 55)') : 'oklch(0.94 0.005 75 / 0.4)'}`,
        transition: 'all 0.4s ease',
      }}>
        <div className="w-5 h-5 rounded-full shrink-0 flex items-center justify-center" style={{
          background: on ? (resolved ? 'oklch(0.45 0.15 160)' : 'oklch(0.6 0.12 55)') : 'oklch(0.88 0.005 75)',
          transition: 'background 0.4s ease',
        }}>
          <span className="text-[10px] font-bold" style={{ color: on ? '#fff' : 'transparent', transition: 'color 0.4s ease' }}>
            {resolved ? '\u2713' : '\u2026'}
          </span>
        </div>
        <div className="min-w-0">
          <p className="text-[11px] font-medium" style={t(resolved ? 'oklch(0.2 0.05 160)' : 'oklch(0.2 0.02 55)', 'oklch(0.15 0.02 55 / 0.18)')}>
            {resolved ? 'Resolved' : 'Investigating'}
          </p>
          <p className="text-[9px]" style={t(resolved ? 'oklch(0.35 0.08 160)' : 'oklch(0.4 0.02 55)', 'oklch(0.12 0.02 55 / 0.1)')}>
            {resolved ? 'Fixed in 8 min' : 'Looking into it'}
          </p>
        </div>
      </div>
    );
  }

  return null;
}

function AIWorkflowVisual() {
  const [step, setStep] = useState(-1);

  useEffect(() => {
    const t = setTimeout(() => setStep(0), 600);
    return () => clearTimeout(t);
  }, []);

  useEffect(() => {
    if (step < 0) return;
    const d = [800, 1800, 2000, 1200, 2000, 2000, 2000, 2000, 2000, 2000, 1200, 2000, 1600];
    const t = setTimeout(() => setStep((s) => (s + 1) % 13), d[step]);
    return () => clearTimeout(t);
  }, [step]);

  const { x: cx, y: cy } = WF_CTR;
  const centerOn = step >= 2 && step <= 12;
  const processing = step === 3 || step === 10;

  return (
    <div className="relative w-full" style={{ aspectRatio: '900 / 480' }}>

      {/* ══ SVG Layer ══ */}
      <svg viewBox="0 0 900 480" fill="none" className="absolute inset-0 w-full h-full select-none">
        <defs>
          <filter id="wf-glow" x="-100%" y="-100%" width="300%" height="300%">
            <feGaussianBlur stdDeviation="6" result="b" />
            <feComposite in="SourceGraphic" in2="b" operator="over" />
          </filter>
          <radialGradient id="wf-ambient" cx="50%" cy="50%" r="50%">
            <stop offset="0%" stopColor="oklch(0.48 0.15 155 / 0.06)" />
            <stop offset="100%" stopColor="oklch(0.48 0.15 155 / 0)" />
          </radialGradient>
          {/* Mask: hide lines inside center circle */}
          <mask id="wf-center-mask">
            <rect width="900" height="480" fill="white" />
            <circle cx={cx} cy={cy} r={42} fill="black" />
          </mask>
        </defs>

        {/* Background rings */}
        {[100, 165, 230, 295].map((r) => (
          <circle key={r} cx={cx} cy={cy} r={r} stroke="oklch(0.12 0.02 55 / 0.02)" strokeWidth="1" />
        ))}

        {/* Center ambient glow */}
        <circle cx={cx} cy={cy} r={140} fill={centerOn ? 'url(#wf-ambient)' : 'none'}
          style={{ transition: 'opacity 1s ease' }} />

        {/* Lines masked to stop at center circle edge */}
        <g mask="url(#wf-center-mask)">
        {/* ── Connection lines (dotted base) ── */}
        {WF_DATA.map((n) => (
          <path key={`base-${n.id}`}
            d={wfCurve(n.svg.x, n.svg.y, cx, cy)}
            stroke="oklch(0.12 0.02 55 / 0.07)" strokeWidth="1.5"
            strokeDasharray="3 8" strokeLinecap="round" />
        ))}

        {/* ── Active connection (subtle solid line when node is on) ── */}
        {WF_DATA.map((n) => {
          const on = step >= n.activateAt && step > 0;
          return (
            <path key={`active-${n.id}`}
              d={wfCurve(n.svg.x, n.svg.y, cx, cy)}
              stroke="var(--color-pop)"
              strokeWidth="1"
              strokeLinecap="round"
              opacity={on ? 0.15 : 0}
              style={{ transition: 'opacity 0.6s ease' }} />
          );
        })}

        </g>{/* end masked lines */}

        {/* ── Traveling packets (outside mask so they stay visible) ── */}
        {WF_DATA.map((n) => {
          if (step !== n.signalAt) return null;
          const travelPath = n.signalDir === 'in'
            ? wfCurve(n.svg.x, n.svg.y, cx, cy)
            : wfCurveReversed(n.svg.x, n.svg.y, cx, cy);
          return (
            <TravelingPacket key={`pkt-${n.id}-${step}`} pathD={travelPath} label={n.packet} color={n.dot} />
          );
        })}

        {/* ── Return packets (closing the loop) ── */}
        {step === 9 && (
          <TravelingPacket key={`pkt-return-done-${step}`}
            pathD={wfCurve(WF_DATA[1].svg.x, WF_DATA[1].svg.y, cx, cy)}
            label="Done ✓" color="#16a34a" />
        )}
        {step === 11 && (
          <TravelingPacket key={`pkt-return-resolved-${step}`}
            pathD={wfCurveReversed(WF_DATA[0].svg.x, WF_DATA[0].svg.y, cx, cy)}
            label="Resolved" color="#16a34a" />
        )}

        {/* ── Center AI hub ── */}
        <g>
          <circle cx={cx} cy={cy} r={68}
            stroke={centerOn ? 'oklch(0.48 0.15 155 / 0.1)' : 'oklch(0.12 0.02 55 / 0.03)'}
            strokeWidth="1" strokeDasharray="6 10" className="wf-ring-outer"
            style={{ transition: 'stroke 1s ease' }} />
          <circle cx={cx} cy={cy} r={50}
            stroke={centerOn ? 'oklch(0.48 0.15 155 / 0.2)' : 'oklch(0.12 0.02 55 / 0.04)'}
            strokeWidth="1" strokeDasharray="3 6" className="wf-ring-inner"
            style={{ transition: 'stroke 0.8s ease' }} />

          {(step === 2 || step === 3 || step === 9 || step === 10 || step === 12) && (
            <>
              <circle key={`p1-${step}`} cx={cx} cy={cy} r={34}
                stroke="oklch(0.48 0.15 155 / 0.3)" strokeWidth="1.5" className="wf-pulse" />
              <circle key={`p2-${step}`} cx={cx} cy={cy} r={34}
                stroke="oklch(0.48 0.15 155 / 0.18)" strokeWidth="1" className="wf-pulse"
                style={{ animationDelay: '0.3s' }} />
            </>
          )}

          <circle cx={cx} cy={cy} r={36}
            fill={processing ? 'oklch(0.48 0.15 155 / 0.12)' : centerOn ? 'oklch(0.48 0.15 155 / 0.06)' : 'oklch(0.12 0.02 55 / 0.012)'}
            stroke={processing ? 'oklch(0.48 0.15 155 / 0.35)' : centerOn ? 'oklch(0.48 0.15 155 / 0.22)' : 'oklch(0.12 0.02 55 / 0.05)'}
            strokeWidth="1" className="wf-core-breathe"
            style={{ transition: 'fill 0.3s ease, stroke 0.3s ease' }} />

          {/* HELPIN AI text — hidden during processing */}
          <text x={cx} y={cy - 6} textAnchor="middle" dominantBaseline="middle"
            fontSize="9" fontWeight="700" letterSpacing="0.14em"
            fill={processing ? 'transparent' : centerOn ? 'oklch(0.48 0.15 155 / 0.8)' : 'oklch(0.12 0.02 55 / 0.2)'}
            style={{ transition: 'fill 0.3s ease', fontFamily: 'var(--font-sans)' }}>HELPIN</text>
          <text x={cx} y={cy + 10} textAnchor="middle" dominantBaseline="middle"
            fontSize="15" fontWeight="800" letterSpacing="0.2em"
            fill={processing ? 'transparent' : centerOn ? 'oklch(0.48 0.15 155)' : 'oklch(0.12 0.02 55 / 0.15)'}
            style={{ transition: 'fill 0.3s ease', fontFamily: 'var(--font-sans)' }}>AI</text>

          {/* Processing label */}
          <text x={cx} y={cy + 2} textAnchor="middle" dominantBaseline="middle"
            fontSize="10" fontWeight="600" letterSpacing="0.06em"
            fill={processing ? 'oklch(0.48 0.15 155)' : 'transparent'}
            style={{ transition: 'fill 0.3s ease', fontFamily: 'var(--font-sans)' }}>
            {step === 3 ? 'Triaging...' : step === 10 ? 'Resolving...' : ''}
          </text>
        </g>

        {/* ── Small anchor dots at node positions ── */}
        {WF_DATA.map((n) => {
          const on = step >= n.activateAt && step > 0;
          return (
            <circle key={`anchor-${n.id}`}
              cx={n.svg.x} cy={n.svg.y} r={3}
              fill={on ? 'oklch(0.48 0.15 155 / 0.5)' : 'oklch(0.12 0.02 55 / 0.06)'}
              style={{ transition: 'fill 0.4s ease' }} />
          );
        })}
      </svg>

      {/* ══ HTML Layer: Product Cards with Micro UI ══ */}
      <div className="absolute inset-0 pointer-events-none">
        {WF_DATA.map((n) => {
          const on = step >= n.activateAt && step > 0;
          return (
            <div key={`card-${n.id}`} className={`wf-card ${on ? 'on' : ''}`}
              style={{ left: n.css.left, top: n.css.top }}>
              {/* Module label */}
              <div className="flex items-center gap-1.5 mb-1.5">
                <span className="w-1.5 h-1.5 rounded-full shrink-0"
                  style={{ background: on ? n.dot : 'oklch(0.12 0.02 55 / 0.12)', transition: 'background 0.4s ease' }} />
                <span className="text-[9px] font-semibold tracking-wide"
                  style={{ color: on ? 'oklch(0.12 0.02 55 / 0.6)' : 'oklch(0.12 0.02 55 / 0.2)', transition: 'color 0.4s ease' }}>
                  {n.module}
                </span>
              </div>
              {/* Micro UI row */}
              <WfMicroUI id={n.id} on={on} step={step} />
              {/* Action labels — progressive, step-aware */}
              <div className="mt-1.5 flex flex-col gap-0.5">
                {getNodeActions(n.id, step).map((action, ai) => (
                  <div key={ai} className="flex items-center gap-1"
                    style={{ opacity: 1, transition: 'opacity 0.3s ease' }}>
                    <span className="h-1.5 w-1.5 rounded-full shrink-0" style={{
                      background: action.done ? 'oklch(0.48 0.15 155)' : 'oklch(0.55 0.12 55)',
                    }} />
                    <span className="text-[10px] font-medium" style={{
                      color: action.done ? 'oklch(0.12 0.02 55 / 0.4)' : 'oklch(0.12 0.02 55 / 0.55)',
                      textDecoration: action.done ? 'line-through' : 'none',
                    }}>{action.text}</span>
                  </div>
                ))}
              </div>
            </div>
          );
        })}
      </div>

    </div>
  );
}
