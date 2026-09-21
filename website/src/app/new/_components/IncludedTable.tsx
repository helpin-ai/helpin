import { SectionHead, GITHUB_URL } from './ui';

const ROWS = [
  ['Support inbox — chat and email', 'Included', 'Included'],
  ['Knowledge and help center', 'Included', 'Included'],
  ['Ask Agent and specialist agents', 'Included', 'Included'],
  ['Projects, sprints, roadmap and objectives', 'Included', 'Included'],
  ['CRM, deals, signals and playbooks', 'Included', 'Included'],
  ['Meetings', 'Included', 'Included'],
  ['Coding agents', 'Included', 'Included'],
  ['Automation and triggers', 'Included', 'Included'],
  ['Hosting', 'Your infrastructure', 'Managed by Helpin'],
  ['Enterprise features', 'Separate Enterprise license', 'See plan details'],
] as const;

export function IncludedTable() {
  return <section id="whats-included" className="included-section"><div className="wrap">
    <SectionHead eyebrow="Choose how you run Helpin" title="One open-source product. Your choice of hosting." lede="Support, projects, CRM, meetings, knowledge and agents are open source. Run Helpin yourself or let us host it for you. Enterprise features are licensed separately." />
    <div className="included-table-scroll" role="region" aria-label="Self-hosted and Cloud comparison" tabIndex={0}><table className="included-table"><caption>Product modules and hosting options</caption><thead><tr><th scope="col">Capability</th><th scope="col">Open source<span>Self-hosted · AGPL-3.0</span></th><th scope="col">Helpin Cloud<span>Hosted by Helpin</span></th></tr></thead><tbody>{ROWS.map(([feature,selfHosted,cloud])=><tr key={feature}><th scope="row">{feature}</th><td>{selfHosted}</td><td>{cloud}</td></tr>)}</tbody></table></div>
    <p className="included-note">Enterprise code includes subscription billing, managed AI routes and commercial policies under a separate license. <a href={`${GITHUB_URL}/blob/develop/ee/LICENSE`}>Enterprise license →</a> <a href="/pricing">Cloud plans and limits →</a></p>
    <p className="included-note">Configure the services your workflows need, including email delivery, meeting capture, model providers and code access. Provider usage and hosting may have separate costs. <a href={`${GITHUB_URL}/blob/develop/docs/community/configuration.md`}>Configuration guide →</a></p>
    <p className="included-note">Use Docker Compose for the documented self-hosting setup. <a href={`${GITHUB_URL}/blob/develop/docs/community/deployment.md`}>Deployment guide →</a></p>
  </div></section>;
}
