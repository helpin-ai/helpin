import { defineConfig, devices } from '@playwright/test';
export default defineConfig({
 testDir: './e2e/automation', fullyParallel: true, timeout: 30_000, workers: 2, reporter: 'list',
 webServer: { command: 'pnpm exec vite --mode test --host 127.0.0.1 --port 5196 --strictPort', url: 'http://127.0.0.1:5196/e2e/automation/harness/semantic-condition.html', reuseExistingServer: !process.env.CI, timeout: 60_000 },
 use: { baseURL: 'http://127.0.0.1:5196', trace: 'retain-on-failure' },
 projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
});
