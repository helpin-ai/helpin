import type { KnipConfig } from 'knip';

const enterprise = process.env.VITE_EDITION === 'ee';
const edition = enterprise ? 'src/ee/edition' : 'src/edition/community';

export default {
  workspaces: {
    '.': { entry: ['scripts/**/*.{js,mjs,cjs,ts}'], project: ['scripts/**/*.{js,mjs,cjs,ts}'] },
    frontend: {
      entry: ['src/main.tsx', 'src/routes/**/*.{ts,tsx}'],
      project: ['src/**/*.{ts,tsx}', ...(!enterprise ? ['!src/ee/**'] : [])],
      paths: {
        '@/*': ['./src/*'],
        '@edition': [`./${edition}`],
        '@edition/*': [`./${edition}/*`],
        '@helpin-ai/widget-core': ['../packages/widget-core/src/index.ts'],
      },
    },
    'apps/support-desktop': {
      entry: ['src/main.tsx'],
      project: ['src/**/*.{ts,tsx}'],
      paths: { '@edition': [`../../frontend/${edition}`], '@edition/*': [`../../frontend/${edition}/*`] },
    },
    'apps/support-mobile': { entry: ['src/main.tsx'], project: ['src/**/*.{ts,tsx}'] },
    'apps/admin': { entry: ['src/main.tsx'], project: ['src/**/*.{ts,tsx}'] },
    'apps/email-notice': {},
    'help-center': { entry: ['src/client.tsx', 'src/routes/**/*.{ts,tsx}'], project: ['src/**/*.{ts,tsx}'] },
    website: {},
    'packages/sdk-js': {
      entry: ['src/index.ts', 'src/loader.ts', 'playwright*.config.ts', 'test/e2e/**/*.ts'],
      project: ['src/**/*.ts'],
      // These configs only declare testDir; static entry globs avoid executing Playwright.
      playwright: false,
      includeEntryExports: false,
    },
    'packages/*': {
      // Published package entrypoints are public API, even without local consumers.
      entry: ['src/index.ts', 'src/main.tsx'],
      project: ['src/**/*.{ts,tsx,vue}'],
      includeEntryExports: false,
    },
  },
} satisfies KnipConfig;
