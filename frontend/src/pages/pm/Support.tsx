import { useTitle } from '@/hooks/useTitle';
import { SupportInboxLayout } from '@/components/support/SupportInboxLayout';

export function SupportPage() {
  useTitle('Support');
  return <SupportInboxLayout />;
}
