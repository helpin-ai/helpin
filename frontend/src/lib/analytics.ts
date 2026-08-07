import type { UsermavenOptions } from '@usermaven/sdk-js';
import type { OrganizationWithRole, User, Workspace, WorkspaceAccess, WorkspaceBillingSummary } from './types';

export const APP_ANALYTICS_HOST = 'app.helpin.ai';
export const USERMAVEN_KEY = 'UMpgKYZLxR';
export const CUSTOMER_IO_WRITE_KEY = 'a3fced22111b6be05726';
export const CUSTOMER_IO_WORKSPACE_OBJECT_TYPE_ID = '1';

type UsermavenInitOptions = Pick<UsermavenOptions, 'key' | 'tracking_host' | 'autocapture' | 'cookie_domain'>;

export type AnalyticsTraits = Record<string, unknown>;

export type AnalyticsClients = {
  usermaven: {
    init: (options: UsermavenInitOptions) => void;
    id: (traits: AnalyticsTraits) => void | Promise<void>;
    group: (traits: AnalyticsTraits) => void | Promise<void>;
    track: (eventName: string, properties?: Record<string, unknown>) => void;
    reset: () => void | Promise<void>;
  };
  customerio: {
    load: (settings: { writeKey: string }) => void;
    identify: (userId: string, traits?: AnalyticsTraits) => void | Promise<unknown>;
    group: (groupId: string, traits?: AnalyticsTraits) => void | Promise<unknown>;
    track: (eventName: string, properties?: Record<string, unknown>) => void | Promise<unknown>;
    reset: () => void;
  };
};

type AnalyticsOptions = {
  hostname?: string;
  clients?: AnalyticsClients;
};

let initialized = false;
let defaultUsermavenClient: AnalyticsClients['usermaven'] | null = null;
let defaultCustomerIoClient: AnalyticsClients['customerio'] | null = null;
let defaultSdkPromise: Promise<void> | null = null;
let pendingUsermavenOptions: UsermavenInitOptions | null = null;
let pendingCustomerIoSettings: { writeKey: string } | null = null;
let customerIoLoaded = false;

function ensureDefaultSdkClients() {
  if (!defaultSdkPromise) {
    defaultSdkPromise = Promise.all([
      import('@usermaven/sdk-js'),
      import('@customerio/cdp-analytics-browser'),
    ]).then(([usermavenSdk, customerIoSdk]) => {
      if (pendingUsermavenOptions && !defaultUsermavenClient) {
        const client = usermavenSdk.usermavenClient(pendingUsermavenOptions);
        defaultUsermavenClient = {
          init() {},
          id: (traits) => client.id(traits),
          group: (traits) => client.group(traits as Parameters<typeof client.group>[0]),
          track: (eventName, properties) => client.track(eventName, properties),
          reset: () => client.reset(),
        };
      }

      if (!defaultCustomerIoClient) {
        const client = new customerIoSdk.AnalyticsBrowser();
        defaultCustomerIoClient = {
          load: (settings) => {
            client.load(settings);
          },
          identify: (userId, traits) => client.identify(userId, traits),
          group: (groupId, traits) => client.group(groupId, traits),
          track: (eventName, properties) => client.track(eventName, properties),
          reset: () => client.reset(),
        };
      }

      if (pendingCustomerIoSettings && defaultCustomerIoClient && !customerIoLoaded) {
        defaultCustomerIoClient.load(pendingCustomerIoSettings);
        customerIoLoaded = true;
      }
    });
  }
  return defaultSdkPromise;
}

