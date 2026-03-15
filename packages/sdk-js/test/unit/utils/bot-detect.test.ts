import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { isBot } from '../../../src/utils/bot-detect';

describe('isBot', () => {
  const originalNavigator = globalThis.navigator;

  function mockUA(ua: string, webdriver = false) {
    Object.defineProperty(globalThis, 'navigator', {
      value: { userAgent: ua, webdriver },
      writable: true,
      configurable: true,
    });
  }

  afterEach(() => {
    Object.defineProperty(globalThis, 'navigator', {
      value: originalNavigator,
      writable: true,
      configurable: true,
    });
  });

  it('returns true for Googlebot', () => {
    mockUA('Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)');
    expect(isBot()).toBe(true);
  });

  it('returns true for Bingbot', () => {
    mockUA('Mozilla/5.0 (compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm)');
    expect(isBot()).toBe(true);
  });

  it('returns true for HeadlessChrome', () => {
    mockUA('Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 HeadlessChrome/90.0.4430.212');
    expect(isBot()).toBe(true);
  });

  it('returns true for Puppeteer', () => {
    mockUA('Mozilla/5.0 Puppeteer');
    expect(isBot()).toBe(true);
  });

  it('returns true for Selenium webdriver flag', () => {
    mockUA('Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/91.0', true);
    expect(isBot()).toBe(true);
  });

  it('returns true for PhantomJS', () => {
    mockUA('Mozilla/5.0 PhantomJS/2.1.1');
    expect(isBot()).toBe(true);
  });

  it('returns true for Facebook crawler', () => {
    mockUA('facebookexternalhit/1.1');
    expect(isBot()).toBe(true);
  });

  it('returns true for empty user agent', () => {
    mockUA('');
    expect(isBot()).toBe(true);
  });

  it('returns false for normal Chrome browser', () => {
    mockUA('Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36');
    expect(isBot()).toBe(false);
  });

  it('returns false for normal Firefox browser', () => {
    mockUA('Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0');
    expect(isBot()).toBe(false);
  });

  it('returns false for Safari on iOS', () => {
    mockUA('Mozilla/5.0 (iPhone; CPU iPhone OS 17_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Mobile/15E148 Safari/604.1');
    expect(isBot()).toBe(false);
  });

  it('returns true when navigator is undefined', () => {
    Object.defineProperty(globalThis, 'navigator', {
      value: undefined,
      writable: true,
      configurable: true,
    });
    expect(isBot()).toBe(true);
  });
});
