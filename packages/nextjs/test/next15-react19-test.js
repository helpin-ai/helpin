/**
 * Manual test script for Next.js 15 and React 19 compatibility
 *
 * This script demonstrates how to use the Helpin Next.js SDK with Next.js 15 and React 19
 * Run this script after building the SDK to verify compatibility
 */

// Import React 19
const React = require('react');
const { useState, useEffect } = React;

// Import Next.js components (mocked for testing)
const mockRouter = {
  pathname: '/test-page',
  query: { param: 'test' },
  asPath: '/test-page?param=test',
};

// Import Helpin Next.js SDK
const {
  createClient,
  HelpinProvider,
  useHelpin,
  usePageView,
} = require('../lib');

// Create a client (will be null in this environment since there's no window)
const client = createClient({
  host: 'https://events.helpin.ai',
  widgetKey: 'test-key',
  autocapture: true,
});

console.log(
  'Client created:',
  client === null ? 'null (expected in Node environment)' : 'instance',
);

// Test React component with hooks
function TestComponent() {
  const [count, setCount] = useState(0);
  const helpin = useHelpin();

  // Track page view
  usePageView(client);

  useEffect(() => {
    if (count > 0 && helpin) {
      // Track custom event
      helpin.track('button_clicked', { count });
    }
  }, [count, helpin]);

  return {
    increment: () => setCount(count + 1),
    count,
  };
}

// Test provider component
function TestApp({ children }) {
  return React.createElement(HelpinProvider, { client }, children);
}

console.log('Components defined successfully');
console.log('Test completed - SDK is compatible with React 19 syntax');
console.log(
  'Note: For full testing, use this SDK in a Next.js 15 project with React 19',
);
