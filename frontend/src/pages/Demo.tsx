import { useEffect, useState, type FormEvent } from 'react';
import { Link, useNavigate } from '@tanstack/react-router';
import { useTitle } from '@/hooks/useTitle';
import { useAuthStore } from '@/stores/authStore';
import { authService, type AuthConfig } from '@/lib/services/authService';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card';
import { PublicPageShell } from '@/components/layout/PublicPageShell';
import { toast } from 'sonner';

/**
 * Public entry to the read-only demo workspace. Visitors are signed in as a
 * shared viewer account; no password is involved. The email field only feeds
 * the lead webhook and is optional unless the server says otherwise.
 */
export default function Demo() {
  useTitle('Live demo');
  const [email, setEmail] = useState('');
  const [loading, setLoading] = useState(false);
  const [config, setConfig] = useState<AuthConfig | null | undefined>(undefined);
  const { signInDemo } = useAuthStore();
  const navigate = useNavigate();

  useEffect(() => {
    let cancelled = false;
    void authService.config().then(({ data }) => {
      if (!cancelled) setConfig(data ?? null);
    });
    return () => {
      cancelled = true;
    };
  }, []);

  const demoEnabled = config?.demo_enabled === true;
  const emailRequired = config?.demo_requires_email === true;

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (loading) return;
    setLoading(true);
    const result = await signInDemo(email.trim() || undefined);
    setLoading(false);
    if (result.error) {
      toast.error(result.error);
      return;
    }
    navigate({ to: '/workspaces' });
  };

  if (config === undefined) {
    return <PublicPageShell>{null}</PublicPageShell>;
  }

  if (!demoEnabled) {
    return (
      <PublicPageShell>
        <Card className="w-full">
          <CardHeader className="text-center">
            <CardTitle className="text-2xl">Demo not available</CardTitle>
            <CardDescription>This server does not host a public demo workspace.</CardDescription>
          </CardHeader>
          <CardFooter className="justify-center">
            <Link to="/login" className="text-sm underline">
              Sign in instead
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
          <CardTitle className="text-2xl">Explore the live demo</CardTitle>
          <CardDescription>
            You will be signed in as a read-only viewer of a public demo workspace. Everything in it is visible to
            other visitors, and nothing can be changed.
          </CardDescription>
        </CardHeader>
        <form onSubmit={handleSubmit}>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="demo-email">Work email{emailRequired ? '' : ' (optional)'}</Label>
              <Input
                id="demo-email"
                type="email"
                autoComplete="email"
                placeholder="you@company.com"
                value={email}
                required={emailRequired}
                onChange={(e) => setEmail(e.target.value)}
                disabled={loading}
              />
            </div>
          </CardContent>
          <CardFooter className="flex flex-col gap-4 mt-4">
            <Button type="submit" className="w-full" disabled={loading}>
              {loading ? 'Opening…' : 'Open live demo'}
            </Button>
            <p className="text-center text-sm text-muted-foreground">
              Have an account?{' '}
              <Link to="/login" className="underline">
                Sign in
              </Link>
            </p>
          </CardFooter>
        </form>
      </Card>
    </PublicPageShell>
  );
}
