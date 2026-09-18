import { useEffect, useRef, useState } from 'react';
import { Collapsible } from 'radix-ui';
import { buildSettingsHomePath } from '@/lib/settingsDiscovery';
import { SETTINGS_HOME_LABEL, SETTINGS_SIDEBAR_GROUP_LABELS, SETTINGS_TOP_LEVEL_GROUPS } from '@/lib/settingsSections';
import { ArrowRight01Icon } from '@/lib/icons';
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
          {groups.filter(group => SETTINGS_TOP_LEVEL_GROUPS.has(group.label)).flatMap(group => group.items).map(item => (
            <SidebarMenuItem key={item.link}>
              <SidebarMenuButton asChild isActive={isActive(item.link)}>
                <a href={item.link} onClick={event => { event.preventDefault(); onNavigate(item.link); }}><item.icon className="size-4 shrink-0 text-sidebar-foreground/75" /><span>{item.label}</span></a>
              </SidebarMenuButton>
            </SidebarMenuItem>
          ))}
        </SidebarMenu>
      </div>
      {groups.filter(group => !SETTINGS_TOP_LEVEL_GROUPS.has(group.label)).map((group, index) => {
        const isCollapsible = COLLAPSIBLE_SETTINGS_GROUPS.has(group.label);
        const groupLabel = SETTINGS_SIDEBAR_GROUP_LABELS[group.label] ?? group.label;
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
                    className="h-8 rounded-md px-2 text-sm font-normal data-active:font-medium"
                  >
                    <a
                      href={item.link}
                      onClick={(event) => {
                        event.preventDefault();
                        onNavigate(item.link);
                      }}
                    >
                      <item.icon className="size-4 shrink-0 text-sidebar-foreground/75" />
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
              <SidebarGroup className="p-0 pb-1">
                <SidebarGroupLabel asChild className="h-9 w-full gap-2.5 rounded-md px-2 text-sm font-medium normal-case tracking-normal text-sidebar-foreground transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground focus-visible:ring-2 focus-visible:ring-sidebar-ring">
                  <Collapsible.Trigger type="button" className="cursor-pointer select-none text-left">
                    <span className="min-w-0 flex-1">{groupLabel}</span>
                    <ArrowRight01Icon className={`size-4 shrink-0 text-sidebar-foreground/70 transition-transform duration-200 motion-reduce:transition-none ${isOpen ? 'rotate-90' : ''}`} />
                  </Collapsible.Trigger>
                </SidebarGroupLabel>
                <Collapsible.Content className="mb-2 ml-4 mt-1 border-l border-sidebar-border pl-2">
                  {menuItems}
                </Collapsible.Content>
              </SidebarGroup>
            </Collapsible.Root>
          );
        }

        return (
          <SidebarGroup key={group.label || index} className="p-0 pb-1">
            {group.label && (
              <SidebarGroupLabel className="h-9 px-2 text-sm font-medium text-sidebar-foreground">
                {groupLabel}
              </SidebarGroupLabel>
            )}
            {menuItems}
          </SidebarGroup>
        );
      })}
    </>
  );
}
