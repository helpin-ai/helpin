import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './test/e2e/widget',
  testIgnore: ['**/live/**'],
  timeout: 60000,
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: [['html'], ['list']],
  use: {
    baseURL: 'http://localhost:3017',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
    {
      name: 'firefox',
      use: { ...devices['Desktop Firefox'] },
    },
  ],
  webServer: [
    {
      command: 'pnpm exec http-server . -p 3017',
      url: 'http://localhost:3017',
      reuseExistingServer: false,
      timeout: 30000,
    },
  ],
});
