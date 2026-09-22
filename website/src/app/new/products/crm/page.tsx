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

export const metadata = previewMetadata(
  "CRM \u2014 Helpin",
  "/new/products/crm",
  "Contacts, companies, and deals with the email, meetings, support, and project work behind them.",
);

const ESSENTIALS = [
  {
    Icon: Building2,
    title: "Organize your relationships.",
    body: "Contacts with owners, lifecycle stages, and labels, linked to their companies.",
  },
  {
    Icon: Mail,
    title: "Keep email and meetings close.",
    body: "Sync Gmail and Google Calendar, and send email from the record.",
  },
  {
    Icon: SlidersHorizontal,
    title: "Make it fit your process.",
    body: "Configure pipelines, stages, and win probability.",
  },
  {
    Icon: ListFilter,
    title: "Find the right accounts.",
    body: "Search contacts, companies, and deals. Filter contacts and deals by stage, owner, and more.",
  },
  {
    Icon: FileInput,
    title: "Bring your existing data.",
    body: "Import CSV files and map columns before processing.",
  },
  {
    Icon: Sparkles,
    title: "Fill in missing details.",
    body: "Ask Agent to research a contact or company. Every enriched field keeps its source link.",
  },
];
const FAQS = [
  [
    "What can I manage in Helpin CRM?",
    "Contacts, companies, and deals across new business, renewal, and expansion, connected to the conversations and work behind each account. Your team can see the commercial relationship alongside support and delivery.",
    "/new/products/crm#crm-record"
  ],
  [
    "Can I manage renewals and expansion as well as new business?",
    "Yes. Create a pipeline for each process, with its own stages and win probability. Each deal has an owner, a value, and a close date.",
    "/new/products/crm#crm-pipeline"
  ],
  [
    "Where do customer signals come from?",
    "Email, Google Calendar, recorded meetings, support conversations, linked project work, and product usage if you send events. Each signal quotes the message or record it came from.",
    "/new/products/crm#crm-signals"
  ],
  [
    "Will agents send messages or change deals without review?",
    "Agents follow the tool access and approval rules you set, so you choose which actions need review. Deal automation can create or advance deals from high-confidence signals, but only for signal rules an admin has promoted.",
    "/new/products/crm#crm-follow-through"
  ],
  [
    "Can we bring our existing data and email?",
    "Yes. Import contacts, companies, and deals from CSV and map the columns before processing. Connect Gmail and Google Calendar. Zendesk and Intercom import is coming soon."
  ],
  [
    "What is included on each plan?",
    "On Cloud, Starter includes 5,000 contacts. Growth adds unlimited contacts and deal automation. Both are included when you self-host, with no plan limits."
  ],
  ["Can we self-host CRM?", "Yes. CRM, including meetings, is part of the AGPL-3.0 product: free, with no plan limits. Community is in 0.1 beta. Your team runs the installation and covers hosting and provider costs.", "/new/self-hosting#whats-included"]
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
                Every deal,{" "}
                <span>with the whole customer history.</span>
              </h1>
              <p className="lede">
                Manage contacts, companies, and deals with the email, meetings, support conversations, and project work behind them. Ask Agent prepares you for calls and drafts follow-ups from that history.
              </p>
              <CtaRow primaryLabel="Start free trial" secondaryHref={DEMO_URL} secondaryLabel="Book a demo" /><p className="crm-supporting-note">14-day free trial · No card required<br />Open source · Self-host free, or let us run it</p>
            </div>
            <CRMWorkspace /><p className="crm-demo-caption">Contacts and companies, each with an owner and the full history.</p>
            <div className="crm-outcomes">
              {[
                ["01", "Know the account.", "See what your customer has asked, discussed, and agreed."],
                ["02", "Find the next step.", "Signals flag buying intent, risk, and expansion, and quote the message behind each."],
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
            <CRMWorkspace mode="account" /><p className="crm-demo-caption">Email, meeting notes, and the linked task, summarized on one record.</p>
            <div className="crm-record-benefits">
              <article>
                <h3>Catch up before the call.</h3>
                <p>
                  An AI summary on every record, regenerated on demand.
                </p>
              </article>
              <article>
                <h3>Give the next teammate a starting point.</h3>
                <p>
                  Tasks, emails, meetings, calls, deals, support, and notes in one timeline.
                </p>
              </article>
              <article>
                <h3>Keep the relationship in view.</h3>
                <p>
                  Linked project tasks show what engineering owes the customer.
                </p>
              </article>
            </div>
          </div>
        </section>

        <section id="crm-pipeline" className="crm-soft">
          <div className="wrap">
            <div className="crm-centered">
              <SectionHead
                eyebrow="Pipelines"
                title="See where each deal stands, and why."
                lede="Track each deal alongside the email, meetings, and support activity that explain it."
              />
            </div>
            <CRMPipeline /><p className="crm-demo-caption">Separate pipelines for new business, renewals, and expansion.</p>
            <div className="crm-under-demo">
              <span>Multiple pipelines, configurable stages</span>
              <span>Board and list views</span>
              <span>Owners, values, close dates, and win probability</span>
            </div>
          </div>
        </section>

        <section id="crm-signals" className="crm-dark section-motion"><HeroVortex variant="converge" tone="dark" />
          <div className="wrap">
            <div className="crm-section-intro">
              <SectionHead
                eyebrow="Signals"
                title="Spot the opportunity. Understand the concern."
                lede="Helpin reads email, meetings, support conversations, and linked project work for buying intent, risk, and expansion. Each signal quotes the message it came from."
              />
              <span className="crm-small-label">
                CUSTOMER EVIDENCE → NEXT ACTION
              </span>
            </div>
            <CRMSignals />
            <p className="crm-dark-note">
              Every signal links to its source. You decide what happens next.
            </p>
          </div>
        </section>


        <section id="crm-follow-through">
          <div className="wrap">
            <div className="crm-centered cp-intro">
              <SectionHead
                eyebrow="Playbooks"
                title="Give every follow-up milestones, owners, and approvals."
                lede="Turn a signal into a plan with milestones and success criteria. With automation on, an agent checks progress on a schedule and proposes next steps for your approval."
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
              title="Everyday CRM, covered."
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
              <span className="eyebrow">Start with the history.</span>
              <h2 id="crm-final-title">Your deals, next to<br />the work behind them.</h2>
              <p className="lede">
                Import your contacts, connect Gmail and Google Calendar, and give your team and Ask Agent the same customer history.
              </p>
              <CtaRow primaryLabel="Start free trial" secondaryHref={DEMO_URL} secondaryLabel="Book a demo" /><p className="crm-supporting-note">14-day free trial · No card required<br />Open source · Self-host free, or let us run it</p>
            </div>
          </div>
        </section>
      </div>
      <PreviewFooter homepage />
    </>
  );
}
