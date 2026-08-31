import http from 'k6/http';
import exec from 'k6/execution';
import { check, sleep } from 'k6';
import { SharedArray } from 'k6/data';
import { Counter, Rate } from 'k6/metrics';

const targetUrl = __ENV.TARGET_URL || 'http://127.0.0.1:3000/api/v1/event';
const targetUrls = (__ENV.TARGET_URLS || targetUrl)
  .split(',')
  .map((value) => value.trim())
  .filter(Boolean);
const token = __ENV.TOKEN || 'e2e-server-secret';
const profile = (__ENV.PROFILE || 'arrival').toLowerCase();
const eventRate = integerEnv('EVENT_RATE', 300);
const duration = __ENV.DURATION || '60s';
const batchSize = integerEnv('BATCH_SIZE', 1);
const visitors = integerEnv('VISITORS', 200000);
const preAllocatedVUs = integerEnv('PRE_ALLOCATED_VUS', 512);
const maxVUs = integerEnv('MAX_VUS', preAllocatedVUs);
const connections = integerEnv('CONNECTIONS', 256);
const p50LimitMs = numberEnv('P50_LIMIT_MS', 10);
const p99LimitMs = numberEnv('P99_LIMIT_MS', 25);
const summaryPath = __ENV.SUMMARY_PATH || 'k6-capture-summary.json';
const noConnectionReuse = booleanEnv('NO_CONNECTION_REUSE', false);
const noVUConnectionReuse = booleanEnv('NO_VU_CONNECTION_REUSE', false);
const clientIp = __ENV.CLIENT_IP || '203.0.113.42';
const userAgent = __ENV.USER_AGENT || 'HelpinK6Capture/1.0';
const dataProfile = (__ENV.DATA_PROFILE || 'simple').toLowerCase();
const realisticUserAgentLimit = integerEnv('REALISTIC_USER_AGENTS', 9999);
const realisticIpPoolSize = integerEnv('REALISTIC_IPS', 65536);
const userAgentFile = __ENV.USER_AGENT_FILE || '/fixtures/user-agents.txt';
const realisticUserAgents = new SharedArray('realistic user agents', function () {
  return open(userAgentFile)
    .split(/\r?\n/)
    .map((value) => value.trim())
    .filter(Boolean)
    .slice(0, realisticUserAgentLimit);
});

