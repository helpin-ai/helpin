import { useEffect, useState } from 'react';
import { useNavigate, useParams } from '@tanstack/react-router';
import { useTitle } from '@/hooks/useTitle';
import { useAuthStore } from '@/stores/authStore';
import { useQueryClient } from '@tanstack/react-query';
import { inviteService } from '@/lib/services/inviteService';
import type { InviteInfo } from '@/lib/types';
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { toast } from 'sonner';

export default function JoinWorkspace() {
  useTitle('Join Workspace');
  const { token } = useParams({ from: '/join/$token' });
  const navigate = useNavigate();
  const { user } = useAuthStore();
  const queryClient = useQueryClient();

  const [info, setInfo] = useState<InviteInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [joining, setJoining] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [fullName, setFullName] = useState('');
  const [password, setPassword] = useState('');
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    const fetchInfo = async () => {
      const { data, error: err } = await inviteService.getInfo(token);
      if (err || !data) {
        setError(err || 'Invitation not found');
      } else {
        setInfo(data);
      }
      setLoading(false);
    };
    fetchInfo();
  }, [token]);

  const handleJoin = async () => {
    setJoining(true);
    const { error: err } = await inviteService.accept(token);
    if (err) {
      toast.error(err);
      setJoining(false);
      return;
    }
    toast.success(`Joined ${info?.workspace_name}!`);
    await queryClient.invalidateQueries({ queryKey: ['workspaces'] });
    navigate({ to: '/w/$slug/pm/stories', params: { slug: info?.workspace_slug ?? '' } });
  };

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center px-4">
        <Card className="w-full max-w-md">
          <CardHeader className="text-center">
            <Skeleton className="h-8 w-48 mx-auto" />
            <Skeleton className="h-4 w-64 mx-auto mt-2" />
          </CardHeader>
          <CardContent>
            <Skeleton className="h-10 w-full" />
          </CardContent>
        </Card>
      </div>
    );
  }

  // Invalid / expired / error state
  if (error || !info || info.expired || info.status !== 'pending') {
    const message = error
      || (info?.expired ? 'This invitation has expired.' : null)
      || (info?.status === 'accepted' ? 'This invitation has already been accepted.' : null)
      || (info?.status === 'revoked' ? 'This invitation has been revoked.' : null)
      || 'This invitation is no longer valid.';

    return (
      <div className="min-h-screen flex items-center justify-center px-4">
        <Card className="w-full max-w-md">
          <CardHeader className="text-center">
            <CardTitle className="text-2xl">Invalid Invitation</CardTitle>
            <CardDescription>{message}</CardDescription>
          </CardHeader>
          <CardFooter className="justify-center">
            <Button variant="outline" onClick={() => navigate({ to: '/login' })}>
              Go to sign in
            </Button>
          </CardFooter>
        </Card>
      </div>
    );
  }

  // Not logged in — inline registration form
  if (!user) {
    const handleSignupAndJoin = async (e: React.FormEvent) => {
      e.preventDefault();
      if (password.length < 8) {
        toast.error('Password must be at least 8 characters');
        return;
      }
      setSubmitting(true);
      const { data, error: err } = await inviteService.acceptWithSignup(token, password, fullName);
      if (err || !data) {
        toast.error(err || 'Failed to create account');
        setSubmitting(false);
        return;
      }
      localStorage.setItem('access_token', data.access_token);
      localStorage.setItem('refresh_token', data.refresh_token);
      useAuthStore.setState({ user: data.user, serverUnreachable: false });
      toast.success(`Welcome to ${info?.workspace_name}!`);
      await queryClient.invalidateQueries({ queryKey: ['workspaces'] });
      navigate({ to: '/w/$slug/pm/stories', params: { slug: data.workspace_slug } });
    };

    return (
      <div className="min-h-screen flex items-center justify-center px-4">
        <Card className="w-full max-w-md">
          <CardHeader className="text-center">
            <CardTitle className="text-2xl">Join {info.workspace_name}</CardTitle>
            <CardDescription>
              {info.invited_by_name} invited you to join as <Badge variant="secondary" className="ml-1">{info.role}</Badge>
            </CardDescription>
          </CardHeader>
          <form onSubmit={handleSignupAndJoin}>
            <CardContent className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="email">Email</Label>
                <Input id="email" type="email" value={info.email} disabled className="bg-muted" />
              </div>
              <div className="space-y-2">
                <Label htmlFor="fullName">Full Name</Label>
                <Input
                  id="fullName"
                  value={fullName}
                  onChange={(e) => setFullName(e.target.value)}
                  placeholder="Enter your full name"
                  required
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="password">Password</Label>
                <Input
                  id="password"
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder="At least 8 characters"
                  minLength={8}
                  required
                />
              </div>
            </CardContent>
            <CardFooter className="flex flex-col gap-3">
              <Button type="submit" className="w-full" disabled={submitting}>
                {submitting ? 'Creating account...' : `Create account & join ${info.workspace_name}`}
              </Button>
              <p className="text-sm text-muted-foreground">
                Already have an account?{' '}
                <button
                  type="button"
                  className="text-primary hover:underline cursor-pointer"
                  onClick={() => navigate({ to: `/login?redirect=/join/${token}` as string })}
                >
                  Sign in
                </button>
              </p>
            </CardFooter>
          </form>
        </Card>
      </div>
    );
  }

  // Logged in — show join button
  return (
    <div className="min-h-screen flex items-center justify-center px-4">
      <Card className="w-full max-w-md">
        <CardHeader className="text-center">
          <CardTitle className="text-2xl">Join {info.workspace_name}</CardTitle>
          <CardDescription>
            {info.invited_by_name} invited you to join as <Badge variant="secondary" className="ml-1">{info.role}</Badge>
          </CardDescription>
        </CardHeader>
        <CardContent className="text-center text-sm text-muted-foreground">
          <p>You're signed in as <span className="font-medium">{user.email}</span></p>
        </CardContent>
        <CardFooter className="justify-center">
          <Button className="w-full" onClick={handleJoin} disabled={joining}>
            {joining ? 'Joining...' : 'Join Workspace'}
          </Button>
        </CardFooter>
      </Card>
    </div>
  );
}
