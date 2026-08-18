import { describe, expect, it } from 'vitest';
import {
  buildExternalMCPCreateRequest,
  externalMCPConnectSources,
  isValidExternalMCPURL,
} from '@/lib/externalMCPConnect';

describe('external MCP connection helpers', () => {
  it('offers provider presets alongside a generic remote server', () => {
    const sources = externalMCPConnectSources([]);

    expect(sources.map((source) => source.name)).toEqual([
      'Customer.io (US)',
      'Customer.io (EU)',
      'Linear',
      'Sentry',
      'Remote MCP server',
    ]);
    expect(sources.at(-1)).toMatchObject({ provider: 'custom', preset: false, authType: 'headers' });
  });

  it('uses provider metadata returned by the API for Customer.io presets', () => {
    const sources = externalMCPConnectSources([{
      provider: 'customer_io',
      region: 'eu',
      name: 'Customer.io EU',
      website_url: 'https://customer.io',
      endpoint_url: 'https://custom-eu.example.com/mcp',
      auth_type: 'oauth',
      default_scopes: ['read'],
      optional_scopes: ['write'],
    }]);

    expect(sources[1]).toMatchObject({
      websiteURL: 'https://customer.io',
      endpointURL: 'https://custom-eu.example.com/mcp',
      defaultScopes: ['read'],
      optionalScopes: ['write'],
    });
  });

  it('builds a custom-header request without leaking unrelated credentials', () => {
    const source = externalMCPConnectSources([]).at(-1)!;

    expect(buildExternalMCPCreateRequest({
      source,
      name: '  Internal tools  ',
      endpointURL: ' https://mcp.example.com/mcp ',
      authType: 'headers',
      scopes: ['read'],
      bearerToken: 'unused-token',
      headers: [{ name: ' X-API-Key ', value: ' secret ' }],
    })).toEqual({
      name: 'Internal tools',
      provider: 'custom',
      endpoint_url: 'https://mcp.example.com/mcp',
      auth_type: 'headers',
      oauth_scopes: undefined,
      bearer_token: undefined,
      headers: { 'X-API-Key': 'secret' },
    });
  });

  it('only accepts absolute HTTPS server URLs', () => {
    expect(isValidExternalMCPURL('https://mcp.example.com/mcp')).toBe(true);
    expect(isValidExternalMCPURL('http://mcp.example.com/mcp')).toBe(false);
    expect(isValidExternalMCPURL('/mcp')).toBe(false);
    expect(isValidExternalMCPURL('not a URL')).toBe(false);
  });
});
