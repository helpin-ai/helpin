import { defineConfig, mergeConfig } from 'vitest/config'
import viteConfig from './vite.config'

// vite.config now exports a config *function* (to read env for the dev proxy).
// Resolve it for the test (build) mode so mergeConfig gets a plain config
// object. The env/proxy branch is dev-server-only and inert under vitest.
const resolvedViteConfig = viteConfig({ command: 'build', mode: 'test' })

export default mergeConfig(
  resolvedViteConfig,
  defineConfig({
    test: { environment: 'jsdom', globals: true, setupFiles: ['./src/test-setup.ts'] },
  }),
)
