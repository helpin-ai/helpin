import type { ReactNode } from "react";
import { MarkdownContent } from "@/components/pm/CodingSession/MarkdownContent";
import { AskAgentAvatar } from "@/components/agents/AskAgentAvatar";
import { DisclosureChevron } from "@/components/agents/transcript/DisclosureChevron";
import { useWorkspaceStore } from "@/stores/workspaceStore";
import { ArrowDown01Icon, ArrowUpRight01Icon, BubbleChatIcon, File01Icon } from "@/lib/icons";
import { timeAgo } from "@/lib/utils";
import { helpinReferenceRoute, parseHelpinReference } from "@/lib/helpinReferences";
import type { DockContextMessage as ContextMessage, DockEntityReference } from "@/lib/dockTypes";

function ContextDisclosure({ label, count, children }: {
  label: string;
  count?: number;
  children: ReactNode;
}) {
  return (
    <details className="group/findings min-w-0">
      <summary className="flex min-h-9 cursor-pointer list-none items-center gap-2 text-xs text-quiet-text-secondary transition-colors hover:text-quiet-text-primary focus-visible:outline-2 focus-visible:outline-ring">
        <DisclosureChevron open={false} className="h-3 w-3 group-open/findings:rotate-90" />
        <span className="font-medium">{label}</span>
        {count !== undefined && <span className="ml-auto tabular-nums text-quiet-text-tertiary">{count}</span>}
      </summary>
      <div className="pb-3 pl-5 pt-1">{children}</div>
    </details>
  );
}

/** A saved brief appears before the real chat turns, without claiming a new AI run. */
export function DockContextMessage({
  message,
  compact,
}: {
  message: ContextMessage;
  compact: boolean;
}) {
  const slug = useWorkspaceStore((state) => state.currentWorkspace?.slug);
  const hasSources = Boolean(message.sources || message.source_summary);
  const sourceLink = (
    reference: DockEntityReference,
    full = false,
    title = reference.display_title,
  ) => {
    const resource = parseHelpinReference(`helpin://${reference.entity_type}/${encodeURIComponent(reference.entity_id)}`);
    const href = slug && resource ? helpinReferenceRoute(resource, slug) : null;
    const Icon = reference.entity_type === "support_conversation" ? BubbleChatIcon : File01Icon;
    if (!href) return <span key={`${reference.entity_type}:${reference.entity_id}`}>{title}</span>;
    return (
      <a
        key={`${reference.entity_type}:${reference.entity_id}`}
        className={`inline-flex min-w-0 max-w-full items-center gap-1.5 text-quiet-text-secondary underline decoration-quiet-divider-strong underline-offset-4 hover:text-quiet-accent hover:decoration-current focus-visible:outline-2 focus-visible:outline-ring ${full ? "" : "sm:max-w-[280px]"}`}
        href={href}
        title={reference.display_title}
        aria-label={full && title !== reference.display_title ? `Open ${title}: ${reference.display_title}` : undefined}
        target="_blank"
        rel="noopener noreferrer"
      >
        {!full && <Icon className="h-3.5 w-3.5 shrink-0 text-quiet-text-tertiary" aria-hidden="true" />}
        <span className={full ? "break-words" : "truncate"}>{title}</span>
        {full && <ArrowUpRight01Icon className="h-3 w-3 shrink-0" aria-hidden="true" />}
      </a>
    );
  };
  const body = (
    <div className="space-y-3 pb-2 pl-7 text-sm leading-6">
      <MarkdownContent content={message.content} className="text-inherit" />
      {slug && Boolean(message.references?.length) && (
        <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs">
          {message.references?.map((reference) => sourceLink(reference))}
        </div>
      )}
      {(hasSources || Boolean(message.details?.length)) && <div className="divide-y divide-quiet-divider-strong border-y border-quiet-divider-strong">
        {hasSources && (
          <ContextDisclosure label="Sources" count={message.sources?.length}>
              {message.source_summary && (
                <p className="mb-3 text-xs leading-5 text-quiet-text-tertiary">{message.source_summary}</p>
              )}
              <ul className="divide-y divide-quiet-divider-strong" aria-label="Source excerpts and guidance">
                {message.sources?.map((source) => {
                  const reference = source.reference;
                  const Icon = reference?.entity_type === "support_conversation" ? BubbleChatIcon : File01Icon;
                  return (
                    <li key={source.id} className="py-3 first:pt-0 last:pb-0">
                      <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-xs">
                        {reference && <Icon className="h-3.5 w-3.5 shrink-0 text-quiet-text-tertiary" aria-hidden="true" />}
                        {reference && sourceLink(reference, true, reference.entity_type === "support_conversation" ? "Conversation" : reference.display_title)}
                        <span className="text-quiet-text-tertiary">{source.label}</span>
                        {source.captured_at && (
                          <time className="ml-auto whitespace-nowrap text-quiet-text-tertiary" dateTime={source.captured_at} title={new Date(source.captured_at).toLocaleString()}>
                            {timeAgo(source.captured_at)}
                          </time>
                        )}
                      </div>
                      {source.content && (
                        <div className={`mt-2 ${reference ? "pl-5" : ""}`}>
                          <MarkdownContent content={source.content} className="text-sm leading-5 text-quiet-text-secondary" />
                        </div>
                      )}
                    </li>
                  );
                })}
              </ul>
          </ContextDisclosure>
        )}
        {message.details?.map((detail) => (
          <ContextDisclosure key={detail.label} label={detail.label}>
            <MarkdownContent content={detail.content} className="text-sm leading-6 text-quiet-text-secondary [&_p]:mb-2 [&_strong]:font-medium [&_strong]:text-quiet-text-primary" />
          </ContextDisclosure>
        ))}
      </div>}
    </div>
  );
  const label = (
    <span className="flex items-center gap-2 py-2 text-xs text-muted-foreground">
      <AskAgentAvatar state="idle" size={20} />
      <span>Saved findings</span>
      <time
        dateTime={message.captured_at}
        title={new Date(message.captured_at).toLocaleString()}
        className="ml-auto"
      >
        {timeAgo(message.captured_at)}
      </time>
    </span>
  );
  return compact ? (
    <details
      data-agent-context-message
      className="group border-b border-border/40 pb-2"
    >
      <summary className="flex cursor-pointer list-none items-center gap-1">
        <ArrowDown01Icon className="h-3 w-3 shrink-0 -rotate-90 text-muted-foreground transition-transform group-open:rotate-0" />
        <span className="min-w-0 flex-1">{label}</span>
      </summary>
      {body}
    </details>
  ) : (
    <div data-agent-context-message>
      {label}
      {body}
    </div>
  );
}
