import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { demoFrame, DEMO_DURATION, DEMO_MESSAGES } from '../src/app/new/products/customer-support/support-widget-script.ts';

const timestamp = '2026-09-21T10:00:00Z';
describe('Support widget story', () => {
  it('checks connected logs before handing the findings to Sam', () => {
    const checking = demoFrame(7500, timestamp);
    assert.equal(checking.isAIThinking, true);
    assert.match(checking.aiProgressLabel, /Searching connected export logs/);
    assert.equal(checking.handedOff, false);
    assert.equal(checking.messages.at(-1).role, 'customer');
    const found = demoFrame(12500, timestamp);
    assert.match(found.messages.at(-1).content, /Pagination stops after 10,000 rows/);
    assert.equal(found.handedOff, false);
    assert.equal(demoFrame(13000, timestamp).handedOff, true);
  });
  it('reserves complete message content throughout a stream', () => {
    const first = demoFrame(1800, timestamp);
    const midway = demoFrame(2600, timestamp);
    assert.equal(first.messages.at(-1).content, midway.messages.at(-1).content);
    assert.deepEqual(first.stream, { id: 'answer', duration: 1600, elapsed: 0 });
    assert.equal(midway.stream.elapsed, 800);
    assert.equal(first.messages.at(-1).sources, undefined);
    const finished = demoFrame(3400, timestamp);
    assert.equal(finished.stream, null);
    assert.equal(finished.messages.at(-1).isStreaming, false);
    assert.equal(finished.messages.at(-1).sources.length, 1);
  });
  it('sends the follow-up from Helpin AI only after release and approval', () => {
    const approved = demoFrame(21000, timestamp);
    assert.match(approved.stage, /Fix released · Follow-up approved/);
    assert.equal(approved.messages.some(message => message.id === 'resolved'), false);
    const sending = demoFrame(23400, timestamp);
    assert.equal(sending.messages.at(-1).id, 'resolved');
    assert.equal(sending.messages.at(-1).role, 'ai');
    assert.equal(sending.handedOff, false);
    assert.equal(sending.stream.elapsed, 0);
  });
  it('provides a complete stationary conversation for pause and reduced motion', () => {
    const completed = demoFrame(DEMO_DURATION, timestamp);
    assert.equal(completed.messages.length, DEMO_MESSAGES.length);
    assert.equal(completed.stream, null);
    assert.equal(completed.isAIThinking, false);
    assert.equal(completed.isTyping, false);
    assert(completed.messages.every(message => !message.isStreaming && message.createdAt === timestamp));
    assert.equal(demoFrame(0, timestamp).messages.length, 1);
  });
});
