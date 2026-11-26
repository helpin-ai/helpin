import Link from 'next/link';
import { ArrowRight } from 'lucide-react';
import { HeroVortex } from '../../_components/HeroVortex';
import { Availability, DEMO_URL, FAQList, SectionHead, SIGNUP_URL } from '../../_components/ui';
import { PreviewNav } from '../../_components/PreviewNav';
import { PreviewFooter } from '../../_components/PreviewFooter';
import { ConnectedWorkspace } from '../../_components/ConnectedWorkspace';
import { createPageMetadata, PAGE_SEO } from '@/lib/metadata';
import { CompanyAccessScene, PortalHeroScene, PortalInboxScene, ReplyByScene, RequestTypesScene, SelfServiceReport, SelfServiceScene } from './portal-scenes';
import '../customer-support/support.css';
import './portal.css';

export const metadata = createPageMetadata(PAGE_SEO.customerPortal);

const FAQS = [
  [
    "Do customers need a password?",
    "No. Customers sign in with a link sent to their email address. You choose who can sign in: contacts you approve, members of companies with portal access, or anyone who confirms their email address."
  ],
  [
    "Can people send a request without signing in?",
    "Yes, if you allow it. Turn on new requests in the portal settings, then allow requests without sign-in. The customer gets an email about next steps, and your team replies by email."
  ],
  [
    "Who can see a company’s requests?",
    "Company admins and members see every request from their company, and admins can invite colleagues. Requesters see only their own requests. Anyone copied on a request finds it under Copied to me."
  ],
  [
    "Does AI reply to portal requests?",
    "No. Ask AI answers questions from your help articles before a request is sent, with links to the sources. Your team answers requests, and a support agent can draft private suggestions for the teammate that customers never see."
  ],
  [
    "Do portal requests follow our SLA policies?",
    "Yes. Policies can match the portal channel and request types. While a response target is running, the customer sees when to expect a reply. Missed targets and policy names stay internal."
  ],
  [
    "Can the portal use our domain and branding?",
    "Yes. The portal runs at /requests on your help center’s domain, with your help center’s branding."
  ],
  [
    "Can customers rate the support they received?",
    "Yes. When a request is resolved, customers rate it on a five-point scale and can add a comment. Ratings go to the team that handled the request and appear in Reports."
  ],
  [
    "Is the portal included in every plan?",
    "Yes. The customer portal is included in every Helpin Cloud plan and in the open-source Community edition."
  ]
] as const;

function Actions() {
  return <div className="cta-row"><a className="btn btn-primary" href={SIGNUP_URL}>Start free trial</a><a className="btn btn-secondary" href={DEMO_URL}>Book a demo</a></div>;
}

