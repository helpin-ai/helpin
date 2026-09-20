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
  title: 'AI Agents & Ask Agent — Helpin',
  description: 'Ask questions, coordinate agents, and hand off work with customer context attached. Connect selected MCP tools and control approvals in Helpin.',
  alternates: { canonical: '/new/products/ai-agents' },
  robots: { index: false, follow: false },
};
const CAPABILITIES = [
  { id: 'context', label: 'Your workspace', title: 'Start with the full picture.', description: 'Bring the customer question, the relevant docs, and the task behind it into one answer.', prompt: 'What’s holding up Maya’s export?' },
  { id: 'coordination', label: 'Your agents', title: 'Give complex work a plan.', description: 'Run independent work in parallel. Sequence the steps that depend on it, then bring the results back together.', prompt: 'Review the code and docs in parallel, then propose a fix plan.' },
  { id: 'tools', label: 'Your other tools', title: 'Bring outside context in.', description: 'Connect external MCP servers and choose the tools each agent can use alongside your workspace context.', prompt: 'Check EXP-142 in our connected issue tracker.' },
] as const;
const FAQS = [
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
            <span className="eyebrow">Ask Agent</span>
            <h1 id="agents-title">Ask a question.<br /><span>Hand off the work.</span></h1>
            <p className="lede">Find the answer, investigate an issue, or describe what needs to happen. Ask Agent uses your customer context and selected tools to move the work forward.</p>
            <CtaRow secondaryHref="#agent-workflows" secondaryLabel="See how it works" />
            <div className="agents-hero-assure"><span><GitBranch size={14} />Coordinate agents</span><span><Plug size={14} />Connect your tools</span><span><ShieldCheck size={14} />Control approvals</span></div>
          </div>
          <figure className="agents-product-image">
            <img src="/new/support/ask-agent-support-sep20-1920.webp" srcSet="/new/support/ask-agent-support-sep20-960.webp 960w, /new/support/ask-agent-support-sep20-1440.webp 1440w, /new/support/ask-agent-support-sep20-1920.webp 1920w, /new/support/ask-agent-support-sep20-3840.webp 3840w" sizes="(max-width: 1240px) calc(100vw - 48px), 1192px" width={3840} height={2160} fetchPriority="high" alt="Illustrative OrbitDesk workspace: Ask Agent investigates Maya’s incomplete CSV export, creates EXP-142, coordinates a coding agent, and assigns the proposed fix to Sam for review. The fix is not merged." />
          </figure>
        </div>
      </section>
      <nav className="agents-page-nav" aria-label="On this page"><div className="wrap"><strong>AI Agents</strong><a href="#agent-workflows">Context & coordination</a><a href="#agent-coding">Coding agents</a><a href="#agent-controls">Tools & approvals</a><a href="#agent-faq">FAQs</a></div></nav>
      <section id="agent-workflows" className="agents-workflows">
        <div className="wrap">
          <SectionHead eyebrow="From question to next step" title="The context to understand. The tools to act." lede="Your team and agents work from the same customer history. Keep the conversation connected as a question becomes a plan, a task, or a follow-up." />
          <div className="agents-capability-grid">{CAPABILITIES.map(item => <article key={item.id}><div className="agents-capability-copy"><span className="eyebrow">{item.label}</span><h3>{item.title}</h3><p>{item.description}</p></div><AgentWorkflowArt variant={item.id} /><div className="agents-prompt"><span>Try asking</span><p>“{item.prompt}”</p></div></article>)}</div>
        </div>
      </section>
      <section id="agent-coding" className="agents-coding">
        <div className="wrap agents-split">
          <div><SectionHead eyebrow="Coding agents" title="Take the customer issue into the code." lede="Give a coding agent the linked task, customer conversation, and repository. Prepare a fix and tests, then bring the changes back for review." /><ul className="agents-benefits"><li><MessageSquare size={18} />Keep the original request attached.</li><li><Code2 size={18} />Work in a connected repository.</li><li><ShieldCheck size={18} />Review the changes before they ship.</li></ul><p className="agents-availability">Repository workflows are available in the full workspace, outside the Community beta.</p><a className="agents-text-link" href={`${GITHUB_URL}/blob/develop/docs/coding-agent-execution.md`} target="_blank" rel="noopener noreferrer">Explore coding agents<ArrowRight size={15} /></a></div>
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
