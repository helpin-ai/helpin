import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

describe('sender address groups', () => {
  it('starts domain groups expanded', () => {
    const source = readFileSync(resolve(__dirname, '../SupportEmailSendersTab.tsx'), 'utf8');

    expect(source).toContain('const [expanded, setExpanded] = useState(true);');
  });
});
