import { LogLevel } from '../utils/logger';
import { generateRandom } from '../utils/common';

export function generateId(): string {
  return generateRandom(10);
}

export function isValidEmail(email: string): boolean {
  const re =
    /^(([^<>()[\]\\.,;:\s@"]+(\.[^<>()[\]\\.,;:\s@"]+)*)|(".+"))@((\[[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\])|(([a-zA-Z\-0-9]+\.)+[a-zA-Z]{2,}))$/;
  return re.test(String(email).toLowerCase());
}

export function debounce<T extends (...args: P) => any, P extends any[]>(
  func: T,
  wait: number,
): (...args: P) => void {
  let timeout: ReturnType<typeof setTimeout>;
  return function (...args: P): void {
    const later = () => {
      clearTimeout(timeout);
      func(...args);
    };
    clearTimeout(timeout);
    timeout = setTimeout(later, wait);
  };
}

export function getUtmParams(): Record<string, string> {
  const utmParams: Record<string, string> = {};
  const queryParams = parseQueryString(window.location.search);
  const utmKeys = [
    'utm_source',
    'utm_medium',
    'utm_campaign',
    'utm_term',
    'utm_content',
  ];

  utmKeys.forEach((key) => {
    if (queryParams[key]) {
      utmParams[key.replace('utm_', '')] = queryParams[key];
    }
  });

  return utmParams;
}

export function parseQueryString(queryString: string): Record<string, string> {
  const params: Record<string, string> = {};
  // Remove the leading '?' if present
  const queries = queryString.replace(/^\?/, '').split('&');

  for (let i = 0; i < queries.length; i++) {
    const pair = queries[i].split('=');
    if (pair[0] !== '') {
      params[decodeURIComponent(pair[0])] = decodeURIComponent(pair[1] || '');
    }
  }

  return params;
}

export function isString(value: any): boolean {
  return typeof value === 'string' || value instanceof String;
}

export function isObject(value: any): boolean {
  return (
    value !== null && typeof value === 'object' && value.constructor === Object
  );
}

export function parseLogLevel(value: string | null): LogLevel {
  if (value === null) {
    return LogLevel.ERROR; // Default value
  }

  const upperValue = value.toUpperCase();
  const level = LogLevel[upperValue as keyof typeof LogLevel] as LogLevel;

  if (level || level === 0) {
    return level;
  }

  return LogLevel.ERROR;
}

export const getExclusionState = () => {
  let state: string | null = 'false';

  if (typeof window !== 'undefined' && window.localStorage) {
    state = localStorage.getItem('helpin_exclusion');
  }

  return state === undefined || state === null || state === 'false'
    ? false
    : true;
};
