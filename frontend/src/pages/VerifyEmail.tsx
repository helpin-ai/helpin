import { useEffect, useState } from 'react';
import { Link, useSearch } from '@tanstack/react-router';
import { useTitle } from '@/hooks/useTitle';
import { authService } from '@/lib/services/authService';
import { useAuthStore } from '@/stores/authStore';
import { Button } from '@/components/ui/button';
import { Card, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card';
import { PublicPageShell } from '@/components/layout/PublicPageShell';
import { Loading01Icon } from '@/lib/icons';

export default function VerifyEmail() {
  useTitle('Verify Email');
  const { token } = useSearch({ strict: false }) as { token?: string };
  const user = useAuthStore((state) => state.user);
  const [status, setStatus] = useState<'loading' | 'success' | 'error'>('loading');
  const [errorMessage, setErrorMessage] = useState('');

  useEffect(() => {
    if (!token) {
      setStatus('error');
      setErrorMessage('Invalid verification link.');
      return;
    }

    authService.verifyEmail(token).then(({ data, error }) => {
      if (error) {
        setStatus('error');
        setErrorMessage(error);
      } else {
        if (data && user?.id === data.id) {
          useAuthStore.setState({ user: data });
        }
        setStatus('success');
      }
    });
  }, [token, user?.id]);

  return (
    <PublicPageShell>
      <Card className="w-full">
        {status === 'loading' && (
          <CardHeader className="space-y-4 text-center">
            <div className="flex justify-center">
              <Loading01Icon className="h-8 w-8 animate-spin text-muted-foreground" />
            </div>
            <div className="space-y-1">
              <CardTitle role="heading" aria-level={1} className="text-2xl">Verifying your email...</CardTitle>
              <CardDescription>Please wait while we verify your email address.</CardDescription>
            </div>
          </CardHeader>
        )}

        {status === 'success' && (
          <>
            <CardHeader className="text-center">
              <CardTitle role="heading" aria-level={1} className="text-2xl">Email verified</CardTitle>
              <CardDescription>Your email has been verified successfully.</CardDescription>
            </CardHeader>
            <CardFooter className="flex flex-col gap-4 mt-4">
              <Button asChild className="w-full">
                <Link to={user ? '/workspaces' : '/login'}>{user ? 'Continue' : 'Continue to sign in'}</Link>
              </Button>
            </CardFooter>
          </>
        )}

        {status === 'error' && (
          <>
            <CardHeader className="text-center">
              <CardTitle role="heading" aria-level={1} className="text-2xl">Verification failed</CardTitle>
              <CardDescription>{errorMessage || 'This verification link is invalid or has expired.'}</CardDescription>
            </CardHeader>
            <CardFooter className="flex flex-col gap-4 mt-4">
              <Button asChild variant="outline" className="w-full">
                <Link to="/login">Back to sign in</Link>
              </Button>
            </CardFooter>
          </>
        )}
      </Card>
    </PublicPageShell>
  );
}
