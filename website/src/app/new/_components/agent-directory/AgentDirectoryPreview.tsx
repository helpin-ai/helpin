"use client";

import {
  useEffect,
  useId,
  useRef,
  useState,
  type KeyboardEvent,
  type CSSProperties,
} from "react";
import {
  BookOpen,
  Bot,
  ChevronDown,
  ChevronRight,
  Clock3,
  FileText,
  FolderKanban,
  LayoutGrid,
  List,
  MessageSquare,
  Settings,
  Sun,
  Workflow,
  Wrench,
  X,
  BriefcaseBusiness,
  PanelLeftClose,
  Bell,
  Search,
} from "lucide-react";
import "./agent-directory.css";

// App references: Agents.tsx AgentRow/AgentCard/system drawer, AutomationShell,
// sidebar/config.ts, and AgentAvatar.tsx. Prepared OrbitDesk data only.
const AGENTS = [
  {
    name: "Atlas",
    role: "Epic planner",
    purpose:
      "Turn customer requirements into scoped epics, milestones, and a delivery plan.",
    mode: "Interactive",
    runs: 24,
    last: "12m ago",
    flows: 2,
    status: "Completed",
    tools: ["search_workspace", "list_tasks", "get_task_context"],
    flow: "Plan work from customer requests",
  },
  {
    name: "Scribe",
    role: "Coding task planner",
    purpose:
      "Break approved work into clear tasks with requirements and acceptance criteria.",
    mode: "Interactive",
    runs: 36,
    last: "8m ago",
    flows: 3,
    status: "Completed",
    tools: ["list_tasks", "get_task_context", "create_task"],
    flow: "Prepare engineering tasks",
  },
  {
    name: "Forge",
    role: "Coding agent",
    purpose:
      "Investigate the linked issue, implement the change, and prepare it for review.",
    mode: "Autonomous",
    runs: 18,
    last: "4m ago",
    flows: 2,
    status: "Running",
    tools: ["get_task_context", "get_pull_request_diff", "read_document"],
    flow: "Implement assigned engineering work",
  },
  {
    name: "Echo",
    role: "Help chat agent",
    purpose:
      "Answer customers with relevant knowledge and the history behind their question.",
    mode: "Interactive",
    runs: 128,
    last: "2m ago",
    flows: 2,
    status: "Completed",
    tools: [
      "search_knowledge",
      "draft_support_reply",
      "update_conversation_status",
    ],
    flow: "Answer incoming customer conversations",
  },
  {
    name: "Lens",
    role: "QA & code reviewer",
    purpose:
      "Check the proposed fix against the original requirements and flag what needs attention.",
    mode: "Interactive",
    runs: 21,
    last: "6m ago",
    flows: 2,
    status: "Completed",
    tools: ["get_task_context", "get_pull_request_diff", "read_document"],
    flow: "Review proposed engineering changes",
  },
  {
    name: "Beacon",
    role: "CRM operator",
    purpose:
      "Review customer signals and prepare the next step with the account context attached.",
    mode: "Interactive",
    runs: 42,
    last: "18m ago",
    flows: 3,
    status: "Needs approval",
    tools: [
      "list_deals",
      "list_contacts",
      "list_crm_signals",
      "add_deal_note",
      "update_deal_stage",
    ],
    flow: "Review renewal signals",
  },
  {
    name: "Quill",
    role: "Documentation agent",
    purpose:
      "Keep customer-facing answers aligned with product changes and what the team learns.",
    mode: "Interactive",
    runs: 15,
    last: "32m ago",
    flows: 2,
    status: "Completed",
    tools: [
      "list_documents",
      "read_document",
      "get_document_blocks",
      "edit_document",
    ],
    flow: "Review documentation after a release",
  },
  {
    name: "Mira",
    role: "Marketer",
    purpose:
      "Bring customer insights and product knowledge into useful launch and campaign content.",
    mode: "Interactive",
    runs: 9,
    last: "1h ago",
    flows: 1,
    status: "Completed",
    tools: ["search_workspace", "read_document", "list_documents"],
    flow: "Prepare a product launch brief",
  },
];
const TABS = ["Versions", "Analytics", "Limits"] as const;
const MODULES = [
  [FolderKanban, "Projects"],
  [MessageSquare, "Support"],
  [FileText, "Docs"],
  [BriefcaseBusiness, "CRM"],
  [Bot, "Automation"],
  [Settings, "Settings"],
] as const;
function Avatar({ name, size = 32 }: { name: string; size?: number }) {
  return (
    <img
      className="adp-avatar"
      style={{ "--avatar-size": `${size}px` } as CSSProperties}
      src={`/new/agents/${name.toLowerCase()}.svg`}
      width={size}
      height={size}
      alt=""
    />
  );
}
function RunBars({ pending = false }: { pending?: boolean }) {
  return (
    <span className="adp-bars" aria-hidden="true">
      {[0, 1, 2, 3, 4].map((n) => (
        <i key={n} data-pending={pending && n === 4} />
      ))}
    </span>
  );
}
export function AgentDirectoryPreview() {
  const [view, setView] = useState<"list" | "cards">("list");
  const [selected, setSelected] = useState<number | null>(null);
  const [tab, setTab] = useState<(typeof TABS)[number]>("Versions");
  const dialog = useRef<HTMLDialogElement>(null);
  const id = useId();
  const agent = selected === null ? null : AGENTS[selected];
  useEffect(() => {
    if (selected !== null && !dialog.current?.open) dialog.current?.showModal();
  }, [selected]);
  function openAgent(index: number) {
    setTab("Versions");
    setSelected(index);
  }
  function moveTab(event: KeyboardEvent<HTMLButtonElement>, index: number) {
    const next =
      event.key === "ArrowRight"
        ? (index + 1) % 3
        : event.key === "ArrowLeft"
          ? (index + 2) % 3
          : event.key === "Home"
            ? 0
            : event.key === "End"
              ? 2
              : -1;
    if (next < 0) return;
    event.preventDefault();
    setTab(TABS[next]);
    document.getElementById(`${id}-tab-${next}`)?.focus();
  }
  return (
    <div className="adp" aria-label="OrbitDesk agents workspace preview">
      <div className="adp-frame">
        <aside className="adp-sidebar" aria-label="Workspace navigation">
          <div className="adp-brand">
            <b>O</b>
            <strong>OrbitDesk</strong>
            <ChevronDown size={12} />
            <Bell size={15} />
            <PanelLeftClose size={15} />
          </div>
          <div className="adp-navigation">
            <div className="adp-rail" aria-hidden="true">
              <div>
                {MODULES.map(([Icon, label]) => (
                  <span key={label} data-active={label === "Automation"}>
                    <i>
                      <Icon size={16} />
                    </i>
                    <small>{label}</small>
                  </span>
                ))}
              </div>
              <div className="adp-account">
                <Sun size={14} />
                <img
                  src="/new/crm/ask-agent-mark.svg"
                  width={24}
                  height={24}
                  alt=""
                />
                <img
                  src="/new/avatars/sam.webp"
                  width={32}
                  height={32}
                  alt=""
                />
              </div>
            </div>
            <div className="adp-module">
              <div className="adp-nav-items">
                <span>
                  <Workflow size={16} />
                  Flows
                </span>
                <span data-active="true">
                  <Bot size={16} />
                  Agents
                </span>
                <span>
                  <Clock3 size={16} />
                  Activity<i>1</i>
                </span>
                <small>Catalog</small>
                <span>
                  <Bot size={16} />
                  Triggers
                </span>
                <span>
                  <Wrench size={16} />
                  Tools
                </span>
                <span>
                  <BookOpen size={16} />
                  Skills
                </span>
              </div>
              <div className="adp-search" aria-hidden="true">
                <Search size={14} />
                Search OrbitDesk<small>⌘K</small>
              </div>
            </div>
          </div>
        </aside>
        <div className="adp-main">
          <header className="adp-header">
            <div>
              <h3>Agents</h3>
              <p>
                Built-in and custom agents for manual runs and automated flows.
              </p>
            </div>
            <div
              className="adp-view"
              role="group"
              aria-label="Agent directory view"
            >
              <button
                type="button"
                aria-label="Show agents as a list"
                aria-pressed={view === "list"}
                onClick={() => setView("list")}
              >
                <List size={17} />
              </button>
              <button
                type="button"
                aria-label="Show agents as a grid"
                aria-pressed={view === "cards"}
                onClick={() => setView("cards")}
              >
                <LayoutGrid size={16} />
              </button>
            </div>
          </header>
          <div className="adp-scroll" data-view={view}>
            {view === "list" && (
              <div className="adp-columns" aria-hidden="true">
                <span>Agent</span>
                <span>Config</span>
                <span>Runs · 7d</span>
                <span>Last run</span>
                <span>Used in flows</span>
                <span />
              </div>
            )}
            <ul className="adp-agents">
              {AGENTS.map((item, index) => (
                <li key={item.name}>
                  <button
                    className="adp-agent"
                    type="button"
                    onClick={() => openAgent(index)}
                    aria-label={`Open ${item.name}`}
                  >
                    <span className="adp-identity">
                      <Avatar name={item.name} />
                      <span>
                        <strong>{item.name}</strong>
                        <span className="adp-role">
                          {item.role}
                          <i>System</i>
                        </span>
                      </span>
                    </span>
                    {view === "cards" && (
                      <span className="adp-purpose">{item.purpose}</span>
                    )}
                    <span className="adp-config">
                      <small className="adp-cell-label">Config</small>
                      <strong>Workspace default</strong>
                      <small>{item.mode}</small>
                    </span>
                    <span className="adp-runs">
                      <small className="adp-cell-label">Runs · 7d</small>
                      <span>
                        <RunBars pending={item.status !== "Completed"} />
                        {item.runs}
                      </span>
                    </span>
                    <span className="adp-last">
                      <small className="adp-cell-label">Last run</small>
                      <span className="adp-status" data-status={item.status}>
                        {item.status}
                      </span>
                      <small>{item.last}</small>
                    </span>
                    <span className="adp-flows">
                      <small className="adp-cell-label">Used in flows</small>
                      {item.flows} {item.flows === 1 ? "flow" : "flows"}
                    </span>
                    <ChevronRight
                      className="adp-open"
                      size={15}
                      aria-hidden="true"
                    />
                  </button>
                </li>
              ))}
            </ul>
          </div>
        </div>
      </div>
      <dialog
        className="adp-dialog"
        ref={dialog}
        aria-labelledby={`${id}-title`}
        onClose={() => setSelected(null)}
        onClick={(e) => {
          if (e.target === dialog.current) dialog.current.close();
        }}
      >
        {agent && (
          <div className="adp-dialog-inner" key={agent.name}>
            <header className="adp-dialog-header">
              <Avatar name={agent.name} size={44} />
              <div>
                <h3 id={`${id}-title`}>{agent.name}</h3>
                <p>{agent.role} · OrbitDesk</p>
              </div>
              <button
                type="button"
                aria-label="Close agent details"
                onClick={() => dialog.current?.close()}
                autoFocus
              >
                <X size={18} />
              </button>
            </header>
            <div className="adp-tabs" role="tablist" aria-label="Agent details">
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
                  onKeyDown={(e) => moveTab(e, index)}
                >
                  {name}
                </button>
              ))}
            </div>
            <div
              className="adp-detail-body"
              id={`${id}-panel`}
              role="tabpanel"
              aria-labelledby={`${id}-tab-${TABS.indexOf(tab)}`}
              tabIndex={0}
            >
              {tab === "Versions" && (
                <>
                  <div className="adp-version">
                    <div>
                      <strong>Customer context</strong>
                      <span>Workspace version</span>
                      <b>Current</b>
                    </div>
                    <p>{agent.purpose}</p>
                    <dl>
                      <div>
                        <dt>AI profile</dt>
                        <dd>Workspace default</dd>
                      </div>
                      <div>
                        <dt>Mode</dt>
                        <dd>{agent.mode}</dd>
                      </div>
                      <div>
                        <dt>Created</dt>
                        <dd>Sep 18, 2026</dd>
                      </div>
                      <div>
                        <dt>Last run</dt>
                        <dd>{agent.last}</dd>
                      </div>
                    </dl>
                  </div>
                  <div className="adp-detail-label">
                    <h4>Version details</h4>
                    <span>Read-only</span>
                  </div>
                  <details className="adp-disclosure">
                    <summary>System prompt</summary>
                    <p>
                      {agent.purpose} Use the relevant customer history and
                      linked work. Explain your findings, preserve the source
                      context, and follow the workspace’s approval requirements.
                    </p>
                  </details>
                  <details className="adp-disclosure" open>
                    <summary>Allowed tools</summary>
                    <div>
                      <p>
                        Allowed tools are the actions and data sources this
                        version may call.
                      </p>
                      <div className="adp-tool-tags">
                        {agent.tools.map((tool) => (
                          <code key={tool}>{tool}</code>
                        ))}
                      </div>
                    </div>
                  </details>
                </>
              )}
              {tab === "Analytics" && (
                <>
                  <h4>Agent analytics</h4>
                  <p className="adp-description">
                    Usage and run health for this agent · Last 7 days
                  </p>
                  <div className="adp-metrics">
                    <div>
                      <small>Runs · 7d</small>
                      <strong>{agent.runs}</strong>
                    </div>
                    <div>
                      <small>Completed</small>
                      <strong>
                        {agent.runs - (agent.status === "Completed" ? 0 : 1)}
                      </strong>
                    </div>
                    <div>
                      <small>Awaiting approval</small>
                      <strong>
                        {agent.status === "Needs approval" ? 1 : 0}
                      </strong>
                    </div>
                  </div>
                  <div className="adp-run-summary">
                    <h4>Latest run</h4>
                    <div>
                      <Avatar name={agent.name} />
                      <span>
                        <strong>{agent.flow}</strong>
                        <small>{agent.last}</small>
                      </span>
                      <span className="adp-status" data-status={agent.status}>
                        {agent.status}
                      </span>
                    </div>
                  </div>
                </>
              )}
              {tab === "Limits" && (
                <>
                  <h4>Agent limits</h4>
                  <p className="adp-description">
                    Usage limits that apply across all versions of this agent.
                  </p>
                  <div className="adp-metrics">
                    <div>
                      <small>Monthly limit</small>
                      <strong>1M</strong>
                      <span>Tokens allowed this month</span>
                    </div>
                    <div>
                      <small>Used this month</small>
                      <strong>{(agent.runs * 4000) / 1000}k</strong>
                      <span>
                        {Math.round((agent.runs * 4000) / 10000)}% of limit
                      </span>
                    </div>
                    <div>
                      <small>Total tokens</small>
                      <strong>{(agent.runs * 8000) / 1000}k</strong>
                      <span>Across recorded runs</span>
                    </div>
                  </div>
                  <div className="adp-limit">
                    <span>Monthly token limit</span>
                    <strong>1,000,000</strong>
                    <div>
                      <i style={{ width: `${(agent.runs * 4000) / 10000}%` }} />
                    </div>
                  </div>
                </>
              )}
            </div>
          </div>
        )}
      </dialog>
    </div>
  );
}
