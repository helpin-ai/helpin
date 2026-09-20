import type { Metadata } from 'next';
import Link from 'next/link';
import { ArrowRight, CheckSquare, ChevronRight, GitBranch, ListFilter, MessagesSquare, Repeat2, Tags, Users } from 'lucide-react';
import { PreviewNav } from '../../_components/PreviewNav';
import { PreviewFooter } from '../../_components/PreviewFooter';
import { ConnectedWorkspace } from '../../_components/ConnectedWorkspace';
import { CtaRow, SectionHead } from '../../_components/ui';
import { ProjectScene } from './project-scenes';
import { ProjectHero } from './project-hero';
import { ProjectHealth } from './project-health';
import { ProjectDelivery } from './project-delivery';
import './projects.css';
import './project-outcomes.css';
import './projects-polish.css';

export const metadata: Metadata = {
  title: 'Projects — Helpin',
  description: 'Plan the right work, spot blockers, and connect delivery to customer outcomes. Bring tasks, sprints, objectives, and agents together in Helpin Projects.',
  alternates: { canonical: '/new/products/projects' },
  robots: { index: false, follow: false },
};
const DETAILS = [
  { Icon: ListFilter, title: 'Save the views you work from.', body: 'Filter by team, owner, priority, or labels. Save a view so the work you need is easy to find.' },
  { Icon: GitBranch, title: 'Make blockers visible.', body: 'Connect related tasks, mark dependencies, and see what needs to happen before the next step.' },
  { Icon: Users, title: 'Keep ownership clear.', body: 'Assign teammates, set priorities and due dates, and give agents a defined task to work on.' },
  { Icon: CheckSquare, title: 'Put the details in the task.', body: 'Keep descriptions, checklists, attachments, and linked docs beside the work they explain.' },
  { Icon: Repeat2, title: 'Give recurring work a home.', body: 'Use task templates and recurring schedules for the work your team comes back to.' },
  { Icon: Tags, title: 'Match the way your team works.', body: 'Organize bugs, features, and chores with labels and workflow states that fit your process.' },
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
          <div className="projects-hero-grid"><div className="projects-hero-copy"><span className="eyebrow">Product & project management</span><h1 id="projects-title">Plan the right work.<br /><span>Ship it with context.</span></h1><p className="lede">Turn customer requests into clear priorities, focused sprints, and reviewed changes. Keep your team and agents connected to the people waiting for the work.</p><CtaRow secondaryHref="#project-tasks" secondaryLabel="Explore Projects" /><div className="projects-hero-points"><span><MessagesSquare size={14} />Prioritize with context</span><span><CheckSquare size={14} />Keep delivery moving</span><span><GitBranch size={14} />Review with your team</span></div></div>
          <ProjectHero /></div>
        </div>
      </section>
      <nav className="projects-page-nav" aria-label="On this page"><div className="wrap"><strong>Projects</strong><a href="#project-context">Prioritize</a><a href="#project-tasks">Organize</a><a href="#project-planning">Plan</a><a href="#project-progress">Track outcomes</a><a href="#project-agents">Deliver</a><a href="#project-faq">FAQs</a></div></nav>
      <section id="project-context"><div className="wrap projects-split"><div><SectionHead eyebrow="Choose the work that matters" title="Prioritize with the customer in view." lede="A request is easier to judge when you know who needs it and what is getting in their way. Keep the conversation, customer, and requirements attached as you decide what to build." /><div className="projects-context-points"><div><span>01</span><p><strong>Understand the need.</strong> Read the original request, without losing the details in a handoff.</p></div><div><span>02</span><p><strong>Connect the relationship.</strong> Link the contact, company, or deal behind the work.</p></div><div><span>03</span><p><strong>Make a clear commitment.</strong> Set the priority, assign an owner, and define the next step.</p></div></div></div><ProjectScene variant="context" /></div></section>
      <section id="project-tasks" className="projects-tasks"><div className="wrap"><SectionHead eyebrow="The work in front of you" title="Make the next step obvious." lede="See who owns each task, what is moving, and what needs attention. Keep Ask Agent beside the board to summarize progress and bring the next step into focus." /><figure className="projects-board-agent-image"><img src="/new/projects/board-ask-agent-dark-1920-v2.webp" srcSet="/new/projects/board-ask-agent-dark-960-v2.webp 960w, /new/projects/board-ask-agent-dark-1440-v2.webp 1440w, /new/projects/board-ask-agent-dark-1920-v2.webp 1920w, /new/projects/board-ask-agent-dark-3840-v2.webp 3840w" sizes="(max-width: 1240px) calc(100vw - 48px), 1192px" width={3840} height={2160} loading="lazy" alt="Illustrative OrbitDesk task board in dark mode with Ask Agent alongside it. Alex is working on webhook delivery and API retries; Sam’s export fix and Jules’s filter fix are in review. The summary links to those same tasks and identifies Maya’s Slack alert request as planned." /></figure><div className="projects-details">{DETAILS.map(({ Icon, title, body }) => <article key={title}><Icon size={20} aria-hidden="true" /><h3>{title}</h3><p>{body}</p></article>)}</div></div></section>
      <section id="project-planning" className="projects-planning"><div className="wrap"><SectionHead eyebrow="From this sprint to the bigger picture" title="Turn a long backlog into a plan people can follow." lede="Break bigger efforts into epics, decide what belongs in the next sprint, and put the work on a roadmap tied to your objectives." /><div className="projects-planning-grid"><article><div className="projects-planning-copy"><span className="projects-planning-step">01 / SPRINTS</span><h3>Finish the sprint with a clear handoff.</h3><p>Bring in work from the backlog, track progress, and carry unfinished tasks into the next sprint with the closeout recorded.</p></div><ProjectScene variant="sprint" /></article><article><div className="projects-planning-copy"><span className="projects-planning-step">02 / ROADMAP</span><h3>Give everyone a view of what’s coming.</h3><p>Put scheduled epics on a roadmap. Group them by team or objective, track health, and keep the larger goal in view.</p></div><ProjectScene variant="roadmap" /></article></div></div></section>
      <section id="project-progress"><div className="wrap projects-split projects-progress-grid"><div><SectionHead eyebrow="Progress you can act on" title="See what’s moving and what needs attention." lede="Connect epics to objectives and track key results alongside delivery. Keep target dates, project health, and the latest update visible when plans change." /><div className="projects-progress-points"><p><strong>Keep the outcome in sight.</strong> Track a measurable result alongside completed tasks.</p><p><strong>Surface the risk.</strong> Use health updates and task dependencies to show where help is needed.</p><p><strong>Make the update useful.</strong> Keep the owner, next step, and supporting context together.</p></div><p className="projects-demo-hint">Select an epic to explore its progress and next step.</p></div><ProjectHealth /></div></section>
      <section id="project-agents" className="projects-agents"><div className="wrap projects-split"><div><SectionHead eyebrow="Your team and agents, working together" title="Give your agents work your team can review." lede="Turn an idea into a plan with Atlas, refine the tasks with Scribe, and let Forge prepare code and tests. Lens brings review findings back to the work." /><p className="projects-agent-copy">Follow agent runs, branches, commits, and pull requests from the task. Keep tool access and approvals explicit, with your team deciding what is ready to ship.</p><Link className="projects-inline-link" href="/new/products/ai-agents">Meet the agents<ArrowRight size={15} /></Link></div><ProjectScene variant="agents" /></div></section>
      <section className="projects-loop"><div className="wrap projects-delivery-grid"><div><SectionHead eyebrow="Delivery is a customer moment" title="Ship the change. Close the conversation." lede="Give support more than a task marked done. Return to the linked conversation with the change, the setup guide, and the context to explain what is ready." /><p className="projects-agent-copy">Prepare the follow-up with your team or Ask Agent, then review it before sending. The customer gets an answer that reflects the work.</p><Link className="projects-inline-link" href="/new/products/customer-support">See how support connects<ArrowRight size={15} /></Link></div><ProjectDelivery /></div></section>
      <section id="project-faq"><div className="wrap projects-faq-grid"><SectionHead eyebrow="A few useful answers" title="Make room for the way your team works." /><div className="projects-faqs">{FAQS.map(([q,a]) => <details key={q}><summary>{q}</summary><p>{a}</p></details>)}</div></div></section>
      <section className="final-cta final-cta-connected" aria-labelledby="projects-final-title"><div className="wrap"><ConnectedWorkspace /><div className="final"><span className="eyebrow">From the request to the work</span><h2 id="projects-final-title">Turn the next customer request into your next shipped improvement.</h2><p className="lede">Bring the customer context, the plan, and the people and agents doing the work into one workspace.</p><CtaRow /><p className="projects-edition">Projects is available in the full Helpin workspace.</p></div></div></section>
    </div>
    <PreviewFooter />
  </>;
}
