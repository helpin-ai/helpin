import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './e2e/crm', testMatch: '**/*.spec.ts', fullyParallel: true,
  timeout: 45_000, workers: 2, reporter: 'list',
  webServer: {
    command: 'pnpm exec vite --host 127.0.0.1 --port 5193 --strictPort',
    url: 'http://127.0.0.1:5193/e2e/crm/harness/playbooks.html', reuseExistingServer: !process.env.CI, timeout: 60_000,
  },
  use: { baseURL: 'http://127.0.0.1:5193', trace: 'retain-on-failure', screenshot: 'only-on-failure' },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'], viewport: { width: 1440, height: 1000 } } }],
});
