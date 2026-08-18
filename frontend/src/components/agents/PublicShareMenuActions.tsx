import { useEffect, useState } from 'react';
import { toast } from 'sonner';

import { DropdownMenuItem } from '@/components/ui/dropdown-menu';
import { Link01Icon } from '@/lib/icons';
import type { PublicShareLink, PublicShareResourceType } from '@/lib/dockTypes';
import { dockChatService } from '@/lib/services/dockChatService';

export function PublicShareMenuActions({ workspaceId, resourceType, resourceId }: {
	workspaceId: string;
	resourceType: PublicShareResourceType;
	resourceId: string;
}) {
	const [link, setLink] = useState<PublicShareLink | null>(null);
	const [loading, setLoading] = useState(true);
	const [busy, setBusy] = useState(false);

	useEffect(() => {
		let active = true;
		setLoading(true);
		void dockChatService.getPublicShare(workspaceId, resourceType, resourceId).then((result) => {
			if (!active) return;
			setLink(result.data ?? null);
			setLoading(false);
		});
		return () => { active = false; };
	}, [resourceId, resourceType, workspaceId]);

	const copy = async (created: boolean) => {
		if (busy) return;
		setBusy(true);
		try {
			const current = link ?? (await dockChatService.createPublicShare(workspaceId, resourceType, resourceId)).data;
			if (!current) throw new Error('Unable to create public link');
			await navigator.clipboard.writeText(current.url);
			setLink(current);
			toast.success(created ? 'Public link created and copied' : 'Public link copied');
		} catch (error) {
			toast.error(error instanceof Error ? error.message : 'Unable to copy public link');
		} finally {
			setBusy(false);
		}
	};

	const revoke = async () => {
		if (busy) return;
		setBusy(true);
		const result = await dockChatService.revokePublicShare(workspaceId, resourceType, resourceId);
		setBusy(false);
		if (result.error) {
			toast.error(result.error);
			return;
		}
		setLink(null);
		toast.success('Public sharing stopped');
	};

	if (loading) return <DropdownMenuItem disabled>Checking public link…</DropdownMenuItem>;
	if (!link) return <DropdownMenuItem disabled={busy} onSelect={() => void copy(true)}><Link01Icon className="h-3.5 w-3.5" />Share publicly</DropdownMenuItem>;
	return <>
		<DropdownMenuItem disabled={busy} onSelect={() => void copy(false)}><Link01Icon className="h-3.5 w-3.5" />Copy public link</DropdownMenuItem>
		<DropdownMenuItem disabled={busy} className="text-destructive" onSelect={() => void revoke()}>Stop public sharing</DropdownMenuItem>
	</>;
}
