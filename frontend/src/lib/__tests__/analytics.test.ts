import { beforeEach, describe, expect, it, vi } from 'vitest';

import type { OrganizationWithRole, User, Workspace, WorkspaceAccess } from '../types';
import {
  buildAnalyticsOrganizationTraits,
  buildAnalyticsUserTraits,
  buildAnalyticsWorkspaceTraits,
  buildCustomerIoWorkspaceTraits,
  CUSTOMER_IO_WORKSPACE_OBJECT_TYPE_ID,
  getWorkspaceAnalyticsPlan,
  identifyAnalyticsOrganization,
  identifyAnalyticsUser,
  identifyAnalyticsWorkspace,
  initializeAppAnalytics,
  resetAnalytics,
  shouldEnableAppAnalytics,
  trackAnalyticsEvent,
  type AnalyticsClients,
} from '../analytics';

const baseUser: User = {
  id: 'user-1',
  email: 'waqar@example.com',
  full_name: 'Waqar Azeem',
  default_workspace_id: 'ws-1',
  email_verified: true,
  email_verified_at: '2026-07-01T10:00:00Z',
  two_fa_enabled: false,
  created_at: '2026-07-01T09:00:00Z',
  updated_at: '2026-07-02T09:00:00Z',
};

const baseWorkspace: Workspace = {
  id: 'ws-1',
  name: 'Acme',
  slug: 'acme',
  workspace_key: 'acme',
  owner_id: 'user-1',
  organization_id: 'org-1',
  role: 'owner',
  website_url: 'https://www.acme.com',
  company_product_context: 'Acme helps support teams.',
  timezone: 'UTC',
  created_at: '2026-07-01T11:00:00Z',
  updated_at: '2026-07-02T11:00:00Z',
  billing: {
    workspace_id: 'ws-1',
    plan: 'growth',
    status: 'trialing',
    locked: false,
    billing_interval: 'monthly',
    trialing: true,
    trial_ends_at: '2026-07-15T11:00:00Z',
    current_period_start: '2026-07-01T11:00:00Z',
    current_period_end: '2026-07-15T11:00:00Z',
    included_credits: 5000,
    credits_used: 1200,
    credits_remaining: 3800,
    next_charge_cents: 4900,
    on_demand_enabled: false,
    on_demand_available: true,
    cancel_at_period_end: false,
    manage_billing_enabled: true,
    on_demand_blocks_invoiced: 0,
    ai_usage_allowance_microusd: 299_000_000,
    ai_usage_used_microusd: 71_760_000,
    ai_usage_remaining_microusd: 227_240_000,
    ai_usage_reserved_microusd: 0,
    ai_usage_overage_microusd: 0,
    extra_ai_usage_enabled: false,
    extra_ai_usage_available: true,
    pricing_version: '2026-08-13',
    seat_limit: 10,
    seat_usage: 3,
    seat_over_limit: false,
  },
};

const baseOrganization: OrganizationWithRole = {
  id: 'org-1',
  name: 'Acme Inc',
  slug: 'acme-inc',
  owner_id: 'user-1',
  role: 'owner',
  created_at: '2026-07-01T08:00:00Z',
  updated_at: '2026-07-02T08:00:00Z',
};

const baseAccess: WorkspaceAccess = {
  workspace_id: 'ws-1',
  membership: {
    id: 'membership-1',
    user_id: 'user-1',
    role: 'owner',
    status: 'active',
    support_task_dialog_dismissed: false,
  },
  permissions: ['workspace.read', 'settings.manage'],
  team_memberships: [
    { team_id: 'team-1', role: 'owner' },
    { team_id: 'team-2', role: 'member' },
  ],
  modules: ['pm', 'docs', 'support'],
};

function makeClients(): AnalyticsClients {
  return {
    usermaven: {
      init: vi.fn(),
      id: vi.fn(),
      group: vi.fn(),
      track: vi.fn(),
      reset: vi.fn(),
    },
    customerio: {
      load: vi.fn(),
      identify: vi.fn(),
      group: vi.fn(),
      track: vi.fn(),
      reset: vi.fn(),
    },
  };
}

