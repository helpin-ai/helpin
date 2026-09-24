const CUSTOMERS = [
  { name: 'ContentStudio', domain: 'contentstudio.io' },
  { name: 'Replug', domain: 'replug.io' },
  { name: 'Usermaven', domain: 'usermaven.com' },
  { name: 'ContentPen', domain: 'contentpen.ai' },
  { name: 'Hyprcore', domain: 'hyprcore.ai' },
  { name: 'Hyperengage', domain: 'hyperengage.io' },
] as const;

export function CustomerLogos() {
  return (
    <div className="hero-proof">
      <span>Used by teams at</span>
      <ul aria-label="Teams using Helpin">
        {CUSTOMERS.map(({ name, domain }) => (
          <li key={domain}>
            <img src={`/favicons/${domain}.png`} alt="" width={24} height={24} loading="lazy" decoding="async" />
            <span>{name}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}
