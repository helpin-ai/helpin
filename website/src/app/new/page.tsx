import { PreviewNav } from './_components/PreviewNav';
import { PreviewFooter } from './_components/PreviewFooter';
import { LoopWire } from './_components/LoopWire';
import { RecordStory } from './_components/RecordStory';
import { ReviewNote, Flag } from './_components/ReviewNotes';
import { CtaRow, GithubIcon, SectionHead, GITHUB_URL } from './_components/ui';

// Which record card each item lights up on hover or focus.
const RECORD_KINDS = ['conversations', 'meetings', 'projects', 'deal', 'docs', 'meetings'];

const RECORD_FACTS = [
  ['Conversations', 'See every chat, email, note, assignment, and reply in context.'],
  ['Meetings', 'Record, transcribe, summarize, and turn decisions and next steps into work.'],
  ['Projects', 'Turn customer needs into projects, tasks, priorities, owners, and progress.'],
  ['Deals', 'Connect pipeline activity, renewals, and buyer signals to the conversations behind them.'],
  ['Docs', 'See what customers read, what your team shared, and where new knowledge is needed.'],
  ['Email & calendar', 'Keep customer threads, calls, and meetings on the same timeline.'],
];

const AGENTS = [
  ['Support', 'Draft answers using customer history and product knowledge.'],
  ['Triage', 'Turn conversations into requests, bugs, tasks, and product work.'],
  ['Product', 'Find patterns across feedback and identify what customers keep asking for.'],
  ['Engineering', 'Carry the customer context behind an issue into development workflows.'],
  ['Docs', 'Update knowledge when the product changes.'],
  ['Customer', 'Find the people waiting for a fix or feature and prepare the follow-up.'],
];

const OPEN_SOURCE = [
  ['Open source', 'Read the code. Change it. Build on it.'],
  ['Self-host', 'Run Helpin inside your own infrastructure.'],
  ['Helpin Cloud', "Use Helpin without managing the deployment yourself."],
  ['Bring your own models', 'Connect supported AI providers and use the models that work for your team.'],
  ['Own your context', 'Keep control of the customer and product data your workflows depend on.'],
  ['Avoid platform lock-in', 'Your customer history shouldn’t disappear behind a closed system.'],
];

const DEVELOPERS = [
  ['REST API', "Build on Helpin’s customer, project, conversation, and workspace model."],
  ['SDK', 'Send customer and product context directly from your application.'],
  ['Webhooks', 'React to conversations, projects, tasks, customers, and product events.'],
  ['MCP', 'Let AI tools work with Helpin context and actions.'],
  ['GitHub & GitLab', 'Connect customer work directly to engineering.'],
  ['Docker', 'Run Helpin where your team runs software.'],
];

