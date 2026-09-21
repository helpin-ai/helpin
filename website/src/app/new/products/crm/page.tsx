import { HeroVortex } from '../../_components/HeroVortex';
import { DEMO_URL, FAQList } from '../../_components/ui';
import { previewMetadata } from '../../_components/preview-metadata';
import Link from "next/link";
import {
  ArrowRight,
  Building2,
  ChevronRight,
  FileInput,
  ListFilter,
  Mail,
  SlidersHorizontal,
  Sparkles,
} from "lucide-react";
import { PreviewNav } from "../../_components/PreviewNav";
import { PreviewFooter } from "../../_components/PreviewFooter";
import { ConnectedWorkspace } from "../../_components/ConnectedWorkspace";
import { CtaRow, SectionHead } from "../../_components/ui";
import { CRMPipeline, CRMSignals } from "./crm-scenes";
import { CRMPlaybook } from "./crm-playbook";
import { CRMWorkspace } from "./crm-workspace";
import "./crm.css";

export const metadata = previewMetadata("CRM \u2014 Helpin", "/new/products/crm");

const ESSENTIALS = [
  {
    Icon: Building2,
    title: "Organize your relationships.",
    body: "Manage contacts and companies with owners, stages, and labels.",
  },
  {
    Icon: Mail,
    title: "Keep email and meetings close.",
    body: "Connect Gmail and Google Calendar to your customer records.",
  },
  {
    Icon: SlidersHorizontal,
    title: "Make it fit your process.",
    body: "Add custom properties and configure your pipeline stages.",
  },
  {
    Icon: ListFilter,
    title: "Find the right accounts.",
    body: "Search and filter contacts, companies, and deals.",
  },
  {
    Icon: FileInput,
    title: "Bring your existing data.",
    body: "Import CSV files and map columns before processing.",
  },
  {
    Icon: Sparkles,
    title: "Fill in missing details.",
    body: "Research contacts and companies with source-linked enrichment.",
  },
];
const FAQS = [
  [
    "What can I manage in Helpin CRM?",
    "Contacts, companies, deals, and renewals, connected to the conversations and work behind each account. Your team can see the commercial relationship alongside support and delivery.",
    "/new/products/crm#crm-record"
  ],
  [
    "Can I manage renewals and expansion as well as new business?",
    "Yes. Configure pipelines for each process, with owners, values, and expected close dates.",
    "/new/products/crm#crm-pipeline"
  ],
  [
    "Where do customer signals come from?",
    "Conversations and connected customer activity. Review the supporting evidence before acting.",
    "/new/products/crm#crm-signals"
  ],
  [
    "Will agents send messages or change deals without review?",
    "Agent actions follow your configured tool access and approval rules. Set which actions require review, and enable the workflows you intend to run.",
    "/new/products/crm#crm-follow-through"
  ],
  [
    "Can we bring our existing data and email?",
    "Yes. Import CSV records and connect Gmail and Google Calendar.",
    "/new/developers"
  ],
  ["Can we self-host CRM?", "Yes. CRM is included in the open-source product. Your team operates the installation and covers hosting and provider costs. Enterprise features are licensed separately.", "/new/self-hosting#whats-included"]
] as const;

