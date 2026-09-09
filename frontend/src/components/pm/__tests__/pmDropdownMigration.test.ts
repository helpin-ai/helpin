import { readFileSync, readdirSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

function sources(directory: string): string[] {
  return readdirSync(directory, { withFileTypes: true }).flatMap(entry => {
    if (entry.name === '__tests__') return [];
    const path = resolve(directory, entry.name);
    return entry.isDirectory() ? sources(path) : path.endsWith('.tsx') ? [path] : [];
  });
}

describe('PM shared dropdown migration', () => {
  it('routes option lists through the shared dropdown instead of raw Select/Command or native select', () => {
    const files = [...sources(resolve(__dirname, '..')), ...sources(resolve(__dirname, '../../../pages/pm'))];
    const legacy = files.filter(file => /from\s+['"]@\/components\/ui\/(?:command|select)['"]|<select\b/.test(readFileSync(file, 'utf8')));
    expect(legacy).toEqual([]);
  });
});
