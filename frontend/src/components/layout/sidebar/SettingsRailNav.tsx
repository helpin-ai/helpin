import { useEffect, useRef, useState } from 'react';
import { Collapsible } from 'radix-ui';
import { buildSettingsHomePath } from '@/lib/settingsDiscovery';
import { SETTINGS_HOME_LABEL } from '@/lib/settingsSections';
import { Search01Icon, ArrowRight01Icon } from '@/lib/icons';
import {
  SidebarGroup,
  SidebarGroupLabel,
  SidebarMenu,
  SidebarMenuAction,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarMenuSub,
  SidebarMenuSubButton,
  SidebarMenuSubItem,
} from '@/components/ui/sidebar';
import { cn } from '@/lib/utils';
import { COLLAPSIBLE_SETTINGS_GROUPS } from './state';
import type { NavGroup } from './types';

type SettingsRailNavProps = {
  workspaceSlug: string;
  groups: NavGroup[];
  isActive: (link: string) => boolean;
  collapsedGroups: Set<string>;
  toggleGroup: (groupLabel: string) => void;
  onNavigate: (link: string) => void;
};

export function SettingsRailNav({
  workspaceSlug,
  groups,
  isActive,
  collapsedGroups,
  toggleGroup,
  onNavigate,
}: SettingsRailNavProps) {
  const initialCollapsibleItemLinks = groups.flatMap((group) =>
    group.items.filter((item) => item.children?.length).map((item) => item.link),
  );
  const knownCollapsibleItemLinks = useRef(new Set(initialCollapsibleItemLinks));
  const [collapsedItems, setCollapsedItems] = useState<Set<string>>(
    () => new Set(initialCollapsibleItemLinks),
  );

  useEffect(() => {
    const collapsibleItemLinks = groups.flatMap((group) =>
      group.items.filter((item) => item.children?.length).map((item) => item.link),
    );
    const activeParentLinks = groups.flatMap((group) =>
      group.items
        .filter((item) => item.children?.some((child) => isActive(child.link)))
        .map((item) => item.link),
    );
    const activeParentLinkSet = new Set(activeParentLinks);
    const newParentLinks = collapsibleItemLinks.filter(
      (link) => !knownCollapsibleItemLinks.current.has(link),
    );
    knownCollapsibleItemLinks.current = new Set(collapsibleItemLinks);

    setCollapsedItems((previous) => {
      const next = new Set(previous);
      newParentLinks.forEach((link) => {
        if (!activeParentLinkSet.has(link)) next.add(link);
      });
      activeParentLinks.forEach((link) => next.delete(link));

      if (
        next.size === previous.size
        && [...next].every((link) => previous.has(link))
      ) {
        return previous;
      }
      return next;
    });
  }, [groups, isActive]);

  return (
    <>
      <div className="sticky top-0 z-10 space-y-1 border-b border-border/50 bg-sidebar pb-3 mb-3">
        <SidebarMenu>
          <SidebarMenuItem><SidebarMenuButton asChild isActive={isActive(buildSettingsHomePath(workspaceSlug))}><a href={buildSettingsHomePath(workspaceSlug)} onClick={event => { event.preventDefault(); onNavigate(buildSettingsHomePath(workspaceSlug)); }}>{SETTINGS_HOME_LABEL}</a></SidebarMenuButton></SidebarMenuItem>
          <SidebarMenuItem><SidebarMenuButton asChild><a href={`${buildSettingsHomePath(workspaceSlug)}?search=1`} onClick={event => { event.preventDefault();
            const search = document.querySelector<HTMLInputElement>('input[data-settings-search]');
            if (search) search.focus(); else onNavigate(`${buildSettingsHomePath(workspaceSlug)}?search=1`); }}><Search01Icon /><span>Search settings</span></a></SidebarMenuButton></SidebarMenuItem>
        </SidebarMenu>
      </div>
      {groups.map((group, index) => {
        const isCollapsible = COLLAPSIBLE_SETTINGS_GROUPS.has(group.label);
        const isOpen = !collapsedGroups.has(group.label);

        const menuItems = (
          <SidebarMenu>
            {group.items.map((item) => {
              const hasActiveChild = item.children?.some((child) => isActive(child.link)) ?? false;
              const hasChildren = Boolean(item.children?.length);
              const areChildrenOpen = !collapsedItems.has(item.link);

              return (
                <SidebarMenuItem key={item.link}>
                  <SidebarMenuButton
                    asChild
                    tooltip={item.label}
                    isActive={isActive(item.link) && !hasActiveChild}
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
                    </a>
                  </SidebarMenuButton>

                  {hasChildren && (
                    <SidebarMenuAction
                      type="button"
                      aria-label={`${areChildrenOpen ? 'Collapse' : 'Expand'} ${item.label}`}
                      aria-expanded={areChildrenOpen}
                      className="top-1.5 rounded-md"
                      onClick={() => {
                        setCollapsedItems((previous) => {
                          const next = new Set(previous);
                          if (next.has(item.link)) next.delete(item.link);
                          else next.add(item.link);
                          return next;
                        });
                      }}
                    >
                      <ArrowRight01Icon
                        className={cn('size-3 transition-transform duration-200', areChildrenOpen && 'rotate-90')}
                      />
                    </SidebarMenuAction>
                  )}

                  {item.children && areChildrenOpen && (
                    <SidebarMenuSub className="my-0.5 mr-0 gap-0.5 border-sidebar-border/80 py-0.5 pr-0">
                      {item.children.map((child) => (
                        <SidebarMenuSubItem key={child.link}>
                          <SidebarMenuSubButton
                            asChild
                            isActive={isActive(child.link)}
                            className="h-7 rounded-md px-2 text-xs font-normal data-active:font-medium"
                          >
                            <a
                              href={child.link}
                              title={child.label}
                              onClick={(event) => {
                                event.preventDefault();
                                onNavigate(child.link);
                              }}
                            >
                              <span>{child.label}</span>
                            </a>
                          </SidebarMenuSubButton>
                        </SidebarMenuSubItem>
                      ))}
                    </SidebarMenuSub>
                  )}
                </SidebarMenuItem>
              );
            })}
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
                    <ArrowRight01Icon className={`h-3 w-3 shrink-0 transition-transform duration-200 ${isOpen ? 'rotate-90' : ''}`} />
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
