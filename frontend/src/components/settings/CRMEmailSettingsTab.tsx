import { EmailAccountConnect } from '@/components/crm/EmailAccountConnect';
import { useAuthStore } from '@/stores/authStore';

export function CRMEmailSettingsTab({ workspaceId }: { workspaceId: string }) {
  const user = useAuthStore((s) => s.user);
  return <EmailAccountConnect workspaceId={workspaceId} memberId={user?.id ?? ''} />;
}
