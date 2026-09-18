import Link from 'next/link';
import { PreviewNav } from './_components/PreviewNav';
import { PreviewFooter } from './_components/PreviewFooter';
import { LoopWire } from './_components/LoopWire';
import { RecordStory } from './_components/RecordStory';
import { ReviewNote, Flag } from './_components/ReviewNotes';
import { CtaRow, GithubIcon, SectionHead, SIGNUP_URL, GITHUB_URL } from './_components/ui';

// Which record card each item lights up on hover or focus.
const RECORD_KINDS = ['conversations', 'meetings', 'projects', 'deal', 'docs', 'meetings'];

const RECORD_FACTS = [
  ['Conversations', 'Every support conversation, email, note, assignment, and reply — connected to the customer and company.'],
  ['Meetings', 'Recordings, transcripts, summaries, decisions, objections, and next steps from your customer calls.'],
  ['Projects', 'Full product and project work — projects, tasks, priorities, owners, and progress — with the customer context that created it still attached.'],
  ['Deals', 'Pipeline activity, stages, renewals, and buyer signals connected back to their source.'],
  ['Docs', 'Help-center and internal knowledge, including what customers read and the gaps their questions uncover.'],
  ['Email & calendar', 'Customer threads, meetings, and appointments on the same record as everything else.'],
];

const AGENTS = [
  ['Support Agent', 'Answer questions using customer history, product knowledge, conversations, and documentation.'],
  ['Triage Agent', 'Turn incoming conversations into requests, bugs, projects, tasks, and structured feedback.'],
  ['Product Agent', 'Find patterns across customer requests and help your team understand what customers actually need.'],
  ['Engineering Agent', 'Carry the original customer context into issues and development workflows.'],
  ['Docs Agent', 'Keep documentation aligned with what changes in the product.'],
  ['Customer Agent', 'Know who should hear about fixes, releases, and requested features.'],
];

const OPEN_SOURCE = [
  ['Open source', 'Inspect it. Extend it. Change it.'],
  ['Self-host', 'Run Helpin inside your own infrastructure.'],
  ['Helpin Cloud', "Use the hosted version when you don't want to manage infrastructure."],
  ['Bring your own models', 'Connect the AI providers and models your team prefers.'],
  ['Your data', 'Keep control of your customer and product context.'],
  ['No lock-in', "Your customer history and workflows shouldn't depend on a closed platform."],
];

const DEVELOPERS = [
  ['REST API', "Build directly on Helpin's workspace and customer model."],
  ['SDK', 'Send product and customer context directly from your application.'],
  ['Webhooks', 'React to customer, conversation, project, task, and product events.'],
  ['MCP', 'Give AI tools access to Helpin context and actions.'],
  ['GitHub & GitLab', 'Connect customer work directly to engineering.'],
  ['Docker', 'Run Helpin wherever your team runs software.'],
];

const INTEGRATIONS: [string, string[]][] = [
  ['Communication', ['Email', 'Slack']],
  ['Engineering', ['GitHub', 'GitLab']],
  ['AI', ['OpenAI', 'Anthropic', 'Gemini', 'OpenRouter']],
  ['CRM', ['HubSpot', 'Salesforce']],
  ['Your product', ['API', 'SDK', 'Webhooks']],
];

