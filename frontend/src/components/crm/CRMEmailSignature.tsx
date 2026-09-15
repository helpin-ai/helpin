import { useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { QuietPrimaryAction } from '@/components/design-system/quiet';
import { CRMEmailSettingsSection } from './CRMEmailSettingsSection';
import { Textarea } from '@/components/ui/textarea';
import { crmEmailService } from '@/lib/services/crmService';
import { unwrap } from '@/lib/queryUtils';
import type { CRMEmailAccount } from '@/lib/crmTypes';

export function CRMEmailSignatureSettings({ workspaceId, account }: { workspaceId: string; account: CRMEmailAccount }) {
  const queryClient = useQueryClient();
  const [signature, setSignature] = useState(account.signature ?? '');
  const [saved, setSaved] = useState(account.signature ?? '');
  const [saving, setSaving] = useState(false);
  return <CRMEmailSettingsSection title="Email signature">
    <div className="mt-3 space-y-2">
      <Textarea aria-label={`Signature for ${account.email_address}`} value={signature} onChange={(event) => setSignature(event.target.value)} placeholder="Your name\nRole · Company\nPhone or website" rows={4} maxLength={10000} disabled={saving} />
      <div className="flex justify-end"><QuietPrimaryAction disabled={saving || signature === saved} onClick={async () => {
        setSaving(true);
        try {
          unwrap(await crmEmailService.updateSignature(workspaceId, account.id, signature));
          setSaved(signature);
          await queryClient.invalidateQueries({ queryKey: ['crm', workspaceId] });
          toast.success('Signature saved');
        } catch (error) { toast.error(error instanceof Error ? error.message : 'Could not save signature'); }
        finally { setSaving(false); }
      }}>{saving ? 'Saving…' : 'Save signature'}</QuietPrimaryAction></div>
    </div>
  </CRMEmailSettingsSection>;
}
