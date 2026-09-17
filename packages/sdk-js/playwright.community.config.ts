import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './test/e2e/widget/live',
  testMatch: 'community.spec.ts',
  timeout: 90000,
  workers: 1,
  retries: 0,
  reporter: 'list',
  // Traces contain authentication responses. Keep them out of CI artifacts.
  use: { ...devices['Desktop Chrome'], trace: 'off', screenshot: 'only-on-failure' },
});
