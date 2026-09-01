import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

describe('ReplyComposer chrome', () => {
  it('keeps the Docs brand focus ring and Support note emphasis', () => {
    const source = readFileSync(resolve(process.cwd(), 'src/components/support/ReplyComposer.tsx'), 'utf8');

    expect(source).toContain('border-ring bg-background ring-1 ring-ring/40');
    expect(source).toContain('border-amber-400 dark:border-amber-500');
    expect(source).toContain('rounded-xl border border-border/40 bg-card');
  });
});
