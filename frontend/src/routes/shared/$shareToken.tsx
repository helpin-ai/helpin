import { createFileRoute } from '@tanstack/react-router';

import { PublicSharedView } from '@/pages/PublicSharedView';

export const Route = createFileRoute('/shared/$shareToken')({
	component: PublicSharedView,
});