export default function CRMPage() {
  return (
    <>
      <PreviewNav />
      <div className="crm-page">
        <section className="crm-hero motion-hero" aria-labelledby="crm-title"><HeroVortex variant="orbit" tone="dark" />
          <div className="wrap">
            <div className="crm-breadcrumb">
              <Link href="/new">Helpin</Link>
              <ChevronRight size={12} />
              <span>CRM</span>
            </div>
            <div className="crm-hero-copy">
              <span className="eyebrow">CRM for SaaS teams</span>
              <h1 id="crm-title">
                Move deals forward.{" "}
                <span>With AI agents that know your customers.</span>
              </h1>
              <p className="lede">
                Manage contacts, companies, and deals alongside the conversations and work behind them. AI agents help you prepare for calls, understand customer needs, and follow up with the history in view.
              </p>
              <CtaRow primaryLabel="Start free trial" secondaryHref={DEMO_URL} secondaryLabel="Book a demo" /><p className="crm-supporting-note">14-day cloud trial · No credit card required.</p>
            </div>
            <CRMWorkspace /><p className="crm-demo-caption">The people you work with. The relationships you’re building.</p>
            <div className="crm-outcomes">
              {[
                ["01", "Know the account.", "See what your customer has asked, discussed, and agreed."],
                ["02", "Find the next step.", "Understand what needs attention before deciding how to act."],
                ["03", "Follow through.", "Give the next step an owner and keep the work connected."],
              ].map(([n, title, body]) => (
                <div key={n}>
                  <span>{n}</span>
                  <div>
                    <strong>{title}</strong>
                    <p>{body}</p>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </section>

        <section id="crm-record">
          <div className="wrap">
            <div className="crm-centered">
              <SectionHead
                eyebrow="One account. A shared history."
                title="Walk into the next call already up to speed."
                lede="Bring the latest email, meeting decisions, support issues, and linked work into one view. Ask Agent to bring you up to speed before you speak—without asking every team for an update."
              />
            </div>
            <CRMWorkspace mode="account" /><p className="crm-demo-caption">Know what the customer needs before deciding what to say.</p>
            <div className="crm-record-benefits">
              <article>
                <h3>Catch up before the call.</h3>
                <p>
                  Start with the situation, not a search through old messages.
                </p>
              </article>
              <article>
                <h3>Give the next teammate a starting point.</h3>
                <p>
                  Make the handoff about what happens next—not explaining everything again.
                </p>
              </article>
              <article>
                <h3>Keep the relationship in view.</h3>
                <p>
                  Remember who is waiting and why the work matters.
                </p>
              </article>
            </div>
          </div>
        </section>

        <section id="crm-pipeline" className="crm-soft">
          <div className="wrap">
            <div className="crm-centered">
              <SectionHead
                eyebrow="More than a deal stage"
                title="See where the deal stands. Know what needs to happen next."
                lede="Track the opportunity alongside the customer activity that explains it. Understand what is holding up the decision before choosing your next move."
              />
            </div>
            <CRMPipeline /><p className="crm-demo-caption">The next move depends on the relationship—not just the pipeline stage.</p>
            <div className="crm-under-demo">
              <span>Configurable stages</span>
              <span>Board and list views</span>
              <span>Owners, values, and close dates</span>
            </div>
          </div>
        </section>

        <section id="crm-signals" className="crm-dark section-motion"><HeroVortex variant="converge" tone="dark" />
          <div className="wrap">
            <div className="crm-section-intro">
              <SectionHead
                eyebrow="Know when to reach out"
                title="Spot the opportunity. Understand the concern."
                lede="Find buying intent, renewal concerns, and expansion requests in customer activity. Check the original message before deciding what to do."
              />
              <span className="crm-small-label">
                CUSTOMER EVIDENCE → NEXT ACTION
              </span>
            </div>
            <CRMSignals />
            <p className="crm-dark-note">
              A reason to investigate—not a conclusion to accept without checking.
            </p>
          </div>
        </section>


        <section id="crm-follow-through">
          <div className="wrap">
            <div className="crm-centered cp-intro">
              <SectionHead
                eyebrow="From a signal to a plan"
                title="Give customer follow‑ups a clear path forward."
                lede="Define milestones, owners, and success criteria. Let agents review progress and propose next steps within your approval rules."
              />
              <Link className="crm-text-link" href="/new/products/ai-agents">
                Explore AI agents <ArrowRight size={16} />
              </Link>
            </div>
            <CRMPlaybook /><p className="crm-demo-caption">Resolve the concern. Confirm the outcome. Agree on what comes next.</p>
          </div>
        </section>

        <section className="crm-essentials crm-soft">
          <div className="wrap">
            <SectionHead
              eyebrow="The essentials, connected"
              title="A CRM your team can work from every day."
            />
            <div className="crm-feature-grid">
              {ESSENTIALS.map(({ Icon, title, body }) => (
                <article key={title}>
                  <Icon size={22} strokeWidth={1.6} />
                  <h3>{title}</h3>
                  <p>{body}</p>
                </article>
              ))}
            </div>
          </div>
        </section>

        <section id="crm-faq">
          <div className="wrap crm-faq-grid">
            <SectionHead
              eyebrow="Before you bring your relationships over"
              title="Get to know Helpin CRM."
            />
            <div className="crm-faqs">
              <FAQList items={FAQS} className="faq-items" /><Link className="crm-text-link" href="/new/self-hosting">Explore self-hosting<ArrowRight size={16} /></Link>
            </div>
          </div>
        </section>
        <section
          className="final-cta final-cta-connected"
          aria-labelledby="crm-final-title"
        >
          <div className="wrap">
            <ConnectedWorkspace />
            <div className="final">
              <span className="eyebrow">Know the relationship. Make the next move.</span>
              <h2 id="crm-final-title">Give every deal<br />the history it deserves.</h2>
              <p className="lede">
                Bring your customer records, conversations, and work together. Give your team and AI agents the context to prepare, follow up, and move the relationship forward.
              </p>
              <CtaRow primaryLabel="Start free trial" secondaryHref={DEMO_URL} secondaryLabel="Book a demo" /><p className="crm-supporting-note">14-day cloud trial · No credit card required.</p>
            </div>
          </div>
        </section>
      </div>
      <PreviewFooter homepage />
    </>
  );
}
