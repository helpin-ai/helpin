import Link from 'next/link';

// ─────────────────────────────────────────────
// Data
// ─────────────────────────────────────────────

const AGENTS = [
  {
    initial: 'P',
    role: 'Product Planner',
    accent: 'bg-amber-100 text-amber-700',
    scenario:
      'You write "Add SSO support" as an epic. The planner agent breaks it into 14 stories — each with acceptance criteria, priority, estimates, and dependencies. Your backlog is sprint-ready before standup.',
    capabilities: ['Spec writing', 'Story breakdown', 'Dependency mapping'],
  },
  {
    initial: 'E',
    role: 'Engineer',
    accent: 'bg-slate-100 text-slate-700',
    scenario:
      'A bug story gets assigned. The engineer agent reads the codebase, writes a fix, opens a PR, and links it back to the story. You review the code — you don\'t write it.',
    capabilities: ['Code generation', 'PR creation', 'Bug fixes'],
  },
  {
    initial: 'R',
    role: 'Reviewer',
    accent: 'bg-emerald-100 text-emerald-700',
    scenario:
      'Every PR gets reviewed before a human touches it. The reviewer checks for regressions, runs tests, and flags issues. By the time you look at the diff, the obvious problems are already caught.',
    capabilities: ['Code review', 'Test validation', 'Regression checks'],
  },
  {
    initial: 'S',
    role: 'Support Agent',
    accent: 'bg-sky-100 text-sky-700',
    scenario:
      'A ticket comes in at 2 AM. The support agent drafts a response, matches the customer to their CRM contact, and flags a potential churn risk. By morning, your team has context — not a backlog.',
    capabilities: ['Draft replies', 'CRM matching', 'Escalation routing'],
  },
];

const TIMELINE = [
  {
    time: '10:14 AM',
    module: 'Support',
    badge: 'text-emerald-700 bg-emerald-50',
    event: 'Customer email received',
    detail:
      'AI support agent reads the message, categorizes the issue as a billing bug, and drafts a reply.',
  },
  {
    time: '10:14 AM',
    module: 'PM',
    badge: 'text-amber-700 bg-amber-50',
    event: 'Bug story auto-created',
    detail:
      'Story added to the backlog with severity: high. Linked to the support conversation.',
  },
  {
    time: '10:15 AM',
    module: 'CRM',
    badge: 'text-sky-700 bg-sky-50',
    event: 'Churn risk elevated',
    detail:
      'Third ticket in two weeks. Contact health score drops. Account owner gets notified.',
  },
  {
    time: '10:16 AM',
    module: 'Agent',
    badge: 'text-violet-700 bg-violet-50',
    event: 'Engineer agent investigating',
    detail:
      'Agent reads the codebase, identifies the billing calculation error, and opens a fix PR.',
  },
  {
    time: '10:22 AM',
    module: 'Support',
    badge: 'text-emerald-700 bg-emerald-50',
    event: 'Customer notified',
    detail:
      'Fix deployed. AI drafts a follow-up. Customer gets resolution — 8 minutes, not 8 hours.',
  },
];

const REPLACED = [
  'Project management tool',
  'CRM',
  'Support desk',
  'Internal wiki',
  'The integrations between them',
];

// ─────────────────────────────────────────────
// Page
// ─────────────────────────────────────────────

export default function Home() {
  return (
    <>
      <HeroSection />
      <PreviewSection />
      <ModulesSection />
      <AgentsSection />
      <TimelineSection />
      <CTASection />
    </>
  );
}

// ─────────────────────────────────────────────
// Hero
// ─────────────────────────────────────────────

