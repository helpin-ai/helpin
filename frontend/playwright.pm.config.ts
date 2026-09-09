import { defineConfig, devices } from '@playwright/test';
export default defineConfig({
  testDir: './e2e/pm', testMatch: '**/*.spec.ts', fullyParallel: true, timeout: 30_000, workers: 2, reporter: 'list',
  webServer: { command: 'pnpm exec vite --host 127.0.0.1 --port 5194 --strictPort', url: 'http://127.0.0.1:5194/e2e/pm/harness/my-work.html', reuseExistingServer: !process.env.CI, timeout: 60_000 },
  use: { baseURL: 'http://127.0.0.1:5194', trace: 'retain-on-failure', screenshot: 'only-on-failure' },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'], viewport: { width: 1440, height: 1000 } } }],
});
