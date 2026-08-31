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
      field_mappings: { email: 'contact.email' },
    });
    expect(lead).toHaveBeenCalledWith({ email: 'buyer@example.com' });
    capture.destroy();
  });

  it('automatically maps common contact and company fields', () => {
    const track = vi.fn();
    const lead = vi.fn();
    document.body.innerHTML = `
      <form id="demo">
        <input name="work_email" type="email" value="buyer@example.com">
        <input name="first_name" value="Ada">
        <input name="company" value="Analytical Engines">
        <input name="role" value="VP Revenue">
      </form>`;
    const capture = new ConfiguredCapture(
      { track, lead },
      [
        {
          selector: '#demo',
          formId: 'demo-request',
          fields: ['work_email', 'first_name', 'company', 'role'],
        },
      ],
    );
    capture.init();

    document
      .querySelector<HTMLFormElement>('#demo')
      ?.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));

    expect(lead).toHaveBeenCalledWith({
      work_email: 'buyer@example.com',
      first_name: 'Ada',
      company: { name: 'Analytical Engines' },
      role: 'VP Revenue',
      email: 'buyer@example.com',
      job_title: 'VP Revenue',
    });
    expect(track).toHaveBeenCalledWith(
      '$form',
      expect.objectContaining({
        field_mappings: {
          work_email: 'contact.email',
          first_name: 'contact.first_name',
          company: 'company.name',
          role: 'contact.job_title',
        },
      }),
    );
    capture.destroy();
  });

  it('uses explicit mappings for non-standard field names', () => {
    const track = vi.fn();
    const lead = vi.fn();
    document.body.innerHTML = `
      <form id="demo">
        <input name="reply_to" value="buyer@example.com">
        <input name="account" value="Analytical Engines">
      </form>`;
    const capture = new ConfiguredCapture(
      { track, lead },
      [
        {
          selector: '#demo',
          formId: 'demo-request',
          fields: ['reply_to', 'account'],
          fieldMappings: {
            reply_to: 'contact.email',
            account: 'company.name',
          },
        },
      ],
    );
    capture.init();

    document
      .querySelector<HTMLFormElement>('#demo')
      ?.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));

    expect(lead).toHaveBeenCalledWith({
      reply_to: 'buyer@example.com',
      account: 'Analytical Engines',
      email: 'buyer@example.com',
      company: { name: 'Analytical Engines' },
    });
    capture.destroy();
  });

  it('allows automatic mappings to be disabled explicitly', () => {
    const track = vi.fn();
    const lead = vi.fn();
    document.body.innerHTML = `
      <form id="demo">
        <input name="email" value="buyer@example.com">
        <input name="company" value="Not a CRM company">
      </form>`;
    const capture = new ConfiguredCapture(
      { track, lead },
      [
        {
          selector: '#demo',
          formId: 'demo-request',
          fields: ['email', 'company'],
          fieldMappings: { company: 'ignore' },
        },
      ],
    );
    capture.init();

    document
      .querySelector<HTMLFormElement>('#demo')
      ?.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));

    expect(lead).toHaveBeenCalledWith({
      email: 'buyer@example.com',
      company: 'Not a CRM company',
    });
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
