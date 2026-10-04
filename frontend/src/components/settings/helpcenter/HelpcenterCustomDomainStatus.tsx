import { useState } from 'react';
import { formatDistanceToNow } from 'date-fns';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import type { DocsHelpcenterConfig } from '@/lib/docsTypes';
import { cn } from '@/lib/utils';

export type HelpcenterDomainVerification = Pick<
  DocsHelpcenterConfig,
  | 'custom_domain'
  | 'custom_domain_status'
  | 'custom_domain_checked_at'
  | 'custom_domain_last_error'
  | 'custom_domain_failing_since'
  | 'custom_domain_target'
  | 'custom_domain_challenge_name'
  | 'custom_domain_challenge_value'
>;

const STATUS: Record<NonNullable<HelpcenterDomainVerification['custom_domain_status']>, { label: string; className: string }> = {
  verified: { label: 'Live', className: 'text-emerald-700 dark:text-emerald-400' },
  pending: { label: 'Waiting for DNS', className: 'text-amber-700 dark:text-amber-400' },
  failing: { label: 'Not pointing to Helpin', className: 'text-destructive' },
};

/**
 * HelpcenterCustomDomainStatus shows whether the saved custom domain is live
 * and the two DNS records it needs: a CNAME that routes it to Helpin, and a
 * TXT record that proves this workspace owns it.
 */
export function HelpcenterCustomDomainStatus({ verification, editedDomain, editable, onCheck }: {
  verification: HelpcenterDomainVerification | null;
  editedDomain: string;
  editable: boolean;
  onCheck: () => Promise<HelpcenterDomainVerification | null>;
}) {
  const [checking, setChecking] = useState(false);
  const target = verification?.custom_domain_target;
  const saved = verification?.custom_domain ?? '';
  const edited = editedDomain.trim().toLowerCase();

  if (!target) {
    return (
      <p className="text-[11px] leading-relaxed text-muted-foreground">
        Point your domain at this help center with a <span className="font-mono text-foreground">CNAME</span> record. SSL certificates are issued automatically once DNS resolves.
      </p>
    );
  }
  if (!edited) return null;
  if (edited !== saved) {
    return <p className="text-[11px] text-muted-foreground">Save to get the DNS records for {edited}.</p>;
  }

  const status = verification?.custom_domain_status ?? 'pending';
  const meta = STATUS[status];
  const checkedAt = verification?.custom_domain_checked_at;
  const check = async () => {
    setChecking(true);
    try {
      const next = await onCheck();
      if (next?.custom_domain_status === 'verified') toast.success(`${saved} is live`);
    } finally {
      setChecking(false);
    }
  };

  return (
    <div className="space-y-3 rounded-md border border-border/60 bg-muted/40 p-3 text-[11px] leading-relaxed text-muted-foreground" data-testid="hc-domain-status">
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
        <span className={cn('text-xs font-medium', meta.className)}>{meta.label}</span>
        {checkedAt ? <span>Checked {formatDistanceToNow(new Date(checkedAt), { addSuffix: true })}</span> : null}
        {editable && status !== 'verified' ? (
          <Button type="button" size="xs" variant="outline" className="ml-auto" disabled={checking} onClick={() => void check()}>
            {checking ? 'Checking…' : 'Check DNS'}
          </Button>
        ) : null}
      </div>
      {status !== 'verified' && verification?.custom_domain_last_error ? (
        <p role="status" className={cn(status === 'failing' && 'text-destructive')}>{verification.custom_domain_last_error}</p>
      ) : null}
      {status === 'failing' ? (
        <p>Your help center and customer portal keep being served on {saved}, but visitors may not reach them until DNS points to Helpin again.</p>
      ) : null}
      <div>
        <p className="mb-1.5 font-medium text-foreground">{status === 'verified' ? 'DNS records' : 'Add these DNS records'}</p>
        <div className="overflow-x-auto">
          <table className="w-full min-w-[420px] border-collapse text-left">
            <thead>
              <tr className="border-b border-border/60">
                <th className="py-1 pr-3 font-medium">Type</th>
                <th className="py-1 pr-3 font-medium">Name</th>
                <th className="py-1 font-medium">Value</th>
              </tr>
            </thead>
            <tbody className="font-mono text-foreground">
              <tr className="border-b border-border/40">
                <td className="py-1 pr-3">CNAME</td>
                <td className="py-1 pr-3 break-all">{saved}</td>
                <td className="py-1 break-all">{target}</td>
              </tr>
              {verification?.custom_domain_challenge_name ? (
                <tr>
                  <td className="py-1 pr-3">TXT</td>
                  <td className="py-1 pr-3 break-all">{verification.custom_domain_challenge_name}</td>
                  <td className="py-1 break-all">{verification.custom_domain_challenge_value}</td>
                </tr>
              ) : null}
            </tbody>
          </table>
        </div>
        <p className="mt-1.5">
          The CNAME routes visitors to Helpin, and the TXT record proves the domain is yours. It goes live once both resolve, usually within a few minutes. SSL certificates are issued automatically.
        </p>
      </div>
    </div>
  );
}
