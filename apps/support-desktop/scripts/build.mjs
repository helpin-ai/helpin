import { spawnSync } from 'node:child_process';

const edition = process.env.VITE_EDITION || 'community';
if (!['community', 'ee'].includes(edition)) throw new Error('Unknown VITE_EDITION');
const config = edition === 'ee' ? 'tsconfig.ee.json' : 'tsconfig.app.json';
for (const args of [['exec', 'tsc', '-b', config, 'tsconfig.node.json'], ['exec', 'vite', 'build']]) {
  const result = spawnSync('pnpm', args, {
    stdio: 'inherit',
    env: { ...process.env, VITE_EDITION: edition, NODE_OPTIONS: process.env.NODE_OPTIONS || '--max-old-space-size=4096' },
    shell: process.platform === 'win32',
  });
  if (result.error) throw result.error;
  if (result.status !== 0) process.exit(result.status ?? 1);
}
