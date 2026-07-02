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
});