function HeroSection() {
  return (
    <section className="pt-28 pb-16 lg:pt-36 lg:pb-20">
      <div className="mx-auto max-w-7xl px-6 lg:px-8">
        <p className="text-sm font-bold tracking-[0.2em] uppercase text-primary">
          The stack replacement
        </p>

        <h1 className="mt-6 max-w-4xl text-[clamp(2.75rem,6vw,5rem)] font-extrabold leading-[1.08] tracking-tight">
          Your PM, CRM, support, and docs
          — replaced.{' '}
          <br className="hidden lg:block" />
          <span className="font-display italic font-normal">AI&nbsp;agents</span>{' '}
          handle the rest.
        </h1>

        <p className="mt-7 max-w-xl text-[19px] leading-[1.65] text-muted-foreground">
          Stop duct-taping five tools together. Helpin is one connected system
          for everything your team runs on. AI agents — planners, engineers,
          reviewers, support reps — execute the work automatically.
        </p>

        <div className="mt-10 flex flex-wrap items-center gap-4">
          <Link
            href="https://helpin.ai/login"
            className="inline-flex items-center rounded-lg bg-foreground px-6 py-3 text-[15px] font-semibold text-background transition-opacity hover:opacity-85"
          >
            Start free
          </Link>
          <Link
            href="/features"
            className="inline-flex items-center text-[15px] font-semibold text-foreground transition-colors hover:text-primary"
          >
            See how it works
            <span className="ml-1.5 text-muted-foreground">→</span>
          </Link>
        </div>

        {/* Replaces strip */}
        <div className="mt-16 flex flex-wrap items-center gap-x-5 gap-y-2">
          <span className="text-[13px] font-semibold tracking-wide uppercase text-foreground/30">
            Replaces your
          </span>
          {REPLACED.map((tool) => (
            <span
              key={tool}
              className="text-[15px] text-muted-foreground/60 line-through decoration-primary/50 decoration-[1.5px]"
            >
              {tool}
            </span>
          ))}
        </div>
      </div>
    </section>
  );
}

// ─────────────────────────────────────────────
// Product Preview (CSS-only mockup)
// ─────────────────────────────────────────────

