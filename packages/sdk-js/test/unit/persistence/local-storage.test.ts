import { describe, it, expect, vi, afterEach } from 'vitest';
import { LocalStoragePersistence } from '../../../src/persistence/local-storage';

vi.mock('../../../src/utils/logger', () => ({
  getLogger: () => ({
    debug: vi.fn(),
    error: vi.fn(),
    warn: vi.fn(),
  }),
}));

describe('LocalStoragePersistence', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    localStorage.clear();
  });

  it('disables storage and logs once when quota is exceeded', () => {
    const quotaError = Object.assign(new Error('quota exceeded'), {
      name: 'QuotaExceededError',
    });
    const setItemSpy = vi
      .spyOn(localStorage, 'setItem')
      .mockImplementation(() => {
        throw quotaError;
      });
    const logger = {
      debug: vi.fn(),
      error: vi.fn(),
      warn: vi.fn(),
    } as any;
    const persistence = new LocalStoragePersistence('test', logger);

    persistence.set('foo', 'bar');

    expect(persistence.isStorageEnabled()).toBe(false);
    expect(persistence.get('foo')).toBe('bar'); // kept in memory
    expect(setItemSpy).toHaveBeenCalledTimes(1);
    expect(logger.error).toHaveBeenCalledTimes(1);

    persistence.set('baz', 'qux');

    expect(setItemSpy).toHaveBeenCalledTimes(1); // no further writes attempted
    expect(logger.error).toHaveBeenCalledTimes(1); // no spam
    expect(persistence.get('baz')).toBe('qux');
  });
});
