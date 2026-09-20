import { Check, FileText, GitPullRequest, Link2, MessageSquare } from 'lucide-react';

export function ProjectDelivery() {
  return <div className="project-delivery" aria-label="Illustrative customer follow-up after the Slack alerts release">
    <div className="pd-top"><span className="ps-key">ORB-491 / RELEASE FOLLOW-UP</span><span className="pd-complete"><Check size={12} aria-hidden="true" />Shipped</span></div>
    <h3>Slack alerts are ready for Northstar.</h3>
    <div className="pd-evidence"><span><GitPullRequest size={15} aria-hidden="true" />Reviewed change</span><span><FileText size={15} aria-hidden="true" />Setup guide</span><span><Link2 size={15} aria-hidden="true" />Maya’s conversation</span></div>
    <div className="pd-reply"><div><MessageSquare size={15} aria-hidden="true" /><strong>Customer update</strong><span>Draft · Not sent</span></div><p>Hi Maya, Slack alerts for failed syncs are now available. Connect your team’s channel in Integrations to get the affected account and a link to investigate. Here’s the setup guide to get started.</p></div>
    <div className="pd-owner"><img src="/new/avatars/sam.webp" width={26} height={26} alt="" /><span>Sam reviews the update before sending.</span></div>
  </div>;
}
