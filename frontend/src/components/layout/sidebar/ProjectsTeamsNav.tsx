import { Collapsible } from 'radix-ui';
import { TeamBadge } from '@/components/workspace/TeamBadge';
import { ArrowRight01Icon, MoreVerticalIcon, ArrowReloadHorizontalIcon, Setting06Icon, PlusSignIcon } from '@/lib/icons';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import {
  SidebarGroup,
  SidebarGroupLabel,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarMenuSub,
  SidebarMenuSubButton,
  SidebarMenuSubItem,
} from '@/components/ui/sidebar';
import { teamSubItems } from './config';
import { SidebarSectionAction } from './SidebarSectionAction';

type Team = {
  id: string;
  name: string;
  color?: string | null;
  sprints_enabled?: boolean;
};

type ProjectsTeamsNavProps = {
  wsSlug: string;
  teams: Team[];
  expandedTeams: Set<string>;
  isTeamSubActive: (teamId: string, subPath: string) => boolean;
  toggleTeam: (teamId: string) => void;
  onNavigate: (args: { to: string; params?: Record<string, string>; search?: Record<string, string> }) => void;
  canManageTeams?: boolean;
};

export function ProjectsTeamsNav({
  wsSlug,
  teams,
  expandedTeams,
  isTeamSubActive,
  toggleTeam,
  onNavigate,
  canManageTeams = false,
}: ProjectsTeamsNavProps) {
  return (
    <SidebarGroup className="p-0 pb-3">
      <SidebarGroupLabel className="h-7 px-2 text-[11px] uppercase tracking-wide text-muted-foreground/90 flex items-center justify-between">
        <span>Your Teams</span>
        {canManageTeams && (
          <SidebarSectionAction
            label="Create team"
            onClick={() =>
              onNavigate({
                to: '/w/$slug/settings/$section',
                params: { slug: wsSlug, section: 'teams' },
              })
            }
          />
        )}
      </SidebarGroupLabel>
      <SidebarMenu>
        {teams.length === 0 && (
          <SidebarMenuItem>
            <SidebarMenuButton
              className="h-8 rounded-md px-2 text-muted-foreground"
              onClick={() =>
                onNavigate({
                  to: '/w/$slug/settings/$section',
                  params: { slug: wsSlug, section: 'teams' },
                })
              }
            >
              <PlusSignIcon className="h-3.5 w-3.5" />
              <span>Create team</span>
            </SidebarMenuButton>
          </SidebarMenuItem>
        )}
        {teams.map((team) => {
          const isExpanded = expandedTeams.has(team.id);

          return (
            <Collapsible.Root
              key={team.id}
              asChild
              open={isExpanded}
              onOpenChange={() => toggleTeam(team.id)}
            >
              <SidebarMenuItem>
                <div className="group/team relative flex items-center">
                  <Collapsible.Trigger asChild>
                    <SidebarMenuButton className="h-8 flex-1 rounded-md px-2 cursor-pointer">
                      <TeamBadge name={team.name} color={team.color} className="h-[20px] w-[20px] rounded-[4px] text-xs" />
                      <span className="min-w-0 flex-1 truncate pr-5">{team.name}</span>
                      <span data-team-chevron aria-hidden="true" className="-mr-0.5 flex h-5 w-5 shrink-0 items-center justify-center">
                        <ArrowRight01Icon className={`h-3.5 w-3.5 text-muted-foreground transition-transform ${isExpanded ? 'rotate-90' : ''}`} />
                      </span>
                    </SidebarMenuButton>
                  </Collapsible.Trigger>
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <button
                        type="button"
                        aria-label={`Team actions for ${team.name}`}
                        className="absolute right-7 flex h-5 w-5 items-center justify-center rounded opacity-0 transition-opacity hover:bg-muted group-hover/team:opacity-100 focus-visible:opacity-100 data-[state=open]:opacity-100"
                      >
                        <MoreVerticalIcon className="h-3.5 w-3.5 text-muted-foreground" />
                      </button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent side="right" align="start">
                      <DropdownMenuItem
                        onClick={() =>
                          onNavigate({
                            to: '/w/$slug/settings/$section',
                            params: { slug: wsSlug, section: 'teams' },
                          })
                        }
                      >
                        <Setting06Icon className="h-4 w-4" />
                        Settings
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                </div>
                <Collapsible.Content>
                  <SidebarMenuSub>
                    {teamSubItems
                      .filter((sub) => {
                        if (sub.key !== 'sprints') return true;
                        // Sprints enabled → show normally
                        if (team.sprints_enabled !== false) return true;
                        // Sprints disabled + can manage → show as nudge
                        if (canManageTeams) return true;
                        // Sprints disabled + no permission → hide
                        return false;
                      })
                      .map((sub) => {
                        const sprintsDisabled = sub.key === 'sprints' && team.sprints_enabled === false;
                        const link = `/w/${wsSlug}/pm/${sub.path}?team=${team.id}`;
                        const active = !sprintsDisabled && isTeamSubActive(team.id, sub.path);

                        if (sprintsDisabled) {
                          return (
                            <SidebarMenuSubItem key={sub.key}>
                              <Tooltip>
                                <TooltipTrigger asChild>
                                  <SidebarMenuSubButton
                                    asChild
                                    size="sm"
                                    className="opacity-40"
                                  >
                                    <a
                                      href={`/w/${wsSlug}/settings/teams?team=${team.id}&section=sprints`}
                                      onClick={(event) => {
                                        event.preventDefault();
                                        onNavigate({
                                          to: '/w/$slug/settings/teams',
                                          params: { slug: wsSlug },
                                          search: { team: team.id, section: 'sprints' },
                                        });
                                      }}
                                    >
                                      <ArrowReloadHorizontalIcon className="h-3.5 w-3.5" />
                                      <span>Sprints</span>
                                    </a>
                                  </SidebarMenuSubButton>
                                </TooltipTrigger>
                                <TooltipContent side="right" className="text-xs">
                                  Sprints disabled — click to enable in settings
                                </TooltipContent>
                              </Tooltip>
                            </SidebarMenuSubItem>
                          );
                        }

                        return (
                          <SidebarMenuSubItem key={sub.key}>
                            <SidebarMenuSubButton
                              asChild
                              size="sm"
                              isActive={active}
                            >
                              <a
                                href={link}
                                onClick={(event) => {
                                  event.preventDefault();
                                  onNavigate({
                                    to: `/w/$slug/pm/${sub.path}`,
                                    params: { slug: wsSlug },
                                    search: { team: team.id },
                                  });
                                }}
                              >
                                <sub.icon className="h-3.5 w-3.5" />
                                <span>{sub.label}</span>
                              </a>
                            </SidebarMenuSubButton>
                          </SidebarMenuSubItem>
                        );
                      })}
                  </SidebarMenuSub>
                </Collapsible.Content>
              </SidebarMenuItem>
            </Collapsible.Root>
          );
        })}
      </SidebarMenu>
    </SidebarGroup>
  );
}
