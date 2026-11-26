import { useEffect, useRef, useState } from 'react';
import { useMessageSlackSource } from '@/hooks/queries/useSupport';
import { ArrowUpRight01Icon, SlackIcon } from '@/lib/icons';

export function SlackMessageSource({ workspaceId, messageId }: { workspaceId: string; messageId: string }) {
  const footer = useRef<HTMLDivElement>(null);
  const [visible, setVisible] = useState(() => typeof IntersectionObserver === 'undefined');

  useEffect(() => {
    if (visible || !footer.current) return;
    const observer = new IntersectionObserver(entries => {
      if (entries.some(entry => entry.isIntersecting)) setVisible(true);
    });
    observer.observe(footer.current);
    return () => observer.disconnect();
  }, [visible]);

  const { data } = useMessageSlackSource(workspaceId, messageId, visible);
  const attribution = <><SlackIcon className="size-3" />Received from Slack</>;
  return (
    <div ref={footer} className="mb-0.5 flex text-[11px] text-muted-foreground">
      {data?.permalink ? (
        <a href={data.permalink} target="_blank" rel="noopener noreferrer"
          title="Open original message in Slack" aria-label="Received from Slack, open original message in a new tab"
          className="inline-flex items-center gap-1 rounded-sm transition-colors hover:text-foreground hover:underline focus-visible:outline-2 focus-visible:outline-ring">
          {attribution}<ArrowUpRight01Icon aria-hidden="true" className="size-3" />
        </a>
      ) : <span className="inline-flex items-center gap-1">{attribution}</span>}
    </div>
  );
}

export function slackMessageOrigin(href: unknown): string | undefined {
  const match = typeof href === 'string' && /^(https:\/\/(?:[a-z0-9-]+\.)*slack(?:-gov)?\.com)\/archives\//i.exec(href);
  return match ? match[1] : undefined;
}

export function SlackSourceLabel({ href, label = 'via Slack' }: { href?: unknown; label?: string }) {
  const content = <><SlackIcon className="size-3 shrink-0" />{label}</>
  const style = 'inline-flex items-center gap-1 text-[11px] text-muted-foreground'
  return typeof href === 'string' && slackMessageOrigin(href)
    ? <a className={style + ' hover:text-foreground hover:underline'} href={href} target="_blank" rel="noopener noreferrer" title="Open in Slack">{content}<ArrowUpRight01Icon className="size-3" /></a>
    : <span className={style}>{content}</span>
}
