// @vitest-environment jsdom

import { afterEach, describe, expect, it, vi } from 'vitest';
import { measureVisibleEmailContentHeight, prepareCollapsedEmailLayout } from '../EmailBodyRendererLayout';

function mockStyleForCollapsedEmail() {
  return vi.spyOn(window, 'getComputedStyle').mockImplementation((el: Element) => ({
    display: el.closest('[data-helpin-quote]') ? 'none' : 'block',
    visibility: 'visible',
    paddingBottom: el === document.body ? '8px' : '0',
  } as CSSStyleDeclaration));
}

describe('EmailBodyRenderer collapsed quote layout', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    document.body.innerHTML = '';
  });

  it('measures only visible reply content when collapsed quoted content sits inside a tall wrapper', () => {
    document.body.innerHTML = `
      <div id="email-wrapper" style="min-height: 260px">
        <p>Testing reply back.</p>
        <div data-helpin-quote="true">
          <p>Quoted history should stay hidden.</p>
        </div>
      </div>
    `;

    mockStyleForCollapsedEmail();

    let selectedText: Text | null = null;
    vi.spyOn(document, 'createRange').mockImplementation(() => ({
      selectNodeContents: (node: Node) => {
        selectedText = node as Text;
      },
      getClientRects: () => {
        const bottom = selectedText?.textContent?.includes('Testing reply back') ? 32 : 260;
        return [{ bottom }] as unknown as DOMRectList;
      },
      detach: vi.fn(),
    } as unknown as Range));

    const wrapper = document.getElementById('email-wrapper');

    prepareCollapsedEmailLayout(document, true);

    expect(wrapper?.getAttribute('data-helpin-collapse-host')).toBe('true');
    expect(measureVisibleEmailContentHeight(document, true)).toBe(40);

    prepareCollapsedEmailLayout(document, false);

    expect(wrapper?.hasAttribute('data-helpin-collapse-host')).toBe(false);
  });

  it('marks Outlook quoted body siblings after the reply header as collapsible', () => {
    document.body.innerHTML = `
      <div>
        <p>We are cutting costs and management said to cut it.</p>
        <div id="Signature"><img src="signature.png" alt="Signature" /></div>
        <div id="appendonsend" data-helpin-quote="true"></div>
        <hr id="quote-rule" />
        <div id="divRplyFwdMsg" data-helpin-quote="true">From: Support</div>
        <div id="old-thread">
          <p>-- Please type your reply above this line --</p>
          <p>Hi Brian, We're sorry to hear that.</p>
        </div>
      </div>
    `;

    prepareCollapsedEmailLayout(document, true);

    expect(document.getElementById('Signature')?.hasAttribute('data-helpin-quote')).toBe(false);
    expect(document.getElementById('quote-rule')?.getAttribute('data-helpin-quote')).toBe('true');
    expect(document.getElementById('old-thread')?.getAttribute('data-helpin-quote')).toBe('true');
  });
});
