import { PreviewNav } from './_components/PreviewNav';
import { PreviewFooter } from './_components/PreviewFooter';
import { LoopWire } from './_components/LoopWire';
import { ProductExplorer } from './_components/ProductExplorer';
import { AgentControlArt } from './_components/AgentControlArt';
import { AskAgentBento } from './_components/AskAgentBento';
import { CustomerRecordBento } from './_components/CustomerRecordBento';
import { CtaRow, GithubIcon, SectionHead, GITHUB_URL } from './_components/ui';

const RECORD_FACTS = [
  {
    "title": "Conversations",
    "description": "Let Helpin AI answer first, tag conversations, and pass the full history to your team when needed.",
    "image": "conversations",
    "wide": true
  },
  {
    "title": "Meetings",
    "description": "Keep recordings, transcripts, decisions, and next steps attached to the customer.",
    "image": "meetings",
    "wide": false
  },
  {
    "title": "Projects",
    "description": "See the requests and conversations behind each project, alongside its tasks and progress.",
    "image": "projects",
    "wide": false
  },
  {
    "title": "Deals",
    "description": "See the conversations, objections, and buyer signals behind each deal and renewal.",
    "image": "deals",
    "wide": false
  },
  {
    "title": "Docs",
    "description": "See the guides customers read, the answers your team shared, and the questions still open.",
    "image": "docs",
    "wide": false
  },
  {
    "title": "Email & calendar",
    "description": "Follow customer emails, calls, and meetings on one timeline.",
    "image": "email-calendar",
    "wide": true
  }
] as const;

const OPEN_SOURCE = [
  ['Open source', 'Read the code. Change it. Build on it.'],
  ['Self-host', 'Run Helpin inside your own infrastructure.'],
  ['Helpin Cloud', "Let Helpin manage the infrastructure."],
  ['Bring your own models', 'Choose from supported model providers.'],
  ['Own your data', 'Choose where your customer and product data lives.'],
  ['Build your own workflows', 'Connect your tools through the API, SDK, webhooks, and MCP.'],
];

