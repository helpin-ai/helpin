import Link from 'next/link';
import { PreviewNav } from './_components/PreviewNav';
import { PreviewFooter } from './_components/PreviewFooter';
import { LiveRecord } from './_components/LiveRecord';
import { LoopWire } from './_components/LoopWire';
import { LoopSection } from './_components/LoopSection';
import { ReviewBanner, ReviewNote, Flag } from './_components/ReviewNotes';
import { Chip, CtaRow, SectionHead, SIGNUP_URL, GITHUB_URL } from './_components/ui';

const RECORD_FACTS = [
  ['Conversations', 'Website chat and email, in a shared inbox with assignment, notes, tags, and saved replies.'],
  ['Meetings', 'A notetaker joins Google Meet, Zoom, Teams, or Webex. You get the recording, a speaker-attributed transcript, a summary, decisions, objections, and next steps.'],
  ['Deals', 'Pipelines and stages, with buyer signals detected from email, meetings, and support, each linked to its source.'],
  ['Tasks', 'Created from a conversation, a meeting action item, or a plan. Linked back to whatever started them.'],
  ['Docs', 'Help-center articles and internal documents, including the ones a customer was sent and the ones their questions produced.'],
  ['Emails and calls', 'Gmail and calendar sync put the thread and the appointment on the same timeline.'],
];

const AI_FACTS = [
  ['Handoff is built in', 'A customer can ask for a person at any time. When the team is busy or offline, conversations queue instead of stalling.'],
  ['Actions can ask first', 'Any agent can be set to request approval before it runs a command, changes files, or takes an action. The request appears where you are working.'],
  ['Watch it work', 'Runs stream live. You see the steps and the reasoning as they happen, and you can reply mid-run when the agent needs a decision.'],
  ['Sources on every answer', 'Each AI draft shows the article it used. If it used nothing, the question is logged as a gap.'],
  ['Your keys, your models', 'Anthropic, OpenAI, or OpenRouter. Change the model per agent. Community uses only the keys you connect.'],
];

const AGENTS = [
  ['Support agent', 'Answers from your docs, drafts notes, escalates.'],
  ['Documentation agent', 'Drafts and updates articles from gaps and changes.'],
  ['Epic planner', 'Turns an epic and your notes into scoped tasks.'],
  ['Coding task planner', 'Scopes a task before code is written.'],
  ['Code builder', 'Implements on a branch and opens the PR.'],
  ['Review agent', 'Reviews the PR and runs checks.'],
  ['CRM operator', 'Updates deals and contacts from what happened.'],
  ['Researcher', 'Investigates a question across the web and your workspace.'],
  ['Marketer', 'Drafts changelogs and announcements from shipped work.'],
  ['Ask and Command', 'Answer a question, or run an instruction on any object.'],
];

const OS_FACTS = [
  ['AGPL-3.0', 'The application is AGPL-3.0-only. The JavaScript SDK and widget are Apache-2.0, so embedding them in your product carries no copyleft obligation.'],
  ['One database you own', 'Conversations, deals, tasks, articles, transcripts, and AI credentials live in your Postgres and your object storage.'],
  ['Your own model keys', 'Community has no managed AI route and no AI billing. Connect Anthropic, OpenAI, or OpenRouter and pay your provider directly.'],
  ['Nothing phones home', 'No telemetry, no Helpin account required, and the support widget sends no analytics events to us.'],
  ['Docker Compose', 'One bundle with Postgres, Redis, NATS, Temporal, and object storage. Backup, restore, and public-deployment guides included.'],
];

