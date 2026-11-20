import { Check, Loader2 } from 'lucide-react';
import '../docs-paths/docs-paths.css';
import './import-scene.css';

// Follows Settings → Import → Intercom (frontend/src/components/settings/IntercomImportWizard.tsx). Figures are fictional.
export function ImportScene() {
  return <figure className="dp-root im-root" aria-label="Importing a help center and past conversations from Intercom into Helpin">
    <div className="dp-frame im-frame">
      <div className="dp-bar"><span className="dp-logo">O</span><span className="dp-bar-title">Import from Intercom</span></div>
      <div className="im-body">
        <div className="im-connected"><Check size={14} />Connected to <b>OrbitDesk</b></div>
        <dl className="im-fields">
          <div><dt>Data region</dt><dd>Europe</dd></div>
          <div><dt>Created on or after</dt><dd>1 Jan 2025</dd></div>
          <div><dt>Inbox</dt><dd>Main inbox</dd></div>
          <div><dt>Article status</dt><dd>Match Intercom</dd></div>
        </dl>
        <p className="im-heading">Importing from Intercom</p>
        <div className="im-job" data-state="done">
          <div className="im-job-head"><b>Help Center</b><span><Check size={13} />214 articles</span></div>
          <div className="im-bar"><i /></div>
        </div>
        <div className="im-job">
          <div className="im-job-head"><b>Conversations</b><span><Loader2 size={13} className="dp-spin" />8,412 of 12,480</span></div>
          <div className="im-bar"><i /></div>
          <small>Replies, internal notes, and tags come with each conversation.</small>
        </div>
      </div>
    </div>
  </figure>;
}
