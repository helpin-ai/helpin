"use client";

import { useState, useId, type CSSProperties } from "react";
import {
  ArrowRight,
  Building2,
  Check,
  Circle,
  CircleDot,
  Mail,
  Pause,
  Play,
  ShieldCheck,
  Sparkles,
} from "lucide-react";
import { useCRMPlayback } from "./use-crm-playback";
import { CRMAvatar, CRMMark } from "./crm-workspace";
import { CRMIcon } from "./crm-icons";

const PIPELINES = [
  {
    label: "New business",
    stages: ["Discovery", "Evaluation", "Decision"],
    deals: [
      {
        company: "Harbor Metrics",
        name: "Support workspace",
        amount: "$12,000",
        initials: "HM",
        date: "Oct 12",
        note: "Confirm the support team’s requirements.",
        evidence:
          "The discovery call identified shared inbox routing as the first priority.",
        source: "Discovery meeting",
        stage: 0,
      },
      {
        company: "Latticepoint",
        name: "Team workspace",
        amount: "$24,000",
        initials: "LP",
        date: "Oct 16",
        note: "Review the team’s evaluation checklist.",
        evidence:
          "Leo’s team is testing the workspace with engineering. The API requirements need a final review.",
        source: "Rollout review · Meeting",
        stage: 1,
      },
      {
        company: "Forma",
        name: "Customer workspace",
        amount: "$18,000",
        initials: "F",
        date: "Oct 23",
        note: "Confirm the procurement next step.",
        evidence:
          "The team has reviewed the proposal and asked for a final procurement discussion.",
        source: "Proposal follow-up · Email",
        stage: 2,
      },
    ],
  },
  {
    label: "Renewals",
    stages: ["Review", "Follow-up", "Decision"],
    deals: [
      {
        company: "Northstar Labs",
        name: "Annual renewal",
        amount: "$42,000",
        initials: "NL",
        date: "Nov 23",
        note: "Confirm EXP-142 with engineering, then prepare an update.",
        evidence:
          "Maya needs complete CSV exports before the wider rollout and renewal discussion. EXP-142 is in engineering review.",
        source: "Renewal discussion · Email",
        stage: 0,
      },
      {
        company: "Forma",
        name: "Workspace renewal",
        amount: "$18,000",
        initials: "F",
        date: "Nov 15",
        note: "Send the revised renewal proposal for review.",
        evidence:
          "The account owner recorded the team’s updated seat requirements after the review call.",
        source: "Account review · Meeting",
        stage: 1,
      },
      {
        company: "Harbor Metrics",
        name: "Support renewal",
        amount: "$12,000",
        initials: "HM",
        date: "Nov 21",
        note: "Confirm the customer’s decision date.",
        evidence:
          "The customer has received the renewal terms and is completing its internal review.",
        source: "Renewal terms · Email",
        stage: 2,
      },
    ],
  },
  {
    label: "Expansion",
    stages: ["Interest", "Scoping", "Decision"],
    deals: [
      {
        company: "Forma",
        name: "Second team rollout",
        amount: "$9,000",
        initials: "F",
        date: "Oct 28",
        note: "Clarify the second team’s workflow.",
        evidence:
          "A customer reply asked whether another department could use the same workspace.",
        source: "Team rollout · Support",
        stage: 0,
      },
      {
        company: "Northstar Labs",
        name: "Additional team seats",
        amount: "$12,000",
        initials: "NL",
        date: "Nov 06",
        note: "Confirm the team size once the export fix is verified.",
        evidence:
          "Maya asked about adding the wider team once complete CSV exports are available.",
        source: "Rollout review · Meeting",
        stage: 1,
      },
      {
        company: "Harbor Metrics",
        name: "Product team workspace",
        amount: "$6,000",
        initials: "HM",
        date: "Nov 12",
        note: "Review the proposed rollout schedule.",
        evidence:
          "The product lead has reviewed the workspace and requested a phased rollout.",
        source: "Expansion planning · Email",
        stage: 2,
      },
    ],
  },
];

