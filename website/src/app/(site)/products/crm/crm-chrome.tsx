"use client";

import { CRMIcon } from "./crm-icons";

export type CRMView = "contacts" | "companies" | "deals";
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
export function CRMNavigation({
  view,
  onView,
  dealsEnabled = false,
}: {
  view: CRMView | "playbooks";
  onView?: (view: CRMView) => void;
  dealsEnabled?: boolean;
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
    ["InboxIcon", "Emails"],
    ["Camera01Icon", "Meetings"],
    ["BookOpen01Icon", "Playbooks"],
    ["BulbIcon", "Signals"],
  ] as const;
  return (
    <aside className="cw-nav preview-sidebar">
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
            <span className="cw-rail-control" title="Switch to dark mode">
              <CRMIcon name="Sun01Icon" size={14} />
            </span>
            <span className="cw-rail-control" title="Ask Agents">
              <img
                src="/new/crm/ask-agent-mark.svg"
                width={24}
                height={24}
                alt=""
              />
            </span>
            <span className="cw-account-control" title="Sam Rivera">
              <CRMAvatar size={32} />
              <i />
            </span>
          </div>
        </div>
        <div className="cw-module">
          <div className="cw-crm">
            <span>
              <CRMIcon name="ChartColumnIcon" />
              Overview
            </span>
            {(["contacts", "companies", "deals"] as const).map((item) => (
              <button
                type="button"
                key={item}
                aria-pressed={view === item}
                onClick={() => onView?.(item)}
                disabled={!onView || (item === "deals" && !dealsEnabled)}
              >
                <CRMIcon
                  name={
                    item === "contacts" ? "UserGroupIcon" : item === "companies" ? "Building03Icon" : "DollarCircleIcon"
                  }
                />
                <span>{item === "contacts" ? "Contacts" : item === "companies" ? "Companies" : "Deals"}</span>
              </button>
            ))}
            {links.map(([name, label]) => (
              <span
                key={label}
                data-active={
                  (view === "playbooks" && label === "Playbooks")
                }
                className={
                  label === "Playbooks" ? "cw-nav-separator" : undefined
                }
              >
                <CRMIcon name={name} />
                {label}
              </span>
            ))}
            <div className="cw-module-footer" aria-hidden="true">
              <span title="Pipelines">
                <CRMIcon name="WorkflowSquare01Icon" />
              </span>
              <span title="Email Accounts">
                <CRMIcon name="Mail01Icon" />
              </span>
              <span title="Autonomy">
                <CRMIcon name="SparklesIcon" />
              </span>
            </div>
          </div>
          <div className="cw-sidebar-search" aria-hidden="true">
            <CRMIcon name="Search01Icon" size={15} />
            <span>Search OrbitDesk</span>
            <small>⌘K</small>
          </div>
        </div>
      </div>
    </aside>
  );
}
