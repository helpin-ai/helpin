import type { ComponentProps } from 'react';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import type { GitProvider } from './gitProvider';

export type { GitProvider } from './gitProvider';

/** Official GitHub Invertocat mark (simple-icons, 24×24). */
const GITHUB_PATH =
  'M12 .297c-6.63 0-12 5.373-12 12 0 5.303 3.438 9.8 8.205 11.385.6.113.82-.258.82-.577 0-.285-.01-1.04-.015-2.04-3.338.724-4.042-1.61-4.042-1.61C4.422 18.07 3.633 17.7 3.633 17.7c-1.087-.744.084-.729.084-.729 1.205.084 1.838 1.236 1.838 1.236 1.07 1.835 2.809 1.305 3.495.998.108-.776.417-1.305.76-1.605-2.665-.3-5.466-1.332-5.466-5.93 0-1.31.465-2.38 1.235-3.22-.135-.303-.54-1.523.105-3.176 0 0 1.005-.322 3.3 1.23.96-.267 1.98-.399 3-.405 1.02.006 2.04.138 3 .405 2.28-1.552 3.285-1.23 3.285-1.23.645 1.653.24 2.873.12 3.176.765.84 1.23 1.91 1.23 3.22 0 4.61-2.805 5.625-5.475 5.92.42.36.81 1.096.81 2.22 0 1.606-.015 2.896-.015 3.286 0 .315.21.69.825.57C20.565 22.092 24 17.592 24 12.297c0-6.627-5.373-12-12-12';

/**
 * Official GitLab tanuki (press kit, 25×24) as its four brand-colored facets.
 * The colors are the brand's own and stay the same in light and dark themes.
 */
const GITLAB_FACETS: Array<{ d: string; fill: string }> = [
  {
    d: 'm24.507 9.5-.034-.09L21.082.562a.896.896 0 0 0-1.694.091l-2.29 7.01H7.825L5.535.653a.898.898 0 0 0-1.694-.09L.451 9.411.416 9.5a6.297 6.297 0 0 0 2.09 7.278l.012.01.03.022 5.16 3.867 2.56 1.935 1.554 1.176a1.051 1.051 0 0 0 1.268 0l1.555-1.176 2.56-1.935 5.197-3.89.014-.01A6.297 6.297 0 0 0 24.507 9.5Z',
    fill: '#E24329',
  },
  {
    d: 'm24.507 9.5-.034-.09a11.44 11.44 0 0 0-4.56 2.051l-7.447 5.632 4.742 3.584 5.197-3.89.014-.01A6.297 6.297 0 0 0 24.507 9.5Z',
    fill: '#FC6D26',
  },
  {
    d: 'm7.707 20.677 2.56 1.935 1.555 1.176a1.051 1.051 0 0 0 1.268 0l1.555-1.176 2.56-1.935-4.743-3.584-4.755 3.584Z',
    fill: '#FCA326',
  },
  {
    d: 'M5.01 11.461a11.43 11.43 0 0 0-4.56-2.05L.416 9.5a6.297 6.297 0 0 0 2.09 7.278l.012.01.03.022 5.16 3.867 4.745-3.584-7.444-5.632Z',
    fill: '#FC6D26',
  },
];

/**
 * Brand mark for a Git provider. GitHub's Invertocat is drawn in its brand ink
 * (#181717) and inverts to white in dark mode; GitLab's tanuki keeps its full
 * brand colors in both themes. Decorative: pair it with a visible label.
 */
export function GitProviderIcon({ provider, className }: { provider: GitProvider; className?: string }) {
  if (provider === 'gitlab') {
    return (
      <svg
        viewBox="0 0 25 24"
        aria-hidden="true"
        focusable="false"
        data-git-provider-icon="gitlab"
        className={cn('h-4 w-4 shrink-0', className)}
      >
        {GITLAB_FACETS.map((facet) => <path key={facet.d} d={facet.d} fill={facet.fill} />)}
      </svg>
    );
  }
  return (
    <svg
      viewBox="0 0 24 24"
      fill="currentColor"
      aria-hidden="true"
      focusable="false"
      data-git-provider-icon="github"
      className={cn('h-4 w-4 shrink-0 text-[#181717] dark:text-white', className)}
    >
      <path d={GITHUB_PATH} />
    </svg>
  );
}

export type GitProviderButtonProps = Omit<ComponentProps<typeof Button>, 'variant' | 'size'> & {
  provider: GitProvider;
};

/**
 * Quiet outlined action carrying a provider's brand mark, for connect/install
 * actions. It stays secondary to the screen's dark primary action: hairline
 * border, surface background and a 7px radius.
 */
export function GitProviderButton({ provider, className, children, ...props }: GitProviderButtonProps) {
  return (
    <Button
      type="button"
      variant="outline"
      size="sm"
      data-git-provider={provider}
      className={cn(
        'gap-2 rounded-[7px] border-quiet-divider-strong bg-quiet-surface text-quiet-text-primary hover:bg-quiet-hover hover:text-quiet-text-primary dark:bg-quiet-surface dark:hover:bg-quiet-hover',
        className,
      )}
      {...props}
    >
      <GitProviderIcon provider={provider} />
      {children}
    </Button>
  );
}
