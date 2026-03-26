import { useState, type FormEvent } from 'react';
import { Link, useSearch } from '@tanstack/react-router';
import { useTitle } from '@/hooks/useTitle';
import { authService } from '@/lib/services/authService';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card';
import { PublicPageShell } from '@/components/layout/PublicPageShell';
import { toast } from 'sonner';

export default function ResetPassword() {
  useTitle('Reset Password');
  const { token } = useSearch({ strict: false }) as { token?: string };
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const [success, setSuccess] = useState(false);

  if (!token) {
    return (
      <PublicPageShell>
        <Card className="w-full">
          <CardHeader className="text-center">
            <CardTitle className="text-2xl">Invalid link</CardTitle>
            <CardDescription>This password reset link is invalid or has expired.</CardDescription>
          </CardHeader>
          <CardFooter className="flex flex-col gap-4 mt-4">
            <Link to="/forgot-password">
              <Button variant="outline" className="w-full">Request a new link</Button>
            </Link>
          </CardFooter>
        </Card>
      </PublicPageShell>
    );
  }

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (password.length < 8) {
      toast.error('Password must be at least 8 characters');
      return;
    }
    if (password !== confirmPassword) {
      toast.error('Passwords do not match');
      return;
    }
    setLoading(true);
    const { error } = await authService.resetPassword(token, password);
    setLoading(false);
    if (error) {
      toast.error(error);
      return;
    }
    setSuccess(true);
  };

  if (success) {
    return (
      <PublicPageShell>
        <Card className="w-full">
          <CardHeader className="text-center">
            <CardTitle className="text-2xl">Password reset</CardTitle>
            <CardDescription>Your password has been reset successfully. You can now sign in with your new password.</CardDescription>
          </CardHeader>
          <CardFooter className="flex flex-col gap-4 mt-4">
            <Link to="/login" className="w-full">
              <Button className="w-full">Sign in</Button>
            </Link>
          </CardFooter>
        </Card>
      </PublicPageShell>
    );
  }

  return (
    <PublicPageShell>
      <Card className="w-full">
        <CardHeader className="text-center">
          <CardTitle className="text-2xl">Set a new password</CardTitle>
          <CardDescription>Choose a new password for your account</CardDescription>
        </CardHeader>
        <form onSubmit={handleSubmit}>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="password">New password</Label>
              <Input id="password" type="password" placeholder="••••••••" value={password} onChange={e => setPassword(e.target.value)} required minLength={8} />
            </div>
            <div className="space-y-2">
              <Label htmlFor="confirm">Confirm password</Label>
              <Input id="confirm" type="password" placeholder="••••••••" value={confirmPassword} onChange={e => setConfirmPassword(e.target.value)} required minLength={8} />
            </div>
          </CardContent>
          <CardFooter className="flex flex-col gap-4 mt-4">
            <Button type="submit" className="w-full" disabled={loading}>
              {loading ? 'Resetting...' : 'Reset password'}
            </Button>
          </CardFooter>
        </form>
      </Card>
    </PublicPageShell>
  );
}
