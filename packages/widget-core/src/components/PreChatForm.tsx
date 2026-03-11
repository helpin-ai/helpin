import { FunctionComponent } from 'preact';
import { useState } from 'preact/hooks';

interface PreChatFormProps {
  requireEmail?: boolean;
  requireName?: boolean;
  welcomeMessage?: string;
  onSubmit: (data: { name: string; email: string }) => void;
}

export const PreChatForm: FunctionComponent<PreChatFormProps> = ({
  requireEmail = true,
  requireName = true,
  welcomeMessage = 'Hi! How can we help you today?',
  onSubmit,
}) => {
  const [name, setName] = useState('');
  const [email, setEmail] = useState('');
  const [step, setStep] = useState<'email' | 'name' | 'done'>('email');

  const handleEmailSubmit = (e: Event) => {
    e.preventDefault();
    if (email.trim() && requireName) {
      setStep('name');
    } else {
      onSubmit({ name: '', email: email.trim() });
      setStep('done');
    }
  };

  const handleNameSubmit = (e: Event) => {
    e.preventDefault();
    onSubmit({ name: name.trim(), email: email.trim() });
    setStep('done');
  };

  if (step === 'done') return null;

  return (
    <div className="helpin-pre-chat-form">
      <div className="helpin-pre-chat-welcome">{welcomeMessage}</div>
      
      {step === 'email' && (
        <form onSubmit={handleEmailSubmit}>
          <label className="helpin-sr-only" htmlFor="helpin-email-input">Email address</label>
          <input
            id="helpin-email-input"
            type="email"
            className="helpin-input"
            placeholder="Enter your email"
            value={email}
            onInput={(e) => setEmail((e.target as HTMLInputElement).value)}
            required={requireEmail}
            autoFocus
          />
          <button type="submit" className="helpin-btn-primary">
            Continue
          </button>
        </form>
      )}

      {step === 'name' && (
        <form onSubmit={handleNameSubmit}>
          <label className="helpin-sr-only" htmlFor="helpin-name-input">Your name</label>
          <input
            id="helpin-name-input"
            type="text"
            className="helpin-input"
            placeholder="Enter your name"
            value={name}
            onInput={(e) => setName((e.target as HTMLInputElement).value)}
            required={requireName}
            autoFocus
          />
          <button type="submit" className="helpin-btn-primary">
            Start Chat
          </button>
        </form>
      )}
    </div>
  );
};
