import { FunctionComponent } from 'preact';
import { useState, useRef, useEffect } from 'preact/hooks';
import type { WidgetConfig } from '../types';
import { MailIcon, PhoneIcon } from './icons';

interface PreChatFormProps {
  config: WidgetConfig;
  onSubmit: (data: { phone: string; email: string }) => void;
}

export const PreChatForm: FunctionComponent<PreChatFormProps> = ({
  config,
  onSubmit,
}) => {
  const [email, setEmail] = useState('');
  const [phone, setPhone] = useState('');
  const [step, setStep] = useState<'email' | 'phone' | 'done'>('email');
  const formRef = useRef<HTMLDivElement>(null);
  const brandColor = config.branding?.primaryColor || '#6366f1';

  useEffect(() => {
    if (typeof formRef.current?.scrollIntoView === 'function') {
      formRef.current.scrollIntoView({ behavior: 'smooth', block: 'end' });
    }
  }, [step]);

  const requirePhone = config.features?.requirePhone === true;
  const emailRequired = config.features?.forceIdentify === true;
  const isOnline = config.availability?.isOnline !== false;
  const replyTimeText = config.availability?.replyTimeText || 'We typically reply in a few minutes';
  const punctuatedReplyTimeText = /[.!?]$/.test(replyTimeText)
    ? replyTimeText
    : `${replyTimeText}.`;
  const emailDescription = isOnline
    ? `Where should we send their reply? ${punctuatedReplyTimeText}`
    : 'Our team is currently offline. Leave your email and we’ll notify you when someone replies.';
  const emailSubmitLabel = requirePhone
    ? 'Continue'
    : isOnline
      ? 'Continue to human support'
      : 'Send message and notify me';

  const handleEmailSubmit = (e: Event) => {
    e.preventDefault();
    if (!email.trim()) return;
    if (requirePhone) {
      setStep('phone');
    } else {
      onSubmit({ phone: '', email: email.trim() });
      setStep('done');
    }
  };

  const handleSkipEmail = () => {
    if (requirePhone) {
      setStep('phone');
    } else {
      onSubmit({ phone: '', email: '' });
      setStep('done');
    }
  };

  const handlePhoneSubmit = (e: Event) => {
    e.preventDefault();
    onSubmit({ phone: phone.trim(), email: email.trim() });
    setStep('done');
  };

  if (step === 'done') return null;

  return (
    <div className="helpin-human-contact-card" ref={formRef} role="region" aria-label="Talk to a person">
      {step === 'email' && (
        <>
          <div className="helpin-human-contact-card__heading">
            <span className="helpin-human-contact-card__title">Talk to a person</span>
            <span className="helpin-human-contact-card__description">{emailDescription}</span>
          </div>
          <form onSubmit={handleEmailSubmit} className="helpin-human-contact-form">
            <label className="helpin-sr-only" htmlFor="helpin-prechat-email">Email address</label>
            <div className="helpin-human-contact-input-wrap">
              <MailIcon size={16} class="helpin-human-contact-icon" />
              <input
                id="helpin-prechat-email"
                type="email"
                className="helpin-human-contact-input"
                placeholder="you@example.com"
                value={email}
                onInput={(e) => setEmail((e.target as HTMLInputElement).value)}
                required
                autoFocus
              />
            </div>
            <button
              type="submit"
              className="helpin-human-contact-primary"
              style={{ backgroundColor: brandColor }}
              disabled={!email.trim()}
            >
              {emailSubmitLabel}
            </button>
          </form>
          {!emailRequired && (
            <button
              type="button"
              className="helpin-human-contact-secondary"
              onClick={handleSkipEmail}
            >
              Continue without email
            </button>
          )}
          <span className="helpin-human-contact-card__privacy">
            Your email is used only for this conversation.
          </span>
        </>
      )}
      {step === 'phone' && (
        <>
          <div className="helpin-human-contact-card__heading">
            <span className="helpin-human-contact-card__title">One more detail</span>
            <span className="helpin-human-contact-card__description">
              What phone number should our team use?
            </span>
          </div>
          <form onSubmit={handlePhoneSubmit} className="helpin-human-contact-form">
            <label className="helpin-sr-only" htmlFor="helpin-prechat-phone">Your phone number</label>
            <div className="helpin-human-contact-input-wrap">
              <PhoneIcon size={16} class="helpin-human-contact-icon" />
              <input
                id="helpin-prechat-phone"
                type="tel"
                className="helpin-human-contact-input"
                placeholder="+1 (555) 000-0000"
                value={phone}
                onInput={(e) => setPhone((e.target as HTMLInputElement).value)}
                autoFocus
              />
            </div>
            <button
              type="submit"
              className="helpin-human-contact-primary"
              style={{ backgroundColor: brandColor }}
            >
              Continue to human support
            </button>
          </form>
          <button
            type="button"
            className="helpin-human-contact-secondary"
            onClick={() => {
              onSubmit({ phone: '', email: email.trim() });
              setStep('done');
            }}
          >
            Continue without phone
          </button>
        </>
      )}
    </div>
  );
};
