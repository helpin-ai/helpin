import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { HelpinClient } from '../../../src/core/client';
import { Config } from '../../../src/core/types';

describe('HelpinClient', () => {
  let client: HelpinClient;
  let addSpy: ReturnType<typeof vi.spyOn>;
  const mockConfig: Config = {
    widgetKey: 'test-api-key',
    host: 'https://test.helpin.ai',
  };

  beforeEach(() => {
    client = new HelpinClient(mockConfig);
    vi.spyOn(client, 'track');
    addSpy = vi.spyOn(client['retryQueue'], 'add').mockImplementation(() => {});
    vi.spyOn(client['transport'], 'send').mockImplementation(() =>
      Promise.resolve(),
    );
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe('id method', () => {
    it('should not send event when doNotSendEvent is true', async () => {
      const userData = { id: 'user123', email: 'test@example.com' };
      await client.id(userData, true);

      expect(client['retryQueue'].add).not.toHaveBeenCalled();
    });

    it('should throw an error for invalid email', async () => {
      const userData = { id: 'user123', email: 'invalid-email' };
      await expect(client.id(userData)).rejects.toThrow(
        'Invalid email provided',
      );
    });

    it('should carry identified email into lazy widget boot', async () => {
      const widgetController = {
        boot: vi.fn(),
        shutdown: vi.fn(),
        show: vi.fn(),
        hide: vi.fn(),
        open: vi.fn(),
        close: vi.fn(),
        toggle: vi.fn(),
        openMessages: vi.fn(),
        openNewMessage: vi.fn(),
        openConversation: vi.fn(),
        openArticle: vi.fn(),
        onOpen: vi.fn(),
        onClose: vi.fn(),
        onUnreadCountChange: vi.fn(),
        onUserEmailSupplied: vi.fn(),
        onConversationStarted: vi.fn(),
        onMessageReceived: vi.fn(),
        getVisitorId: vi.fn(() => ''),
        isWidgetReady: vi.fn(() => false),
      };
      const lazyClient = new HelpinClient(mockConfig, widgetController);

      await lazyClient.id(
        {
          id: 'user123',
          email: 'test@example.com',
          name: 'Test User',
        },
        true,
      );

      lazyClient.open();

      expect(widgetController.boot).toHaveBeenCalledWith({
        widgetKey: 'test-api-key',
        host: 'https://test.helpin.ai',
        user: {
          email: 'test@example.com',
          name: 'Test User',
          userId: 'user123',
        },
      });
      expect(widgetController.open).toHaveBeenCalled();
    });

    it('uses the latest grouped company when the widget boots after an account switch', async () => {
      const widgetController = {
        boot: vi.fn(),
        shutdown: vi.fn(),
        show: vi.fn(),
        hide: vi.fn(),
        open: vi.fn(),
        close: vi.fn(),
        toggle: vi.fn(),
        openMessages: vi.fn(),
        openNewMessage: vi.fn(),
        openConversation: vi.fn(),
        openArticle: vi.fn(),
        onOpen: vi.fn(),
        onClose: vi.fn(),
        onUnreadCountChange: vi.fn(),
        onUserEmailSupplied: vi.fn(),
        onConversationStarted: vi.fn(),
        onMessageReceived: vi.fn(),
        getVisitorId: vi.fn(() => ''),
        isWidgetReady: vi.fn(() => false),
      };
      const lazyClient = new HelpinClient(mockConfig, widgetController);
      const originalCompany = {
        id: 'account-1',
        name: 'Account One',
        created_at: '2025-01-01',
      };
      const activeCompany = {
        id: 'account-2',
        name: 'Account Two',
        created_at: '2025-02-01',
      };

      await lazyClient.id({ id: 'user-1', company: originalCompany }, true);
      await lazyClient.group(activeCompany, true);
      lazyClient.open();

      expect(widgetController.boot).toHaveBeenCalledWith(
        expect.objectContaining({
          user: expect.objectContaining({ company: activeCompany }),
        }),
      );
    });

    it('should include company data in backend identify payload', async () => {
      const originalFetch = globalThis.fetch;
      const fetchSpy = vi.fn(() => Promise.resolve({ ok: true } as Response));
      globalThis.fetch = fetchSpy as any;

      try {
        await client.id(
          {
            id: 'user123',
            email: 'test@example.com',
            first_name: 'Test',
            last_name: 'User',
            phone: '+1 555 0100',
            job_title: 'VP Revenue',
            company: {
              id: 'company123',
              name: 'Test Company',
              domain: 'test.example',
              created_at: '2024-01-15T00:00:00Z',
              plan: 'enterprise',
            },
          },
          true,
        );

        expect(fetchSpy).toHaveBeenCalledWith(
          'https://test.helpin.ai/widget/identify',
          expect.objectContaining({
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
          }),
        );
        const body = JSON.parse(fetchSpy.mock.calls[0][1].body);
        expect(body).toEqual(
          expect.objectContaining({
            api_key: 'test-api-key',
            email: 'test@example.com',
            first_name: 'Test',
            last_name: 'User',
            phone: '+1 555 0100',
            job_title: 'VP Revenue',
            source: 'sdk_identify',
            company: {
              id: 'company123',
              name: 'Test Company',
              domain: 'test.example',
              created_at: '2024-01-15T00:00:00Z',
              plan: 'enterprise',
            },
          }),
        );
      } finally {
        globalThis.fetch = originalFetch;
      }
    });

    it('uses a previously grouped company when id has no inline company', async () => {
      const originalFetch = globalThis.fetch;
      const fetchSpy = vi.fn(() => Promise.resolve({ ok: true } as Response));
      globalThis.fetch = fetchSpy as any;
      try {
        const company = { id: 'account-1', name: 'Account One', created_at: '2025-01-01' };
        await client.group(company, true);
        await client.id({ id: 'user-1', email: 'user@example.com' }, true);
        const body = JSON.parse(fetchSpy.mock.calls.at(-1)?.[1].body);
        expect(body.company).toEqual(company);
      } finally {
        globalThis.fetch = originalFetch;
      }
    });
  });

  describe('shutdown', () => {
    it('rotates the anonymous visitor identity', () => {
      const resetSpy = vi.spyOn(client, 'reset');

      client.shutdown();

      expect(resetSpy).toHaveBeenCalledWith(true);
    });
  });

  describe('track method', () => {
    it('should track an event with correct payload', () => {
      const eventName = 'test_event';
      const eventPayload = { key: 'value' };
      expect(() => client.track(eventName, eventPayload)).not.toThrow();
    });

    it('should throw an error for non-string event names', () => {
      expect(() => client.track(123 as any)).toThrow(
        'Event name must be a string',
      );
    });

    it('should throw an error for non-object payloads', () => {
      expect(() =>
        client.track('test_event', 'invalid_payload' as any),
      ).toThrow('Event payload must be a non-null object and not an array');
    });

    it('should not throw an error when payload is undefined', () => {
      expect(() => client.track('test_event')).not.toThrow();
    });

    it('should throw an error for null payload', () => {
      expect(() => client.track('test_event', null as any)).toThrow(
        'Event payload must be a non-null object and not an array',
      );
    });

    it('should throw an error for array payload', () => {
      expect(() => client.track('test_event', [] as any)).toThrow(
        'Event payload must be a non-null object and not an array',
      );
    });

    it('should not throw an error for empty object payload', () => {
      expect(() => client.track('test_event', {})).not.toThrow();
    });

    it('should not throw an error for complex nested object payload', () => {
      const complexPayload = {
        user: {
          id: 1,
          name: 'John Doe',
          preferences: {
            theme: 'dark',
            notifications: true,
          },
        },
        items: [
          { id: 1, name: 'Item 1' },
          { id: 2, name: 'Item 2' },
        ],
      };
      expect(() => client.track('test_event', complexPayload)).not.toThrow();
    });
  });

  describe('lead method', () => {
    it('should track lead event when payload includes valid email', () => {
      const errorSpy = vi.spyOn(client['logger'], 'error');
      const payload = { email: 'lead@example.com', name: 'Lead User' };

      client.lead(payload);

      expect(client.track).toHaveBeenCalledWith('lead', payload, false);
      expect(errorSpy).not.toHaveBeenCalled();
    });

    it('should not track lead event when email is missing', () => {
      const errorSpy = vi.spyOn(client['logger'], 'error');

      client.lead({ name: 'Lead User' } as any);

      expect(client.track).not.toHaveBeenCalled();
      expect(errorSpy).toHaveBeenCalledWith(
        'Lead event requires a valid email attribute',
      );
    });

    it('should not track lead event when email is invalid', () => {
      const errorSpy = vi.spyOn(client['logger'], 'error');

      client.lead({ email: 'invalid-email', name: 'Lead User' });

      expect(client.track).not.toHaveBeenCalled();
      expect(errorSpy).toHaveBeenCalledWith(
        'Lead event requires a valid email attribute',
      );
    });

    it('should track lead event with direct send flag', () => {
      const errorSpy = vi.spyOn(client['logger'], 'error');
      const payload = { email: 'lead@example.com' };

      client.lead(payload, true);

      expect(client.track).toHaveBeenCalledWith('lead', payload, true);
      expect(errorSpy).not.toHaveBeenCalled();
    });

    it('should throw when payload is not an object', () => {
      expect(() => client.lead('invalid' as any)).toThrow(
        'Lead payload must be a non-null object and not an array',
      );
    });

    it('should elevate a lead company object to the top-level company payload', () => {
      client.lead({
        email: 'lead@example.com',
        first_name: 'Lead',
        last_name: 'User',
        company: {
          id: 'company123',
          name: 'Test Company',
          created_at: '2023-01-01',
        },
        source: 'landing-page',
      });

      expect(addSpy).toHaveBeenCalled();
      const queuedPayload = addSpy.mock.calls.at(-1)?.[0] as any;

      expect(queuedPayload.company).toEqual({
        id: 'company123',
        name: 'Test Company',
        created_at: '2023-01-01',
      });
      expect(queuedPayload.event_attributes).toEqual({
        email: 'lead@example.com',
        first_name: 'Lead',
        last_name: 'User',
        source: 'landing-page',
      });
      expect(queuedPayload.event_attributes.company).toBeUndefined();
    });

    it('does not inherit the persisted customer company for an unrelated lead', async () => {
      await client.group({
        id: 'customer-account',
        name: 'Customer Account',
        created_at: '2025-01-01',
      }, true);
      addSpy.mockClear();

      client.lead({ email: 'lead@example.com', name: 'New Lead' });

      const queuedPayload = addSpy.mock.calls.at(-1)?.[0] as any;
      expect(queuedPayload.company).toBeUndefined();
    });
  });

  describe('group method', () => {
    it('should set company properties and send group event', async () => {
      const companyProps = {
        id: 'company123',
        name: 'Test Company',
        created_at: '2023-01-01',
      };
      await client.group(companyProps);

      expect(client.track).toHaveBeenCalledWith(
        'group',
        expect.objectContaining(companyProps),
      );
    });

    it('should not send event when doNotSendEvent is true', async () => {
      const companyProps = {
        id: 'company123',
        name: 'Test Company',
        created_at: '2023-01-01',
      };
      await client.group(companyProps, true);

      expect(client.track).not.toHaveBeenCalled();
    });

    it('syncs a new active company when an identified email is stored', async () => {
      const originalFetch = globalThis.fetch;
      const fetchSpy = vi.fn(() => Promise.resolve({ ok: true } as Response));
      globalThis.fetch = fetchSpy as any;
      try {
        await client.id({ id: 'user-1', email: 'user@example.com', name: 'User One' }, true);
        fetchSpy.mockClear();
        const company = { id: 'account-2', name: 'Account Two', created_at: '2025-02-01' };
        await client.group(company, true);
        const body = JSON.parse(fetchSpy.mock.calls[0][1].body);
        expect(body).toEqual(expect.objectContaining({
          email: 'user@example.com',
          source: 'sdk_group',
          company,
        }));
      } finally {
        globalThis.fetch = originalFetch;
      }
    });

    it('should throw an error for invalid company properties', async () => {
      const invalidProps = { id: 'company123' };
      await expect(client.group(invalidProps as any)).rejects.toThrow(
        'Company properties must include id, name, and created_at',
      );
    });
  });

  describe('pageview method', () => {
    it('should track a pageview event', () => {
      client.pageview();

      expect(client.track).toHaveBeenCalledWith(
        'pageview',
        expect.objectContaining({
          url: expect.any(String),
          referrer: expect.any(String),
          title: expect.any(String),
        }),
        true,
      );
    });
  });

  describe('articleView method', () => {
    it('includes article identity and browser-claimed provenance', async () => {
      await client.id({ id: 'user123', email: 'buyer@example.com' }, true);
      client.articleView('article-42', { collection_id: 'collection-1' });

      expect(client.track).toHaveBeenCalledWith('article_view', {
        article_id: 'article-42',
        collection_id: 'collection-1',
        identity_method: 'sdk_identify',
        identity_trust: 'probabilistic',
      });
    });

    it('rejects an empty article ID', () => {
      expect(() => client.articleView('  ')).toThrow('articleId is required');
    });
  });

  describe('event_id handling', () => {
    it('should generate a unique event_id per tracked event', () => {
      client.track('signed_up', { source: 'test' });
      client.track('signed_up', { source: 'test' });

      const firstId = (addSpy.mock.calls[0][0] as any).event_id;
      const secondId = (addSpy.mock.calls[1][0] as any).event_id;

      expect(firstId).toBeDefined();
      expect(firstId).not.toBe('');
      expect(secondId).toBeDefined();
      expect(secondId).not.toBe('');
      expect(firstId).not.toEqual(secondId);
    });

    it('should respect a provided event_id and avoid echoing it inside event_attributes', () => {
      client.track('signed_up', { event_id: 'custom-id', plan: 'pro' });

      const payload = addSpy.mock.calls[0][0] as any;

      expect(payload.event_id).toBe('custom-id');
      expect(payload.event_attributes).toMatchObject({ plan: 'pro' });
      expect(payload.event_attributes).not.toHaveProperty('event_id');
    });
  });

  describe('reset method', () => {
    it('should reset client state', async () => {
      const persistenceSpy = vi.spyOn(client['persistence'], 'clear');
      let cookieManagerDeleteCalled = false;

      if (client['cookieManager']) {
        vi.spyOn(client['cookieManager'], 'delete').mockImplementation(() => {
          cookieManagerDeleteCalled = true;
        });
      }

      await client.reset();

      expect(persistenceSpy).toHaveBeenCalled();
      expect(cookieManagerDeleteCalled).toBe(false);
    });

    it('should reset client state and anonymous id when resetAnonId is true', async () => {
      const persistenceSpy = vi.spyOn(client['persistence'], 'clear');
      let cookieManagerDeleteCalled = false;

      if (client['cookieManager']) {
        vi.spyOn(client['cookieManager'], 'delete').mockImplementation(() => {
          cookieManagerDeleteCalled = true;
        });
      }

      await client.reset(true);

      expect(persistenceSpy).toHaveBeenCalled();
      if (client['cookieManager']) {
        expect(cookieManagerDeleteCalled).toBe(true);
      } else {
        expect(cookieManagerDeleteCalled).toBe(false);
      }
    });
  });
});
