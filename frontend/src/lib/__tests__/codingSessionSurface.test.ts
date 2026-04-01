import { describe, expect, it } from 'vitest';

import { buildCodingSessionPath, canOpenCodingSession, supportsCodingSessionSurface } from '../codingSessionSurface';

describe('codingSessionSurface helpers', () => {
  it('recognizes supported coding runtimes', () => {
    expect(supportsCodingSessionSurface('codex')).toBe(true);
    expect(supportsCodingSessionSurface('opencode')).toBe(true);
    expect(supportsCodingSessionSurface('native_sdk')).toBe(true);
  });

  it('returns false when no runtime is available', () => {
    expect(supportsCodingSessionSurface(undefined)).toBe(false);
    expect(canOpenCodingSession(null)).toBe(false);
  });

  it('builds coding session paths only when slug and session id exist', () => {
    expect(buildCodingSessionPath('acme', 'run-1')).toBe('/w/acme/pm/coding-sessions/run-1');
    expect(buildCodingSessionPath('', 'run-1')).toBeNull();
    expect(buildCodingSessionPath('acme', '')).toBeNull();
  });
});
