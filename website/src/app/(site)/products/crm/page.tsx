import { WorkScene } from '../../_components/WorkScene';
import { HeroVortex } from '../../_components/HeroVortex';
import { DEMO_URL, FAQList } from '../../_components/ui';
import { createPageMetadata, PAGE_SEO } from '@/lib/metadata';
import Link from "next/link";
import {
  ArrowRight,
  Building2,
  CheckCircle2,
  MessagesSquare,
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
import { CRMPipeline } from "./crm-scenes";
import { CRMPlaybook } from "./crm-playbook";
import { CRMWorkspace } from "./crm-workspace";
import "./crm.css";

export const metadata = createPageMetadata(PAGE_SEO.crm);

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
    "/products/crm#crm-record"
  ],
  [
    "Can I manage renewals and expansion as well as new business?",
    "Yes. Create a pipeline for each process, with its own stages and win probability. Each deal has an owner, a value, and a close date.",
    "/products/crm#crm-pipeline"
  ],
  [
    "Where do customer signals come from?",
    "Email, Google Calendar, recorded meetings, support conversations, linked project work, and product usage if you send events. Each signal quotes the message or record it came from.",
    "/products/crm#crm-signals"
  ],
  [
    "Will agents send messages or change deals without review?",
    "Agents follow the tool access and approval rules you set, so you choose which actions need review. Deal automation can create or advance deals from high-confidence signals, but only for signal rules an admin has promoted.",
    "/products/crm#crm-follow-through"
  ],
  [
    "Can we bring our existing data and email?",
    "Yes. Import contacts, companies, and deals from CSV and map the columns before processing. Connect Gmail and Google Calendar. Zendesk and Intercom import is coming soon."
  ],
  ["Can we self-host CRM?", "Yes. CRM, including meetings, is part of the AGPL-3.0 product: free, with no plan limits. Community is in 0.2 beta. Your team runs the installation and covers hosting and provider costs.", "/self-hosting#whats-included"]
] as const;

export default function CRMPage() {
  return (
    <>
      <PreviewNav tone="dark" />
      <div className="crm-page">
        <section className="crm-hero motion-hero" aria-labelledby="crm-title"><HeroVortex variant="orbit" tone="dark" />
          <div className="wrap">

            <div className="crm-hero-copy">
              <span className="eyebrow">CRM for SaaS teams</span>
              <h1 id="crm-title">
                Know who needs a follow-up.{" "}
                <span>Your agent prepares it.</span>
              </h1>
              <p className="lede">
                The Beacon agent helps your team spot buying interest, understand a stalled deal, and prepare the next message. Keep contacts, deals, email, and customer conversations together.
              </p>
              <CtaRow primaryLabel="Start free trial" secondaryHref={DEMO_URL} secondaryLabel="Book a demo" /><p className="crm-supporting-note">14-day free trial · No card required<br />Open source · Self-host free, or let us run it</p>
            </div>
            <CRMWorkspace deals /><p className="crm-demo-caption">Contacts, companies, and deals, with the customer history connected.</p>
            <div className="crm-outcomes">
              {([
                [MessagesSquare, "Know the account.", "See the customer’s conversations, meeting decisions, and open issues in one place."],
                [Sparkles, "Find the next step.", "AI spots buying interest, renewal risks, and expansion opportunities, with the source attached."],
                [CheckCircle2, "Follow through.", "Give every follow-up an owner. Agents help prepare the next action with the account context."],
              ] as const).map(([Icon, title, body]) => (
                <div key={title}>
                  <span className="crm-outcome-icon"><Icon size={21} strokeWidth={1.5} aria-hidden="true" /></span>
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
                title="Get the account update before the call."
                lede="Ask Agent what changed since your last conversation. See the open issue, latest email, and meeting decision before choosing what to say."
              />
            </div>
            <CRMWorkspace mode="account" /><p className="crm-demo-caption">Email, meeting notes, and the linked task, summarized on one record.</p>
            <div className="crm-record-benefits">
              <article>
                <h3>Ask for a quick briefing.</h3>
                <p>
                  An AI summary on every record, regenerated on demand.
                </p>
              </article>
              <article>
                <h3>Make the handoff easier.</h3>
                <p>
                  Tasks, emails, meetings, calls, deals, support, and notes in one timeline.
                </p>
              </article>
              <article>
                <h3>See the work the customer is waiting for.</h3>
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
                title="Keep the pipeline useful, not just up to date."
                lede="See the conversation behind a deal stage. Use agent assistance and configured deal rules to keep the record moving with the relationship."
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
          <div className="wrap crm-split">
            <div>
              <SectionHead
                eyebrow="Signals"
                title="Spot buying interest while the conversation is fresh."
                lede="A prospect asks about plans in chat. A customer mentions adding a team. Helpin finds these signals in conversations and meetings, with a source you can check before the Beacon agent helps you follow up."
              />
            </div>
            <WorkScene variant="sales" />
          </div>
        </section>


        <section id="crm-follow-through">
          <div className="wrap">
            <div className="crm-centered cp-intro">
              <SectionHead
                eyebrow="Playbooks"
                title="Know who to follow up with, and what to say."
                lede="Set a follow-up plan and owner. With automation enabled, an agent checks progress on a schedule and prepares the next action. Your approval rules decide what it can send or change."
              />
              <Link prefetch={false} className="crm-text-link" href="/products/ai-agents">
                Explore AI agents <ArrowRight size={16} />
              </Link>
            </div>
            <CRMPlaybook />
          </div>
        </section>

        <section className="crm-essentials crm-soft">
          <div className="wrap">
            <SectionHead
              eyebrow="The essentials, connected"
              title="Your everyday CRM. With agents to help."
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
              eyebrow="Questions, answered"
              title="FAQs about Helpin’s CRM"
            />
            <div className="crm-faqs">
              <FAQList items={FAQS} className="faq-items" /><Link prefetch={false} className="crm-text-link" href="/self-hosting">Explore self-hosting<ArrowRight size={16} /></Link>
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
              <h2 id="crm-final-title">Give your sales team<br />a useful next step.</h2>
              <p className="lede">
                Bring your contacts and connect Gmail and Google Calendar. The Beacon agent helps your team catch up, prioritize, and follow through.
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
