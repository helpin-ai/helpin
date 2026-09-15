import { describe, expect, it } from 'vitest';
import {
  buildWidgetInstallPrompt,
  type WidgetInstallFramework,
} from '../installPrompts';

const frameworks: WidgetInstallFramework[] = ['html', 'react', 'vue', 'nextjs'];

describe('buildWidgetInstallPrompt', () => {
  it.each(frameworks)('includes the real widget configuration for %s', (framework) => {
    const prompt = buildWidgetInstallPrompt({
      framework,
      widgetKey: 'widget-public-key',
      host: 'http://support.example.test:8080',
      runtimeURL: 'http://assets.example.test/sdk/lib.js',
    });

    expect(prompt).toContain('Public widget key: widget-public-key');
    expect(prompt).toContain('Helpin host: http://support.example.test:8080');
    expect(prompt).toContain('http://assets.example.test/sdk/lib.js');
    expect(prompt).toContain('openArticle(articleKey, options?)');
    expect(prompt).toContain('production build passes');
    expect(prompt).not.toContain('{{WIDGET_KEY}}');
  });

  it('provides Vue and Nuxt-specific setup guidance', () => {
    const prompt = buildWidgetInstallPrompt({
      framework: 'vue',
      widgetKey: 'vue-key',
      host: 'http://support.example.test:8080',
      runtimeURL: 'http://assets.example.test/sdk/lib.js',
    });

    expect(prompt).toContain('@helpin-ai/vue');
    expect(prompt).toContain('HelpinPlugin');
    expect(prompt).toContain('plugins/helpin.client.ts');
  });

  it('keeps Next.js initialization on the client', () => {
    const prompt = buildWidgetInstallPrompt({
      framework: 'nextjs',
      widgetKey: 'next-key',
      host: 'http://support.example.test:8080',
      runtimeURL: 'http://assets.example.test/sdk/lib.js',
    });

    expect(prompt).toContain('Client Component');
    expect(prompt).toContain('returns null during SSR');
    expect(prompt).toContain('HelpinProvider');
  });

  it('warns HTML integrations to preserve content security policy', () => {
    const prompt = buildWidgetInstallPrompt({
      framework: 'html',
      widgetKey: 'html-key',
      host: 'http://support.example.test:8080',
      runtimeURL: 'http://assets.example.test/sdk/lib.js',
    });

    expect(prompt).toContain('Content Security Policy');
    expect(prompt).toContain('data-widget-key');
    expect(prompt).toContain('window.helpin');
  });
});