const defaultClients: AnalyticsClients = {
  usermaven: {
    init(options) {
      pendingUsermavenOptions = options;
      void ensureDefaultSdkClients();
    },
    id(traits) {
      void ensureDefaultSdkClients().then(() => defaultUsermavenClient?.id(traits));
    },
    group(traits) {
      void ensureDefaultSdkClients().then(() => defaultUsermavenClient?.group(traits));
    },
    track(eventName, properties) {
      void ensureDefaultSdkClients().then(() => defaultUsermavenClient?.track(eventName, properties));
    },
    reset() {
      void ensureDefaultSdkClients().then(() => defaultUsermavenClient?.reset());
    },
  },
  customerio: {
    load(settings) {
      pendingCustomerIoSettings = settings;
      void ensureDefaultSdkClients();
    },
    identify(userId, traits) {
      void ensureDefaultSdkClients().then(() => defaultCustomerIoClient?.identify(userId, traits));
    },
    group(groupId, traits) {
      void ensureDefaultSdkClients().then(() => defaultCustomerIoClient?.group(groupId, traits));
    },
    track(eventName, properties) {
      void ensureDefaultSdkClients().then(() => defaultCustomerIoClient?.track(eventName, properties));
    },
    reset() {
      void ensureDefaultSdkClients().then(() => defaultCustomerIoClient?.reset());
    },
  },
};

function currentHostname() {
  return typeof window === 'undefined' ? '' : window.location.hostname;
}

function clientsFor(options?: AnalyticsOptions) {
  return options?.clients ?? defaultClients;
}

function hostFor(options?: AnalyticsOptions) {
  return options?.hostname ?? currentHostname();
}

export function shouldEnableAppAnalytics(hostname = currentHostname()) {
  return hostname === APP_ANALYTICS_HOST;
}

export function initializeAppAnalytics(options?: AnalyticsOptions) {
  if (!shouldEnableAppAnalytics(hostFor(options))) return false;
  if (!options?.clients && initialized) return true;

  const clients = clientsFor(options);
  clients.usermaven.init({
    key: USERMAVEN_KEY,
    tracking_host: 'https://events.usermaven.com',
    autocapture: true,
    cookie_domain: 'helpin.ai',
  });
  clients.customerio.load({ writeKey: CUSTOMER_IO_WRITE_KEY });
  if (!options?.clients) initialized = true;
  return true;
}

function splitFullName(fullName: string) {
  const parts = fullName.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) return { firstName: undefined, lastName: undefined };
  if (parts.length === 1) return { firstName: parts[0], lastName: undefined };
  return { firstName: parts[0], lastName: parts.slice(1).join(' ') };
}

function emailDomain(email: string) {
  const [, domain] = email.split('@');
  return domain?.toLowerCase() || undefined;
}

function websiteDomain(websiteUrl?: string) {
  if (!websiteUrl) return undefined;
  try {
    return new URL(websiteUrl).hostname.replace(/^www\./, '').toLowerCase();
  } catch {
    return undefined;
  }
}

function daysUntil(isoDate?: string) {
  if (!isoDate) return undefined;
  const time = new Date(isoDate).getTime();
  if (!Number.isFinite(time)) return undefined;
  return Math.max(0, Math.ceil((time - Date.now()) / 86_400_000));
}

function isAnalyticsOptions(value: unknown): value is AnalyticsOptions {
  return !!value && typeof value === 'object' && ('hostname' in value || 'clients' in value);
}

function normalizeUserArgs(
  organizationOrOptions?: OrganizationWithRole | AnalyticsOptions | null,
  maybeOptions?: AnalyticsOptions,
) {
  if (isAnalyticsOptions(organizationOrOptions)) {
    return { organization: undefined, options: organizationOrOptions };
  }
  return { organization: organizationOrOptions ?? undefined, options: maybeOptions };
}

function normalizeWorkspaceArgs(
  organizationOrOptions?: OrganizationWithRole | AnalyticsOptions | null,
  maybeOptions?: AnalyticsOptions,
) {
  if (isAnalyticsOptions(organizationOrOptions)) {
    return { organization: undefined, options: organizationOrOptions };
  }
  return { organization: organizationOrOptions ?? undefined, options: maybeOptions };
}

