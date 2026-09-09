// @vitest-environment jsdom
import React, { act, useState } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { DelayedTeamReplySettings } from '../DelayedTeamReplySettings';
import { DEFAULT_DELAYED_TEAM_REPLY_MESSAGE, DEFAULT_DELAYED_TEAM_REPLY_MESSAGE_NO_EMAIL } from '../delayedTeamReply';
import { buildSettingsDraftFromServer } from '../utils';
import type { SupportInboxSettings } from '@/lib/pmTypes';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

describe('delayed team reply settings', () => {
  let root: Root;
  let container: HTMLDivElement;
  const onMinutesChange = vi.fn();
  function Harness() {
    const [minutes, setMinutes] = useState(5);
    const [message, setMessage] = useState(DEFAULT_DELAYED_TEAM_REPLY_MESSAGE);
    const [messageNoEmail, setMessageNoEmail] = useState(DEFAULT_DELAYED_TEAM_REPLY_MESSAGE_NO_EMAIL);
    return <DelayedTeamReplySettings minutes={minutes} message={message} messageNoEmail={messageNoEmail} onMinutesChange={(value) => { onMinutesChange(value); setMinutes(value); }} onMessageChange={setMessage} onMessageNoEmailChange={setMessageNoEmail} />;
  }
  beforeEach(() => {
    vi.clearAllMocks();
    container = document.createElement('div'); document.body.appendChild(container); root = createRoot(container);
    act(() => root.render(<Harness />));
  });
  afterEach(() => { act(() => root.unmount()); container.remove(); });
  function change(selector: string, value: string) {
    const element = container.querySelector(selector) as HTMLInputElement | HTMLTextAreaElement;
    act(() => {
      const prototype = element instanceof HTMLInputElement ? HTMLInputElement.prototype : HTMLTextAreaElement.prototype;
      Object.getOwnPropertyDescriptor(prototype, 'value')!.set!.call(element, value);
      element.dispatchEvent(new Event('input', { bubbles: true }));
    });
  }
  it('accepts valid timing changes and prevents saving empty, fractional or out-of-range values', () => {
    for (const value of ['', '0', '1.5', '1441']) change('#delayed-team-reply-minutes', value);
    expect(onMinutesChange).not.toHaveBeenCalled();
    expect(container.querySelector('[role="alert"]')).not.toBeNull();
    change('#delayed-team-reply-minutes', '12');
    expect(onMinutesChange).toHaveBeenCalledWith(12);
    expect(container.querySelector('[role="alert"]')).toBeNull();
  });
  it('previews edited messages and offers only one noninteractive email capture preview', () => {
    change('#delayed-team-reply-message', 'We will email your reply.');
    change('#delayed-team-reply-message-no-email', 'Leave an email for your reply.');
    expect(container.textContent).toContain('We will email your reply.');
    expect(container.textContent).toContain('Leave an email for your reply.');
    expect(container.querySelectorAll('input[type="email"]')).toHaveLength(1);
    expect(container.querySelector<HTMLInputElement>('input[type="email"]')?.disabled).toBe(true);
    expect(container.querySelector('[role="switch"]')).toBeNull();
    change('#delayed-team-reply-message', '');
    expect(container.textContent).toContain(DEFAULT_DELAYED_TEAM_REPLY_MESSAGE);
  });
  it('normalizes older settings to defaults and preserves saved customization in drafts', () => {
    const original = {} as SupportInboxSettings;
    const draft = buildSettingsDraftFromServer(original);
    expect(draft.delayed_team_reply_minutes).toBe(5);
    expect(draft.delayed_team_reply_message).toBe(DEFAULT_DELAYED_TEAM_REPLY_MESSAGE);
    expect(draft.delayed_team_reply_message_no_email).toBe(DEFAULT_DELAYED_TEAM_REPLY_MESSAGE_NO_EMAIL);
    const custom = buildSettingsDraftFromServer({ ...original, delayed_team_reply_minutes: 7, delayed_team_reply_message: 'Known email', delayed_team_reply_message_no_email: 'Missing email' });
    expect(custom).toMatchObject({ delayed_team_reply_minutes: 7, delayed_team_reply_message: 'Known email', delayed_team_reply_message_no_email: 'Missing email' });
  });
});
