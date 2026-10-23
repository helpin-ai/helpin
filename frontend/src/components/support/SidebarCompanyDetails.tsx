import { useEffect, useMemo, useState } from 'react';
import { formatDistanceToNow } from 'date-fns';
import { ArrowReloadHorizontalIcon, ArrowUpDownIcon, ArrowUpRight01Icon, Cancel01Icon, PlusSignIcon } from '@/lib/icons';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  QuietDropdownContent,
  QuietDropdownEmpty,
  QuietDropdownGroup,
  QuietDropdownItem,
  QuietDropdownOptions,
  QuietDropdownRoot,
  QuietDropdownTrigger,
} from '@/components/design-system/quiet-dropdown';
import { Skeleton } from '@/components/ui/skeleton';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { CollapsibleSection } from '@/components/ui/collapsible-section';
import { usePermissions, useWorkspaceAccess } from '@/hooks/queries';
import { useUpdateConversationCRMCompany, useVisitorContext } from '@/hooks/queries/useSupport';
import { crmSearchService } from '@/lib/services/crmService';
import { isSystemCRMCustomProperty } from '@/lib/crmCustomProperties';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import type { CRMSearchResult } from '@/lib/crmTypes';
import type { VisitorCompanyOption } from '@/lib/pmTypes';

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

function companyHref(slug: string, companyId: string): string {
  return slug ? `/w/${slug}/crm/companies/${companyId}` : '#';
}

function openCompanyInNewTab(slug: string, companyId: string) {
  window.open(companyHref(slug, companyId), '_blank', 'noopener,noreferrer');
}

interface CompanyPickerProps {
  workspaceId: string;
  slug: string;
  contactFirstName: string | null;
  currentCompany: { id: string; name: string } | null;
  options: VisitorCompanyOption[];
  disabled: boolean;
  onChoose: (companyId: string | null) => void;
  trigger: React.ReactElement;
}

/**
 * CompanyPicker switches the conversation's company among the contact's
 * companies, finds any other CRM company, and unlinks the conversation.
 */
