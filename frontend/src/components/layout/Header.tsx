import { Fragment, useEffect, useMemo, useState } from "react";
import { useLocation, useNavigate } from "@tanstack/react-router";
import {
  Bell,
  CircleHelp,
  LogOut,
  Moon,
  Search,
  Sun,
  User,
  Users,
} from "lucide-react";
import { useTheme } from "next-themes";
import { SidebarTrigger } from "@/components/ui/sidebar";
import { getInitials } from "@/lib/utils";
import { QuarterSelector } from "@/components/quarter/QuarterSelector";
import { useWorkspaceStore } from "@/stores/workspaceStore";
import { useAuthStore } from "@/stores/authStore";
import { Button } from "@/components/ui/button";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { SearchCommandPalette } from "@/components/search/SearchCommandPalette";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

type Crumb = {
  label: string;
  to?: string;
};

export function Header() {
  const navigate = useNavigate();
  const location = useLocation();
  const { currentWorkspace } = useWorkspaceStore();
  const { user, signOut } = useAuthStore();
  const { theme, setTheme } = useTheme();

  const initials = getInitials(user?.full_name || user?.email);
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
    const workspaceLabel = currentWorkspace?.name ?? "Workspace";

    const crumbs: Crumb[] = [
      { label: workspaceLabel, to: `/w/${slug}/pm/stories` },
    ];
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
    };

    if (section === "sprints" && subRoute[1]) {
      crumbs.push({ label: "Sprints", to: `/w/${slug}/sprints` });
      crumbs.push({ label: "Sprint Detail" });
      return crumbs;
    }

    const pmSubMap: Record<string, string> = {
      stories: "Work Items",
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
      workflowstates: "Workflow States",
      tiers: "Bonus Tiers",
      import: "Import / Export",
    };

    if (section === "pm") {
      crumbs.push({ label: "Projects", to: `/w/${slug}/pm/stories` });
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
    <header className="h-14 border-b border-border/70 bg-background/95 px-3 flex items-center gap-3">
      <div className="flex min-w-0 items-center gap-2">
        <SidebarTrigger className="-ml-1" />
        {breadcrumbs.length > 0 && (
          <nav
            aria-label="Breadcrumb"
            className="hidden min-w-0 items-center gap-1 text-sm md:flex"
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

      <div className="hidden lg:flex flex-1 max-w-xl items-center">
        <button
          type="button"
          onClick={() => setSearchOpen(true)}
          className="relative flex h-8 w-3/4 items-center gap-2 rounded-md border border-border/70 bg-muted/40 px-2.5 text-sm text-muted-foreground transition-colors hover:bg-muted/60 hover:text-foreground cursor-pointer"
        >
          <Search className="h-4 w-4 shrink-0" />
          <span className="truncate">Search {currentWorkspace?.name ?? "workspace"}...</span>
          <kbd className="ml-auto hidden rounded border bg-muted px-1.5 py-0.5 text-[10px] font-mono text-muted-foreground md:inline-block">
            ⌘K
          </kbd>
        </button>
      </div>

      <SearchCommandPalette open={searchOpen} onOpenChange={setSearchOpen} />

      <div className="ml-auto flex items-center gap-1">
        <Button
          variant="ghost"
          size="icon"
          className="size-8 text-muted-foreground"
          onClick={() => setTheme(theme === "dark" ? "light" : "dark")}
          title={theme === "dark" ? "Switch to light mode" : "Switch to dark mode"}
        >
          <Sun className="h-4 w-4 rotate-0 scale-100 transition-transform dark:rotate-90 dark:scale-0" />
          <Moon className="absolute h-4 w-4 rotate-90 scale-0 transition-transform dark:rotate-0 dark:scale-100" />
          <span className="sr-only">Toggle theme</span>
        </Button>
        <Button
          variant="ghost"
          size="icon"
          className="size-8 text-muted-foreground"
        >
          <Bell className="h-4 w-4" />
        </Button>
        <Button
          variant="ghost"
          size="icon"
          className="size-8 text-muted-foreground"
        >
          <CircleHelp className="h-4 w-4" />
        </Button>
        <QuarterSelector />
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              variant="ghost"
              size="icon"
              className="ml-1 size-8 rounded-full border border-border/70 p-0"
            >
              <Avatar className="size-7">
                <AvatarFallback className="text-[11px]">
                  {initials}
                </AvatarFallback>
              </Avatar>
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-56">
            <DropdownMenuLabel className="truncate">
              {user?.full_name || user?.email || "Account"}
            </DropdownMenuLabel>
            <DropdownMenuSeparator />
            <DropdownMenuItem onClick={() => navigate({ to: '/w/$slug/settings/$section', params: { slug: currentWorkspace?.slug ?? '', section: 'profile' } })}>
              <User className="h-4 w-4" />
              <span>Profile</span>
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => navigate({ to: "/workspaces" })}>
              <Users className="h-4 w-4" />
              <span>All Workspaces</span>
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem onClick={signOut} variant="destructive">
              <LogOut className="h-4 w-4" />
              <span>Sign out</span>
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </header>
  );
}
