import { WorkScene } from '../../_components/WorkScene';
import { FAQList } from '../../_components/ui';
import { createPageMetadata, PAGE_SEO } from '@/lib/metadata';
import Link from 'next/link';
import { ArrowRight, Bot, Clock3, Code2, LockKeyhole, MessageSquare, Settings2, ShieldCheck, Users } from 'lucide-react';
import { PreviewNav } from '../../_components/PreviewNav';
import { PreviewFooter } from '../../_components/PreviewFooter';
import { ConnectedWorkspace } from '../../_components/ConnectedWorkspace';
import { CtaRow, SectionHead } from '../../_components/ui';
import { HeroVortex } from '../../_components/HeroVortex';
import { ProductPreview } from '../../_components/product-previews';
import { AgentWorkflowArt } from './agent-workflow-art';
import './agents.css';

export const metadata = createPageMetadata(PAGE_SEO.aiAgents);
// Names and roles follow the system presets and the application's native persona catalog.
const SPECIALISTS = [
  { name: 'Echo', icon: 'echo', role: 'Support agent', description: 'Answer from your docs and earlier conversations, check in after a reply, and hand over when a person is needed.', outcome: 'A grounded answer. A clean handoff.' },
  { name: 'Atlas', icon: 'atlas', role: 'Planning agent', description: 'Turn a request or product idea into a clear scope and a plan your team can review.', outcome: 'A plan with the reason behind it.' },
  { name: 'Scribe', icon: 'scribe', role: 'Task planning agent', description: 'Read the requirements and code, work out the changes, and prepare a task before coding starts.', outcome: 'A task ready to build.' },
  { name: 'Forge', icon: 'forge', role: 'Coding agent', description: 'Write the change, run the tests, and open a pull request in your connected GitHub or GitLab repository.', outcome: 'A pull request. Original request attached.' },
  { name: 'Lens', icon: 'lens', role: 'Code review agent', description: 'Check the proposed code for bugs, missing tests, and requirements that still need work.', outcome: 'Findings before the final decision.' },
  { name: 'Quill', icon: 'quill', role: 'Docs agent', description: 'Update guides from customer questions and product changes. Use browser screenshots and recordings where the steps need showing.', outcome: 'Useful guidance, ready for review.' },
  { name: 'Beacon', icon: 'beacon', role: 'CRM agent', description: 'Find who needs attention, update customer records, and prepare a follow-up using the account’s conversations and open work.', outcome: 'The relationship behind the next step.' },
  { name: 'Mira', icon: 'mira', role: 'Marketing agent', description: 'Use what customers ask for to prepare launch messages, campaign ideas, and clearer product explanations.', outcome: 'Messaging informed by customer needs.' },
];
const CAPABILITIES = [
  { id: 'context', label: 'Your workspace', title: 'Start with what’s already known.', description: 'Bring the customer’s question, relevant guidance, and linked work into the same answer.' },
  { id: 'coordination', label: 'Your agents', title: 'Check the problem before planning the fix.', description: 'Your AI agents check the report, code, and guide. Use their findings to choose the next action.' },
  { id: 'tools', label: 'Your other tools', title: 'Check the tools your team already uses.', description: 'Connect external tools through MCP (beta) and select what each agent may use. Access stays limited to the tools you grant.' },
] as const;
const FAQS = [["Which agents does Helpin include?", "Eight specialists cover support, planning, coding task planning, coding, code review, docs, CRM, and marketing. Ask Agent is your starting point for finding answers and coordinating their work."], ["What is Ask Agent?", "It is the assistant your team talks to inside Helpin. Ask a question or give it work. It can use permitted tools directly or bring in specialists when needed."], ["Can agents work together?", "Yes. Ask Agent can coordinate specialists, run independent steps at the same time, and bring their findings back into the conversation."], ["Can an agent write code?", "Yes. Forge works in a connected GitHub or GitLab repository, runs the tests, and opens a pull request for your team to review. Coding agents need a connected repository and Agent Runtime, on Helpin Cloud or self-hosted."], ["Can agents use tools from other systems?", "Yes, in beta. Connect an external MCP server and select which tools an agent can use. The connection, tool permissions, and approval rules determine what it may do."], ["Can I create my own agents?", "Yes. Configure the instructions, skills, model, tools, and approvals for a specific job."], ["Can agents run without a chat message?", "Yes. Assign a task to an agent, or use an automation flow to start one when a task enters a stage, when a pull request is opened, merged, or needs review, or on a schedule."], ["Can we self-host the agents?", "Yes. Self-hosted Helpin is the complete product with every agent, coding agents included, free under AGPL-3.0 with no plan limits. You run the installation and connect your own AI provider."]] as const;
export default function AIAgentsPage() {
  return <>
    <PreviewNav tone="dark" />
    <main className="agents-page">
      <section className="agents-hero" aria-labelledby="agents-title">
        <HeroVortex />
        <div className="wrap">

          <div className="agents-hero-copy">
            <span className="eyebrow">AI agents for SaaS teams</span>
            <h1 id="agents-title">AI agents that take work <span>off your team’s list.</span></h1>
            <p className="lede">Answer a customer, update a guide, plan a fix, or follow up with a lead. Give agents the knowledge and tools for the job, then choose what runs automatically and what needs review.</p>
            <CtaRow primaryLabel="Start free trial" /><p className="agents-supporting-note">Open source · Self-host free, or let us run it</p>
            <div className="agents-hero-assure"><span><Users size={14} />Shared customer history</span><span><Bot size={14} />Specialist agents</span><span><ShieldCheck size={14} />Your approval rules</span></div>
          </div>
          <div className="agents-hero-roster" aria-label="Meet Helpin’s specialist agents">{SPECIALISTS.map(agent => <a href={`#agent-${agent.icon}`} key={agent.icon}><img src={`/new/agents/${agent.icon}.svg`} width={52} height={52} alt="" /><span>{agent.role}</span><small>{agent.name}</small></a>)}</div>
        </div>
      </section>

      <section id="agent-directory" className="agents-directory">
        <div className="wrap">
          <SectionHead eyebrow="A specialist for the job" title="Meet the agents. See what they can do." lede="Choose an agent for the job. It can use the relevant conversations, docs, and tasks already in Helpin, along with the tools you allow." />
          <div className="agents-directory-grid">{SPECIALISTS.map(agent => <article id={`agent-${agent.icon}`} key={agent.icon}><div className="agents-directory-persona"><img src={`/new/agents/${agent.icon}.svg`} width={56} height={56} alt="" /></div><h3>{agent.role} · {agent.name}</h3><p>{agent.description}</p><div className="agents-directory-outcome">{agent.outcome}</div></article>)}</div>
          <div className="agents-custom-strip"><div><h3>Build custom agents and flows with natural language.</h3><p>Describe the work you want done, from reviewing code to following up with leads. Helpin AI helps you create the agent or flow, then you review its instructions, tools, and steps before it runs.</p></div></div>

        </div>
      </section>
      <section id="agent-ask" className="agents-ask-section ask-agent-section section-motion"><HeroVortex variant="converge" tone="dark" />
        <div className="wrap">
          <SectionHead eyebrow="Your starting point" title="Ask a question. Hand off the work." lede="Ask about a customer or give Helpin a task. Helpin AI finds the relevant history, uses the tools you’ve allowed, and brings in specialist agents when the work needs them." />
          <figure className="ask-agent-preview">
            <ProductPreview product="ask-agent" theme="light" />
            <figcaption>Helpin AI brings the specialists’ findings back to your conversation.</figcaption>
          </figure>
        </div>
      </section>
      <section id="agent-workflows" className="agents-workflows">
        <div className="wrap">
          <SectionHead eyebrow="Across the workspace" title="Give the job to the right agents." lede="Ask Agent can have specialists check the code, customer report, and docs together. Their findings shape the plan before anyone starts changing things." />
          <div className="agents-capability-grid">{CAPABILITIES.map(item => <article key={item.id}><div className="agents-capability-copy"><span className="eyebrow">{item.label}</span><h3>{item.title}</h3><p>{item.description}</p></div><AgentWorkflowArt variant={item.id} />{item.id === 'tools' && <Link prefetch={false} className="agents-text-link" href="/developers#mcp">Connect external tools<ArrowRight size={15} /></Link>}</article>)}</div>
        </div>
      </section>
      <section id="agent-coding" className="agents-coding section-motion"><HeroVortex variant="converge" tone="dark" />
        <div className="wrap agents-split">
          <div><SectionHead eyebrow="From request to proposed change" title="Your team and coding agents have the full context." lede="Scribe plans the task. Forge writes code and tests. Lens reviews the change. Keep the original report close so everyone can check whether the fix solves it." /><ul className="agents-benefits"><li><MessageSquare size={18} />Keep the original request attached.</li><li><Code2 size={18} />Write the change, run the tests, open a pull request.</li><li><ShieldCheck size={18} />Review the change before release.</li></ul><p className="agents-supporting-note">Coding agents need a connected GitHub or GitLab repository and Agent Runtime, on Helpin Cloud or self-hosted.</p><a className="agents-text-link" href="/products/projects#project-agents">Explore coding agents<ArrowRight size={15} /></a></div>
          <div><AgentWorkflowArt variant="coding" /></div>
        </div>
      </section>
      <section id="agent-controls" className="agents-controls">
        <div className="wrap">
          <SectionHead eyebrow="Your team sets the limits" title="See what your agents did and what needs you." lede="Follow the run, inspect the results, and see what needs a decision. Your team keeps a place to review past work, check the evidence, and pick up anything an agent could not finish." />
          <div className="agents-split"><div className="agents-control-list">
            <article><Settings2 size={21} /><div><h3>Give it a clear job.</h3><p>Set the instructions, skills, and model for the work. Configure a specialist or create an agent for your own process.</p></div></article>
            <article><LockKeyhole size={21} /><div><h3>Choose its tools and limits.</h3><p>Select its workspace and external tools. Decide which actions can proceed and which need approval.</p></div></article>
            <article><Clock3 size={21} /><div><h3>Start work automatically.</h3><p>Run an agent on a schedule or when a task changes stage or a pull request opens or merges. You can also start it yourself. Review the run and results in Helpin.</p></div></article>
          </div><div><WorkScene variant="review" /></div></div>
        </div>
      </section>
      <section id="agent-faq"><div className="wrap agents-faq-grid"><SectionHead eyebrow="Questions, answered" title="FAQs about Helpin’s AI agents" /><div className="agents-faqs"><FAQList items={FAQS} className="faq-items" /><Link prefetch={false} className="agents-text-link" href="/self-hosting">Explore self-hosting<ArrowRight size={15} /></Link></div></div></section>
      <section className="final-cta final-cta-connected" aria-labelledby="agents-final-title"><div className="wrap"><ConnectedWorkspace /><div className="final"><span className="eyebrow">Put the history to work</span><h2 id="agents-final-title">Choose a job.<br />Put your agent to work.</h2><p className="lede">Start with a useful task, give the agent the right tools, and review the result in the same workspace.</p><CtaRow primaryLabel="Start free trial" /><p className="agents-supporting-note">14-day free trial · No card required</p></div></div></section>
    </main>
    <PreviewFooter homepage />
  </>;
}
