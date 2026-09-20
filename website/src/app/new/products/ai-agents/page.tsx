import type { Metadata } from 'next';
import Link from 'next/link';
import { ArrowRight, ChevronRight, Clock3, Code2, GitBranch, LockKeyhole, MessageSquare, Plug, Settings2, ShieldCheck } from 'lucide-react';
import { PreviewNav } from '../../_components/PreviewNav';
import { PreviewFooter } from '../../_components/PreviewFooter';
import { ConnectedWorkspace } from '../../_components/ConnectedWorkspace';
import { CtaRow, GITHUB_URL, SectionHead } from '../../_components/ui';
import { AgentWorkflowArt } from './agent-workflow-art';
import './agents.css';

export const metadata: Metadata = {
  title: 'AI Agents — Helpin',
  description: 'Meet Helpin’s agents for support, product planning, coding, review, docs, CRM, and marketing. Coordinate their work with Ask Agent and shared customer context.',
  alternates: { canonical: '/new/products/ai-agents' },
  robots: { index: false, follow: false },
};
// Names and roles follow the system presets and the application's native persona catalog.
const SPECIALISTS = [
  { name: 'Echo', icon: 'echo', role: 'Support Agent', description: 'Answer customers using product knowledge, triage their requests, and bring in your team when needed.', outcome: 'A grounded reply. A handoff with context.' },
  { name: 'Atlas', icon: 'atlas', role: 'Epic Planner', description: 'Turn a product idea into a PRD and a task plan your team can review and build on.', outcome: 'From customer need to a plan.' },
  { name: 'Scribe', icon: 'scribe', role: 'Coding Task Planner', description: 'Refine a task using the specification and code context. Work through the details before implementation starts.', outcome: 'A clear, actionable engineering task.' },
  { name: 'Forge', icon: 'forge', role: 'Code Builder', description: 'Work in a connected repository to implement a task, add tests, and prepare changes for review.', outcome: 'A proposed fix with the request attached.' },
  { name: 'Lens', icon: 'lens', role: 'QA & Code Reviewer', description: 'Review changes, identify risks and missing verification, and work through agreed fixes on the same branch.', outcome: 'Findings your team can act on.' },
  { name: 'Quill', icon: 'quill', role: 'Documentation Agent', description: 'Create and maintain help articles, internal docs, and API documentation as your product changes.', outcome: 'Knowledge that keeps up with the product.' },
  { name: 'Beacon', icon: 'beacon', role: 'CRM Operator', description: 'Work with contacts, companies, and deals using the support conversations and documents behind each account.', outcome: 'Customer records with the story attached.' },
  { name: 'Mira', icon: 'mira', role: 'Marketing Agent', description: 'Turn customer signals into positioning, campaign plans, launch copy, and lifecycle messaging.', outcome: 'Marketing grounded in what customers need.' },
];
const CAPABILITIES = [
  { id: 'context', label: 'Your workspace', title: 'Start with the full picture.', description: 'Bring the customer question, the relevant docs, and the task behind it into one answer.', prompt: 'What’s holding up Maya’s export?' },
  { id: 'coordination', label: 'Your agents', title: 'Give complex work a plan.', description: 'Run independent work in parallel. Sequence the steps that depend on it, then bring the results back together.', prompt: 'Review the code and docs in parallel, then propose a fix plan.' },
  { id: 'tools', label: 'Your other tools', title: 'Bring outside context in.', description: 'Connect external MCP servers and choose the tools each agent can use alongside your workspace context.', prompt: 'Check EXP-142 in our connected issue tracker.' },
] as const;
const FAQS = [
  ['Which agents does Helpin include?', 'The specialist lineup includes Echo for support, Atlas for epic planning, Scribe for coding task planning, Forge for implementation, Lens for QA and code review, Quill for documentation, Beacon for CRM, and Mira for marketing. Ask Agent handles workspace requests and coordinates specialists and sub-agents when needed. Availability depends on your edition, enabled modules, and configuration.'],
  ['What is Ask Agent?', 'Ask Agent is your workspace assistant for questions and work. It can find customer context, investigate an issue, create or update workspace records, and coordinate other agents when the task calls for it. What it can do depends on the tools, permissions, and modules available in your workspace.'],
  ['Can agents work together?', 'Yes. Helpin supports sub-agents and plans with dependent steps. Independent tasks can run in parallel, while later steps wait for the results they need. Ask Agent brings the findings back into the conversation.'],
  ['Can an agent write code?', 'Repository-capable agents can investigate a connected repository, prepare changes and tests, and return work for review. Repository access and delivery settings determine what they can do. Coding workflows require the full workspace and a configured Agent Runtime; they are outside the Community 0.1 beta.'],
  ['How do external MCP tools work?', 'Connect a supported external MCP server, review its available tools, then select which tools an agent may use. Installing a server does not give every agent access to all its tools. Actions remain subject to the configured approval policy.'],
  ['Can I create my own agents?', 'Yes. Configure an agent’s instructions, skills, model, tools, and approval settings for the work you want it to do. System agents and custom agents use the same execution and review surfaces.'],
  ['Can agents run without a chat message?', 'Yes. Supported events, automation rules, and schedules can start agent work. Available triggers and targets depend on the enabled modules and configuration. Runs still use the agent’s tools and approval settings.'],
  ['Can we self-host?', 'Yes. The Community beta includes support, docs, and agents. Repository coding and the broader project and CRM workflows are outside that beta surface. Review the Community setup guide for the components and configuration needed to run agents.'],
];
export default function AIAgentsPage() {
  return <>
    <PreviewNav />
    <main className="agents-page">
      <section className="agents-hero" aria-labelledby="agents-title">
        <div className="wrap">
          <div className="agents-breadcrumb"><Link href="/new">Helpin</Link><ChevronRight size={12} /><span>AI agents</span></div>
          <div className="agents-hero-copy">
            <span className="eyebrow">AI agents for your workspace</span>
            <h1 id="agents-title">Specialist agents.<br /><span>One customer history.</span></h1>
            <p className="lede">Answer customers, plan product work, write code, keep docs current, and move deals forward—with agents that share your workspace context.</p>
            <CtaRow secondaryHref="#agent-directory" secondaryLabel="Meet the agents" />
            <div className="agents-hero-assure"><span><GitBranch size={14} />Coordinate agents</span><span><Plug size={14} />Connect your tools</span><span><ShieldCheck size={14} />Control approvals</span></div>
          </div>
          <div className="agents-hero-roster" aria-label="Meet Helpin’s specialist agents">{SPECIALISTS.map(agent => <a href={`#agent-${agent.icon}`} key={agent.icon}><img src={`/new/agents/${agent.icon}.svg`} width={52} height={52} alt="" /><span>{agent.name}</span><small>{agent.role}</small></a>)}</div>
        </div>
      </section>
      <nav className="agents-page-nav" aria-label="On this page"><div className="wrap"><strong>AI Agents</strong><a href="#agent-directory">Meet the agents</a><a href="#agent-ask">Ask Agent</a><a href="#agent-coding">Coding agents</a><a href="#agent-controls">Tools & approvals</a><a href="#agent-faq">FAQs</a></div></nav>
      <section id="agent-directory" className="agents-directory">
        <div className="wrap">
          <SectionHead eyebrow="Meet your agents" title="Different jobs. Connected work." lede="Start with agents built for the work your team does every day. Each has a focused role, with tools and instructions you can configure." />
          <div className="agents-directory-grid">{SPECIALISTS.map(agent => <article id={`agent-${agent.icon}`} key={agent.icon}><div className="agents-directory-persona"><img src={`/new/agents/${agent.icon}.svg`} width={56} height={56} alt="" /><span>{agent.name}</span></div><h3>{agent.role}</h3><p>{agent.description}</p><div className="agents-directory-outcome">{agent.outcome}</div></article>)}</div>
          <div className="agents-custom-strip"><Settings2 size={22} aria-hidden="true" /><div><h3>A different job? Make an agent for it.</h3><p>Choose its instructions, skills, model, tools, and approval settings.</p></div><a href="#agent-controls">Configure your agents<ArrowRight size={15} /></a></div>
          <p className="agents-directory-availability">Agent availability follows your enabled modules and edition. Community beta includes support, docs, and agents; project, CRM, and repository coding workflows require the full workspace.</p>
        </div>
      </section>
      <section id="agent-ask" className="agents-ask-section">
        <div className="wrap">
          <SectionHead eyebrow="Ask Agent" title="Ask a question. Hand off the work." lede="Your starting point across the workspace. Ask Agent finds answers and handles work directly, bringing in specialists or sub-agents when the request calls for them." />
          <figure className="agents-product-image">
            <img src="/new/support/ask-agent-support-sep20-1920.webp" srcSet="/new/support/ask-agent-support-sep20-960.webp 960w, /new/support/ask-agent-support-sep20-1440.webp 1440w, /new/support/ask-agent-support-sep20-1920.webp 1920w, /new/support/ask-agent-support-sep20-3840.webp 3840w" sizes="(max-width: 1240px) calc(100vw - 48px), 1192px" width={3840} height={2160} loading="lazy" decoding="async" alt="Illustrative OrbitDesk workspace: Ask Agent investigates Maya’s incomplete CSV export, creates EXP-142, coordinates a coding agent, and assigns the proposed fix to Sam for review. The fix is not merged." />
          </figure>
          <p className="agents-ask-caption">One customer issue. A linked task, a proposed fix, and a teammate ready to review.</p>
        </div>
      </section>
      <section id="agent-workflows" className="agents-workflows">
        <div className="wrap">
          <SectionHead eyebrow="Working together" title="One request can bring the right agents together." lede="Give focused work to a sub-agent, run independent tasks in parallel, and sequence the steps that need earlier results. Keep the findings and next steps in the same conversation." />
          <div className="agents-capability-grid">{CAPABILITIES.map(item => <article key={item.id}><div className="agents-capability-copy"><span className="eyebrow">{item.label}</span><h3>{item.title}</h3><p>{item.description}</p></div><AgentWorkflowArt variant={item.id} /><div className="agents-prompt"><span>Try asking</span><p>“{item.prompt}”</p></div></article>)}</div>
        </div>
      </section>
      <section id="agent-coding" className="agents-coding">
        <div className="wrap agents-split">
          <div><SectionHead eyebrow="Coding agents" title="Take the customer issue into the code." lede="Let Scribe refine the task, Forge prepare the implementation, and Lens review the changes. Keep the customer conversation and repository context attached through each step." /><ul className="agents-benefits"><li><MessageSquare size={18} />Keep the original request attached.</li><li><Code2 size={18} />Work in a connected repository.</li><li><ShieldCheck size={18} />Review the changes before they ship.</li></ul><p className="agents-availability">Repository workflows are available in the full workspace, outside the Community beta.</p><a className="agents-text-link" href={`${GITHUB_URL}/blob/develop/docs/coding-agent-execution.md`} target="_blank" rel="noopener noreferrer">Explore coding agents<ArrowRight size={15} /></a></div>
          <AgentWorkflowArt variant="coding" />
        </div>
      </section>
      <section id="agent-controls" className="agents-controls">
        <div className="wrap">
          <SectionHead eyebrow="Your agents. Your settings." title="Choose what they can do. And when." lede="Give each agent a job, the tools it needs, and clear approval settings. Start work from a conversation or a configured trigger." />
          <div className="agents-split"><div className="agents-control-list">
            <article><Settings2 size={21} /><div><h3>Make the agent fit the work.</h3><p>Set its instructions, skills, and model. Build on an existing agent or configure your own.</p></div></article>
            <article><LockKeyhole size={21} /><div><h3>Select its tools and approvals.</h3><p>Choose built-in and external MCP tools. Decide which actions need your team’s review.</p></div></article>
            <article><Clock3 size={21} /><div><h3>Choose what starts the work.</h3><p>Run agents on demand, from supported events, or on a schedule. Follow progress and review the results.</p></div></article>
          </div><AgentWorkflowArt variant="approval" /></div>
        </div>
      </section>
      <section id="agent-faq"><div className="wrap agents-faq-grid"><SectionHead eyebrow="A few useful answers" title="Before you hand it over." /><div className="agents-faqs">{FAQS.map(([question, answer]) => <details key={question}><summary>{question}<ChevronRight size={17} aria-hidden="true" /></summary><p>{answer}</p></details>)}</div></div></section>
      <section className="final-cta final-cta-connected" aria-labelledby="agents-final-title"><div className="wrap"><ConnectedWorkspace /><div className="final"><span className="eyebrow">Keep the work connected</span><h2 id="agents-final-title">Start with a question.<br />Leave with the work moving.</h2><p className="lede">Bring your customer context, tools, and agents into one workspace.</p><CtaRow /><div className="assure"><span>Open source</span><span>Self-hostable</span><span>Your tools. Your approvals.</span></div></div></div></section>
    </main>
    <PreviewFooter />
  </>;
}
