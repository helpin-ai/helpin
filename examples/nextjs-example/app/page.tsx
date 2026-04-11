'use client';

import { useHelpin } from '@helpin-ai/nextjs';

export default function Home() {
  const helpin = useHelpin();

  const handleIdentify = async () => {
    await helpin.id({
      id: 'user-123',
      email: 'user@example.com',
      first_name: 'Example',
      last_name: 'User',
      company: {
        id: 'company-123',
        name: 'Acme Corp',
        created_at: '2024-01-15T00:00:00Z',
      },
    });
  };

  const handleLead = () => {
    helpin.lead({
      email: 'lead@example.com',
      first_name: 'New',
      last_name: 'Lead',
      company: {
        id: 'company-123',
        name: 'Acme Corp',
        created_at: '2024-01-15T00:00:00Z',
      },
    });
  };

  const handleTrack = () => {
    helpin.track('button_clicked', {
      button: 'cta',
      page: 'home',
    });
  };

  return (
    <main style={{ padding: '2rem', fontFamily: 'sans-serif' }}>
      <h1>Helpin Next.js SDK Example</h1>
      <p>Click the buttons below to test SDK methods.</p>

      <div style={{ display: 'flex', gap: '1rem', marginTop: '1rem' }}>
        <button onClick={handleIdentify}>Identify User</button>
        <button onClick={handleLead}>Submit Lead</button>
        <button onClick={handleTrack}>Track Event</button>
      </div>
    </main>
  );
}