export function buildAnalyticsOrganizationTraits(organization: OrganizationWithRole, workspaces?: Workspace[]): AnalyticsTraits {
  const traits: AnalyticsTraits = {
    id: organization.id,
    name: organization.name,
    created_at: organization.created_at,
    custom: {
      organization_slug: organization.slug,
      owner_user_id: organization.owner_id,
      organization_role: organization.role,
      has_logo: !!organization.logo_url,
    },
  };
  if (workspaces) {
    const paid = workspaces.filter((workspace) => workspace.billing?.status === 'active');
    const trialing = workspaces.filter((workspace) => workspace.billing?.trialing);
    traits.custom = {
      ...(traits.custom as Record<string, unknown>),
      workspace_count: workspaces.length,
      paid_workspace_count: paid.length,
      trialing_workspace_count: trialing.length,
      organization_trialing: trialing.length > 0,
      organization_plan_mix: [...new Set(workspaces.map((workspace) => getWorkspaceAnalyticsPlan(workspace)))],
      aggregates_updated_at: new Date().toISOString(),
    };
  }
  return traits;
}

export function buildAnalyticsUserTraits(user: User, organization?: OrganizationWithRole | null, workspaces?: Workspace[]): AnalyticsTraits {
  const { firstName, lastName } = splitFullName(user.full_name);
  const traits: AnalyticsTraits = {
    id: user.id,
    email: user.email,
    full_name: user.full_name,
    first_name: firstName,
    last_name: lastName,
    created_at: user.created_at,
    updated_at: user.updated_at,
    email_verified: !!user.email_verified,
    email_verified_at: user.email_verified_at,
    default_workspace_id: user.default_workspace_id,
    two_fa_enabled: !!user.two_fa_enabled,
    mfa_satisfied_in_token: !!user.mfa_satisfied_in_token,
    avatar_style: user.avatar_style,
    avatar_background_mode: user.avatar_background_mode,
    custom: {
      has_default_workspace: !!user.default_workspace_id,
      signup_domain: emailDomain(user.email),
      has_avatar: !!user.avatar_url || !!user.avatar_style,
      profile_complete: !!user.full_name?.trim() && !!user.email_verified,
    },
  };
  if (organization) {
    traits.company = buildAnalyticsOrganizationTraits(organization, workspaces);
  }
  return traits;
}

export function getWorkspaceAnalyticsPlan(workspace: Workspace) {
  return workspace.billing?.plan ?? 'unknown';
}

export function buildAnalyticsWorkspaceTraits(
  workspace: Workspace,
  access?: WorkspaceAccess | null,
  organization?: OrganizationWithRole | null,
): AnalyticsTraits {
  const billing = workspace.billing;
  const modules = access?.modules ?? [];
  return {
    id: workspace.id,
    name: workspace.name,
    created_at: workspace.created_at,
    website: workspace.website_url,
    plan: getWorkspaceAnalyticsPlan(workspace),
    custom: {
      workspace_slug: workspace.slug,
      workspace_key: workspace.workspace_key,
      organization_id: workspace.organization_id,
      organization_name: organization?.name,
      organization_slug: organization?.slug,
      organization_role: organization?.role,
      owner_user_id: workspace.owner_id,
      website_domain: websiteDomain(workspace.website_url),
      billing_status: billing?.status,
      billing_interval: billing?.billing_interval,
      trialing: !!billing?.trialing,
      trial_ends_at: billing?.trial_ends_at,
      trial_days_left: daysUntil(billing?.trial_ends_at),
      locked: !!billing?.locked,
      credits_used: billing?.credits_used,
      credits_remaining: billing?.credits_remaining,
      seat_limit: billing?.seat_limit,
      seat_usage: billing?.seat_usage,
      seat_over_limit: billing?.seat_over_limit,
      has_company_product_context: !!workspace.company_product_context?.trim(),
      modules,
      module_count: modules.length,
      team_membership_count: access?.team_memberships?.length ?? 0,
      workspace_role: access?.membership?.role ?? workspace.role,
      membership_status: access?.membership?.status,
    },
  };
}

