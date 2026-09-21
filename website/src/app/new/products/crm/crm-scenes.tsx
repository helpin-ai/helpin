"use client";

import { useState, useId, useRef, type CSSProperties } from "react";
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
  X,
} from "lucide-react";
import { useCRMPlayback } from "./use-crm-playback";
import { CRMAvatar, CRMMark, CRMNavigation } from "./crm-workspace";
import { CRMIcon } from "./crm-icons";
import { CRMCompanyLogo } from "./crm-company-logo";
import { COMPANIES } from "./crm-demo-data";

// Customer records and pipeline cards share identities; amounts remain numeric so
// filtered counts and stage totals always reflect the deals actually on screen.
const money = (amount: number) => "$" + amount.toLocaleString("en-US");
const STAGE_COLORS = ["#94a3b8", "#60a5fa", "#f59e0b", "#8b5cf6", "#22c55e"];
const DEALS = [
  [
    0,
    "Annual renewal",
    42000,
    0,
    "Nov 23",
    40,
    "Maya needs complete CSV exports before the wider rollout and renewal discussion. EXP-142 is in engineering review.",
    "Confirm EXP-142 with engineering, then prepare an update.",
    "Renewal discussion · Email",
  ],
  [
    3,
    "Engineering workspace",
    24000,
    0,
    "Nov 25",
    35,
    "The engineering team needs an API usage review before confirming next year’s plan.",
    "Share the API usage report with Leo.",
    "Account review · Meeting",
  ],
  [
    10,
    "Developer workspace",
    16800,
    0,
    "Dec 02",
    30,
    "Ada is reviewing adoption across the engineering teams before renewing.",
    "Schedule the adoption review with Ada.",
    "Renewal planning · Email",
  ],
  [
    2,
    "Workspace renewal",
    18000,
    1,
    "Nov 15",
    55,
    "The account owner recorded updated seat requirements after the review call.",
    "Send the revised renewal proposal for review.",
    "Account review · Meeting",
  ],
  [
    5,
    "Platform team renewal",
    28800,
    1,
    "Nov 18",
    60,
    "The platform team wants to include two additional workspaces in its contract.",
    "Confirm workspace requirements and pricing.",
    "Workspace planning · Meeting",
  ],
  [
    14,
    "Security team renewal",
    36000,
    1,
    "Nov 26",
    50,
    "Leila needs the updated security questionnaire before procurement can proceed.",
    "Return the reviewed security questionnaire.",
    "Security review · Email",
  ],
  [
    1,
    "Support renewal",
    12000,
    2,
    "Nov 21",
    65,
    "The customer has received the renewal terms and is completing its internal review.",
    "Confirm the customer’s decision date.",
    "Renewal terms · Email",
  ],
  [
    7,
    "Team plan renewal",
    9600,
    2,
    "Nov 27",
    70,
    "The team approved the seat count and asked for annual billing terms.",
    "Send the annual billing proposal.",
    "Billing discussion · Email",
  ],
  [
    12,
    "Infrastructure renewal",
    21600,
    2,
    "Dec 04",
    65,
    "Amara’s team has accepted the rollout scope and is reviewing the proposal.",
    "Check whether procurement needs any further details.",
    "Proposal review · Meeting",
  ],
  [
    6,
    "Analytics workspace",
    19200,
    3,
    "Nov 20",
    85,
    "The evaluation is complete. Legal is reviewing the updated agreement.",
    "Follow up on the legal review.",
    "Contract review · Email",
  ],
  [
    11,
    "Customer success renewal",
    32400,
    3,
    "Nov 28",
    80,
    "Kai has confirmed the renewal. The finance team needs the final order form.",
    "Prepare the order form for finance.",
    "Renewal confirmation · Meeting",
  ],
  [
    17,
    "Automation workspace",
    26400,
    3,
    "Dec 03",
    90,
    "The team approved the renewal scope and requested a countersigned agreement.",
    "Send the agreement for signature.",
    "Procurement · Email",
  ],
  [
    4,
    "Annual team plan",
    14400,
    4,
    "Sep 18",
    100,
    "The agreement is signed and the new annual plan is active.",
    "Schedule the next quarterly account review.",
    "Signed agreement · Email",
  ],
  [
    8,
    "Product workspace",
    24000,
    4,
    "Sep 17",
    100,
    "The product team completed its renewal and confirmed its rollout schedule.",
    "Share the rollout checklist with the team.",
    "Renewal completed · Email",
  ],
  [
    9,
    "Engineering plan",
    18000,
    4,
    "Sep 16",
    100,
    "The engineering plan renewed after the team completed its account review.",
    "Arrange the next engineering check-in.",
    "Account review · Meeting",
  ],
] as const;
const PIPELINES = [
  {
    label: "New business",
    stages: ["Discovery", "Evaluation", "Proposal", "Negotiation", "Won"],
  },
  {
    label: "Renewals",
    stages: ["Review", "Follow-up", "Proposal", "Negotiation", "Renewed"],
  },
  {
    label: "Expansion",
    stages: ["Interest", "Scoping", "Proposal", "Negotiation", "Won"],
  },
].map((pipeline, pipelineIndex) => ({
  ...pipeline,
  deals: DEALS.map(
    (
      [
        companyIndex,
        name,
        amount,
        stage,
        date,
        probability,
        evidence,
        note,
        source,
      ],
      index,
    ) => ({
      company: COMPANIES[companyIndex].name,
      companyIndex,
      name:
        pipelineIndex === 1
          ? name
          : pipelineIndex === 0
            ? ["Support workspace", "Engineering workspace", "Team workspace"][
                index % 3
              ]
            : [
                "Additional team seats",
                "Second team rollout",
                "Product team workspace",
              ][index % 3],
      amount:
        pipelineIndex === 1
          ? amount
          : pipelineIndex === 0
            ? amount
            : amount / 2,
      stage,
      date,
      probability,
      owner: COMPANIES[companyIndex].owner,
      evidence:
        pipelineIndex === 1
          ? evidence
          : pipelineIndex === 0
            ? "The team is evaluating a shared workspace for support and product. Their latest conversation captures the requirements behind this opportunity."
            : "The customer wants to extend the workspace to another team. The account review records the rollout scope and requested seats.",
      note:
        pipelineIndex === 1
          ? note
          : stage === 4
            ? "Schedule an onboarding check-in with the team."
            : "Review the requirements with the customer and confirm the next step.",
      source: pipelineIndex === 1 ? source : "Workspace planning · Meeting",
      id: 104 + pipelineIndex * DEALS.length + index,
    }),
  ),
}));

