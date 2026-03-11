import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { HelpinClient } from '../../../src/core/client';
import { Config } from '../../../src/core/types';

describe('HelpinClient', () => {
  let client: HelpinClient;
  let addSpy: ReturnType<typeof vi.spyOn>;
  const mockConfig: Config = {
    key: 'test-api-key',
    trackingHost: 'https://test.helpin.ai',
    tracking_host: 'https://test.helpin.ai',
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

    it('should throw an error for invalid email', () => {
      const userData = { id: 'user123', email: 'invalid-email' };
      expect(() => client.id(userData)).rejects.toThrow(
        'Invalid email provided',
      );
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

    it('should throw an error for invalid company properties', () => {
      const invalidProps = { id: 'company123' };
      expect(() => client.group(invalidProps as any)).rejects.toThrow(
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