export function buildAnalyticsBillingEventProperties(
  eventName: string,
  workspaceId: string,
  billing?: WorkspaceBillingSummary | null,
  extra?: Record<string, unknown>,
) {
  return {
    event_name: eventName,
    workspace_id: workspaceId,
    plan: billing?.plan,
    billing_status: billing?.status,
    billing_interval: billing?.billing_interval,
    trialing: !!billing?.trialing,
    trial_ends_at: billing?.trial_ends_at,
    trial_days_left: daysUntil(billing?.trial_ends_at),
    locked: !!billing?.locked,
    credits_used: billing?.credits_used,
    credits_remaining: billing?.credits_remaining,
    included_credits: billing?.included_credits,
    seat_limit: billing?.seat_limit,
    seat_usage: billing?.seat_usage,
    seat_over_limit: billing?.seat_over_limit,
    on_demand_enabled: billing?.on_demand_enabled,
    on_demand_available: billing?.on_demand_available,
    manage_billing_enabled: billing?.manage_billing_enabled,
    cancel_at_period_end: billing?.cancel_at_period_end,
    ...extra,
  };
}

export function buildCustomerIoWorkspaceTraits(workspace: Workspace, access?: WorkspaceAccess | null): AnalyticsTraits {
  const billing = workspace.billing;
  const modules = access?.modules ?? [];
  return {
    objectTypeId: CUSTOMER_IO_WORKSPACE_OBJECT_TYPE_ID,
    name: workspace.name,
    created_at: workspace.created_at,
    website: workspace.website_url,
    workspace_id: workspace.id,
    workspace_slug: workspace.slug,
    workspace_key: workspace.workspace_key,
    organization_id: workspace.organization_id,
    owner_user_id: workspace.owner_id,
    website_domain: websiteDomain(workspace.website_url),
    plan: getWorkspaceAnalyticsPlan(workspace),
    billing_status: billing?.status,
    billing_interval: billing?.billing_interval,
    trialing: !!billing?.trialing,
    trial_ends_at: billing?.trial_ends_at,
    trial_days_left: daysUntil(billing?.trial_ends_at),
    locked: !!billing?.locked,
    credits_used: billing?.credits_used,
    credits_remaining: billing?.credits_remaining,
    seat_limit: billing?.seat_limit,
    seat_usage: billing?.seat_usage,
    seat_over_limit: billing?.seat_over_limit,
    has_company_product_context: !!workspace.company_product_context?.trim(),
    modules,
    module_count: modules.length,
    relationshipAttributes: {
      workspace_role: access?.membership?.role ?? workspace.role,
      membership_status: access?.membership?.status,
      team_membership_count: access?.team_memberships?.length ?? 0,
    },
  };
}

export function identifyAnalyticsUser(
  user: User | null | undefined,
  organizationOrOptions?: OrganizationWithRole | AnalyticsOptions | null,
  maybeOptions?: AnalyticsOptions,
) {
  const { organization, options } = normalizeUserArgs(organizationOrOptions, maybeOptions);
  if (!user || !shouldEnableAppAnalytics(hostFor(options))) return;
  const traits = buildAnalyticsUserTraits(user, organization);
  const clients = clientsFor(options);
  void clients.usermaven.id(traits);
  void clients.customerio.identify(user.id, traits);
}

export function identifyAnalyticsWorkspace(
  workspace: Workspace | null | undefined,
  access?: WorkspaceAccess | null,
  organizationOrOptions?: OrganizationWithRole | AnalyticsOptions | null,
  maybeOptions?: AnalyticsOptions,
) {
  const { organization, options } = normalizeWorkspaceArgs(organizationOrOptions, maybeOptions);
  if (!workspace || !shouldEnableAppAnalytics(hostFor(options))) return;
  const traits = buildAnalyticsWorkspaceTraits(workspace, access, organization);
  const customerIoTraits = buildCustomerIoWorkspaceTraits(workspace, access);
  const clients = clientsFor(options);
  void clients.usermaven.group(traits);
  void clients.customerio.group(workspace.id, customerIoTraits);
}

