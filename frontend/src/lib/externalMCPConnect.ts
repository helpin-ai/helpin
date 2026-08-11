import type {
  CreateExternalMCPServerRequest,
  ExternalMCPAuthType,
  ExternalMCPProvider,
} from '@/lib/externalMCPTypes';

export type ExternalMCPConnectSource = {
  key: string;
  name: string;
  description: string;
  monogram: string;
  websiteURL: string;
  provider: 'customer_io' | 'custom';
  region?: string;
  endpointURL: string;
  authType: ExternalMCPAuthType;
  defaultScopes: string[];
  optionalScopes: string[];
  preset: boolean;
};

const FALLBACK_CUSTOMER_IO: ExternalMCPProvider[] = [
  {
    provider: 'customer_io',
    region: 'us',
    name: 'Customer.io US',
    website_url: 'https://customer.io',
    endpoint_url: 'https://mcp.customer.io/mcp',
    auth_type: 'oauth',
    default_scopes: ['read'],
    optional_scopes: ['read:sensitive', 'write', 'write:live', 'configure'],
  },
  {
    provider: 'customer_io',
    region: 'eu',
    name: 'Customer.io EU',
    website_url: 'https://customer.io',
    endpoint_url: 'https://mcp-eu.customer.io/mcp',
    auth_type: 'oauth',
    default_scopes: ['read'],
    optional_scopes: ['read:sensitive', 'write', 'write:live', 'configure'],
  },
];

export function externalMCPConnectSources(providers: ExternalMCPProvider[]): ExternalMCPConnectSource[] {
  const customerIO = FALLBACK_CUSTOMER_IO.map((fallback) => {
    const provider = providers.find((candidate) => candidate.provider === 'customer_io' && candidate.region === fallback.region) ?? fallback;
    const region = provider.region.toLowerCase();
    return {
      key: `customer_io:${region}`,
      name: `Customer.io (${region.toUpperCase()})`,
      description: `Preconfigured OAuth · ${region.toUpperCase()}`,
      monogram: 'C.io',
      websiteURL: provider.website_url,
      provider: 'customer_io' as const,
      region,
      endpointURL: provider.endpoint_url,
      authType: 'oauth' as const,
      defaultScopes: [...provider.default_scopes],
      optionalScopes: [...provider.optional_scopes],
      preset: true,
    };
  });

  const linear = providers.find((provider) => provider.provider === 'custom' && provider.region === 'linear');
  const sentry = providers.find((provider) => provider.provider === 'custom' && provider.region === 'sentry');

  return [
    ...customerIO,
    {
      key: 'custom:linear',
      name: 'Linear',
      description: 'Preconfigured OAuth',
      monogram: 'Lin',
      websiteURL: linear?.website_url ?? 'https://linear.app',
      provider: 'custom',
      endpointURL: linear?.endpoint_url ?? 'https://mcp.linear.app/mcp',
      authType: linear?.auth_type ?? 'oauth',
      defaultScopes: linear?.default_scopes ?? [],
      optionalScopes: linear?.optional_scopes ?? [],
      preset: true,
    },
    {
      key: 'custom:sentry',
      name: 'Sentry',
      description: 'Preconfigured OAuth',
      monogram: 'Sn',
      websiteURL: sentry?.website_url ?? 'https://sentry.io',
      provider: 'custom',
      endpointURL: sentry?.endpoint_url ?? 'https://mcp.sentry.dev/mcp',
      authType: sentry?.auth_type ?? 'oauth',
      defaultScopes: sentry?.default_scopes ?? [],
      optionalScopes: sentry?.optional_scopes ?? [],
      preset: true,
    },
    {
      key: 'custom:remote',
      name: 'Remote MCP server',
      description: 'Connect any approved Streamable HTTP endpoint with the auth method your server uses.',
      monogram: 'M',
      websiteURL: '',
      provider: 'custom',
      endpointURL: '',
      authType: 'headers',
      defaultScopes: [],
      optionalScopes: [],
      preset: false,
    },
  ];
}

export function isValidExternalMCPURL(value: string) {
  try {
    const url = new URL(value.trim());
    return url.protocol === 'https:' && Boolean(url.hostname.includes('.'));
  } catch {
    return false;
  }
}

export function buildExternalMCPCreateRequest({
  source,
  name,
  endpointURL,
  authType,
  scopes,
  bearerToken,
  headers,
}: {
  source: ExternalMCPConnectSource;
  name: string;
  endpointURL: string;
  authType: ExternalMCPAuthType;
  scopes: string[];
  bearerToken: string;
  headers: Array<{ name: string; value: string }>;
}): CreateExternalMCPServerRequest {
  if (source.provider === 'customer_io') {
    return {
      name: name.trim(),
      provider: 'customer_io',
      region: source.region,
      oauth_scopes: scopes,
    };
  }

  return {
    name: name.trim(),
    provider: 'custom',
    endpoint_url: endpointURL.trim(),
    auth_type: authType,
    oauth_scopes: authType === 'oauth' ? scopes : undefined,
    bearer_token: authType === 'bearer_token' ? bearerToken.trim() : undefined,
    headers: authType === 'headers'
      ? Object.fromEntries(headers.map((header) => [header.name.trim(), header.value.trim()]))
      : undefined,
  };
}
