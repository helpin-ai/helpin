import { HeroVortex } from '../_components/HeroVortex';
import { createPageMetadata, PAGE_SEO } from '@/lib/metadata';
import { PreviewNav } from '../_components/PreviewNav';
import { PreviewFooter } from '../_components/PreviewFooter';
import { CtaRow, SectionHead } from '../_components/ui';
import { DOCS } from '../_components/docsLinks';

export const metadata = createPageMetadata(PAGE_SEO.product);

// Product detail moved off the homepage. Each area is a section with an anchor the homepage links to.
const AGENTS = [
  ['Echo agent', 'Answer customers and follow up using the guidance and tools you allow.'],
  ['Atlas agent', 'Shape an idea or request into a scope and plan.'],
  ['Scribe agent', 'Read the requirements and code before planning the task.'],
  ['Forge and Lens agents', 'Prepare the code and tests, then review the proposed change.'],
  ['Quill agent', 'Prepare the article update when the product or customer question changes.'],
  ['Beacon agent', 'Help sales choose who to contact and prepare a relevant follow-up.'],
];

const DEVELOPERS = [
  ['REST API', 'Build on Helpin’s customer, project, conversation, and workspace model.'],
  ['SDK', 'Send customer and product context directly from your application.'],
  ['Events & automation', 'Start agent work from supported workspace events, repository events, and schedules.'],
  ['AI tools', 'Your AI tools can use Helpin’s customer history and take permitted actions.'],
  ['GitHub & GitLab', 'Connect customer work directly to engineering.'],
  ['Run it yourself', 'Run Helpin where your team runs software.'],
];

function Arrow() { return <span className="arrow" aria-hidden="true">→</span>; }

