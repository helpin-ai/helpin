import { CloudServerIcon } from "@/lib/icons";
import { cn } from "@/lib/utils";

const OPENAI_PATH =
  "M22.282 9.821a5.985 5.985 0 0 0-.516-4.91 6.046 6.046 0 0 0-6.51-2.9A6.065 6.065 0 0 0 4.981 4.18a5.985 5.985 0 0 0-3.998 2.9 6.046 6.046 0 0 0 .743 7.097 5.98 5.98 0 0 0 .51 4.911 6.051 6.051 0 0 0 6.515 2.9A5.985 5.985 0 0 0 13.26 24a6.056 6.056 0 0 0 5.772-4.206 5.99 5.99 0 0 0 3.997-2.9 6.056 6.056 0 0 0-.747-7.073zM13.26 22.43a4.476 4.476 0 0 1-2.876-1.04l.141-.081 4.779-2.758a.795.795 0 0 0 .392-.681v-6.737l2.02 1.168a.071.071 0 0 1 .038.052v5.583a4.504 4.504 0 0 1-4.494 4.494zM3.6 18.304a4.47 4.47 0 0 1-.535-3.014l.142.085 4.783 2.759a.771.771 0 0 0 .78 0l5.843-3.369v2.332a.08.08 0 0 1-.033.062L9.74 19.95a4.5 4.5 0 0 1-6.14-1.646zM2.34 7.896a4.485 4.485 0 0 1 2.366-1.973V11.6a.766.766 0 0 0 .388.677l5.815 3.355-2.02 1.168a.076.076 0 0 1-.071 0l-4.83-2.786A4.504 4.504 0 0 1 2.34 7.872zm16.597 3.855-5.833-3.387L15.119 7.2a.076.076 0 0 1 .071 0l4.83 2.791a4.494 4.494 0 0 1-.676 8.105v-5.678a.79.79 0 0 0-.407-.667zm2.01-3.023-.141-.085-4.774-2.782a.776.776 0 0 0-.785 0L9.409 9.23V6.897a.066.066 0 0 1 .028-.061l4.83-2.787a4.5 4.5 0 0 1 6.68 4.66zm-12.64 4.135-2.02-1.164a.08.08 0 0 1-.038-.057V6.075a4.5 4.5 0 0 1 7.375-3.453l-.142.08L8.704 5.46a.795.795 0 0 0-.393.681zm1.097-2.365 2.602-1.5 2.607 1.5v2.999l-2.597 1.5-2.607-1.5z";

const ANTHROPIC_PATH =
  "M13.827 3.52h3.603L24 20.48h-3.603zm-7.258 0h3.767L16.906 20.48h-3.674l-1.343-3.461H5.017l-1.344 3.46H0zm4.132 10.409L8.453 7.687 6.205 13.93z";

const OPENROUTER_PATH =
  "M16.778 1.5v3.096c-1.51.007-2.328.19-3.012.54-.688.35-1.303.897-2.03 1.737l-.72.83-.36.415c-.913 1.05-1.723 1.94-2.6 2.587a6.3 6.3 0 0 1-1.34.76c-.83.33-1.72.48-2.87.51H0v-3.1h3.85c.9-.02 1.47-.13 1.93-.32.31-.13.58-.3.87-.53.63-.48 1.29-1.2 2.15-2.19l.36-.42.74-.85c.83-.95 1.66-1.7 2.68-2.22C13.7 1.7 14.94 1.5 16.78 1.5zM24 6.21l-6.2 3.58V2.63zM3.85 10.72c.9.02 1.47.13 1.93.32.31.13.58.3.87.53.63.48 1.29 1.2 2.15 2.19l.36.42.74.85c.83.95 1.66 1.7 2.68 2.22 1.12.57 2.36.77 4.2.77v-3.1c-1.51-.01-2.33-.19-3.01-.54-.69-.35-1.3-.9-2.03-1.74l-.72-.83-.36-.41c-.91-1.05-1.72-1.94-2.6-2.59a6.3 6.3 0 0 0-1.34-.76c-.83-.33-1.72-.48-2.87-.51H0v3.1zM24 17.79l-6.2-3.58v7.16z";

const PROVIDER_PATHS: Record<string, string> = {
  openai: OPENAI_PATH,
  // ChatGPT is OpenAI's consumer plan, so it carries the same mark.
  openai_chatgpt: OPENAI_PATH,
  anthropic: ANTHROPIC_PATH,
  openrouter: OPENROUTER_PATH,
};

/**
 * Brand mark for an AI provider. The OpenAI, Anthropic and OpenRouter marks are
 * single-color, so they are drawn at full-contrast primary ink (near-black in
 * light mode, near-white in dark) rather than a muted tone, matching how the
 * brands present them. Providers without a mark, including self-hosted
 * endpoints, fall back to a server glyph.
 */
export function ProviderIcon({
  provider,
  className,
}: {
  provider: string;
  className?: string;
}) {
  const path = PROVIDER_PATHS[provider];
  if (!path)
    return (
      <CloudServerIcon className={cn("h-4 w-4", className)} data-ai-provider-icon={provider} />
    );
  return (
    <svg
      viewBox="0 0 24 24"
      fill="currentColor"
      aria-hidden="true"
      data-ai-provider-icon={provider}
      className={cn("h-4 w-4 text-quiet-text-primary", className)}
    >
      <path d={path} />
    </svg>
  );
}

export function ProviderIconTile({
  provider,
  className,
}: {
  provider: string;
  className?: string;
}) {
  return (
    <span
      className={cn(
        "flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border border-border/70 bg-muted/30",
        className,
      )}
    >
      <ProviderIcon provider={provider} className="h-4 w-4" />
    </span>
  );
}
