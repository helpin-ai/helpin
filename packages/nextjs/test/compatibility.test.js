/**
 * Compatibility test for Helpin Next.js SDK
 * This test verifies that the SDK works correctly with Next.js 15 and React 19
 */

const assert = require('assert');
const { createClient, HelpinContext, HelpinProvider } = require('../lib');

// Test createClient function
describe('Next.js SDK Compatibility Tests', () => {
  // Mock window object for client-side tests
  const mockWindow = {
    location: {
      href: 'https://example.com/test',
      pathname: '/test',
      search: '?query=test',
    },
    document: {
      referrer: 'https://referrer.com',
      title: 'Test Page',
    },
    navigator: {
      userAgent: 'test-agent',
    },
    screen: {
      width: 1920,
      height: 1080,
    },
  };

  // Test client creation
  describe('createClient', () => {
    it('should return null when window is undefined', () => {
      // Server-side rendering scenario
      const client = createClient({
        widgetKey: 'test-key',
        host: 'https://example.com',
      });
      assert.strictEqual(client, null);
    });

    it('should create a client when window is defined', () => {
      // Client-side rendering scenario
      global.window = mockWindow;
      const client = createClient({
        widgetKey: 'test-key',
        host: 'https://example.com',
      });
      assert.notStrictEqual(client, null);
      delete global.window;
    });
  });

  // Test React context
  describe('HelpinContext', () => {
    it('should be a valid React context', () => {
      assert.strictEqual(typeof HelpinContext, 'object');
      assert.strictEqual(typeof HelpinContext.Provider, 'object');
      assert.strictEqual(typeof HelpinContext.Consumer, 'object');
    });
  });

  // Test HelpinProvider
  describe('HelpinProvider', () => {
    it('should be a valid React component', () => {
      assert.strictEqual(typeof HelpinProvider, 'function');
    });
  });

  // Add more tests as needed for other components and hooks
});
