// @vitest-environment jsdom
import React, { act, useState } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { TooltipProvider } from '@/components/ui/tooltip';
import { WelcomeMessageSettings } from '../WelcomeMessageSettings';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const teamGreeting = 'Hi there! How can we help you today?';
const aiGreeting = 'Hi! I’m your AI assistant. I can help with most questions, and you can ask to speak with our team anytime.';

describe('WelcomeMessageSettings', () => {
  let root: Root;
  let container: HTMLDivElement;
  const onChange = vi.fn();

  function Harness({ aiFirst = true, initialValue = teamGreeting }: { aiFirst?: boolean; initialValue?: string }) {
    const [value, setValue] = useState(initialValue);
    return <TooltipProvider><WelcomeMessageSettings value={value} aiFirst={aiFirst} onChange={(next) => { onChange(next); setValue(next); }} /></TooltipProvider>;
  }

  beforeEach(() => {
    onChange.mockClear();
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);
  });
  afterEach(() => { act(() => root.unmount()); container.remove(); });

  function textarea() { return container.querySelector('textarea')!; }
  function change(value: string) {
    act(() => {
      Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value')!.set!.call(textarea(), value);
      textarea().dispatchEvent(new Event('input', { bubbles: true }));
    });
  }

  it('shows an editable automatic default without saving an override on focus', () => {
    act(() => root.render(<Harness />));
    expect(textarea().value).toBe(aiGreeting);
    act(() => textarea().focus());
    expect(textarea().value).toBe(aiGreeting);
    act(() => textarea().blur());
    expect(onChange).not.toHaveBeenCalled();
    act(() => root.render(<Harness aiFirst={false} />));
    expect(textarea().value).toBe(teamGreeting);
  });

  it('preserves custom text across mode changes and resets to an adaptive default', () => {
    act(() => root.render(<Harness initialValue="Welcome to Acme" />));
    expect(textarea().value).toBe('Welcome to Acme');
    act(() => root.render(<Harness initialValue="Welcome to Acme" aiFirst={false} />));
    expect(textarea().value).toBe('Welcome to Acme');
    const reset = Array.from(container.querySelectorAll('button')).find((button) => button.textContent === 'Reset to default')!;
    act(() => reset.click());
    expect(onChange).toHaveBeenLastCalledWith('');
    expect(textarea().value).toBe(teamGreeting);
    act(() => root.render(<Harness />));
    expect(textarea().value).toBe(aiGreeting);
  });

  it('allows clearing and replacing a greeting without refilling it mid-edit', () => {
    act(() => root.render(<Harness />));
    act(() => textarea().focus());
    change('');
    expect(textarea().value).toBe('');
    change('How can we help with your order?');
    expect(textarea().value).toBe('How can we help with your order?');
    act(() => textarea().blur());
    expect(textarea().value).toBe('How can we help with your order?');
    act(() => textarea().focus());
    change('');
    act(() => textarea().blur());
    expect(textarea().value).toBe(aiGreeting);
  });
});