const botUserAgents = [
  'Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)',
  'Mozilla/5.0 (compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm)',
  'facebookexternalhit/1.1 (+http://www.facebook.com/externalhit_uatext.php)',
  'Slackbot-LinkExpanding 1.0 (+https://api.slack.com/robots)',
];
const aiBotUserAgents = [
  'Mozilla/5.0 (compatible; GPTBot/1.0; +https://openai.com/gptbot)',
  'Mozilla/5.0 (compatible; ClaudeBot/1.0; +https://www.anthropic.com/bot)',
  'Mozilla/5.0 (compatible; OAI-SearchBot/1.0; +https://openai.com/searchbot)',
  'Mozilla/5.0 (compatible; Claude-SearchBot/1.0; +https://www.anthropic.com/searchbot)',
  'Mozilla/5.0 (compatible; PerplexityBot/1.0; +https://www.perplexity.ai/bot)',
  'Mozilla/5.0 (compatible; ChatGPT-User/1.0; +https://openai.com/bot)',
  'Mozilla/5.0 (compatible; Claude-User/1.0; +https://www.anthropic.com/user)',
  'Mozilla/5.0 (compatible; Perplexity-User/1.0; +https://www.perplexity.ai/user)',
  'Mozilla/5.0 (compatible; Google-CloudVertexBot/1.0; +https://cloud.google.com/vertex-ai)',
  'Mozilla/5.0 (compatible; OAI-AdsBot/1.0; +https://openai.com/adsbot)',
  'Mozilla/5.0 (compatible; Amazonbot/1.0; +https://developer.amazon.com/amazonbot)',
  'Mozilla/5.0 (compatible; Amzn-SearchBot/1.0; +https://developer.amazon.com/searchbot)',
  'Mozilla/5.0 (compatible; Amzn-User/1.0; +https://developer.amazon.com/user)',
];
const ipFirstOctets = [
  1, 2, 5, 8, 9, 11, 13, 14, 15, 17, 18, 20, 23, 24, 27, 31, 34, 35, 36, 37,
  38, 39, 41, 42, 43, 44, 45, 46, 47, 49, 50, 51, 52, 54, 57, 58, 59, 60, 61,
  62, 63, 64, 65, 66, 67, 68, 69, 70, 71, 72, 73, 74, 75, 76, 77, 78, 79, 80,
  81, 82, 83, 84, 85, 86, 87, 88, 89, 90, 91, 92, 93, 94, 95,
];
const ipv6Prefixes = ['2001:4860', '2606:4700', '2a00:1450', '2404:6800'];
const domains = ['app.example.test', 'www.example.test', 'docs.example.test', 'shop.example.test'];
const paths = ['/home', '/pricing', '/docs/getting-started', '/dashboard', '/checkout', '/settings/team'];
const referrers = [
  '',
  'https://www.google.com/search?q=analytics',
  'https://www.linkedin.com/feed/',
  'https://news.ycombinator.com/',
  'https://partner.example.test/reviews',
];
const utmSources = ['google', 'linkedin', 'newsletter', 'partner', 'product-hunt', 'reddit'];
const utmMediums = ['cpc', 'paid-social', 'email', 'referral', 'organic-social'];
const utmCampaigns = ['brand-search', 'summer-launch', 'retargeting', 'customer-newsletter'];
const utmContents = ['hero-cta', 'sidebar', 'text-link', 'video', 'comparison-table'];
const utmTerms = ['product-analytics', 'crm-signals', 'customer-support', 'project-management'];
const languages = ['en-US', 'en-GB', 'de-DE', 'fr-FR', 'sv-SE', 'es-ES', 'ja-JP'];
const screenResolutions = ['1920x1080', '2560x1440', '1440x900', '1366x768', '390x844'];
const viewportSizes = ['1903x969', '1440x780', '1280x720', '390x755', '412x915'];

if (eventRate % batchSize !== 0) {
  throw new Error(`EVENT_RATE (${eventRate}) must be divisible by BATCH_SIZE (${batchSize})`);
}
if (targetUrls.length === 0) {
  throw new Error('TARGET_URLS must contain at least one capture endpoint');
}
if (profile !== 'arrival' && profile !== 'connections') {
  throw new Error(`PROFILE must be arrival or connections, got ${profile}`);
}
if (dataProfile !== 'simple' && dataProfile !== 'realistic') {
  throw new Error(`DATA_PROFILE must be simple or realistic, got ${dataProfile}`);
}
if (dataProfile === 'realistic' && realisticUserAgents.length < realisticUserAgentLimit) {
  throw new Error(
    `USER_AGENT_FILE contains ${realisticUserAgents.length} entries; ` +
      `REALISTIC_USER_AGENTS requires ${realisticUserAgentLimit}`,
  );
}

const requestRate = eventRate / batchSize;
const eventsAttempted = new Counter('events_attempted');
const eventsAccepted = new Counter('events_accepted');
const eventRequestSuccess = new Rate('event_request_success');
let connectionProfileStarted = false;

const scenario = profile === 'arrival'
  ? {
      executor: 'constant-arrival-rate',
      rate: requestRate,
      timeUnit: '1s',
      duration,
      preAllocatedVUs,
      maxVUs,
      gracefulStop: '30s',
      tags: { profile: 'arrival', batch_size: String(batchSize) },
    }
  : {
      executor: 'constant-vus',
      vus: connections,
      duration,
      gracefulStop: '30s',
      tags: { profile: 'connections', batch_size: String(batchSize) },
    };

export const options = {
  discardResponseBodies: true,
  noConnectionReuse,
  noVUConnectionReuse,
  scenarios: { capture: scenario },
  summaryTrendStats: ['avg', 'min', 'med', 'max', 'p(90)', 'p(95)', 'p(99)', 'count'],
  thresholds: {
    checks: ['rate==1'],
    event_request_success: ['rate==1'],
    http_req_failed: ['rate==0'],
    http_req_duration: [`p(50)<${p50LimitMs}`, `p(99)<${p99LimitMs}`],
    dropped_iterations: ['count==0'],
  },
};

