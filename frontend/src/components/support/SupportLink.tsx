import { useState, type AnchorHTMLAttributes, type MouseEvent as ReactMouseEvent, type ReactNode } from 'react';
import { AlertCircleIcon } from '@/lib/icons';
import type { SupportLinkSecurity } from '@/lib/pmTypes';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { Checkbox } from '@/components/ui/checkbox';

const THREAT_LABELS: Record<string, string> = {
  SOCIAL_ENGINEERING: 'Phishing or deceptive site',
  MALWARE: 'Malware',
  UNWANTED_SOFTWARE: 'Unwanted software',
};

interface SupportLinkProps extends Omit<AnchorHTMLAttributes<HTMLAnchorElement>, 'href' | 'security'> {
  href?: string;
  security?: SupportLinkSecurity;
  children: ReactNode;
  showInlineIndicator?: boolean;
}

interface SupportLinkWarningDialogProps {
  href: string;
  security: SupportLinkSecurity;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

/** Asks for confirmation before opening a link Google Web Risk flagged. */
export function SupportLinkWarningDialog({ href, security, open, onOpenChange }: SupportLinkWarningDialogProps) {
  const [acknowledged, setAcknowledged] = useState(false);

  const openAcknowledgedLink = () => {
    if (!acknowledged) return;
    window.open(href, '_blank', 'noopener,noreferrer');
  };

  return (
    <AlertDialog open={open} onOpenChange={(next) => { if (!next) setAcknowledged(false); onOpenChange(next); }}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Potentially harmful link</AlertDialogTitle>
          <AlertDialogDescription>
            Google Web Risk identified this destination as potentially harmful. Go back unless you recognize and trust it.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <div className="space-y-3">
          <div className="break-all rounded-md border border-red-200 bg-red-50 p-3 font-mono text-xs text-red-900 dark:border-red-900 dark:bg-red-950/30 dark:text-red-200">{href}</div>
          <ul className="list-disc space-y-1 pl-5 text-sm text-muted-foreground">
            {(security.threat_types ?? []).map((type) => <li key={type}>{THREAT_LABELS[type] ?? type}</li>)}
          </ul>
          <label className="flex cursor-pointer items-start gap-2 text-sm">
            <Checkbox checked={acknowledged} onCheckedChange={(checked) => setAcknowledged(checked === true)} />
            <span>I understand the risk and want to continue.</span>
          </label>
        </div>
        <AlertDialogFooter>
          <AlertDialogCancel>Go back</AlertDialogCancel>
          <AlertDialogAction disabled={!acknowledged} variant="destructive" onClick={openAcknowledgedLink}>Open link</AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}

export function SupportLink({ href, security, children, className = '', showInlineIndicator = true, ...props }: SupportLinkProps) {
  const [warningOpen, setWarningOpen] = useState(false);
  const isMalicious = security?.status === 'malicious';

  const handleClick = (event: ReactMouseEvent<HTMLAnchorElement>) => {
    if (!isMalicious || !href) return;
    event.preventDefault();
    setWarningOpen(true);
  };

  const indicator = showInlineIndicator && isMalicious ? (
    <span
      aria-label="Potentially harmful"
      title="Potentially harmful link"
      className="ml-0.5 inline-flex align-text-bottom text-red-600 dark:text-red-400"
    >
      <AlertCircleIcon className="h-3.5 w-3.5" aria-hidden="true" />
    </span>
  ) : null;

  return (
    <>
      <a
        {...props}
        href={href}
        title={href}
        target="_blank"
        rel="noopener noreferrer"
        onClick={handleClick}
        className={`${className} ${isMalicious ? 'text-red-600 decoration-red-400 dark:text-red-400' : ''}`.trim()}
      >
        {children}
        {indicator}
      </a>
      {isMalicious && href ? (
        <SupportLinkWarningDialog href={href} security={security} open={warningOpen} onOpenChange={setWarningOpen} />
      ) : null}
    </>
  );
}
