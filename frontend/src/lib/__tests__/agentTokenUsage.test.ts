import { describe, expect, it } from 'vitest';

import { formatRunTokenUsage } from '../agentTokenUsage';

describe('formatRunTokenUsage', () => {
  it('renders split usage for native sdk runs', () => {
    expect(formatRunTokenUsage({
      runtime_kind: 'native_sdk',
      input_tokens: 500_000,
      output_tokens: 20_000,
      tokens_used: 520_000,
    })).toBe('500k input / 20k output');
  });

  it('falls back to total usage for non-native runtimes', () => {
    expect(formatRunTokenUsage({
      runtime_kind: 'codex',
      input_tokens: 500_000,
      output_tokens: 20_000,
      tokens_used: 520_000,
    }, { includeUnit: true })).toBe('520k tokens');
  });

  it('falls back to total usage when native split values are unavailable', () => {
    expect(formatRunTokenUsage({
      runtime_kind: 'native_sdk',
      input_tokens: 0,
      output_tokens: 0,
      tokens_used: 12_300,
    }, { includeUnit: true })).toBe('12.3k tokens');
  });
});