export default function NewHomePage() {
  return (
    <>
      <PreviewNav />

      {/* 01 Hero */}
      <section className="hero">
        <div className="wrap hero-wrap">
          <div className="wgrid" aria-hidden="true"><i /><i /><i /><i /><i /></div>
          <div className="hero-inner">
            <span className="eyebrow">The open-source workspace for SaaS teams</span>
            <h1><span>Support, product, and CRM.</span>{' '}<span>One customer history.</span>{' '}<span>AI that does the work.</span></h1>
            <p className="lede">AI agents answer customers, plan tasks, and prepare follow-ups — with the full context of every account behind each step.</p>
            <CtaRow />
            <div className="assure"><span>Open source</span><span>Self-hostable</span><span>Bring your own models</span><span>Cloud available</span></div>
          </div>
          <LoopWire />
        </div>
      </section>

      {/* 03 One customer record (dark) */}
      <section id="record" className="dark">
        <div className="wrap">
          <div className="rs-copy record-intro">
            <span className="rs-eyebrow">One customer. One record.</span>
            <h2>Every agent starts with the full picture.</h2>
            <p>Conversations, meetings, projects, deals, docs, and engineering activity connected around each customer. Your team, support agents, and coding agents work from the same history.</p>
          </div>
          <div className="record-bento">
            {RECORD_FACTS.map(({ title, description, image, wide }) => (
              <article className={`record-bento-card${wide ? ' record-bento-wide' : ''}`} key={title}>
                <div className="record-bento-copy">
                  <h3>{title}</h3>
                  <p>{description}</p>
                </div>
                <CustomerRecordBento variant={image} />
              </article>
            ))}
          </div>
          <p className="rs-close"><span className="rs-keep">One customer.</span> Every interaction. Full context.</p>
        </div>
      </section>

      {/* Explore each product area alongside the workspace image. */}
      <section id="product">
        <div className="wrap">
          <SectionHead eyebrow="What’s inside" title="Close the loop in one workspace." lede="Connect the customer question, the product work, and the follow-up—so your team doesn’t have to piece the story together across tools." />
          <ProductExplorer />
        </div>
      </section>

      {/* Ask Agent: answers, execution, and connected tools. */}
      <section id="ask-agent" className="ask-agent-section" aria-labelledby="ask-agent-title">
        <div className="wrap">
          <div className="ask-agent-intro">
            <span className="eyebrow">Ask Agent</span>
            <h2 id="ask-agent-title">Ask a question.<br />Hand off the work.</h2>
            <p className="lede">Ask about a customer, investigate an issue, or describe the work you need done. Ask Agent finds the context, coordinates specialist agents, and brings the results back to one conversation.</p>
          </div>
          <figure className="ask-agent-screenshot">
            <a href="/new/product/workspace-ask-agent-4k-v1.webp" target="_blank" rel="noopener noreferrer" aria-label="View the Ask Agent screenshot at full size (opens in a new tab)">
              <img src="/new/product/workspace-ask-agent-4k-v1.webp" alt="Helpin Ask Agent demo reviewing OrbitDesk’s SSO rollout, with four specialist sub-agents, findings, a completed five-step work plan, and a customer follow-up draft." width={3840} height={2016} loading="lazy" decoding="async" />
            </a>
            <figcaption>An example rollout review: four sub-agents, one work plan, and a follow-up ready for review.</figcaption>
          </figure>
          <div className="ask-agent-capabilities">
            <article>
              <span className="ask-agent-label">Your workspace</span>
              <h3>Find answers across your workspace.</h3>
              <p>Find the customer question, the meeting decision, and the task behind it. Ask Agent brings them together in one answer.</p>
              <AskAgentBento variant="answers" />
              <div className="ask-agent-example"><span>Try asking</span><blockquote>What’s blocking OrbitDesk’s rollout?</blockquote></div>
            </article>
            <article>
              <span className="ask-agent-label">Your agents</span>
              <h3>Coordinate agents from start to finish.</h3>
              <p>Break complex requests into a plan. Run sub-agents in parallel, sequence dependent steps, and bring their results back to the conversation.</p>
              <AskAgentBento variant="coordination" />
              <div className="ask-agent-example"><span>Try asking</span><blockquote>Review the rollout blockers, then create a plan from the findings.</blockquote></div>
            </article>
            <article>
              <span className="ask-agent-label">External MCP</span>
              <h3>Bring your other tools into the conversation.</h3>
              <p>Give agents selected tools from external MCP servers. Pull in external data and use it alongside customer history, tasks, and docs.</p>
              <AskAgentBento variant="mcp" />
              <div className="ask-agent-example"><span>With a connected agent</span><blockquote>Check the issue status in our connected tracker.</blockquote></div>
            </article>
          </div>
          <div className="ask-agent-links"><a className="btn-link" href="/new/product#agents">Explore agents →</a><a className="btn-link" href={`${GITHUB_URL}/blob/develop/docs/external-mcp-servers.md`} target="_blank" rel="noopener noreferrer">Connect external tools →</a></div>
        </div>
      </section>

      {/* Agent controls */}
      <section id="control">
        <div className="wrap">
          <SectionHead eyebrow="Agents, on your terms" title="Give agents work. Keep control."
            lede="Choose the tools each agent can use, when it needs approval, and what starts the work." />
          <figure className="control-screenshot">
            <a href="/new/product/agents-directory-4k-v1.webp" target="_blank" rel="noopener noreferrer" aria-label="View the Agents directory screenshot at full size (opens in a new tab)">
              <img src="/new/product/agents-directory-1920-v1.webp" srcSet="/new/product/agents-directory-1920-v1.webp 1920w, /new/product/agents-directory-4k-v1.webp 3840w" sizes="(max-width: 1120px) calc(100vw - 48px), 1072px" alt="Helpin’s Agents directory in the OrbitDesk demo workspace, showing eight agents, their configurations, recent runs, approval requests, and connected flows." width={3840} height={2016} loading="lazy" decoding="async" />
            </a>
            <figcaption>One place to manage your agents and see their latest runs.</figcaption>
          </figure>
          <div className="ctrl">
            <article>
              <AgentControlArt variant="tools" />
              <div className="ctrl-copy">
                <span className="k">Tool access</span>
                <h3>Give each agent the right tools.</h3>
                <p className="ctrl-description">Choose the workspace tools and connected services each agent can use to do its job.</p>
              </div>
            </article>
            <article>
              <AgentControlArt variant="approvals" />
              <div className="ctrl-copy">
                <span className="k">Approvals</span>
                <h3>Set when agents ask first.</h3>
                <p className="ctrl-description">Set an approval mode for each agent. Review actions that need permission before they go ahead.</p>
              </div>
            </article>
            <article>
              <AgentControlArt variant="triggers" />
              <div className="ctrl-copy">
                <span className="k">Triggers</span>
                <h3>Put repeat work on a schedule.</h3>
                <p className="ctrl-description">Start agents from an event or a schedule, using the tools and approval settings you’ve chosen.</p>
              </div>
            </article>
          </div>
          <p className="section-close"><a className="inline-link" href="/new/product#agents">Explore agents and automation →</a></p>
        </div>
      </section>

      {/* Open source */}
      <section id="open-source">
        <div className="wrap">
          <SectionHead eyebrow="Open by design" title="Your customer context should belong to you."
            lede="Run Helpin on your infrastructure or use ours. Choose your models and extend the product around your team." />
          <div className="six">
            {OPEN_SOURCE.map(([k, v]) => <div key={k}><h3>{k}</h3><p>{v}</p></div>)}
          </div>
          <div className="links"><a className="btn btn-primary" href={GITHUB_URL} target="_blank" rel="noopener noreferrer"><GithubIcon />View the repository →</a><a className="btn-link" href="/new/product#developers">API, SDK, MCP, and integrations →</a><span className="mono muted">AGPL-3.0</span></div>
        </div>
      </section>

      {/* 13 Final CTA */}
      <section>
        <div className="wrap">
          <div className="final">
            <span className="eyebrow">Close the loop</span>
            <h2>Start with a customer.<br />End with something shipped.</h2>
            <p className="lede">We built Helpin and the agent system behind it for our own products—to keep customer questions, product work, and follow-ups in one place.</p>
            <CtaRow />
            <div className="assure"><span>Open source</span><span>Self-hostable</span><span>Built for SaaS teams</span></div>
          </div>
        </div>
      </section>

      <PreviewFooter />
    </>
  );
}
