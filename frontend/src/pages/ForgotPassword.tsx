import { useState, type FormEvent } from 'react';
import { Link } from '@tanstack/react-router';
import { useTitle } from '@/hooks/useTitle';
import { authService } from '@/lib/services/authService';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card';
import { PublicPageShell } from '@/components/layout/PublicPageShell';
import { toast } from 'sonner';

export default function ForgotPassword() {
  useTitle('Forgot Password');
  const [email, setEmail] = useState('');
  const [submittedEmail, setSubmittedEmail] = useState('');
  const [loading, setLoading] = useState(false);
  const [sent, setSent] = useState(false);

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    const normalizedEmail = email.trim().toLowerCase();
    if (!normalizedEmail) {
      toast.error('Email is required');
      return;
    }
    setLoading(true);
    const { error } = await authService.forgotPassword(normalizedEmail);
    setLoading(false);
    if (error) {
      toast.error(error);
      return;
    }
    setSubmittedEmail(normalizedEmail);
    setSent(true);
  };

  if (sent) {
    return (
      <PublicPageShell>
        <Card className="w-full">
          <CardHeader className="text-center">
            <CardTitle role="heading" aria-level={1} className="text-2xl">Check your email</CardTitle>
            <CardDescription>
              If an account with <strong>{submittedEmail}</strong> exists, we've sent a password reset link.
            </CardDescription>
          </CardHeader>
          <CardFooter className="flex flex-col gap-4 mt-4">
            <Button variant="outline" className="w-full" onClick={() => setSent(false)}>
              Try another email
            </Button>
            <p className="text-sm text-muted-foreground text-center">
              The link expires in 1 hour. If you requested more than once, use the newest email.
            </p>
            <p className="text-sm text-muted-foreground">
              <Link to="/login" className="text-primary hover:underline">Back to sign in</Link>
            </p>
          </CardFooter>
        </Card>
      </PublicPageShell>
    );
  }

  return (
    <PublicPageShell>
      <Card className="w-full">
        <CardHeader className="text-center">
          <CardTitle role="heading" aria-level={1} className="text-2xl">Forgot your password?</CardTitle>
          <CardDescription>Enter your email and we'll send you a reset link</CardDescription>
        </CardHeader>
        <form onSubmit={handleSubmit}>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="email">Email</Label>
              <Input id="email" type="email" placeholder="you@example.com" autoComplete="email" value={email} onChange={e => setEmail(e.target.value)} required />
            </div>
          </CardContent>
          <CardFooter className="flex flex-col gap-4 mt-4">
            <Button type="submit" className="w-full" disabled={loading}>
              {loading ? 'Sending...' : 'Send reset link'}
            </Button>
            <p className="text-sm text-muted-foreground">
              Remember your password? <Link to="/login" className="text-primary hover:underline">Sign in</Link>
            </p>
          </CardFooter>
        </form>
      </Card>
    </PublicPageShell>
  );
}