export function identifyAnalyticsOrganization(
  organization: OrganizationWithRole | null | undefined,
  user?: User | null,
  options?: AnalyticsOptions,
  workspaces?: Workspace[],
) {
  if (!organization || !shouldEnableAppAnalytics(hostFor(options))) return;
  const clients = clientsFor(options);
  if (user) {
    void clients.usermaven.id(buildAnalyticsUserTraits(user, organization, workspaces));
  }
  clients.usermaven.track('organization_identified', {
    organization_id: organization.id,
    organization_name: organization.name,
    organization_slug: organization.slug,
    organization_role: organization.role,
    owner_user_id: organization.owner_id,
    user_id: user?.id,
    has_logo: !!organization.logo_url,
    workspace_count: workspaces?.length,
    paid_workspace_count: workspaces?.filter((workspace) => workspace.billing?.status === 'active').length,
    trialing_workspace_count: workspaces?.filter((workspace) => workspace.billing?.trialing).length,
  });
}

export function trackAnalyticsEvent(
  eventName: string,
  properties?: Record<string, unknown>,
  options?: AnalyticsOptions,
) {
  if (!shouldEnableAppAnalytics(hostFor(options))) return;
  const clients = clientsFor(options);
  clients.usermaven.track(eventName, properties);
  void clients.customerio.track(eventName, properties);
}

export function buildAnalyticsWorkspaceEventProperties(
  eventName: string,
  workspace: Workspace,
  access?: WorkspaceAccess | null,
  organization?: OrganizationWithRole | null,
  extra?: Record<string, unknown>,
) {
  return {
    ...buildAnalyticsBillingEventProperties(eventName, workspace.id, workspace.billing, {
      workspace_name: workspace.name,
      workspace_slug: workspace.slug,
      workspace_key: workspace.workspace_key,
      organization_id: workspace.organization_id,
      organization_name: organization?.name,
      organization_slug: organization?.slug,
      workspace_role: access?.membership?.role ?? workspace.role,
      membership_status: access?.membership?.status,
      enabled_modules: access?.modules ?? [],
      module_count: access?.modules?.length ?? 0,
      ...extra,
    }),
  };
}

export function trackWorkspaceActivationEvent(
  eventName: string,
  workspace: Workspace | null | undefined,
  access?: WorkspaceAccess | null,
  organization?: OrganizationWithRole | null,
  extra?: Record<string, unknown>,
  options?: AnalyticsOptions,
) {
  if (!workspace) return;
  trackAnalyticsEvent(
    eventName,
    buildAnalyticsWorkspaceEventProperties(eventName, workspace, access, organization, extra),
    options,
  );
}

export function trackWorkspaceFirstValueOnce(
  workspaceId: string,
  module: string,
  milestone: string,
  properties?: Record<string, unknown>,
  options?: AnalyticsOptions,
) {
  if (!workspaceId) return;
  const storageKey = `helpin:activation:${workspaceId}:${module}:${milestone}`;
  try {
    if (localStorage.getItem(storageKey)) return;
    localStorage.setItem(storageKey, new Date().toISOString());
  } catch {
    // Analytics remains best-effort when browser storage is unavailable.
  }
  trackAnalyticsEvent('module_first_value', {
    workspace_id: workspaceId,
    module,
    milestone,
    ...properties,
  }, options);
}

export function trackWorkspaceBillingEvent(
  eventName: string,
  workspaceId: string | undefined,
  billing?: WorkspaceBillingSummary | null,
  extra?: Record<string, unknown>,
  options?: AnalyticsOptions,
) {
  if (!workspaceId) return;
  trackAnalyticsEvent(eventName, buildAnalyticsBillingEventProperties(eventName, workspaceId, billing, extra), options);
}

export function resetAnalytics(options?: AnalyticsOptions) {
  if (!shouldEnableAppAnalytics(hostFor(options))) return;
  const clients = clientsFor(options);
  void clients.usermaven.reset();
  clients.customerio.reset();
}
