import { Component, useEffect, useState, type ReactNode } from 'react';
import type { ErrorComponentProps } from '@tanstack/react-router';
import { Button } from '@/components/ui/button';
import { checkAppVersion, isAppLoadError, prepareAppRefresh, type AppVersionStatus } from '@/lib/appLoadRecovery';

export function AppLoadError({ error, reset }: Pick<ErrorComponentProps, 'error' | 'reset'>) {
  const loadError = isAppLoadError(error);
  const [version, setVersion] = useState<AppVersionStatus | 'checking'>('checking');
  const [busy, setBusy] = useState(false);
  const [blocked, setBlocked] = useState<string | null>(null);

  useEffect(() => {
    if (!loadError) return;
    let active = true;
    void checkAppVersion().then(status => { if (active) setVersion(status); });
    return () => { active = false; };
  }, [error, loadError]);

  const recover = async () => {
    if (busy) return;
    if (!loadError) { reset(); return; }
    setBusy(true);
    setBlocked(null);
    const status = await checkAppVersion();
    setVersion(status);
    if (status !== 'unavailable') {
      const reason = prepareAppRefresh();
      if (reason) setBlocked(reason);
      else {
        // A native unsaved-changes prompt may cancel navigation. Keep the
        // recovery button usable in that case; refresh is never automatic.
        setBusy(false);
        window.location.reload();
        return;
      }
    }
    setBusy(false);
  };

  return <div role="alert" className="flex h-full min-h-48 w-full items-center justify-center p-6">
    <div className="max-w-sm space-y-3 text-center">
      <h2 className="text-sm font-medium">{version === 'updated' ? 'Helpin has been updated' : 'This part of Helpin couldn’t load'}</h2>
      <p className="text-sm text-muted-foreground">
        {blocked ?? (version === 'updated' ? 'Save any other unsaved changes, then refresh to continue.'
          : loadError && version === 'checking' ? 'Checking for an update…'
          : loadError ? 'Check your connection, then retry. This will refresh the page.' : 'Please try again.')}
      </p>
      <Button size="sm" variant="outline" disabled={busy || (loadError && version === 'checking')} onClick={() => { void recover(); }}>
        {busy ? 'Checking…' : version === 'updated' ? 'Refresh' : 'Retry'}
      </Button>
    </div>
  </div>;
}

// Isolate optional panels so a failed chunk does not unmount the composer.
export class AppLoadBoundary extends Component<{ children: ReactNode }, { error: Error | null }> {
  state = { error: null as Error | null };
  static getDerivedStateFromError(error: Error) { return { error }; }
  render() {
    if (this.state.error) {
      if (!isAppLoadError(this.state.error)) throw this.state.error;
      return <AppLoadError error={this.state.error} reset={() => this.setState({ error: null })} />;
    }
    return this.props.children;
  }
}
