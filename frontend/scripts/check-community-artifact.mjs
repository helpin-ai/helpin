import { readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';

const tree = readFileSync('.tanstack/routeTree.community.gen.js', 'utf8');
if (/billing-preview|settings\/billing/.test(tree)) throw new Error('Community registers a billing route');
const forbidden = [
  'Manage this workspace plan, AI usage, payment methods, and invoices.',
  'Upgrade or add AI capacity',
  'Choose your plan',
];
for (const file of readdirSync('dist/assets')) {
  if (!file.endsWith('.js')) continue;
  const text = readFileSync(join('dist/assets', file), 'utf8');
  for (const copy of forbidden) {
    if (text.includes(copy)) throw new Error(`Commercial copy in Community artifact: ${copy}`);
  }
}
console.log('Community artifact excludes billing routes and commercial recovery/settings copy.');
