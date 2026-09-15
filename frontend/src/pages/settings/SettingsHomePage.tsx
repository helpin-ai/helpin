import { useRef, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { QuietPageHeader, QuietSearchInput, QuietSectionHeader, QuietTextAction } from '@/components/design-system/quiet';
import { ArrowRight01Icon } from '@/lib/icons';
import { getSettingsSidebarGroups, buildSettingsRoutePath, type SettingsSidebarGroup, type SettingsRouteSection } from '@/lib/settingsSections';
import { searchSettings, readRecentSettings } from '@/lib/settingsDiscovery';
import { useWorkspaceAccess, usePermissions } from '@/hooks/queries/useSession';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useAuthStore } from '@/stores/authStore';
import { useTitle } from '@/hooks/useTitle';

export function SettingsHomeView({ groups, slug, recent = [], autoFocus = false, onNavigate }: {
  groups: SettingsSidebarGroup[]; slug: string; recent?: SettingsRouteSection[]; autoFocus?: boolean; onNavigate?: (href: string) => void;
}) {
  const [query, setQuery] = useState('');
  const resultList = useRef<HTMLDivElement>(null);
  const sections = groups.flatMap(group => group.sections);
  const recentSections = recent.map(id => sections.find(section => section.id === id)).filter(section => !!section);
  const results = searchSettings(groups, query);
  const linkProps = (href: string) => ({ href, onClick: (event: React.MouseEvent<HTMLAnchorElement>) => {
    if (onNavigate && !event.metaKey && !event.ctrlKey && !event.shiftKey && !event.altKey && event.button === 0) { event.preventDefault(); onNavigate(href); }
  } });
  return <div className="mx-auto w-full max-w-5xl space-y-8">
    <QuietPageHeader title="Settings" description="Find what you need to manage your account and workspace." />
    <QuietSearchInput data-settings-search aria-label="Search settings" placeholder="Search settings… Try signature, invite, or domain" value={query} onChange={event => setQuery(event.target.value)} autoFocus={autoFocus} onKeyDown={event => {
      if (event.key !== 'Enter' && event.key !== 'ArrowDown') return;
      const first = resultList.current?.querySelector<HTMLAnchorElement>('a');
      if (!first) return;
      event.preventDefault();
      if (event.key === 'Enter') first.click(); else first.focus();
    }} containerClassName="w-full" />
    {query.trim() ? <section aria-label="Search results">
      <p className="mb-3 text-xs text-muted-foreground" role="status">{results.length ? `${results.length} ${results.length === 1 ? 'result' : 'results'}` : 'No matching settings. Try a different name or keyword.'}</p>
      <div ref={resultList} className="divide-y divide-quiet-divider-light">
        {results.map(result => <a key={`${result.sectionId}:${result.optionId ?? ''}`} {...linkProps(`${buildSettingsRoutePath(slug, result.sectionId)}${result.optionId ? `#${result.optionId}` : ''}`)} className="group flex items-center justify-between gap-4 rounded-sm py-4 focus-visible:outline-2 focus-visible:outline-quiet-field">
          <span><span className="block text-sm font-medium group-hover:underline">{result.label}</span><span className="mt-1 block text-xs text-muted-foreground">{result.group} · {result.pageLabel}{result.scope === 'organization' ? ' · Organization-wide' : ''}</span></span>
          <ArrowRight01Icon className="h-4 w-4 shrink-0 text-muted-foreground" />
        </a>)}
      </div>
      <QuietTextAction onClick={() => setQuery('')} className="mt-4">Clear search</QuietTextAction>
    </section> : <>
      {recentSections.length > 0 && <section aria-label="Recently visited">
        <QuietSectionHeader title="Recently visited" />
        <div className="mt-3 flex flex-wrap gap-x-6 gap-y-3">{recentSections.map(section => <QuietTextAction key={section.id} asChild><a {...linkProps(buildSettingsRoutePath(slug, section.id))}>{section.label}</a></QuietTextAction>)}</div>
      </section>}
      <div className="grid gap-x-12 gap-y-8 md:grid-cols-2">
        {groups.map(group => <section key={group.label} aria-label={group.label}>
          <QuietSectionHeader title={group.label} className="mb-2" />
          <div className="divide-y divide-quiet-divider-light">{group.sections.map(section => <a key={section.id} {...linkProps(buildSettingsRoutePath(slug, section.id))} className="group flex items-start gap-3 rounded-sm py-2 focus-visible:outline-2 focus-visible:outline-quiet-field">
            <section.icon className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" />
            <span className="min-w-0"><span className="text-sm font-medium group-hover:underline">{section.label}</span>{section.scope === 'organization' && <span className="ml-2 text-[11px] text-muted-foreground">Organization-wide</span>}<span className="mt-1 block truncate text-xs leading-5 text-muted-foreground" title={section.description}>{section.description}</span></span>
          </a>)}</div>
        </section>)}
      </div>
    </>}
  </div>;
}

export function SettingsHomePage({ autoFocus = false }: { autoFocus?: boolean }) {
  useTitle('Settings');
  const ws = useWorkspaceStore(state => state.currentWorkspace);
  const userId = useAuthStore(state => state.user?.id);
  const access = useWorkspaceAccess(ws?.id ?? '');
  const permissions = usePermissions(access.data);
  const navigate = useNavigate();
  if (access.isPending) return <p className="text-sm text-muted-foreground" role="status">Loading settings…</p>;
  if (access.isError) return <QuietTextAction onClick={() => void access.refetch()}>Couldn’t load settings. Try again</QuietTextAction>;
  if (!ws) return null;
  return <SettingsHomeView groups={getSettingsSidebarGroups(permissions.canManageSettings, permissions.permissionSet)} slug={ws.slug} recent={readRecentSettings(`${userId ?? ''}:${ws.id}`)} autoFocus={autoFocus} onNavigate={href => void navigate({ to: href })} />;
}
