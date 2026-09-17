import type { HelpinWidgetController } from './client';
import type { Config } from './types';
import type { ShowArticleOptions, WidgetSettings } from './widget';

type HelpinCommand = (...args: any[]) => any;

const DEFAULT_RUNTIME_URL = 'https://cdn.helpin.ai/lib.js';

function normalizeRuntimeUrl(url?: string): string {
  return url?.trim() || DEFAULT_RUNTIME_URL;
}

function resolveNamespace(config: Partial<Config>): string {
  return config.namespace || 'helpin';
}

function resolveRuntimeUrl(config: Partial<Config>): string {
  const explicit = config.widgetRuntimeUrl || config.widget_runtime_url;
  if ((config.supportOnly || config.support_only) && !explicit) {
    if (!config.host) throw new Error('supportOnly requires an explicit host');
    return new URL('/sdk/lib.js', config.host).href;
  }
  return normalizeRuntimeUrl(explicit);
}

function resolveRuntimeChannel(config: Partial<Config>): string | undefined {
  return config.widgetRuntimeChannel || config.widget_runtime_channel;
}

function resolveRuntimeVersion(config: Partial<Config>): string | undefined {
  return config.widgetRuntimeVersion || config.widget_runtime_version;
}

function getWindow(): (Window & typeof globalThis) | null {
  return typeof window === 'undefined' ? null : window;
}

function getDocument(): Document | null {
  return typeof document === 'undefined' ? null : document;
}

function scriptNamespace(script: HTMLScriptElement): string {
  return script.getAttribute('data-namespace') || 'helpin';
}

function isHelpinRuntimeScript(script: HTMLScriptElement, namespace: string): boolean {
  if (scriptNamespace(script) !== namespace) {
    return false;
  }

  if (script.getAttribute('data-helpin-runtime') === 'hosted') {
    return true;
  }

  return Boolean(
    script.getAttribute('data-widget-key') &&
    (script.src.includes('/lib.js') || script.src.includes('/helpin.')),
  );
}

export class HostedWidgetController implements HelpinWidgetController {
  private namespace: string;
  private supportOnly: boolean;
  private runtimeUrl: string;
  private runtimeChannel?: string;
  private runtimeVersion?: string;
  private currentSettings: WidgetSettings | null = null;
  private pendingCommands: any[][] = [];
  private hasBooted = false;

  constructor(config: Partial<Config>) {
    this.namespace = resolveNamespace(config);
    this.supportOnly = config.supportOnly ?? config.support_only ?? false;
    this.runtimeUrl = resolveRuntimeUrl(config);
    this.runtimeChannel = resolveRuntimeChannel(config);
    this.runtimeVersion = resolveRuntimeVersion(config);
  }

  boot(settings: WidgetSettings): void {
    const widgetKey = settings.widgetKey || settings.key;
    if (!widgetKey) {
      console.error('[Helpin] Hosted widget boot skipped: widgetKey is required.');
      return;
    }

    this.currentSettings = {
      ...settings,
      widgetKey,
    };
    this.ensureRuntime();
    this.flushPendingCommands();
    this.callRuntime('boot', this.currentSettings);
    this.hasBooted = true;
  }

  shutdown(): void {
    this.command('shutdown');
    this.hasBooted = false;
  }

  show(): void {
    this.command('show');
  }

  hide(): void {
    this.command('hide');
  }

  open(): void {
    this.command('open');
  }

  close(): void {
    this.command('close');
  }

  toggle(): void {
    this.command('toggle');
  }

  openMessages(): void {
    this.command('openMessages');
  }

  openNewMessage(content?: string): void {
    this.command('openNewMessage', content);
  }

  openConversation(conversationId: string): void {
    this.command('openConversation', conversationId);
  }

  openArticle(articleKey: string, options?: ShowArticleOptions): void {
    this.command('openArticle', articleKey, options);
  }

  onOpen(callback: (...args: any[]) => void): void {
    this.command('onOpen', callback);
  }

  onClose(callback: (...args: any[]) => void): void {
    this.command('onClose', callback);
  }

  onUnreadCountChange(callback: (...args: any[]) => void): void {
    this.command('onUnreadCountChange', callback);
  }

  onUserEmailSupplied(callback: (...args: any[]) => void): void {
    this.command('onUserEmailSupplied', callback);
  }

  onConversationStarted(callback: (...args: any[]) => void): void {
    this.command('onConversationStarted', callback);
  }

  onMessageReceived(callback: (...args: any[]) => void): void {
    this.command('onMessageReceived', callback);
  }

  getVisitorId(): string {
    const result = this.callRuntime('getVisitorId');
    return typeof result === 'string' ? result : '';
  }

  isWidgetReady(): boolean {
    return this.callRuntime('isWidgetReady') === true;
  }

  private command(method: string, ...args: any[]): void {
    if (!this.currentSettings?.widgetKey) {
      this.pendingCommands.push([method, ...args]);
      return;
    }

    if (!this.hasBooted && method !== 'boot') {
      this.boot(this.currentSettings);
    }
    this.callRuntime(method, ...args);
  }

  private ensureRuntime(): void {
    const doc = getDocument();
    if (!doc || !this.currentSettings?.widgetKey) {
      return;
    }

    this.ensureCommandStub();

    const existingRuntime = Array.from(doc.scripts).some((script) =>
      isHelpinRuntimeScript(script, this.namespace),
    );
    if (existingRuntime) {
      return;
    }

    const script = doc.createElement('script');
    script.async = true;
    script.src = this.runtimeUrl;
    script.setAttribute('data-helpin-runtime', 'hosted');
    script.setAttribute('data-widget-key', this.currentSettings.widgetKey);
    script.setAttribute('data-host', this.currentSettings.host || '');
    script.setAttribute('data-namespace', this.namespace);
    script.setAttribute('data-no-auto-init', 'true');
    if (this.supportOnly) script.setAttribute('data-support-only', 'true');
    if (this.runtimeChannel) {
      script.setAttribute('data-runtime-channel', this.runtimeChannel);
    }
    if (this.runtimeVersion) {
      script.setAttribute('data-runtime-version', this.runtimeVersion);
    }

    doc.head.appendChild(script);
  }

  private ensureCommandStub(): HelpinCommand | null {
    const win = getWindow();
    if (!win) {
      return null;
    }

    const queueName = `${this.namespace}Q`;
    (win as any)[queueName] = (win as any)[queueName] || [];

    if (typeof (win as any)[this.namespace] !== 'function') {
      (win as any)[this.namespace] = (...args: any[]) => {
        (win as any)[queueName].push(args);
      };
    }

    return (win as any)[this.namespace] as HelpinCommand;
  }

  private callRuntime(method: string, ...args: any[]): any {
    const command = this.ensureCommandStub();
    if (!command) {
      return undefined;
    }
    return command(method, ...args);
  }

  private flushPendingCommands(): void {
    const pending = this.pendingCommands;
    this.pendingCommands = [];
    for (const command of pending) {
      this.callRuntime(command[0], ...command.slice(1));
    }
  }
}
