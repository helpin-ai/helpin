import { HeroVortex } from '../../_components/HeroVortex';
import { DEMO_URL, FAQList } from '../../_components/ui';
import { createPageMetadata, PAGE_SEO } from '@/lib/metadata';
import Link from 'next/link';
import { ArrowRight, CheckSquare, GitBranch, ListFilter, Repeat2, Tags, Users } from 'lucide-react';
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

export const metadata = createPageMetadata(PAGE_SEO.projects);
const DETAILS = [
  { Icon: ListFilter, title: 'Views that fit the job', body: 'Save filters for a sprint, bug queue, or review list, and share them with the team.' },
  { Icon: GitBranch, title: 'Dependencies in view', body: 'Mark tasks as blocking, related, or duplicate. A blocked task shows what it waits on.' },
  { Icon: Users, title: 'Owners and requesters', body: 'Set one or more owners, a requester, and a due date on every task.' },
  { Icon: CheckSquare, title: 'Requirements within reach', body: 'Keep descriptions, checklists, files, and docs with the task.' },
  { Icon: Repeat2, title: 'Repeatable work', body: 'Use templates and recurrence for routine checks and maintenance.' },
  { Icon: Tags, title: 'Your workflow', body: 'Define your own workflow states and labels.' },
];
const FAQS = [
  [
    "What can we manage in Helpin Projects?",
    "Product development, customer requests, bugs, maintenance, and internal initiatives. Use tasks, epics, sprints, roadmaps, and objectives to organize the work.",
    "/products/projects#project-tasks"
  ],
  [
    "Can a customer conversation become a task?",
    "Yes. Create new work or link the conversation to an existing task. Keep the request available without creating a duplicate item for work already underway.",
    "/products/projects#project-context"
  ],
  [
    "Which views are available?",
    "Board, list, and roadmap views.",
    "/products/projects#project-planning"
  ],
  [
    "How do sprints work?",
    "Select backlog tasks, track the sprint, review its closeout, and carry unfinished work forward.",
    "/products/projects#project-planning"
  ],
  [
    "Can we track outcomes as well as completed tasks?",
    "Yes. Record key-result progress separately from delivery progress.",
    "/products/projects#project-progress"
  ],
  [
    "How do agents help with planning and delivery?",
    "Planning agents develop the scope. Coding agents prepare implementation work and tests, while review agents examine the changes. Tool access and approval settings determine how they proceed. Coding agents need a connected repository and Agent Runtime, on Helpin Cloud or self-hosted.",
    "/products/ai-agents"
  ],
  [
    "Can we connect engineering work?",
    "Yes. Connect GitHub or GitLab activity to tasks. Coding agents work in connected repositories, with changes subject to your review process.",
    "/products/projects#project-agents"
  ],
  [
    "Will customers be notified automatically when a task is done?",
    "No. After release, Helpin prepares an update in the customer’s original conversation. A teammate approves it before it sends.",
    "/products/projects#project-followup"
  ],
  ["Can we self-host Projects?", "Yes. Projects is included when you self-host, coding agents too, free under AGPL-3.0 with no plan limits. Your team runs the installation and covers hosting and model provider costs. Community is a 0.2 beta.", "/self-hosting#whats-included"]
] as const;
export default function ProjectsPage() {
  return <>
    <PreviewNav tone="dark" />
    <div className="projects-page">
      <section className="projects-hero motion-hero" aria-labelledby="projects-title"><HeroVortex variant="connections" tone="dark" />
        <div className="wrap">

          <div className="projects-hero-grid"><div className="projects-hero-copy"><span className="eyebrow">Project management for SaaS teams</span><h1 id="projects-title">AI agents do the work.<br /><span>Your team reviews it.</span></h1><p className="lede">Tasks, stories, epics, and sprints for the work your team needs to ship. AI agents can plan changes, write code, and review pull requests. Keep the roadmap and decisions in view.</p><CtaRow primaryLabel="Start free trial" secondaryHref={DEMO_URL} secondaryLabel="Book a demo" /><p className="projects-supporting-note">14-day free trial · No card required</p></div>
          <div><ProjectHero /><p className="projects-demo-caption">A customer request becomes a task, an agent prepares the fix, and your team reviews it.</p></div></div>
        </div>
      </section>

      <section id="project-context"><div className="wrap projects-split"><div><SectionHead eyebrow="Start with the need" title="Turn a request into a clear task." lede="Ask Agent to turn a bug report or meeting action into work, or link an existing task. The Atlas agent can help shape the scope before the team commits to it." /><div className="projects-context-points"><div><span>01</span><p><strong>Create or link.</strong> Turn a conversation into a new task, or link it to one already underway.</p></div><div><span>02</span><p><strong>Ask Agent drafts it.</strong> It proposes the scope, team, priority, owner, and sprint for you to approve.</p></div><div><span>03</span><p><strong>The source stays attached.</strong> The conversation and the company stay linked to the task.</p></div></div></div><div><ProjectScene variant="context" /></div></div></section>
      <section id="project-tasks" className="projects-tasks section-motion"><HeroVortex variant="converge" tone="dark" /><div className="wrap"><SectionHead eyebrow="The everyday work" title="Keep people and agents working from the same plan." lede="Organize tasks and stories for new features, bugs, maintenance, and internal projects. Switch between board and list views, with owners, priorities, and blockers close by." /><div className="projects-board-agent-preview"><ProjectFocus /></div><div className="projects-details">{DETAILS.map(({ Icon, title, body }) => <article key={title}><Icon size={20} aria-hidden="true" /><h3>{title}</h3><p>{body}</p></article>)}</div></div></section>
      <section id="project-progress"><div className="wrap"><div className="projects-outcomes-heading"><SectionHead eyebrow="Know what success means" title="See whether the work is helping you reach the goal." lede="Link objectives to epics and key results. Track what shipped alongside the outcome you wanted, so completed tasks do not become the only measure of success." /></div><ProjectHealth /></div></section>
      <section id="project-planning" className="projects-planning"><div className="wrap"><SectionHead eyebrow="From the bigger plan to the next sprint" title="Make the bigger plan. Choose the next sprint." lede="Schedule epics by objective on the roadmap. Plan sprints from the backlog. Unfinished work rolls into the next sprint automatically." /><div className="projects-planning-grid"><article><div className="projects-planning-copy"><span className="projects-planning-step">01 / ROADMAP</span><h3>See how the bigger pieces fit.</h3><p>Schedule epics around objectives, with owners, timing, and health in view.</p></div><ProjectScene variant="roadmap" /></article><article><div className="projects-planning-copy"><span className="projects-planning-step">02 / SPRINTS</span><h3>Make the next commitment clear.</h3><p>Choose backlog tasks, review the sprint closeout, and carry unfinished work into the next sprint.</p></div><ProjectScene variant="sprint" /></article></div></div></section>

      <section id="project-agents" className="projects-agents"><div className="wrap projects-split"><div><SectionHead eyebrow="From a defined task to a proposed change" title="AI agents plan and code. Your team reviews." lede="Scribe works through the task and codebase. Forge writes the change and tests. Lens checks the result. Start them from a task or a configured automation, with the approvals you choose." /><p className="projects-agent-copy">Coding agents need a connected repository and Agent Runtime, on Helpin Cloud or self-hosted.</p><Link prefetch={false} className="projects-inline-link" href="/products/ai-agents">Meet the agents<ArrowRight size={15} /></Link></div><div><ProjectScene variant="agents" /></div></div></section>
      <section id="project-followup" className="projects-loop section-motion"><HeroVortex variant="converge" tone="dark" /><div className="wrap projects-delivery-stage"><div className="projects-delivery-heading"><SectionHead eyebrow="Delivery includes the follow-up" title="AI agents notify customers when the fix ships." lede="Once a release is confirmed, the Echo agent can notify customers in the linked conversations, with the relevant guide attached. Your approval rules decide whether it sends the update or asks your team to review it first." /></div><ProjectDelivery /><Link prefetch={false} className="projects-inline-link" href="/products/customer-support">See how support connects<ArrowRight size={15} /></Link></div></section>
      <section id="project-faq"><div className="wrap projects-faq-grid"><SectionHead eyebrow="Questions, answered" title="FAQs about Helpin’s Projects tool" /><div className="projects-faqs"><FAQList items={FAQS} className="faq-items" /><Link prefetch={false} className="projects-inline-link" href="/self-hosting">Explore self-hosting<ArrowRight size={15} /></Link></div></div></section>
      <section className="final-cta final-cta-connected" aria-labelledby="projects-final-title"><div className="wrap"><ConnectedWorkspace /><div className="final"><span className="eyebrow">Plan together. Follow through.</span><h2 id="projects-final-title">Give your team and agents<br />a clear place to work.</h2><p className="lede">Plan the sprint, review the change, and keep the result easy to find.</p><CtaRow primaryLabel="Start free trial" secondaryHref={DEMO_URL} secondaryLabel="Book a demo" /><p className="projects-supporting-note">14-day free trial · No card required</p><p className="projects-supporting-note">Open source · Self-host free, or let us run it</p></div></div></section>
    </div>
    <PreviewFooter homepage />
  </>;
}