export default function () {
  if (profile === 'connections' && !connectionProfileStarted) {
    sleep((exec.vu.idInTest - 1) / requestRate);
    connectionProfileStarted = true;
  }

  const iteration = exec.scenario.iterationInTest;
  const firstSequence = iteration * batchSize;
  const visitor = dataProfile === 'realistic' ? iteration % visitors : null;
  const requestClientIp = dataProfile === 'realistic' ? clientIpForVisitor(visitor) : clientIp;
  const requestUserAgent = dataProfile === 'realistic' ? userAgentForVisitor(visitor) : userAgent;
  const events = [];

  for (let offset = 0; offset < batchSize; offset += 1) {
    const sequence = firstSequence + offset;
    const eventVisitor = dataProfile === 'realistic' ? visitor : sequence % visitors;
    events.push(
      dataProfile === 'realistic'
        ? realisticEvent(sequence, eventVisitor, offset)
        : simpleEvent(sequence, eventVisitor),
    );
  }

  const payload = batchSize === 1 ? events[0] : events;
  eventsAttempted.add(batchSize);
  const requestUrl = targetUrls[iteration % targetUrls.length];
  const url = dataProfile === 'realistic' ? requestUrlWithPolicies(requestUrl, visitor) : requestUrl;
  const response = http.post(url, JSON.stringify(payload), {
    headers: {
      'Content-Type': 'application/json',
      'X-Auth-Token': token,
      'User-Agent': requestUserAgent,
      'X-Forwarded-For': requestClientIp,
    },
    tags: { name: 'capture_event' },
    timeout: __ENV.REQUEST_TIMEOUT || '30s',
  });

  const accepted = response.status === 200;
  eventRequestSuccess.add(accepted);
  if (accepted) {
    eventsAccepted.add(batchSize);
  }
  check(response, {
    'capture returned HTTP 200': (result) => result.status === 200,
  });

  if (profile === 'connections') {
    const intervalSeconds = connections / requestRate;
    const requestSeconds = response.timings.duration / 1000;
    sleep(Math.max(0, intervalSeconds - requestSeconds));
  }
}

export function handleSummary(data) {
  const metric = (name, value) => data.metrics[name]?.values?.[value] ?? null;
  const result = {
    configuration: {
      target_url: targetUrl,
      target_urls: targetUrls,
      profile,
      event_rate: eventRate,
      request_rate: requestRate,
      duration,
      batch_size: batchSize,
      visitors,
      pre_allocated_vus: preAllocatedVUs,
      max_vus: maxVUs,
      connections,
      data_profile: dataProfile,
      client_ip: clientIp,
      user_agent: userAgent,
      realistic_user_agents: dataProfile === 'realistic' ? realisticUserAgents.length : 1,
      realistic_ips: dataProfile === 'realistic' ? realisticIpPoolSize : 1,
      no_connection_reuse: noConnectionReuse,
      no_vu_connection_reuse: noVUConnectionReuse,
    },
    results: {
      events_attempted: metric('events_attempted', 'count'),
      events_accepted: metric('events_accepted', 'count'),
      requests: metric('http_reqs', 'count'),
      request_rate: metric('http_reqs', 'rate'),
      failed_request_rate: metric('http_req_failed', 'rate'),
      dropped_iterations: metric('dropped_iterations', 'count') || 0,
      vus_max: metric('vus_max', 'max'),
      latency_ms: {
        p50: metric('http_req_duration', 'med'),
        p95: metric('http_req_duration', 'p(95)'),
        p99: metric('http_req_duration', 'p(99)'),
        max: metric('http_req_duration', 'max'),
      },
    },
    thresholds: Object.fromEntries(
      Object.entries(data.metrics)
        .filter(([, value]) => value.thresholds)
        .map(([name, value]) => [name, value.thresholds]),
    ),
    raw: data,
  };

  const concise = [
    '',
    `profile=${profile} event_rate=${eventRate}/s request_rate=${requestRate}/s batch=${batchSize}`,
    `requests=${result.results.requests} accepted_events=${result.results.events_accepted} dropped=${result.results.dropped_iterations}`,
    `latency_ms p50=${result.results.latency_ms.p50} p95=${result.results.latency_ms.p95} p99=${result.results.latency_ms.p99} max=${result.results.latency_ms.max}`,
    `summary=${summaryPath}`,
    '',
  ].join('\n');

  return {
    stdout: concise,
    [summaryPath]: `${JSON.stringify(result, null, 2)}\n`,
  };
}

