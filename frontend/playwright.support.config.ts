import { defineConfig, devices } from '@playwright/test'

export default defineConfig({
  testDir: './e2e/support',
  testIgnore: ['**/live/**'],
  testMatch: /.*\.spec\.ts/,
  timeout: 30_000,
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: 'list',
  webServer: {
    command: process.env.SUPPORT_E2E_PREVIEW ? 'pnpm exec vite preview --host 127.0.0.1 --port 4173' : 'pnpm exec vite --host 127.0.0.1 --port 4173',
    url: process.env.SUPPORT_E2E_PREVIEW ? 'http://127.0.0.1:4173/' : 'http://127.0.0.1:4173/e2e/support/harness/support-presence-harness.html',
    reuseExistingServer: !process.env.CI,
    timeout: 30_000,
  },
  use: {
    baseURL: 'http://127.0.0.1:4173',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
})
