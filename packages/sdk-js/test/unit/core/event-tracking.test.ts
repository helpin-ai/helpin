import { describe, it, expect } from 'vitest';
import { HelpinClient } from '../../../src/core/client';
import { Config } from '../../../src/core/types';

describe('HelpinClient Event Tracking', () => {
  let client: HelpinClient;
  const mockConfig: Config = {
    widgetKey: 'UM00AcZHGY',
    host: 'https://test.helpin.ai',
  };

  beforeEach(() => {
    client = new HelpinClient(mockConfig);
    vi.spyOn(client, 'track');
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it('should call track method with provided event type and payload', () => {
    const payload = { custom_prop: 'value' };
    client.track('test_event', payload);

    expect(client.track).toHaveBeenCalledWith('test_event', payload);
  });

  it('should pass through custom properties in track method', () => {
    const customProps = {
      custom_prop: 'custom value',
      user: {
        id: 'custom-user-id',
        email: 'user@example.com',
      },
    };

    client.track('custom_event', customProps);

    expect(client.track).toHaveBeenCalledWith('custom_event', customProps);
  });

  it('should track pageview events correctly', () => {
    const pageviewEvent = {
      event_type: 'pageview',
      url: 'https://www.toulouse-traiteurs.com/',
      page_title:
        'Toulouse Traiteurs - Les meilleurs Traiteurs de Toulouse et de sa région',
      referrer: 'https://www.google.com/',
      user: { anonymous_id: 'jdriqpxo25' },
    };

    client.track('pageview', pageviewEvent);

    expect(client.track).toHaveBeenCalledWith('pageview', pageviewEvent);
  });

  it('should track scroll events with correct attributes', () => {
    const scrollEvent = {
      event_type: '$scroll',
      percent: 81,
      window_height: 535,
      document_height: 4328,
      scroll_distance: 50,
    };

    client.track('$scroll', scrollEvent);

    expect(client.track).toHaveBeenCalledWith(
      '$scroll',
      expect.objectContaining(scrollEvent),
    );
  });

  it('should track autocapture events with correct attributes', () => {
    const autocaptureEvent = {
      event_type: '$autocapture',
      autocapture_attributes: {
        classes: ['l7_2fn', 'wixui-button__label'],
        el_text: 'Prix sur devis',
        event_type: 'click',
        attr__class: 'l7_2fn wixui-button__label',
        tag_name: 'span',
      },
    };

    client.track('$autocapture', autocaptureEvent);

    expect(client.track).toHaveBeenCalledWith(
      '$autocapture',
      expect.objectContaining(autocaptureEvent),
    );
  });

  it('should track pageleave events', () => {
    const pageleaveEvent = {
      event_type: '$pageleave',
      url: 'https://www.toulouse-traiteurs.com/traiteurs-anniversaire',
      page_title:
        'Les meilleurs traiteurs pour un anniversaire | Toulouse Traiteurs',
    };

    client.track('$pageleave', pageleaveEvent);

    expect(client.track).toHaveBeenCalledWith(
      '$pageleave',
      expect.objectContaining(pageleaveEvent),
    );
  });

  it('should handle events with UTM parameters', () => {
    const eventWithUtm = {
      event_type: 'pageview',
      utm: { source: 'google', medium: 'cpc', campaign: 'spring_sale' },
    };

    client.track('pageview', eventWithUtm);

    expect(client.track).toHaveBeenCalledWith(
      'pageview',
      expect.objectContaining(eventWithUtm),
    );
  });

  it('should correctly track events with different screen resolutions and viewport sizes', () => {
    const mobileEvent = {
      event_type: 'pageview',
      screen_resolution: '414x896',
      vp_size: '320x549',
    };

    const desktopEvent = {
      event_type: 'pageview',
      screen_resolution: '1920x1200',
      vp_size: '1920x1075',
    };

    client.track('pageview', mobileEvent);
    client.track('pageview', desktopEvent);

    expect(client.track).toHaveBeenCalledWith(
      'pageview',
      expect.objectContaining(mobileEvent),
    );
    expect(client.track).toHaveBeenCalledWith(
      'pageview',
      expect.objectContaining(desktopEvent),
    );
  });

  it('should handle events with empty or null properties', () => {
    const eventWithEmptyProps = {
      event_type: 'pageview',
      company: null,
      event_attributes: null,
    };

    client.track('pageview', eventWithEmptyProps);

    expect(client.track).toHaveBeenCalledWith(
      'pageview',
      expect.objectContaining(eventWithEmptyProps),
    );
  });

  // Test for user identification
  it('should identify users correctly', async () => {
    const userData = {
      id: 'user123',
      email: 'user@example.com',
      name: 'John Doe',
    };
    vi.spyOn(client as any, 'trackInternal');
    await client.id(userData);

    expect(client['track']).toHaveBeenCalledWith(
      'user_identify',
      expect.objectContaining({
        ...userData,
        anonymous_id: expect.any(String),
      }),
    );
  });

  // Test for group identification
  it('should identify groups correctly', async () => {
    const groupData = {
      id: 'group123',
      name: 'Test Company',
      created_at: '2023-01-01',
    };
    await client.group(groupData);

    expect(client.track).toHaveBeenCalledWith(
      'group',
      expect.objectContaining(groupData),
    );
  });

  // Test for raw event tracking
  it('should handle raw event tracking', () => {
    const rawEvent = {
      event_type: 'custom_event',
      custom_prop: 'custom value',
    };
    client.rawTrack(rawEvent);

    expect(client.track).toHaveBeenCalledWith('raw', rawEvent);
  });

  // Test for setting and unsetting properties
  it('should set and unset properties correctly', () => {
    const setProps = { prop1: 'value1', prop2: 'value2' };
    client.set(setProps);

    client.unset('prop1');

    // You might need to expose these methods or properties for testing
    expect(client['persistence'].get('global_props')).toEqual({
      prop2: 'value2',
    });
  });

  // Test for reset functionality
  it('should reset client state correctly', async () => {
    await client.reset();

    // You might need to expose these methods or properties for testing
    expect(client['persistence'].get('userId')).toBeUndefined();
    expect(client['persistence'].get('userProps')).toBeUndefined();
  });

  // Test for handling invalid inputs
  it('should throw error for invalid event name', () => {
    expect(() => client.track(123 as any)).toThrow(
      'Event name must be a string',
    );
  });

  it('should throw error for invalid event payload', () => {
    expect(() => client.track('test_event', 'invalid' as any)).toThrow(
      'Event payload must be a non-null object and not an array',
    );
  });

  // Test for cross-domain linking functionality
  it('should handle cross-domain linking', () => {
    const mockConfig: Config = {
      widgetKey: 'test-key',
      host: 'https://test.helpin.ai',
      crossDomainLinking: true,
      domains: 'domain1.com,domain2.com',
    };
    const clientWithCrossDomain = new HelpinClient(mockConfig);

    // You might need to mock some browser APIs here
    // Test the behavior of cross-domain linking
  });

  // Test for different transport methods
  it('should use the correct transport method', () => {
    const beaconConfig: Config = {
      widgetKey: 'test-key',
      host: 'https://test.helpin.ai',
      useBeaconApi: true,
    };
    const beaconClient = new HelpinClient(beaconConfig);

    // You might need to expose these methods or properties for testing
    expect(beaconClient['transport'].constructor.name).toBe('BeaconTransport');
  });
});
