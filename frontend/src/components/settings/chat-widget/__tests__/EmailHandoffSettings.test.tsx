// @vitest-environment jsdom
import React, { act, useState } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { EmailHandoffSettings } from '../EmailHandoffSettings';
import { DEFAULT_EMAIL_HANDOFF_MESSAGE, DEFAULT_EMAIL_HANDOFF_MESSAGE_AFTER_HOURS } from '../emailHandoff';
import { buildSettingsDraftFromServer } from '../utils';
import type { SupportInboxSettings } from '@/lib/pmTypes';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

describe('email handoff settings', () => {
  let root: Root;
  let container: HTMLDivElement;
  let latest = { message: '', afterHours: '' };
  function Harness() {
    const [message, setMessage] = useState(DEFAULT_EMAIL_HANDOFF_MESSAGE);
    const [afterHours, setAfterHours] = useState(DEFAULT_EMAIL_HANDOFF_MESSAGE_AFTER_HOURS);
    latest = { message, afterHours };
    return <EmailHandoffSettings message={message} messageAfterHours={afterHours} onMessageChange={setMessage} onMessageAfterHoursChange={setAfterHours} />;
  }
  beforeEach(() => {
    container = document.createElement('div'); document.body.appendChild(container); root = createRoot(container);
    act(() => root.render(<Harness />));
  });
  afterEach(() => { act(() => root.unmount()); container.remove(); });

  it('edits the business hours and after-hours messages separately', () => {
    const afterHours = container.querySelector('#email-handoff-message-after-hours') as HTMLTextAreaElement;
    act(() => {
      Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value')!.set!.call(afterHours, 'We are away until {next_open}.');
      afterHours.dispatchEvent(new Event('input', { bubbles: true }));
    });
    expect(latest).toEqual({ message: DEFAULT_EMAIL_HANDOFF_MESSAGE, afterHours: 'We are away until {next_open}.' });
    expect(container.textContent).toContain('During business hours');
  });

  it('fills older settings with the defaults and keeps saved messages in drafts', () => {
    const original = {} as SupportInboxSettings;
    const draft = buildSettingsDraftFromServer(original);
    expect(draft.email_handoff_message).toBe(DEFAULT_EMAIL_HANDOFF_MESSAGE);
    expect(draft.email_handoff_message_after_hours).toBe(DEFAULT_EMAIL_HANDOFF_MESSAGE_AFTER_HOURS);
    const custom = buildSettingsDraftFromServer({ ...original, email_handoff_message: 'Our team has your email.', email_handoff_message_after_hours: 'Back {next_open}.' });
    expect(custom).toMatchObject({ email_handoff_message: 'Our team has your email.', email_handoff_message_after_hours: 'Back {next_open}.' });
  });
});
