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
    "Not in Community 0.1",
    "Included"
  ],
  [
    "Automation and triggers",
    "Included",
    "Plan-dependent"
  ],
  [
    "Users",
    "Unlimited",
    "Per plan"
  ],
  [
    "AI models",
    "Your own provider keys",
    "Included AI; your own keys on Enterprise"
  ],
  [
    "Hosting and support",
    "Your team; community support",
    "Managed by Helpin, with support"
  ]
] as const;

export function IncludedTable() {
  return <section id="whats-included" className="included-section"><div className="wrap">
    <SectionHead eyebrow="Choose how you run Helpin" title="One connected product. Two ways to run it." lede="Same product either way. Cloud adds hosting, included AI, and support." />
    <div className="included-table-scroll" role="region" aria-label="Self-hosted and Cloud comparison" tabIndex={0}><table className="included-table"><caption>Product modules and hosting options</caption><thead><tr><th scope="col">Capability</th><th scope="col">Open-source self-hosting<span>AGPL-3.0 · 0.1 beta</span></th><th scope="col">Helpin Cloud<span>Hosted by Helpin</span></th></tr></thead><tbody>{ROWS.map(([feature,selfHosted,cloud])=><tr key={feature}><th scope="row">{feature}</th><td>{selfHosted}</td><td>{cloud}</td></tr>)}</tbody></table></div>
    <p className="included-note">The open-source product has no license fee. Your team covers hosting, operation, and AI provider usage. Cloud capacity and AI usage depend on your plan.</p>
    <p className="included-note">Need a commercial license instead of AGPL, deployment help, or support terms? Enterprise covers them. <a href="/pricing">Compare plans →</a> <a href={`${GITHUB_URL}/blob/develop/docs/community/configuration.md`}>Read the configuration guide →</a></p>
  </div></section>;
}