function simpleEvent(sequence, visitor) {
  return {
    api_key: token,
    event_id: `k6-${sequence}`,
    event_type: 'k6_capture_test',
    url: 'https://example.test/k6',
    user: {
      anonymous_id: `k6-visitor-${visitor}`,
      id: `k6-user-${visitor}`,
    },
    event_attributes: {
      load_test: 'k6-capture',
      sequence,
    },
    src: 'k6-capture',
  };
}

function realisticEvent(sequence, visitor, batchOffset) {
  const bucket = sequence % 100;
  const eventType = bucket < 45
    ? 'page_view'
    : bucket < 80
      ? 'product_activity'
      : bucket < 90
        ? 'autocapture'
        : bucket < 95
          ? 'user_identify'
          : 'checkout_completed';
  const domain = domains[visitor % domains.length];
  const path = paths[(visitor + batchOffset) % paths.length];
  const includeUtm = sequence % 4 === 0;
  const utm = includeUtm
    ? {
        source: utmSources[visitor % utmSources.length],
        medium: utmMediums[visitor % utmMediums.length],
        campaign: utmCampaigns[(visitor + batchOffset) % utmCampaigns.length],
        content: utmContents[sequence % utmContents.length],
        term: utmTerms[visitor % utmTerms.length],
      }
    : undefined;
  const query = includeUtm
    ? `?utm_source=${utm.source}&utm_medium=${utm.medium}&utm_campaign=${utm.campaign}`
    : `?view=${sequence % 7}&experiment=exp-${visitor % 32}`;
  const event = {
    api_key: token,
    event_id: `k6-${sequence}`,
    event_type: eventType,
    url: `https://${domain}${path}${query}`,
    referrer: referrers[visitor % referrers.length],
    page_title: `Load test page ${path}`,
    user_language: languages[visitor % languages.length],
    screen_resolution: screenResolutions[visitor % screenResolutions.length],
    vp_size: viewportSizes[visitor % viewportSizes.length],
    doc_encoding: 'UTF-8',
    user: {
      anonymous_id: `k6-visitor-${visitor}`,
      id: `k6-user-${visitor}`,
      email: `load-${visitor % 100000}@example.test`,
      first_name: `Load${visitor % 1000}`,
      last_name: `Visitor${visitor % 10000}`,
      created_at: '2025-01-01T00:00:00Z',
      custom: {
        plan: ['free', 'starter', 'growth', 'enterprise'][visitor % 4],
        cohort: `cohort-${visitor % 24}`,
        seats: 1 + (visitor % 250),
        trial: visitor % 5 === 0,
      },
    },
    company: visitor % 2 === 0
      ? {
          id: `k6-company-${visitor % 50000}`,
          name: `Load Company ${visitor % 50000}`,
          created_at: '2024-01-01T00:00:00Z',
          custom: {
            industry: ['software', 'retail', 'finance', 'media'][visitor % 4],
            employees: 10 + (visitor % 5000),
            region: ['na', 'emea', 'apac', 'latam'][visitor % 4],
          },
        }
      : undefined,
    event_attributes: {
      load_test: 'k6-capture-realistic',
      sequence,
      request_visitor: visitor,
      batch_offset: batchOffset,
      feature: ['crm', 'support', 'projects', 'knowledge'][sequence % 4],
      duration_ms: sequence % 30000,
      successful: sequence % 17 !== 0,
      properties: {
        variant: `v${sequence % 8}`,
        region_hint: ['north', 'south', 'east', 'west'][visitor % 4],
      },
    },
    autocapture_attributes: eventType === 'autocapture'
      ? {
          event_type: sequence % 2 === 0 ? 'click' : 'submit',
          tag_name: sequence % 2 === 0 ? 'button' : 'form',
          el_text: `Action ${sequence % 20}`,
          attr__id: `action-${sequence % 100}`,
          attr__class: `button variant-${sequence % 5}`,
          attr__href: `https://${domain}${path}`,
        }
      : undefined,
    utm,
    ids: visitor % 5 !== 0
      ? {
          ajs_anonymous_id: `ajs-anon-${visitor}`,
          ajs_user_id: `ajs-user-${visitor}`,
          ga: `GA1.1.${100000000 + (visitor % 900000000)}.${1700000000 + (visitor % 1000000)}`,
          fbp: `fb.1.${1700000000 + (visitor % 1000000)}.${1000000000 + (visitor % 9000000000)}`,
        }
      : undefined,
    click_id: sequence % 10 === 0
      ? {
          gclid: `gclid-${visitor}-${sequence % 100000}`,
          fbclid: `fbclid-${visitor}-${sequence % 100000}`,
        }
      : undefined,
    src: 'k6-capture',
  };

  return event;
}

