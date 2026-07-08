// Global jsdom shims for browser APIs that motion/react (Framer Motion) and
// vaul touch during render but jsdom does not implement:
// - matchMedia: read for prefers-reduced-motion / prefers-color-scheme checks
// - ResizeObserver: used by layout/layoutId animations (e.g. SegmentedControl's thumb)

if (!window.matchMedia) {
  window.matchMedia = ((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: () => {},
    removeListener: () => {},
    addEventListener: () => {},
    removeEventListener: () => {},
    dispatchEvent: () => false,
  })) as unknown as typeof window.matchMedia
}

if (typeof window.ResizeObserver === 'undefined') {
  class ResizeObserverStub {
    observe() {}
    unobserve() {}
    disconnect() {}
  }
  ;(window as unknown as { ResizeObserver: unknown }).ResizeObserver = ResizeObserverStub
}
