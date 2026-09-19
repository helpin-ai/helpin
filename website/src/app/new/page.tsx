import { PreviewNav } from './_components/PreviewNav';
import { PreviewFooter } from './_components/PreviewFooter';
import { LoopWire } from './_components/LoopWire';
import { ProductExplorer } from './_components/ProductExplorer';
import { ReviewNote, Flag } from './_components/ReviewNotes';
import { CtaRow, GithubIcon, SectionHead, GITHUB_URL } from './_components/ui';

const RECORD_FACTS = [
  {
    "title": "Conversations",
    "description": "Keep chats, emails, replies, and internal notes together with the customer behind them.",
    "image": "conversations",
    "alt": "Maya’s Okta question, a linked SSO project, an internal note, and a teammate assignment in one conversation.",
    "wide": true
  },
  {
    "title": "Meetings",
    "description": "Keep recordings, transcripts, decisions, and next steps attached to the customer.",
    "image": "meetings",
    "alt": "Acme’s security review with a speaker-attributed transcript, decisions, and two action items.",
    "wide": false
  },
  {
    "title": "Projects",
    "description": "See the requests and conversations behind each project, alongside its tasks and progress.",
    "image": "projects",
    "alt": "SSO Enterprise Readiness with customer requests, task statuses, and eight of twelve tasks complete.",
    "wide": false
  },
  {
    "title": "Deals",
    "description": "See the conversations, objections, and buyer signals behind each deal and renewal.",
    "image": "deals",
    "alt": "Acme’s $42,000 enterprise renewal, negotiation stage, and security requirement linked to Maya’s conversation.",
    "wide": false
  },
  {
    "title": "Docs",
    "description": "See the guides customers read, the answers your team shared, and the questions still open.",
    "image": "docs",
    "alt": "A published Okta setup article shared with Maya, its linked product update, and a customer question revealing a knowledge gap.",
    "wide": false
  },
  {
    "title": "Email & calendar",
    "description": "Follow customer emails, calls, and meetings on one timeline.",
    "image": "email-calendar",
    "alt": "Acme’s customer timeline connecting Maya’s email, the security review meeting, and the shared setup guide.",
    "wide": true
  }
];

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
            <h1>Customer support, product work, and CRM—for teams and agents.</h1>
            <p className="lede">Work with agents to answer customers, plan tasks, and prepare follow-ups. Helpin connects the customer history behind every step.</p>
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
            <p>Conversations, meetings, projects, deals, and docs connected around each customer. Your team and agents work from the same history.</p>
          </div>
          <div className="record-bento">
            {RECORD_FACTS.map(({ title, description, image, alt, wide }) => (
              <article className={`record-bento-card${wide ? ' record-bento-wide' : ''}`} key={title}>
                <div className="record-bento-copy">
                  <h3>{title}</h3>
                  <p>{description}</p>
                </div>
                <div className="record-bento-art">
                  <img
                    src={`/new/bento/customer-${image}-v2.webp`}
                    alt={alt}
                    width={1536}
                    height={1024}
                    loading="lazy"
                    decoding="async"
                  />
                </div>
              </article>
            ))}
          </div>
          <p className="rs-close"><span className="rs-keep">One customer.</span> Every interaction. Full context.</p>
        </div>
      </section>

      {/* Explore each product area alongside the workspace image. */}
      <section id="product">
        <div className="wrap">
          <SectionHead eyebrow="What’s inside" title="One workspace. From first reply to follow-up." lede="Follow a customer request through the inbox, a meeting, a project, and the next follow-up." />
          <ProductExplorer />
          <div id="ask-agent" className="ask-agent-intro">
            <SectionHead eyebrow="Ask Agent" title="Ask a question. Hand off the work." lede="Ask about a customer, investigate an issue, or describe a task. Ask Agent uses your workspace context to find answers and coordinate the work—from wherever you’re working." />
            <a className="btn-link" href="/new/product#agents">Explore agents →</a>
          </div>
        </div>
      </section>

      {/* AI control */}
      <section id="control">
        <div className="wrap">
          <SectionHead eyebrow="You set the boundaries" title="You decide what agents can do." lede="Start with suggestions. Add approvals. Automate workflows when you’re ready." />
          <div className="ctrl">
            <div>
              <span className="k">Suggestions</span>
              <h3>Agents suggest. You decide.</h3>
              <div className="ctrl-card"><p>OrbitDesk may be reporting a regression. Create an engineering issue?</p><div className="ctrl-btns"><span className="btnm">Create issue</span><span className="btno">Dismiss</span></div></div>
              <p className="muted">Nothing happens until you approve it.</p>
            </div>
            <div>
              <span className="k">Approvals</span>
              <h3>Review before agents act.</h3>
              <div className="ctrl-card"><p>PR #728 resolves an issue reported by seven customers. Send them an update?</p><div className="ctrl-btns"><span className="btnm">Approve</span><span className="btno">Edit</span></div></div>
              <p className="muted">Review the action before it happens.</p>
            </div>
            <div>
              <span className="k">Automation</span>
              <h3>Let trusted workflows run.</h3>
              <ol className="chain"><li>Task received</li><li>Agent does the work</li><li>Results ready for your team</li></ol>
            </div>
          </div>
          <p className="section-close">Automate what you trust. Keep control of the rest. <a className="inline-link" href="/new/product#agents">Meet the agents →</a></p>
          <ReviewNote tag="AI control">
            <p><Flag>CONFIRMED</Flag> Per-agent approval modes, approval and review-checkpoint prompts, automation rules with triggers and actions.</p>
          </ReviewNote>
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
            <p className="lede">Bring your team and agents together to answer customers, act on requests, and follow through.</p>
            <CtaRow />
            <div className="assure"><span>Open source</span><span>Self-hostable</span><span>Built for SaaS teams</span></div>
          </div>
        </div>
      </section>

      <PreviewFooter />
    </>
  );
}
