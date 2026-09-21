import { Availability, CtaNote, ExampleLabel, FAQList } from '../../_components/ui';
import { previewMetadata } from '../../_components/preview-metadata';
import Link from 'next/link';
import { ArrowRight, ChevronRight, Clock3, Code2, GitBranch, LockKeyhole, MessageSquare, Plug, Settings2, ShieldCheck } from 'lucide-react';
import { PreviewNav } from '../../_components/PreviewNav';
import { PreviewFooter } from '../../_components/PreviewFooter';
import { ConnectedWorkspace } from '../../_components/ConnectedWorkspace';
import { CtaRow, GITHUB_URL, SectionHead } from '../../_components/ui';
import { AgentWorkflowArt } from './agent-workflow-art';
import { ProductPreview } from '../../_components/product-previews';
import './agents.css';

export const metadata = previewMetadata("AI Agents \u2014 Helpin", "/new/products/ai-agents");
// Names and roles follow the system presets and the application's native persona catalog.
const SPECIALISTS = [
  { name: 'Echo', icon: 'echo', role: 'Support agent', description: 'Answer customers using product knowledge, triage their requests, and bring in your team when needed.', outcome: 'A grounded reply. A handoff with context.' },
  { name: 'Atlas', icon: 'atlas', role: 'Planning agent', description: 'Turn a product idea into a specification and a task plan your team can review and build on.', outcome: 'From customer need to a plan.' },
  { name: 'Scribe', icon: 'scribe', role: 'Coding task planner', description: 'Refine a task using the specification and code context. Work through the details before implementation starts.', outcome: 'A clear, actionable engineering task.' },
  { name: 'Forge', icon: 'forge', role: 'Coding agent', description: 'Work in a connected repository to implement a task, add tests, and prepare changes for review.', outcome: 'A proposed fix with the request attached.' },
  { name: 'Lens', icon: 'lens', role: 'Code reviewer', description: 'Review changes, identify risks and missing verification, and work through agreed fixes on the same branch.', outcome: 'Findings your team can act on.' },
  { name: 'Quill', icon: 'quill', role: 'Docs agent', description: 'Create and maintain help articles, internal docs, and API documentation as your product changes.', outcome: 'Knowledge that keeps up with the product.' },
  { name: 'Beacon', icon: 'beacon', role: 'CRM agent', description: 'Work with contacts, companies, and deals using the support conversations and documents behind each account.', outcome: 'Customer records with the story attached.' },
  { name: 'Mira', icon: 'mira', role: 'Marketing agent', description: 'Turn customer signals into positioning, campaign plans, launch copy, and lifecycle messaging.', outcome: 'Marketing grounded in what customers need.' },
];
const CAPABILITIES = [
  { id: 'context', label: 'Your workspace', title: 'Start with the customer history.', description: 'Bring the customer question, the relevant docs, and the task behind it into one answer.', prompt: 'What’s holding up Maya’s export?' },
  { id: 'coordination', label: 'Your agents', title: 'Give complex work a plan.', description: 'Run independent work in parallel. Sequence the steps that depend on it, then bring the results back together.', prompt: 'Review the code and docs in parallel, then propose a fix plan.' },
  { id: 'tools', label: 'Your other tools', title: 'Bring outside context in.', description: 'Connect external MCP servers and choose the tools each agent can use alongside your workspace context.', prompt: 'Check EXP-142 in our connected issue tracker.' },
] as const;
const FAQS = [
  [
    "Which agents does Helpin include?",
    "Eight specialists — support, planning, coding task planning, coding, code review, docs, CRM and marketing — plus Ask Agent to coordinate them. Build your own for another job.",
    "/new/products/ai-agents#agent-directory"
  ],
  [
    "What is Ask Agent?",
    "Ask Agent is the assistant your team talks to inside Helpin. Ask a question or give it work, and it uses the tools you have allowed.",
    "/new/products/ai-agents#agent-ask"
  ],
  [
    "Can agents work together?",
    "Yes. Ask Agent can split a request across specialists, run independent steps at once and bring the results into one conversation.",
    "/new/products/ai-agents#agent-workflows"
  ],
  [
    "Can an agent write code?",
    "Yes, in Cloud. Coding agents prepare changes and tests in your connected codebase, then hand the work to your team for review.",
    "/new/products/ai-agents#agent-coding"
  ],
  [
    "Can agents use tools from other systems?",
    "Yes. Connect a tool server and select the tools each agent may use; actions still follow your approval settings.",
    "/new/developers#mcp"
  ],
  [
    "Can I create my own agents?",
    "Yes. Configure an agent’s instructions, skills, model, tools and approval settings.",
    "/new/products/ai-agents#agent-controls"
  ],
  [
    "Can agents run without a chat message?",
    "Yes. Configured events and schedules can start agent work using your chosen tools and approval rules.",
    "/new/developers#webhooks"
  ],
  [
    "Can we self-host?",
    "Yes. Community includes support, docs and agents.",
    "/new/self-hosting#whats-included",
    "See what’s included"
  ]
] as const;
export default function AIAgentsPage() {
  return <>
    <PreviewNav />
    <main className="agents-page">
      <section className="agents-hero" aria-labelledby="agents-title">
        <div className="wrap">
          <div className="agents-breadcrumb"><Link href="/new">Helpin</Link><ChevronRight size={12} /><span>AI agents</span></div>
          <div className="agents-hero-copy">
            <Availability category="AI agents" />
            <h1 id="agents-title">AI agents that already <span>know your customers.</span></h1>
            <p className="lede">Answer customers, plan product work, write code, keep docs current, and move deals forward—with agents that share your customer history.</p>
            <CtaRow /><CtaNote />
            <div className="agents-hero-assure"><span><GitBranch size={14} />Coordinate agents</span><span><Plug size={14} />Connect your tools</span><span><ShieldCheck size={14} />Control approvals</span></div>
          </div>
          <div className="agents-hero-roster" aria-label="Meet Helpin’s specialist agents">{SPECIALISTS.map(agent => <a href={`#agent-${agent.icon}`} key={agent.icon}><img src={`/new/agents/${agent.icon}.svg`} width={52} height={52} alt="" /><span>{agent.role}</span><small>{agent.name}</small></a>)}</div>
        </div>
      </section>
      <nav className="agents-page-nav" aria-label="On this page"><div className="wrap"><strong>AI Agents</strong><a href="#agent-directory">Meet the agents</a><a href="#agent-ask">Ask Agent</a><a href="#agent-coding">Coding agents</a><a href="#agent-controls">Tools & approvals</a><a href="#agent-faq">FAQs</a></div></nav>
      <section id="agent-directory" className="agents-directory">
        <div className="wrap">
          <SectionHead eyebrow="Meet your agents" title="Meet the agents that move your work forward." lede="Start with agents built for the work your team does every day. Each has a focused role, with tools and instructions you can configure." />
          <div className="agents-directory-grid">{SPECIALISTS.map(agent => <article id={`agent-${agent.icon}`} key={agent.icon}><div className="agents-directory-persona"><img src={`/new/agents/${agent.icon}.svg`} width={56} height={56} alt="" /></div><h3>{agent.role} · {agent.name}</h3><p>{agent.description}</p><div className="agents-directory-outcome">{agent.outcome}</div></article>)}</div>
          <div className="agents-custom-strip"><Settings2 size={22} aria-hidden="true" /><div><h3>A different job? Make an agent for it.</h3><p>Choose its instructions, skills, model, tools, and approval settings.</p></div><a href="#agent-controls">Configure your agents<ArrowRight size={15} /></a></div>

        </div>
      </section>
      <section id="agent-ask" className="agents-ask-section ask-agent-section">
        <div className="wrap">
          <SectionHead eyebrow="Ask Agent" title="Ask Agent: one question, the right agents on it." lede="Your starting point across the workspace. Ask Agent finds answers and handles work directly, bringing in specialists when the request calls for them." />
          <ExampleLabel /><figure className="ask-agent-preview">
            <ProductPreview product="ask-agent" theme="light" />
            <figcaption>An example rollout review: four specialist agents, one work plan, and a follow-up ready for review.</figcaption>
          </figure>
        </div>
      </section>
      <section id="agent-workflows" className="agents-workflows">
        <div className="wrap">
          <SectionHead eyebrow="Working together" title="Big requests get split up, run at once, and come back as one plan." lede="Ask Agent hands parts of the job to specialist agents, runs independent steps at the same time, and brings the findings back to one conversation." />
          <div className="agents-capability-grid">{CAPABILITIES.map(item => <article key={item.id}><div className="agents-capability-copy"><span className="eyebrow">{item.label}</span><h3>{item.title}</h3><p>{item.description}</p></div><AgentWorkflowArt variant={item.id} /><div className="agents-prompt"><span>Try asking</span><p>“{item.prompt}”</p></div></article>)}</div>
        </div>
      </section>
      <section id="agent-coding" className="agents-coding">
        <div className="wrap agents-split">
          <div><SectionHead eyebrow="Coding agents" title="From bug report to a fix your team reviews, with the customer attached." lede="The coding task planner refines the task, the coding agent prepares the change, and the code reviewer checks it. The customer conversation stays attached throughout." /><ul className="agents-benefits"><li><MessageSquare size={18} />Keep the original request attached.</li><li><Code2 size={18} />Work in a connected repository.</li><li><ShieldCheck size={18} />Review the changes before they ship.</li></ul><a className="agents-text-link" href={`${GITHUB_URL}/blob/develop/docs/coding-agent-execution.md`} target="_blank" rel="noopener noreferrer">Explore coding agents<ArrowRight size={15} /></a></div>
          <AgentWorkflowArt variant="coding" />
        </div>
      </section>
      <section id="agent-controls" className="agents-controls">
        <div className="wrap">
          <SectionHead eyebrow="Approvals built in" title="Decide what each agent can do, and when it asks first." lede="Give each agent a job, the tools it needs, and clear approval settings. Start work from a conversation or a configured trigger." />
          <div className="agents-split"><div className="agents-control-list">
            <article><Settings2 size={21} /><div><h3>Make the agent fit the work.</h3><p>Set its instructions, skills, and model. Build on an existing agent or configure your own.</p></div></article>
            <article><LockKeyhole size={21} /><div><h3>Select its tools and approvals.</h3><p>Choose built-in and external MCP tools. Decide which actions need your team’s review.</p></div></article>
            <article><Clock3 size={21} /><div><h3>Choose what starts the work.</h3><p>Run agents on demand, from supported events, or on a schedule. Follow progress and review the results.</p></div></article>
          </div><AgentWorkflowArt variant="approval" /></div>
        </div>
      </section>
      <section id="agent-faq"><div className="wrap agents-faq-grid"><SectionHead eyebrow="Questions" title="Before you hand it over." /><div className="agents-faqs"><FAQList items={FAQS} className="faq-items" /></div></div></section>
      <section className="final-cta final-cta-connected" aria-labelledby="agents-final-title"><div className="wrap"><ConnectedWorkspace /><div className="final"><span className="eyebrow">Keep the work connected</span><h2 id="agents-final-title">Ask once. The right agents take it from there.</h2><p className="lede">Bring your customer history, tools, and agents into one workspace.</p><CtaRow /><div className="assure"><span>Open source</span><span>Run it yourself or use our cloud</span><span>You approve what ships</span></div></div></div></section>
    </main>
    <PreviewFooter />
  </>;
}