export default function ProductPage() {
  return (
    <>
      <PreviewNav />

      <section className="hero page-hero motion-hero"><HeroVortex variant="orbit" tone="light" />
        <div className="wrap">
          <div className="hero-inner">
            <span className="eyebrow">Product</span>
            <h1>Your teams and AI agents. Working together.</h1>
            <p className="lede">Handle the question, plan the work, update the guide, and follow up. Helpin gives people and agents the same conversations, tasks, and records to work from.</p>

          </div>
        </div>
      </section>

      <section id="inbox">
        <div className="wrap">
          <SectionHead eyebrow="More than a support inbox" title="The Echo agent answers and follows up." lede="Answer chat and email, check earlier conversations, and hand over with the findings attached. Configure follow-ups so a quiet conversation gets another look." />
          <ul className="steps big"><li>Assign conversations.</li><li>Add notes and tags.</li><li>Use saved replies.</li><li>See customer history.</li><li>Create product work.</li><li>Ask an agent for help.</li></ul>
          <p className="section-close">When the conversation becomes something bigger, the history stays attached.</p>
          <p className="section-close"><a className="btn-link" href="/products/customer-support">Explore Support →</a></p>
        </div>
      </section>

      <section id="meetings">
        <div className="wrap">
          <SectionHead eyebrow="Every call adds to the history" title="Give meeting notes a useful next step." lede="Capture internal discussions and customer calls. AI prepares the notes, and Ask Agent helps your team use the decisions when planning work." />
          <ul className="steps big"><li>Record the conversation.</li><li>Get a speaker-attributed transcript.</li><li>Summarize decisions, objections, and next steps.</li><li>Create tasks and product work from action items.</li><li>Keep the meeting linked to the customer, deal, and project.</li></ul>
          <p className="section-close">Google Meet · Zoom · Microsoft Teams · Webex</p>
          <p className="section-close"><a className="btn-link" href="/products/meetings">Explore Meetings →</a></p>
        </div>
      </section>

      <section id="projects">
        <div className="wrap">
          <SectionHead eyebrow="From feedback to roadmap" title="AI agents build. Your team reviews."
            lede="Manage tasks, stories, epics, sprints, and objectives. Bring in Scribe to plan, Forge to code, and Lens to review." />
          <div className="pm-grid">
            <div className="pm-items">
              <div><h3>See the demand</h3><p>Know who is asking, how often it comes up, and which accounts it matters to.</p></div>
              <div><h3>Plan the work</h3><p>Organize projects, tasks, priorities, owners, and progress in the same workspace.</p></div>
              <div><h3>Connect engineering</h3><p>Link tasks to GitHub or GitLab so coding agents can pick them up with the customer request attached.</p></div>
              <div><h3>Know who is waiting</h3><p>When something ships, see every customer who asked for it and prepare their follow-up.</p></div>
            </div>
            <aside className="pm-panel" aria-label="Project SSO Enterprise Readiness with linked customer requests">
              <div className="pm-head"><span className="k">Project</span><b>SSO Enterprise Readiness</b><span className="muted">8 / 12 tasks · owner Sam Rivera · due next week</span></div>
              <div className="rs-bar light"><i className="on" style={{ width: '66.7%' }} /></div>
              <div className="pm-meta"><span><b>7</b> customer requests</span><span><b>3</b> linked engineering issues</span><span><b>3</b> renewals depend on it</span></div>
              <ul className="rs-tasks light">
                <li><span>Okta SAML mapping</span><em className="prog">In progress · Change #728</em></li>
                <li><span>SCIM provisioning</span><em>Planned</em></li>
                <li><span>Role mapping</span><em className="done">Shipped</em></li>
              </ul>
              <div className="pm-who"><span className="k">Asked for this</span><div className="avs"><img src="/new/avatars/maya.webp" alt="" /><img src="/new/avatars/dev.webp" alt="" /><img src="/new/avatars/lin.webp" alt="" /><img src="/new/avatars/aisha.webp" alt="" /><span>+3</span></div></div>
            </aside>
          </div>
          <div className="flow big"><span>Feedback</span><Arrow /><span>Project</span><Arrow /><span>Engineering</span><Arrow /><span>Shipped</span><Arrow /><span>Customer</span></div>
          <p className="section-close"><a className="btn-link" href="/products/projects">Explore Projects →</a></p>
        </div>
      </section>

      <section id="crm">
        <div className="wrap">
          <SectionHead eyebrow="CRM with the conversation attached" title="Give the Beacon agent a place on your sales team." lede="Spot buying interest, prepare for calls, and plan the next follow-up. Keep contacts, deals, and the conversations behind them in one place." />
          <ul className="steps big"><li>See the support issue holding up a deal.</li><li>See the feature request tied to a renewal.</li><li>See the meeting where the objection came up.</li><li>See what changed before you follow up.</li></ul>
          <p className="section-close">The stage says where the deal is. The history says why.</p>
          <p className="section-close"><a className="btn-link" href="/products/crm">Explore CRM →</a></p>
        </div>
      </section>

      <section id="knowledge">
        <div className="wrap">
          <SectionHead eyebrow="Knowledge that stays current" title="The Quill agent keeps your docs current." lede="Turn unanswered questions and product changes into proposed guide updates. Add clearer steps and fresh visuals, then review and publish." />
          <ul className="steps big"><li>See which articles customers used.</li><li>Find unanswered questions.</li><li>Create documentation from repeated conversations.</li><li>Update docs when the product changes.</li><li>Give agents the same knowledge your team uses.</li></ul>
          <p className="section-close"><a className="btn-link" href="/products/knowledge">Explore Knowledge →</a></p>
        </div>
      </section>

      <section id="agents">
        <div className="wrap">
          <SectionHead eyebrow="Agents with context" title="Choose a job. Give an agent the tools for it."
            lede="Start an agent yourself or from a supported event or schedule. Choose its tools and approval rules, then follow the work in Helpin." />
          <div className="six">
            {AGENTS.map(([k, v]) => <div key={k}><h3>{k}</h3><p>{v}</p></div>)}
          </div>
          <p className="section-close">Different jobs. One place to follow the work.</p>
          <p className="section-close"><a className="btn-link" href="/products/ai-agents">Explore AI agents →</a></p>
        </div>
      </section>

      <section id="developers">
        <div className="wrap">
          <SectionHead eyebrow="Built to be extended" title="Connect Helpin to the rest of your stack."
            lede="Add support to your app, connect your AI tools and use product events to start the next step." />
          <div className="six">
            {DEVELOPERS.map(([k, v]) => <div key={k}><h3>{k}</h3><p>{v}</p></div>)}
          </div>
          <div className="links"><a className="btn-link" href={DOCS.home}>Read the docs →</a></div>
        </div>
      </section>

      <section>
        <div className="wrap">
          <div className="final">
            <h2>Start with one useful agent workflow.</h2>
            <CtaRow />
          </div>
        </div>
      </section>

      <PreviewFooter />
    </>
  );
}
