import { useEffect, useState, type ComponentProps } from 'react';
import { AvatarImage } from './avatar';
import { getGravatarUrl } from '@/lib/gravatar';
import { cn } from '@/lib/utils';

type ContactAvatarImageProps = ComponentProps<typeof AvatarImage> & {
  email?: string | null;
};

/** Use inside Avatar with the surface's existing AvatarFallback. */
export function ContactAvatarImage({ email, src, ...props }: ContactAvatarImageProps) {
  const normalizedEmail = email?.trim().toLowerCase() || '';
  const primarySrc = src?.trim() || undefined;
  return <ContactAvatarImageSource key={JSON.stringify([normalizedEmail, primarySrc])} email={normalizedEmail} src={primarySrc} {...props} />;
}

function ContactAvatarImageSource({ email, src, onLoadingStatusChange, ...props }: ContactAvatarImageProps) {
  const [primaryFailed, setPrimaryFailed] = useState(false);
  const [gravatarUrl, setGravatarUrl] = useState<string>();
  const [gravatarFailed, setGravatarFailed] = useState(false);
  const useGravatar = !src || primaryFailed;

  useEffect(() => {
    if (!useGravatar || !email) return;
    let active = true;
    void getGravatarUrl(email).then((url) => {
      if (active) setGravatarUrl(url);
    });
    return () => { active = false; };
  }, [email, useGravatar]);

  if (useGravatar) {
    if (!gravatarUrl || gravatarFailed) return null;

    // Keep Radix's initials fallback mounted beneath the image. Gravatar returns
    // a transparent PNG for missing photos, so no error request is necessary.
    return (
      <img
        {...props}
        data-slot="avatar-image"
        className={cn('absolute inset-0 aspect-square size-full rounded-full object-cover', props.className)}
        src={gravatarUrl}
        referrerPolicy="no-referrer"
        onLoad={(event) => {
          onLoadingStatusChange?.('loaded');
          props.onLoad?.(event);
        }}
        onError={(event) => {
          setGravatarFailed(true);
          onLoadingStatusChange?.('error');
          props.onError?.(event);
        }}
      />
    );
  }

  return (
    <AvatarImage
      {...props}
      src={src}
      referrerPolicy="no-referrer"
      onLoadingStatusChange={(status) => {
        if (status === 'error' && !useGravatar) setPrimaryFailed(true);
        onLoadingStatusChange?.(status);
      }}
    />
  );
}
