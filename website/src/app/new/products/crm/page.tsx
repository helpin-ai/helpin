import { HeroVortex } from '../../_components/HeroVortex';
import { Availability, CtaNote, DEMO_URL, FAQList } from '../../_components/ui';
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
    title: "Give every relationship a home.",
    body: "Organize contacts and companies with owners, lifecycle stages, labels, and the records they belong to.",
  },
  {
    Icon: Mail,
    title: "Keep the conversation close.",
    body: "Connect Gmail and Google Calendar so email threads and meetings can sit alongside your customer records.",
  },
  {
    Icon: SlidersHorizontal,
    title: "Track what matters to your team.",
    body: "Add custom properties and configure pipeline stages to reflect how you sell and support customers.",
  },
  {
    Icon: ListFilter,
    title: "Find the accounts you need.",
    body: "Search and filter contacts, companies, and deals. Switch between a pipeline board and a detailed deal list.",
  },
  {
    Icon: FileInput,
    title: "Bring your existing relationships.",
    body: "Import contacts, companies, and deals from CSV. Map columns to fields before processing your import.",
  },
  {
    Icon: Sparkles,
    title: "Fill in the missing context.",
    body: "Use enrichment to research customer and company details, with source links you can check.",
  },
];
const FAQS = [
  [
    "What can I manage in Helpin CRM?",
    "Contacts, companies, deals, renewals, conversations and meetings, with support and project work attached.",
    "/new/products/crm#crm-record"
  ],
  [
    "Can I manage renewals and expansion as well as new business?",
    "Yes. Configure pipelines for new business, expansion or renewals, with owners, amounts and expected close dates.",
    "/new/products/crm#crm-pipeline"
  ],
  [
    "Where do customer signals come from?",
    "Helpin finds buying intent, objections and risks in conversations and connected customer activity. Check the evidence before choosing the next step.",
    "/new/products/crm#crm-signals"
  ],
  [
    "Will agents send messages or change deals without review?",
    "Playbooks keep proposed messages, record changes and tasks behind your approval rules. Your team separately enables the workflow.",
    "/new/products/crm#crm-follow-through"
  ],
  [
    "Can we bring our existing data and email?",
    "Import contacts, companies and deals from CSV. Connect Gmail and Google Calendar to add customer conversations and events.",
    "/new/developers"
  ],
  ["Can we self-host CRM?", "Yes. CRM is part of the open-source product. Run it on your own infrastructure or use Helpin Cloud.", "/new/self-hosting#whats-included"]
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
              <Availability category="CRM" />
              <h1 id="crm-title">
                A CRM that knows what
                <span> support and engineering are doing.</span>
              </h1>
              <p className="lede">
                Contacts, companies, deals and renewals — with the tickets, meetings and tasks behind each account attached. Ask Agent briefs you before every call.
              </p>
              <CtaRow secondaryHref={DEMO_URL} secondaryLabel="Book a demo" /><CtaNote trial />
            </div>
            <CRMWorkspace />
            <div className="crm-outcomes">
              {[
                ["01", "Know the account", "One history across teams."],
                ["02", "Find the next step", "Evidence behind every signal."],
                ["03", "Follow through", "An owner and work attached."],
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
                eyebrow="One record, a shared starting point"
                title="Get briefed before every call."
                lede="The last email. The rollout decision. The support issue waiting on a fix. Keep them connected to the customer—and ask Ask Agent to bring you up to speed before the next call."
              />
            </div>
            <CRMWorkspace mode="account" />
            <div className="crm-record-benefits">
              <article>
                <h3>Get up to speed.</h3>
                <p>
                  Read the customer summary and recent activity before the next
                  call. See the history that explains the current situation.
                </p>
              </article>
              <article>
                <h3>Keep the handoff intact.</h3>
                <p>
                  Link the people, companies, deals, and tasks involved. Give
                  sales, support, and product a shared starting point.
                </p>
              </article>
              <article>
                <h3>Follow the work.</h3>
                <p>
                  Find the customer request behind a task and the deal it
                  affects. Keep the relationship connected as the work
                  progresses.
                </p>
              </article>
            </div>
          </div>
        </section>

        <section id="crm-pipeline" className="crm-soft">
          <div className="wrap">
            <div className="crm-centered">
              <SectionHead
                eyebrow="A pipeline you can act on"
                title="See what’s holding up the deal."
                lede="Track stages, owners, and close dates. Then look beyond the amount: the customer’s concern, the work in progress, and the next step that could move the relationship forward."
              />
            </div>
            <CRMPipeline />
            <div className="crm-under-demo">
              <span>Configurable pipelines & stages</span>
              <span>Board & list views</span>
              <span>Owners, amounts & close dates</span>
            </div>
          </div>
        </section>

        <section id="crm-signals" className="crm-dark section-motion"><HeroVortex variant="converge" tone="dark" />
          <div className="wrap">
            <div className="crm-section-intro">
              <SectionHead
                eyebrow="Know where to focus"
                title="Find the customer signals worth acting on."
                lede="Buying intent in a support reply. A concern ahead of renewal. A request to bring in another team. Surface the signal, check the evidence, and agree the next step."
              />
              <span className="crm-small-label">
                CUSTOMER EVIDENCE → NEXT ACTION
              </span>
            </div>
            <CRMSignals />
            <p className="crm-dark-note">
              Your customer’s words stay attached to the recommendation.
            </p>
          </div>
        </section>


        <section id="crm-follow-through">
          <div className="wrap">
            <div className="crm-centered cp-intro">
              <SectionHead
                eyebrow="From a signal to a customer outcome"
                title="Give every follow‑up an owner and a plan."
                lede="Turn renewals, sales handoffs, and customer follow-ups into clear plans. Define the outcome, milestones, and owners, then let agents review progress and propose next steps for your team to approve."
              />
              <Link className="crm-text-link" href="/new/products/ai-agents">
                Meet the agents behind the work <ArrowRight size={16} />
              </Link>
            </div>
            <CRMPlaybook />
          </div>
        </section>

        <section className="crm-essentials crm-soft">
          <div className="wrap">
            <SectionHead
              eyebrow="Also included"
              title="The everyday CRM, covered."
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
              eyebrow="Questions"
              title="Get to know your CRM."
            />
            <div className="crm-faqs">
              <FAQList items={FAQS} className="faq-items" />
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
              <span className="eyebrow">Keep the relationship moving</span>
              <h2 id="crm-final-title">Every deal, with the story behind it.</h2>
              <p className="lede">
                Bring the context, the people, and the work together to help it
                happen.
              </p>
              <CtaRow secondaryHref={DEMO_URL} secondaryLabel="Book a demo" /><CtaNote trial />
            </div>
          </div>
        </section>
      </div>
      <PreviewFooter />
    </>
  );
}
