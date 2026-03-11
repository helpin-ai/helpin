import { HelpinClient } from '../core/client';

export class PageviewTracking {
  private client: HelpinClient;
  private lastPageUrl: string;

  constructor(client: HelpinClient) {
    this.client = client;
    this.lastPageUrl = window.location.href;
    this.trackInitialPageview();
    this.initializePageviewTracking();
  }

  private trackInitialPageview(): void {
    this.trackPageview();
  }

  private initializePageviewTracking(): void {
    // Handle browser back/forward buttons
    window.addEventListener('popstate', this.handlePageview.bind(this));

    // Handle programmatic navigation in SPAs
    const originalPushState = history.pushState;
    history.pushState = (...args: Parameters<typeof history.pushState>) => {
      originalPushState.apply(history, args);
      this.handlePageview();
    };

    // Handle hash changes (for SPAs that use hash-based routing)
    window.addEventListener('hashchange', this.handlePageview.bind(this));

    // Periodically check for changes (catches any missed navigations)
    setInterval(this.checkForUrlChange.bind(this), 1000);
  }

  private handlePageview(): void {
    this.trackPageview();
  }

  private checkForUrlChange(): void {
    if (window.location.href !== this.lastPageUrl) {
      this.trackPageview();
    }
  }

  private trackPageview(): void {
    const currentUrl = window.location.href;
    if (currentUrl !== this.lastPageUrl) {
      this.lastPageUrl = currentUrl;
      this.client.track('pageview', {
        url: currentUrl,
        referrer: document.referrer,
        title: document.title,
      });
    }
  }
}
