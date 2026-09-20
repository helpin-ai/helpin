import type { Metadata } from 'next';
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
import './project-hero.css';
import './project-focus.css';
import './planning-scenes.css';
import './project-intake.css';
import './project-objectives.css';

export const metadata: Metadata = {
  title: 'Projects — Helpin',
  description: 'Ship the work your customers are waiting for. Turn requests into priorities, plan sprints, and let agents help prepare changes your team can review.',
  alternates: { canonical: '/new/products/projects' },
  robots: { index: false, follow: false },
};
const DETAILS = [
  { Icon: ListFilter, title: 'Keep your priorities in view.', body: 'Filter by team, owner, priority, or label. Save views for your sprint, incoming bugs, or tasks waiting for review.' },
  { Icon: GitBranch, title: 'Make blockers visible.', body: 'Link related tasks and mark dependencies so your team can see what needs to happen first.' },
  { Icon: Users, title: 'Keep ownership clear.', body: 'Assign an owner, set a due date, and give teammates and agents clear requirements to work from.' },
  { Icon: CheckSquare, title: 'Keep the requirements close.', body: 'Add descriptions, checklists, attachments, and linked docs so the person picking up a task can get started.' },
  { Icon: Repeat2, title: 'Set up the work that repeats.', body: 'Use templates and recurring schedules for release checks, maintenance, and other regular tasks.' },
  { Icon: Tags, title: 'Organize your team’s workflow.', body: 'Organize bugs, features, and chores with labels and workflow states that fit your process.' },
];
const FAQS = [
  ['What can we manage in Helpin Projects?', 'Organize tasks into epics, plan sprints, and connect larger efforts to objectives and key results. Tasks support owners, priorities, due dates, estimates, labels, dependencies, checklists, and links to customer context.'],
  ['Can a customer conversation become a task?', 'Yes. Create or link a task from a support conversation, keeping the original request available to the team doing the work. Tasks can also connect to contacts, companies, deals, and documents.'],
  ['Which views are available?', 'Tasks have board and list views, with filters, grouping, and saved views. The roadmap places scheduled epics on a timeline that can be grouped by objective, team, or epic.'],
  ['How do sprints work?', 'Bring tasks from the backlog into a sprint with a start and end date. Track task and estimate progress, review the closeout, and carry unfinished work into another sprint when needed.'],
  ['Can we track outcomes as well as completed tasks?', 'Yes. Objectives connect to epics and support numeric, percentage, or yes/no key results. Your team can record progress and notes, review delivery progress, and maintain health updates with the context behind them.'],
  ['How do agents help with planning and delivery?', 'Atlas helps turn an idea into a PRD and task plan. Scribe refines individual tasks using specifications and code context. Forge can prepare implementation changes and tests, while Lens reviews the work. Ask Agent can investigate and coordinate work using the tools and permissions available to it.'],
  ['Can we connect engineering work?', 'Yes. Link repository activity such as branches, commits, and pull requests to tasks. Coding agents need a connected repository and configured execution environment. Their work remains subject to the access, delivery, and approval settings you choose.'],
  ['Will customers be notified automatically when a task is done?', 'Linked conversations help your team see who needs an update. Ask Agent can help prepare the follow-up. Sending it depends on your configured tools and workflow; completing a task alone does not mean a customer message was sent.'],
  ['Is Projects included in the Community beta?', 'Projects is part of the full Helpin workspace. The current Community 0.1 beta includes support, docs, and agents; project management, CRM, and repository coding workflows are outside that beta surface.'],
];
export default function ProjectsPage() {
  return <>
    <PreviewNav />
    <div className="projects-page">
      <section className="projects-hero" aria-labelledby="projects-title">
        <div className="wrap">
          <div className="projects-breadcrumb"><Link href="/new">Helpin</Link><ChevronRight size={12} /><span>Projects</span></div>
          <div className="projects-hero-grid"><div className="projects-hero-copy"><span className="eyebrow">Product & project management</span><h1 id="projects-title">Ship the work <br /><span>your customers are waiting for.</span></h1><p className="lede">Turn customer requests into clear priorities. Plan the work, let agents help build and review it, and keep your team in control of what ships.</p><CtaRow secondaryHref="#project-tasks" secondaryLabel="Explore Projects" /><div className="projects-hero-points"><span><MessagesSquare size={14} />Customer-linked tasks</span><span><CheckSquare size={14} />Sprint planning</span><span><GitBranch size={14} />Coding agents</span></div></div>
          <ProjectHero /></div>
        </div>
      </section>
      <nav className="projects-page-nav" aria-label="On this page"><div className="wrap"><strong>Projects</strong><a href="#project-context">Prioritize</a><a href="#project-tasks">Organize</a><a href="#project-progress">Objectives</a><a href="#project-planning">Plan</a><a href="#project-agents">Deliver</a><a href="#project-faq">FAQs</a></div></nav>
      <section id="project-context"><div className="wrap projects-split"><div><SectionHead eyebrow="Start with the customer" title="Turn customer requests into planned work." lede="Create a task from a customer conversation with the request and requirements attached. Set the priority, choose an owner, and bring it into your plan without losing who needs it or why." /><div className="projects-context-points"><div><span>01</span><p><strong>Capture the need.</strong> Use the original conversation to define the problem and what the task needs to deliver.</p></div><div><span>02</span><p><strong>Make a clear commitment.</strong> Review the requirements, set a priority, and assign the work to a team and owner.</p></div><div><span>03</span><p><strong>Keep the customer attached.</strong> Link the task to the conversation and company, so the reason for the work stays in view.</p></div></div></div><ProjectScene variant="context" /></div></section>
      <section id="project-tasks" className="projects-tasks"><div className="wrap"><SectionHead eyebrow="Give every task a path to done" title="See who owns it and what’s blocking it." lede="Move between board and list views to see what is planned, in progress, or ready for review. Ask Agent can summarize the tasks and help your team decide where to focus." /><div className="projects-board-agent-preview"><ProjectFocus /></div><div className="projects-details">{DETAILS.map(({ Icon, title, body }) => <article key={title}><Icon size={20} aria-hidden="true" /><h3>{title}</h3><p>{body}</p></article>)}</div></div></section>
      <section id="project-progress"><div className="wrap"><div className="projects-outcomes-heading"><SectionHead eyebrow="Define what success looks like" title="Keep the outcome in view as the work moves." lede="Set measurable goals, connect the epics that support them, and track delivery alongside the results your team records." /></div><ProjectHealth /></div></section>
      <section id="project-planning" className="projects-planning"><div className="wrap"><SectionHead eyebrow="Make a plan your team can follow" title="Turn the bigger goal into your team’s next steps." lede="Schedule the epics that support your objective, then choose the tasks your team will take on next. Keep the roadmap and sprint plan connected to the work behind them." /><div className="projects-planning-grid"><article><div className="projects-planning-copy"><span className="projects-planning-step">01 / ROADMAP</span><h3>Show how the bigger pieces fit.</h3><p>Schedule epics and group them by objective or team. Keep ownership, timing, and health visible as the plan takes shape.</p></div><ProjectScene variant="roadmap" /></article><article><div className="projects-planning-copy"><span className="projects-planning-step">02 / SPRINTS</span><h3>Decide what your team takes on next.</h3><p>Choose tasks from the backlog, make a sprint commitment, and track what ships. Carry unfinished work forward with its closeout recorded.</p></div><ProjectScene variant="sprint" /></article></div></div></section>

      <section id="project-agents" className="projects-agents"><div className="wrap projects-split"><div><SectionHead eyebrow="From a task to a reviewed change" title="Let agents build. Keep your team in control." lede="Give agents a task with the request and requirements attached. Atlas helps plan it, Scribe refines the details, Forge prepares code and tests, and Lens reviews the changes." /><p className="projects-agent-copy">Follow agent runs, branches, commits, and pull requests from the task. You choose the tools and approvals. Your team decides what is ready to ship.</p><Link className="projects-inline-link" href="/new/products/ai-agents">Meet the agents<ArrowRight size={15} /></Link></div><ProjectScene variant="agents" /></div></section>
      <section id="project-followup" className="projects-loop"><div className="wrap projects-delivery-grid"><div><SectionHead eyebrow="Close the loop" title="Connect shipped work to the customer who asked." lede="The task still points to the original customer request after release. Use that connection to explain what changed and bring the update back to the conversation." /><p className="projects-agent-copy">Ask Agent prepares the update from the linked work. Your team controls approval and sending through the tools and workflow you configure.</p><Link className="projects-inline-link" href="/new/products/customer-support">See how support connects<ArrowRight size={15} /></Link></div><ProjectDelivery /></div></section>
      <section id="project-faq"><div className="wrap projects-faq-grid"><SectionHead eyebrow="Questions about Projects" title="Get to know Helpin Projects." /><div className="projects-faqs">{FAQS.map(([q,a]) => <details key={q}><summary>{q}</summary><p>{a}</p></details>)}</div></div></section>
      <section className="final-cta final-cta-connected" aria-labelledby="projects-final-title"><div className="wrap"><ConnectedWorkspace /><div className="final"><span className="eyebrow">Build what your customers need</span><h2 id="projects-final-title">Turn your next customer request into a shipped improvement.</h2><p className="lede">Bring the request, the plan, and the people and agents delivering it into one workspace.</p><CtaRow /><p className="projects-edition">Projects is available in the full Helpin workspace.</p></div></div></section>
    </div>
    <PreviewFooter />
  </>;
}