const FAQS: [string, string[]][] = [
  ['What is Helpin?', ['Helpin is an open-source workspace for SaaS teams that connects customer conversations, meetings, product work, engineering, documentation, and CRM around one customer record.']],
  ['Is Helpin a customer support platform?', ['Support is part of Helpin.', 'The larger idea is connecting what customers say to what your company does next.', 'A support conversation can become product work, engineering activity, documentation, and ultimately a customer follow-up without losing the original context.']],
  ['Does Helpin include product management?', ['Yes.', 'Product teams can organize customer requests into projects and tasks, manage priorities and ownership, and connect that work to engineering and the customers who requested it.']],
  ['Why open source?', ['Customer context is some of the most important information inside a SaaS company.', 'Open source gives teams more control over where that data lives, how the product is deployed, and how AI interacts with their workflows.']],
  ['Can I self-host Helpin?', ['Yes.', 'You can run Helpin on your own infrastructure or use Helpin Cloud.']],
  ['Can we use our own AI models?', ['Helpin is designed around model flexibility, so teams can connect supported model providers rather than being locked into a single provider.']],
  ['Will AI take actions automatically?', ['Only when you allow it.', 'Helpin workflows can range from recommendations, to approval-based actions, to automation for workflows your team trusts.']],
  ['Does Helpin replace our existing tools?', ['It can consolidate parts of your customer, product, and knowledge stack over time.', 'It can also connect with the tools your team wants to keep.']],
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
            <p className="lede">Support, product, engineering, docs, and CRM share one customer context.</p>
            <p className="lede lede-2">Helpin's AI agents carry that context from conversation to product work to pull request to customer answer — while you decide what gets executed.</p>
            <CtaRow />
            <div className="assure"><span>Open source</span><span>Self-hostable</span><span>Bring your own models</span><span>Cloud available</span></div>
          </div>
          <LoopWire />
        </div>
      </section>

      {/* 02 Product loop */}
      <section id="loop">
        <div className="wrap">
          <SectionHead eyebrow="From conversation to shipped" title="Customer context shouldn't stop at the inbox."
            lede="What customers tell you should influence what your team decides, what engineering ships, and what customers hear next. Helpin keeps that context moving." />
          <div className="loop4">
            <div>
              <span className="k">01 — Hear</span>
              <h3>Capture what customers are telling you.</h3>
              <p>Conversations, meetings, emails, support requests, feedback, and buyer signals all become part of the same customer context.</p>
              <div className="tags"><span>Chat</span><span>Email</span><span>Meetings</span><span>Feedback</span><span>CRM</span></div>
            </div>
            <div>
              <span className="k">02 — Decide</span>
              <h3>Turn conversations into work.</h3>
              <p>Helpin identifies requests, bugs, recurring themes, action items, and opportunities — then connects them to the work your team is already doing.</p>
              <ul className="steps"><li>Request identified</li><li>Next action suggested</li><li>Product work created</li></ul>
            </div>
            <div>
              <span className="k">03 — Ship</span>
              <h3>Give engineering the why, not just the ticket.</h3>
              <p>Move customer context into product work, GitHub, GitLab, coding agents, pull requests, and releases. The original conversation stays attached.</p>
              <div className="flow"><span>Customer request</span><Arrow /><span>Project</span><Arrow /><span>Issue</span><Arrow /><span>PR</span><Arrow /><span>Release</span></div>
            </div>
            <div>
              <span className="k">04 — Tell</span>
              <h3>Close the loop.</h3>
              <p>When something ships, Helpin knows who asked for it, which conversations are waiting, what documentation changed, and who should hear about it.</p>
              <ul className="steps"><li>Update the docs.</li><li>Draft the reply.</li><li>Tell the customer.</li><li>Then listen again.</li></ul>
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
              <h2>The customer record your whole company works from.</h2>
              <p>Helpin connects every conversation, meeting, project, deal, document, and interaction to the customer behind it.</p>
              <p>Support, product, engineering, and sales work from the same context.</p>
              <p className="rs-strong">So do your AI agents.</p>
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
          <SectionHead eyebrow="From feedback to roadmap" title="Product work starts with customer context."
            lede="A feature request shouldn't become an anonymous task the moment it reaches product. Helpin keeps the customer, conversation, commercial context, and engineering work attached all the way through." />
          <div className="pm-grid">
            <div className="pm-items">
              <div><h3>Capture demand</h3><p>Turn conversations, meetings, and feedback into structured product requests.</p><ul className="steps"><li>See who is asking.</li><li>See how often.</li><li>See which accounts it matters to.</li></ul></div>
              <div><h3>Plan the work</h3><p>Organize requests into projects. Set priorities, owners, statuses, timelines, and tasks. Give product teams the complete context behind the work.</p></div>
              <div><h3>Connect engineering</h3><p>Link projects and tasks to GitHub or GitLab issues, pull requests, releases, and coding agents.</p>
                <div className="contrast"><div><span className="k">Engineering sees more than</span><q>Build SSO.</q></div><div><span className="k">They see</span><q>Seven customers requested SSO. Three enterprise renewals depend on it.</q></div></div>
              </div>
              <div><h3>Know who is waiting</h3><p>When work ships, Helpin already knows which customers asked for it.</p><ul className="steps"><li>No spreadsheets.</li><li>No searching old support conversations.</li><li>No forgotten follow-ups.</li></ul></div>
            </div>
            <aside className="pm-panel" aria-label="Project SSO Enterprise Readiness with linked customer requests">
              <div className="pm-head"><span className="k">Project</span><b>SSO Enterprise Readiness</b><span className="muted">8 / 12 tasks · owner Sam K. · due Oct 3</span></div>
              <div className="rs-bar light"><i className="on" style={{ width: '66.7%' }} /></div>
              <div className="pm-meta"><span><b>7</b> customer requests</span><span><b>3</b> linked engineering issues</span><span><b>3</b> renewals depend on it</span></div>
              <ul className="rs-tasks light">
                <li><span>Okta SAML mapping</span><em className="prog">In progress · PR #482</em></li>
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

      {/* 05 AI agents */}
      <section id="agents">
        <div className="wrap">
          <SectionHead eyebrow="Agents with context" title="AI works better when it knows the customer."
            lede="Most AI assistants start every task with a fraction of the story. Helpin agents work with the same customer, conversation, product, project, documentation, and engineering context as your team." />
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
          <SectionHead eyebrow="You set the boundaries" title="You decide when AI acts." lede="Start with suggestions. Add approvals. Automate the workflows you trust." />
          <div className="ctrl">
            <div>
              <span className="k">Ask</span>
              <h3>AI recommends. You decide.</h3>
              <div className="ctrl-card"><p>Acme appears to be reporting a regression. Create an engineering issue?</p><div className="ctrl-btns"><span className="btnm">Create issue</span><span className="btno">Dismiss</span></div></div>
              <p className="muted">Nothing happens without your approval.</p>
            </div>
            <div>
              <span className="k">Approve</span>
              <h3>AI prepares the work.</h3>
              <div className="ctrl-card"><p>PR #482 fixes an issue reported by seven customers. Send them an update?</p><div className="ctrl-btns"><span className="btnm">Approve</span><span className="btno">Edit</span></div></div>
              <p className="muted">Your team reviews the action before it happens.</p>
            </div>
            <div>
              <span className="k">Auto</span>
              <h3>Trusted workflows run on their own.</h3>
              <ol className="chain"><li>Bug fixed</li><li>Documentation updated</li><li>Affected customers identified</li><li>Follow-up sent</li></ol>
            </div>
          </div>
          <p className="section-close">Automation should be earned, not assumed.</p>
          <ReviewNote tag="06">
            <p><Flag>CONFIRMED</Flag> Per-agent approval modes, approval and review-checkpoint prompts, automation rules with triggers and actions. <Flag>VERIFY</Flag> An "affected customers identified → follow-up sent" automation as a shipped workflow.</p>
          </ReviewNote>
        </div>
      </section>

      {/* 07 Open source */}
      <section id="open-source">
        <div className="wrap">
          <SectionHead eyebrow="Open by design" title="Your customer context should belong to you."
            lede="Helpin is open source and designed to run on your infrastructure or ours. Inspect the code. Extend the product. Connect your own models. Build your own workflows and agents." />
          <div className="six">
            {OPEN_SOURCE.map(([k, v]) => <div key={k}><h3>{k}</h3><p>{v}</p></div>)}
          </div>
          <div className="links"><a className="btn btn-primary" href={GITHUB_URL} target="_blank" rel="noopener noreferrer"><GithubIcon />Explore the repository →</a><span className="mono muted">AGPL-3.0</span></div>
        </div>
      </section>

      {/* 08 Developers */}
      <section id="developers">
        <div className="wrap">
          <SectionHead eyebrow="Built to be extended" title="Make Helpin part of your stack."
            lede="Connect your product, internal tools, agents, customer data, and engineering workflows. You shouldn't have to wait for us to build every integration." />
          <div className="six">
            {DEVELOPERS.map(([k, v]) => <div key={k}><h3>{k}</h3><p>{v}</p></div>)}
          </div>
          <div className="links"><a className="btn-link" href={`${GITHUB_URL}/blob/develop/docs/README.md`}>Read the docs →</a></div>
          <ReviewNote tag="08">
            <p><Flag>CONFIRMED</Flag> API routes, SDK packages, MCP server, GitHub and GitLab, Docker Compose. <Flag>VERIFY</Flag> Public REST API documentation and outbound webhooks were not found in the audit.</p>
          </ReviewNote>
        </div>
      </section>

      {/* 09 Integrations */}
      <section id="integrations">
        <div className="wrap">
          <SectionHead eyebrow="Work with your stack" title="Keep the tools that still make sense."
            lede="Helpin connects the systems where customer and product work already happens. Bring the context together without rebuilding your entire stack on day one." />
          <div className="intg">
            {INTEGRATIONS.map(([g, items]) => (
              <div key={g}><span className="k">{g}</span><ul>{items.map((it) => <li key={it}>{it}</li>)}</ul></div>
            ))}
          </div>
          <ReviewNote tag="09">
            <p><Flag>NOT FOUND</Flag> Slack, HubSpot, and Salesforce integrations do not exist in the codebase; Gemini is reachable only through OpenRouter. <Flag>CONFIRMED</Flag> Email, GitHub, GitLab, OpenAI, Anthropic, OpenRouter, API, SDK.</p>
          </ReviewNote>
        </div>
      </section>

      {/* 10 Community */}
      <section id="community">
        <div className="wrap">
          <div className="community">
            <SectionHead tight eyebrow="Build with us" title="Helpin is being built in the open."
              lede="We're building the workspace we think modern SaaS teams need: customer context connected directly to the work that follows." />
            <ul className="steps big"><li>Report an issue.</li><li>Build an integration.</li><li>Create an agent.</li><li>Improve a workflow.</li><li>Shape what comes next.</li></ul>
            <div className="cta-row left"><a className="btn btn-primary" href={GITHUB_URL} target="_blank" rel="noopener noreferrer"><GithubIcon />View on GitHub →</a><a className="btn btn-secondary" href={`${GITHUB_URL}/blob/develop/CONTRIBUTING.md`} target="_blank" rel="noopener noreferrer">Contribute →</a></div>
          </div>
        </div>
      </section>

      {/* 11 Pricing */}
      <section id="pricing">
        <div className="wrap">
          <SectionHead eyebrow="Start your way" title="Cloud or self-hosted." lede="Run Helpin yourself or let us handle the infrastructure." />
          <div className="plans">
            <div className="plan"><h3>Open Source</h3><div className="price">Free</div><p className="muted">Run Helpin on your own infrastructure.</p>
              <ul><li>Customer records</li><li>Inbox</li><li>Meetings</li><li>Projects &amp; tasks</li><li>Docs</li><li>Agents</li><li>API</li><li>Developer integrations</li></ul>
              <a className="btn btn-secondary" href={`${GITHUB_URL}/blob/develop/community/README.md`}>Self-host Helpin →</a></div>
            <div className="plan featured"><h3>Helpin Cloud</h3><div className="price">Managed for you</div><p className="muted">Get Helpin without managing the infrastructure.</p>
              <ul><li>Managed hosting</li><li>Automatic updates</li><li>Backups</li><li>Team collaboration</li><li>AI controls</li></ul>
              <Link className="btn btn-primary" href={SIGNUP_URL}>Start free →</Link></div>
            <div className="plan"><h3>Enterprise</h3><div className="price">For larger teams</div><p className="muted">Advanced deployment, administration, security, and support requirements.</p>
              <ul><li>SSO / SAML</li><li>Advanced permissions</li><li>Private deployment</li><li>Audit controls</li><li>Custom retention</li><li>Priority support</li></ul>
              <a className="btn btn-secondary" href="mailto:hello@helpin.ai">Talk to us →</a></div>
          </div>
          <ReviewNote tag="11">
            <p><Flag>DECISION</Flag> The Open Source column lists meetings, projects, tasks, and agents; Community 0.1 ships support, docs, and agents. <Flag>NOT FOUND</Flag> SSO/SAML, audit controls, custom retention, and a private-deployment offering do not exist yet. <Flag>DROPPED</Flag> The $99 and $299 plan prices from the live pricing page are not on this page.</p>
          </ReviewNote>
        </div>
      </section>

      {/* 12 FAQ */}
      <section id="faq">
        <div className="wrap">
          <SectionHead eyebrow="Questions" title="FAQ" />
          <div className="faq">
            {FAQS.map(([q, ps], i) => (
              <details key={q} open={i === 0}><summary>{q}</summary>{ps.map((t) => <p key={t}>{t}</p>)}</details>
            ))}
          </div>
        </div>
      </section>

      {/* 13 Final CTA */}
      <section>
        <div className="wrap">
          <div className="final">
            <span className="eyebrow">Close the loop</span>
            <h2>Start with a conversation.<br />End with something shipped.</h2>
            <p className="lede">Give your team — and your AI agents — the context to understand customers and act on what matters.</p>
            <CtaRow />
            <div className="assure"><span>Open source</span><span>Self-hostable</span><span>Built for SaaS teams</span></div>
          </div>
        </div>
      </section>

      <PreviewFooter />
    </>
  );
}
