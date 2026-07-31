type EmailVerificationBannerVisibilityInput = {
  emailVerified?: boolean;
  pathname: string;
  search?: Record<string, unknown>;
};

export function shouldShowEmailVerificationBanner({
  emailVerified,
  pathname,
  search,
}: EmailVerificationBannerVisibilityInput) {
  if (emailVerified !== false) {
    return false;
  }

  const createParam = search?.create;
  const isCreatingWorkspace = createParam === true || createParam === 'true';
  if (pathname === '/onboarding' || (pathname === '/workspaces' && isCreatingWorkspace)) {
    return false;
  }

  return true;
}