describe('app analytics', () => {
  beforeEach(() => {
    vi.useRealTimers();
  });

  it('is enabled only on app.helpin.ai', () => {
    expect(shouldEnableAppAnalytics('app.helpin.ai')).toBe(true);
    expect(shouldEnableAppAnalytics('stage.helpin.ai')).toBe(false);
    expect(shouldEnableAppAnalytics('helpin.ai')).toBe(false);
    expect(shouldEnableAppAnalytics('localhost')).toBe(false);
  });

  it('initializes both SDKs only on the production app host', () => {
    const disabledClients = makeClients();
    expect(initializeAppAnalytics({ hostname: 'localhost', clients: disabledClients })).toBe(false);
    expect(disabledClients.usermaven.init).not.toHaveBeenCalled();
    expect(disabledClients.customerio.load).not.toHaveBeenCalled();

    const clients = makeClients();
    expect(initializeAppAnalytics({ hostname: 'app.helpin.ai', clients })).toBe(true);
    expect(clients.usermaven.init).toHaveBeenCalledWith({
      key: 'UMpgKYZLxR',
      tracking_host: 'https://events.usermaven.com',
      autocapture: true,
      cookie_domain: 'helpin.ai',
    });
    expect(clients.customerio.load).toHaveBeenCalledWith({
      writeKey: 'a3fced22111b6be05726',
    });
  });

  it('builds useful user-level traits without putting plan on the user', () => {
    expect(buildAnalyticsUserTraits(baseUser, baseOrganization)).toEqual({
      id: 'user-1',
      email: 'waqar@example.com',
      full_name: 'Waqar Azeem',
      first_name: 'Waqar',
      last_name: 'Azeem',
      created_at: '2026-07-01T09:00:00Z',
      updated_at: '2026-07-02T09:00:00Z',
      email_verified: true,
      email_verified_at: '2026-07-01T10:00:00Z',
      default_workspace_id: 'ws-1',
      two_fa_enabled: false,
      mfa_satisfied_in_token: false,
      avatar_style: undefined,
      avatar_background_mode: undefined,
      company: {
        id: 'org-1',
        name: 'Acme Inc',
        created_at: '2026-07-01T08:00:00Z',
        custom: {
          organization_slug: 'acme-inc',
          organization_role: 'owner',
          owner_user_id: 'user-1',
          has_logo: false,
        },
      },
      custom: {
        has_default_workspace: true,
        signup_domain: 'example.com',
        has_avatar: false,
        profile_complete: true,
      },
    });
  });

  it('builds organization traits for Usermaven product analytics', () => {
    expect(buildAnalyticsOrganizationTraits(baseOrganization)).toEqual({
      id: 'org-1',
      name: 'Acme Inc',
      created_at: '2026-07-01T08:00:00Z',
      custom: {
        organization_slug: 'acme-inc',
        owner_user_id: 'user-1',
        organization_role: 'owner',
        has_logo: false,
      },
    });
  });

  it('adds organization workspace billing aggregates without flattening them onto the user', () => {
    const traits = buildAnalyticsOrganizationTraits(baseOrganization, [
      baseWorkspace,
      { ...baseWorkspace, id: 'ws-2', billing: { ...baseWorkspace.billing!, status: 'active', trialing: false } },
    ]);

    expect(traits.custom).toMatchObject({
      workspace_count: 2,
      paid_workspace_count: 1,
      trialing_workspace_count: 1,
      organization_trialing: true,
      organization_plan_mix: ['growth'],
    });
  });

  it('builds workspace/company traits with billing and relationship context', () => {
    expect(buildAnalyticsWorkspaceTraits(baseWorkspace, baseAccess, baseOrganization)).toMatchObject({
      id: 'ws-1',
      name: 'Acme',
      created_at: '2026-07-01T11:00:00Z',
      website: 'https://www.acme.com',
      plan: 'growth',
      custom: {
        workspace_slug: 'acme',
        organization_id: 'org-1',
        organization_name: 'Acme Inc',
        organization_slug: 'acme-inc',
        organization_role: 'owner',
        owner_user_id: 'user-1',
        website_domain: 'acme.com',
        billing_status: 'trialing',
        billing_interval: 'monthly',
        trialing: true,
        trial_days_left: expect.any(Number),
        locked: false,
        ai_usage_used_microusd: 71_760_000,
        ai_usage_remaining_microusd: 227_240_000,
        seat_limit: 10,
        seat_usage: 3,
        has_company_product_context: true,
        modules: ['pm', 'docs', 'support'],
        module_count: 3,
        team_membership_count: 2,
        workspace_role: 'owner',
        membership_status: 'active',
      },
    });
  });

  it('builds Customer.io workspace object traits with object type and relationship attributes', () => {
    expect(buildCustomerIoWorkspaceTraits(baseWorkspace, baseAccess)).toMatchObject({
      objectTypeId: CUSTOMER_IO_WORKSPACE_OBJECT_TYPE_ID,
      name: 'Acme',
      workspace_slug: 'acme',
      organization_id: 'org-1',
      plan: 'growth',
      billing_status: 'trialing',
      trialing: true,
      modules: ['pm', 'docs', 'support'],
      relationshipAttributes: {
        workspace_role: 'owner',
        membership_status: 'active',
        team_membership_count: 2,
      },
    });
  });

  it('uses workspace plan as canonical plan', () => {
    expect(getWorkspaceAnalyticsPlan(baseWorkspace)).toBe('growth');
    expect(getWorkspaceAnalyticsPlan({ ...baseWorkspace, billing: undefined })).toBe('unknown');
  });

  it('identifies users and workspaces in both providers on app.helpin.ai', () => {
    const clients = makeClients();

    identifyAnalyticsUser(baseUser, baseOrganization, { hostname: 'app.helpin.ai', clients });
    identifyAnalyticsWorkspace(baseWorkspace, baseAccess, baseOrganization, { hostname: 'app.helpin.ai', clients });
    identifyAnalyticsOrganization(baseOrganization, baseUser, { hostname: 'app.helpin.ai', clients });

    expect(clients.usermaven.id).toHaveBeenCalledWith(expect.objectContaining({
      id: 'user-1',
      email: 'waqar@example.com',
      company: expect.objectContaining({ id: 'org-1', name: 'Acme Inc' }),
    }));
    expect(clients.customerio.identify).toHaveBeenCalledWith('user-1', expect.objectContaining({ email: 'waqar@example.com' }));
    expect(clients.usermaven.group).not.toHaveBeenCalled();
    expect(clients.customerio.group).toHaveBeenCalledWith('ws-1', expect.objectContaining({
      objectTypeId: '1',
      name: 'Acme',
      plan: 'growth',
      relationshipAttributes: expect.objectContaining({ workspace_role: 'owner' }),
    }));
    expect(clients.usermaven.track).not.toHaveBeenCalled();
  });

  it('does not resend unchanged Usermaven identities after query refetches', () => {
    const clients = makeClients();
    const options = { hostname: 'app.helpin.ai', clients };
    const workspaces = [baseWorkspace];

    identifyAnalyticsUser(baseUser, undefined, options);
    identifyAnalyticsUser({ ...baseUser }, undefined, options);
    identifyAnalyticsOrganization(baseOrganization, baseUser, options, workspaces);
    identifyAnalyticsOrganization(
      { ...baseOrganization },
      { ...baseUser },
      options,
      workspaces.map((workspace) => ({ ...workspace })),
    );

    expect(clients.usermaven.id).toHaveBeenCalledTimes(2);
  });

  it('resends a Usermaven organization identity when meaningful traits change', () => {
    const clients = makeClients();
    const options = { hostname: 'app.helpin.ai', clients };

    identifyAnalyticsOrganization(baseOrganization, baseUser, options, [baseWorkspace]);
    identifyAnalyticsOrganization(baseOrganization, baseUser, options, [
      baseWorkspace,
      { ...baseWorkspace, id: 'ws-2' },
    ]);

    expect(clients.usermaven.id).toHaveBeenCalledTimes(2);
  });


  it('tracks events and resets only on app.helpin.ai', () => {
    const clients = makeClients();

    trackAnalyticsEvent('workspace_created', { workspace_id: 'ws-1' }, { hostname: 'stage.helpin.ai', clients });
    resetAnalytics({ hostname: 'stage.helpin.ai', clients });
    expect(clients.usermaven.track).not.toHaveBeenCalled();
    expect(clients.customerio.track).not.toHaveBeenCalled();
    expect(clients.usermaven.reset).not.toHaveBeenCalled();
    expect(clients.customerio.reset).not.toHaveBeenCalled();

    trackAnalyticsEvent('workspace_created', { workspace_id: 'ws-1' }, { hostname: 'app.helpin.ai', clients });
    resetAnalytics({ hostname: 'app.helpin.ai', clients });
    expect(clients.usermaven.track).toHaveBeenCalledWith('workspace_created', { workspace_id: 'ws-1' });
    expect(clients.customerio.track).toHaveBeenCalledWith('workspace_created', { workspace_id: 'ws-1' });
    expect(clients.usermaven.reset).toHaveBeenCalled();
    expect(clients.customerio.reset).toHaveBeenCalled();
  });

});
