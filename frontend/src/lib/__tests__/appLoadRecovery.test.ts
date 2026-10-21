// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { checkAppVersion, installAppLoadRecovery, isAppLoadError, prepareAppRefresh, registerAppRefreshGuard } from '../appLoadRecovery';

const shell = (entry: string) => `<html><head></head><body><div id="root"></div><script type="module" src="${entry}"></script></body></html>`;
const reply = (body: string, status = 200, type = 'text/html') => new Response(body, { status, headers: { 'Content-Type': type } });

beforeEach(() => {
  document.body.innerHTML = shell('/assets/index-old.js');
  sessionStorage.clear();
  vi.stubGlobal('fetch', vi.fn());
});
afterEach(() => vi.unstubAllGlobals());

describe('release recovery', () => {
  it('recognizes browser chunk errors without swallowing application errors', () => {
    for (const message of ['Failed to fetch dynamically imported module: /assets/old.js', 'error loading dynamically imported module', 'Importing a module script failed.', 'Unable to preload CSS for /assets/old.css']) {
      expect(isAppLoadError(new Error(message))).toBe(true);
    }
    expect(isAppLoadError(new Error('Failed to fetch'))).toBe(false);
    expect(isAppLoadError(new TypeError('Cannot read properties of undefined'))).toBe(false);
  });

  it('confirms a changed release using fresh HTML, rather than the missing chunk response', async () => {
    vi.mocked(fetch).mockResolvedValue(reply(shell('/assets/index-new.js')));
    expect(await checkAppVersion()).toBe('updated');
    expect(fetch).toHaveBeenCalledWith(expect.any(URL), expect.objectContaining({ cache: 'no-store', signal: expect.any(AbortSignal) }));
  });

  it('does not claim an update for the same release, a login page, or an HTTP failure', async () => {
    vi.mocked(fetch).mockResolvedValueOnce(reply(shell('/assets/index-old.js')))
      .mockResolvedValueOnce(reply('<html>Sign in</html>'))
      .mockResolvedValueOnce(reply(shell('/assets/index-new.js'), 503));
    expect(await checkAppVersion()).toBe('current');
    expect(await checkAppVersion()).toBe('unavailable');
    expect(await checkAppVersion()).toBe('unavailable');
  });

  it('reports connection failures without refreshing', async () => {
    vi.mocked(fetch).mockRejectedValue(new TypeError('Failed to fetch'));
    expect(await checkAppVersion()).toBe('unavailable');
  });

  it('leaves the original import rejection intact and prevents the router from auto-refreshing', () => {
    const cleanup = installAppLoadRecovery();
    const error = new TypeError('Failed to fetch dynamically imported module: /assets/old.js');
    const event = Object.assign(new Event('vite:preloadError', { cancelable: true }), { payload: error });
    window.dispatchEvent(event);
    expect(event.defaultPrevented).toBe(false);
    expect(event.payload).toBe(error);
    expect(sessionStorage.getItem(`tanstack_router_reload:${error.message}`)).toBe('1');
    cleanup();
  });

  it('flushes drafts and refuses refresh if a composer cannot safely save', () => {
    const saved = vi.fn(() => null);
    const removeSaved = registerAppRefreshGuard(saved);
    const removeUpload = registerAppRefreshGuard(() => 'Wait for the attachment to finish uploading.');
    expect(prepareAppRefresh()).toBe('Wait for the attachment to finish uploading.');
    expect(saved).toHaveBeenCalledOnce();
    removeUpload();
    expect(prepareAppRefresh()).toBeNull();
    removeSaved();
  });

  it('blocks refresh if browser storage fails', () => {
    const remove = registerAppRefreshGuard(() => { throw new DOMException('Quota exceeded'); });
    expect(prepareAppRefresh()).toContain('could not be saved');
    remove();
  });
});
