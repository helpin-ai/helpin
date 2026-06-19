import { useMemo, useState } from 'react';
import { toast } from 'sonner';
import { useTitle } from '@/hooks/useTitle';
import { useOrganizationStore } from '@/stores/organizationStore';
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from '@/components/ui/tabs';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Skeleton } from '@/components/ui/skeleton';
import {
  useOrgBilling,
  useBillingCards,
  useBillingInvoices,
  useOrganizationMembers,
  useCreateSetupIntent,
} from '@/hooks/queries';
import type { PaymentMethod, WorkspaceBillingCard as WSCard } from '@/lib/billingTypes';
import { BillingSummaryHeader } from '@/components/billing/BillingSummaryHeader';
import { WorkspaceBillingCard } from '@/components/billing/WorkspaceBillingCard';
import { PaymentMethodCard } from '@/components/billing/PaymentMethodCard';
import { InvoicesTable } from '@/components/billing/InvoicesTable';
import { UsageDetail } from '@/components/billing/UsageDetail';
import { PlanChangeModal } from '@/components/billing/PlanChangeModal';

export default function OrganizationBilling() {
  useTitle('Billing');
  const orgId = useOrganizationStore((s) => s.currentOrganization?.id);

  const { data: billing, isLoading } = useOrgBilling(orgId);
  const { data: cards = [] } = useBillingCards(orgId);
  const { data: invoices = [] } = useBillingInvoices(orgId);
  const { data: members = [] } = useOrganizationMembers(orgId);
  const setupIntent = useCreateSetupIntent(orgId);

  const [manageCard, setManageCard] = useState<WSCard | null>(null);
  const [planCard, setPlanCard] = useState<WSCard | null>(null);
  const [search, setSearch] = useState('');

  const orgDefaultCardId = useMemo(
    () => cards.find((c) => c.is_org_default)?.id ?? null,
    [cards],
  );

  // The org owner can reassign billing owners; delegated managers cannot.
  const canManageOwner = useOrganizationStore(
    (s) => s.currentOrganization?.role === 'owner',
  );

  const handleAddCard = () => {
    // Card capture happens via Stripe. We start a SetupIntent; the Stripe
    // Elements/Checkout collection UI is handled server-side / via redirect.
    setupIntent.mutate(undefined, {
      onError: (e) => toast.error(e instanceof Error ? e.message : 'Failed to start card setup'),
      onSuccess: () => toast.info('Continue card entry in the Stripe form'),
    });
  };

  if (isLoading || !billing) {
    return (
      <div className="space-y-4">
        <Skeleton className="h-24 w-full" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }

  const workspaces = billing.workspaces.filter((w) =>
    search ? w.workspace_name.toLowerCase().includes(search.toLowerCase()) : true,
  );

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-lg font-semibold">Billing</h1>
        <p className="text-sm text-muted-foreground">
          Manage plans, cards, and usage across all workspaces in this organization.
        </p>
      </div>

      <BillingSummaryHeader summary={billing} />

      <Tabs defaultValue="plans">
        <TabsList>
          <TabsTrigger value="plans">Plans &amp; Subscriptions</TabsTrigger>
          <TabsTrigger value="cards">Cards</TabsTrigger>
          <TabsTrigger value="invoices">Invoices</TabsTrigger>
        </TabsList>

        {/* Plans & Subscriptions */}
        <TabsContent value="plans" className="space-y-4">
          {billing.workspaces.length > 12 && (
            <Input
              placeholder="Search workspaces…"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="max-w-xs"
            />
          )}
          <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
            {workspaces.map((card) => (
              <WorkspaceBillingCard
                key={card.workspace_id}
                orgId={orgId!}
                card={card}
                cards={cards}
                members={members}
                canManageOwner={canManageOwner}
                onManage={setManageCard}
                onChangePlan={setPlanCard}
                onAddCard={handleAddCard}
              />
            ))}
          </div>
          {workspaces.length === 0 && (
            <div className="rounded-xl border bg-card p-8 text-center text-sm text-muted-foreground">
              No workspaces found.
            </div>
          )}
        </TabsContent>

        {/* Cards */}
        <TabsContent value="cards" className="space-y-3">
          <div className="flex justify-end">
            <Button size="sm" onClick={handleAddCard} disabled={setupIntent.isPending}>
              Add payment method
            </Button>
          </div>
          {cards.length === 0 ? (
            <div className="rounded-xl border bg-card p-8 text-center text-sm text-muted-foreground">
              No saved cards yet.
            </div>
          ) : (
            cards.map((card: PaymentMethod) => (
              <PaymentMethodCard key={card.id} orgId={orgId!} card={card} />
            ))
          )}
        </TabsContent>

        {/* Invoices */}
        <TabsContent value="invoices">
          <InvoicesTable invoices={invoices} />
        </TabsContent>
      </Tabs>

      {/* Usage detail dialog */}
      <Dialog open={!!manageCard} onOpenChange={(o) => !o && setManageCard(null)}>
        <DialogContent className="max-h-[85vh] overflow-y-auto sm:max-w-3xl">
          <DialogHeader>
            <DialogTitle>{manageCard?.workspace_name} · Usage</DialogTitle>
          </DialogHeader>
          {manageCard && <UsageDetail workspaceId={manageCard.workspace_id} />}
        </DialogContent>
      </Dialog>

      {/* Plan change modal */}
      {planCard && (
        <PlanChangeModal
          open={!!planCard}
          onOpenChange={(o) => !o && setPlanCard(null)}
          workspaceId={planCard.workspace_id}
          workspaceName={planCard.workspace_name}
          currentPlan={planCard.plan}
          cards={cards}
          defaultCardId={orgDefaultCardId}
        />
      )}
    </div>
  );
}
