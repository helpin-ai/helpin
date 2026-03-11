import { HelpinClient } from '../core/client';
import { debounce } from '../utils/helpers';

export class ScrollDepth {
  private client: HelpinClient;
  private maxScrollDepth: number = 0;
  private milestones: number[] = [25, 50, 75, 90];
  private lastScrollDepth: number = 0;
  private documentElement: HTMLElement;
  private debouncedHandleScroll: () => void;

  constructor(client: HelpinClient) {
    this.client = client;
    this.documentElement = document.documentElement;
    this.debouncedHandleScroll = debounce(this.handleScroll.bind(this), 250);
    this.initializeEventListener();
  }

  private initializeEventListener(): void {
    window.addEventListener('scroll', this.debouncedHandleScroll);
  }

  public track(): void {
    const scrollDepth = this.getScrollDepth();

    if (scrollDepth > this.lastScrollDepth) {
      this.lastScrollDepth = scrollDepth;
      this.checkMilestones(scrollDepth);
    }
  }

  public send(eventType = '$scroll'): void {
    if (!this.lastScrollDepth) {
      // if there is no scroll depth, do not send the event
      return;
    }

    const props = {
      percent: this.lastScrollDepth,
      window_height: this.getWindowHeight(),
      document_height: this.getDocumentHeight(),
      scroll_distance: this.getScrollDistance(),
    };

    this.client.track(eventType, props);
  }

  private handleScroll(): void {
    this.track();
  }

  private getScrollDepth(): number {
    const windowHeight = this.getWindowHeight();
    const docHeight = this.getDocumentHeight();
    const scrollTop = this.getScrollDistance();
    const trackLength = docHeight - windowHeight;

    return Math.min(100, Math.floor((scrollTop / trackLength) * 100));
  }

  private getWindowHeight(): number {
    return (
      window.innerHeight ||
      this.documentElement.clientHeight ||
      document.body.clientHeight ||
      0
    );
  }

  private getDocumentHeight(): number {
    return Math.max(
      document.body.scrollHeight || 0,
      this.documentElement.scrollHeight || 0,
      document.body.offsetHeight || 0,
      this.documentElement.offsetHeight || 0,
      document.body.clientHeight || 0,
      this.documentElement.clientHeight || 0,
    );
  }

  private getScrollDistance(): number {
    return (
      window.pageYOffset ||
      this.documentElement.scrollTop ||
      document.body.scrollTop ||
      0
    );
  }

  private checkMilestones(scrollPercentage: number): void {
    const reachedMilestones = this.milestones.filter(
      (milestone) => scrollPercentage >= milestone,
    );

    reachedMilestones.forEach((milestone) => {
      this.send();
      this.milestones = this.milestones.filter((m) => m !== milestone);
    });
  }

  public destroy(): void {
    // Clean up all event listeners
    window.removeEventListener('scroll', this.debouncedHandleScroll);
  }
}
