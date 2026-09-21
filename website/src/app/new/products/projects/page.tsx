import { HeroVortex } from '../../_components/HeroVortex';
import { Availability, CtaNote, DEMO_URL, FAQList } from '../../_components/ui';
import { previewMetadata } from '../../_components/preview-metadata';
import Link from 'next/link';
import { ArrowRight, CheckSquare, ChevronRight, GitBranch, ListFilter, MessagesSquare, Repeat2, Tags, Users } from 'lucide-react';
import { PreviewNav } from '../../_components/PreviewNav';
import { PreviewFooter } from '../../_components/PreviewFooter';
import { ConnectedWorkspace } from '../../_components/ConnectedWorkspace';
import { CtaRow, SectionHead } from '../../_components/ui';
import { ProjectScene } from './project-scenes';
import { ProjectHero } from './project-hero';
import { ProjectFocus } from './project-focus';
import { ProjectHealth } from './project-health';
import { ProjectDelivery } from './project-delivery';

import './projects.css';
import './project-outcomes.css';
import './projects-polish.css';
import './project-focus.css';
import './planning-scenes.css';
import './project-intake.css';
import './project-objectives.css';
import './project-palette.css';
import './project-delivery.css';

export const metadata = previewMetadata("Projects \u2014 Helpin", "/new/products/projects");
const DETAILS = [
  { Icon: ListFilter, title: 'Keep your priorities in view.', body: 'Filter by team, owner, priority, or label. Save views for your sprint, incoming bugs, or tasks waiting for review.' },
  { Icon: GitBranch, title: 'Make blockers visible.', body: 'Link related tasks and mark dependencies so your team can see what needs to happen first.' },
  { Icon: Users, title: 'Keep ownership clear.', body: 'Assign an owner, set a due date, and give teammates and agents clear requirements to work from.' },
  { Icon: CheckSquare, title: 'Keep the requirements close.', body: 'Add descriptions, checklists, attachments, and linked docs so the person picking up a task can get started.' },
  { Icon: Repeat2, title: 'Set up the work that repeats.', body: 'Use templates and recurring schedules for release checks, maintenance, and other regular tasks.' },
  { Icon: Tags, title: 'Organize your team’s workflow.', body: 'Organize bugs, features, and chores with labels and workflow states that fit your process.' },
];
const FAQS = [
  [
    "What can we manage in Helpin Projects?",
    "Tasks, epics, sprints, roadmaps, objectives and key results, with owners, deadlines and customer requests attached.",
    "/new/products/projects#project-tasks"
  ],
  [
    "Can a customer conversation become a task?",
    "Yes. Create or link a task from a support conversation, keeping the original request attached.",
    "/new/products/projects#project-context"
  ],
  [
    "Which views are available?",
    "Switch between board and list views, with filters, grouping and saved views. Scheduled epics appear on the roadmap.",
    "/new/products/projects#project-planning"
  ],
  [
    "How do sprints work?",
    "Bring backlog tasks into a sprint, track progress and review the closeout. Carry unfinished work into another sprint when needed.",
    "/new/products/projects#project-planning"
  ],
  [
    "Can we track outcomes as well as completed tasks?",
    "Yes. Connect objectives to epics and record progress against numeric, percentage or yes/no key results.",
    "/new/products/projects#project-progress"
  ],
  [
    "How do agents help with planning and delivery?",
    "Planning agents scope the work, coding agents prepare changes and tests, and code reviewers check the result. Your team controls approvals.",
    "/new/products/ai-agents"
  ],
  [
    "Can we connect engineering work?",
    "Yes. Link GitHub or GitLab activity to tasks, let coding agents prepare changes, and have your team approve what merges.",
    "/new/products/projects#project-agents"
  ],
  [
    "Will customers be notified automatically when a task is done?",
    "Completing a task alone does not send a message. Ask Agent can prepare the follow-up for your team to review and send.",
    "/new/products/projects#project-followup"
  ],
  ["Can we self-host Projects?", "Yes. Projects is part of the open-source product. Run it on your own infrastructure or use Helpin Cloud.", "/new/self-hosting#whats-included"]
] as const;
export default function ProjectsPage() {
  return <>
    <PreviewNav />
    <div className="projects-page">
      <section className="projects-hero motion-hero" aria-labelledby="projects-title"><HeroVortex variant="connections" tone="dark" />
        <div className="wrap">
          <div className="projects-breadcrumb"><Link href="/new">Helpin</Link><ChevronRight size={12} /><span>Projects</span></div>
          <div className="projects-hero-grid"><div className="projects-hero-copy"><Availability category="Projects" /><h1 id="projects-title">Ship the work <br /><span>your customers are waiting for.</span></h1><p className="lede">Turn customer requests into clear priorities. Plan the work, let agents help build and review it, and keep your team in control of what ships.</p><CtaRow secondaryHref={DEMO_URL} secondaryLabel="Book a demo" /><CtaNote trial /><div className="projects-hero-points"><span><MessagesSquare size={14} />Customer-linked tasks</span><span><CheckSquare size={14} />Sprint planning</span><span><GitBranch size={14} />Coding agents</span></div></div>
          <div><ProjectHero /></div></div>
        </div>
      </section>
      <nav className="projects-page-nav" aria-label="On this page"><div className="wrap"><strong>Projects</strong><a href="#project-context">Prioritize</a><a href="#project-tasks">Organize</a><a href="#project-progress">Objectives</a><a href="#project-planning">Plan</a><a href="#project-agents">Deliver</a><a href="#project-faq">FAQs</a></div></nav>
      <section id="project-context"><div className="wrap projects-split"><div><SectionHead eyebrow="Start with the customer" title="Turn customer requests into planned work." lede="Create a task from a customer conversation with the request and requirements attached. Set the priority, choose an owner, and bring it into your plan without losing who needs it or why." /><div className="projects-context-points"><div><span>01</span><p><strong>Capture the need.</strong> Use the original conversation to define the problem and what the task needs to deliver.</p></div><div><span>02</span><p><strong>Make a clear commitment.</strong> Review the requirements, set a priority, and assign the work to a team and owner.</p></div><div><span>03</span><p><strong>Keep the customer attached.</strong> Link the task to the conversation and company, so the reason for the work stays in view.</p></div></div></div><ProjectScene variant="context" /></div></section>
      <section id="project-tasks" className="projects-tasks"><div className="wrap"><SectionHead eyebrow="Give every task a path to done" title="See who owns it and what’s blocking it." lede="Move between board and list views to see what is planned, in progress, or ready for review. Ask Agent can summarize the tasks and help your team decide where to focus." /><div className="projects-board-agent-preview"><ProjectFocus /></div><div className="projects-details">{DETAILS.map(({ Icon, title, body }) => <article key={title}><Icon size={20} aria-hidden="true" /><h3>{title}</h3><p>{body}</p></article>)}</div></div></section>
      <section id="project-progress"><div className="wrap"><div className="projects-outcomes-heading"><SectionHead eyebrow="Define what success looks like" title="Track outcomes, not just tickets." lede="Set measurable goals, connect the epics that support them, and track delivery alongside the results your team records." /></div><ProjectHealth /></div></section>
      <section id="project-planning" className="projects-planning"><div className="wrap"><SectionHead eyebrow="Make a plan your team can follow" title="Roadmap and sprints, connected to the same tasks." lede="Schedule the epics that support your objective, then choose the tasks your team will take on next. Keep the roadmap and sprint plan connected to the work behind them." /><div className="projects-planning-grid"><article><div className="projects-planning-copy"><span className="projects-planning-step">01 / ROADMAP</span><h3>Show how the bigger pieces fit.</h3><p>Schedule epics and group them by objective or team. Keep ownership, timing, and health visible as the plan takes shape.</p></div><ProjectScene variant="roadmap" /></article><article><div className="projects-planning-copy"><span className="projects-planning-step">02 / SPRINTS</span><h3>Decide what your team takes on next.</h3><p>Choose tasks from the backlog, make a sprint commitment, and track what ships. Carry unfinished work forward with its closeout recorded.</p></div><ProjectScene variant="sprint" /></article></div></div></section>

      <section id="project-agents" className="projects-agents"><div className="wrap projects-split"><div><SectionHead eyebrow="From a task to a reviewed change" title="Agents write the code. Your team reviews it before it ships." lede="The planning agent scopes it, the coding agent writes the change and tests, and the code reviewer checks it. Your team approves the merge." /><p className="projects-agent-copy">Follow every step from the task, down to the code change.</p><Link className="projects-inline-link" href="/new/products/ai-agents">Meet the agents<ArrowRight size={15} /></Link></div><ProjectScene variant="agents" /></div></section>
      <section id="project-followup" className="projects-loop"><div className="wrap projects-delivery-stage"><div className="projects-delivery-heading"><SectionHead eyebrow="Close the loop" title="Connect shipped work to the customer who asked." lede="Keep the released change, the updated guide, and the customer’s follow-up attached to the original request." /><p className="projects-agent-copy">Your team reviews the release, documentation, and customer update. Approval and sending follow the tools and workflow you configure.</p></div><ProjectDelivery /><Link className="projects-inline-link" href="/new/products/customer-support">See how support connects<ArrowRight size={15} /></Link></div></section>
      <section id="project-faq"><div className="wrap projects-faq-grid"><SectionHead eyebrow="Questions" title="Get to know Helpin Projects." /><div className="projects-faqs"><FAQList items={FAQS} className="faq-items" /></div></div></section>
      <section className="final-cta final-cta-connected" aria-labelledby="projects-final-title"><div className="wrap"><ConnectedWorkspace /><div className="final"><span className="eyebrow">Build what your customers need</span><h2 id="projects-final-title">From request to release, without losing who asked.</h2><p className="lede">Bring the request, the plan, and the people and agents delivering it into one workspace.</p><CtaRow secondaryHref={DEMO_URL} secondaryLabel="Book a demo" /><CtaNote trial /></div></div></section>
    </div>
    <PreviewFooter />
  </>;
}
