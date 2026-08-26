import http from 'k6/http';
import exec from 'k6/execution';
import { check, sleep } from 'k6';
import { Counter, Rate } from 'k6/metrics';

const targetUrl = __ENV.TARGET_URL || 'http://127.0.0.1:3000/api/v1/event';
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

if (eventRate % batchSize !== 0) {
  throw new Error(`EVENT_RATE (${eventRate}) must be divisible by BATCH_SIZE (${batchSize})`);
}
if (profile !== 'arrival' && profile !== 'connections') {
  throw new Error(`PROFILE must be arrival or connections, got ${profile}`);
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
  const events = [];

  for (let offset = 0; offset < batchSize; offset += 1) {
    const sequence = firstSequence + offset;
    const visitor = sequence % visitors;
    events.push({
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
    });
  }

  const payload = batchSize === 1 ? events[0] : events;
  eventsAttempted.add(batchSize);
  const response = http.post(targetUrl, JSON.stringify(payload), {
    headers: {
      'Content-Type': 'application/json',
      'X-Auth-Token': token,
      'User-Agent': userAgent,
      'X-Forwarded-For': clientIp,
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
      profile,
      event_rate: eventRate,
      request_rate: requestRate,
      duration,
      batch_size: batchSize,
      visitors,
      pre_allocated_vus: preAllocatedVUs,
      max_vus: maxVUs,
      connections,
      client_ip: clientIp,
      user_agent: userAgent,
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