function userAgentForVisitor(visitor) {
  const distribution = mix32(visitor) % 100;
  if (distribution < 80) {
    return realisticUserAgents[mix32(visitor + 1) % Math.min(64, realisticUserAgents.length)];
  }
  if (distribution < 96) {
    return realisticUserAgents[mix32(visitor + 2) % realisticUserAgents.length];
  }
  if (distribution === 96) {
    return botUserAgents[mix32(visitor + 3) % botUserAgents.length];
  }
  if (distribution < 99) {
    return aiBotUserAgents[mix32(visitor + 4) % aiBotUserAgents.length];
  }
  return `HelpinUnknownClient/${1 + (visitor % 1000)}.0`;
}

function clientIpForVisitor(visitor) {
  const ordinal = visitor % realisticIpPoolSize;
  if (ordinal % 10 === 0) {
    const prefix = ipv6Prefixes[ordinal % ipv6Prefixes.length];
    return `${prefix}:${hexWord(ordinal * 17)}:${hexWord(ordinal * 31)}::${hexWord(ordinal * 47)}:${hexWord(ordinal * 61)}`;
  }
  const first = ipFirstOctets[(ordinal * 17) % ipFirstOctets.length];
  const second = 1 + (Math.floor(ordinal / ipFirstOctets.length) % 223);
  const third = 1 + (Math.floor(ordinal / 251) % 253);
  const fourth = 1 + ((ordinal * 37) % 253);
  return `${first}.${second}.${third}.${fourth}`;
}

function requestUrlWithPolicies(baseUrl, visitor) {
  if (visitor % 10 === 0) {
    const separator = baseUrl.includes('?') ? '&' : '?';
    return `${baseUrl}${separator}cookie_policy=keep&ip_policy=keep`;
  }
  const policyBucket = visitor % 20;
  const policy = policyBucket === 1 ? 'strict' : policyBucket < 5 ? 'comply' : 'keep';
  const separator = baseUrl.includes('?') ? '&' : '?';
  return `${baseUrl}${separator}cookie_policy=${policy}&ip_policy=${policy}`;
}

function hexWord(value) {
  return (value % 65536).toString(16);
}

function mix32(value) {
  let mixed = (value + 0x9e3779b9) >>> 0;
  mixed ^= mixed >>> 16;
  mixed = Math.imul(mixed, 0x85ebca6b) >>> 0;
  mixed ^= mixed >>> 13;
  mixed = Math.imul(mixed, 0xc2b2ae35) >>> 0;
  mixed ^= mixed >>> 16;
  return mixed >>> 0;
}

function integerEnv(name, fallback) {
  const value = Number.parseInt(__ENV[name] || String(fallback), 10);
  if (!Number.isInteger(value) || value <= 0) {
    throw new Error(`${name} must be a positive integer`);
  }
  return value;
}

function numberEnv(name, fallback) {
  const value = Number.parseFloat(__ENV[name] || String(fallback));
  if (!Number.isFinite(value) || value <= 0) {
    throw new Error(`${name} must be a positive number`);
  }
  return value;
}

function booleanEnv(name, fallback) {
  const value = (__ENV[name] || String(fallback)).toLowerCase();
  if (value !== 'true' && value !== 'false') {
    throw new Error(`${name} must be true or false`);
  }
  return value === 'true';
}
