import { useEffect, useState } from 'react';
import { Link, useSearch } from '@tanstack/react-router';
import { useTitle } from '@/hooks/useTitle';
import { authService } from '@/lib/services/authService';
import { Button } from '@/components/ui/button';
import { Card, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card';
import { Loader2 } from 'lucide-react';

export default function VerifyEmail() {
  useTitle('Verify Email');
  const { token } = useSearch({ strict: false }) as { token?: string };
  const [status, setStatus] = useState<'loading' | 'success' | 'error'>('loading');
  const [errorMessage, setErrorMessage] = useState('');

  useEffect(() => {
    if (!token) {
      setStatus('error');
      setErrorMessage('Invalid verification link.');
      return;
    }

    authService.verifyEmail(token).then(({ error }) => {
      if (error) {
        setStatus('error');
        setErrorMessage(error);
      } else {
        setStatus('success');
      }
    });
  }, [token]);

  return (
    <div className="min-h-screen flex items-center justify-center px-4">
      <Card className="w-full max-w-md">
        {status === 'loading' && (
          <CardHeader className="text-center">
            <div className="flex justify-center mb-4">
              <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
            </div>
            <CardTitle className="text-2xl">Verifying your email...</CardTitle>
            <CardDescription>Please wait while we verify your email address.</CardDescription>
          </CardHeader>
        )}

        {status === 'success' && (
          <>
            <CardHeader className="text-center">
              <CardTitle className="text-2xl">Email verified</CardTitle>
              <CardDescription>Your email has been verified successfully.</CardDescription>
            </CardHeader>
            <CardFooter className="flex flex-col gap-4 mt-4">
              <Link to="/login" className="w-full">
                <Button className="w-full">Continue to sign in</Button>
              </Link>
            </CardFooter>
          </>
        )}

        {status === 'error' && (
          <>
            <CardHeader className="text-center">
              <CardTitle className="text-2xl">Verification failed</CardTitle>
              <CardDescription>{errorMessage || 'This verification link is invalid or has expired.'}</CardDescription>
            </CardHeader>
            <CardFooter className="flex flex-col gap-4 mt-4">
              <Link to="/login" className="w-full">
                <Button variant="outline" className="w-full">Back to sign in</Button>
              </Link>
            </CardFooter>
          </>
        )}
      </Card>
    </div>
  );
}
