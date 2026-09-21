import { SectionHead, GITHUB_URL } from './ui';

const ROWS = [
  [
    "Chat and email support",
    "Included",
    "Included"
  ],
  [
    "Knowledge and help center",
    "Included",
    "Included"
  ],
  [
    "Ask Agent and specialists",
    "Included",
    "Included"
  ],
  [
    "Projects, sprints, roadmaps, and objectives",
    "Included",
    "Included"
  ],
  [
    "CRM, deals, signals, and playbooks",
    "Included",
    "Included; automation varies by plan"
  ],
  [
    "Meetings",
    "Included",
    "Included"
  ],
  [
    "Coding agents",
    "Included",
    "Included"
  ],
  [
    "Automation and triggers",
    "Included",
    "Plan-dependent"
  ],
  [
    "Hosting and operation",
    "Managed by your team",
    "Managed by Helpin"
  ],
  [
    "Enterprise capabilities",
    "Separately licensed",
    "Check plan and licensing details"
  ]
] as const;

export function IncludedTable() {
  return <section id="whats-included" className="included-section"><div className="wrap">
    <SectionHead eyebrow="Choose how you run Helpin" title="One connected product. Two ways to run it." lede="Operate Helpin on your infrastructure or choose managed hosting. Either way, bring the customer relationship and the work behind it into the same workspace." />
    <div className="included-table-scroll" role="region" aria-label="Self-hosted and Cloud comparison" tabIndex={0}><table className="included-table"><caption>Product modules and hosting options</caption><thead><tr><th scope="col">Capability</th><th scope="col">Open-source self-hosting<span>AGPL-3.0</span></th><th scope="col">Helpin Cloud<span>Hosted by Helpin</span></th></tr></thead><tbody>{ROWS.map(([feature,selfHosted,cloud])=><tr key={feature}><th scope="row">{feature}</th><td>{selfHosted}</td><td>{cloud}</td></tr>)}</tbody></table></div>
    <p className="included-note">Cloud capacity, AI usage, and advanced controls depend on your plan. Product inclusion does not mean every integration is configured or every provider service is included.</p>
    <p className="included-note">The open-source product has no software license fee. Your team covers hosting, operation, and external provider usage. Enterprise capabilities are licensed separately.</p>
    <p className="included-note"><a href="/pricing">Compare Cloud plans →</a> <a href={`${GITHUB_URL}/blob/develop/ee/LICENSE`}>Review Enterprise licensing →</a></p>
    <p className="included-note">Connect the services required for the workflows you use, then test those workflows before inviting your team. <a href={`${GITHUB_URL}/blob/develop/docs/community/configuration.md`}>Read the configuration guide →</a></p>
  </div></section>;
}
