import { Collapsible } from 'radix-ui';
import { ChevronRight } from 'lucide-react';
import {
  SidebarGroup,
  SidebarGroupLabel,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from '@/components/ui/sidebar';
import { COLLAPSIBLE_SETTINGS_GROUPS } from './state';
import type { NavGroup } from './types';

type SettingsRailNavProps = {
  groups: NavGroup[];
  isActive: (link: string) => boolean;
  collapsedGroups: Set<string>;
  toggleGroup: (groupLabel: string) => void;
  onNavigate: (link: string) => void;
};

export function SettingsRailNav({
  groups,
  isActive,
  collapsedGroups,
  toggleGroup,
  onNavigate,
}: SettingsRailNavProps) {
  return (
    <>
      {groups.map((group, index) => {
        const isCollapsible = COLLAPSIBLE_SETTINGS_GROUPS.has(group.label);
        const isOpen = !collapsedGroups.has(group.label);

        const menuItems = (
          <SidebarMenu>
            {group.items.map((item) => (
              <SidebarMenuItem key={item.link}>
                <SidebarMenuButton
                  asChild
                  tooltip={item.label}
                  isActive={isActive(item.link)}
                  className="h-8 rounded-md px-2 text-[13px]"
                >
                  <a
                    href={item.link}
                    onClick={(event) => {
                      event.preventDefault();
                      onNavigate(item.link);
                    }}
                  >
                    <item.icon />
                    <span>{item.label}</span>
                  </a>
                </SidebarMenuButton>
              </SidebarMenuItem>
            ))}
          </SidebarMenu>
        );

        if (isCollapsible) {
          return (
            <Collapsible.Root
              key={group.label}
              open={isOpen}
              onOpenChange={() => toggleGroup(group.label)}
              asChild
            >
              <SidebarGroup className="p-0 pb-3">
                <Collapsible.Trigger asChild>
                  <SidebarGroupLabel className="h-7 cursor-pointer select-none px-2 text-[11px] uppercase tracking-wide text-muted-foreground/90 hover:text-muted-foreground">
                    <span className="flex-1">{group.label}</span>
                    <ChevronRight className={`h-3 w-3 shrink-0 transition-transform duration-200 ${isOpen ? 'rotate-90' : ''}`} />
                  </SidebarGroupLabel>
                </Collapsible.Trigger>
                <Collapsible.Content>
                  {menuItems}
                </Collapsible.Content>
              </SidebarGroup>
            </Collapsible.Root>
          );
        }

        return (
          <SidebarGroup key={group.label || index} className="p-0 pb-3">
            {group.label && (
              <SidebarGroupLabel className="h-7 px-2 text-[11px] uppercase tracking-wide text-muted-foreground/90">
                {group.label}
              </SidebarGroupLabel>
            )}
            {menuItems}
          </SidebarGroup>
        );
      })}
    </>
  );
}
