import { useEffect, useState, type ComponentProps } from 'react';
import { AvatarImage } from './avatar';
import { getGravatarUrl } from '@/lib/gravatar';

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
  const useGravatar = !src || primaryFailed;

  useEffect(() => {
    if (!useGravatar || !email) return;
    let active = true;
    void getGravatarUrl(email).then((url) => {
      if (active) setGravatarUrl(url);
    });
    return () => { active = false; };
  }, [email, useGravatar]);

  return (
    <AvatarImage
      {...props}
      src={useGravatar ? gravatarUrl : src}
      referrerPolicy="no-referrer"
      onLoadingStatusChange={(status) => {
        if (status === 'error' && !useGravatar) setPrimaryFailed(true);
        onLoadingStatusChange?.(status);
      }}
    />
  );
}
