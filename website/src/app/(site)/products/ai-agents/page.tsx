import { FAQList } from '../../_components/ui';
import { createPageMetadata, PAGE_SEO } from '@/lib/metadata';
import Link from 'next/link';
import { ArrowRight, Bot, ChevronRight, Clock3, Code2, LockKeyhole, MessageSquare, Settings2, ShieldCheck, Users } from 'lucide-react';
import { PreviewNav } from '../../_components/PreviewNav';
import { PreviewFooter } from '../../_components/PreviewFooter';
import { ConnectedWorkspace } from '../../_components/ConnectedWorkspace';
import { CtaRow, SectionHead } from '../../_components/ui';
import { HeroVortex } from '../../_components/HeroVortex';
import { AgentWorkflowArt } from './agent-workflow-art';
import { ProductPreview } from '../../_components/product-previews';
import './agents.css';

export const metadata = createPageMetadata(PAGE_SEO.aiAgents);
// Names and roles follow the system presets and the application's native persona catalog.
const SPECIALISTS = [
  { name: 'Echo', icon: 'echo', role: 'Support agent', description: 'Answer customers in live chat from your help center and earlier conversations. Hand the conversation to a teammate when it needs a person.', outcome: 'A grounded answer. A clean handoff.' },
  { name: 'Atlas', icon: 'atlas', role: 'Planning agent', description: 'Turn customer needs and product requirements into a scope and task plan your team can review.', outcome: 'A plan with the reason behind it.' },
  { name: 'Scribe', icon: 'scribe', role: 'Coding task planner', description: 'Work through implementation details using the task, specification, and connected codebase before coding begins.', outcome: 'A task ready to build.' },
  { name: 'Forge', icon: 'forge', role: 'Coding agent', description: 'Implement the agreed task in a connected GitHub or GitLab repository, run the tests, and open a pull request for review.', outcome: 'A pull request. Original request attached.' },
  { name: 'Lens', icon: 'lens', role: 'Code reviewer', description: 'Examine the changes for risks, missing tests, and differences from the agreed requirements.', outcome: 'Findings before the final decision.' },
  { name: 'Quill', icon: 'quill', role: 'Docs agent', description: 'Prepare help articles and document updates using customer questions and relevant product changes.', outcome: 'Useful guidance, ready for review.' },
  { name: 'Beacon', icon: 'beacon', role: 'CRM agent', description: 'Use account history to work with customer records, review deal concerns, and prepare the next follow-up.', outcome: 'The relationship behind the next step.' },
  { name: 'Mira', icon: 'mira', role: 'Marketing agent', description: 'Use customer needs and signals to prepare positioning, campaign plans, and launch messaging.', outcome: 'Messaging informed by customer needs.' },
];
const CAPABILITIES = [
  { id: 'context', label: 'Your workspace', title: 'Start with what’s already known.', description: 'Bring the customer’s question, relevant guidance, and linked work into the same answer.', prompt: 'What’s holding up Maya’s export, and what have we already tried?' },
  { id: 'coordination', label: 'Your agents', title: 'Investigate together. Act in the right order.', description: 'Let specialists examine different parts of the problem, then use their findings to shape the next step.', prompt: 'Check the export code and the guide at the same time. Bring me a fix plan.' },
  { id: 'tools', label: 'Your other tools · Beta', title: 'Check the tools your team already uses.', description: 'Connect external tools through MCP (beta) and select what each agent may use. Access stays limited to the tools you grant.', prompt: 'Check the linked issue in our connected tracker before preparing Maya’s update.' },
] as const;
const FAQS = [["Which agents does Helpin include?", "Eight specialists cover support, planning, coding task planning, coding, code review, docs, CRM, and marketing. Ask Agent is your starting point for finding answers and coordinating their work."], ["What is Ask Agent?", "It is the assistant your team talks to inside Helpin. Ask a question or give it work. It can use permitted tools directly or bring in specialists when needed."], ["Can agents work together?", "Yes. Ask Agent can coordinate specialists, run independent steps at the same time, and bring their findings back into the conversation."], ["Can an agent write code?", "Yes. Forge works in a connected GitHub or GitLab repository, runs the tests, and opens a pull request for your team to review. Coding agents need a connected repository and Agent Runtime, on Helpin Cloud or self-hosted."], ["Can agents use tools from other systems?", "Yes, in beta. Connect an external MCP server and select which tools an agent can use. The connection, tool permissions, and approval rules determine what it may do."], ["Can I create my own agents?", "Yes. Configure the instructions, skills, model, tools, and approvals for a specific job. On Helpin Cloud, custom agents are in the Growth plan. They’re included when you self-host."], ["Can agents run without a chat message?", "Yes. Assign a task to an agent, or use an automation flow to start one when a task enters a stage, when a pull request is opened, merged, or needs review, or on a schedule. On Helpin Cloud, automation flows and scheduled runs are in the Growth plan. They’re included when you self-host."], ["Can we self-host the agents?", "Yes. Self-hosted Helpin is the complete product with every agent, coding agents included, free under AGPL-3.0 with no plan limits. You run the installation and connect your own AI provider."]] as const;
export default function AIAgentsPage() {
  return <>
    <PreviewNav />
    <main className="agents-page">
      <section className="agents-hero" aria-labelledby="agents-title">
        <HeroVortex />
        <div className="wrap">
          <div className="agents-breadcrumb"><Link href="/">Helpin</Link><ChevronRight size={12} /><span>AI agents</span></div>
          <div className="agents-hero-copy">
            <span className="eyebrow">AI agents for SaaS teams</span>
            <h1 id="agents-title">AI agents that turn <span>customer history into action.</span></h1>
            <p className="lede">Agents answer customers, plan the work, open pull requests, and update the CRM, using the same customer history your team sees. Choose the tools they can use and the actions your team reviews.</p>
            <CtaRow primaryLabel="Start free trial" /><p className="agents-supporting-note">Open source · Self-host free, or let us run it</p>
            <div className="agents-hero-assure"><span><Users size={14} />Shared customer history</span><span><Bot size={14} />Specialist agents</span><span><ShieldCheck size={14} />Your approval rules</span></div>
          </div>
          <div className="agents-hero-roster" aria-label="Meet Helpin’s specialist agents">{SPECIALISTS.map(agent => <a href={`#agent-${agent.icon}`} key={agent.icon}><img src={`/new/agents/${agent.icon}.svg`} width={52} height={52} alt="" /><span>{agent.role}</span><small>{agent.name}</small></a>)}</div><p className="agents-demo-caption">Eight specialists. One customer history.</p>
        </div>
      </section>
      <nav className="agents-page-nav" aria-label="On this page"><div className="wrap"><strong>AI Agents</strong><a href="#agent-directory">Meet the agents</a><a href="#agent-ask">Ask Agent</a><a href="#agent-coding">Coding agents</a><a href="#agent-controls">Tools & approvals</a><a href="#agent-faq">FAQs</a></div></nav>
      <section id="agent-directory" className="agents-directory">
        <div className="wrap">
          <SectionHead eyebrow="A specialist for the job" title="Give each agent a role. Keep the work connected." lede="Start with a specialist for the task at hand. Give it the relevant history, instructions, and tools—not another brief written from scratch." />
          <div className="agents-directory-grid">{SPECIALISTS.map(agent => <article id={`agent-${agent.icon}`} key={agent.icon}><div className="agents-directory-persona"><img src={`/new/agents/${agent.icon}.svg`} width={56} height={56} alt="" /></div><h3>{agent.role} · {agent.name}</h3><p>{agent.description}</p><div className="agents-directory-outcome">{agent.outcome}</div></article>)}</div>
          <div className="agents-custom-strip"><Settings2 size={22} aria-hidden="true" /><div><h3>A different job? Build an agent for it.</h3><p>Define its instructions, skills, model, tools, and approval settings.</p></div><a href="#agent-controls">Configure your agents<ArrowRight size={15} /></a></div><p className="agents-supporting-note">On Helpin Cloud, custom agents are in the Growth plan. Included when you self-host.</p>

        </div>
      </section>
      <section id="agent-ask" className="agents-ask-section ask-agent-section section-motion"><HeroVortex variant="converge" tone="dark" />
        <div className="wrap">
          <SectionHead eyebrow="Your starting point" title="Ask a question. Hand off the work." lede="Ask about a customer or give Helpin a task. Ask Agent finds the relevant history, uses the tools you’ve allowed, and brings in specialist agents when the work needs them." />
          <h3 className="agents-demo-heading">One request. The right specialists on it.</h3><figure className="ask-agent-preview">
            <ProductPreview product="ask-agent" theme="light" />
            <figcaption>One request. Four specialist checks. One plan and a draft reply.</figcaption>
          </figure>
        </div>
      </section>
      <section id="agent-workflows" className="agents-workflows">
        <div className="wrap">
          <SectionHead eyebrow="Across the workspace" title="Different specialists. One coordinated workflow." lede="Run independent checks at the same time. Bring the findings together before starting the steps that depend on them. Keep the request and results in one conversation." />
          <div className="agents-capability-grid">{CAPABILITIES.map(item => <article key={item.id}><div className="agents-capability-copy"><span className="eyebrow">{item.label}</span><h3>{item.title}</h3><p>{item.description}</p></div><AgentWorkflowArt variant={item.id} /><div className="agents-prompt"><span>Try asking</span><p>“{item.prompt}”</p></div><p className="agents-demo-caption">{item.id === 'context' ? 'Don’t recommend a workaround the customer has already outgrown.' : item.id === 'coordination' ? 'Separate investigations. A joined-up plan.' : 'Read access doesn’t mean write access.'}</p>{item.id === 'tools' && <Link className="agents-text-link" href="/developers#mcp">Connect external tools<ArrowRight size={15} /></Link>}</article>)}</div>
        </div>
      </section>
      <section id="agent-coding" className="agents-coding section-motion"><HeroVortex variant="converge" tone="dark" />
        <div className="wrap agents-split">
          <div><SectionHead eyebrow="From request to proposed change" title="Give coding agents the reason behind the issue." lede="Carry the customer request into planning, implementation, and review. Coding agents work from the task and connected repository, so the proposed change stays tied to the problem your team agreed to solve." /><ul className="agents-benefits"><li><MessageSquare size={18} />Keep the original request attached.</li><li><Code2 size={18} />Write the change, run the tests, open a pull request.</li><li><ShieldCheck size={18} />Review the change before release.</li></ul><p className="agents-supporting-note">Coding agents need a connected GitHub or GitLab repository and Agent Runtime, on Helpin Cloud or self-hosted.</p><a className="agents-text-link" href="/products/projects#project-agents">Explore coding agents<ArrowRight size={15} /></a></div>
          <div><AgentWorkflowArt variant="coding" /><p className="agents-demo-caption">Review the implementation—and whether it solves the original problem.</p></div>
        </div>
      </section>
      <section id="agent-controls" className="agents-controls">
        <div className="wrap">
          <SectionHead eyebrow="Your team sets the limits" title="Delegate the job. Keep control of the actions." lede="Set each agent’s role, tools, and approval rules. Choose what starts the work, then follow the run and review the results." />
          <div className="agents-split"><div className="agents-control-list">
            <article><Settings2 size={21} /><div><h3>Make the role clear.</h3><p>Set the instructions, skills, and model for the work. Configure a specialist or create an agent for your own process.</p></div></article>
            <article><LockKeyhole size={21} /><div><h3>Choose what it can do.</h3><p>Select its workspace and external tools. Decide which actions can proceed and which need approval.</p></div></article>
            <article><Clock3 size={21} /><div><h3>Decide when it gets to work.</h3><p>Start it yourself, assign it a task, or trigger it when a task enters a stage, a pull request opens or merges, or on a schedule. Check the activity and results afterward.</p></div></article>
          <p className="agents-supporting-note">On Helpin Cloud, custom agents, scheduled runs, and automation flows are in the Growth plan. Included when you self-host.</p></div><div><h3 className="agents-demo-heading">Review the next action.</h3><AgentWorkflowArt variant="approval" /><p className="agents-demo-caption">Approve the task update—not every action that might follow.</p></div></div>
        </div>
      </section>
      <section id="agent-faq"><div className="wrap agents-faq-grid"><SectionHead eyebrow="Before you delegate" title="Get to know Helpin’s agents." /><div className="agents-faqs"><FAQList items={FAQS} className="faq-items" /><Link className="agents-text-link" href="/self-hosting">Explore self-hosting<ArrowRight size={15} /></Link></div></div></section>
      <section className="final-cta final-cta-connected" aria-labelledby="agents-final-title"><div className="wrap"><ConnectedWorkspace /><div className="final"><span className="eyebrow">Put the history to work</span><h2 id="agents-final-title">Hand off the work.<br />Keep the customer in it.</h2><p className="lede">Support, planning, coding, docs, CRM, and marketing agents working from one customer history.</p><CtaRow primaryLabel="Start free trial" /><p className="agents-supporting-note">14-day free trial · No card required</p></div></div></section>
    </main>
    <PreviewFooter homepage />
  </>;
}
