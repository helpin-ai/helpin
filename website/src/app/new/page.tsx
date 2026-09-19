import { PreviewNav } from './_components/PreviewNav';
import { PreviewFooter } from './_components/PreviewFooter';
import { LoopWire } from './_components/LoopWire';
import { ReviewNote, Flag } from './_components/ReviewNotes';
import { CtaRow, GithubIcon, SectionHead, GITHUB_URL } from './_components/ui';

const RECORD_FACTS = [
  {
    "title": "Conversations",
    "description": "See every chat, email, note, assignment, and reply in context.",
    "image": "conversations",
    "alt": "Maya’s Okta question, a linked SSO project, an internal note, and a teammate assignment in one conversation.",
    "wide": true
  },
  {
    "title": "Meetings",
    "description": "Record, transcribe, summarize, and turn decisions and next steps into work.",
    "image": "meetings",
    "alt": "Acme’s security review with a speaker-attributed transcript, decisions, and two action items.",
    "wide": false
  },
  {
    "title": "Projects",
    "description": "Turn customer needs into projects, tasks, priorities, owners, and progress.",
    "image": "projects",
    "alt": "SSO Enterprise Readiness with customer requests, task statuses, and eight of twelve tasks complete.",
    "wide": false
  },
  {
    "title": "Deals",
    "description": "Connect pipeline activity, renewals, and buyer signals to the conversations behind them.",
    "image": "deals",
    "alt": "Acme’s $42,000 enterprise renewal, negotiation stage, and security requirement linked to Maya’s conversation.",
    "wide": false
  },
  {
    "title": "Docs",
    "description": "See what customers read, what your team shared, and where new knowledge is needed.",
    "image": "docs",
    "alt": "A published Okta setup article shared with Maya, its linked product update, and a customer question revealing a knowledge gap.",
    "wide": false
  },
  {
    "title": "Email & calendar",
    "description": "Keep customer threads, calls, and meetings on the same timeline.",
    "image": "email-calendar",
    "alt": "Acme’s customer timeline connecting Maya’s email, the security review meeting, and the shared setup guide.",
    "wide": true
  }
];

const OPEN_SOURCE = [
  ['Open source', 'Read the code. Change it. Build on it.'],
  ['Self-host', 'Run Helpin inside your own infrastructure.'],
  ['Helpin Cloud', "Use Helpin without managing the deployment yourself."],
  ['Bring your own models', 'Connect supported AI providers and use the models that work for your team.'],
  ['Own your context', 'Keep control of the customer and product data your workflows depend on.'],
  ['Avoid platform lock-in', 'Your customer history shouldn’t disappear behind a closed system.'],
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
            <h1>Hear customers.<br />Decide what matters.<br />Ship it.</h1>
            <p className="lede">Connect customer conversations to product work, engineering, docs, and CRM. Helpin keeps the context attached as work moves — with your team in control.</p>
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
            <h2>Everything your team knows about a customer, connected.</h2>
            <p>Conversations, meetings, projects, deals, docs, and engineering activity stay attached to the same customer record.</p>
            <p>Support sees what’s shipping. Product sees who’s asking. Sales sees what matters to the account.</p>
            <p className="rs-strong">And your agents work from the same context.</p>
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

      {/* Product areas: one line each, detail lives on the product page. */}
      <section id="product">
        <div className="wrap">
          <SectionHead eyebrow="What’s inside" title="One workspace. Five places the context lives." lede="Each area stands on its own. Together they keep the customer attached to the work." />
          <div className="img-ph" role="img" aria-label="Product image placeholder">
            <span className="img-ph-l">Product image</span>
            <span className="img-ph-m">2240 × 1260 · replace with a workspace screenshot from the seeded demo</span>
          </div>
          <div className="pstrip">
            <a href="/new/product#inbox"><h3>Inbox</h3><p>Chat and email in one shared inbox, with the customer’s history beside every reply.</p><span>Learn more →</span></a>
            <a href="/new/product#meetings"><h3>Meetings</h3><p>Calls recorded, transcribed, summarized, and turned into work.</p><span>Learn more →</span></a>
            <a href="/new/product#projects"><h3>Projects</h3><p>Customer requests become projects and tasks that reach GitHub and GitLab with the why attached.</p><span>Learn more →</span></a>
            <a href="/new/product#crm"><h3>CRM</h3><p>Deals, renewals, and buyer signals next to the conversations that explain them.</p><span>Learn more →</span></a>
            <a href="/new/product#knowledge"><h3>Knowledge</h3><p>Help-center and internal docs that grow from what customers ask.</p><span>Learn more →</span></a>
          </div>
        </div>
      </section>

      {/* AI control */}
      <section id="control">
        <div className="wrap">
          <SectionHead eyebrow="You set the boundaries" title="Decide how much the AI can do." lede="Start with suggestions. Add approvals. Automate workflows when you’re ready." />
          <div className="ctrl">
            <div>
              <span className="k">Ask</span>
              <h3>AI suggests. You decide.</h3>
              <div className="ctrl-card"><p>Acme may be reporting a regression. Create an engineering issue?</p><div className="ctrl-btns"><span className="btnm">Create issue</span><span className="btno">Dismiss</span></div></div>
              <p className="muted">Nothing happens until you approve it.</p>
            </div>
            <div>
              <span className="k">Approve</span>
              <h3>AI prepares the work.</h3>
              <div className="ctrl-card"><p>PR #728 resolves an issue reported by seven customers. Send them an update?</p><div className="ctrl-btns"><span className="btnm">Approve</span><span className="btno">Edit</span></div></div>
              <p className="muted">Review the action before it happens.</p>
            </div>
            <div>
              <span className="k">Auto</span>
              <h3>Let trusted workflows run.</h3>
              <ol className="chain"><li>Issue resolved</li><li>Docs updated</li><li>Affected customers found</li><li>Follow-up sent</li></ol>
            </div>
          </div>
          <p className="section-close">Automate what you trust. Keep control of the rest. <a className="inline-link" href="/new/product#agents">Meet the agents →</a></p>
          <ReviewNote tag="AI control">
            <p><Flag>CONFIRMED</Flag> Per-agent approval modes, approval and review-checkpoint prompts, automation rules with triggers and actions. <Flag>VERIFY</Flag> An "affected customers found → follow-up sent" automation as a shipped workflow.</p>
          </ReviewNote>
        </div>
      </section>

      {/* Open source */}
      <section id="open-source">
        <div className="wrap">
          <SectionHead eyebrow="Open by design" title="Your customer context should belong to you."
            lede="Run Helpin on your infrastructure or use ours." secondaryLede="Inspect the code. Extend the product. Connect your models. Build your own workflows." />
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
            <p className="lede">Keep the customer context connected from the first conversation to the work that follows.</p>
            <CtaRow />
            <div className="assure"><span>Open source</span><span>Self-hostable</span><span>Built for SaaS teams</span></div>
          </div>
        </div>
      </section>

      <PreviewFooter />
    </>
  );
}