const FAQS: [string, string][] = [
  ['Will the AI do things without my team seeing?', 'In support, only in "AI replies first" mode, and only with a confident match in your published articles. Elsewhere, any agent can be set to ask before it runs a command, changes files, or takes an action, and the request appears where you are working. We recommend starting with approvals on.'],
  ['How do meetings get in?', 'A notetaker joins your Google Meet, Zoom, Teams, or Webex call. Afterwards the meeting sits on the customer\'s record with the recording, a transcript, a summary, decisions, objections, and next steps. Action items become tasks when a person accepts them.'],
  ['Which AI models can I use?', 'Anthropic, OpenAI, or OpenRouter, which routes to Gemini and other models. Community uses only the keys you connect. Cloud includes a monthly allowance and lets you connect your own keys as well.'],
  ['What is in the open-source edition?', 'Website chat, the shared inbox, email support, the public help center, and AI agents on your own keys. Projects and CRM are being added to Community; the edition table above shows the current state.'],
  ['Can I move from Intercom, Linear, or HubSpot?', 'Today Helpin imports help-center content from Help Scout and Nextra, projects from Shortcut, and contacts from CSV. There is no Intercom, Linear, or HubSpot importer yet. Tell us what you need to move and we\'ll say plainly whether we can help.'],
  ['Does it replace GitHub or my editor?', 'No. Helpin connects to GitHub or GitLab, opens pull requests on your repositories, and shows PR status on the task. Claude Code, Codex, or any MCP client can work inside your Helpin workspace through the MCP server.'],
  ['What do I need to self-host?', 'Docker Engine with Compose v2, Bash, OpenSSL, and an amd64 or arm64 host with about 8 GB of RAM and 20 GB of disk for evaluation. The install guide covers DNS, HTTPS, backups, and upgrades. There is no Kubernetes chart yet.'],
  ['Is it ready for production?', 'Cloud is what we run our own products on. Community 0.1 is a beta with a published list of known limitations; read it before putting it in front of customers.'],
  ['What is the license, exactly?', 'The application is AGPL-3.0-only. The SDK and widget packages are Apache-2.0. Code in enterprise directories, which covers billing and managed AI, is under a separate Helpin Enterprise License and is not needed to run Community.'],
];

