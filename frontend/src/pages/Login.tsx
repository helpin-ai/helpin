import { useEffect, useState, type FormEvent } from 'react';
import { Link, useNavigate } from '@tanstack/react-router';
import { useTitle } from '@/hooks/useTitle';
import { useAuthStore } from '@/stores/authStore';
import { workspacesService } from '@/lib/services/workspacesService';
import { passkeyService } from '@/lib/services/passkeyService';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card';
import { PublicPageShell } from '@/components/layout/PublicPageShell';
import { toast } from 'sonner';
import { consumeRedirectAfterLogin, loginRedirectFromSearch } from '@/lib/authRedirect';

export default function Login() {
  useTitle('Sign In');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [rememberMe, setRememberMe] = useState(true);
  const [twoFaToken, setTwoFaToken] = useState<string | null>(null);
  const [twoFactorCode, setTwoFactorCode] = useState('');
  const [useRecoveryCode, setUseRecoveryCode] = useState(false);
  const [loading, setLoading] = useState(false);
  const { signIn, signInWithPasskey, verify2FASignIn } = useAuthStore();
  const navigate = useNavigate();
  const passkeySupported = passkeyService.isSupported();
  const redirect = loginRedirectFromSearch();
  const registerHref = redirect ? `/register?redirect=${encodeURIComponent(redirect)}` : '/register';

  const completeLoginRedirect = async (isCancelled?: () => boolean) => {
    const redirect = loginRedirectFromSearch() ?? consumeRedirectAfterLogin();
    if (redirect) {
      window.location.assign(redirect);
      return;
    }

    // After login, redirect to the user's default workspace if set.
    const { data: workspaces } = await workspacesService.list();
    if (isCancelled?.()) {
      return;
    }

    if (workspaces && workspaces.length > 0) {
      const user = useAuthStore.getState().user;
      const defaultWs = user?.default_workspace_id
        ? workspaces.find((w) => w.id === user.default_workspace_id)
        : null;
      const targetSlug = defaultWs ? defaultWs.slug : workspaces[0].slug;
      navigate({ to: '/w/$slug/pm/my-work', params: { slug: targetSlug } });
    } else {
      navigate({ to: '/workspaces' });
    }
  };

  useEffect(() => {
    if (twoFaToken || !passkeySupported) {
      return;
    }

    let cancelled = false;

    const beginPasskeyAutofill = async () => {
      const autofillSupported = await passkeyService.isAutofillSupported();
      if (cancelled || !autofillSupported) {
        return;
      }

      const result = await signInWithPasskey(undefined, rememberMe, { useAutofill: true });
      if (cancelled || result.cancelled) {
        return;
      }
      if (result.error) {
        toast.error(result.error);
        return;
      }
      if (result.requires2FA && result.twoFAToken) {
        setTwoFaToken(result.twoFAToken);
        setTwoFactorCode('');
        setUseRecoveryCode(false);
        toast.success('Passkey accepted. Enter your authenticator code.');
        return;
      }

      setLoading(true);
      try {
        await completeLoginRedirect(() => cancelled);
      } finally {
        setLoading(false);
      }
    };

    void beginPasskeyAutofill();

    return () => {
      cancelled = true;
      passkeyService.cancelPendingAuthentication();
    };
  }, [navigate, passkeySupported, rememberMe, signInWithPasskey, twoFaToken]);

  const handlePasswordSubmit = async () => {
    const { error, requires2FA, twoFAToken } = await signIn(email, password, rememberMe);
    if (error) {
      toast.error(error);
      return;
    }
    if (requires2FA && twoFAToken) {
      setTwoFaToken(twoFAToken);
      setTwoFactorCode('');
      setUseRecoveryCode(false);
      toast.success('Password accepted. Enter your authenticator code.');
      return;
    }
    await completeLoginRedirect();
  };

  const handleTwoFactorSubmit = async () => {
    if (!twoFaToken) {
      toast.error('Two-factor session expired. Sign in again.');
      setTwoFaToken(null);
      setTwoFactorCode('');
      return;
    }

    const { error } = await verify2FASignIn(twoFaToken, twoFactorCode, useRecoveryCode, rememberMe);
    if (error) {
      toast.error(error);
      return;
    }

    await completeLoginRedirect();
  };

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      if (twoFaToken) {
        await handleTwoFactorSubmit();
      } else {
        passkeyService.cancelPendingAuthentication();
        await handlePasswordSubmit();
      }
    } finally {
      setLoading(false);
    }
  };

  const handlePasskeyLogin = async () => {
    setLoading(true);
    try {
      const { error, requires2FA, twoFAToken } = await signInWithPasskey(undefined, rememberMe);
      if (error) {
        toast.error(error);
        return;
      }
      if (requires2FA && twoFAToken) {
        setTwoFaToken(twoFAToken);
        setTwoFactorCode('');
        setUseRecoveryCode(false);
        toast.success('Passkey accepted. Enter your authenticator code.');
        return;
      }
      await completeLoginRedirect();
    } finally {
      setLoading(false);
    }
  };

  return (
    <PublicPageShell>
      <Card className="w-full">
        <CardHeader className="text-center">
          <CardTitle className="text-2xl">Sign in</CardTitle>
          <CardDescription>
            {twoFaToken
              ? useRecoveryCode
                ? 'Enter one of your saved recovery codes.'
                : 'Enter the 6-digit code from your authenticator app.'
              : 'Enter your email and password.'}
          </CardDescription>
        </CardHeader>
        <form onSubmit={handleSubmit}>
          <CardContent className="space-y-4">
            {twoFaToken ? (
              <>
                <div className="rounded-lg border bg-muted/30 px-3 py-2 text-sm text-muted-foreground">
                  Signing in as <span className="font-medium text-foreground">{email}</span>
                </div>
                <div className="space-y-2">
                  <Label htmlFor="two-factor-code">{useRecoveryCode ? 'Recovery Code' : 'Authenticator Code'}</Label>
                  <Input
                    id="two-factor-code"
                    inputMode={useRecoveryCode ? 'text' : 'numeric'}
                    autoComplete={useRecoveryCode ? 'one-time-code' : 'one-time-code'}
                    placeholder={useRecoveryCode ? 'ABCD1234' : '123456'}
                    value={twoFactorCode}
                    onChange={e => setTwoFactorCode(e.target.value)}
                    required
                  />
                  <div className="flex items-center justify-between text-sm">
                    <button
                      type="button"
                      className="text-primary hover:underline"
                      onClick={() => {
                        setUseRecoveryCode((value) => !value);
                        setTwoFactorCode('');
                      }}
                    >
                      {useRecoveryCode ? 'Use authenticator code' : 'Use recovery code'}
                    </button>
                    <button
                      type="button"
                      className="text-muted-foreground hover:text-foreground"
                      onClick={() => {
                        setTwoFaToken(null);
                        setTwoFactorCode('');
                        setUseRecoveryCode(false);
                      }}
                    >
                      Back
                    </button>
                  </div>
                </div>
              </>
            ) : (
              <>
                <div className="space-y-2">
                  <Label htmlFor="email">Email</Label>
                  <Input
                    id="email"
                    name="email"
                    type="email"
                    autoComplete="username webauthn"
                    placeholder="you@example.com"
                    value={email}
                    onChange={e => setEmail(e.target.value)}
                    required
                  />
                </div>
                <div className="space-y-2">
                  <div className="flex items-center justify-between">
                    <Label htmlFor="password">Password</Label>
                    <Link to="/forgot-password" className="text-sm text-primary hover:underline">Forgot password?</Link>
                  </div>
                  <Input
                    id="password"
                    name="password"
                    type="password"
                    autoComplete="current-password"
                    placeholder="••••••••"
                    value={password}
                    onChange={e => setPassword(e.target.value)}
                    required
                  />
                </div>
                <div className="flex items-center gap-2">
                  <Checkbox id="remember-me" checked={rememberMe} onCheckedChange={(checked) => setRememberMe(checked === true)} />
                  <Label htmlFor="remember-me" className="text-sm font-normal cursor-pointer">Remember me</Label>
                </div>
              </>
            )}
          </CardContent>
          <CardFooter className="flex flex-col gap-4 mt-4">
            <Button type="submit" className="w-full" disabled={loading}>
              {loading ? (twoFaToken ? 'Verifying...' : 'Signing in...') : (twoFaToken ? 'Verify and continue' : 'Sign in')}
            </Button>
            {!twoFaToken && (
              <>
                <Button type="button" variant="outline" className="w-full" disabled={loading || !passkeySupported} onClick={() => void handlePasskeyLogin()}>
                  Sign in with passkey
                </Button>
                {!passkeySupported && (
                  <p className="text-xs text-muted-foreground text-center">
                    This browser does not support passkeys.
                  </p>
                )}
              </>
            )}
            {!twoFaToken && (
              <p className="text-sm text-muted-foreground">
                Don't have an account? <Link to={registerHref as '/register'} className="text-primary hover:underline">Sign up</Link>
              </p>
            )}
          </CardFooter>
        </form>
      </Card>
    </PublicPageShell>
  );
}
