import { Braces, BrainCircuit, Cloud, Database, Plug, Server, SlidersHorizontal, Webhook } from 'lucide-react';
import { PreviewNav } from './_components/PreviewNav';
import { ConnectedWorkspace } from './_components/ConnectedWorkspace';
import { GridFlow } from './_components/GridFlow';
import { PreviewFooter } from './_components/PreviewFooter';
import { LoopWire } from './_components/LoopWire';
import { ProductExplorer } from './_components/ProductExplorer';
import { HostingDiagram } from './_components/HostingDiagram';
import { AgentDirectoryPreview } from './_components/agent-directory/AgentDirectoryPreview';
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
    "title": "Engineering",
    "description": "Give coding agents the customer context behind each task. Work in a connected repository, prepare changes, and bring a pull request back for review.",
    "image": "coding",
    "wide": true
  },
  {
    "title": "Deals",
    "description": "See the conversations, objections, and buyer signals behind each deal and renewal.",
    "image": "deals",
    "wide": false
  },
  {
    "title": "Knowledge",
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


export default function NewHomePage() {
  return (
    <>
      <PreviewNav />

      {/* 01 Hero */}
      <section className="hero">
        <div className="wrap hero-wrap">
          <div className="wgrid" aria-hidden="true"><i /><i /><i /><i /><i /></div>
          <div className="hero-inner">
            <span className="eyebrow">The customer workspace for teams and AI agents</span>
            <h1>Turn customer conversations into work that gets done.</h1>
            <p className="lede">AI agents answer customers, turn requests into work, and follow through—with your conversations, tasks, deals, docs, and customer history connected.</p>
            <CtaRow />
            <div className="assure hero-assure">
              <span><Braces size={15} aria-hidden="true" />Open source</span>
              <span><Server size={15} aria-hidden="true" />Self-hostable</span>
              <span><BrainCircuit size={15} aria-hidden="true" />Bring your own models</span>
              <span><Cloud size={15} aria-hidden="true" />Cloud available</span>
            </div>
          </div>
          <LoopWire />
        </div>
      </section>

      {/* Shared customer history: the context behind the work. */}
      <section id="record" className="dark">
        <GridFlow />
        <div className="wrap">
          <div className="rs-copy record-intro">
            <span className="rs-eyebrow">One customer. One record.</span>
            <h2>Your team and agents work from the same customer history.</h2>
            <p>Conversations, meetings, projects, deals, knowledge, and engineering activity—connected around each customer.</p>
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
              <span className="ask-agent-label">Connected tools</span>
              <h3>Work with tools outside Helpin.</h3>
              <p>Let agents check your connected tools and bring the result back into the same conversation.</p>
              <AskAgentBento variant="mcp" />
              <div className="ask-agent-example"><span>With a connected agent</span><blockquote>Check the issue status in our connected tracker.</blockquote></div>
            </article>
          </div>
          <div className="ask-agent-links"><a className="btn-link" href="/new/product#agents">Explore agents →</a><a className="btn-link ask-agent-technical-link" href={`${GITHUB_URL}/blob/develop/docs/external-mcp-servers.md`} target="_blank" rel="noopener noreferrer">Powered by MCP →</a></div>
        </div>
      </section>

      {/* Explore each product area alongside the workspace image. */}
      <section id="product">
        <div className="wrap">
          <SectionHead eyebrow="What’s inside" title="Everything your team needs to close the loop." lede="Explore the products that turn customer context into answers, actions, and follow-ups." />
          <ProductExplorer />
        </div>
      </section>

      {/* Agent controls */}
      <section id="control">
        <div className="wrap">
          <SectionHead eyebrow="Agents, on your terms" title="Give agents work. Keep control."
            lede="Choose the tools each agent can use, when it needs approval, and what starts the work." />
          <figure className="control-screenshot">
            <AgentDirectoryPreview />
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

      {/* Self-hosting */}
      <section id="open-source" className="dark self-host-section">
        <div className="wrap">
          <div className="self-host-intro">
            <div>
              <SectionHead eyebrow="Open source" title="Your customer history stays yours."
                lede="Run Helpin on your infrastructure and control your data and models. Choose Helpin Cloud when you’d rather leave the hosting to us." />
              <div className="links"><a className="btn btn-primary" href={`${GITHUB_URL}/blob/develop/community/README.md`} target="_blank" rel="noopener noreferrer">Self-host Helpin →</a><a className="btn-link" href={GITHUB_URL} target="_blank" rel="noopener noreferrer"><GithubIcon />View the code →</a></div>
              <p className="self-host-license">Open source · AGPL-3.0</p>
            </div>
            <HostingDiagram />
          </div>
          <div className="self-host-features">
            <article><Server size={23} aria-hidden="true" /><h3>Deploy with Docker.</h3><p>Self-host Community’s support, docs, and agents with Docker Compose.</p></article>
            <article><Database size={23} aria-hidden="true" /><h3>Keep your data with you.</h3><p>Own your customer history, files, and backups.</p></article>
            <article><SlidersHorizontal size={23} aria-hidden="true" /><h3>Choose your connections.</h3><p>Choose supported models and configure your connections.</p></article>
          </div>
        </div>
      </section>

      {/* Developer tools and installation CLI */}
      <section id="developers" className="developer-section">
        <div className="wrap">
          <SectionHead eyebrow="Built to be extended" title="Connect your stack. Build your own workflows."
            lede="Connect your product and tools with APIs, SDKs, MCP, and webhooks." />
          <article className="developer-cli-feature">
            <figure className="developer-visual">
              <a href="/new/product/helpin-cli-tilted-4k-v1.webp" target="_blank" rel="noopener noreferrer" aria-label="View the Helpin CLI illustration at full size (opens in a new tab)">
                <img src="/new/product/helpin-cli-tilted-1920-v1.webp" srcSet="/new/product/helpin-cli-tilted-1920-v1.webp 1920w, /new/product/helpin-cli-tilted-4k-v1.webp 3840w" sizes="(max-width: 700px) calc(100vw - 78px), (max-width: 1120px) 55vw, 590px" width={3840} height={2160} alt="An angled Helpin CLI terminal showing installation checks, diagnostics, status, and logs, with a softly blurred right edge." loading="lazy" decoding="async" />
              </a>
            </figure>
            <div className="developer-cli-copy">
              <span className="eyebrow">Helpin CLI</span>
              <h3>Your instance.<br />One terminal.</h3>
              <p>Install, configure, and manage your Helpin instance from the terminal.</p>
              <a className="btn-link" href={`${GITHUB_URL}/blob/develop/community/README.md`} target="_blank" rel="noopener noreferrer">Explore the Helpin CLI →</a>
            </div>
          </article>
          <div className="developer-features">
            <article>
              <span className="developer-feature-icon"><Braces size={22} aria-hidden="true" /></span>
              <h3>APIs &amp; SDKs</h3><p>Connect customer data and bring support into your product.</p>
              <a className="inline-link" href={`${GITHUB_URL}/blob/develop/docs/README.md`} target="_blank" rel="noopener noreferrer">Developer docs →</a>
              <a className="inline-link" href={`${GITHUB_URL}/tree/develop/packages/sdk-js`} target="_blank" rel="noopener noreferrer">Explore the SDK →</a>
            </article>
            <article>
              <span className="developer-feature-icon"><Plug size={22} aria-hidden="true" /></span>
              <h3>MCP, both ways</h3><p>Bring Helpin context to AI tools, and external tools to agents.</p>
              <a className="inline-link" href={`${GITHUB_URL}/blob/develop/docs/public-mcp-server.md`} target="_blank" rel="noopener noreferrer">Connect AI tools →</a>
              <a className="inline-link" href={`${GITHUB_URL}/blob/develop/docs/external-mcp-servers.md`} target="_blank" rel="noopener noreferrer">Connect external tools →</a>
            </article>
            <article>
              <span className="developer-feature-icon"><Webhook size={22} aria-hidden="true" /></span>
              <h3>Webhooks &amp; triggers</h3><p>Start agent runs from GitHub, GitLab, and workflow events.</p>
              <a className="inline-link" href={`${GITHUB_URL}/blob/develop/docs/agents-and-automation.md`} target="_blank" rel="noopener noreferrer">Explore automation →</a>
            </article>
          </div>
        </div>
      </section>

      {/* 13 Final CTA */}
      <section className="final-cta final-cta-connected" aria-labelledby="final-cta-title">
        <div className="wrap">
          <ConnectedWorkspace />
          <div className="final">
            <span className="eyebrow">Close the loop</span>
            <h2 id="final-cta-title">Every conversation has a next step.<br />Turn it into action.</h2>
            <p className="lede">Bring your customer history, team, and AI agents together to answer, decide, and get the work done.</p>
            <CtaRow />
            <div className="assure"><span>Open source</span><span>Self-hostable</span><span>Built for SaaS teams</span></div>
          </div>
        </div>
      </section>

      <PreviewFooter />
    </>
  );
}
