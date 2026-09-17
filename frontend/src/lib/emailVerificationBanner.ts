type EmailVerificationBannerVisibilityInput = {
  emailVerified?: boolean;
  verificationRequired?: boolean;
  pathname: string;
  search?: Record<string, unknown>;
};

export function shouldShowEmailVerificationBanner({
  emailVerified,
  verificationRequired = true,
  pathname,
  search,
}: EmailVerificationBannerVisibilityInput) {
  if (!verificationRequired || emailVerified !== false) {
    return false;
  }

  const createParam = search?.create;
  const isCreatingWorkspace = createParam === true || createParam === 'true';
  if (pathname === '/onboarding' || (pathname === '/workspaces' && isCreatingWorkspace)) {
    return false;
  }

  return true;
}
