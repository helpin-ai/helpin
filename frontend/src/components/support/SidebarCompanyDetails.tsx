import { useEffect, useMemo, useState } from 'react';
import { formatDistanceToNow } from 'date-fns';
import { ArrowReloadHorizontalIcon, Building03Icon, PlusSignIcon } from '@/lib/icons';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Skeleton } from '@/components/ui/skeleton';
import { CollapsibleSection } from '@/components/ui/collapsible-section';
import { usePermissions, useWorkspaceAccess } from '@/hooks/queries';
import { useUpdateConversationCRMCompany, useVisitorContext } from '@/hooks/queries/useSupport';
import { crmSearchService } from '@/lib/services/crmService';
import { isSystemCRMCustomProperty } from '@/lib/crmCustomProperties';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import type { CRMSearchResult } from '@/lib/crmTypes';

interface SidebarCompanyDetailsProps {
  workspaceId: string;
  conversationId: string;
}

const PRIORITY_KEYS = [
  'plan', 'subscription_status', 'support_tier', 'seats_used', 'seats_total',
  'trial_ends_at', 'renewal_date', 'mrr', 'arr',
] as const;
const HIDDEN_KEYS = new Set(['sdk_company_id', 'sdk_created_at', 'currency']);
const SUBSCRIPTION_STATUS_COLORS: Record<string, string> = {
  active: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300',
  trialing: 'bg-blue-100 text-blue-700 dark:bg-blue-950 dark:text-blue-300',
  past_due: 'bg-amber-100 text-amber-700 dark:bg-amber-950 dark:text-amber-300',
  paused: 'bg-amber-100 text-amber-700 dark:bg-amber-950 dark:text-amber-300',
  canceled: 'bg-red-100 text-red-700 dark:bg-red-950 dark:text-red-300',
  cancelled: 'bg-red-100 text-red-700 dark:bg-red-950 dark:text-red-300',
};

function humanize(key: string): string {
  return key.replaceAll('_', ' ').replace(/\b\w/g, (letter) => letter.toUpperCase());
}

function titleCase(value: unknown): string {
  return String(value ?? '').replaceAll('_', ' ').replace(/\b\w/g, (letter) => letter.toUpperCase());
}

function hasValue(value: unknown): boolean {
  return value !== null && value !== undefined && value !== '';
}

export function formatCompanyValue(key: string, value: unknown, currency = 'USD'): string | null {
  if (!hasValue(value)) return null;
  if (typeof value === 'boolean') return value ? 'Yes' : 'No';
  if ((key === 'mrr' || key === 'arr') && typeof value === 'number') {
    return new Intl.NumberFormat(undefined, { style: 'currency', currency, maximumFractionDigits: 0 }).format(value);
  }
  if (key.endsWith('_at') || key === 'renewal_date') {
    const date = new Date(String(value));
    if (!Number.isNaN(date.getTime())) return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium' }).format(date);
  }
  if (typeof value === 'number') return new Intl.NumberFormat().format(value);
  if (typeof value === 'string') return titleCase(value);
  return JSON.stringify(value);
}

function CompanyRow({ label, value }: { label: string; value: string | null }) {
  if (!value) return null;
  return (
    <div className="grid grid-cols-[88px_1fr] items-center gap-2 text-[12px]">
      <span className="text-muted-foreground">{label}</span>
      <span className="truncate font-medium text-foreground/90" title={value}>{value}</span>
    </div>
  );
}

