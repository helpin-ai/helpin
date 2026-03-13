import { Fragment, useEffect, useMemo, useState } from "react";
import { useLocation, useNavigate } from "@tanstack/react-router";
import {
  Search,
} from "lucide-react";
import { SidebarTrigger } from "@/components/ui/sidebar";
import { useWorkspaceStore } from "@/stores/workspaceStore";
import { SearchCommandPalette } from "@/components/search/SearchCommandPalette";

type Crumb = {
  label: string;
  to?: string;
};

export function Header() {
  const navigate = useNavigate();
  const location = useLocation();
  const { currentWorkspace } = useWorkspaceStore();
  const [searchOpen, setSearchOpen] = useState(false);

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

  const breadcrumbs = useMemo<Crumb[]>(() => {
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
      bonus: "Bonus Dashboard",
      "my-quarter": "My Quarter",
      settings: "Settings",
      tasks: "Tasks",
      pm: "Projects",
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
      stories: "Stories",
      epics: "Epics",
      sprints: "Sprints",
      objectives: "Objectives",
      roadmap: "Roadmap",
      reports: "Reports",
    };

    const settingsSubMap: Record<string, string> = {
      system: "General",
      members: "Members",
      teams: "Teams",
      people: "People",
      jobroles: "Job Roles",
      workflows: "Workflows",
      ai: "AI",
      tiers: "Bonus Tiers",
      import: "Import / Export",
    };

    if (section === "pm") {
      crumbs.push({ label: "Projects", to: `/w/${slug}/pm/my-work` });
      if (subRoute[1]) {
        const pmSub = subRoute[1];
        const pmLabel = pmSubMap[pmSub] ?? pmSub.replace(/-/g, " ");
        if (subRoute[2]) {
          crumbs.push({ label: pmLabel, to: `/w/${slug}/pm/${pmSub}` });
          crumbs.push({ label: `${pmLabel.replace(/s$/, "")} Detail` });
        } else {
          crumbs.push({ label: pmLabel });
        }
      }
      return crumbs;
    }

    if (section === "crm") {
      crumbs.push({ label: "CRM", to: `/w/${slug}/crm/contacts` });
      if (subRoute[1]) {
        const crmSub = subRoute[1];
        const crmLabel = crmSubMap[crmSub] ?? crmSub.replace(/-/g, " ");
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
      crumbs.push({ label: "Settings", to: `/w/${slug}/settings/system` });
      if (subRoute[1]) {
        const settingsSub = subRoute[1];
        crumbs.push({
          label: settingsSubMap[settingsSub] ?? settingsSub.replace(/-/g, " "),
        });
      }
      return crumbs;
    }

    crumbs.push({ label: sectionMap[section] ?? section.replace(/-/g, " ") });
    return crumbs;
  }, [location.pathname, currentWorkspace?.name]);

  return (
    <header className="relative h-14 border-b border-border/70 bg-background/95 px-3 flex items-center gap-3">
      <div className="flex min-w-0 max-w-[45%] items-center gap-2 z-10">
        <SidebarTrigger className="-ml-1" />
        {breadcrumbs.length > 0 && (
          <nav
            aria-label="Breadcrumb"
            className="hidden min-w-0 items-center gap-1 text-sm md:flex overflow-hidden"
          >
            {breadcrumbs.map((crumb, index) => {
              const isLast = index === breadcrumbs.length - 1;
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
                      {crumb.label}
                    </span>
                  )}
                </Fragment>
              );
            })}
          </nav>
        )}
      </div>

      <div className="hidden lg:flex absolute inset-0 justify-center items-center pointer-events-none">
        <button
          type="button"
          onClick={() => setSearchOpen(true)}
          className="pointer-events-auto relative flex h-8 w-full max-w-xl items-center gap-2 rounded-md border border-border/70 bg-muted/40 px-2.5 text-sm text-muted-foreground transition-colors hover:bg-muted/60 hover:text-foreground cursor-pointer"
        >
          <Search className="h-4 w-4 shrink-0" />
          <span className="truncate">Search {currentWorkspace?.name ?? "workspace"}...</span>
          <kbd className="ml-auto hidden rounded border bg-muted px-1.5 py-0.5 text-[10px] font-mono text-muted-foreground md:inline-block">
            ⌘K
          </kbd>
        </button>
      </div>

      <SearchCommandPalette open={searchOpen} onOpenChange={setSearchOpen} />
    </header>
  );
}
