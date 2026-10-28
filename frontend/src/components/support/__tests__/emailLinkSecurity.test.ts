// @vitest-environment jsdom

import { describe, expect, it } from 'vitest';
import type { SupportLinkSecurity } from '@/lib/pmTypes';
import { RISKY_EMAIL_LINK_ATTR, markRiskyEmailLinks, riskyEmailLinkHref } from '../emailLinkSecurity';

const phishing = 'https://login-example.test/verify';
const entries: SupportLinkSecurity[] = [
  { url: phishing, status: 'malicious', threat_types: ['SOCIAL_ENGINEERING'], checked_at: '2026-10-28T12:00:00Z', expires_at: '2099-01-01T00:00:00Z' },
  { url: 'https://example.com/docs', status: 'no_match', checked_at: '2026-10-28T12:00:00Z', expires_at: '2099-01-01T00:00:00Z' },
];

function emailDocument(body: string): Document {
  const doc = document.implementation.createHTMLDocument('email');
  doc.body.innerHTML = body;
  return doc;
}

describe('markRiskyEmailLinks', () => {
  it('disarms a flagged link hidden behind text and leaves safe links alone', () => {
    const doc = emailDocument(`<a id="bad" href="${phishing}#top"><b>Fix your mailbox</b></a> <a id="good" href="https://example.com/docs">Docs</a>`);

    expect(markRiskyEmailLinks(doc, entries)).toBe(1);

    const bad = doc.getElementById('bad')!;
    expect(bad.hasAttribute('href')).toBe(false);
    expect(bad.getAttribute(RISKY_EMAIL_LINK_ATTR)).toBe(`${phishing}#top`);
    expect(bad.getAttribute('role')).toBe('link');
    expect(bad.getAttribute('tabindex')).toBe('0');
    expect(bad.querySelector('[aria-label="Potentially harmful link"] svg')).not.toBeNull();
    expect(doc.getElementById('good')!.getAttribute('href')).toBe('https://example.com/docs');
    expect(doc.querySelectorAll('#helpin-risky-link-styles')).toHaveLength(1);
  });

  it('marks each link once when verdicts are applied again', () => {
    const doc = emailDocument(`<a href="${phishing}">Verify</a><map><area href="${phishing}"></map>`);

    expect(markRiskyEmailLinks(doc, entries)).toBe(2);
    expect(markRiskyEmailLinks(doc, entries)).toBe(0);

    expect(doc.querySelectorAll('[aria-label="Potentially harmful link"]')).toHaveLength(1);
    expect(doc.querySelector('area')!.hasAttribute('href')).toBe(false);
    expect(doc.querySelectorAll('#helpin-risky-link-styles')).toHaveLength(1);
  });

  it('changes nothing without a harmful verdict', () => {
    const doc = emailDocument(`<a href="${phishing}">Verify</a>`);
    const before = doc.body.innerHTML;

    expect(markRiskyEmailLinks(doc, entries.filter((entry) => entry.status !== 'malicious'))).toBe(0);
    expect(doc.body.innerHTML).toBe(before);
    expect(doc.getElementById('helpin-risky-link-styles')).toBeNull();
  });
});

describe('riskyEmailLinkHref', () => {
  it('finds the flagged address from anywhere inside the link', () => {
    const doc = emailDocument(`<a href="${phishing}"><b>Verify</b></a><p id="outside">Thanks</p>`);
    markRiskyEmailLinks(doc, entries);

    expect(riskyEmailLinkHref(doc.querySelector('b'))).toBe(phishing);
    expect(riskyEmailLinkHref(doc.querySelector('svg path'))).toBe(phishing);
    expect(riskyEmailLinkHref(doc.getElementById('outside'))).toBeNull();
    expect(riskyEmailLinkHref(null)).toBeNull();
  });
});
