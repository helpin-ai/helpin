import { HeroVortex } from '../../_components/HeroVortex';
import { DEMO_URL, FAQList } from '../../_components/ui';
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
  { Icon: ListFilter, title: 'Views that fit the job', body: 'Save filters for a sprint, bug queue, or review list.' },
  { Icon: GitBranch, title: 'Dependencies in view', body: 'Show what must happen before another task can proceed.' },
  { Icon: Users, title: 'Clear ownership', body: 'Assign responsibility and dates before work gets lost between teams.' },
  { Icon: CheckSquare, title: 'Requirements within reach', body: 'Keep descriptions, checklists, files, and docs with the task.' },
  { Icon: Repeat2, title: 'Repeatable work', body: 'Use templates and recurrence for routine checks and maintenance.' },
  { Icon: Tags, title: 'Your team’s workflow', body: 'Organize work with the labels and states your process needs.' },
];
const FAQS = [
  [
    "What can we manage in Helpin Projects?",
    "Product development, customer requests, bugs, maintenance, and internal initiatives. Use tasks, epics, sprints, roadmaps, and objectives to organize the work. Customer context supports relevant projects; it does not have to define every project.",
    "/new/products/projects#project-tasks"
  ],
  [
    "Can a customer conversation become a task?",
    "Yes. Create new work or link the conversation to an existing task. Keep the request available without creating a duplicate item for work already underway.",
    "/new/products/projects#project-context"
  ],
  [
    "Which views are available?",
    "Board, list, and roadmap views.",
    "/new/products/projects#project-planning"
  ],
  [
    "How do sprints work?",
    "Select backlog tasks, track the sprint, review its closeout, and carry unfinished work forward.",
    "/new/products/projects#project-planning"
  ],
  [
    "Can we track outcomes as well as completed tasks?",
    "Yes. Record key-result progress separately from delivery progress.",
    "/new/products/projects#project-progress"
  ],
  [
    "How do agents help with planning and delivery?",
    "Planning agents develop the scope. Coding agents prepare implementation work and tests, while review agents examine the changes. Tool access and approval settings determine how they proceed.",
    "/new/products/ai-agents"
  ],
  [
    "Can we connect engineering work?",
    "Yes. Connect GitHub or GitLab activity to tasks. Coding agents work in connected repositories, with changes subject to your review process.",
    "/new/products/projects#project-agents"
  ],
  [
    "Will customers be notified automatically when a task is done?",
    "No. Marking a task complete does not itself send a customer message. Follow-up depends on the configured tools, workflow, and approvals.",
    "/new/products/projects#project-followup"
  ],
  ["Can we self-host Projects?", "Yes. Projects is included in Helpin’s open-source product. Your team operates the installation and covers hosting and provider costs. Enterprise features are licensed separately.", "/new/self-hosting#whats-included"]
] as const;
export default function ProjectsPage() {
  return <>
    <PreviewNav />
    <div className="projects-page">
      <section className="projects-hero motion-hero" aria-labelledby="projects-title"><HeroVortex variant="connections" tone="dark" />
        <div className="wrap">
          <div className="projects-breadcrumb"><Link href="/new">Helpin</Link><ChevronRight size={12} /><span>Projects</span></div>
          <div className="projects-hero-grid"><div className="projects-hero-copy"><span className="eyebrow">Project management for SaaS teams</span><h1 id="projects-title">Plan the work.<br /><span>Build with AI agents that know <em>why</em>.</span></h1><p className="lede">Bring roadmaps, sprints, and objectives into one workspace. AI agents use your requirements, tasks, and linked customer history to help scope work and prepare changes—with your team setting priorities and approvals.</p><CtaRow primaryLabel="Start free trial" secondaryHref={DEMO_URL} secondaryLabel="Book a demo" /><p className="projects-supporting-note">14-day cloud trial · No credit card required.</p><div className="projects-hero-points"><span><CheckSquare size={14} />Roadmaps and sprints</span><span><MessagesSquare size={14} />Customer context</span><span><GitBranch size={14} />AI-assisted delivery</span></div></div>
          <div><h2 className="projects-demo-heading">From the plan to the customer’s next step.</h2><ProjectHero /><p className="projects-demo-caption">The request, the work, and the follow-up stay connected.</p></div></div>
        </div>
      </section>
      <nav className="projects-page-nav" aria-label="On this page"><div className="wrap"><strong>Projects</strong><a href="#project-context">Prioritize</a><a href="#project-tasks">Organize</a><a href="#project-progress">Objectives</a><a href="#project-planning">Plan</a><a href="#project-agents">Deliver</a><a href="#project-faq">FAQs</a></div></nav>
      <section id="project-context"><div className="wrap projects-split"><div><SectionHead eyebrow="Start with the need" title="Bring customer needs into the plan." lede="Create or link work from a customer conversation. Keep the original request available when your team decides what to prioritize, what to build, and what can wait." /><div className="projects-context-points"><div><span>01</span><p><strong>Find the problem worth solving.</strong> Look beyond the requested feature. Understand what the customer is trying to achieve and what stands in the way.</p></div><div><span>02</span><p><strong>Decide what belongs in the plan.</strong> Weigh the need against your priorities. Agree on the scope before assigning the work.</p></div><div><span>03</span><p><strong>Carry the reason into the task.</strong> Give the next teammate or agent the original request—not just a title to interpret.</p></div></div></div><div><ProjectScene variant="context" /><p className="projects-demo-caption">A customer request informs the plan. Your team makes the commitment.</p></div></div></section>
      <section id="project-tasks" className="projects-tasks"><div className="wrap"><SectionHead eyebrow="The everyday work" title="See what’s moving. Know what needs attention." lede="Manage product work, bugs, maintenance, and internal projects together. Give your team a clear view of the priorities—and the next step they own." /><div className="projects-board-agent-preview"><ProjectFocus /></div><div className="projects-details">{DETAILS.map(({ Icon, title, body }) => <article key={title}><Icon size={20} aria-hidden="true" /><h3>{title}</h3><p>{body}</p></article>)}</div></div></section>
      <section id="project-progress"><div className="wrap"><div className="projects-outcomes-heading"><SectionHead eyebrow="Know what success means" title="Track what you delivered. Measure what it changed." lede="Connect objectives to epics and key results. Keep task completion and recorded outcomes visible as separate measures of progress." /></div><ProjectHealth /></div></section>
      <section id="project-planning" className="projects-planning"><div className="wrap"><SectionHead eyebrow="From the bigger plan to the next sprint" title="Give the roadmap a plan your team can work from." lede="Keep the longer-term direction and the next sprint in the same workspace, rather than treating planning and day-to-day delivery as separate jobs." /><div className="projects-planning-grid"><article><div className="projects-planning-copy"><span className="projects-planning-step">01 / ROADMAP</span><h3>See how the bigger pieces fit.</h3><p>Schedule epics around objectives, with owners, timing, and health in view.</p></div><ProjectScene variant="roadmap" /><p className="projects-demo-caption">Validate the mapping. Prepare the pilot. Support the rollout.</p></article><article><div className="projects-planning-copy"><span className="projects-planning-step">02 / SPRINTS</span><h3>Make the next commitment clear.</h3><p>Choose backlog tasks, review the sprint closeout, and carry unfinished work into the next sprint.</p></div><ProjectScene variant="sprint" /><p className="projects-demo-caption">The plan changes. The record of what happened remains.</p></article></div></div></section>

      <section id="project-agents" className="projects-agents"><div className="wrap projects-split"><div><SectionHead eyebrow="From a defined task to a proposed change" title="Give agents more than a task title." lede="Let planning and coding agents work from the requirements and relevant customer history. They can refine the task, prepare code and tests, and review the changes. Your team sets the approval rules." /><p className="projects-agent-copy">Keep the reason for the work available throughout implementation.</p><Link className="projects-inline-link" href="/new/products/ai-agents">Meet the agents<ArrowRight size={15} /></Link></div><div><ProjectScene variant="agents" /><p className="projects-demo-caption">The proposed change reflects what the customer agreed to—not a broader rollout they never requested.</p></div></div></section>
      <section id="project-followup" className="projects-loop"><div className="wrap projects-delivery-stage"><div className="projects-delivery-heading"><SectionHead eyebrow="Delivery includes the follow-up" title="Ship the change. Bring the answer back." lede="Connect the released work to the original request. Prepare the customer update with the relevant guide, then review and send it through the workflow your team has configured." /></div><ProjectDelivery /><Link className="projects-inline-link" href="/new/products/customer-support">See how support connects<ArrowRight size={15} /></Link></div></section>
      <section id="project-faq"><div className="wrap projects-faq-grid"><SectionHead eyebrow="Before you bring the team in" title="Get to know Helpin Projects." /><div className="projects-faqs"><FAQList items={FAQS} className="faq-items" /><Link className="projects-inline-link" href="/new/self-hosting">Explore self-hosting<ArrowRight size={15} /></Link></div></div></section>
      <section className="final-cta final-cta-connected" aria-labelledby="projects-final-title"><div className="wrap"><ConnectedWorkspace /><div className="final"><span className="eyebrow">Plan together. Follow through.</span><h2 id="projects-final-title">Bring the plan,<br />the work, and the customer together.</h2><p className="lede">Give your team a place to manage the project—and your AI agents the history to help move it forward. From the first decision to the customer’s next step.</p><CtaRow primaryLabel="Start free trial" secondaryHref={DEMO_URL} secondaryLabel="Book a demo" /><p className="projects-supporting-note">14-day cloud trial · No credit card required.</p></div></div></section>
    </div>
    <PreviewFooter homepage />
  </>;
}