export default function CustomerPortalPage() {
  return <>
    <PreviewNav tone="dark" />
    <div className="support-page portal-page">
      <section className="hero support-hero portal-hero motion-hero"><HeroVortex variant="flow" tone="dark" />
        <div className="wrap">
          <div className="hero-inner">
            <Availability category="Customer portal · Included in every plan" />
            <h1>Give every customer a place to follow their requests</h1>
            <p className="lede">Customers sign in with an emailed link to open requests, follow every reply, and search their history. Your help articles answer first, and every request lands in your team’s shared inbox.</p>
            <Actions />
            <p className="cta-note">14-day free trial · No card required · Included in the open-source Community edition</p>
          </div>
          <PortalHeroScene />
        </div>
      </section>

      <section id="portal-self-service" className="support-desk-section">
        <div className="wrap support-desk-grid">
          <div className="support-desk-copy">
            <SectionHead eyebrow="Self-service" title="Answer before they ask" tight
              lede="Ask AI answers questions from your help articles, with links to the sources. While a customer writes a request, the portal suggests up to five published articles from the help spaces you choose." />
            <p>The Portal self-service report shows what happened next: how often a suggested article likely answered the question, and how often customers asked anyway. Requests where no article matched go to Coverage gaps, so the next article gets written.</p>
          </div>
          <div><SelfServiceScene /><SelfServiceReport /></div>
        </div>
      </section>

      <section id="portal-requests" className="support-desk-section support-desk-section-tint">
        <div className="wrap support-desk-grid support-desk-grid-flip">
          <div className="support-desk-copy">
            <SectionHead eyebrow="Request types" title="Ask the right questions up front" tight
              lede="Request types let customers say what their request is about and fill in the fields that type needs. The same request types can set SLA targets, so each kind of request gets its own response times." />
            <p>Customers attach screenshots and files, and copy colleagues: anyone, or only people from their own company. You can also accept requests from people who haven’t signed in.</p>
          </div>
          <RequestTypesScene />
        </div>
      </section>

      <section id="portal-reply-by" className="support-desk-section">
        <div className="wrap support-desk-grid">
          <div className="support-desk-copy">
            <SectionHead eyebrow="Reply times" title="Tell customers when to expect a reply" tight
              lede="While an SLA response target is running, the request shows when to expect a reply. If a target is missed, the portal shows no late notice, and policy names stay internal." />
            <p>Replies appear on the request as soon as your team sends them, marked as new. The status shows whose turn it is: Active, Waiting on you, or Resolved.</p>
          </div>
          <ReplyByScene />
        </div>
      </section>

      <section id="portal-companies" className="support-desk-section support-desk-section-tint">
        <div className="wrap support-desk-grid support-desk-grid-flip">
          <div className="support-desk-copy">
            <SectionHead eyebrow="Company access" title="Built for the companies you support" tight
              lede="Company admins see every request from their company and invite colleagues. Members see all company requests, and requesters see only their own." />
            <p>You decide who can sign in: contacts you approve, members of companies with portal access turned on, or anyone who confirms their email address. Blocked contacts can never sign in. Company access and company admins are set on the company record in the CRM.</p>
          </div>
          <CompanyAccessScene />
        </div>
      </section>

      <section id="portal-inbox" className="support-desk-section">
        <div className="wrap support-desk-grid">
          <div className="support-desk-copy">
            <SectionHead eyebrow="Your team’s side" title="Every request arrives in your shared inbox" tight
              lede="Portal requests are conversations in the shared inbox, with routing, tags, and SLA timers like every other channel. Your team replies from Helpin, and the answer appears in the portal." />
            <p>On unassigned requests, a support agent can draft a private suggestion for the teammate; customers never see it. When a request is resolved, the customer rates the support they received, and the rating appears in Reports.</p>
            <Link prefetch={false} className="btn-link" href="/products/customer-support">Explore customer support <ArrowRight size={16} aria-hidden="true" /></Link>
          </div>
          <PortalInboxScene />
        </div>
      </section>

      <section id="portal-domain" className="portal-domain-section">
        <div className="wrap">
          <SectionHead eyebrow="Your help center" title="On your help center, in your brand"
            lede="The portal runs at /requests on your help center’s domain, with your help center’s branding. It’s included in every plan and in the open-source Community edition, so you can also run it on your own servers." />
          <div className="home-section-actions"><Link prefetch={false} className="btn-link" href="/products/knowledge">Explore the help center <ArrowRight size={16} aria-hidden="true" /></Link></div>
        </div>
      </section>

      <section id="portal-faq" className="support-faq-section">
        <div className="wrap support-faq-grid"><SectionHead eyebrow="Questions, answered" title="FAQs about the customer portal" /><div className="support-faqs"><FAQList items={FAQS} className="faq-items" /></div></div>
      </section>

      <section className="final-cta final-cta-connected" aria-labelledby="portal-final-cta-title"><div className="wrap"><ConnectedWorkspace focus="support" /><div className="final"><span className="eyebrow">14-day free trial</span><h2 id="portal-final-cta-title">Give customers one place for every request</h2><p className="lede">Turn on the portal, choose who can sign in, and connect your help articles. Included in every plan, with no per-seat fees.</p><Actions /><p className="cta-note">No card required · Self-host free, or let us run it</p></div></div></section>
    </div>
    <PreviewFooter homepage />
  </>;
}