function PreviewSection() {
  return (
    <section className="pb-24 lg:pb-32">
      <div className="mx-auto max-w-6xl px-6 lg:px-8">
        <div className="overflow-hidden rounded-xl border border-border shadow-2xl shadow-foreground/[0.04]">
          {/* Browser chrome */}
          <div className="flex items-center gap-2 border-b border-border/60 bg-muted/60 px-4 py-2.5">
            <div className="flex gap-1.5">
              <div className="h-2.5 w-2.5 rounded-full bg-foreground/[0.08]" />
              <div className="h-2.5 w-2.5 rounded-full bg-foreground/[0.08]" />
              <div className="h-2.5 w-2.5 rounded-full bg-foreground/[0.08]" />
            </div>
            <div className="mx-auto rounded-md bg-background/80 px-10 py-1 text-[11px] text-muted-foreground">
              helpin.ai
            </div>
            <div className="w-14" />
          </div>

          {/* App frame */}
          <div className="flex min-h-[300px] lg:min-h-[400px]">
            {/* Sidebar */}
            <div className="hidden sm:flex w-40 shrink-0 flex-col border-r border-border/50 bg-muted/30 p-3">
              <div className="px-2 text-[11px] font-extrabold tracking-tight">
                helpin
              </div>
              <nav className="mt-5 space-y-0.5">
                <SidebarItem label="Projects" active />
                <SidebarItem label="CRM" />
                <SidebarItem label="Support" />
                <SidebarItem label="Docs" />
                <SidebarItem label="Agents" />
              </nav>
              <div className="mt-auto">
                <SidebarItem label="Settings" />
              </div>
            </div>

            {/* Main content */}
            <div className="flex-1 bg-background p-4 lg:p-5">
              <div className="mb-4 flex items-center justify-between">
                <div className="text-[11px] font-bold">My Work</div>
                <div className="flex gap-1">
                  <div className="rounded bg-foreground/[0.06] px-2 py-0.5 text-[9px] font-medium text-muted-foreground">
                    Assigned
                  </div>
                  <div className="rounded px-2 py-0.5 text-[9px] text-muted-foreground/50">
                    Requested
                  </div>
                </div>
              </div>

              <div className="grid grid-cols-3 gap-2.5">
                <KanbanColumn
                  title="Backlog"
                  count={3}
                  cards={[
                    { dot: 'bg-amber-400', widths: [72, 88, 52] },
                    { dot: 'bg-sky-400', widths: [56, 78] },
                    { dot: 'bg-foreground/15', widths: [64, 84, 42] },
                  ]}
                />
                <KanbanColumn
                  title="In Progress"
                  count={2}
                  cards={[
                    { dot: 'bg-rose-400', widths: [60, 74, 48] },
                    { dot: 'bg-amber-400', widths: [52, 88] },
                  ]}
                />
                <KanbanColumn
                  title="Done"
                  count={4}
                  cards={[
                    { dot: 'bg-emerald-400', widths: [66, 72] },
                    { dot: 'bg-emerald-400', widths: [54, 80, 44] },
                    { dot: 'bg-emerald-400', widths: [70, 62] },
                  ]}
                />
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}

function SidebarItem({ label, active }: { label: string; active?: boolean }) {
  return (
    <div
      className={`rounded-md px-2.5 py-1.5 text-[11px] ${
        active
          ? 'bg-primary/10 font-semibold text-primary'
          : 'text-muted-foreground'
      }`}
    >
      {label}
    </div>
  );
}

function KanbanColumn({
  title,
  count,
  cards,
}: {
  title: string;
  count: number;
  cards: { dot: string; widths: number[] }[];
}) {
  return (
    <div>
      <div className="mb-2 flex items-center gap-1.5">
        <div className="text-[9px] font-semibold uppercase tracking-wider text-muted-foreground">
          {title}
        </div>
        <span className="text-[9px] text-muted-foreground/40">{count}</span>
      </div>
      <div className="space-y-1.5">
        {cards.map((card, i) => (
          <MockCard key={i} dot={card.dot} widths={card.widths} />
        ))}
      </div>
    </div>
  );
}

function MockCard({ dot, widths }: { dot: string; widths: number[] }) {
  return (
    <div className="rounded-lg border border-border/30 bg-muted/20 p-2 space-y-1">
      <div className="flex items-center gap-1.5">
        <div className={`h-1.5 w-1.5 rounded-full ${dot}`} />
        <div
          className="h-[5px] rounded-sm bg-foreground/10"
          style={{ width: `${widths[0]}%` }}
        />
      </div>
      {widths.slice(1).map((w, i) => (
        <div
          key={i}
          className="h-[4px] rounded-sm bg-foreground/[0.04]"
          style={{ width: `${w}%` }}
        />
      ))}
    </div>
  );
}

// ─────────────────────────────────────────────
// Modules
// ─────────────────────────────────────────────

function ModulesSection() {
  return (
    <section className="py-24 lg:py-32 bg-muted">
      <div className="mx-auto max-w-7xl px-6 lg:px-8">
        <div className="max-w-2xl">
          <h2 className="text-[clamp(2rem,4vw,3rem)] font-bold tracking-tight">
            Four tools replaced. Connected by default.
          </h2>
          <p className="mt-5 text-[19px] leading-[1.65] text-muted-foreground">
            Every module is a full replacement — not a stripped-down version.
            And because they live in one system, context flows between them
            without integrations, webhooks, or duct tape.
          </p>
        </div>

        <div className="mt-16 grid gap-3 sm:grid-cols-2">
          {/* PM */}
          <div className="rounded-2xl bg-background p-6 lg:p-8">
            <span className="inline-flex items-center rounded-full bg-amber-100 px-3 py-1 text-xs font-semibold text-amber-700">
              Project Management
            </span>
            <h3 className="mt-4 text-[22px] font-bold leading-snug">
              Stories, sprints, epics, objectives — the whole system
            </h3>
            <p className="mt-3 text-base leading-relaxed text-muted-foreground">
              Kanban boards. Sprint planning with velocity tracking. Custom
              workflows per team. Estimate scales (fibonacci, t-shirt, hours).
              Roadmap objectives. Not a simplified task list — the full PM
              system your engineering team actually needs.
            </p>
            <div className="mt-6 grid grid-cols-3 gap-2">
              <MiniColumn title="Backlog" color="bg-amber-50 border-amber-200/40" count={2} />
              <MiniColumn title="Started" color="bg-sky-50 border-sky-200/40" count={1} />
              <MiniColumn title="Done" color="bg-emerald-50 border-emerald-200/40" count={3} />
            </div>
          </div>

          {/* CRM */}
          <div className="rounded-2xl bg-background p-6 lg:p-8">
            <span className="inline-flex items-center rounded-full bg-sky-100 px-3 py-1 text-xs font-semibold text-sky-700">
              CRM
            </span>
            <h3 className="mt-4 text-[22px] font-bold leading-snug">
              A sales pipeline that reads your email and moves itself
            </h3>
            <p className="mt-3 text-base leading-relaxed text-muted-foreground">
              Contacts, companies, deals, and pipeline stages. But unlike your
              current CRM, this one syncs with Gmail, detects 7 types of buying
              signals via AI, and auto-creates deals when confidence is high.
              You close — it handles the pipeline.
            </p>
            <div className="mt-6 flex items-end gap-1">
              {[
                { h: 56, label: 'Lead' },
                { h: 42, label: 'Qualified' },
                { h: 30, label: 'Proposal' },
                { h: 20, label: 'Negotiation' },
                { h: 12, label: 'Won' },
              ].map((s) => (
                <div key={s.label} className="flex-1 text-center">
                  <div
                    className="mx-auto w-full rounded-t bg-sky-400/15"
                    style={{ height: s.h }}
                  />
                  <div className="mt-1.5 text-[8px] text-muted-foreground/50">
                    {s.label}
                  </div>
                </div>
              ))}
            </div>
          </div>

          {/* Support */}
          <div className="rounded-2xl bg-background p-6 lg:p-8">
            <span className="inline-flex items-center rounded-full bg-emerald-100 px-3 py-1 text-xs font-semibold text-emerald-700">
              Support
            </span>
            <h3 className="mt-4 text-[22px] font-bold leading-snug">
              Customer support where AI writes the first draft
            </h3>
            <p className="mt-3 text-base leading-relaxed text-muted-foreground">
              Conversation threads, ticket routing, and AI-drafted replies.
              Every ticket auto-matches to a CRM contact — so you see the
              customer&apos;s full history, open deals, and risk score before
              typing a word. Your team handles escalations. AI handles volume.
            </p>
          </div>

          {/* Docs */}
          <div className="rounded-2xl bg-background p-6 lg:p-8">
            <span className="inline-flex items-center rounded-full bg-violet-100 px-3 py-1 text-xs font-semibold text-violet-700">
              Docs
            </span>
            <h3 className="mt-4 text-[22px] font-bold leading-snug">
              A knowledge base your agents actually read
            </h3>
            <p className="mt-3 text-base leading-relaxed text-muted-foreground">
              Specs, decisions, runbooks — organized in spaces and collections.
              Linked to epics and stories. When a planner agent breaks down an
              epic, it reads your docs first. When a support agent drafts a
              reply, it checks your knowledge base. Not a static wiki — a
              living reference.
            </p>
          </div>
        </div>
      </div>
    </section>
  );
}

function MiniColumn({
  title,
  color,
  count,
}: {
  title: string;
  color: string;
  count: number;
}) {
  return (
    <div>
      <div className="text-[8px] font-semibold text-muted-foreground/50 uppercase tracking-wider mb-1.5">
        {title}
      </div>
      <div className="space-y-1">
        {Array.from({ length: count }).map((_, i) => (
          <div key={i} className={`h-5 rounded border ${color}`} />
        ))}
      </div>
    </div>
  );
}

// ─────────────────────────────────────────────
// AI Agents
// ─────────────────────────────────────────────

function AgentsSection() {
  return (
    <section className="py-24 lg:py-32">
      <div className="mx-auto max-w-7xl px-6 lg:px-8">
        <div className="max-w-2xl">
          <h2 className="text-[clamp(2rem,4vw,3rem)] font-bold tracking-tight">
            Not assistants. Workers.
          </h2>
          <p className="mt-5 text-[19px] leading-[1.65] text-muted-foreground">
            Helpin&apos;s AI agents aren&apos;t chat widgets you prompt for
            suggestions. They&apos;re specialized workers with defined roles,
            access to your codebase, and configurable autonomy. Assign work
            to them like you would a teammate.
          </p>
        </div>

        <div className="mt-16 grid gap-6 sm:grid-cols-2 lg:grid-cols-4 lg:gap-8">
          {AGENTS.map((agent) => (
            <div key={agent.role}>
              <div
                className={`flex h-12 w-12 items-center justify-center rounded-xl text-base font-bold ${agent.accent}`}
              >
                {agent.initial}
              </div>
              <h3 className="mt-4 text-lg font-bold">{agent.role}</h3>
              <p className="mt-2 text-[15px] leading-[1.65] text-muted-foreground">
                {agent.scenario}
              </p>
              <div className="mt-4 flex flex-wrap gap-1.5">
                {agent.capabilities.map((cap) => (
                  <span
                    key={cap}
                    className="rounded-full bg-muted px-2.5 py-0.5 text-xs font-medium text-muted-foreground"
                  >
                    {cap}
                  </span>
                ))}
              </div>
            </div>
          ))}
        </div>

        <div className="mt-14 rounded-2xl bg-muted p-6 lg:p-8">
          <p className="text-base leading-[1.65] text-muted-foreground">
            <span className="font-semibold text-foreground">
              You stay in control.
            </span>{' '}
            Set trigger modes — manual, on assignment, or on event. Choose
            autonomy levels — always require human review, or let agents
            auto-execute above a confidence threshold. Pick which tools each
            agent can access. Configure once, run continuously.
          </p>
        </div>
      </div>
    </section>
  );
}

// ─────────────────────────────────────────────
// Timeline (replaces abstract Connected flows)
// ─────────────────────────────────────────────

function TimelineSection() {
  return (
    <section className="py-24 lg:py-32 bg-muted">
      <div className="mx-auto max-w-7xl px-6 lg:px-8">
        <div className="max-w-2xl">
          <h2 className="text-[clamp(2rem,4vw,3rem)] font-bold tracking-tight">
            What this looks like in practice
          </h2>
          <p className="mt-5 text-[19px] leading-[1.65] text-muted-foreground">
            A customer emails about a billing bug. Here&apos;s what happens
            next — across support, PM, CRM, and engineering — without a
            single human copying data between tools.
          </p>
        </div>

        <div className="mt-16">
          <div className="space-y-0">
            {TIMELINE.map((event, i) => (
              <div
                key={i}
                className="relative grid grid-cols-[60px_1fr] gap-4 pb-8 lg:grid-cols-[80px_1fr] lg:gap-6"
              >
                {/* Vertical line */}
                {i < TIMELINE.length - 1 && (
                  <div className="absolute left-[29px] top-8 bottom-0 w-px bg-border/60 lg:left-[39px]" />
                )}

                {/* Time */}
                <div className="pt-1 text-right">
                  <span className="text-sm font-semibold tabular-nums text-foreground/40">
                    {event.time}
                  </span>
                </div>

                {/* Event */}
                <div className="rounded-xl bg-background p-4 lg:p-5">
                  <div className="flex items-center gap-2.5 mb-2">
                    <span
                      className={`inline-flex rounded-md px-2 py-0.5 text-xs font-bold ${event.badge}`}
                    >
                      {event.module}
                    </span>
                    <span className="text-base font-semibold">
                      {event.event}
                    </span>
                  </div>
                  <p className="text-[15px] leading-relaxed text-muted-foreground">
                    {event.detail}
                  </p>
                </div>
              </div>
            ))}
          </div>

          {/* Summary */}
          <div className="mt-4 text-center">
            <p className="text-lg font-semibold text-foreground">
              8 minutes. Four modules. Zero tab-switching.
            </p>
            <p className="mt-1 text-[15px] text-muted-foreground">
              No human copied data between tools. No one waited for a handoff.
              The system handled it.
            </p>
          </div>
        </div>
      </div>
    </section>
  );
}

// ─────────────────────────────────────────────
// Bottom CTA
// ─────────────────────────────────────────────

function CTASection() {
  return (
    <section className="py-24 lg:py-32 bg-foreground">
      <div className="mx-auto max-w-7xl px-6 text-center lg:px-8">
        <h2 className="text-[clamp(2rem,4.5vw,3.25rem)] font-bold tracking-tight text-background">
          Five tools. Five bills.
          <br />
          Five places where context dies.
        </h2>
        <p className="mx-auto mt-5 max-w-lg text-[19px] leading-[1.65] text-background/50">
          Or one platform where PM, CRM, support, and docs are connected —
          and AI agents handle the execution. Free to start. No credit card.
        </p>
        <div className="mt-10">
          <Link
            href="https://helpin.ai/login"
            className="inline-flex items-center rounded-lg bg-primary px-7 py-3.5 text-[15px] font-semibold text-primary-foreground transition-opacity hover:opacity-90"
          >
            Get started free
          </Link>
        </div>
      </div>
    </section>
  );
}
