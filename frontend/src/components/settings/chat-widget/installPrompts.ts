export type WidgetInstallFramework = 'html' | 'react' | 'vue' | 'nextjs';

type WidgetInstallPromptOptions = {
  framework: WidgetInstallFramework;
  widgetKey: string;
  host: string;
};

const FRAMEWORK_INSTRUCTIONS: Record<WidgetInstallFramework, string> = {
  html: `This is the HTML / JavaScript integration.
- Do not install an npm package unless the project already bundles its scripts.
- Define window.helpin as a command queue before loading https://cdn.helpin.ai/lib.js.
- Load the script once, preferably before the closing </body> tag, with data-widget-key and data-host attributes.
- Keep the existing Content Security Policy in mind. If it blocks the widget, report the exact directives that need the Helpin origins rather than weakening the policy.`,
  react: `This is the React integration.
- Install @helpin-ai/react and @helpin-ai/sdk-js with the package manager already used by the project.
- Create one Helpin client and wrap the application root with HelpinProvider.
- Use useHelpin() only below the provider.
- Preserve React Strict Mode compatibility and avoid initializing a new client on every render.`,
  vue: `This is the Vue integration. It requires Vue 3.3 or newer.
- Install @helpin-ai/vue and @helpin-ai/sdk-js with the package manager already used by the project.
- For Vue, create one client in the application bootstrap and install HelpinPlugin with app.use(HelpinPlugin, { client }).
- For Nuxt, use a client-only plugin such as plugins/helpin.client.ts.
- Use useHelpin() only after the plugin is installed. Vue Router is optional.`,
  nextjs: `This is the Next.js integration.
- Install @helpin-ai/nextjs and @helpin-ai/sdk-js with the package manager already used by the project.
- Initialize Helpin in a Client Component and wrap the application with HelpinProvider.
- createClient() is browser-only and returns null during SSR; do not initialize it in a Server Component.
- Reuse one client instance and preserve the project's App Router or Pages Router conventions.`,
};

export function buildWidgetInstallPrompt({
  framework,
  widgetKey,
  host,
}: WidgetInstallPromptOptions): string {
  return `Integrate the Helpin support widget into this application.

First inspect the repository, identify its framework, package manager, application bootstrap, authentication flow, routing, and test commands. Follow the existing project conventions and make the implementation directly; do not only describe it.

Helpin configuration:
- Integration: ${framework === 'html' ? 'HTML / JavaScript' : framework === 'nextjs' ? 'Next.js' : framework === 'react' ? 'React' : 'Vue'}
- Public widget key: ${widgetKey}
- Helpin host: ${host}
- Hosted widget runtime: https://cdn.helpin.ai/lib.js

${FRAMEWORK_INSTRUCTIONS[framework]}

Implementation requirements:
1. Add the widget exactly once in the correct application entry point.
2. Use the widget key and host above. The widget key is public, but do not expose private API keys, access tokens, session tokens, or unrelated environment variables.
3. After authentication is ready, identify signed-in users with id() using the application's real user model. Include stable user ID and email; include first name, last name, and company only when available. Do not send undefined fields.
4. Preserve existing authentication, analytics, routing, error handling, and rendering behavior.
5. Make widget controls available where the product needs them. Supported controls include open(), close(), toggle(), show(), hide(), openMessages(), openNewMessage(content?), openConversation(conversationId), openArticle(articleKey, options?), and shutdown().
6. For openArticle(), pass the final article key from a Helpin URL, not a legacy provider's article ID or the complete URL.
7. Add or update focused tests when this repository tests integrations.
8. Run the relevant typecheck, tests, lint, and production build. Fix issues caused by the integration.

Acceptance checks:
- The widget runtime is loaded once.
- The launcher appears and open()/close() work.
- Signed-in identity is supplied only after authentication is known.
- Client-side navigation and SSR/hydration continue to work where applicable.
- No private credentials are added to client code.
- The production build passes.

When finished, report the files changed, where Helpin is initialized, how identity is synchronized, the verification commands and results, and any missing application context that still requires review.`;
}
