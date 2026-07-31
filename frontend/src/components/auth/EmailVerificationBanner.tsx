import { useEffect, useState } from 'react';
import { MailCheck } from 'lucide-react';
import { toast } from 'sonner';

import { Button } from '@/components/ui/button';
import { authService } from '@/lib/services/authService';

interface EmailVerificationBannerProps {
  emailVerified?: boolean;
  resendCooldownSeconds?: number;
}

export function EmailVerificationBanner({ emailVerified, resendCooldownSeconds = 60 }: EmailVerificationBannerProps) {
  const [resending, setResending] = useState(false);
  const [cooldownSeconds, setCooldownSeconds] = useState(0);

  useEffect(() => {
    if (cooldownSeconds <= 0) {
      return;
    }
    const intervalId = window.setInterval(() => {
      setCooldownSeconds((current) => Math.max(0, current - 1));
    }, 1000);
    return () => window.clearInterval(intervalId);
  }, [cooldownSeconds]);

  if (emailVerified !== false) {
    return null;
  }

  const handleResend = async () => {
    if (resending || cooldownSeconds > 0) {
      return;
    }
    setResending(true);
    try {
      const { error } = await authService.resendVerification();
      if (error) {
        toast.error(error);
        return;
      }
      setCooldownSeconds(resendCooldownSeconds);
      toast.success('Verification email sent. Check your inbox.');
    } finally {
      setResending(false);
    }
  };

  return (
    <div className="sticky top-0 z-50 border-b border-amber-200 bg-amber-50 px-4 py-2 text-amber-950 shadow-sm">
      <div className="mx-auto flex max-w-screen-2xl items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-2 text-sm">
          <MailCheck className="h-4 w-4 shrink-0" aria-hidden="true" />
          <span className="min-w-0">
            <span className="font-medium">We sent a verification email.</span>{' '}
            <span className="text-amber-900">Open it and click the verification link to finish securing your account.</span>
          </span>
        </div>
        <Button
          type="button"
          size="sm"
          variant="outline"
          className="h-8 shrink-0 border-amber-300 bg-white text-amber-950 hover:bg-amber-100"
          disabled={resending || cooldownSeconds > 0}
          onClick={() => void handleResend()}
        >
          {resending ? 'Sending...' : cooldownSeconds > 0 ? `Resend in ${cooldownSeconds}s` : 'Resend email'}
        </Button>
      </div>
    </div>
  );
}
