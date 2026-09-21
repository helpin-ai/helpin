import { previewMetadata } from '../_components/preview-metadata';
import { PreviewNav } from '../_components/PreviewNav';
import { PreviewFooter } from '../_components/PreviewFooter';
import { CtaRow, SectionHead, GITHUB_URL } from '../_components/ui';

export const metadata = previewMetadata("Product overview \u2014 Helpin", "/new/product");

// Product detail moved off the homepage. Each area is a section with an anchor the homepage links to.
const AGENTS = [
  ['Support', 'Draft answers using customer history and product knowledge.'],
  ['Triage', 'Turn conversations into requests, bugs, tasks, and product work.'],
  ['Product', 'Find patterns across feedback and identify what customers keep asking for.'],
  ['Engineering', 'Carry the customer history behind an issue into development workflows.'],
  ['Docs', 'Update knowledge when the product changes.'],
  ['Customer', 'Find the people waiting for a fix or feature and prepare the follow-up.'],
];

const DEVELOPERS = [
  ['REST API', 'Build on Helpin’s customer, project, conversation, and workspace model.'],
  ['SDK', 'Send customer and product context directly from your application.'],
  ['Webhooks', 'React to conversations, projects, tasks, customers, and product events.'],
  ['AI tools', 'Let your AI tools use Helpin’s customer history and take permitted actions.'],
  ['GitHub & GitLab', 'Connect customer work directly to engineering.'],
  ['Run it yourself', 'Run Helpin where your team runs software.'],
];

function Arrow() { return <span className="arrow" aria-hidden="true">→</span>; }

export default function ProductPage() {
  return (
    <>
      <PreviewNav />

      <section className="hero page-hero">
        <div className="wrap">
          <div className="hero-inner">
            <span className="eyebrow">Product</span>
            <h1>Everything in Helpin, attached to the customer.</h1>
            <p className="lede">Support, meetings, projects, CRM, docs and agents share one customer history. Start with the area you need.</p>
            <nav className="jump" aria-label="Product areas">
              <a href="#inbox">Support</a><a href="#meetings">Meetings</a><a href="#projects">Projects</a><a href="#crm">CRM</a><a href="#knowledge">Knowledge</a><a href="#agents">Agents</a><a href="#developers">Developers</a>
            </nav>
          </div>
        </div>
      </section>

      <section id="inbox">
        <div className="wrap">
          <SectionHead eyebrow="More than a support inbox" title="Answer the customer with the history attached." lede="Handle chat and email from one shared inbox without disconnecting support from the rest of the company." />
          <ul className="steps big"><li>Assign conversations.</li><li>Add notes and tags.</li><li>Use saved replies.</li><li>See customer history.</li><li>Create product work.</li><li>Ask an agent for help.</li></ul>
          <p className="section-close">When the conversation becomes something bigger, the history stays attached.</p>
          <p className="section-close"><a className="btn-link" href="/new/products/customer-support">Explore Support →</a></p>
        </div>
      </section>

      <section id="meetings">
        <div className="wrap">
          <SectionHead eyebrow="Every call adds to the history" title="Turn customer meetings into work." lede="Helpin joins your calls, captures what was said, and connects the outcome to the customer." />
          <ul className="steps big"><li>Record the conversation.</li><li>Get a speaker-attributed transcript.</li><li>Summarize decisions, objections, and next steps.</li><li>Create tasks and product work from action items.</li><li>Keep the meeting connected to the customer, deal, and projects that matter.</li></ul>
          <p className="section-close">Google Meet · Zoom · Microsoft Teams · Webex</p>
        </div>
      </section>

      <section id="projects">
        <div className="wrap">
          <SectionHead eyebrow="From feedback to roadmap" title="Turn customer feedback into product work."
            lede="Create projects and tasks from conversations, meetings, and customer requests — with the original customer history attached." />
          <div className="pm-grid">
            <div className="pm-items">
              <div><h3>See the demand</h3><p>Know who is asking, how often it comes up, and which accounts it matters to.</p></div>
              <div><h3>Plan the work</h3><p>Organize projects, tasks, priorities, owners, and progress in the same workspace.</p></div>
              <div><h3>Connect engineering</h3><p>Link tasks to GitHub or GitLab and let coding agents pick them up, with the customer request attached.</p></div>
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
        </div>
      </section>

      <section id="crm">
        <div className="wrap">
          <SectionHead eyebrow="CRM with the conversation attached" title="Know what’s happening before the next sales call." lede="Manage companies, contacts, deals, stages, and renewals alongside the customer activity that explains them." />
          <ul className="steps big"><li>See the support issue holding up a deal.</li><li>See the feature request tied to a renewal.</li><li>See the meeting where the objection came up.</li><li>See what changed before you follow up.</li></ul>
          <p className="section-close">The stage says where the deal is. The history says why.</p>
          <p className="section-close"><a className="btn-link" href="/new/products/crm">Explore CRM →</a></p>
        </div>
      </section>

      <section id="knowledge">
        <div className="wrap">
          <SectionHead eyebrow="Knowledge that stays current" title="Turn what your team learns into answers." lede="Create public help-center content and internal docs alongside the conversations and product work that produce them." />
          <ul className="steps big"><li>See which articles customers used.</li><li>Find unanswered questions.</li><li>Create documentation from repeated conversations.</li><li>Update docs when the product changes.</li><li>Give agents the same knowledge your team uses.</li></ul>
        </div>
      </section>

      <section id="agents">
        <div className="wrap">
          <SectionHead eyebrow="Agents with context" title="Give agents the customer history your team already has."
            lede="Helpin agents work across customers, conversations, projects, docs, meetings, and engineering activity. They don’t start every task from scratch." />
          <div className="six">
            {AGENTS.map(([k, v]) => <div key={k}><h3>{k}</h3><p>{v}</p></div>)}
          </div>
          <p className="section-close">Different agents. Shared context.</p>
        </div>
      </section>

      <section id="developers">
        <div className="wrap">
          <SectionHead eyebrow="Built to be extended" title="Connect Helpin to the rest of your stack."
            lede="Add support to your app, connect your AI tools and use product events to start the next step." />
          <div className="six">
            {DEVELOPERS.map(([k, v]) => <div key={k}><h3>{k}</h3><p>{v}</p></div>)}
          </div>
          <div className="links"><a className="btn-link" href={`${GITHUB_URL}/blob/develop/docs/README.md`}>Read the docs →</a></div>
        </div>
      </section>

      <section>
        <div className="wrap">
          <div className="final">
            <h2>Start with support. Add the rest when you’re ready.</h2>
            <CtaRow />
          </div>
        </div>
      </section>

      <PreviewFooter />
    </>
  );
}
