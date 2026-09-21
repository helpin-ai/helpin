import { SectionHead, GITHUB_URL } from './ui';

const ROWS = [
  ['Support inbox — chat and email', 'Included; email requires Postmark', 'Included'],
  ['Knowledge and help center', 'Included', 'Included'],
  ['Ask Agent, support agent and docs agent', 'Included', 'Included'],
  ['Projects, sprints, roadmap and objectives', 'Not in the supported bundle', 'Included'],
  ['CRM, deals, signals and playbooks', 'Not in the supported bundle', 'Included'],
  ['Meeting capture', 'Not in the supported bundle', 'Included; capture service required'],
  ['Coding agents with code access', 'Not in the supported bundle', 'Included; code connection required'],
  ['Automation and triggers', 'Not in the supported bundle', 'Included; enable your workflows'],
  ['Hosting', 'Your servers, with Docker Compose', 'Helpin cloud'],
] as const;

export function IncludedTable() {
  return <section id="whats-included" className="included-section"><div className="wrap">
    <SectionHead eyebrow="Compare your options" title="What’s included." lede="Start with free, self-hosted support, docs and agents, or use Cloud for the connected product suite." />
    <div className="included-table-scroll" role="region" aria-label="Community and Cloud comparison" tabIndex={0}><table className="included-table"><caption>Supported Community bundle and Cloud plan</caption><thead><tr><th scope="col">Capability</th><th scope="col">Community<span>Self-hosted · AGPL-3.0 · Free</span></th><th scope="col">Cloud plan<span>Hosted by Helpin</span></th></tr></thead><tbody>{ROWS.map(([feature,community,cloud])=><tr key={feature}><th scope="row">{feature}</th><td>{community}</td><td>{cloud}</td></tr>)}</tbody></table></div>
    <p className="included-note">This comparison follows the supported Community 0.1 release scope. Provider usage, email delivery and your own hosting may have separate costs. <a href={`${GITHUB_URL}/blob/develop/ROADMAP.md`}>Community scope and limitations →</a> <a href="/pricing">Cloud pricing →</a></p>
    <p className="included-note">Docker Compose is the supported Community install. Kubernetes and fully air-gapped deployments are not yet validated. <a href={`${GITHUB_URL}/blob/develop/docs/community/deployment.md`}>Deployment guide →</a></p>
  </div></section>;
}