export default function NewHomePage() {
  return (
    <>
      <ReviewBanner />
      <PreviewNav />

      {/* Hero */}
      <section className="hero">
        <div className="wrap">
          <div className="hero-inner">
            <span className="eyebrow">Open source, for SaaS teams</span>
            <h1>Where SaaS teams answer customers, ship fixes, and close deals.</h1>
            <p className="lede">Helpin puts your support inbox, help center, project tracker, and CRM on one customer record. AI agents draft replies, plan work, and open pull requests inside it, and ask before they act.</p>
            <CtaRow />
            <div className="assure"><span>AGPL-3.0</span><span>Unlimited seats</span><span>Self-host or Cloud</span></div>
          </div>
          <LoopWire />
          <ReviewNote tag="Hero">
            <p><b>Audience-first headline.</b> "SaaS teams" is the subject; the three verbs map to support and docs, projects, and CRM. The frame is a placeholder for the real company page in the seeded workspace.</p>
            <p><b>Wire concept.</b> One line, four stages on the page grid, one customer. The line draws left to right, each stage hangs its artifact off it, and the work inside Decide and Ship ticks off item by item. The approval moment is explicit in Ship: the run waits, an Approve pill appears, then "Approved by Sam." About ten seconds, plays once, finished state under reduced motion, vertical on phones. Every intermediate state is readable text; nothing overlaps mid-transition.</p>
            <p><b>Order:</b> headline, description, and buttons first, then the wire plays below them. The animated customer record is the visual for section two and starts when scrolled into view.</p>
            <p><Flag>DECISION</Flag> "View on GitHub" assumes the repository and a release bundle are public. <Flag>CLOUD</Flag> Until the Community module decision, this frame shows Cloud.</p>
          </ReviewNote>
        </div>
      </section>

      {/* One record */}
      <section id="record">
        <div className="wrap">
          <div className="onerec">
            <div>
              <SectionHead tight eyebrow="What Helpin is" title="One record per customer. Support, product, and sales read the same one."
                lede="In a SaaS company of ten or forty people, the same customer talks to support on Tuesday, comes up in the roadmap meeting on Wednesday, and renews on Friday. Most tools keep three versions of that story. Helpin keeps one, and everything that happens attaches to it." />
              <p className="lede" style={{ fontSize: 15.5 }}>What lands on the record:</p>
            </div>
            <div className="facts tight">
              {RECORD_FACTS.map(([k, v]) => <div key={k}><b>{k}</b><p>{v}</p></div>)}
            </div>
          </div>
          <LiveRecord />
          <ReviewNote tag="Section 2">
            <p><Flag>CONFIRMED</Flag> Meeting intelligence fields, Recall.ai and Vexa providers, Meet, Zoom, Teams, Webex; Gmail and calendar sync; associations across conversation, task, deal, contact, company. <Flag>VERIFY</Flag> Whether "calls" has its own capture path; whether "Emails" covers support email as well as Gmail-synced sales email.</p>
          </ReviewNote>
        </div>
      </section>

      {/* The loop */}
      <section id="loop">
        <div className="wrap">
          <SectionHead eyebrow="How it works" title="One customer, start to finish." lede="This is the loop Helpin is built around. Here it is with one account and one week." />
          <LoopSection />
          <div className="outcome">
            <span className="k">OUTCOME</span>
            <p>One account, one week, five tools' worth of work, in one record. Nobody copied anything between systems, and the help center is one article better.</p>
          </div>
          <ReviewNote tag="Signature demo">
            <p><b>Four verbs, seven frames, one customer.</b> Also the script for the recorded demo. Capture every frame from the seeded workspace.</p>
            <p><Flag>CAREFUL</Flag> Meeting action items become tasks only after a person accepts them. The PR is merged by a person. Deal stage changes are human actions unless deal automation is on (Growth). <Flag>CLOUD</Flag> Decide and Ship need PM and CRM, which are not in Community 0.1.</p>
          </ReviewNote>
        </div>
      </section>

      {/* AI control */}
      <section id="ai">
        <div className="wrap">
          <div className="modes">
            <div>
              <SectionHead tight eyebrow="AI, on your terms" title="You decide when the AI acts."
                lede="In support, the agent has three settings. Most teams start at private notes and move up once the drafts have earned it. Everywhere else, actions can be set to ask first." />
              <div className="seg">
                <div><div className="t"><i />Off</div><p>People reply. AI only helps with search and rewriting drafts.</p></div>
                <div className="on"><div className="t"><i />Private notes</div><p>AI drafts a reply as an internal note on each new conversation. Your team edits and sends.</p></div>
                <div><div className="t"><i />AI replies first</div><p>AI answers when it finds a confident match. Everything else waits for a person.</p></div>
              </div>
              <div className="approval" aria-label="An agent run paused for approval">
                <div className="row"><b>Code builder is asking</b><Chip tone="am">Paused</Chip></div>
                <div>Wants to run <span className="mono">pnpm test</span> and push branch <span className="mono">HLP-142-okta-saml-mapping</span>.</div>
                <div className="row"><span className="btnm">Approve</span><span className="links">Reply to agent · Open run</span></div>
              </div>
            </div>
            <div className="facts">
              {AI_FACTS.map(([k, v]) => <div key={k}><b>{k}</b><p>{v}</p></div>)}
            </div>
          </div>
          <ReviewNote tag="Section 4">
            <p><Flag>CONFIRMED</Flag> Support modes off / internal note / AI-first; escalation and handoff states; approval, review-checkpoint, and user-input interactions with a pending-interaction card; live streaming of run segments and reasoning; providers Anthropic, OpenAI, OpenRouter. <Flag>VERIFY</Flag> Exact wording of the paused-run card.</p>
          </ReviewNote>
        </div>
      </section>

      {/* Agents and rules */}
      <section id="agents">
        <div className="wrap">
          <div className="two">
            <div>
              <SectionHead tight eyebrow="Agents and automations" title="Ten agents with a job each. Rules that run them."
                lede="Every agent runs on the same runtime and sees the same record. Pick one, give it a target, and choose whether it asks first." />
              <div className="agents">
                {AGENTS.map(([k, v]) => <div key={k}><b>{k}</b><span>{v}</span></div>)}
              </div>
              <p className="footnote">Build your own: choose tools, targets, schedule, and approval mode. <Chip>Growth</Chip></p>
            </div>
            <div>
              <h3 className="sub-h3">When this happens, do that.</h3>
              <p className="lede sub-lede">Triggers from tasks, pull requests, releases, published docs, approvals, or a schedule. Actions that run an agent, move a task, merge a branch, or run a command.</p>
              <div className="rules">
                <div className="rule"><div className="sw" /><div><b>When a PR is merged</b> → run Review agent, then move task to <b>Done</b><br /><span>Team: Product · stop on match</span></div></div>
                <div className="rule"><div className="sw" /><div><b>When a task enters "Ready"</b> → run Coding task planner<br /><span>Only if the description mentions a customer</span></div></div>
                <div className="rule"><div className="sw" /><div><b>Every Monday 08:00</b> → run Marketer on last week's releases<br /><span>Cron · requires approval before publishing</span></div></div>
              </div>
              <p className="footnote">Conditions can be written in plain language. Automations are included on Growth.</p>
            </div>
          </div>
          <ReviewNote tag="Section 5">
            <p><Flag>CONFIRMED</Flag> Ten presets plus researcher; custom agents gated by entitlement. Triggers, actions, ordering, stop-on-match, natural-language conditions, automation_flows entitlement. <Flag>VERIFY</Flag> One-line job descriptions against each preset's prompt and tools; replace the example rules with rules from the seeded workspace.</p>
          </ReviewNote>
        </div>
      </section>

      {/* Open source */}
      <section className="os" id="os">
        <div className="wrap">
          <div className="os-grid">
            <div>
              <SectionHead tight eyebrow="Open source" title="Run it yourself, or let us."
                lede="The Community edition is the same product on your own servers. It's open source because the record of what your customers said and what you did about it should be yours to keep." />
              <div className="facts">
                {OS_FACTS.map(([k, v]) => <div key={k}><b>{k}</b><p>{v}</p></div>)}
              </div>
              <div className="links">
                <a className="btn btn-primary" href={GITHUB_URL} target="_blank" rel="noopener noreferrer">View on GitHub</a>
                <a className="btn-link" href={`${GITHUB_URL}/blob/develop/community/README.md`} target="_blank" rel="noopener noreferrer">Read the install guide →</a>
              </div>
            </div>
            <div>
              <table>
                <thead><tr><th></th><th>Community, self-hosted</th><th>Cloud</th></tr></thead>
                <tbody>
                  <tr><td>Price</td><td>Free, AGPL-3.0</td><td>From $99/month, unlimited seats</td></tr>
                  <tr><td>Chat, inbox, help center, AI agents</td><td className="tick">Included</td><td className="tick">Included</td></tr>
                  <tr><td>Projects and CRM</td><td className="tick">Included <span className="pending">pending decision</span></td><td className="tick">Included</td></tr>
                  <tr><td>Meeting notetaker</td><td className="dash">To decide <span className="pending">pending decision</span></td><td className="tick">Included</td></tr>
                  <tr><td>AI model keys</td><td>Bring your own</td><td>Included allowance, or bring your own</td></tr>
                  <tr><td>Hosting, upgrades, backups</td><td>You</td><td>Us</td></tr>
                  <tr><td>Support</td><td>Community, best effort</td><td>Priority on Growth</td></tr>
                </tbody>
              </table>
              <p className="small-note">Community 0.1 ships support, docs, and agents and is a beta. Read the scope and known limitations before serving production traffic.</p>
            </div>
          </div>
          <ReviewNote tag="Section 6">
            <p><Flag>DECISION</Flag> Projects and CRM in Community; meeting notetaker in Community (it needs a Recall.ai or Vexa account). <Flag>CONFIRMED</Flag> License scopes; BYOK-only Community; no telemetry; Compose bundle contents; no Helm chart. <Flag>VERIFY</Flag> Which Growth entitlements are ungated on a self-hosted install.</p>
          </ReviewNote>
        </div>
      </section>

      {/* Developers */}
      <section id="dev">
        <div className="wrap">
          <SectionHead eyebrow="For developers" title="Built to be run, extended, and worked on by agents." />
          <div className="dev">
            <div>
              <h3>Install</h3>
              <pre>{'curl -fsSL https://helpin.ai/install.sh | bash\nhelpin install\n'}<span className="c">{'# or, from a downloaded bundle:\n./setup.sh install · edit .env · ./setup.sh start'}</span></pre>
              <p>A guided installer that checks Docker, verifies the bundle, generates secrets, and starts the services. Linux or macOS, Docker Engine, Compose v2, 8 GB RAM and 20 GB disk to evaluate.</p>
            </div>
            <div>
              <h3>MCP, both directions</h3>
              <p>Connect Claude Code, Codex, or any MCP client to your workspace over OAuth 2.1, with scoped tools and workspace permissions re-checked on every call. Give your own agents external MCP servers too, with Linear, Sentry, and Customer.io presets, each run on a short-lived credential.</p>
            </div>
            <div>
              <h3>Git, SDK, runtime</h3>
              <p>GitHub App and GitLab integration with branch templates and PR status on the task. Apache-2.0 JavaScript SDK with React, Vue, and Next.js packages. Agent runs on the native runtime, Codex, or OpenCode.</p>
            </div>
          </div>
          <div className="links">
            <a className="btn-link" href={`${GITHUB_URL}/blob/develop/docs/README.md`}>Documentation →</a>
            <a className="btn-link" href={`${GITHUB_URL}/blob/develop/ARCHITECTURE.md`}>Architecture →</a>
            <a className="btn-link" href={`${GITHUB_URL}/blob/develop/CONTRIBUTING.md`}>Contributing →</a>
            <a className="btn-link" href={`${GITHUB_URL}/blob/develop/SECURITY.md`}>Security policy →</a>
          </div>
          <ReviewNote tag="Section 7">
            <p><Flag>CONFIRMED</Flag> A CLI at community/cli with install, status, logs, and doctor; the curl bootstrap requires a published Community release with CLI assets. MCP server at /mcp with OAuth 2.1; external MCP connections in Settings; GitHub App and GitLab; runtime kinds native, codex, opencode. <Flag>BLOCKER</Flag> Source builds still need the private agent-runtime repository; no release bundle is confirmed.</p>
          </ReviewNote>
        </div>
      </section>

      {/* Proof */}
      <section id="proof">
        <div className="wrap">
          <div className="proof">
            <div>
              <SectionHead tight eyebrow="Who's behind it" title="Built by a team that runs support, sales, and product on it."
                lede="Helpin is made by the team behind ContentStudio, ContentPen, and Usermaven. We built it for our own customer queues and our own roadmap first, and we publish what works and what doesn't, in the open." />
              <div className="logos"><span>ContentStudio</span><span>ContentPen</span><span>Usermaven</span><span>Replug</span></div>
              <p className="disclose">Products from the same company. Not third-party customers.</p>
            </div>
            <div className="quote">
              <p>“[Real pilot quote goes here. One sentence about the before, one about the after, with a measurable change if the customer agrees to it.]”</p>
              <small>Name, role, company — with written approval</small>
            </div>
          </div>
          <ReviewNote tag="Section 8">
            <p><Flag>VERIFY</Flag> That the named products run support, sales, or product work in Helpin today. <Flag>NEEDS EVIDENCE</Flag> No testimonial or metric exists yet; keep the placeholder until a pilot yields one.</p>
          </ReviewNote>
        </div>
      </section>

      {/* Pricing */}
      <section id="pricing">
        <div className="wrap">
          <SectionHead eyebrow="Pricing" title="Free to run yourself. One price per workspace to let us." />
          <div className="plans">
            <div className="plan">
              <h3>Community</h3><div className="price">Free<small> · self-hosted</small></div>
              <ul><li>Chat, inbox, help center, AI agents</li><li>Projects and CRM <span className="pending">pending decision</span></li><li>Your own model keys</li><li>AGPL-3.0, no telemetry</li></ul>
              <a className="btn btn-secondary" href={`${GITHUB_URL}/blob/develop/community/README.md`}>Read the install guide</a>
            </div>
            <div className="plan">
              <h3>Starter</h3><div className="price">$99<small> / month</small></div>
              <ul><li>Unlimited seats</li><li>Every module included</li><li>Standard AI allowance</li><li>Help center on your domain, GitHub integration</li></ul>
              <Link className="btn btn-primary" href={`${SIGNUP_URL}?plan=starter`}>Start free trial</Link>
            </div>
            <div className="plan">
              <h3>Growth</h3><div className="price">$299<small> / month</small></div>
              <ul><li>Everything in Starter</li><li>3× AI allowance</li><li>Custom agents, automations, scheduling, AI routing</li><li>Remove branding, priority support</li></ul>
              <Link className="btn btn-primary" href={`${SIGNUP_URL}?plan=growth`}>Start free trial</Link>
            </div>
          </div>
          <p className="footnote">14-day Growth trial, no card required. Annual billing is 20% less. <Link href="/pricing" style={{ textDecoration: 'underline' }}>Full plan details</Link>.</p>
          <ReviewNote tag="Section 9">
            <p><Flag>CONFIRMED</Flag> Prices, unlimited seats, trial, annual discount; Growth entitlements. <Flag>DROP</Flag> "SLA management" and PM "custom fields" from the live matrix. <Flag>VERIFY</Flag> Meeting notetaker availability by plan; desktop and mobile apps are at beta stage and stay off the page.</p>
          </ReviewNote>
        </div>
      </section>

      {/* FAQ */}
      <section id="faq">
        <div className="wrap">
          <SectionHead eyebrow="Questions" title="Before you decide." />
          <div className="faq">
            {FAQS.map(([q, a], i) => (
              <details key={q} open={i === 0}><summary>{q}</summary><p>{a}</p></details>
            ))}
          </div>
          <ReviewNote tag="Section 10">
            <p><Flag>DECISION</Flag> The open-source answer says "being added"; change it to match whatever 0.2 ships. <Flag>VERIFY</Flag> "Cloud is what we run our own products on" needs the proof-section confirmation.</p>
          </ReviewNote>
        </div>
      </section>

      {/* Final CTA */}
      <section>
        <div className="wrap">
          <div className="final">
            <h2>Start with one inbox, one repo, and one calendar.</h2>
            <p className="lede">Connect website chat, link a repository, invite the notetaker to one call, and turn on private notes. See the record fill itself in the first week.</p>
            <CtaRow />
          </div>
        </div>
      </section>

      <PreviewFooter />
    </>
  );
}
