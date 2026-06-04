import { describe, expect, it } from 'vitest';

import { formatRunTokenUsage, formatRunTokenUsageBreakdown, formatRunTokenUsageTotal } from '../agentTokenUsage';

describe('formatRunTokenUsage', () => {
  it('renders split usage for native sdk runs', () => {
    expect(formatRunTokenUsage({
      runtime_kind: 'native_sdk',
      cached_input_tokens: 0,
      input_tokens: 500_000,
      output_tokens: 20_000,
      tokens_used: 520_000,
    })).toBe('500k input / 20k output');
  });

  it('renders cached input tokens when present for native sdk runs', () => {
    expect(formatRunTokenUsage({
      runtime_kind: 'native_sdk',
      cached_input_tokens: 120_000,
      input_tokens: 500_000,
      output_tokens: 20_000,
      tokens_used: 520_000,
    })).toBe('500k input (120k cached) / 20k output');
  });

  it('renders split usage for codex runs', () => {
    expect(formatRunTokenUsage({
      runtime_kind: 'codex',
      cached_input_tokens: 0,
      input_tokens: 500_000,
      output_tokens: 20_000,
      tokens_used: 520_000,
    }, { includeUnit: true })).toBe('500k input / 20k output');
  });

  it('renders cached input tokens when present for codex runs', () => {
    expect(formatRunTokenUsage({
      runtime_kind: 'codex',
      cached_input_tokens: 120_000,
      input_tokens: 500_000,
      output_tokens: 20_000,
      tokens_used: 520_000,
    })).toBe('500k input (120k cached) / 20k output');
  });

  it('falls back to total usage when native split values are unavailable', () => {
    expect(formatRunTokenUsage({
      runtime_kind: 'native_sdk',
      cached_input_tokens: 0,
      input_tokens: 0,
      output_tokens: 0,
      tokens_used: 12_300,
    }, { includeUnit: true })).toBe('12.3k tokens');
  });
});

describe('formatRunTokenUsageTotal', () => {
  it('renders total tokens without split details for table cells', () => {
    expect(formatRunTokenUsageTotal({
      runtime_kind: 'codex',
      cached_input_tokens: 119_000,
      input_tokens: 120_000,
      output_tokens: 722,
      tokens_used: 120_722,
    }, { includeUnit: true })).toBe('121k tokens');
  });

  it('falls back to input plus output when total is missing', () => {
    expect(formatRunTokenUsageTotal({
      runtime_kind: 'native_sdk',
      cached_input_tokens: 20_000,
      input_tokens: 70_000,
      output_tokens: 5_000,
      tokens_used: 0,
    })).toBe('75k');
  });
});

describe('formatRunTokenUsageBreakdown', () => {
  it('renders input, output, cached, and total details for hover text', () => {
    expect(formatRunTokenUsageBreakdown({
      runtime_kind: 'codex',
      cached_input_tokens: 119_000,
      input_tokens: 120_000,
      output_tokens: 722,
      tokens_used: 120_722,
    })).toEqual([
      'Total: 121k tokens',
      'Input: 120k',
      'Output: 722',
      'Cached input: 119k',
    ]);
  });
});
