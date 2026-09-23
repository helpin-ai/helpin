import { useEffect, useState, type FormEvent } from 'react';
import { Link, useNavigate } from '@tanstack/react-router';
import { useTitle } from '@/hooks/useTitle';
import { ensureAuthConfiguration, useAuthStore } from '@/stores/authStore';
import { selfSignupAllowed } from '@/lib/services/authService';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card';
import { PublicPageShell } from '@/components/layout/PublicPageShell';
import { toast } from 'sonner';
import { loginRedirectFromSearch } from '@/lib/authRedirect';
import { signupSuccessRedirect } from '@/lib/signupRedirect';
import { formatDomainList, INVITE_ONLY_MESSAGE } from '@/lib/signupPolicy';

export default function Register() {
  useTitle('Sign Up');
  const [fullName, setFullName] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const [verificationEmail, setVerificationEmail] = useState<string | null>(null);
  const signUp = useAuthStore((state) => state.signUp);
  const configuration = useAuthStore((state) => state.configuration);
  const navigate = useNavigate();
  const redirect = loginRedirectFromSearch();
  const loginHref = redirect ? `/login?redirect=${encodeURIComponent(redirect)}` : '/login';

  useEffect(() => {
    void ensureAuthConfiguration();
  }, []);

  const firstUser = Boolean(configuration?.signup_first_user);
  const domains = configuration?.signup_mode === 'domains' ? configuration.signup_allowed_domains ?? [] : [];

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setLoading(true);
    const result = await signUp(email, password, fullName);
    setLoading(false);
    if (result.error) {
      toast.error(result.error);
      return;
    }
    if (result.verificationRequired) {
      setVerificationEmail(result.email ?? email);
      return;
    }
    toast.success('Account created successfully');
    const target = signupSuccessRedirect(redirect);
    if ('search' in target) {
      navigate({ to: target.to, search: target.search });
    } else {
      navigate({ to: target.to });
    }
  };

  const signInLink = <Link to={loginHref as '/login'} className="text-primary hover:underline">Sign in</Link>;

  if (verificationEmail) {
    return (
      <PublicPageShell>
        <Card className="w-full">
          <CardHeader className="text-center">
            <CardTitle role="heading" aria-level={1} className="text-2xl">Check your email</CardTitle>
            <CardDescription>We sent a confirmation link to {verificationEmail}. Open it, then sign in.</CardDescription>
          </CardHeader>
          <CardFooter className="justify-center">
            <p className="text-sm text-muted-foreground">Confirmed already? {signInLink}</p>
          </CardFooter>
        </Card>
      </PublicPageShell>
    );
  }

  if (!selfSignupAllowed(configuration)) {
    return (
      <PublicPageShell>
        <Card className="w-full">
          <CardHeader className="text-center">
            <CardTitle role="heading" aria-level={1} className="text-2xl">Signup is by invitation</CardTitle>
            <CardDescription>{INVITE_ONLY_MESSAGE}</CardDescription>
          </CardHeader>
          <CardFooter className="justify-center">
            <p className="text-sm text-muted-foreground">Already have an account? {signInLink}</p>
          </CardFooter>
        </Card>
      </PublicPageShell>
    );
  }

  return (
    <PublicPageShell>
      <Card className="w-full">
        <CardHeader className="text-center">
          <CardTitle role="heading" aria-level={1} className="text-2xl">{firstUser ? 'Create the admin account' : 'Create your account'}</CardTitle>
          <CardDescription>
            {firstUser ? 'You’re the first person on this server, so this account will manage it.' : 'A new home for your team’s work.'}
          </CardDescription>
        </CardHeader>
        <form onSubmit={handleSubmit}>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="fullName">Full name</Label>
              <Input id="fullName" name="name" autoComplete="name" type="text" placeholder="Jane Doe" value={fullName} onChange={e => setFullName(e.target.value)} required />
            </div>
            <div className="space-y-2">
              <Label htmlFor="email">Work email</Label>
              <Input id="email" name="email" autoComplete="email" type="email" placeholder="you@example.com" value={email} onChange={e => setEmail(e.target.value)} required aria-describedby={!firstUser && domains.length > 0 ? 'signup-domains-hint' : undefined} />
              {!firstUser && domains.length > 0 && (
                <p id="signup-domains-hint" className="text-xs text-muted-foreground">
                  Use your address at {formatDomainList(domains)}. We’ll email you a link to confirm it.
                </p>
              )}
            </div>
            <div className="space-y-2">
              <Label htmlFor="password">Password</Label>
              <Input id="password" name="password" autoComplete="new-password" type="password" placeholder="At least 8 characters" value={password} onChange={e => setPassword(e.target.value)} required minLength={8} />
            </div>
          </CardContent>
          <CardFooter className="flex flex-col gap-4 mt-4">
            <Button type="submit" className="w-full" disabled={loading}>
              {loading ? 'Creating account...' : 'Create account'}
            </Button>
            <p className="text-sm text-muted-foreground">
              Already have an account? {signInLink}
            </p>
          </CardFooter>
        </form>
      </Card>
    </PublicPageShell>
  );
}