function Arrow() { return <span className="arrow" aria-hidden="true">→</span>; }

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
            <p className="lede">Connect customer conversations to product work, engineering, docs, and CRM.</p>
            <p className="lede lede-2">Helpin keeps the context attached as work moves — with your team in control.</p>
            <CtaRow />
            <div className="assure"><span>Open source</span><span>Self-hostable</span><span>Bring your own models</span><span>Cloud available</span></div>
          </div>
          <LoopWire />
        </div>
      </section>

      {/* 02 Product loop */}
      <section id="loop">
        <div className="wrap">
          <SectionHead eyebrow="From conversation to shipped" title="Keep customer context moving."
            lede="What customers tell you should shape what product decides, what engineering ships, and what customers hear next." secondaryLede="Helpin connects the whole loop." />
          <div className="loop4">
            <div>
              <span className="k">01 — Hear</span>
              <h3>Capture what customers are telling you.</h3>
              <p>Bring conversations, email, meetings, feedback, and buyer signals into one place.</p>
              <div className="ctrl-card"><b>Conversation received</b><p>“Does SSO work with Okta? We need it before rollout.”</p></div>
            </div>
            <div>
              <span className="k">02 — Decide</span>
              <h3>Turn the signal into work.</h3>
              <p>Identify requests, bugs, themes, and next steps. Connect them to the product work that follows.</p>
              <div className="ctrl-card"><b>Feature request detected</b><p>Okta SAML support</p><span>7 customers asking</span></div>
            </div>
            <div>
              <span className="k">03 — Ship</span>
              <h3>Give engineering the context.</h3>
              <p>Move work into projects, issues, coding agents, pull requests, and releases without losing the customer behind it.</p>
              <div className="ctrl-card"><b>SSO Enterprise Readiness</b><div className="flow"><Arrow /><span>GitHub issue #482</span><Arrow /><span>PR #728</span><Arrow /><span>Shipped</span></div></div>
            </div>
            <div>
              <span className="k">04 — Tell</span>
              <h3>Close the loop.</h3>
              <p>When work ships, Helpin knows who was waiting and what they need to hear.</p>
              <div className="ctrl-card"><b>7 customers affected</b><ul className="steps"><li>Docs updated</li><li>Replies prepared</li><li>Customers notified</li></ul></div>
            </div>
          </div>
        </div>
      </section>

      {/* 03 One customer record (dark) */}
      <section id="record" className="dark">
        <div className="wrap">
          <div className="rs-grid">
            <div className="rs-copy">
              <span className="rs-eyebrow">One customer. One record.</span>
              <h2>Everything your team knows about a customer, connected.</h2>
              <p>Conversations, meetings, projects, deals, docs, and engineering activity stay attached to the same customer record.</p>
              <p>Support sees what’s shipping. Product sees who’s asking. Sales sees what matters to the account.</p>
              <p className="rs-strong">And your agents work from the same context.</p>
            </div>
            <RecordStory />
          </div>
          <div className="rs-items">
            {RECORD_FACTS.map(([k, v], i) => <div key={k} data-kind={RECORD_KINDS[i]} tabIndex={0}><h3>{k}</h3><p>{v}</p></div>)}
          </div>
          <p className="rs-close"><span className="rs-keep">One customer.</span> Every interaction. Full context.</p>
        </div>
      </section>

      {/* 04 Product & project management */}
      <section id="projects">
        <div className="wrap">
          <SectionHead eyebrow="From feedback to roadmap" title="Turn customer feedback into product work."
            lede="Create projects and tasks from conversations, meetings, and customer requests — with the original context still attached." />
          <div className="pm-grid">
            <div className="pm-items">
              <div><h3>See the demand</h3><p>Know who is asking, how often it comes up, and which accounts it matters to.</p></div>
              <div><h3>Plan the work</h3><p>Organize projects, tasks, priorities, owners, and progress in the same workspace.</p></div>
              <div><h3>Connect engineering</h3><p>Link product work to GitHub, GitLab, issues, pull requests, releases, and coding agents.</p></div>
              <div><h3>Know who is waiting</h3><p>When something ships, see every customer who asked for it.</p></div>
            </div>
            <aside className="pm-panel" aria-label="Project SSO Enterprise Readiness with linked customer requests">
              <div className="pm-head"><span className="k">Project</span><b>SSO Enterprise Readiness</b><span className="muted">8 / 12 tasks · owner Sam K. · due Oct 3</span></div>
              <div className="rs-bar light"><i className="on" style={{ width: '66.7%' }} /></div>
              <div className="pm-meta"><span><b>7</b> customer requests</span><span><b>3</b> linked engineering issues</span><span><b>3</b> renewals depend on it</span></div>
              <ul className="rs-tasks light">
                <li><span>Okta SAML mapping</span><em className="prog">In progress · PR #728</em></li>
                <li><span>SCIM provisioning</span><em>Planned</em></li>
                <li><span>Role mapping</span><em className="done">Shipped</em></li>
              </ul>
              <div className="pm-who"><span className="k">Asked for this</span><div className="avs"><img src="/new/avatars/maya.webp" alt="" /><img src="/new/avatars/dev.webp" alt="" /><img src="/new/avatars/lin.webp" alt="" /><img src="/new/avatars/aisha.webp" alt="" /><span>+3</span></div></div>
            </aside>
          </div>
          <div className="flow big"><span>Feedback</span><Arrow /><span>Project</span><Arrow /><span>Engineering</span><Arrow /><span>Shipped</span><Arrow /><span>Customer</span></div>
          <ReviewNote tag="04">
            <p><Flag>VERIFY</Flag> "Structured product requests" as a distinct object, request counts per project, and "renewals depend on it" roll-ups are not confirmed in code. Projects, tasks, epics, owners, states, GitHub and GitLab links, and task-from-conversation are.</p>
          </ReviewNote>
        </div>
      </section>

      <section id="meetings">
        <div className="wrap">
          <SectionHead eyebrow="Every call becomes context" title="Turn customer meetings into work." lede="Helpin joins your calls, captures what was said, and connects the outcome to the customer." />
          <ul className="steps big"><li>Record the conversation.</li><li>Get a speaker-attributed transcript.</li><li>Summarize decisions, objections, and next steps.</li><li>Create tasks and product work from action items.</li><li>Keep the meeting connected to the customer, deal, and projects that matter.</li></ul>
          <p className="section-close">Google Meet · Zoom · Microsoft Teams · Webex</p>
        </div>
      </section>

      <section id="inbox">
        <div className="wrap">
          <SectionHead eyebrow="More than a support inbox" title="Answer the customer. Keep the context." lede="Handle chat and email from one shared inbox without disconnecting support from the rest of the company." />
          <ul className="steps big"><li>Assign conversations.</li><li>Add notes and tags.</li><li>Use saved replies.</li><li>See customer history.</li><li>Create product work.</li><li>Ask an agent for help.</li></ul>
          <p className="section-close">When the conversation becomes something bigger, the context goes with it.</p>
        </div>
      </section>

      <section id="crm">
        <div className="wrap">
          <SectionHead eyebrow="CRM with the conversation attached" title="Know what’s happening before the next sales call." lede="Manage companies, contacts, deals, stages, and renewals alongside the customer activity that explains them." />
          <ul className="steps big"><li>See the support issue holding up a deal.</li><li>See the feature request tied to a renewal.</li><li>See the meeting where the objection came up.</li><li>See what changed before you follow up.</li></ul>
          <p className="section-close">The record tells you more than the stage.</p>
        </div>
      </section>

      {/* AI agents */}
      <section id="agents">
        <div className="wrap">
          <SectionHead eyebrow="Agents with context" title="Give agents the context your team already has."
            lede="Helpin agents work across customers, conversations, projects, docs, meetings, and engineering activity." secondaryLede="They don’t start every task from scratch." />
          <div className="six">
            {AGENTS.map(([k, v]) => <div key={k}><h3>{k}</h3><p>{v}</p></div>)}
          </div>
          <p className="section-close">Different agents. Shared context.</p>
          <ReviewNote tag="05">
            <p><Flag>VERIFY</Flag> Shipped presets are Support, Documentation, Epic planner, Coding task planner, Code builder, Review, CRM operator, Marketer, Command, Ask, Researcher. "Triage Agent," "Product Agent," "Engineering Agent," and "Customer Agent" are marketing names for capabilities spread across those presets and triage rules; confirm the mapping or rename.</p>
          </ReviewNote>
        </div>
      </section>

      {/* 06 AI control */}
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
          <p className="section-close">Automate what you trust. Keep control of the rest.</p>
          <ReviewNote tag="06">
            <p><Flag>CONFIRMED</Flag> Per-agent approval modes, approval and review-checkpoint prompts, automation rules with triggers and actions. <Flag>VERIFY</Flag> An "affected customers identified → follow-up sent" automation as a shipped workflow.</p>
          </ReviewNote>
        </div>
      </section>

      <section id="knowledge">
        <div className="wrap">
          <SectionHead eyebrow="Knowledge that stays current" title="Turn what your team learns into answers." lede="Create public help-center content and internal docs alongside the conversations and product work that produce them." />
          <ul className="steps big"><li>See which articles customers used.</li><li>Find unanswered questions.</li><li>Create documentation from repeated conversations.</li><li>Update docs when the product changes.</li><li>Give agents the same knowledge your team uses.</li></ul>

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
          <div className="links"><a className="btn btn-primary" href={GITHUB_URL} target="_blank" rel="noopener noreferrer"><GithubIcon />View the repository →</a><span className="mono muted">AGPL-3.0</span></div>
        </div>
      </section>

      {/* 08 Developers */}
      <section id="developers">
        <div className="wrap">
          <SectionHead eyebrow="Built to be extended" title="Connect Helpin to the rest of your stack."
            lede="Use the API, SDK, webhooks, MCP, and developer integrations to bring Helpin into your existing workflows." />
          <div className="six">
            {DEVELOPERS.map(([k, v]) => <div key={k}><h3>{k}</h3><p>{v}</p></div>)}
          </div>
          <div className="links"><a className="btn-link" href={`${GITHUB_URL}/blob/develop/docs/README.md`}>Read the docs →</a></div>
          <ReviewNote tag="08">
            <p><Flag>CONFIRMED</Flag> API routes, SDK packages, MCP server, GitHub and GitLab, Docker Compose. <Flag>VERIFY</Flag> Public REST API documentation and outbound webhooks were not found in the audit.</p>
          </ReviewNote>
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
