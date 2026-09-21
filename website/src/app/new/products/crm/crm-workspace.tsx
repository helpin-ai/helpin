"use client";

import { Fragment, useEffect, useId, useRef, useState } from "react";
import {
  ArrowLeft,
  ArrowUp,
  BookOpen,
  Building2,
  Check,
  ChevronDown,
  Flag,
  Link2,
  ListChecks,
  Mail,
  MessageSquare,
  MoreHorizontal,
  Pause,
  Play,
  Search,
  Settings,
  Sparkles,
  Video,
  Zap,
} from "lucide-react";
import {
  CONTACTS,
  COMPANIES,
  ACCOUNT_SUMMARY,
  ACCOUNT_BRIEF,
} from "./crm-demo-data";
import { useCRMPlayback } from "./use-crm-playback";
import { CRMIcon } from "./crm-icons";
import "./crm-workspace.css";

type View = "contacts" | "companies";
export function CRMAvatar({
  person = "sam",
  size = 23,
}: {
  person?: string;
  size?: number;
}) {
  return (
    <img
      className="cw-avatar"
      src={`/new/avatars/${person}.webp`}
      width={size}
      height={size}
      alt=""
    />
  );
}
export function CRMMark() {
  return (
    <span className="cw-ai-mark">
      <img src="/brand/helpin-icon-white.svg" width={15} height={15} alt="" />
    </span>
  );
}
// Layout and navigation order mirror Sidebar.tsx, SidebarRail, and CrmRailNav.
function Navigation({
  view,
  onView,
}: {
  view: View;
  onView: (view: View) => void;
}) {
  const modules = [
    ["FolderKanbanIcon", "Projects"],
    ["Message01Icon", "Support"],
    ["File01Icon", "Docs"],
    ["Briefcase01Icon", "CRM"],
    ["BotIcon", "Automation"],
    ["Setting07Icon", "Settings"],
  ] as const;
  const links = [
    ["DollarCircleIcon", "Deals"],
    ["InboxIcon", "Emails"],
    ["Camera01Icon", "Meetings"],
    ["BookOpen01Icon", "Playbooks"],
    ["BulbIcon", "Signals"],
  ] as const;
  return (
    <aside className="cw-nav">
      <div className="cw-brand">
        <b>O</b>
        <strong>OrbitDesk</strong>
        <CRMIcon name="ArrowDown01Icon" size={12} />
        <CRMIcon name="Notification01Icon" size={15} />
        <CRMIcon name="SidebarLeft01Icon" size={15} />
      </div>
      <div className="cw-nav-columns">
        <div className="cw-apps">
          <div aria-hidden="true">
            {modules.map(([name, label]) => (
              <span key={label} data-selected={label === "CRM"}>
                <i>
                  <CRMIcon name={name} />
                </i>
                <small>{label}</small>
              </span>
            ))}
          </div>
          <div className="cw-app-account" aria-hidden="true">
            <CRMIcon name="Sun01Icon" size={14} />
            <CRMIcon name="PlayIcon" size={15} />
            <CRMAvatar size={28} />
          </div>
        </div>
        <div className="cw-crm">
          <span>
            <CRMIcon name="ChartColumnIcon" />
            Overview
          </span>
          {(["contacts", "companies"] as const).map((item) => (
            <button
              type="button"
              key={item}
              aria-pressed={view === item}
              onClick={() => onView(item)}
            >
              <CRMIcon
                name={item === "contacts" ? "UserGroupIcon" : "Building03Icon"}
              />
              <span>{item === "contacts" ? "Contacts" : "Companies"}</span>
            </button>
          ))}
          {links.map(([name, label]) => (
            <span
              key={label}
              className={label === "Playbooks" ? "cw-nav-separator" : undefined}
            >
              <CRMIcon name={name} />
              {label}
            </span>
          ))}
          <div className="cw-module-footer" aria-hidden="true">
            <CRMIcon name="WorkflowSquare01Icon" />
            <CRMIcon name="Mail01Icon" />
            <CRMIcon name="SparklesIcon" />
          </div>
        </div>
      </div>
      <div className="cw-sidebar-search" aria-hidden="true">
        <CRMIcon name="Search01Icon" size={15} />
        <span>Search OrbitDesk</span>
        <small>⌘K</small>
      </div>
    </aside>
  );
}
const DETAIL_TABS = [
  "Overview",
  "Tasks",
  "Emails",
  "Meetings",
  "Calls",
  "Deals",
  "Support",
  "Notes",
] as const;
type DetailTab = (typeof DETAIL_TABS)[number];
function RecordDetail({
  contactIndex,
  entity,
  onBack,
  tab,
  onTab,
}: {
  contactIndex: number;
  entity: View;
  onBack: () => void;
  tab: DetailTab;
  onTab: (tab: DetailTab) => void;
}) {
  const contact = CONTACTS[contactIndex],
    company = COMPANIES[contact.company],
    primary = contactIndex === 0;
  const id = useId();
  const heading = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    heading.current?.focus({ preventScroll: true });
  }, [contactIndex, entity]);
  return (
    <div className="cw-record">
      <header className="cw-record-header">
        <button ref={heading} type="button" onClick={onBack}>
          <ArrowLeft size={13} />
          {entity === "contacts" ? "Contacts" : "Companies"}
        </button>
        <div>
          {entity === "companies" ? (
            <span className="cw-company-initials">{company.initials}</span>
          ) : (
            <span className="cw-initials">{contact.initials}</span>
          )}
          <div>
            <h3>{entity === "contacts" ? contact.name : company.name}</h3>
            <p>
              {entity === "contacts"
                ? `${contact.role} at ${company.name}`
                : `${company.domain} · ${company.industry}`}
              <span className="cw-record-id">
                {entity === "contacts" ? "CON" : "COM"}-
                {String(contactIndex + 1).padStart(3, "0")}
              </span>
              <span className="cw-record-lifecycle">{contact.stage}</span>
            </p>
          </div>
          <div className="cw-record-actions" aria-hidden="true">
            <CRMIcon name="Mail01Icon" size={15} />
            <span>
              <CRMIcon name="PlusSignIcon" size={14} />
              New deal
            </span>
          </div>
        </div>
      </header>
      <div className="cw-record-grid">
        <div className="cw-record-main">
          <div
            className="cw-detail-tabs"
            role="tablist"
            aria-label="Customer record views"
          >
            {DETAIL_TABS.map((value, index) => (
              <button
                type="button"
                role="tab"
                key={value}
                id={`${id}-${value}`}
                aria-controls={`${id}-panel`}
                aria-selected={tab === value}
                tabIndex={tab === value ? 0 : -1}
                onClick={() => onTab(value)}
                onKeyDown={(e) => {
                  if (
                    ["ArrowRight", "ArrowLeft", "Home", "End"].includes(e.key)
                  ) {
                    e.preventDefault();
                    const next =
                      e.key === "Home"
                        ? 0
                        : e.key === "End"
                          ? DETAIL_TABS.length - 1
                          : (index +
                              (e.key === "ArrowRight" ? 1 : -1) +
                              DETAIL_TABS.length) %
                            DETAIL_TABS.length;
                    onTab(DETAIL_TABS[next]);
                    document
                      .getElementById(`${id}-${DETAIL_TABS[next]}`)
                      ?.focus();
                  }
                }}
              >
                {value}
              </button>
            ))}
          </div>
          <div
            className="cw-record-content"
            role="tabpanel"
            id={`${id}-panel`}
            aria-labelledby={`${id}-${tab}`}
          >
            {tab === "Overview" ? (
              <>
                <div className="cw-summary">
                  <span>
                    <CRMIcon name="SparklesIcon" size={15} />
                    Summary
                  </span>
                  <p>
                    {primary
                      ? ACCOUNT_SUMMARY
                      : `${contact.name} is the ${contact.role.toLowerCase()} at ${company.name}. ${contact.owner} owns the relationship. Review the linked records before planning the next conversation.`}
                  </p>
                  {primary && (
                    <div className="cw-summary-context">
                      <div>
                        <small>Next step</small>
                        <p>Confirm the export fix with engineering.</p>
                        <span>
                          <CRMAvatar size={20} />
                          Sam Rivera
                        </span>
                      </div>
                      <div>
                        <small>Sources</small>
                        <button type="button" onClick={() => onTab("Emails")}>
                          <CRMIcon name="Mail01Icon" size={13} />
                          Renewal planning
                        </button>
                        <button type="button" onClick={() => onTab("Tasks")}>
                          <CRMIcon name="CheckListIcon" size={13} />
                          EXP-142 · In review
                        </button>
                      </div>
                    </div>
                  )}
                </div>
                <div className="cw-signal">
                  <span>
                    <Zap size={14} />
                    CRM signals
                  </span>
                  <strong>
                    {primary
                      ? "Renewal depends on the export workflow"
                      : "Keep the next conversation in view"}
                  </strong>
                  <p>
                    {primary
                      ? "Maya needs a clear update before discussing the next term."
                      : "Review the customer’s requirements with the account owner."}
                  </p>
                  <small>Needs review · {contact.owner}</small>
                </div>
                <div className="cw-activity-heading">
                  Activity{" "}
                  <span>
                    All activities <ChevronDown size={12} />
                  </span>
                </div>
                {primary ? (
                  <div className="cw-timeline">
                    <div>
                      <Mail size={15} />
                      <span>
                        <strong>Renewal planning</strong>
                        <p>
                          “Can you confirm the export fix before we discuss next
                          year?”
                        </p>
                        <small>Maya Chen · Email · 18m ago</small>
                      </span>
                    </div>
                    <div>
                      <Video size={15} />
                      <span>
                        <strong>Rollout & renewal review</strong>
                        <p>
                          Complete exports are required for the wider team
                          rollout.
                        </p>
                        <small>Meeting · Sep 18 · 32 minutes</small>
                      </span>
                    </div>
                    <div>
                      <ListChecks size={15} />
                      <span>
                        <strong>EXP-142 · Fix incomplete CSV exports</strong>
                        <p>
                          Engineering review in progress. Sam owns the
                          follow-up.
                        </p>
                        <small>Linked task · In review</small>
                      </span>
                    </div>
                  </div>
                ) : (
                  <div className="cw-empty-record">
                    <MessageSquare size={22} />
                    <strong>The relationship has a place to grow.</strong>
                    <p>
                      Notes, conversations, and linked work appear in the
                      customer timeline.
                    </p>
                  </div>
                )}
              </>
            ) : primary ? (
              <div className="cw-focused-record">
                <span className="cw-focus-label">{tab}</span>
                {tab === "Tasks" ? (
                  <>
                    <ListChecks size={24} />
                    <h4>EXP-142 · Fix incomplete CSV exports</h4>
                    <span className="cw-review-state">In review</span>
                    <p>
                      Exports stop at 10,000 rows when filters are applied. The
                      pagination fix is ready for engineering review.
                    </p>
                    <div>
                      <CRMAvatar />
                      Sam Rivera · Customer follow-up owner
                    </div>
                  </>
                ) : tab === "Emails" ? (
                  <>
                    <Mail size={24} />
                    <h4>Renewal planning</h4>
                    <p>
                      Hi Sam, can you confirm the export fix before we discuss
                      next year? We need complete exports for the wider team
                      rollout.
                    </p>
                    <div>
                      <CRMAvatar person="maya" />
                      Maya Chen · 18m ago
                    </div>
                  </>
                ) : tab === "Meetings" ? (
                  <>
                    <Video size={24} />
                    <h4>Rollout & renewal review</h4>
                    <p>
                      Decision: validate the complete CSV export before adding
                      the wider team. Sam will confirm the engineering review
                      and prepare a customer update.
                    </p>
                    <div>Sep 18 · 32 minutes · 3 participants</div>
                  </>
                ) : tab === "Deals" ? (
                  <>
                    <Flag size={24} />
                    <h4>Northstar Labs · Annual renewal</h4>
                    <strong className="cw-deal-value">$42,000</strong>
                    <p>Renewals pipeline · Review · Expected close Nov 23</p>
                    <div>
                      <CRMAvatar />
                      Sam Rivera · Owner
                    </div>
                  </>
                ) : tab === "Calls" || tab === "Notes" ? (
                  <>
                    <CRMIcon
                      name={tab === "Calls" ? "Camera01Icon" : "File01Icon"}
                      size={24}
                    />
                    <h4>
                      {tab === "Calls"
                        ? "Rollout check-in"
                        : "Renewal preparation"}
                    </h4>
                    <p>
                      {tab === "Calls"
                        ? "Maya confirmed that complete exports are needed before the wider rollout."
                        : "Confirm EXP-142 with engineering, then prepare the customer update. Sam owns the next step."}
                    </p>
                    <div>Sam Rivera · Sep 18</div>
                  </>
                ) : (
                  <>
                    <MessageSquare size={24} />
                    <h4>CSV export stops early</h4>
                    <p>
                      “Our CSV export stops at 10,000 rows when filters are
                      applied.”
                    </p>
                    <div>
                      <CRMAvatar person="maya" />
                      Maya Chen · Linked to EXP-142
                    </div>
                  </>
                )}
              </div>
            ) : (
              <div className="cw-empty-record">
                <BookOpen size={24} />
                <strong>
                  No {tab.toLowerCase()} attached in this preview.
                </strong>
                <p>
                  Open Maya’s record to explore the connected customer story.
                </p>
              </div>
            )}
          </div>
        </div>
        <aside className="cw-properties">
          <h4>
            {entity === "contacts" ? "Contact details" : "Company details"}
          </h4>
          <dl>
            {(entity === "contacts"
              ? [
                  ["Email", contact.email],
                  ["Job title", contact.role],
                  ["Stage", contact.stage],
                  ["Owner", contact.owner],
                  ["Source", "Website"],
                ]
              : [
                  ["Domain", company.domain],
                  ["Industry", company.industry],
                  ["Employees", company.employees],
                  ["Annual revenue", company.revenue],
                  ["Owner", company.owner],
                ]
            ).map(([label, value]) => (
              <div key={label}>
                <dt>{label}</dt>
                <dd>{value}</dd>
              </div>
            ))}
          </dl>
          <h4>{entity === "contacts" ? "Companies" : "Contacts"}</h4>
          <div className="cw-association">
            <Building2 size={14} />
            <span>
              {entity === "contacts" ? company.name : contact.name}
              <small>
                {entity === "contacts" ? "Primary company" : contact.role}
              </small>
            </span>
          </div>
          {primary && (
            <>
              <h4>Deals</h4>
              <div className="cw-association">
                <Flag size={14} />
                <span>
                  Annual renewal<small>$42,000 · Review</small>
                </span>
              </div>
              <h4>Tasks</h4>
              <div className="cw-association">
                <ListChecks size={14} />
                <span>
                  EXP-142<small>Fix incomplete CSV exports</small>
                </span>
              </div>
            </>
          )}
        </aside>
      </div>
    </div>
  );
}
function AccountAgent({ active, phase }: { active: boolean; phase: number }) {
  const [typed, setTyped] = useState(ACCOUNT_BRIEF);
  useEffect(() => {
    if (!active || phase === 3) {
      setTyped(ACCOUNT_BRIEF);
      return;
    }
    if (phase < 2) {
      setTyped("");
      return;
    }
    let at = 0;
    const timer = setInterval(() => {
      at = Math.min(at + 4, ACCOUNT_BRIEF.length);
      setTyped(ACCOUNT_BRIEF.slice(0, at));
      if (at === ACCOUNT_BRIEF.length) clearInterval(timer);
    }, 24);
    return () => clearInterval(timer);
  }, [active, phase]);
  return (
    <div
      className="cw-agent"
      role="img"
      aria-label="Ask Agent reviews Maya’s renewal email, the Northstar deal, and linked task EXP-142, then recommends confirming the export fix before drafting a customer update. No deal is changed and no message is sent."
    >
      <div aria-hidden="true">
        <header>
          <CRMMark />
          <span>
            <strong>Ask Agent</strong>
            <small>Northstar account brief</small>
          </span>
          <MoreHorizontal size={16} />
        </header>
        <div className="cw-agent-chat">
          <div className="cw-agent-question">
            <CRMAvatar />
            <p>What should I know before Maya’s renewal call?</p>
          </div>
          <div className="cw-agent-label">
            <CRMMark />
            Ask Agent
          </div>
          <div className="cw-agent-sources">
            <span data-ready={phase >= 1}>
              <Mail size={13} />
              Renewal email
              <Check size={12} />
            </span>
            <span data-ready={phase >= 2}>
              <ListChecks size={13} />
              EXP-142 · In review
              <Check size={12} />
            </span>
          </div>
          <div className="cw-agent-answer">
            <p className="cw-ghost">{ACCOUNT_BRIEF}</p>
            <p>{typed}</p>
          </div>
          <div className="cw-agent-result" data-ready={phase === 3}>
            <CRMAvatar />
            <span>
              Next step for Sam
              <small>Confirm the fix, then prepare the update.</small>
            </span>
          </div>
        </div>
        <div className="cw-agent-composer">
          <span>
            <Link2 size={11} />
            Maya Chen · Northstar Labs
          </span>
          <p>Ask a follow-up…</p>
          <div>
            <span>Workspace context</span>
            <ArrowUp size={14} />
          </div>
        </div>
      </div>
    </div>
  );
}
export function CRMWorkspace({
  mode = "directory",
}: {
  mode?: "directory" | "account";
}) {
  const playback = useCRMPlayback();
  const { container, active, phase, paused, setPaused } = playback;
  const [manualView, setManualView] = useState<View | null>(null);
  const [search, setSearch] = useState("");
  const [owner, setOwner] = useState("all");
  const [selected, setSelected] = useState<number | null>(
    mode === "account" ? 0 : null,
  );
  const [tab, setTab] = useState<DetailTab>("Overview");
  const [agent, setAgent] = useState(false);
  const [manualAgent, setManualAgent] = useState(false);
  const view =
    manualView ??
    (mode === "directory" && active && phase >= 2 ? "companies" : "contacts");
  const showAgent =
    mode === "account" && (manualAgent ? agent : !active || phase >= 1);
  const [selection, setSelection] = useState<number[]>([]);
  const [showEmail, setShowEmail] = useState(true);
  const [showCreated, setShowCreated] = useState(true);
  const [groupBy, setGroupBy] = useState("none");
  const searchRef = useRef<HTMLInputElement>(null);
  const id = useId();
  const changeView = (next: View) => {
    setPaused(true);
    setManualView(next);
    setSelected(null);
    setSelection([]);
    setSearch("");
    setOwner("all");
    setManualAgent(true);
    setAgent(false);
  };
  const openContact = (index: number) => {
    setPaused(true);
    setSelected(index);
    setTab("Overview");
    setManualAgent(true);
    setAgent(false);
  };
  const rows = (
    view === "contacts"
      ? CONTACTS.map((contact, index) => ({ ...contact, index }))
      : COMPANIES.map((company, index) => ({ ...company, index }))
  ).filter(
    (item) =>
      (owner === "all" || item.owner === owner) &&
      (item.name + " " + ("email" in item ? item.email : item.domain))
        .toLowerCase()
        .includes(search.toLowerCase()),
  );
  return (
    <div
      className={`crm-workspace cw-${mode}`}
      ref={container}
      onFocusCapture={(e) => {
        if (mode === "directory" && !e.target.closest(".cw-playback")) {
          setManualView(view);
          setPaused(true);
        }
      }}
      data-playing={active}
      data-phase={phase}
    >
      <div
        className="cw-frame"
        inert={showAgent ? true : undefined}
        aria-hidden={showAgent ? true : undefined}
      >
        <Navigation view={view} onView={changeView} />
        <div className="cw-main">
          {selected === null ? (
            <>
              <header className="cw-list-header">
                <h3>
                  {view === "contacts" ? "Contacts" : "Companies"}{" "}
                  <span>({rows.length})</span>
                </h3>
                <span className="cw-add-record" aria-hidden="true">
                  <CRMIcon name="PlusSignIcon" />
                  {view === "contacts" ? "Add contact" : "Add company"}
                </span>
              </header>
              <div className="cw-filters">
                <label className="cw-search">
                  <CRMIcon name="Search01Icon" />
                  <input
                    type="search"
                    ref={searchRef}
                    aria-label={`Search ${view}`}
                    placeholder={`Search ${view}...`}
                    value={search}
                    onChange={(e) => {
                      setPaused(true);
                      setManualView(view);
                      setSearch(e.target.value);
                    }}
                  />
                </label>
                <details className="cw-filter-menu">
                  <summary>
                    <CRMIcon name="FilterHorizontalIcon" size={14} />
                    Filters
                    <ChevronDown size={12} />
                    {owner !== "all" && <b>1</b>}
                  </summary>
                  <div>
                    <label>
                      Owner
                      <select
                        aria-label="Filter records by owner"
                        value={owner}
                        onChange={(e) => {
                          setPaused(true);
                          setManualView(view);
                          setOwner(e.target.value);
                        }}
                      >
                        <option value="all">All owners</option>
                        <option>Sam Rivera</option>
                        <option>Aisha Patel</option>
                      </select>
                    </label>
                  </div>
                </details>
                <div className="cw-table-tools">
                  <label className="cw-groupby">
                    <CRMIcon name="UserGroupIcon" size={14} />
                    <select
                      aria-label="Group records"
                      value={groupBy}
                      onChange={(e) => {
                        setGroupBy(e.target.value);
                        setPaused(true);
                        setManualView(view);
                      }}
                    >
                      <option value="none">No grouping</option>
                      <option value="owner">Owner</option>
                    </select>
                  </label>
                  <details className="cw-filter-menu cw-columns-menu">
                    <summary>
                      <CRMIcon name="ViewIcon" size={14} />
                      Columns
                    </summary>
                    <div>
                      {view === "contacts" && (
                        <label>
                          <input
                            type="checkbox"
                            checked={showEmail}
                            onChange={(e) => setShowEmail(e.target.checked)}
                          />
                          Email
                        </label>
                      )}
                      <label>
                        <input
                          type="checkbox"
                          checked={showCreated}
                          onChange={(e) => setShowCreated(e.target.checked)}
                        />
                        Created
                      </label>
                    </div>
                  </details>
                </div>
              </div>
              {owner !== "all" && (
                <div className="cw-active-filter">
                  Owner is {owner}
                  <button
                    type="button"
                    onClick={() => setOwner("all")}
                    aria-label="Remove owner filter"
                  >
                    ×
                  </button>
                </div>
              )}
              <div className="cw-table-scroll">
                <table
                  aria-label={
                    view === "contacts"
                      ? "Contacts preview"
                      : "Companies preview"
                  }
                >
                  <thead>
                    <tr>
                      <th className="cw-check-cell">
                        <input
                          type="checkbox"
                          aria-label="Select all visible records"
                          checked={
                            rows.length > 0 &&
                            rows.every((item) => selection.includes(item.index))
                          }
                          onChange={(e) =>
                            setSelection(
                              e.target.checked
                                ? rows.map((item) => item.index)
                                : [],
                            )
                          }
                        />
                      </th>
                      {view === "contacts" && showEmail && (
                        <th className="cw-cell-email">Email</th>
                      )}
                      <th className="cw-cell-name">Name</th>
                      {view === "contacts" ? (
                        <>
                          <th className="cw-cell-stage">Stage</th>
                          <th className="cw-cell-status">Status</th>
                        </>
                      ) : (
                        <>
                          <th className="cw-cell-domain">Domain</th>
                          <th className="cw-cell-industry">Industry</th>
                          <th className="cw-cell-employees">Employees</th>
                          <th className="cw-cell-revenue">Revenue</th>
                        </>
                      )}
                      <th className="cw-cell-owner">Owner</th>
                      {showCreated && (
                        <th className="cw-cell-created">Created</th>
                      )}
                      <th className="cw-actions-cell">
                        <span className="cw-sr-only">Actions</span>
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {[
                      ...(groupBy === "owner"
                        ? rows.sort((a, b) => a.owner.localeCompare(b.owner))
                        : rows),
                    ].map((item, rowIndex, list) => (
                      <Fragment key={item.index}>
                        {groupBy === "owner" &&
                          (rowIndex === 0 ||
                            list[rowIndex - 1].owner !== item.owner) && (
                            <tr className="cw-table-group">
                              <th colSpan={12}>
                                <ChevronDown size={12} />
                                {item.owner}
                                <small>
                                  {
                                    list.filter(
                                      (row) => row.owner === item.owner,
                                    ).length
                                  }
                                </small>
                              </th>
                            </tr>
                          )}
                        <tr data-selected={selection.includes(item.index)}>
                          <td className="cw-check-cell">
                            <input
                              type="checkbox"
                              checked={selection.includes(item.index)}
                              aria-label={`Select ${item.name}`}
                              onChange={(e) =>
                                setSelection((current) =>
                                  e.target.checked
                                    ? [...current, item.index]
                                    : current.filter(
                                        (index) => index !== item.index,
                                      ),
                                )
                              }
                            />
                          </td>
                          {view === "contacts" && showEmail && (
                            <td className="cw-cell-email">
                              <button
                                type="button"
                                onClick={() => openContact(item.index)}
                              >
                                {CONTACTS[item.index].email}
                              </button>
                            </td>
                          )}
                          <td className="cw-cell-name">
                            <button
                              type="button"
                              onClick={() => openContact(item.index)}
                            >
                              {view === "contacts" ? (
                                <span
                                  className="cw-initials"
                                  data-tone={item.index % 3}
                                >
                                  {CONTACTS[item.index].initials}
                                </span>
                              ) : (
                                <span
                                  className="cw-company-initials"
                                  data-tone={item.index % 3}
                                >
                                  {COMPANIES[item.index].initials}
                                </span>
                              )}
                              <span>{item.name}</span>
                            </button>
                          </td>
                          {view === "contacts" ? (
                            <>
                              <td className="cw-cell-stage">
                                <span
                                  className="cw-stage"
                                  data-stage={CONTACTS[item.index].stage}
                                >
                                  {CONTACTS[item.index].stage}
                                </span>
                              </td>
                              <td className="cw-cell-status">
                                <span>{CONTACTS[item.index].status}</span>
                              </td>
                            </>
                          ) : (
                            <>
                              <td className="cw-cell-domain">
                                {COMPANIES[item.index].domain}
                              </td>
                              <td className="cw-cell-industry">
                                {COMPANIES[item.index].industry}
                              </td>
                              <td className="cw-cell-employees">
                                {COMPANIES[item.index].employees}
                              </td>
                              <td className="cw-cell-revenue">
                                {COMPANIES[item.index].revenue}
                              </td>
                            </>
                          )}
                          <td className="cw-cell-owner">
                            <span className="cw-table-owner">
                              <CRMAvatar
                                person={
                                  item.owner === "Sam Rivera" ? "sam" : "aisha"
                                }
                                size={24}
                              />
                              <span>{item.owner}</span>
                            </span>
                          </td>
                          {showCreated && (
                            <td className="cw-cell-created">
                              Sep {18 - item.index}, 2026
                            </td>
                          )}
                          <td className="cw-actions-cell">
                            <CRMIcon name="MoreVerticalIcon" size={14} />
                          </td>
                        </tr>
                      </Fragment>
                    ))}
                  </tbody>
                </table>
                {!rows.length && (
                  <div className="cw-no-results">
                    <Search size={23} />
                    <strong>No records match your filters.</strong>
                    <button
                      type="button"
                      onClick={() => {
                        setSearch("");
                        setOwner("all");
                        searchRef.current?.focus();
                      }}
                    >
                      Clear filters
                    </button>
                  </div>
                )}
              </div>
            </>
          ) : (
            <RecordDetail
              contactIndex={selected}
              entity={view}
              tab={tab}
              onTab={(value) => {
                setPaused(true);
                setTab(value);
                setManualAgent(true);
                setAgent(false);
              }}
              onBack={() => {
                changeView(view);
                searchRef.current?.focus();
              }}
            />
          )}
        </div>
      </div>
      {mode === "account" && (
        <div
          className="cw-agent-layer"
          data-open={showAgent}
          id={`${id}-agent`}
          hidden={!showAgent}
        >
          <AccountAgent active={active} phase={phase} />
        </div>
      )}
      <div className="cw-playback">
        <span>
          <Check size={12} />
          {selected === null
            ? `${rows.length} ${view} · OrbitDesk`
            : "Customer context stays attached"}
        </span>
        <div>
          {mode === "directory" && (
            <div
              className="cw-preview-tabs"
              role="group"
              aria-label="CRM record type"
            >
              {(["contacts", "companies"] as const).map((value) => (
                <button
                  type="button"
                  key={value}
                  aria-pressed={view === value}
                  onClick={() => changeView(value)}
                >
                  {value === "contacts" ? "Contacts" : "Companies"}
                </button>
              ))}
            </div>
          )}
          {mode === "account" && (
            <button
              type="button"
              aria-expanded={showAgent}
              aria-controls={`${id}-agent`}
              onClick={() => {
                setPaused(true);
                setManualAgent(true);
                setSelected(0);
                setManualView("contacts");
                setTab("Overview");
                setAgent(!showAgent);
              }}
            >
              <Sparkles size={13} />
              {showAgent ? "Close Ask Agent" : "Ask Agent"}
            </button>
          )}
          <button
            type="button"
            aria-label={`${paused ? "Play" : "Pause"} ${mode === "directory" ? "CRM directory" : "account context"} animation`}
            aria-pressed={paused}
            onClick={() => {
              setManualAgent(false);
              setManualView(null);
              setSearch("");
              setOwner("all");
              setSelected(mode === "account" ? 0 : null);
              setTab("Overview");
              setPaused(!paused);
            }}
          >
            {paused ? <Play size={12} /> : <Pause size={12} />}
          </button>
        </div>
      </div>
    </div>
  );
}
