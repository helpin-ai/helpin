import { defineConfig, devices } from '@playwright/test';
import path from 'path';

export default defineConfig({
  testDir: './test/e2e',
  timeout: 60000,
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 1,
  workers: process.env.CI ? 1 : undefined,
  reporter: 'html',

  webServer: {
    command: 'npm run serve:test',
    url: 'http://localhost:3000',
    reuseExistingServer: !process.env.CI,
    timeout: 10000,
  },

  use: {
    baseURL: 'http://localhost:3000',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    actionTimeout: 15000,
    navigationTimeout: 30000,
  },

  projects: [
    // BrowserStack projects
    {
      name: 'browserstack-windows-chrome',
      use: {
        browserName: 'chromium',
        channel: 'chrome',
        viewport: { width: 1920, height: 1080 },
      },
    },
    {
      name: 'browserstack-mac-safari',
      use: {
        browserName: 'webkit',
        viewport: { width: 1920, height: 1080 },
      },
    },
    {
      name: 'browserstack-mobile',
      use: {
        ...devices['Galaxy S23 Ultra'],
        browserName: 'chromium',
        channel: 'chrome',
        launchOptions: {
          slowMo: 1000,
        },
        viewport: null,
        deviceScaleFactor: 1,
        isMobile: true,
        hasTouch: true,
        contextOptions: {
          reducedMotion: 'reduce',
          forcedColors: 'none',
        },
      },
    },
    {
      name: 'browserstack-ios',
      use: {
        ...devices['iPhone 12'],
        browserName: 'webkit',
        launchOptions: {
          slowMo: 1000,
        },
        viewport: null,
        deviceScaleFactor: 1,
        isMobile: true,
        hasTouch: true,
        contextOptions: {
          reducedMotion: 'reduce',
          forcedColors: 'none',
        },
      },
    },
    {
      name: 'browserstack-android',
      use: {
        browserName: 'chromium',
        channel: 'chrome',
        launchOptions: {
          slowMo: 1000,
        },
        viewport: { width: 360, height: 800 },
        deviceScaleFactor: 1,
        isMobile: true,
        hasTouch: true,
        contextOptions: {
          reducedMotion: 'reduce',
          forcedColors: 'none',
        },
      },
    },
  ],
});
