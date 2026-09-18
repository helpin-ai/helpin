import { Chip, Frame } from './ui';

// Placeholder for a screenshot of the real company page in the seeded demo workspace.
// Tabs and activity filters mirror frontend/src/pages/crm/CompanyDetail.tsx and components/crm/ActivityTimeline.tsx.
const TABS = ['Overview', 'Tasks', 'Emails', 'Meetings', 'Calls', 'Deals', 'Support', 'Notes'];
const FILTERS = ['All', 'Notes', 'Emails', 'Calls', 'Meetings', 'Tasks', 'Deals', 'Support'];
const TIMELINE = [
  { ic: 'MTG', title: 'Renewal call · Google Meet', sub: 'Summary, transcript, 3 next steps. Action item accepted as a task.', when: 'Mon 14:00' },
  { ic: 'SUP', title: 'Maya R.: "Does SSO work with Okta?"', sub: 'Website chat · answered by Sam · source: Set up SSO', when: 'Tue 10:42' },
  { ic: 'HLP', title: 'HLP-142 · Verify and document Okta SAML mapping', sub: 'In review · PR #318 open · created from the conversation', when: 'Tue 11:05' },
  { ic: 'DOC', title: 'Set up SSO with Okta', sub: 'Help center · drafted from a coverage gap · published', when: 'Thu 16:20' },
  { ic: 'DEAL', title: 'Growth renewal · $48k', sub: 'Proposal → Renewal signed · Sam K.', when: 'Fri 09:30' },
];

export function RecordFrame() {
  return (
    <Frame crumb="CRM · Companies · Acme Corp" label="Acme Corp customer record with one timeline across meetings, support, tasks, docs, and deals" className="hero-frame">
      <div className="record">
        <div>
          <div className="rec-head">
            <div className="av">AC</div>
            <div><b>Acme Corp</b><span>Growth plan · renewal in 22 days · owner Sam K.</span></div>
          </div>
          <div className="tabs">{TABS.map((t, i) => <span key={t} className={i === 0 ? 'on' : undefined}>{t}</span>)}</div>
          <div className="filters">{FILTERS.map((f, i) => <span key={f} className={i === 0 ? 'on' : undefined}>{f}</span>)}</div>
          <div className="tl">
            {TIMELINE.map((r) => (
              <div key={r.title}><div className="ic">{r.ic}</div><div><b>{r.title}</b><span>{r.sub}</span></div><div className="when">{r.when}</div></div>
            ))}
          </div>
        </div>
        <div className="rail">
          <div className="ui-label">Signals</div>
          <div className="sig">
            <div><b>Timeline identified</b><span>“Okta migration before the renewal” <span className="src">Open meeting</span></span></div>
            <div><b>Relationship risk</b><span>Security sign-off pending <span className="src">View ticket</span></span></div>
          </div>
          <div className="ui-label">Next step</div>
          <div className="sig"><div><b>Follow up</b><span>Confirm Okta support before renewal · <span className="src">Accept</span> · Dismiss</span></div></div>
          <div className="ui-label">Linked tasks</div>
          <div className="sig"><div><Chip tone="em">HLP-142 · In review</Chip></div></div>
        </div>
      </div>
    </Frame>
  );
}
