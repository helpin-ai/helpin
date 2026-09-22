import { describe, expect, it } from 'vitest';
import type { SupportMessage } from '@/lib/pmTypes';
import { mergePendingSupportMessages } from '../usePendingSupportSends';
const pending = {id:'optimistic',client_message_id:'client',created_at:'2026-09-22T10:00:00Z',pending_send:'preparing'} as SupportMessage;
const stored = {...pending,id:'pending-job',pending_send_id:'job'};
const final = {id:'final',metadata:JSON.stringify({client_message_id:'client'}),created_at:pending.created_at} as SupportMessage;
describe('pending send reconciliation', () => {
 it('deduplicates a websocket result arriving before the enqueue response', () => {
  expect(mergePendingSupportMessages([pending, final], [stored])).toEqual([final]);
 });
 it('recovers pending replies after reload', () => {
  expect(mergePendingSupportMessages([], [stored])).toEqual([stored]);
 });
 it('retains a confirmed result while the transcript query catches up', () => {
  expect(mergePendingSupportMessages([stored], [final])).toEqual([final]);
 });
 it('retains a local send that the recovery response has not seen yet', () => {
  expect(mergePendingSupportMessages([pending], [])).toEqual([pending]);
 });
});
