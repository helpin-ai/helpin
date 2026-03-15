const BOT_UA_PATTERNS = [
  /bot/i,
  /crawl/i,
  /spider/i,
  /slurp/i,
  /mediapartners/i,
  /googlebot/i,
  /bingbot/i,
  /yandexbot/i,
  /baiduspider/i,
  /facebookexternalhit/i,
  /twitterbot/i,
  /linkedinbot/i,
  /whatsapp/i,
  /telegrambot/i,
  /applebot/i,
  /duckduckbot/i,
  /semrushbot/i,
  /ahrefsbot/i,
  /dotbot/i,
  /rogerbot/i,
  /screaming frog/i,
  /headlesschrome/i,
  /phantomjs/i,
  /selenium/i,
  /puppeteer/i,
  /playwright/i,
  /webdriver/i,
  /lighthouse/i,
  /pagespeed/i,
  /google-inspectiontool/i,
  /chrome-lighthouse/i,
];

/**
 * Detects if the current user agent is a bot or crawler.
 * Returns true if the UA matches known bot patterns or if
 * headless browser signals are detected.
 */
export function isBot(): boolean {
  if (typeof navigator === 'undefined') return true;

  const ua = navigator.userAgent;
  if (!ua) return true;

  // Check UA string against known bot patterns
  for (const pattern of BOT_UA_PATTERNS) {
    if (pattern.test(ua)) return true;
  }

  // Headless browser detection
  if ((navigator as any).webdriver) return true;

  return false;
}
