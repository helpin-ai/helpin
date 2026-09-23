import { createFileRoute, redirect } from '@tanstack/react-router';
import { SetupSuccessPage } from '@/pages/SetupSuccessPage';
import { isSetupSuccessEnabled } from '@/lib/featureFlags';
import { ensureAuthConfiguration } from '@/stores/authStore';

export const Route = createFileRoute('/_authenticated/w/$slug/setup')({
	beforeLoad: async ({ params }) => {
		// Wait for the API's configuration so a direct visit is not redirected
		// before /auth/config has loaded.
		const configuration = await ensureAuthConfiguration();
		if (!isSetupSuccessEnabled(configuration)) {
			throw redirect({ to: '/w/$slug/pm/my-work', params: { slug: params.slug } });
		}
	},
  component: SetupSuccessPage,
});
