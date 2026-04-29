import { Fragment, useEffect, useMemo, useState } from "react";
import { useLocation, useNavigate } from "@tanstack/react-router";
import {
  PlusSignIcon,
  Search01Icon,
} from "@/lib/icons";
import { Button } from "@/components/ui/button";
import { SidebarTrigger } from "@/components/ui/sidebar";
import { useWorkspaceStore } from "@/stores/workspaceStore";
import { useGlobalCreateStore } from "@/stores/globalCreateStore";
import { SearchCommandPalette } from "@/components/search/SearchCommandPalette";
import { useSupportInboxStore } from "@/stores/supportInboxStore";
import { usePageHeaderStore } from "@/stores/pageHeaderStore";
import { buildSettingsRoutePath, SETTINGS_SECTION_LABELS } from "@/lib/settingsSections";

type Crumb = {
  label: string;
  to?: string;
};

export function Header() {
  const navigate = useNavigate();
  const location = useLocation();
  const { currentWorkspace } = useWorkspaceStore();
  const [searchOpen, setSearchOpen] = useState(false);
  const setCreateDialogOpen = useSupportInboxStore((s) => s.setCreateDialogOpen);
  const openGlobalCreate = useGlobalCreateStore((s) => s.openCreate);
  const navFilter = useSupportInboxStore((s) => s.navFilter);
  const titleOverride = usePageHeaderStore((s) => s.titleOverride);
  const headerActions = usePageHeaderStore((s) => s.actions);
  const isSupport = location.pathname.includes("/support");
  const isContactsIndex = /\/crm\/contacts\/?$/.test(location.pathname);

  // Cmd+K / Ctrl+K shortcut
  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === "k" && (e.metaKey || e.ctrlKey)) {
        e.preventDefault();
        setSearchOpen(true);
      }
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, []);

  const handleSearchOpenChange = (open: boolean) => {
    setSearchOpen(open);
  };

  const breadcrumbs = useMemo<Crumb[]>(() => {
    const formatLabel = (value: string) =>
      value
        .split('-')
        .filter(Boolean)
        .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
        .join(' ');

    const segments = location.pathname.split("/").filter(Boolean);
    if (segments.length < 2 || segments[0] !== "w") return [];

    const slug = segments[1];
    const subRoute = segments.slice(2);

    const crumbs: Crumb[] = [];
    if (subRoute.length === 0) {
      crumbs.push({ label: "Dashboard" });
      return crumbs;
    }

    const section = subRoute[0];
    const sectionMap: Record<string, string> = {
      dashboard: "Dashboard",
      goals: "Company Goals",
      "team-goals": "Team Goals",
      sprints: "Sprints",
      "my-quarter": "My Quarter",
      settings: "Settings",
      tasks: "Tasks",
      pm: "Projects",
      automation: "Automation",
      support: "Support",
      docs: "Docs",
      crm: "CRM",
    };

    if (section === "sprints" && subRoute[1]) {
      crumbs.push({ label: "Sprints", to: `/w/${slug}/sprints` });
      crumbs.push({ label: "Sprint Detail" });
      return crumbs;
    }

    const crmSubMap: Record<string, string> = {
      contacts: "Contacts",
      companies: "Companies",
      deals: "Deals",
      insights: "Insights",
    };

    const pmSubMap: Record<string, string> = {
      "my-work": "My Work",
      tasks: "Tasks",
      epics: "Epics",
      sprints: "Sprints",
      objectives: "Objectives",
      roadmap: "Roadmap",
      reports: "Reports",
    };

    const automationSubMap: Record<string, string> = {
      flows: "Flows",
      activity: "Activity",
      agents: "Agents",
      library: "Trigger Catalog",
      tools: "Tool Catalog",
      runs: "Runs",
    };

    if (section === "pm") {
      crumbs.push({ label: "Projects", to: `/w/${slug}/pm/my-work` });
      if (subRoute[1]) {
        const pmSub = subRoute[1];
        const pmLabel = pmSubMap[pmSub] ?? formatLabel(pmSub);
        if (subRoute[2]) {
          crumbs.push({ label: pmLabel, to: `/w/${slug}/pm/${pmSub}` });
          crumbs.push({ label: `${pmLabel.replace(/s$/, "")} Detail` });
        } else {
          crumbs.push({ label: pmLabel });
        }
      }
      return crumbs;
    }

    if (section === "automation") {
      crumbs.push({ label: "Automation", to: `/w/${slug}/automation/flows` });
      if (subRoute[1]) {
        const automationSub = subRoute[1];
        const automationLabel = automationSubMap[automationSub] ?? formatLabel(automationSub);
        if (subRoute[2]) {
          crumbs.push({ label: automationLabel, to: `/w/${slug}/automation/${automationSub}` });
          crumbs.push({ label: `${automationLabel.replace(/s$/, "")} Detail` });
        } else {
          crumbs.push({ label: automationLabel });
        }
      }
      return crumbs;
    }

    if (section === "crm") {
      crumbs.push({ label: "CRM", to: `/w/${slug}/crm/contacts` });
      if (subRoute[1]) {
        const crmSub = subRoute[1];
        const crmLabel = crmSubMap[crmSub] ?? formatLabel(crmSub);
        if (subRoute[2]) {
          crumbs.push({ label: crmLabel, to: `/w/${slug}/crm/${crmSub}` });
          crumbs.push({ label: `${crmLabel.replace(/s$/, "")} Detail` });
        } else {
          crumbs.push({ label: crmLabel });
        }
      }
      return crumbs;
    }

    if (section === "settings") {
      crumbs.push({ label: "Settings", to: buildSettingsRoutePath(slug, 'profile') });
      if (subRoute[1]) {
        const settingsSub = subRoute[1];
        crumbs.push({
          label: SETTINGS_SECTION_LABELS[settingsSub as keyof typeof SETTINGS_SECTION_LABELS] ?? formatLabel(settingsSub),
        });
      }
      return crumbs;
    }

    crumbs.push({ label: sectionMap[section] ?? formatLabel(section) });
    return crumbs;
  }, [location.pathname, currentWorkspace?.name]);

  return (
    <header className="relative flex h-14 items-center gap-3 bg-card/95 px-3 dark:bg-sidebar after:absolute after:right-3 after:bottom-0 after:left-3 after:h-px after:bg-border/70 after:[mask-image:linear-gradient(to_right,transparent,black_24px,black_calc(100%-24px),transparent)] dark:after:bg-sidebar-border">
      <div className="flex min-w-0 max-w-[45%] items-center gap-2 z-10">
        <SidebarTrigger className="-ml-1" />
        {breadcrumbs.length > 0 && (
          <nav
            aria-label="Breadcrumb"
            className="hidden min-w-0 items-center gap-1 text-sm md:flex overflow-hidden"
          >
            {breadcrumbs.map((crumb, index) => {
              const isLast = index === breadcrumbs.length - 1;
              const label = isLast && titleOverride ? titleOverride : crumb.label;
              return (
                <Fragment key={`${crumb.label}-${index}`}>
                  {index > 0 && (
                    <span className="text-muted-foreground">/</span>
                  )}
                  {crumb.to && !isLast ? (
                    <button
                      type="button"
                      className="max-w-[14rem] truncate text-muted-foreground hover:text-foreground transition-colors"
                      onClick={() => navigate({ to: crumb.to as string })}
                    >
                      {crumb.label}
                    </button>
                  ) : (
                    <span className="max-w-[14rem] truncate font-medium text-foreground">
                      {label}
                    </span>
                  )}
                </Fragment>
              );
            })}
          </nav>
        )}
      </div>

      {!(isSupport && navFilter === 'mentions') && (
        <div className="hidden lg:flex absolute inset-0 justify-center items-center pointer-events-none">
          <button
            type="button"
            onClick={() => setSearchOpen(true)}
            className="pointer-events-auto relative flex h-8 w-full max-w-xl items-center gap-2 rounded-md border border-border/70 bg-muted/40 px-2.5 text-sm text-muted-foreground transition-colors hover:bg-muted/60 hover:text-foreground cursor-pointer"
          >
            <Search01Icon className="h-4 w-4 shrink-0" />
            <span className="truncate">Search {currentWorkspace?.name ?? "workspace"}...</span>
            <kbd className="ml-auto hidden rounded border bg-muted px-1.5 py-0.5 text-[10px] font-mono text-muted-foreground md:inline-block">
              ⌘K
            </kbd>
          </button>
        </div>
      )}

      {headerActions ? (
        <div className="ml-auto flex items-center gap-1.5 z-10">
          {headerActions}
        </div>
      ) : (
        <>
          {isSupport && navFilter !== 'mentions' && (
            <div className="ml-auto flex items-center z-10">
              <Button size="sm" onClick={() => setCreateDialogOpen(true)}>
                <PlusSignIcon className="mr-1.5 h-4 w-4" />
                <span className="hidden sm:inline">New Conversation</span>
              </Button>
            </div>
          )}

          {!isSupport && isContactsIndex && (
            <div className="ml-auto flex items-center z-10">
              <Button size="sm" onClick={() => openGlobalCreate('crm_contact')}>
                <PlusSignIcon className="mr-1.5 h-4 w-4" />
                <span className="hidden sm:inline">Contact</span>
              </Button>
            </div>
          )}
        </>
      )}

      <SearchCommandPalette open={searchOpen} onOpenChange={handleSearchOpenChange} />
    </header>
  );
}
