import type { Metadata } from 'next';
import Link from 'next/link';
import { ArrowRight, CheckSquare, ChevronRight, GitBranch, ListFilter, MessagesSquare, Repeat2, Tags, Users } from 'lucide-react';
import { PreviewNav } from '../../_components/PreviewNav';
import { PreviewFooter } from '../../_components/PreviewFooter';
import { ConnectedWorkspace } from '../../_components/ConnectedWorkspace';
import { CtaRow, SectionHead } from '../../_components/ui';
import { ProjectScene } from './project-scenes';
import { ProjectTaskDemo } from './project-task-demo';
import './projects.css';

export const metadata: Metadata = {
  title: 'Projects — Helpin',
  description: 'Turn customer requests into planned work. Organize tasks, epics, sprints, and objectives, then work with agents to prepare changes with customer context attached.',
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
  ['How do agents help with planning and delivery?', 'Atlas helps turn an idea into a PRD and task plan. Scribe refines individual tasks using specifications and code context. Forge can prepare implementation changes and tests, while Lens reviews the work. Ask Agent can investigate and coordinate work using the tools and permissions available to it.'],
  ['Can we connect engineering work?', 'Yes. Link repository activity such as branches, commits, and pull requests to tasks. Coding agents need a connected repository and configured execution environment. Their work remains subject to the access, delivery, and approval settings you choose.'],
  ['Will customers be notified automatically when a task is done?', 'Linked conversations help your team see who needs an update. Ask Agent can help prepare the follow-up. Sending it depends on your configured tools and workflow; completing a task alone does not mean a customer message was sent.'],
  ['Is Projects included in the Community beta?', 'Projects is part of the full Helpin workspace. The current Community 0.1 beta includes support, docs, and agents; project management, CRM, and repository coding workflows are outside that beta surface.'],
];
export default function ProjectsPage() {
  return <>
    <PreviewNav />
    <main className="projects-page">
      <section className="projects-hero" aria-labelledby="projects-title">
        <div className="wrap">
          <div className="projects-breadcrumb"><Link href="/new">Helpin</Link><ChevronRight size={12} /><span>Projects</span></div>
          <div className="projects-hero-copy"><span className="eyebrow">Product & project management</span><h1 id="projects-title">Turn customer requests into work <span>your team can ship.</span></h1><p className="lede">Plan projects, organize tasks, and work with agents from the first request to the reviewed change—with the customer context attached.</p><CtaRow secondaryHref="#project-tasks" secondaryLabel="Explore Projects" /><div className="projects-hero-points"><span><MessagesSquare size={14} />Customer context</span><span><CheckSquare size={14} />Everyday planning</span><span><GitBranch size={14} />Agents & engineering</span></div></div>
          <figure className="projects-screenshot"><img src="/new/product/workspace-projects-1920-v3.webp" srcSet="/new/product/workspace-projects-960-v3.webp 960w, /new/product/workspace-projects-1920-v3.webp 1920w, /new/product/workspace-projects-4k-v3.webp 3840w" sizes="(max-width: 1280px) calc(100vw - 48px), 1232px" width={3840} height={2160} fetchPriority="high" alt="Illustrative OrbitDesk engineering board with customer-reported bugs, feature requests, documentation, and follow-ups organized across Planned, In Progress, In Review, and Shipped." /></figure>
        </div>
      </section>
      <nav className="projects-page-nav" aria-label="On this page"><div className="wrap"><strong>Projects</strong><a href="#project-context">Customer context</a><a href="#project-tasks">Tasks & views</a><a href="#project-planning">Sprints & roadmap</a><a href="#project-agents">Agents & delivery</a><a href="#project-faq">FAQs</a></div></nav>
      <section id="project-context"><div className="wrap projects-split"><SectionHead eyebrow="Start with the customer" title="Keep the reason for the work attached." lede="Turn a customer request into a task without leaving the conversation behind. Give product and engineering the original question, the account, and the details they need to act." secondaryLede="When someone asks why it matters, the answer is already there." /><ProjectScene variant="context" /></div></section>
      <section id="project-tasks" className="projects-tasks"><div className="wrap"><SectionHead eyebrow="The work in front of you" title="Give every task a clear next step." lede="See work on a board or in a list. Set priorities, assign owners, and keep the blockers and details visible as the work moves." /><ProjectTaskDemo /><div className="projects-details">{DETAILS.map(({ Icon, title, body }) => <article key={title}><Icon size={20} aria-hidden="true" /><h3>{title}</h3><p>{body}</p></article>)}</div></div></section>
      <section id="project-planning" className="projects-planning"><div className="wrap"><SectionHead eyebrow="From this sprint to the bigger picture" title="Connect the work you do today to what comes next." lede="Use sprints to focus the team, epics to hold larger efforts together, and objectives to explain what the work is meant to achieve." /><div className="projects-planning-grid"><article><div className="projects-planning-copy"><h3>Plan a sprint your team can follow.</h3><p>Bring in work from the backlog, track progress, and carry unfinished tasks into the next sprint with the closeout recorded.</p></div><ProjectScene variant="sprint" /></article><article><div className="projects-planning-copy"><h3>See where each effort is heading.</h3><p>Put scheduled epics on a roadmap. Group them by team or objective, track health, and keep the larger goal in view.</p></div><ProjectScene variant="roadmap" /></article></div></div></section>
      <section id="project-agents" className="projects-agents"><div className="wrap projects-split"><div><SectionHead eyebrow="Your team and agents, working together" title="Move from a clear plan to a change worth reviewing." lede="Let Atlas shape the plan and Scribe refine the tasks. Give Forge the repository context to prepare a change, then bring in Lens to review it." /><p className="projects-agent-copy">Follow agent runs and linked engineering activity from the task. Your team can review the work with the customer request still in view.</p><Link className="projects-inline-link" href="/new/products/ai-agents">Meet the agents<ArrowRight size={15} /></Link></div><ProjectScene variant="agents" /></div></section>
      <section className="projects-loop"><div className="wrap"><span className="eyebrow">Close the loop</span><h2>Keep the people waiting for the work in view.</h2><p className="lede">Return to the linked conversation when a fix or feature is ready. Give support the context to prepare an update and tell the customer what changed.</p><Link className="projects-inline-link" href="/new/products/customer-support">See how support connects<ArrowRight size={15} /></Link></div></section>
      <section id="project-faq"><div className="wrap projects-faq-grid"><SectionHead eyebrow="A few useful answers" title="Make room for the way your team works." /><div className="projects-faqs">{FAQS.map(([q,a]) => <details key={q}><summary>{q}</summary><p>{a}</p></details>)}</div></div></section>
      <section className="final-cta final-cta-connected" aria-labelledby="projects-final-title"><div className="wrap"><ConnectedWorkspace /><div className="final"><span className="eyebrow">From the request to the work</span><h2 id="projects-final-title">Give your next project the full customer picture.</h2><p className="lede">Bring the request, the plan, and the team doing the work into one workspace.</p><CtaRow /><p className="projects-edition">Projects is available in the full Helpin workspace.</p></div></div></section>
    </main>
    <PreviewFooter />
  </>;
}
