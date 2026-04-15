import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { formatReplyTimeCopy } from '@helpin-ai/shared';

// Shared fixture generated from
//   server/internal/service/support_reply_expectations_test.go#TestReplyTimeFixtureIsUpToDate
//
// Regenerate with:
//   GENERATE_REPLY_TIME_FIXTURE=1 go test ./internal/service/ -run TestReplyTimeFixtureIsUpToDate
//
// The TS formatReplyTimeCopy must produce the exact same string as the
// Go implementation for every case in the fixture. If this test fails,
// one of the two implementations has drifted — fix the one that's wrong
// before merging.
type Case = {
  name: string;
  preset: string;
  custom_minutes: number;
  expected: string;
};

const fixturePath = join(__dirname, '..', '..', '..', 'shared', 'test-data', 'reply-time-cases.json');
const cases: Case[] = JSON.parse(readFileSync(fixturePath, 'utf-8'));

describe('formatReplyTimeCopy — parity with Go backend fixture', () => {
  for (const tc of cases) {
    it(tc.name, () => {
      expect(formatReplyTimeCopy(tc.preset, tc.custom_minutes)).toBe(tc.expected);
    });
  }

  it('has at least the core preset coverage', () => {
    const names = new Set(cases.map((c) => c.name));
    expect(names.has('few_minutes')).toBe(true);
    expect(names.has('few_hours')).toBe(true);
    expect(names.has('same_day')).toBe(true);
    expect(names.has('custom_15min')).toBe(true);
    expect(names.has('custom_10080min_max')).toBe(true);
  });
});
