import { Chip } from './ui';

// Four verbs, seven frames, one customer. Each frame is a placeholder for a screenshot of the seeded workspace.
export function LoopSection() {
  return (
    <div className="loop">
      <div className="verb">
        <div className="vh"><span className="k">01</span><h3>Hear</h3><p>Everything the customer says, wherever they say it.</p></div>
        <div className="sf">
          <div className="t">Meeting · Renewal call</div>
          <div className="q"><b>Next steps</b><br />1. Send Okta SAML mapping steps<br />2. Security sign-off by the 30th</div>
          <div className="row"><Chip tone="am">Timeline identified</Chip><span className="btno">Accept as task ▾</span></div>
          <div className="meta">Notetaker joined Google Meet · transcript attached</div>
        </div>
        <div className="sf">
          <div className="t">Support · website chat</div>
          <div className="q">“We're moving to Okta next month. Does SSO work with it?”</div>
          <div className="meta">Maya R. · matched to Acme Corp · knowledge: Set up SSO, partial match</div>
        </div>
        <p className="step-cap"><b>The meeting and the chat land on the same record.</b> The renewal signal and the support question are one story before anyone connects them by hand.</p>
      </div>

      <div className="verb">
        <div className="vh"><span className="k">02</span><h3>Decide</h3><p>What needs attention, and the next step.</p></div>
        <div className="sf">
          <div className="t">CRM · Signals</div>
          <div className="q"><b>Acme Corp</b> · Timeline identified · <b>High</b><br /><span className="meta">Next step: confirm Okta support before the renewal</span></div>
          <div className="row"><span className="btnm">Accept</span><span className="btno">Dismiss</span></div>
        </div>
        <div className="sf">
          <div className="t">Task · created from conversation</div>
          <div className="task"><b>HLP-142</b> Verify and document Okta SAML mapping<br /><span className="meta">Product · Sam K. · due Thu · linked: chat, meeting, deal</span></div>
        </div>
        <p className="step-cap"><b>A person accepts the next step.</b> The task is drafted from the conversation with the context attached, and it links back to the chat, the meeting, and the deal.</p>
      </div>

      <div className="verb">
        <div className="vh"><span className="k">03</span><h3>Ship</h3><p>From task to pull request, with a person approving.</p></div>
        <div className="sf">
          <div className="t">Task · Run agent</div>
          <div className="pipe"><span className="done">Task planner</span><span className="done">Code builder</span><span>Review agent</span></div>
          <div className="q"><span className="meta">Working branch</span><br /><span className="mono">HLP-142-okta-saml-mapping</span></div>
          <div className="row"><Chip tone="am">Review Code builder request</Chip><span className="btnm">Approve</span></div>
        </div>
        <div className="sf">
          <div className="t">Pull request</div>
          <div className="row"><b>#318 · Okta SAML attribute mapping</b><Chip tone="em">Open</Chip></div>
          <div className="meta">Opened by Code builder · review requested · merged by Sam</div>
        </div>
        <p className="step-cap"><b>Agents do the work you hand them.</b> The task planner scopes it, the code builder opens the PR, the review agent checks it. Your team approves the request and merges.</p>
      </div>

      <div className="verb">
        <div className="vh"><span className="k">04</span><h3>Tell</h3><p>The answer goes back out, and into the docs.</p></div>
        <div className="sf">
          <div className="t">Coverage gap · Okta SSO</div>
          <div className="doc"><b>Draft: Set up SSO with Okta</b><br /><span className="meta">from conversation evidence · review before publishing</span></div>
          <div className="row"><span className="btnm">Publish</span><span className="meta">Published Thu</span></div>
        </div>
        <div className="sf">
          <div className="t">Reply · with source</div>
          <div className="q">“Okta is verified and documented. Here are the exact steps.” <span className="meta">→ Set up SSO with Okta</span></div>
          <div className="meta">Deal: Proposal → Renewal signed · Fri</div>
        </div>
        <p className="step-cap"><b>The gap becomes an article, the article answers the next customer.</b> Maya gets a reply with its source. The renewal signal closes when the deal does.</p>
      </div>
    </div>
  );
}
