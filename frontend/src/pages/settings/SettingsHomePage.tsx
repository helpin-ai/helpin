import { useRef, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { QuietPageHeader, QuietSearchInput, QuietTextAction } from '@/components/design-system/quiet';
import { Card } from '@/components/ui/card';
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
  const recentSections = recent.map(id => sections.find(section => section.id === id)).filter(section => !!section).slice(0, 5);
  const results = searchSettings(groups, query);
  const linkProps = (href: string) => ({ href, onClick: (event: React.MouseEvent<HTMLAnchorElement>) => {
    if (onNavigate && !event.metaKey && !event.ctrlKey && !event.shiftKey && !event.altKey && event.button === 0) { event.preventDefault(); onNavigate(href); }
  } });
  return <div className="mx-auto w-full max-w-5xl space-y-6">
    <QuietPageHeader title="Settings" />
    <QuietSearchInput data-settings-search aria-label="Search settings" placeholder="Search email, notifications, members…" value={query} onChange={event => setQuery(event.target.value)} autoFocus={autoFocus} onKeyDown={event => {
      if (event.key !== 'Enter' && event.key !== 'ArrowDown') return;
      const first = resultList.current?.querySelector<HTMLAnchorElement>('a');
      if (!first) return;
      event.preventDefault();
      if (event.key === 'Enter') first.click(); else first.focus();
    }} containerClassName="w-full" className="h-11" />
      {!query.trim() && recentSections.length > 0 && <section aria-label="Recently visited">
        <h2 className="text-sm font-semibold text-quiet-text-primary">Recently visited</h2>
        <div className="mt-2 flex flex-wrap gap-x-5 gap-y-2">{recentSections.map(section => <QuietTextAction key={section.id} asChild><a {...linkProps(buildSettingsRoutePath(slug, section.id))}>{section.label}</a></QuietTextAction>)}</div>
      </section>}
    {query.trim() ? <section aria-label="Search results">
      <p className="mb-3 text-xs text-muted-foreground" role="status">{results.length ? `${results.length} ${results.length === 1 ? 'result' : 'results'}` : 'No matching settings. Try a different name or keyword.'}</p>
      <div ref={resultList} className="divide-y divide-quiet-divider-light">
        {results.map(result => <a key={`${result.sectionId}:${result.optionId ?? ''}`} {...linkProps(`${buildSettingsRoutePath(slug, result.sectionId)}${result.optionId ? `#${result.optionId}` : ''}`)} className="group flex items-center justify-between gap-4 rounded-md px-3 py-3 transition-colors hover:bg-muted/40 focus-visible:outline-2 focus-visible:outline-ring">
          <span><span className="block text-sm font-medium group-hover:underline">{result.label}</span><span className="mt-1 block text-xs text-muted-foreground">{result.group}{result.optionId ? ` · ${result.pageLabel}` : ''}{result.scope === 'organization' ? ' · Organization-wide' : ''}</span></span>
          <ArrowRight01Icon className="h-4 w-4 shrink-0 text-muted-foreground" />
        </a>)}
      </div>
      <QuietTextAction onClick={() => setQuery('')} className="mt-4">Clear search</QuietTextAction>
    </section> : <>
      <div className="grid items-start gap-4 md:grid-cols-2">
        {groups.map(group => <Card key={group.label} className="gap-0 rounded-lg border-border/70 py-0 shadow-none">
          <section aria-label={group.label}>
            <h2 className="border-b border-quiet-divider-strong px-4 py-3 text-sm font-semibold text-quiet-text-primary">{group.label}</h2>
            <div className="p-2">{group.sections.map(section => <a key={section.id} {...linkProps(buildSettingsRoutePath(slug, section.id))} className="group flex min-w-0 items-center gap-3 rounded-md px-2 py-3 transition-colors hover:bg-muted/40 focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-ring">
                <section.icon className="h-4 w-4 shrink-0 text-quiet-text-secondary" />
                <span className="min-w-0 flex-1"><span className="block text-sm font-medium text-quiet-text-primary">{section.label}</span>{section.scope === 'organization' && <span className="block text-xs text-quiet-text-secondary">Organization-wide</span>}<span className="mt-1 block text-xs leading-5 text-quiet-text-secondary">{section.description}</span></span>
                <ArrowRight01Icon aria-hidden className="h-4 w-4 shrink-0 text-quiet-text-secondary opacity-50 transition-opacity group-hover:opacity-100 group-focus-visible:opacity-100" />
              </a>)}</div>
          </section>
        </Card>)}
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
  return <SettingsHomeView groups={getSettingsSidebarGroups(permissions.canManageSettings, permissions.permissionSet, permissions.modules)} slug={ws.slug} recent={readRecentSettings(`${userId ?? ''}:${ws.id}`)} autoFocus={autoFocus} onNavigate={href => void navigate({ to: href })} />;
}