// Matches Deals.tsx / DealBoard / DealCard: stage totals, ID, amount pill,
// probability, close date, and owner. Data and controls stay local to this preview.
export function CRMPipeline() {
  const [pipeline, setPipeline] = useState(1);
  const [selected, setSelected] = useState(0);
  const [view, setView] = useState<"board" | "list">("board");
  const [search, setSearch] = useState("");
  const id = useId();
  const current = PIPELINES[pipeline];
  const deal = current.deals[selected];
  const visible = current.deals
    .map((item, index) => ({ ...item, index }))
    .filter((item) =>
      (item.name + " " + item.company)
        .toLowerCase()
        .includes(search.toLowerCase()),
    );
  return (
    <div className="crm-pipeline-demo">
      <div className="crm-pipeline-top">
        <strong>Deals</strong>
        <span className="cw-add-record" aria-hidden="true">
          <CRMIcon name="PlusSignIcon" />
          Add deal
        </span>
      </div>
      <div className="crm-board-toolbar">
        <label className="crm-pipeline-select">
          Pipeline:
          <select
            aria-label="Choose a pipeline"
            value={pipeline}
            onChange={(e) => {
              setPipeline(Number(e.target.value));
              setSelected(0);
              setSearch("");
            }}
          >
            {PIPELINES.map((item, index) => (
              <option key={item.label} value={index}>
                {item.label}
              </option>
            ))}
          </select>
        </label>
        <label className="crm-deal-search">
          <CRMIcon name="Search01Icon" size={15} />
          <input
            aria-label="Search deals"
            placeholder="Search deals..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
        </label>
        <div role="group" aria-label="Deal view">
          {(["board", "list"] as const).map((value) => (
            <button
              type="button"
              key={value}
              aria-pressed={view === value}
              onClick={() => setView(value)}
            >
              <CRMIcon
                name={value === "board" ? "LayoutGridIcon" : "Menu01Icon"}
                size={14}
              />
              {value === "board" ? "Board" : "List"}
            </button>
          ))}
        </div>
      </div>
      {view === "board" ? (
        <div className="crm-board">
          {current.stages.map((stage, i) => {
            const item = visible.find((row) => row.index === i);
            return (
              <div
                className="crm-board-column"
                key={stage}
                style={
                  {
                    "--crm-stage-color": ["#94a3b8", "#f59e0b", "#8b5cf6"][i],
                  } as CSSProperties
                }
              >
                <div className="crm-stage-heading">
                  <div>
                    <strong>{stage}</strong>
                    <small>
                      {item ? "1 deal" : "0 deals"}
                      {item && <span>{item.amount}</span>}
                    </small>
                  </div>
                  <CRMIcon name="PlusSignIcon" size={16} />
                </div>
                {item && (
                  <button
                    type="button"
                    className="crm-deal-card"
                    aria-pressed={selected === i}
                    aria-controls={id}
                    onClick={() => setSelected(i)}
                  >
                    <span className="crm-deal-id">
                      <i />
                      DEAL-{104 + pipeline * 3 + i}
                    </span>
                    <strong>
                      {item.company} · {item.name}
                    </strong>
                    <span className="crm-card-metrics">
                      <span className="crm-deal-amount">
                        USD {item.amount.slice(1)}/yr
                      </span>
                      <span className="crm-deal-probability">
                        <i>
                          <b style={{ width: `${40 + i * 20}%` }} />
                        </i>
                        {40 + i * 20}%
                      </span>
                    </span>
                    <span className="crm-deal-bottom">
                      <span>
                        <CRMIcon name="Calendar03Icon" size={12} />
                        {item.date}
                      </span>
                      <CRMAvatar size={28} />
                    </span>
                  </button>
                )}
                <span className="crm-board-add" aria-hidden="true">
                  <CRMIcon name="PlusSignIcon" size={14} />
                  Add deal
                </span>
              </div>
            );
          })}
        </div>
      ) : (
        <div className="crm-deal-table-scroll">
          <table className="crm-deal-table">
            <thead>
              <tr>
                {[
                  "Name",
                  "Amount",
                  "Stage",
                  "Probability",
                  "Owner",
                  "Close date",
                ].map((label) => (
                  <th key={label}>{label}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {visible.map((item) => (
                <tr key={item.index} data-selected={selected === item.index}>
                  <td>
                    <button
                      type="button"
                      onClick={() => setSelected(item.index)}
                      aria-controls={id}
                    >
                      {item.company} · {item.name}
                    </button>
                  </td>
                  <td>USD {item.amount.slice(1)}</td>
                  <td>{current.stages[item.index]}</td>
                  <td>{40 + item.index * 20}%</td>
                  <td>
                    <CRMAvatar size={24} />
                  </td>
                  <td>{item.date}</td>
                </tr>
              ))}
            </tbody>
          </table>
          {!visible.length && <p>No deals match your search.</p>}
        </div>
      )}
      <div
        id={id}
        className="crm-deal-context"
        role="region"
        aria-label={`Context for ${deal.name}`}
      >
        <div>
          <span className="crm-mini-label">
            <CRMIcon name="File01Icon" size={13} />
            THE CONTEXT BEHIND THE DEAL
          </span>
          <h3>
            {deal.company} · {deal.name}
          </h3>
          <p>{deal.evidence}</p>
          <small>{deal.source}</small>
        </div>
        <div className="crm-deal-action">
          <span className="crm-mini-label">NEXT STEP</span>
          <p>{deal.note}</p>
          <span>
            <CRMAvatar />
            Sam Rivera <span>· Owner</span>
          </span>
        </div>
      </div>
    </div>
  );
}

const SIGNALS = [
  {
    label: "Buying intent",
    Icon: Sparkles,
    title: "An opportunity inside a support reply.",
    quote:
      "We’d like to bring the operations team in. Can you walk us through the rollout requirements?",
    source: "Maya Chen · Support conversation",
    context:
      "Northstar Labs is considering a wider rollout. The account owner can clarify requirements before proposing a plan.",
    action: "Confirm the rollout requirements with Maya.",
    tag: "Conversion",
    status: "Needs review",
  },
  {
    label: "Renewal risk",
    Icon: ShieldCheck,
    title: "A concern to resolve before renewal.",
    quote:
      "Can you confirm the export fix before we discuss next year? We need complete exports for the wider team rollout.",
    source: "Maya Chen · Renewal email",
    context:
      "The $42,000 renewal is in review. EXP-142 is with engineering, and Sam needs a confirmed status before responding.",
    action: "Check the linked work and prepare an update for Maya.",
    tag: "Renewal",
    status: "Follow-up needed",
  },
  {
    label: "Expansion",
    Icon: Building2,
    title: "Another team is ready to join.",
    quote:
      "Once the exports are ready, we’d like to bring our customer success team into the workspace too.",
    source: "Maya Chen · Rollout meeting",
    context:
      "The customer is asking about a broader rollout, with the export fix still in review.",
    action: "Confirm the team size and rollout requirements.",
    tag: "Expansion",
    status: "Needs review",
  },
];

export function CRMSignals() {
  const [selected, setSelected] = useState(1);
  const id = useId();
  const item = SIGNALS[selected];
  return (
    <div className="crm-signals-demo">
      <div className="crm-signal-choices">
        <span className="crm-mini-label">CUSTOMER SIGNALS</span>
        {SIGNALS.map(({ label, Icon }, i) => (
          <button
            key={label}
            type="button"
            aria-pressed={selected === i}
            aria-controls={id}
            onClick={() => setSelected(i)}
          >
            <Icon size={18} />
            <span>{label}</span>
            <ArrowRight size={16} />
          </button>
        ))}
        <p>
          See the evidence.
          <br />
          Choose what happens next.
        </p>
      </div>
      <div
        id={id}
        className="crm-signal-evidence"
        role="region"
        aria-label={item.label + " example"}
      >
        <div className="crm-signal-meta">
          <span>{item.tag}</span>
          <span>
            <CircleDot size={11} />
            {item.status}
          </span>
        </div>
        <h3>{item.title}</h3>
        <blockquote>“{item.quote}”</blockquote>
        <div className="crm-signal-source">
          <CRMAvatar person="maya" />
          <span>{item.source}</span>
        </div>
        <div className="crm-signal-reason">
          <span className="crm-mini-label">WHY IT MATTERS</span>
          <p>{item.context}</p>
        </div>
        <div className="crm-signal-action">
          <Sparkles size={19} />
          <div>
            <span className="crm-mini-label">PROPOSED NEXT STEP</span>
            <p>{item.action}</p>
          </div>
        </div>
      </div>
    </div>
  );
}

export function CRMPlaybook() {
  const { container, active, phase, paused, setPaused } = useCRMPlayback();
  return (
    <div
      className="crm-motion"
      ref={container}
      data-playing={active}
      data-phase={phase}
      role="group"
      aria-label="A renewal playbook connects the customer requirement to engineering review and a proposed customer update. The message remains a draft awaiting approval."
    >
      <div className="crm-demo-toolbar">
        <span>
          <b className="crm-workspace-mark">O</b>OrbitDesk <i>/</i> Playbooks
        </span>
        <button
          type="button"
          onClick={() => setPaused(!paused)}
          aria-pressed={paused}
          aria-label={`${paused ? "Play" : "Pause"} playbook animation`}
        >
          {paused ? <Play size={12} /> : <Pause size={12} />}
        </button>
      </div>
      <div className="crm-playbook-demo">
        <div className="crm-playbook-heading">
          <span className="crm-mini-label">NORTHSTAR LABS / RENEWAL</span>
          <h3>Resolve the concern before the renewal.</h3>
          <p>Outcome: give Maya a confirmed path to a complete export.</p>
        </div>
        <div className="crm-milestone" data-current={phase === 0}>
          <span className="crm-milestone-icon">
            <Check size={15} />
          </span>
          <div>
            <strong>Understand the customer requirement</strong>
            <small>Email and meeting evidence attached</small>
          </div>
          <span className="crm-status">Complete</span>
        </div>
        <div className="crm-milestone" data-current={phase === 1}>
          <span className="crm-milestone-icon">
            <CircleDot size={15} />
          </span>
          <div>
            <strong>Confirm the export fix</strong>
            <small>EXP-142 · Engineering review</small>
          </div>
          <span className="crm-status crm-status-review">In review</span>
        </div>
        <div className="crm-milestone" data-current={phase >= 2}>
          <span className="crm-milestone-icon">
            <Circle size={15} />
          </span>
          <div>
            <strong>Prepare the customer update</strong>
            <small>Sam Rivera · After engineering confirms</small>
          </div>
        </div>
        <div className="crm-playbook-proposal" data-ready={phase >= 2}>
          <div>
            <CRMMark />
            <span>Agent proposal</span>
            <span className="crm-review-pill">Approval required</span>
          </div>
          <div className="crm-proposal-message">
            <span>
              TO <strong>Maya Chen</strong>
            </span>
            <p>
              Hi Maya, engineering is reviewing the export fix. I’ll confirm the
              result before we plan the wider rollout and discuss your renewal.
            </p>
          </div>
          <span>
            <Mail size={13} />
            Customer message · Draft for review
          </span>
        </div>
        <div className="crm-playbook-foot">
          <ShieldCheck size={15} />
          Sam reviews the message before it can be sent.
        </div>
      </div>
    </div>
  );
}
