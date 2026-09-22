import { useState, type FormEvent } from 'react';
import { Link, useNavigate } from '@tanstack/react-router';
import { useTitle } from '@/hooks/useTitle';
import { useAuthStore } from '@/stores/authStore';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card';
import { PublicPageShell } from '@/components/layout/PublicPageShell';
import { toast } from 'sonner';
import { loginRedirectFromSearch } from '@/lib/authRedirect';
import { signupSuccessRedirect } from '@/lib/signupRedirect';

export default function Register() {
  useTitle('Sign Up');
  const [fullName, setFullName] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const { signUp } = useAuthStore();
  const navigate = useNavigate();
  const redirect = loginRedirectFromSearch();
  const loginHref = redirect ? `/login?redirect=${encodeURIComponent(redirect)}` : '/login';

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setLoading(true);
    const { error } = await signUp(email, password, fullName);
    setLoading(false);
    if (error) {
      toast.error(error);
    } else {
      toast.success('Account created successfully');
      const target = signupSuccessRedirect(redirect);
      if ('search' in target) {
        navigate({ to: target.to, search: target.search });
      } else {
        navigate({ to: target.to });
      }
    }
  };

  return (
    <PublicPageShell>
      <Card className="w-full">
        <CardHeader className="text-center">
          <CardTitle role="heading" aria-level={1} className="text-2xl">Create your account</CardTitle>
          <CardDescription>A new home for your team’s work.</CardDescription>
        </CardHeader>
        <form onSubmit={handleSubmit}>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="fullName">Full name</Label>
              <Input id="fullName" name="name" autoComplete="name" type="text" placeholder="Jane Doe" value={fullName} onChange={e => setFullName(e.target.value)} required />
            </div>
            <div className="space-y-2">
              <Label htmlFor="email">Work email</Label>
              <Input id="email" name="email" autoComplete="email" type="email" placeholder="you@example.com" value={email} onChange={e => setEmail(e.target.value)} required />
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
              Already have an account? <Link to={loginHref as '/login'} className="text-primary hover:underline">Sign in</Link>
            </p>
          </CardFooter>
        </form>
      </Card>
    </PublicPageShell>
  );
}
