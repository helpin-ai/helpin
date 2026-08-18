import { createFileRoute, redirect } from '@tanstack/react-router';
import { SetupSuccessPage } from '@/pages/SetupSuccessPage';
import { isSetupSuccessEnabled } from '@/lib/featureFlags';

export const Route = createFileRoute('/_authenticated/w/$slug/setup')({
	beforeLoad: ({ params }) => {
		if (!isSetupSuccessEnabled()) {
			throw redirect({ to: '/w/$slug/pm/my-work', params: { slug: params.slug } });
		}
	},
  component: SetupSuccessPage,
});
