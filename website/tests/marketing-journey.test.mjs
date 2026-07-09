import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { describe, it } from 'node:test';

const files = {
  home: readFileSync(new URL('../src/app/page.tsx', import.meta.url), 'utf8'),
  pricing: readFileSync(new URL('../src/app/pricing/page.tsx', import.meta.url), 'utf8'),
  navbar: readFileSync(new URL('../src/components/Navbar.tsx', import.meta.url), 'utf8'),
  footer: readFileSync(new URL('../src/components/Footer.tsx', import.meta.url), 'utf8'),
};

const allMarketingCopy = Object.values(files).join('\n');

describe('marketing signup journey', () => {
  it('does not promote the removed Free plan or early-access waitlist', () => {
    assert.doesNotMatch(allMarketingCopy, /\bFree plan\b/i);
    assert.doesNotMatch(allMarketingCopy, /\bFree forever\b/i);
    assert.doesNotMatch(allMarketingCopy, /\bmoves? to Free\b/i);
    assert.doesNotMatch(allMarketingCopy, /Get early access/i);
    assert.doesNotMatch(allMarketingCopy, /early access/i);
    assert.doesNotMatch(files.pricing, /name:\s*'Free'/);
  });

  it('routes conversion CTAs to signup and demo booking', () => {
    assert.match(allMarketingCopy, /https:\/\/app\.helpin\.ai\/register/);
    assert.match(allMarketingCopy, /https:\/\/cal\.com\/helpin-ai\/30min/);
    assert.match(files.navbar, /Pricing/);
    assert.match(files.navbar, /Start free trial/);
    assert.match(files.navbar, /Book a demo/);
  });

  it('describes trial expiry consistently with app billing behavior', () => {
    assert.match(files.pricing, /14-day Growth trial/);
    assert.match(files.pricing, /choose Starter or Growth/i);
    assert.match(files.pricing, /workspace access is limited/i);
  });
});