function CompanyPicker({ workspaceId, slug, contactFirstName, currentCompany, options, disabled, onChoose, trigger }: CompanyPickerProps) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<CRMSearchResult[]>([]);
  const [searching, setSearching] = useState(false);
  const term = query.trim();

  useEffect(() => {
    if (!open || term.length < 2) {
      setResults([]);
      setSearching(false);
      return;
    }
    let cancelled = false;
    setSearching(true);
    const timer = window.setTimeout(async () => {
      const response = await crmSearchService.search(workspaceId, term);
      if (cancelled) return;
      setResults((response.data ?? []).filter((item) => item.type === 'company'));
      setSearching(false);
    }, 250);
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [open, term, workspaceId]);

  const lowerTerm = term.toLowerCase();
  const contactCompanies = options.filter((option) => (
    !lowerTerm || option.name.toLowerCase().includes(lowerTerm) || (option.domain ?? '').toLowerCase().includes(lowerTerm)
  ));
  const optionIds = new Set(options.map((option) => option.id));
  const otherCompanies = results.filter((result) => !optionIds.has(result.id));

  const choose = (companyId: string | null) => {
    onChoose(companyId);
    setOpen(false);
    setQuery('');
  };

  return (
    <QuietDropdownRoot open={open && !disabled} onOpenChange={(next) => { setOpen(next); if (!next) setQuery(''); }}>
      <QuietDropdownTrigger asChild>{trigger}</QuietDropdownTrigger>
      <QuietDropdownContent align="end" className="w-64" aria-label="Conversation company options">
        <QuietDropdownOptions
          shouldFilter={false}
          searchMode="always"
          searchPlaceholder="Search companies"
          query={query}
          onQueryChange={setQuery}
          footer={currentCompany ? (
            <div className="shrink-0 border-t border-border/60 p-1">
              <Tooltip>
                <TooltipTrigger asChild>
                  <button
                    type="button"
                    className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm hover:bg-accent focus-visible:bg-accent focus-visible:outline-none"
                    onClick={() => choose(null)}
                  >
                    <Cancel01Icon className="h-3.5 w-3.5 text-muted-foreground" />
                    Unlink from conversation
                  </button>
                </TooltipTrigger>
                <TooltipContent side="bottom" className="max-w-56">
                  Removes {currentCompany.name} from this conversation only. The company{contactFirstName ? ` and ${contactFirstName}'s link to it` : ''} stay{contactFirstName ? '' : 's'} in CRM.
                </TooltipContent>
              </Tooltip>
            </div>
          ) : undefined}
        >
          {contactCompanies.length > 0 && (
            <QuietDropdownGroup heading={contactFirstName ? `${contactFirstName}'s companies` : 'Contact companies'}>
              {contactCompanies.map((option) => (
                <QuietDropdownItem
                  key={option.id}
                  value={option.id}
                  fullText={option.name}
                  data-checked={option.id === currentCompany?.id}
                  aria-label={`${option.name}${option.id === currentCompany?.id ? ', selected' : ''}`}
                  onSelect={() => choose(option.id)}
                >
                  <span className="min-w-0 truncate">{option.name}</span>
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <button
                        type="button"
                        aria-label={`Open ${option.name} in CRM`}
                        className="flex h-6 w-6 shrink-0 items-center justify-center rounded-md text-muted-foreground opacity-0 transition-opacity hover:bg-background hover:text-foreground focus-visible:opacity-100 group-hover/command-item:opacity-100 group-data-selected/command-item:opacity-100"
                        onPointerDown={(event) => event.stopPropagation()}
                        onClick={(event) => {
                          event.stopPropagation();
                          openCompanyInNewTab(slug, option.id);
                        }}
                      >
                        <ArrowUpRight01Icon className="h-3.5 w-3.5" />
                      </button>
                    </TooltipTrigger>
                    <TooltipContent side="bottom">Open in CRM</TooltipContent>
                  </Tooltip>
                </QuietDropdownItem>
              ))}
            </QuietDropdownGroup>
          )}
          {otherCompanies.length > 0 && (
            <QuietDropdownGroup heading="Other companies">
              {otherCompanies.map((result) => (
                <QuietDropdownItem key={result.id} value={result.id} fullText={result.name} onSelect={() => choose(result.id)}>
                  <span className="min-w-0 truncate">{result.name}</span>
                  {result.detail && <span className="min-w-0 shrink truncate text-xs text-muted-foreground">{result.detail}</span>}
                </QuietDropdownItem>
              ))}
            </QuietDropdownGroup>
          )}
          {contactCompanies.length === 0 && otherCompanies.length === 0 && (
            <QuietDropdownEmpty className="px-3 py-3 text-center text-muted-foreground">
              {searching ? 'Searching…' : term.length < 2 ? 'Type at least 2 characters' : 'No companies found'}
            </QuietDropdownEmpty>
          )}
        </QuietDropdownOptions>
      </QuietDropdownContent>
    </QuietDropdownRoot>
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

  useEffect(() => {
    setShowMore(false);
  }, [conversationId]);

  const company = data?.company ?? null;
  const options = data?.company_options ?? [];
  const properties = company?.custom_properties ?? {};
  const currency = typeof properties.currency === 'string' ? properties.currency : 'USD';
  const contactFirstName = data?.contact?.name?.trim().split(/\s+/)[0] || null;
  const extraEntries = useMemo(() => Object.entries(properties).filter(([key, value]) => (
    !PRIORITY_KEYS.includes(key as typeof PRIORITY_KEYS[number])
      && !HIDDEN_KEYS.has(key)
      && !isSystemCRMCustomProperty(key)
      && hasValue(value)
  )), [properties]);

  const chooseCompany = (companyId: string | null) => {
    updateCompany.mutate({ conversationId, companyId });
  };

  if (isLoading) {
    return (
      <CollapsibleSection title="Company details" count={0} defaultOpen persistKey="support-details.company">
        <div className="space-y-2"><Skeleton className="h-3 w-full" /><Skeleton className="h-3 w-4/5" /></div>
      </CollapsibleSection>
    );
  }

  if (data?.company_context_status === 'error') {
    return (
      <CollapsibleSection title="Company details" count={0} defaultOpen persistKey="support-details.company">
        <div className="flex items-center justify-between gap-2 text-xs text-muted-foreground">
          <span>Company details could not be loaded.</span>
          <Button variant="ghost" size="sm" className="h-7 px-2" onClick={() => void refetch()}><ArrowReloadHorizontalIcon className="mr-1 h-3 w-3" />Retry</Button>
        </div>
      </CollapsibleSection>
    );
  }

  const pickerProps = {
    workspaceId,
    slug,
    contactFirstName,
    currentCompany: company ? { id: company.id, name: company.name } : null,
    options,
    disabled: updateCompany.isPending,
    onChoose: chooseCompany,
  };

  return (
    <CollapsibleSection title="Company details" count={0} defaultOpen persistKey="support-details.company">
      <div className="space-y-2">
        <div className="grid grid-cols-[88px_1fr] items-center gap-2 text-[12px]">
          <span className="text-muted-foreground">Company</span>
          <div className="flex min-w-0 items-center gap-1">
            {company ? (
              <>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <a
                      href={companyHref(slug, company.id)}
                      target="_blank"
                      rel="noopener noreferrer"
                      aria-label={`Open ${company.name} in CRM`}
                      className="group/company flex min-w-0 items-center gap-1 font-medium text-blue-600 dark:text-blue-400"
                    >
                      <span className="truncate group-hover/company:underline">{company.name}</span>
                      <ArrowUpRight01Icon className="h-3 w-3 shrink-0" />
                    </a>
                  </TooltipTrigger>
                  <TooltipContent side="bottom">Open in CRM</TooltipContent>
                </Tooltip>
                <span className="flex-1" />
                {canEdit && (
                  <CompanyPicker
                    {...pickerProps}
                    trigger={(
                      <button
                        type="button"
                        aria-label={options.length > 1 ? 'Switch company' : 'Company options'}
                        className="-mr-1 flex h-[22px] w-[22px] shrink-0 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-quiet-hover hover:text-foreground"
                      >
                        <ArrowUpDownIcon className="h-3.5 w-3.5" />
                      </button>
                    )}
                  />
                )}
              </>
            ) : canEdit ? (
              <CompanyPicker
                {...pickerProps}
                trigger={(
                  <button
                    type="button"
                    className="-ml-1.5 flex h-6 items-center gap-1 rounded-md px-1.5 text-muted-foreground transition-colors hover:bg-quiet-hover hover:text-foreground"
                  >
                    <PlusSignIcon className="h-3 w-3" />
                    {options.length > 0 ? 'Select a company' : 'Link a company'}
                  </button>
                )}
              />
            ) : (
              <span className="text-muted-foreground">No company linked</span>
            )}
          </div>
        </div>
        {company && (
          <>
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
        )}
      </div>
    </CollapsibleSection>
  );
}