export function SidebarCompanyDetails({ workspaceId, conversationId }: SidebarCompanyDetailsProps) {
  const { data, isLoading, refetch } = useVisitorContext(workspaceId, conversationId);
  const updateCompany = useUpdateConversationCRMCompany(workspaceId);
  const { data: access } = useWorkspaceAccess(workspaceId);
  const { has } = usePermissions(access);
  const canEdit = has('support.edit');
  const slug = useWorkspaceStore((state) => state.currentWorkspace?.slug ?? '');
  const [showMore, setShowMore] = useState(false);
  const [searchOpen, setSearchOpen] = useState(false);
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<CRMSearchResult[]>([]);

  useEffect(() => {
    setShowMore(false);
    setSearchOpen(false);
    setQuery('');
    setResults([]);
  }, [conversationId]);

  useEffect(() => {
    if (!searchOpen || query.trim().length < 2) {
      setResults([]);
      return;
    }
    let cancelled = false;
    const timer = window.setTimeout(async () => {
      const response = await crmSearchService.search(workspaceId, query.trim());
      if (!cancelled) {
        setResults((response.data ?? []).filter((item) => item.type === 'company'));
      }
    }, 250);
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [query, searchOpen, workspaceId]);

  const company = data?.company ?? null;
  const options = data?.company_options ?? [];
  const properties = company?.custom_properties ?? {};
  const currency = typeof properties.currency === 'string' ? properties.currency : 'USD';
  const extraEntries = useMemo(() => Object.entries(properties).filter(([key, value]) => (
    !PRIORITY_KEYS.includes(key as typeof PRIORITY_KEYS[number])
      && !HIDDEN_KEYS.has(key)
      && !isSystemCRMCustomProperty(key)
      && hasValue(value)
  )), [properties]);

  const chooseCompany = (companyId: string | null) => {
    updateCompany.mutate({ conversationId, companyId });
    setSearchOpen(false);
    setQuery('');
  };

  if (isLoading) {
    return (
      <CollapsibleSection title="Company Details" icon={Building03Icon} count={0} defaultOpen>
        <div className="space-y-2"><Skeleton className="h-3 w-full" /><Skeleton className="h-3 w-4/5" /></div>
      </CollapsibleSection>
    );
  }

  if (data?.company_context_status === 'error') {
    return (
      <CollapsibleSection title="Company Details" icon={Building03Icon} count={0} defaultOpen>
        <div className="flex items-center justify-between gap-2 text-xs text-muted-foreground">
          <span>Company details could not be loaded.</span>
          <Button variant="ghost" size="sm" className="h-7 px-2" onClick={() => void refetch()}><ArrowReloadHorizontalIcon className="mr-1 h-3 w-3" />Retry</Button>
        </div>
      </CollapsibleSection>
    );
  }

  return (
    <CollapsibleSection title="Company Details" icon={Building03Icon} count={0} defaultOpen>
      <div className="space-y-2">
        {company ? (
          <>
            <div className="grid grid-cols-[88px_1fr] items-center gap-2 text-[12px]">
              <span className="text-muted-foreground">Company</span>
              <div className="min-w-0">
                <a href={slug ? `/w/${slug}/crm/companies/${company.id}` : '#'} className="truncate font-medium text-blue-600 hover:underline dark:text-blue-400">{company.name}</a>
              </div>
            </div>
            {canEdit && options.length > 1 && (
              <select
                aria-label="Conversation company"
                className="h-7 w-full rounded-md border bg-background px-2 text-xs"
                value={company.id}
                disabled={updateCompany.isPending}
                onChange={(event) => chooseCompany(event.target.value || null)}
              >
                {options.map((option) => <option key={option.id} value={option.id}>{option.name}</option>)}
              </select>
            )}
            <CompanyRow label="Domain" value={company.domain ?? null} />
            <CompanyRow label="Plan" value={formatCompanyValue('plan', properties.plan, currency)} />
            {hasValue(properties.subscription_status) && (
              <div className="grid grid-cols-[88px_1fr] items-center gap-2 text-[12px]">
                <span className="text-muted-foreground">Status</span>
                <span><Badge variant="secondary" className={`h-5 rounded-full px-2 text-[10px] ${SUBSCRIPTION_STATUS_COLORS[String(properties.subscription_status).toLowerCase()] ?? ''}`}>{titleCase(properties.subscription_status)}</Badge></span>
              </div>
            )}
            <CompanyRow label="Support tier" value={formatCompanyValue('support_tier', properties.support_tier, currency)} />
            <CompanyRow label="Seats" value={hasValue(properties.seats_used) || hasValue(properties.seats_total) ? `${properties.seats_used ?? '—'} / ${properties.seats_total ?? '—'}` : null} />
            <CompanyRow label="Trial ends" value={formatCompanyValue('trial_ends_at', properties.trial_ends_at, currency)} />
            <CompanyRow label="Renewal" value={formatCompanyValue('renewal_date', properties.renewal_date, currency)} />
            <CompanyRow label="MRR" value={formatCompanyValue('mrr', properties.mrr, currency)} />
            <CompanyRow label="ARR" value={formatCompanyValue('arr', properties.arr, currency)} />
            {(showMore ? extraEntries : extraEntries.slice(0, 3)).map(([key, value]) => (
              <CompanyRow key={key} label={humanize(key)} value={formatCompanyValue(key, value, currency)} />
            ))}
            {!showMore && extraEntries.length > 3 && <button type="button" className="text-xs font-medium text-blue-600" onClick={() => setShowMore(true)}>Show {extraEntries.length - 3} more</button>}
            <CompanyRow label="Updated" value={company.updated_at ? formatDistanceToNow(new Date(company.updated_at), { addSuffix: true }) : null} />
          </>
        ) : (
          <p className="text-xs text-muted-foreground">{options.length > 1 ? 'Select the company for this conversation.' : 'No company linked to this conversation.'}</p>
        )}

        {canEdit && !company && options.length > 0 && (
          <select aria-label="Select conversation company" className="h-8 w-full rounded-md border bg-background px-2 text-xs" defaultValue="" onChange={(event) => event.target.value && chooseCompany(event.target.value)}>
            <option value="" disabled>Select company</option>
            {options.map((option) => <option key={option.id} value={option.id}>{option.name}</option>)}
          </select>
        )}
        {canEdit && (
          <div className="space-y-2 pt-1">
            <div className="flex gap-1">
              <Button variant="ghost" size="sm" className="h-7 px-2 text-xs" onClick={() => setSearchOpen((open) => !open)}><PlusSignIcon className="mr-1 h-3 w-3" />{company ? 'Link another company' : 'Link a company'}</Button>
              {company && <Button variant="ghost" size="sm" className="h-7 px-2 text-xs text-muted-foreground" onClick={() => chooseCompany(null)}>Clear</Button>}
            </div>
            {searchOpen && (
              <div className="space-y-1">
                <Input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search companies" className="h-8 text-xs" />
                {results.map((result) => <button key={result.id} type="button" className="block w-full rounded px-2 py-1.5 text-left text-xs hover:bg-accent" onClick={() => chooseCompany(result.id)}>{result.name}<span className="ml-1 text-muted-foreground">{result.detail}</span></button>)}
              </div>
            )}
          </div>
        )}
      </div>
    </CollapsibleSection>
  );
}
