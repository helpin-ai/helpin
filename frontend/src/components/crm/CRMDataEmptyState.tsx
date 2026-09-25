import type { ComponentType } from 'react';
import {
  Building03Icon,
  CodeIcon,
  DatabaseIcon,
  PlusSignIcon,
  Search01Icon,
  Upload01Icon,
  UserCheck01Icon,
  UserGroupIcon,
} from '@/lib/icons';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';

type CRMDataEmptyKind = 'contacts' | 'companies';

interface CRMDataEmptyStateProps {
  kind: CRMDataEmptyKind;
  onCreateClick: () => void;
}

interface CRMNoResultsStateProps {
  kind: CRMDataEmptyKind;
  query: string;
  onClear: () => void;
}

const SDK_SNIPPET = `npm install @helpin-ai/sdk-js

import { helpinClient } from '@helpin-ai/sdk-js';

const helpin = helpinClient({
  widgetKey: 'YOUR_WIDGET_KEY',
  host: 'https://client.helpin.ai',
});

const company = {
  id: account.id,
  name: account.name,
  created_at: account.createdAt,
  domain: account.domain,
};

helpin?.lead({
  email: lead.email,
  first_name: lead.firstName,
  last_name: lead.lastName,
  lifecycle_stage: 'marketing_qualified',
  source: 'pricing_form',
  company,
}, true);

await helpin?.id({
  id: user.id,
  email: user.email,
  first_name: user.firstName,
  last_name: user.lastName,
  lifecycle_stage: 'customer',
  company,
});

await helpin?.group(company);`;

const EMPTY_COPY: Record<CRMDataEmptyKind, {
  icon: ComponentType<{ className?: string }>;
  title: string;
  description: string;
  createLabel: string;
  importLabel: string;
  points: Array<{
    icon: ComponentType<{ className?: string }>;
    title: string;
    description: string;
  }>;
}> = {
  contacts: {
    icon: UserGroupIcon,
    title: 'No contacts yet',
    description: 'Start with people your team can act on: leads from forms, customers who sign in, and contacts imported from your existing CRM.',
    createLabel: 'Create contact',
    importLabel: 'Import contacts',
    points: [
      {
        icon: UserCheck01Icon,
        title: 'Leads from forms',
        description: 'Use lead(...) when you only know a prospect email and a few qualification details.',
      },
      {
        icon: UserGroupIcon,
        title: 'Customers from your app',
        description: 'Use id(...) after sign-in so support, CRM, and sales share the same person record.',
      },
      {
        icon: Building03Icon,
        title: 'Company context',
        description: 'Attach company details to people so account, deal, and signal views stay connected.',
      },
    ],
  },
  companies: {
    icon: Building03Icon,
    title: 'No companies yet',
    description: 'Company records group people into accounts so deals, support conversations, and CRM signals land against the right organization.',
    createLabel: 'Create company',
    importLabel: 'Import companies',
    points: [
      {
        icon: Building03Icon,
        title: 'Accounts from product data',
        description: 'Use group(...) with your account ID, name, created_at, domain, plan, and other account fields.',
      },
      {
        icon: UserCheck01Icon,
        title: 'Customers on the account',
        description: 'Use id(...) with the same company object when a user signs in or changes plan.',
      },
      {
        icon: DatabaseIcon,
        title: 'Enriched CRM records',
        description: 'Consistent company IDs let imports, SDK events, support sessions, and sales activity merge cleanly.',
      },
    ],
  },
};

export function CRMDataEmptyState({ kind, onCreateClick }: CRMDataEmptyStateProps) {
  const copy = EMPTY_COPY[kind];
  const Icon = copy.icon;

  return (
    <div className="h-full overflow-auto">
      <div className="mx-auto flex min-h-full w-full max-w-6xl flex-col gap-4 p-2 md:p-4">
        <div className="rounded-lg border bg-card p-5">
          <div className="max-w-3xl">
            <div className="flex h-11 w-11 items-center justify-center rounded-lg border bg-muted/40">
              <Icon className="h-5 w-5 text-muted-foreground" />
            </div>
            <h2 className="mt-4 text-lg font-semibold">{copy.title}</h2>
            <p className="mt-2 max-w-2xl text-sm leading-6 text-muted-foreground">{copy.description}</p>
            <div className="mt-5 flex flex-wrap gap-2">
              <Button size="sm" onClick={onCreateClick}>
                <PlusSignIcon className="h-4 w-4" />
                {copy.createLabel}
              </Button>
              {/* CSV import has no screen yet; keep the entry point visible but inactive. */}
              <Button size="sm" variant="outline" disabled aria-describedby={`crm-import-${kind}-soon`}>
                <Upload01Icon className="h-4 w-4" />
                {copy.importLabel}
                <Badge id={`crm-import-${kind}-soon`} variant="secondary">Coming soon</Badge>
              </Button>
            </div>
          </div>
        </div>

        <div className="grid gap-4 lg:grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)]">
          <div className="rounded-lg border bg-card p-4">
            <div className="flex items-center gap-2">
              <div className="flex h-7 w-7 items-center justify-center rounded-md border bg-muted/40">
                <DatabaseIcon className="h-4 w-4 text-muted-foreground" />
              </div>
              <p className="text-sm font-medium">What to send</p>
            </div>
            <div className="mt-4 grid gap-3">
              {copy.points.map((point) => {
                const PointIcon = point.icon;
                return (
                  <div key={point.title} className="flex gap-3 rounded-md border bg-muted/20 p-3">
                    <div className="mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-md border bg-background">
                      <PointIcon className="h-3.5 w-3.5 text-muted-foreground" />
                    </div>
                    <div className="min-w-0">
                      <p className="text-sm font-medium">{point.title}</p>
                      <p className="mt-1 text-xs leading-5 text-muted-foreground">{point.description}</p>
                    </div>
                  </div>
                );
              })}
            </div>
          </div>

          <div className="overflow-hidden rounded-lg border bg-card">
            <div className="flex items-center justify-between gap-3 border-b bg-muted/20 px-4 py-3">
              <div className="flex items-center gap-2">
                <div className="flex h-7 w-7 items-center justify-center rounded-md border bg-background">
                  <CodeIcon className="h-4 w-4 text-muted-foreground" />
                </div>
                <div>
                  <p className="text-sm font-medium">Send CRM data with the SDK</p>
                  <p className="text-xs text-muted-foreground">Leads, customers, and companies</p>
                </div>
              </div>
            </div>
            <pre className="max-h-[430px] overflow-auto bg-background p-4 text-[11px] leading-5 text-muted-foreground">
              <code>{SDK_SNIPPET}</code>
            </pre>
          </div>
        </div>
      </div>
    </div>
  );
}

export function CRMNoResultsState({ kind, query, onClear }: CRMNoResultsStateProps) {
  return (
    <div className="flex h-full items-center justify-center p-6">
      <div className="max-w-md text-center">
        <div className="mx-auto flex h-11 w-11 items-center justify-center rounded-lg border bg-muted/40">
          <Search01Icon className="h-5 w-5 text-muted-foreground" />
        </div>
        <h2 className="mt-4 text-base font-semibold">No matching {kind}</h2>
        <p className="mt-2 text-sm leading-6 text-muted-foreground">
          No {kind} match &quot;{query}&quot;. Clear the search to return to the full list.
        </p>
        <Button size="sm" variant="outline" className="mt-4" onClick={onClear}>
          Clear search
        </Button>
      </div>
    </div>
  );
}
