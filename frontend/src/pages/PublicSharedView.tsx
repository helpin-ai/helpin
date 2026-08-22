import { useEffect, useState } from 'react';

import { MarkdownContent } from '@/components/pm/CodingSession/MarkdownContent';
import { dockChatService } from '@/lib/services/dockChatService';
import type { PublicSharedResource } from '@/lib/dockTypes';
import { useAuthStore } from '@/stores/authStore';

export function PublicSharedView() {
	const token = decodeURIComponent(window.location.pathname.split('/').filter(Boolean).at(-1) ?? '');
	const [resource, setResource] = useState<PublicSharedResource | null>(null);
	const [missing, setMissing] = useState(false);

	useEffect(() => {
		const robots = document.createElement('meta');
		robots.name = 'robots';
		robots.content = 'noindex, nofollow, noarchive';
		document.head.appendChild(robots);
		return () => robots.remove();
	}, []);

	useEffect(() => {
		let active = true;
		const load = async () => {
			const response = await dockChatService.getPublicSharedResource(token);
			if (!active) return;
			if (response.data) {
				setResource(response.data);
				setMissing(false);
			} else {
				setMissing(true);
			}
		};
		void load();
		const timer = window.setInterval(() => void load(), 5_000);
		return () => { active = false; window.clearInterval(timer); };
	}, [token]);

	return (
		<div className="min-h-screen bg-[#fffefa] text-[#1c1b19] dark:bg-[#242320] dark:text-[#eeeae1]">
			<header className="sticky top-0 z-10 flex h-16 items-center border-b border-[#ece9e2] bg-[#fffefa]/95 px-5 backdrop-blur dark:border-[#37352f] dark:bg-[#242320]/95">
				<a href="/" className="flex items-center gap-2 text-lg font-semibold" aria-label="Helpin home">
					<img src="/brand/helpin-icon-black.svg" alt="" className="h-7 w-7 dark:invert" />
					Helpin
				</a>
				<a href="/login" className="ml-auto rounded-full border border-[#dedad1] px-4 py-2 text-sm font-medium hover:bg-[#f4f2ee] dark:border-[#47443e] dark:hover:bg-[#302f2b]">Sign in</a>
			</header>
			<main className="mx-auto w-full max-w-3xl px-5 py-12 sm:px-8">
				{resource ? <PublicSharedContent resource={resource} /> : missing ? (
					<div className="py-24 text-center"><h1 className="text-xl font-semibold">This public link is unavailable</h1><p className="mt-2 text-sm text-muted-foreground">It may have been revoked or removed.</p></div>
				) : <div className="py-24 text-center text-sm text-muted-foreground">Loading shared conversation…</div>}
			</main>
		</div>
	);
}

export function PublicSharedContent({ resource }: { resource: PublicSharedResource }) {
	const user = useAuthStore((state) => state.user);
	const openPath = resource.dock_chat?.open_path ?? resource.agent_run?.open_path;
	if (resource.dock_chat) {
		return <section aria-label="Shared Ask conversation">
			<div className="mb-10 flex items-start gap-4"><h1 className="min-w-0 flex-1 text-2xl font-semibold tracking-tight">{resource.dock_chat.title || 'Shared Ask conversation'}</h1>{user && openPath ? <a href={openPath} className="shrink-0 rounded-full bg-[#1c1b19] px-4 py-2 text-sm font-medium text-white dark:bg-[#eeeae1] dark:text-[#1c1b19]">Open in Helpin</a> : null}</div>
			<div className="space-y-8">
				{resource.dock_chat.messages.map((message) => message.role === 'user' ? (
					<div key={message.id} className="ml-auto w-fit max-w-[85%] rounded-2xl border border-[#e4e0d8] bg-[#f7f5f1] px-4 py-2.5 text-sm dark:border-[#45423c] dark:bg-[#302f2b]">
						<MarkdownContent content={message.content} className="text-inherit" />
					</div>
				) : message.role === 'assistant' ? (
					<article key={message.id} className="min-w-0"><MarkdownContent content={message.content} className="text-[15px] leading-7" /></article>
				) : null)}
			</div>
		</section>;
	}

	const run = resource.agent_run;
	if (!run) return null;
	return <section aria-label="Shared agent run">
		<div className="mb-8 border-b border-[#ece9e2] pb-6 dark:border-[#37352f]">
			<p className="text-xs font-semibold uppercase tracking-[0.14em] text-muted-foreground">Agent run</p>
			<div className="mt-2 flex items-start gap-4"><h1 className="min-w-0 flex-1 text-2xl font-semibold tracking-tight">{run.title || 'Shared agent run'}</h1>{user && openPath ? <a href={openPath} className="shrink-0 rounded-full bg-[#1c1b19] px-4 py-2 text-sm font-medium text-white dark:bg-[#eeeae1] dark:text-[#1c1b19]">Open in Helpin</a> : null}</div>
			{run.session ? <p className="mt-2 text-sm text-muted-foreground">{run.session.status}</p> : null}
		</div>
		<div className="space-y-5">
			{run.events.map((event) => {
				const content = typeof event.payload?.content === 'string' ? event.payload.content : '';
				const toolName = typeof event.payload?.tool_name === 'string' ? event.payload.tool_name : '';
				const user = event.type.startsWith('user.');
				return <article key={event.id} className={user ? 'ml-auto w-fit max-w-[85%] rounded-2xl border bg-muted/40 px-4 py-2.5' : 'rounded-xl border border-border/70 p-4'}>
					<p className="mb-2 text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">{toolName || event.type.replaceAll('.', ' ')}</p>
					{content ? <MarkdownContent content={content} /> : <pre className="overflow-x-auto whitespace-pre-wrap text-xs text-muted-foreground">{JSON.stringify(event.payload, null, 2)}</pre>}
				</article>;
			})}
		</div>
		{run.interactions.length > 0 ? <section className="mt-10">
			<h2 className="mb-4 text-lg font-semibold">Interactions</h2>
			<div className="space-y-3">{run.interactions.map((interaction) => <article key={interaction.interaction_id || (interaction as { id?: string }).id} className="rounded-xl border border-border/70 p-4">
				<p className="text-sm font-semibold">{interaction.title || interaction.interaction_kind.replaceAll('_', ' ')}</p>
				<p className="mt-1 text-xs text-muted-foreground">{interaction.status}</p>
				{interaction.summary ? <MarkdownContent content={interaction.summary} className="mt-3" /> : null}
				<pre className="mt-3 overflow-x-auto whitespace-pre-wrap text-xs text-muted-foreground">{JSON.stringify({ request: interaction.request_payload, response: interaction.response_payload }, null, 2)}</pre>
			</article>)}</div>
		</section> : null}
		{run.artifacts.length > 0 ? <section className="mt-10">
			<h2 className="mb-4 text-lg font-semibold">Artifacts</h2>
			<div className="space-y-3">{run.artifacts.map((artifact) => <article key={artifact.id} className="rounded-xl border border-border/70 p-4">
				<p className="text-sm font-semibold">{artifact.artifact_type.replaceAll('_', ' ')}</p>
				<p className="mt-1 text-xs text-muted-foreground">{artifact.format}</p>
				{artifact.inline_content ? <MarkdownContent content={artifact.inline_content} className="mt-3" /> : null}
			</article>)}</div>
		</section> : null}
	</section>;
}
