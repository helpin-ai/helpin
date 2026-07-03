import { useState } from 'react';
import { MailCheck } from 'lucide-react';
import { toast } from 'sonner';

import { Button } from '@/components/ui/button';
import { authService } from '@/lib/services/authService';

interface EmailVerificationBannerProps {
  emailVerified?: boolean;
}

export function EmailVerificationBanner({ emailVerified }: EmailVerificationBannerProps) {
  const [resending, setResending] = useState(false);

  if (emailVerified !== false) {
    return null;
  }

  const handleResend = async () => {
    setResending(true);
    try {
      const { error } = await authService.resendVerification();
      if (error) {
        toast.error(error);
        return;
      }
      toast.success('Verification email sent');
    } finally {
      setResending(false);
    }
  };

  return (
    <div className="sticky top-0 z-50 border-b border-amber-200 bg-amber-50 px-4 py-2 text-amber-950 shadow-sm">
      <div className="mx-auto flex max-w-screen-2xl items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-2 text-sm">
          <MailCheck className="h-4 w-4 shrink-0" aria-hidden="true" />
          <span className="truncate">Verify your email to secure your account.</span>
        </div>
        <Button
          type="button"
          size="sm"
          variant="outline"
          className="h-8 shrink-0 border-amber-300 bg-white text-amber-950 hover:bg-amber-100"
          disabled={resending}
          onClick={() => void handleResend()}
        >
          {resending ? 'Sending...' : 'Resend email'}
        </Button>
      </div>
    </div>
  );
}
