import {
  SidebarGroup,
  SidebarGroupLabel,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from '@/components/ui/sidebar';
import type { NavGroup } from './types';

type StandardRailNavProps = {
  groups: NavGroup[];
  isActive: (link: string) => boolean;
  onNavigate: (link: string) => void;
};

export function StandardRailNav({ groups, isActive, onNavigate }: StandardRailNavProps) {
  return (
    <>
      {groups.map((group, index) => (
        <SidebarGroup key={group.label || index} className="p-0 pb-3">
          {group.label && (
            <SidebarGroupLabel className="h-7 px-2 text-[11px] uppercase tracking-wide text-muted-foreground/90">
              {group.label}
            </SidebarGroupLabel>
          )}
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
        </SidebarGroup>
      ))}
    </>
  );
}
