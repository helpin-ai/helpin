import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
  generateId,
  isValidEmail,
  debounce,
  getUtmParams,
  parseQueryString,
  isString,
  isObject,
  parseLogLevel,
} from '../../../src/utils/helpers';
import { LogLevel } from '../../../src/utils/logger';
import * as commonUtils from '../../../src/utils/common';

// Mock dependencies
vi.mock('../../../src/utils/common', () => ({
  generateRandom: vi.fn(),
}));

describe('Helper Functions', () => {
  describe('generateId', () => {
    it('should call generateRandom with 10', () => {
      generateId();
      expect(commonUtils.generateRandom).toHaveBeenCalledWith(10);
    });
  });

  describe('isValidEmail', () => {
    it('should return true for valid emails', () => {
      expect(isValidEmail('test@example.com')).toBe(true);
      expect(isValidEmail('test.name+tag@example.co.uk')).toBe(true);
    });

    it('should return false for invalid emails', () => {
      expect(isValidEmail('notanemail')).toBe(false);
      expect(isValidEmail('missing@tld')).toBe(false);
      expect(isValidEmail('@missingusername.com')).toBe(false);
    });
  });

  describe('debounce', () => {
    beforeEach(() => {
      vi.useFakeTimers();
    });

    afterEach(() => {
      vi.useRealTimers();
    });

    it('should debounce function calls', () => {
      const func = vi.fn();
      const debouncedFunc = debounce(func, 1000);

      debouncedFunc();
      debouncedFunc();
      debouncedFunc();

      expect(func).not.toHaveBeenCalled();

      vi.runAllTimers();

      expect(func).toHaveBeenCalledTimes(1);
    });
  });

  describe('getUtmParams', () => {
    const originalWindow = global.window;

    beforeEach(() => {
      // @ts-ignore
      delete global.window;
      // @ts-ignore
      global.window = { location: { search: '' } };
    });

    afterEach(() => {
      global.window = originalWindow;
    });

    it('should extract UTM params from URL', () => {
      global.window.location.search =
        '?utm_source=test&utm_medium=email&utm_campaign=summer';
      expect(getUtmParams()).toEqual({
        source: 'test',
        medium: 'email',
        campaign: 'summer',
      });
    });

    it('should return an empty object if no UTM params', () => {
      global.window.location.search = '?param1=value1&param2=value2';
      expect(getUtmParams()).toEqual({});
    });
  });

  describe('parseQueryString', () => {
    it('should parse query string correctly', () => {
      const queryString =
        '?param1=value1&param2=value2&param3=value%20with%20spaces';
      expect(parseQueryString(queryString)).toEqual({
        param1: 'value1',
        param2: 'value2',
        param3: 'value with spaces',
      });
    });

    it('should handle empty query string', () => {
      expect(parseQueryString('')).toEqual({});
    });
  });

  describe('isString', () => {
    it('should return true for strings', () => {
      expect(isString('test')).toBe(true);
      expect(isString(new String('test'))).toBe(true);
    });

    it('should return false for non-strings', () => {
      expect(isString(123)).toBe(false);
      expect(isString({})).toBe(false);
      expect(isString(null)).toBe(false);
      expect(isString(undefined)).toBe(false);
    });
  });

  describe('isObject', () => {
    it('should return true for plain objects', () => {
      expect(isObject({})).toBe(true);
      expect(isObject({ key: 'value' })).toBe(true);
    });

    it('should return false for non-objects and non-plain objects', () => {
      expect(isObject(null)).toBe(false);
      expect(isObject([])).toBe(false);
      expect(isObject('string')).toBe(false);
      expect(isObject(123)).toBe(false);
      expect(isObject(new Date())).toBe(false);
    });
  });

  describe('parseLogLevel', () => {
    it('should parse valid log levels', () => {
      expect(parseLogLevel('ERROR')).toBe(LogLevel.ERROR);
      expect(parseLogLevel('WARN')).toBe(LogLevel.WARN);
      expect(parseLogLevel('INFO')).toBe(LogLevel.INFO);
      expect(parseLogLevel('DEBUG')).toBe(LogLevel.DEBUG);
    });

    it('should be case-insensitive', () => {
      expect(parseLogLevel('error')).toBe(LogLevel.ERROR);
      expect(parseLogLevel('WaRn')).toBe(LogLevel.WARN);
    });

    it('should return ERROR for null input', () => {
      expect(parseLogLevel(null)).toBe(LogLevel.ERROR);
    });

    it('should return ERROR for invalid input', () => {
      expect(parseLogLevel('INVALID')).toBe(LogLevel.ERROR);
      expect(parseLogLevel('123')).toBe(LogLevel.ERROR);
    });
  });
});
