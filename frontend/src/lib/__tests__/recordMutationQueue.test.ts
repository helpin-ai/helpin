import { expect, it } from 'vitest';
import { serializeRecordMutation } from '../recordMutationQueue';
it('serializes one record while independent records can save', async () => {
  let release!: () => void;
  const calls: string[] = [];
  const first = serializeRecordMutation('deal:a', async () => {
    calls.push('first');
    await new Promise<void>((resolve) => {
      release = resolve;
    });
    return 1;
  });
  const second = serializeRecordMutation('deal:a', async () => {
    calls.push('second');
    return 2;
  });
  await serializeRecordMutation('deal:b', async () => {
    calls.push('other');
  });
  expect(calls).toEqual(['first', 'other']);
  release();
  expect(await first).toBe(1);
  expect(await second).toBe(2);
  expect(calls).toEqual(['first', 'other', 'second']);
});
it('does not block later saves after a failed request', async () => {
  const first = serializeRecordMutation('deal:failed', async () => {
    throw new Error('failed');
  });
  const second = serializeRecordMutation('deal:failed', async () => 'saved');
  await expect(first).rejects.toThrow('failed');
  await expect(second).resolves.toBe('saved');
});
