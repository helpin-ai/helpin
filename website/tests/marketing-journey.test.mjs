import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { describe, it } from 'node:test';

const files = {
  home: readFileSync(new URL('../src/app/page.tsx', import.meta.url), 'utf8'),
  globals: readFileSync(new URL('../src/app/globals.css', import.meta.url), 'utf8'),
  pricing: ['page.tsx', 'pricing-data.ts', 'PricingPlans.tsx', 'PricingSections.tsx'].map(file => readFileSync(new URL('../src/app/pricing/' + file, import.meta.url), 'utf8')).join('\n'),
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
    assert.match(files.home, /Book a demo/);
    assert.match(files.footer, /Book a demo/);
  });

  it('links the navigation to the homepage agent roster', () => {
    assert.match(files.navbar, /label: 'Agents', href: '\/#agents'/);
    assert.match(files.home, /id="agents"[\s\S]*?scroll-mt-20/);
    assert.match(files.home, /An agent for every team, every workflow\./);
    assert.match(files.globals, /scroll-behavior:\s*smooth/);
  });

  it('keeps demo booking out of the top navigation', () => {
    assert.doesNotMatch(files.navbar, /Book a demo/);
    assert.doesNotMatch(files.navbar, /DEMO_URL/);
  });

  it('describes trial expiry consistently with app billing behavior', () => {
    assert.match(files.pricing, /14-day Growth trial/);
    assert.match(files.pricing, /choose Starter or Growth/i);
    assert.match(files.pricing, /workspace access is limited/i);
  });

  it('keeps conversion buttons out of the pricing hero', () => {
    const hero = files.pricing.slice(files.pricing.indexOf('<section className="pricing-hero">'), files.pricing.indexOf('<PricingPlans />'));
    assert.doesNotMatch(hero, /Start free trial|Book a demo/);
    assert.match(hero, /#cloud-plans/);
    assert.match(hero, /#self-hosted/);
  });

  it('frames the original workflow animation with the ambient hero canvas', () => {
    assert.match(files.home, /className="wf-stage"/);
    assert.match(files.home, /<AIWorkflowVisual\s*\/>/);
    assert.match(files.globals, /\.wf-stage::before/);
    assert.match(files.globals, /radial-gradient/);
    assert.match(files.globals, /linear-gradient/);
  });

  it('leads with the team and agent value proposition', () => {
    assert.match(files.home, /Bring every team together\./);
    assert.doesNotMatch(files.home, /One system\. Every team\./);
    assert.doesNotMatch(files.home, /className="block font-display"[^>]*>Put AI agents to work\./);
    assert.match(files.home, /Put <span className="relative inline-block">AI agents<svg/);
    assert.match(files.home, /stroke="var\(--color-pop\)"/);
  });

  it('scopes the warm coral brand-color trial to the homepage', () => {
    assert.match(files.home, /className="home-coral"/);
    assert.match(files.globals, /\.home-coral\s*\{[^}]*--color-pop:\s*oklch\(0\.62 0\.17 32\)/s);
    const stageStyles = files.globals.slice(files.globals.indexOf('.wf-stage {'), files.globals.indexOf('@keyframes wf-stage-ambient'));
    assert.doesNotMatch(stageStyles, /155/);
    assert.match(stageStyles, /var\(--color-pop\)/);
  });

  it('keeps solution tick marks semantically green across brand themes', () => {
    assert.match(files.home, /font-black flex-shrink-0 mt-0\.5" style=\{\{ color: 'oklch\(0\.45 0\.15 155\)' \}\}>✓<\/span>/);
  });

  it('limits the homepage CTA glow to the primary trial button', () => {
    assert.match(files.home, /glowPrimary\s*=\s*false/);
    assert.match(files.home, /glowPrimary\s*\?\s*\(/);
    assert.match(files.home, /<ConversionActions source=\{source\} glowPrimary\s*\/>/);
  });

  it('keeps both homepage hero actions on one consistent line and height', () => {
    assert.match(files.home, /email-glow-wrapper h-12 shrink-0/);
    assert.match(files.home, /inline-flex shrink-0 items-center justify-center whitespace-nowrap/);
    assert.match(files.home, /glowPrimary \? 'h-full w-full' : 'h-12'/);
    assert.match(files.home, /\$\{actionLayout\} h-12/);
    assert.doesNotMatch(files.home, /CalendarDays/);
  });

  it('opens marketing demo links in a new tab', () => {
    for (const source of [files.home, files.footer]) {
      const links = [...source.matchAll(/<Link[\s\S]*?href=\{DEMO_URL\}[\s\S]*?>/g)];
      assert.ok(links.length > 0);
      for (const link of links) {
        assert.match(link[0], /target="_blank"/);
        assert.match(link[0], /rel="noopener noreferrer"/);
      }
    }
  });

  it('keeps book-a-demo CTAs text-only', () => {
    assert.doesNotMatch(files.home, /CalendarDays/);
    assert.doesNotMatch(files.footer, /CalendarDays/);
  });

  it('uses readable text contrast throughout the dark footer', () => {
    assert.doesNotMatch(files.footer, /text-white\/(?:30|40|50)\b/);
    assert.match(files.footer, /text-white\/75/);
    assert.match(files.footer, /text-white\/70/);
    assert.match(files.footer, /text-white\/60/);
  });

  it('reassures visitors beneath the footer actions', () => {
    assert.match(files.footer, /No credit card required\./);
  });

  it('reassures visitors beneath the homepage hero actions', () => {
    assert.match(files.home, /<SignupCTA source="home-hero" \/>[\s\S]*?No credit card required\./);
  });

  it('anchors the top workflow card inward as its action text grows', () => {
    assert.match(files.home, /wf-card-\$\{n\.id\}/);
    assert.match(files.globals, /\.wf-card-pm\s*\{[^}]*translate\(-50%, 0\)/s);
    assert.match(files.globals, /\.wf-card-pm\.on/);
  });

  it('keeps workflow action labels stationary while a pulse carries direction', () => {
    assert.match(files.home, /const \[midpoint, setMidpoint\]/);
    assert.match(files.home, /const \[progress, setProgress\]/);
    assert.match(files.home, /className="wf-packet-label"/);
    assert.match(files.home, /className="wf-packet-pulse"/);
    assert.match(files.home, /getPointAtLength\(labelStart \+ labelUsableLen \* 0\.5\)/);
    assert.match(files.home, /const nodeClearance = 105/);
    assert.match(files.home, /const hubClearance = 50/);
    assert.match(files.globals, /\.wf-packet-label/);
    assert.match(files.home, /const labelVisibility = Math\.min\(enterProgress, exitProgress\)/);
    assert.match(files.home, /scale\(\$\{labelScale\}\)/);
    assert.doesNotMatch(files.globals, /@keyframes wf-packet-label-enter/);
  });

  it('tells an enterprise SSO recovery story across every product module', () => {
    assert.doesNotMatch(files.home, /id:\s*'customer'/);
    assert.doesNotMatch(files.home, /module:\s*'Customer'/);
    assert.match(files.home, /packet:\s*'SSO issue reported'/);
    assert.match(files.home, /packet:\s*'Renewal risk detected'/);
    assert.match(files.home, /packet:\s*'Workaround found'/);
    assert.match(files.home, /packet:\s*'Urgent task created'/);
    assert.match(files.home, /packet:\s*'Help doc updated'/);
    assert.match(files.home, /label="Workaround ready"/);
    assert.match(files.home, /label="Owner notified"/);
    assert.match(files.home, /label="Workaround sent"/);
    assert.match(files.home, /SSO setup is blocking our launch/);
    assert.match(files.home, /Renewal in 18 days/);
    assert.match(files.home, /SAML certificate mapping/);
    assert.match(files.home, /Prevent SSO mapping failures/);
    assert.match(files.home, /wf-pm-task-title text-\[11px\] font-medium leading-snug/);
    assert.match(files.home, /Enterprise SSO setup/);
    assert.match(files.home, /Customer unblocked/);
    assert.match(files.home, /Assessing\.\.\./);
    assert.match(files.home, />Closing<\/tspan>/);
    assert.match(files.home, />loop\.\.\.<\/tspan>/);
    assert.doesNotMatch(files.home, /Login not working on mobile/);
    assert.doesNotMatch(files.home, /Fix completed/);
    assert.match(files.home, /fontSize="9\.5"/);
    assert.match(files.home, /text-\[11px\][^\n]*tracking-wide/);

    const packetLabelMarkup = files.home.match(/<g className="wf-packet-label"[\s\S]*?<\/g>/)?.[0] ?? '';
    assert.doesNotMatch(packetLabelMarkup, /<circle/);

    const workflowData = files.home.match(/const WF_DATA = \[([\s\S]*?)\n\];/)?.[1] ?? '';
    const points = [...workflowData.matchAll(/svg:\s*\{ x:\s*([\d.]+), y:\s*([\d.]+) \}/g)]
      .map((match) => ({ x: Number(match[1]), y: Number(match[2]) }));
    assert.equal(points.length, 5);
    assert.equal(points.filter((point) => point.x < 450).length, 2);
    assert.equal(points.filter((point) => point.x > 450).length, 2);
    assert.equal(points.filter((point) => point.x === 450).length, 1);
    assert.ok(points[1].y <= 20);
    assert.ok(Math.hypot(points[0].x - 450, points[0].y - 240) > 300);
    assert.ok(Math.hypot(points[2].x - 450, points[2].y - 240) > 300);
  });
});
