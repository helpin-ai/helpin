import { useCallback } from 'react';
import { createFileRoute, useNavigate } from '@tanstack/react-router';
import { BillingSettingsPage } from '@/pages/settings/BillingSettingsPage';
import { SettingsRouteViewport } from '@/pages/settings/SettingsRouteViewport';
import {
  BILLING_CHOOSE_PLAN_SEARCH,
  BILLING_OVERVIEW_SEARCH,
  shouldOpenBillingPlanChooser,
} from '@/lib/billingNavigation';

export const Route = createFileRoute('/_authenticated/w/$slug/settings/billing')({
  validateSearch: (search) => ({
    choose_plan: shouldOpenBillingPlanChooser(search) || undefined,
  }),
  component: BillingSettingsRoute,
});

function BillingSettingsRoute() {
  const search = Route.useSearch();
  const navigate = useNavigate({ from: Route.fullPath });

  const handlePlanChooserChange = useCallback((open: boolean) => {
    void navigate({
      to: Route.fullPath,
      search: open ? BILLING_CHOOSE_PLAN_SEARCH : BILLING_OVERVIEW_SEARCH,
      replace: true,
    });
  }, [navigate]);

  return (
    <SettingsRouteViewport>
      <BillingSettingsPage
        openPlanChooser={search.choose_plan}
        onPlanChooserChange={handlePlanChooserChange}
      />
    </SettingsRouteViewport>
  );
}
