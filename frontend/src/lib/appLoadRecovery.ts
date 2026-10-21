export type AppVersionStatus = 'updated' | 'current' | 'unavailable';

export function isAppLoadError(error: unknown): boolean {
  return error instanceof Error && /Failed to fetch dynamically imported module|error loading dynamically imported module|Importing a module script failed|Unable to preload CSS|Loading chunk .+ failed/i.test(error.message);
}

function entryScripts(doc: Document): string[] {
  return Array.from(doc.querySelectorAll<HTMLScriptElement>('script[type="module"][src]'))
    .map(script => new URL(script.getAttribute('src')!, document.baseURI).href).sort();
}

export async function checkAppVersion(): Promise<AppVersionStatus> {
  const controller = new AbortController();
  const timeout = window.setTimeout(() => controller.abort(), 8000);
  try {
    const current = entryScripts(document);
    if (!current.length) return 'unavailable';
    const response = await fetch(new URL(import.meta.env.BASE_URL, window.location.href), {
      cache: 'no-store', signal: controller.signal, headers: { Accept: 'text/html' },
    });
    if (!response.ok || response.redirected || !response.headers.get('Content-Type')?.includes('text/html')) return 'unavailable';
    const doc = new DOMParser().parseFromString(await response.text(), 'text/html');
    const latest = entryScripts(doc);
    if (!doc.getElementById('root') || !latest.length) return 'unavailable';
    return JSON.stringify(current) === JSON.stringify(latest) ? 'current' : 'updated';
  } catch {
    return 'unavailable';
  } finally {
    window.clearTimeout(timeout);
  }
}

const refreshGuards = new Set<() => string | null>();

// Composers synchronously flush drafts here. A reason prevents refresh while
// an upload, send, or an unsaved edit cannot safely survive it.
export function registerAppRefreshGuard(guard: () => string | null): () => void {
  refreshGuards.add(guard);
  return () => { refreshGuards.delete(guard); };
}

export function prepareAppRefresh(): string | null {
  let blocked: string | null = null;
  for (const guard of refreshGuards) {
    try {
      const reason = guard();
      blocked ??= reason;
    } catch {
      blocked ??= 'Your draft could not be saved. Copy it before refreshing.';
    }
  }
  return blocked;
}

export function installAppLoadRecovery(): () => void {
  const onPreloadError = (event: Event) => {
    const error = (event as Event & { payload?: unknown }).payload;
    if (!isAppLoadError(error)) return;
    // TanStack Router 1.x otherwise automatically reloads on this rejection.
    // Set its one-shot reload marker before lazyRouteComponent
    // handles the error, so our error UI can offer a user-controlled refresh.
    // Keep the rejection intact: preventDefault would resolve imports to
    // undefined and cause a second, unrelated React error.
    try { sessionStorage.setItem(`tanstack_router_reload:${(error as Error).message}`, '1'); }
    catch { /* Router storage access also fails closed if storage is disabled. */ }
    // Snapshot text before a route error boundary has a chance to unmount an
    // editor. The refresh action checks guards again against the latest state.
    prepareAppRefresh();
  };
  window.addEventListener('vite:preloadError', onPreloadError);
  return () => window.removeEventListener('vite:preloadError', onPreloadError);
}
