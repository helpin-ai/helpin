// @vitest-environment jsdom

import { afterEach, describe, expect, it, vi } from 'vitest';

import { ConfiguredCapture } from '../../../src/tracking/configured-capture';

describe('ConfiguredCapture', () => {
  afterEach(() => {
    document.body.innerHTML = '';
  });

  it('captures only configured forms and allowlisted safe fields', () => {
    const track = vi.fn();
    const lead = vi.fn();
    document.body.innerHTML = `
      <form id="demo"><input name="email" value="buyer@example.com"><input name="password" type="password" value="secret"></form>
      <form id="other"><input name="email" value="other@example.com"></form>`;
    const capture = new ConfiguredCapture(
      { track, lead },
      [
        {
          selector: '#demo',
          formId: 'demo-request',
          fields: ['email', 'password'],
        },
      ],
    );
    capture.init();

    document
      .querySelector<HTMLFormElement>('#other')
      ?.dispatchEvent(
        new Event('submit', { bubbles: true, cancelable: true }),
      );
    expect(track).not.toHaveBeenCalled();
    document
      .querySelector<HTMLFormElement>('#demo')
      ?.dispatchEvent(
        new Event('submit', { bubbles: true, cancelable: true }),
      );

    expect(track).toHaveBeenCalledWith('$form', {
      form_id: 'demo-request',
      field_names: ['email'],
      fields: { email: 'buyer@example.com' },
    });
    expect(lead).toHaveBeenCalledWith({ email: 'buyer@example.com' });
    capture.destroy();
  });

  it('attaches an immutable rule identity to configured clicks', () => {
    const track = vi.fn();
    document.body.innerHTML =
      '<button class="pricing-cta"><span>Choose plan</span></button>';
    const capture = new ConfiguredCapture(
      { track },
      [],
      [
        {
          ruleKey: 'pricing_cta_click',
          version: 3,
          clickSelectors: ['.pricing-cta'],
        },
        { ruleKey: 'invalid', version: 0, clickSelectors: ['button'] },
      ],
    );
    capture.init();
    document
      .querySelector('span')
      ?.dispatchEvent(new MouseEvent('click', { bubbles: true }));

    expect(track).toHaveBeenCalledTimes(1);
    expect(track).toHaveBeenCalledWith('$interaction', {
      interaction_type: 'click',
      capture_rule_key: 'pricing_cta_click',
      capture_rule_version: 3,
      selector: '.pricing-cta',
    });
    capture.destroy();
  });

  it('installs no listeners without explicit capture configuration', () => {
    const track = vi.fn();
    document.body.innerHTML =
      '<form><input name="email" value="buyer@example.com"></form><button>Buy</button>';
    const capture = new ConfiguredCapture({ track });
    capture.init();
    document
      .querySelector('form')
      ?.dispatchEvent(new Event('submit', { bubbles: true }));
    document
      .querySelector('button')
      ?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    expect(track).not.toHaveBeenCalled();
  });
});
