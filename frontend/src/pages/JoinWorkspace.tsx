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

  // Not logged in
  if (!user) {
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
            <p>Sign in or create an account to accept this invitation.</p>
            <p className="mt-1">Use <span className="font-medium">{info.email}</span> to join.</p>
          </CardContent>
          <CardFooter className="flex flex-col gap-2">
            <Button className="w-full" onClick={() => navigate({ to: `/login?redirect=/join/${token}` as string })}>
              Sign in
            </Button>
            <Button variant="outline" className="w-full" onClick={() => navigate({ to: `/register?redirect=/join/${token}` as string })}>
              Create account
            </Button>
          </CardFooter>
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
