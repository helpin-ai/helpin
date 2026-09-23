// @vitest-environment jsdom
import { afterEach, describe, expect, it } from 'vitest';
import { GITHUB_LOGIN_PATTERN, gitHubReturnPath, readGitHubReturnResult, stripGitHubReturnParams } from '../githubReturn';

afterEach(() => window.history.replaceState(null, '', '/'));

describe('githubReturn', () => {
  it('reads the current flag and both older flags', () => {
    expect(readGitHubReturnResult('?github=connected&github_message=Done.')).toEqual({ status: 'connected', message: 'Done.' });
    expect(readGitHubReturnResult('?github_app=connected')).toEqual({ status: 'connected', message: 'GitHub connected.' });
    expect(readGitHubReturnResult('?github_app_manifest=created')?.status).toBe('created');
    expect(readGitHubReturnResult('?github=something-else')?.status).toBe('error');
    expect(readGitHubReturnResult('?tab=x')).toBeNull();
  });

  it('strips only GitHub result parameters', () => {
    window.history.replaceState(null, '', '/w/acme/setup?tab=x&github=connected&github_message=Hi&integration_id=gi-1#top');
    stripGitHubReturnParams();
    expect(`${window.location.pathname}${window.location.search}${window.location.hash}`).toBe('/w/acme/setup?tab=x#top');
  });

  it('maps return destinations to the server’s pages', () => {
    expect(gitHubReturnPath('acme')).toBe('/w/acme/settings/git-connections');
    expect(gitHubReturnPath('acme', 'setup')).toBe('/w/acme/setup');
    expect(gitHubReturnPath('acme', 'system_status')).toBe('/w/acme/settings/system-status');
    expect(gitHubReturnPath('acme', 'onboarding')).toBe('/onboarding?step=github&workspace=acme');
  });

  it('keeps the onboarding step and workspace when stripping the result', () => {
    window.history.replaceState(null, '', '/onboarding?step=github&workspace=acme&github=created&github_message=Install+it.');
    expect(readGitHubReturnResult(window.location.search)).toEqual({ status: 'created', message: 'Install it.' });
    stripGitHubReturnParams();
    expect(`${window.location.pathname}${window.location.search}`).toBe('/onboarding?step=github&workspace=acme');
  });

  it('matches GitHub organization logins', () => {
    expect(GITHUB_LOGIN_PATTERN.test('acme-inc')).toBe(true);
    expect(GITHUB_LOGIN_PATTERN.test('-acme')).toBe(false);
    expect(GITHUB_LOGIN_PATTERN.test('a'.repeat(40))).toBe(false);
    expect(GITHUB_LOGIN_PATTERN.test('acme/inc')).toBe(false);
  });
});
