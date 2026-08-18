import { useState } from 'react';
import { createFileRoute } from '@tanstack/react-router';
import { Button } from '@/components/ui/button';
import { BillingSettingsPreview } from '@/pages/settings/BillingSettingsPage';
import {
  founderBillingPreview,
  founderUsagePreview,
  growthBillingPreview,
  growthUsagePreview,
} from '@/mocks/billingPreview';

export const Route = createFileRoute('/billing-preview')({
  component: BillingPreviewRoute,
});

function BillingPreviewRoute() {
  const [plan, setPlan] = useState<'growth' | 'founder'>('growth');
  if (!import.meta.env.DEV) return <div className="p-8">This preview is available only in development.</div>;

  return (
    <div>
      <div className="fixed right-4 top-4 z-50 flex gap-2 rounded-lg border bg-background/95 p-2 shadow-lg backdrop-blur">
        <Button size="sm" variant={plan === 'growth' ? 'default' : 'outline'} onClick={() => setPlan('growth')}>
          Growth example
        </Button>
        <Button size="sm" variant={plan === 'founder' ? 'default' : 'outline'} onClick={() => setPlan('founder')}>
          Founder example
        </Button>
      </div>
      <BillingSettingsPreview
        billing={plan === 'growth' ? growthBillingPreview : founderBillingPreview}
        usage={plan === 'growth' ? growthUsagePreview : founderUsagePreview}
      />
    </div>
  );
}
