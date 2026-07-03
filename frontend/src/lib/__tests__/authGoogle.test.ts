import { describe, expect, it } from 'vitest';

import { API_BASE } from '@/lib/api';
import { buildGoogleAuthStartUrl } from '@/lib/authGoogle';

describe('buildGoogleAuthStartUrl', () => {
  it('points to the backend Google start route', () => {
    expect(buildGoogleAuthStartUrl()).toBe(`${API_BASE}/auth/google/start`);
  });

  it('preserves the post-auth redirect when provided', () => {
    expect(buildGoogleAuthStartUrl('/w/acme/pm/my-work')).toBe(
      `${API_BASE}/auth/google/start?redirect=%2Fw%2Facme%2Fpm%2Fmy-work`,
    );
  });
});
