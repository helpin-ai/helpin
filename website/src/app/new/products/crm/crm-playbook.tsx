"use client";

import { useId, useState, type ReactNode, type KeyboardEvent } from "react";
import { ArrowRight, Check } from "lucide-react";
import { CRMAvatar, CRMNavigation } from "./crm-workspace";
import { CRMIcon } from "./crm-icons";
import "./crm-playbook.css";

// Prepared read-only example of PlaybookDetail, PlaybookEditor and PlaybookSignals.
// Keep navigation local: no workspace credentials, agent runs or generated data.
const TABS = ["Signals", "Setup", "Automation", "Activity"] as const;
const STEPS = [
  "Purpose & scope",
  "Milestones",
  "Team & permissions",
  "Monitoring",
] as const;
const MILESTONES = [
  [
    "Understand the customer requirement",
    "The customer’s concern is documented, with the original conversation and affected work attached.",
  ],
  [
    "Confirm the resolution",
    "Engineering confirms the fix and the customer can complete the workflow that was blocked.",
  ],
  [
    "Agree on the renewal next step",
    "The customer has reviewed the update and agreed on a renewal decision date with the account owner.",
  ],
];
const SIGNALS = [
  [
    "Northstar Labs",
    "Complete exports needed before renewal",
    "Sam Rivera",
    "1 / 3 achieved",
    "Confirm EXP-142 with engineering",
    "Waiting",
  ],
  [
    "Harbor Metrics",
    "Renewal terms under review",
    "Aisha Patel",
    "2 / 3 achieved",
    "Confirm the decision date",
    "Needs approval",
  ],
  [
    "Forma",
    "Seat requirements have changed",
    "Sam Rivera",
    "1 / 3 achieved",
    "Review the revised proposal",
    "Waiting",
  ],
  [
    "Cedar Cloud",
    "Additional workspaces requested",
    "Aisha Patel",
    "1 / 3 achieved",
    "Confirm the rollout scope",
    "Waiting",
  ],
];
function Field({
  label,
  children,
  multiline = false,
}: {
  label: string;
  children: ReactNode;
  multiline?: boolean;
}) {
  return (
    <div className="cp-field">
      <dt>{label}</dt>
      <dd data-multiline={multiline}>{children}</dd>
    </div>
  );
}
export function CRMPlaybook() {
  const [tab, setTab] = useState<(typeof TABS)[number]>("Setup");
  const [step, setStep] = useState(1);
  const [search, setSearch] = useState("");
  const id = useId();
  function moveTab(event: KeyboardEvent<HTMLButtonElement>, index: number) {
    const next =
      event.key === "ArrowRight"
        ? (index + 1) % TABS.length
        : event.key === "ArrowLeft"
          ? (index + TABS.length - 1) % TABS.length
          : event.key === "Home"
            ? 0
            : event.key === "End"
              ? TABS.length - 1
              : -1;
    if (next < 0) return;
    event.preventDefault();
    setTab(TABS[next]);
    document.getElementById(`${id}-tab-${next}`)?.focus();
  }
  return (
    <div
      className="crm-workspace cp-workspace"
      aria-label="OrbitDesk renewal recovery playbook"
    >
      <div className="cp-frame">
        <CRMNavigation view="playbooks" />
        <div className="cp-main">
          <header className="cp-header">
            <span className="cp-breadcrumb">Playbooks</span>
            <div>
              <h3>Renewal recovery</h3>
            </div>
            <p>
              Published version 3{" "}
              <span className="cp-published">Accepting signals</span>
            </p>
          </header>
          <div className="cp-content">
            <div className="cp-tabs" role="tablist" aria-label="Playbook views">
              {TABS.map((name, index) => (
                <button
                  key={name}
                  id={`${id}-tab-${index}`}
                  type="button"
                  role="tab"
                  aria-selected={tab === name}
                  aria-controls={`${id}-panel`}
                  tabIndex={tab === name ? 0 : -1}
                  onClick={() => setTab(name)}
                  onKeyDown={(event) => moveTab(event, index)}
                >
                  {name}
                </button>
              ))}
            </div>
            <div
              id={`${id}-panel`}
              className="cp-panel"
              role="tabpanel"
              aria-labelledby={`${id}-tab-${TABS.indexOf(tab)}`}
              tabIndex={0}
            >
              {tab === "Setup" && (
                <div className="cp-setup">
                  <nav className="cp-steps" aria-label="Playbook setup steps">
                    <ol>
                      {STEPS.map((label, index) => (
                        <li key={label}>
                          <button
                            type="button"
                            aria-current={step === index ? "step" : undefined}
                            aria-controls={`${id}-step`}
                            onClick={() => setStep(index)}
                          >
                            <span aria-hidden="true">
                              {step === index ? (
                                index + 1
                              ) : (
                                <Check size={14} aria-hidden="true" />
                              )}
                            </span>
                            {label}
                          </button>
                        </li>
                      ))}
                    </ol>
                  </nav>
                  <div className="cp-editor">
                    <div
                      id={`${id}-step`}
                      role="region"
                      aria-label={STEPS[step]}
                    >
                      {step === 0 && (
                        <>
                          <dl className="cp-fields">
                            <Field label="Playbook name">
                              Renewal recovery
                            </Field>
                            <Field label="Desired outcome" multiline>
                              Resolve the concern blocking a customer’s renewal
                              and agree on a clear next step with the account
                              owner.
                            </Field>
                            <Field label="Description (optional)" multiline>
                              Bring customer evidence, the work needed to
                              resolve it, and the renewal follow-up into one
                              plan.
                            </Field>
                          </dl>
                          <div className="cp-section">
                            <h4>Matching signals</h4>
                            <span className="cp-motion">
                              <Check size={14} aria-hidden="true" />
                              Renewal
                            </span>
                            <p>
                              Open renewal signals with a customer concern that
                              needs follow-up.
                            </p>
                          </div>
                        </>
                      )}
                      {step === 1 && (
                        <div className="cp-milestones">
                          {MILESTONES.map(([name, criteria], index) => (
                            <dl key={name}>
                              <Field label={`Milestone ${index + 1}`}>
                                {name}
                              </Field>
                              <Field label="Success criteria" multiline>
                                {criteria}
                              </Field>
                            </dl>
                          ))}
                        </div>
                      )}
                      {step === 2 && (
                        <>
                          <div className="cp-section">
                            <h4>Responsibilities</h4>
                            <dl className="cp-properties">
                              <Field label="Owner role">Account owner</Field>
                              <Field label="Approvals go to">
                                Next action owner
                              </Field>
                              <Field label="Escalation owner">
                                <span className="cp-person">
                                  <CRMAvatar size={20} />
                                  Sam Rivera
                                </span>
                              </Field>
                            </dl>
                          </div>
                          <div className="cp-section">
                            <h4>Action permissions</h4>
                            <dl className="cp-properties">
                              {[
                                "Customer messages",
                                "CRM changes",
                                "Tasks",
                              ].map((label) => (
                                <Field key={label} label={label}>
                                  Approval required
                                </Field>
                              ))}
                            </dl>
                          </div>
                        </>
                      )}
                      {step === 3 && (
                        <>
                          <dl className="cp-properties">
                            <Field label="Check after (hours)">24</Field>
                            <Field label="Escalate after (hours)">72</Field>
                          </dl>
                          <div className="cp-section">
                            <h4>Stop when</h4>
                            <p>Customer outcome achieved</p>
                            <p>Signal closed</p>
                          </div>
                          <p className="cp-note">
                            Scheduled checks require automation to be on.
                          </p>
                        </>
                      )}
                    </div>
                    <div className="cp-step-actions">
                      <button
                        type="button"
                        disabled={step === 0}
                        onClick={() => setStep(step - 1)}
                      >
                        Back
                      </button>
                      {step < STEPS.length - 1 && (
                        <button type="button" onClick={() => setStep(step + 1)}>
                          Continue
                          <ArrowRight size={14} aria-hidden="true" />
                        </button>
                      )}
                    </div>
                  </div>
                </div>
              )}
              {tab === "Signals" && (
                <>
                  <div className="cp-search">
                    <CRMIcon name="Search01Icon" size={15} />
                    <input
                      aria-label="Search playbook signals"
                      value={search}
                      onChange={(e) => setSearch(e.target.value)}
                      placeholder="Search customers or signals…"
                    />
                  </div>
                  <div className="cp-table-scroll">
                    <table>
                      <caption>
                        Signals in the renewal recovery playbook
                      </caption>
                      <thead>
                        <tr>
                          {[
                            "Customer / signal",
                            "Owner",
                            "Milestones",
                            "Next step",
                            "Status",
                          ].map((label) => (
                            <th key={label}>{label}</th>
                          ))}
                        </tr>
                      </thead>
                      <tbody>
                        {SIGNALS.filter((row) =>
                          row
                            .join(" ")
                            .toLowerCase()
                            .includes(search.toLowerCase()),
                        ).map(
                          ([
                            company,
                            signal,
                            owner,
                            progress,
                            next,
                            status,
                          ]) => (
                            <tr key={company}>
                              <td>
                                <strong>{company}</strong>
                                <p>{signal}</p>
                              </td>
                              <td>{owner}</td>
                              <td>{progress}</td>
                              <td>{next}</td>
                              <td>
                                <span
                                  className="cp-signal-state"
                                  data-review={status === "Needs approval"}
                                >
                                  {status}
                                </span>
                              </td>
                            </tr>
                          ),
                        )}
                      </tbody>
                    </table>
                    {!SIGNALS.some((row) =>
                      row
                        .join(" ")
                        .toLowerCase()
                        .includes(search.toLowerCase()),
                    ) && <p className="cp-empty">No matching signals.</p>}
                  </div>
                </>
              )}
              {tab === "Automation" && (
                <div className="cp-automation">
                  <div className="cp-enrollment">
                    <span>New enrollment</span>
                    <span>Allowed</span>
                  </div>
                  <div className="cp-section">
                    <h4>
                      Automation <span>Off</span>
                    </h4>
                    <dl className="cp-properties">
                      <Field label="Agent">Beacon</Field>
                      <Field label="Flow">
                        Renewal recovery · Review updates
                      </Field>
                    </dl>
                    <dl className="cp-fields">
                      <Field label="Start automation for">
                        Signals the team starts
                      </Field>
                    </dl>
                    <div className="cp-section">
                      <h4>Usage limits</h4>
                      <dl className="cp-properties">
                        <Field label="Checks per signal / day">4</Field>
                        <Field label="Checks without progress">3</Field>
                      </dl>
                    </div>
                    <p className="cp-note">
                      Beacon reviews updates and proposes next actions for your
                      team to approve. This connection is prepared; automation
                      is off.
                    </p>
                  </div>
                </div>
              )}
              {tab === "Activity" && (
                <ol className="cp-activity">
                  {[
                    [
                      "Playbook published",
                      "Sam Rivera · Sep 18, 2026, 10:42",
                      "Version 3 includes the renewal milestones and approval settings.",
                    ],
                    [
                      "Draft updated",
                      "Sam Rivera · Sep 18, 2026, 10:36",
                      "Added success criteria and assigned the escalation owner.",
                    ],
                    [
                      "Draft created",
                      "Aisha Patel · Sep 17, 2026, 14:20",
                      "Defined the renewal recovery outcome and matching signals.",
                    ],
                  ].map(([title, date, body]) => (
                    <li key={title}>
                      <strong>{title}</strong>
                      <small>{date}</small>
                      <p>{body}</p>
                    </li>
                  ))}
                </ol>
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
