import '@testing-library/jest-dom';
import { vi } from 'vitest';

// Mock XMLHttpRequest
global.XMLHttpRequest = vi.fn(() => ({
  open: vi.fn(),
  send: vi.fn(),
  setRequestHeader: vi.fn(),
})) as any;

global.fetch = vi.fn() as any;

// Mock localStorage
const localStorageMock = {
  getItem: vi.fn(),
  setItem: vi.fn(),
  clear: vi.fn(),
  length: 0,
  key: vi.fn(),
  removeItem: vi.fn(),
};
Object.defineProperty(global, 'localStorage', { value: localStorageMock });

// Mock navigator
Object.defineProperty(global.navigator, 'sendBeacon', {
  value: vi.fn(),
});
