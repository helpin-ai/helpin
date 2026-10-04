// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { HelpcenterCustomDomainStatus, type HelpcenterDomainVerification } from '../HelpcenterCustomDomainStatus';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});
afterEach(() => {
  act(() => root.unmount());
  container.remove();
});

const pending: HelpcenterDomainVerification = {
  custom_domain: 'help.acme.com',
  custom_domain_status: 'pending',
  custom_domain_last_error: 'We couldn’t find the TXT record _helpin-challenge.help.acme.com yet.',
  custom_domain_checked_at: new Date().toISOString(),
  custom_domain_target: 'helpin.center',
  custom_domain_challenge_name: '_helpin-challenge.help.acme.com',
  custom_domain_challenge_value: 'helpin-verify=abc123',
};

const render = (verification: HelpcenterDomainVerification | null, editedDomain: string, onCheck = vi.fn().mockResolvedValue(null)) =>
  act(async () => root.render(<HelpcenterCustomDomainStatus verification={verification} editedDomain={editedDomain} editable onCheck={onCheck} />));

describe('HelpcenterCustomDomainStatus', () => {
  it('lists both DNS records and re-checks a pending domain', async () => {
    const onCheck = vi.fn().mockResolvedValue({ ...pending, custom_domain_status: 'verified' });
    await render(pending, 'help.acme.com', onCheck);
    const text = container.textContent ?? '';
    expect(text).toContain('Waiting for DNS');
    expect(text).toContain('CNAMEhelp.acme.comhelpin.center');
    expect(text).toContain('TXT_helpin-challenge.help.acme.comhelpin-verify=abc123');
    expect(text).toContain('We couldn’t find the TXT record');
    const button = [...container.querySelectorAll('button')].find((b) => b.textContent === 'Check DNS');
    await act(async () => button?.click());
    expect(onCheck).toHaveBeenCalledTimes(1);
  });

  it('shows a live domain without the check button', async () => {
    await render({ ...pending, custom_domain_status: 'verified', custom_domain_last_error: undefined }, 'help.acme.com');
    expect(container.textContent).toContain('Live');
    expect([...container.querySelectorAll('button')].some((b) => b.textContent === 'Check DNS')).toBe(false);
  });

  it('explains a failing domain keeps serving', async () => {
    await render({ ...pending, custom_domain_status: 'failing', custom_domain_last_error: 'help.acme.com doesn’t point to helpin.center yet.' }, 'help.acme.com');
    expect(container.textContent).toContain('Not pointing to Helpin');
    expect(container.textContent).toContain('keep being served on help.acme.com');
  });

  it('asks to save an edited domain before showing its records', async () => {
    await render(pending, 'docs.acme.com');
    expect(container.textContent).toBe('Save to get the DNS records for docs.acme.com.');
  });

  it('keeps the plain CNAME guidance where domains aren’t verified', async () => {
    await render({ custom_domain: 'help.acme.com' }, 'help.acme.com');
    expect(container.textContent).toContain('CNAME');
    expect(container.querySelector('[data-testid="hc-domain-status"]')).toBeNull();
  });
});
