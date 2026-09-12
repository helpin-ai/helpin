// @vitest-environment jsdom
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import { afterEach, expect, it } from 'vitest';
import { clearAutomaticReplyDelivery, loadReplyDelivery, restoreReplyDelivery, saveReplyDelivery, saveReplySubject, useComposerDelivery, useReplySubject, type ReplyPresence } from './replyDelivery';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
const container = document.createElement('div');
let root = createRoot(container);
let sequence = 0;
afterEach(() => { act(() => root.unmount()); root = createRoot(container); localStorage.clear(); });

function Composer({ id, presence = 'online' }: { id: string; presence?: ReplyPresence }) {
  const mode = useComposerDelivery('ws-test', id, { source: 'widget', presence, emailEligible: true, active: true });
  const subject = useReplySubject('ws-test', id, 'Conversation title');
  return <div>{mode} | {subject}</div>;
}

it('synchronizes manual choices between mounted composers and preserves the choice for the next reply', () => {
  const id = `manual-${++sequence}`;
  act(() => root.render(<><Composer id={id} /><Composer id={id} presence="offline" /></>));
  act(() => saveReplyDelivery('ws-test', id, 'chat_only'));
  expect(container.textContent).toBe('chat_only | Conversation titlechat_only | Conversation title');
  act(() => clearAutomaticReplyDelivery('ws-test', id));
  expect(loadReplyDelivery('ws-test', id)).toBe('chat_only');
  expect(container.textContent).toBe('chat_only | Conversation titlechat_only | Conversation title');
});

it('restores the original message mode and subject without replacing a newer manual preference', () => {
  const id = `undo-${++sequence}`;
  saveReplyDelivery('ws-test', id, 'chat_only');
  act(() => root.render(<Composer id={id} />));
  act(() => {
    restoreReplyDelivery('ws-test', id, 'email_only');
    saveReplySubject('ws-test', id, 'Original email subject');
  });
  expect(container.textContent).toBe('email_only | Original email subject');
  expect(loadReplyDelivery('ws-test', id)).toBe('chat_only');
  act(() => {
    clearAutomaticReplyDelivery('ws-test', id);
    saveReplySubject('ws-test', id);
  });
  expect(container.textContent).toBe('chat_only | Conversation title');
});

it('latches automatic email across remounts and resets it for a new reply', () => {
  const id = `automatic-${++sequence}`;
  act(() => root.render(<Composer id={id} presence="offline" />));
  expect(container.textContent).toContain('chat_and_email');
  act(() => root.render(null));
  act(() => root.render(<Composer id={id} />));
  expect(container.textContent).toContain('chat_and_email');
  act(() => clearAutomaticReplyDelivery('ws-test', id));
  expect(container.textContent).toContain('chat_only');
  expect(loadReplyDelivery('ws-test', id)).toBeUndefined();
});

it('does not carry manual choices or subject into another conversation', () => {
  const id = `isolation-${++sequence}`;
  saveReplyDelivery('ws-test', id, 'email_only');
  saveReplySubject('ws-test', id, 'Custom subject');
  act(() => root.render(<Composer id={id} />));
  expect(container.textContent).toBe('email_only | Custom subject');
  act(() => root.render(<Composer id={`${id}-other`} />));
  expect(container.textContent).toBe('chat_only | Conversation title');
});
