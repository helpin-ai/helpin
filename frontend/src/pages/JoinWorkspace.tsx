import { useEffect, useState } from 'react';
import { useNavigate, useParams } from '@tanstack/react-router';
import { useTitle } from '@/hooks/useTitle';
import { clearClientSession, useAuthStore } from '@/stores/authStore';
import { useQueryClient } from '@tanstack/react-query';
import { inviteService } from '@/lib/services/inviteService';
import type { InviteInfo } from '@/lib/types';
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { PublicPageShell } from '@/components/layout/PublicPageShell';
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
  const inviteMatchesCurrentUser = user && info ? user.email.toLowerCase() === info.email.toLowerCase() : false;

  const loginRedirect = `/login?redirect=/join/${token}` as string;

  const handleSwitchAccount = () => {
    clearClientSession();
    useAuthStore.setState({ user: null, loading: false, serverUnreachable: false });
    window.location.assign(loginRedirect);
  };

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
    navigate({ to: '/w/$slug/pm/my-work', params: { slug: info?.workspace_slug ?? '' } });
  };

  if (loading) {
    return (
      <PublicPageShell>
        <Card className="w-full">
          <CardHeader className="text-center">
            <Skeleton className="h-8 w-48 mx-auto" />
            <Skeleton className="h-4 w-64 mx-auto mt-2" />
          </CardHeader>
          <CardContent>
            <Skeleton className="h-10 w-full" />
          </CardContent>
        </Card>
      </PublicPageShell>
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
      <PublicPageShell>
        <Card className="w-full">
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
      </PublicPageShell>
    );
  }

  // Not logged in + invited email already has an account.
  if (!user) {
    if (info.account_exists) {
      return (
        <PublicPageShell>
          <Card className="w-full">
            <CardHeader className="text-center">
              <CardTitle className="text-2xl">Sign In To Join {info.workspace_name}</CardTitle>
              <CardDescription>
                {info.invited_by_name} invited <span className="font-medium text-foreground">{info.email}</span> to join as <Badge variant="secondary" className="ml-1">{info.role}</Badge>
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-3 text-center text-sm text-muted-foreground">
              <p>This invite is linked to an existing Helpin account.</p>
              <p>Sign in with <span className="font-medium text-foreground">{info.email}</span> to accept it.</p>
            </CardContent>
            <CardFooter className="flex flex-col gap-3">
              <Button className="w-full" onClick={() => navigate({ to: loginRedirect })}>
                Sign in to continue
              </Button>
              <p className="text-sm text-muted-foreground">
                Need password help?{' '}
                <button
                  type="button"
                  className="text-primary hover:underline cursor-pointer"
                  onClick={() => navigate({ to: '/forgot-password' })}
                >
                  Reset password
                </button>
              </p>
            </CardFooter>
          </Card>
        </PublicPageShell>
      );
    }

    // Not logged in + no existing account — inline registration form.
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
      navigate({ to: '/w/$slug/pm/my-work', params: { slug: data.workspace_slug } });
    };

    return (
      <PublicPageShell>
        <Card className="w-full">
          <CardHeader className="text-center">
            <CardTitle className="text-2xl">Join {info.workspace_name}</CardTitle>
            <CardDescription>
              {info.invited_by_name} invited you to join as <Badge variant="secondary" className="ml-1">{info.role}</Badge>
            </CardDescription>
          </CardHeader>
          <form onSubmit={handleSignupAndJoin}>
            <CardContent className="space-y-4 pb-6">
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
      </PublicPageShell>
    );
  }

  // Logged in with the wrong account for this invite.
  if (!inviteMatchesCurrentUser) {
    return (
      <PublicPageShell>
        <Card className="w-full">
          <CardHeader className="text-center">
            <CardTitle className="text-2xl">Switch Account To Join</CardTitle>
            <CardDescription>
              {info.invited_by_name} invited <span className="font-medium text-foreground">{info.email}</span> to join as <Badge variant="secondary" className="ml-1">{info.role}</Badge>
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-3 text-center text-sm text-muted-foreground">
            <p>You&apos;re currently signed in as <span className="font-medium text-foreground">{user.email}</span>.</p>
            <p>This invitation can only be accepted by <span className="font-medium text-foreground">{info.email}</span>.</p>
          </CardContent>
          <CardFooter className="flex flex-col gap-3">
            <Button className="w-full" onClick={handleSwitchAccount}>
              Sign in with the invited email
            </Button>
            <Button type="button" variant="outline" className="w-full" onClick={() => navigate({ to: '/workspaces' })}>
              Back to my workspaces
            </Button>
          </CardFooter>
        </Card>
      </PublicPageShell>
    );
  }

  // Logged in with the invited account — show join button.
  return (
    <PublicPageShell>
      <Card className="w-full">
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
    </PublicPageShell>
  );
}
