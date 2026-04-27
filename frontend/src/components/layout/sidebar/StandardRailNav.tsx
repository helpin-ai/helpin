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
                  className="h-8 rounded-md px-2 text-sm"
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
                    {!!item.badge && (
                      <span className="ml-auto flex h-4 min-w-4 items-center justify-center rounded-full border border-amber-500/30 bg-amber-500/10 px-1 text-[10px] font-semibold leading-none text-amber-700 dark:text-amber-300">
                        {item.badge > 99 ? '99+' : item.badge}
                      </span>
                    )}
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