// Matches Deals.tsx / DealBoard / DealCard: stage totals, ID, amount pill,
// probability, close date, and owner. Data and controls stay local to this preview.
export function CRMPipeline() {
  const [pipeline, setPipeline] = useState(1);
  const [agentOpen, setAgentOpen] = useState(true);
  const agentClose = useRef<HTMLButtonElement>(null);
  const agentTrigger = useRef<HTMLButtonElement | null>(null);
  function openDeal(index: number, trigger: HTMLButtonElement) {
    setSelected(index);
    agentTrigger.current = trigger;
    setAgentOpen(true);
    requestAnimationFrame(() =>
      agentClose.current?.focus({ preventScroll: true }),
    );
  }
  function closeAgent() {
    setAgentOpen(false);
    agentTrigger.current?.focus({ preventScroll: true });
  }
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
    <div className="crm-pipeline-demo crm-workspace">
      <div className="crm-pipeline-frame">
        <CRMNavigation view="deals" />
        <div className="crm-pipeline-main">
          <div className="crm-pipeline-top">
            <div>
              <strong>Deals</strong>
              <span className="crm-pipeline-total">
                {visible.length} deals <i />{" "}
                {money(visible.reduce((total, item) => total + item.amount, 0))}
              </span>
            </div>
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
                  onClick={() => {
                    setView(value);
                    setAgentOpen(false);
                  }}
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
                const items = visible.filter((row) => row.stage === i);
                return (
                  <div
                    className="crm-board-column"
                    key={stage}
                    style={
                      {
                        "--crm-stage-color": STAGE_COLORS[i],
                      } as CSSProperties
                    }
                  >
                    <div className="crm-stage-heading">
                      <div>
                        <strong>{stage}</strong>
                        <small>
                          {items.length} {items.length === 1 ? "deal" : "deals"}
                          <span>
                            {money(
                              items.reduce(
                                (total, item) => total + item.amount,
                                0,
                              ),
                            )}
                          </span>
                        </small>
                      </div>
                      <CRMIcon name="PlusSignIcon" size={16} />
                    </div>
                    {items.map((item) => (
                      <button
                        key={item.id}
                        type="button"
                        className="crm-deal-card"
                        aria-pressed={agentOpen && selected === item.index}
                        aria-controls={id}
                        onClick={(event) =>
                          openDeal(item.index, event.currentTarget)
                        }
                      >
                        <span className="crm-deal-id">
                          <i />
                          DEAL-{item.id}
                        </span>
                        <strong>
                          {item.company} · {item.name}
                        </strong>
                        <span className="crm-card-metrics">
                          <span className="crm-deal-amount">
                            USD {money(item.amount).slice(1)}/yr
                          </span>
                          <span className="crm-deal-probability">
                            <i>
                              <b style={{ width: `${item.probability}%` }} />
                            </i>
                            {item.probability}%
                          </span>
                        </span>
                        <span className="crm-deal-bottom">
                          <span>
                            <CRMIcon name="Calendar03Icon" size={12} />
                            {item.date}
                          </span>
                          <CRMAvatar
                            size={28}
                            person={
                              item.owner === "Sam Rivera" ? "sam" : "aisha"
                            }
                          />
                        </span>
                      </button>
                    ))}
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
                    <tr
                      key={item.index}
                      data-selected={selected === item.index}
                    >
                      <td>
                        <button
                          type="button"
                          onClick={(event) =>
                            openDeal(item.index, event.currentTarget)
                          }
                          aria-controls={id}
                        >
                          {item.company} · {item.name}
                        </button>
                      </td>
                      <td>USD {money(item.amount).slice(1)}</td>
                      <td>{current.stages[item.stage]}</td>
                      <td>{item.probability}%</td>
                      <td>
                        <span title={item.owner}>
                          <CRMAvatar
                            size={20}
                            person={
                              item.owner === "Sam Rivera" ? "sam" : "aisha"
                            }
                          />
                        </span>
                      </td>
                      <td>{item.date}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
              {!visible.length && <p>No deals match your search.</p>}
            </div>
          )}
          <aside
            id={id}
            className="crm-deal-agent cw-agent"
            hidden={!agentOpen}
            aria-label={`Ask Agent about ${deal.company}`}
            onKeyDown={(event) => {
              if (event.key === "Escape") {
                event.stopPropagation();
                closeAgent();
              }
            }}
          >
            <header>
              <CRMMark />
              <div>
                <strong>Ask Agent</strong>
                <small>Using your customer context</small>
              </div>
              <button
                ref={agentClose}
                type="button"
                onClick={closeAgent}
                aria-label="Close deal Ask Agent"
              >
                <X size={16} />
              </button>
            </header>
            <div className="crm-deal-agent-body">
              <div className="crm-deal-agent-record">
                <CRMCompanyLogo companyIndex={deal.companyIndex} size={20} />
                <span>
                  {deal.company}
                  <small>
                    {deal.name} · {money(deal.amount)}/yr
                  </small>
                </span>
              </div>
              <div className="cw-agent-question">
                <CRMAvatar />
                <p>
                  {deal.stage === 4
                    ? "What’s the next step for this account?"
                    : "What’s holding up this deal?"}
                </p>
              </div>
              <div className="crm-deal-agent-answer">
                <span className="crm-deal-agent-label">
                  <CRMMark /> Helpin AI
                </span>
                <p>{deal.evidence}</p>
                <span className="crm-deal-agent-source">
                  <CRMIcon name="File01Icon" size={13} />
                  {deal.source}
                </span>
                <div className="crm-deal-agent-next">
                  <strong>Recommended next step</strong>
                  <p>{deal.note}</p>
                  <span>
                    <CRMAvatar
                      size={20}
                      person={deal.owner === "Sam Rivera" ? "sam" : "aisha"}
                    />
                    {deal.owner} · Deal owner
                  </span>
                </div>
              </div>
            </div>
            <footer>
              <ShieldCheck size={13} />
              For your review. No customer message sent.
            </footer>
          </aside>
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
