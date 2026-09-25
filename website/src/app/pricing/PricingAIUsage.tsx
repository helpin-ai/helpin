import { Cloud, Gauge, KeyRound } from 'lucide-react';
import { AI_PRICING } from '@/generated/aiPricing';
import { SectionHead } from '../(site)/_components/ui';
import { AI_ALLOWANCE } from './pricing-data';
import { DOCS } from '../(site)/_components/docsLinks';

const PROVIDERS = [
  { id: 'openai', name: 'OpenAI' },
  { id: 'anthropic', name: 'Anthropic' },
  { id: 'openrouter', name: 'OpenRouter' },
];

const usd = (microusd: number) => new Intl.NumberFormat('en-US', {
  style: 'currency', currency: 'USD', maximumFractionDigits: 4,
}).format(microusd / 1_000_000);

const modelsFor = (tier: string) => [...new Set(AI_PRICING.models.filter(model => model.tier === tier && model.enabled).map(model => model.label))];

export function AIUsage() {
  return (
    <section id="ai-usage" className="pricing-ai-section">
      <div className="wrap">
        <SectionHead
          eyebrow="AI usage"
          title="AI usage is included in every Cloud plan."
          lede="No provider accounts or API keys to manage. Each plan comes with a monthly allowance, and you decide whether usage can go beyond it."
        />
        <div className="pricing-ai-routes">
          <article>
            <Cloud size={20} strokeWidth={1.75} aria-hidden="true" />
            <div>
              <h3>Included allowance</h3>
              <p>Starter includes ${AI_ALLOWANCE.starter.monthly} of AI usage a month and Growth includes ${AI_ALLOWANCE.growth.monthly} (${AI_ALLOWANCE.starter.annual} and ${AI_ALLOWANCE.growth.annual} on annual billing). The allowance resets monthly and doesn’t carry over.</p>
            </div>
            <span>Billed by Helpin</span>
          </article>
          <article>
            <Gauge size={20} strokeWidth={1.75} aria-hidden="true" />
            <div>
              <h3>When the allowance runs out</h3>
              <p>Overage is off by default, so new AI work is declined until the allowance renews. Work already running keeps what it reserved. On an active paid plan, you can turn on metered overage at the same rates.</p>
            </div>
            <span>Optional, billed monthly</span>
          </article>
          <article>
            <KeyRound size={20} strokeWidth={1.75} aria-hidden="true" />
            <div>
              <h3>Your own provider keys</h3>
              <p>Self-hosted teams connect their own provider and pay it directly, with no Helpin AI fees. On Cloud, your own keys are available with Enterprise.</p>
              <ul className="pricing-ai-providers" aria-label="Supported providers">
                {PROVIDERS.map(provider => <li key={provider.id}><img src={`/new/providers/${provider.id}.svg`} width={16} height={16} alt="" />{provider.name}</li>)}
                <li>Compatible endpoints</li>
              </ul>
            </div>
            <span>Self-hosted · Enterprise on Cloud</span>
          </article>
        </div>

        <details className="pricing-disclosure" id="ai-usage-charges">
          <summary>Models and token rates</summary>
          <div>
            <div className="pricing-rate-scroll" tabIndex={0} role="region" aria-label="Managed AI token rates">
              <table>
                <caption>USD per million tokens, effective {AI_PRICING.effective_date}</caption>
                <thead><tr><th scope="col">Profile</th><th scope="col">Models</th><th scope="col">Input</th><th scope="col">Cache read</th><th scope="col">Cache write</th><th scope="col">Output / reasoning</th></tr></thead>
                <tbody>{AI_PRICING.tiers.map(tier => <tr key={tier.key}>
                  <th scope="row">{tier.label}<small>{tier.description}</small></th>
                  <td className="pricing-rate-models">{modelsFor(tier.key).join(', ')}</td>
                  <td>{usd(tier.rates.input_microusd_per_million)}</td>
                  <td>{usd(tier.rates.cache_read_microusd_per_million)}</td>
                  <td>{usd(tier.rates.cache_write_microusd_per_million)}</td>
                  <td>{usd(tier.rates.output_microusd_per_million)}</td>
                </tr>)}</tbody>
              </table>
            </div>
            <dl className="pricing-rate-notes">
              <div><dt>How usage is charged</dt><dd>Tokens × the profile’s rate ÷ 1,000,000. Reasoning tokens are charged at the output rate. Charges come out of your monthly allowance.</dd></div>
              <div><dt>Overage</dt><dd>Opt-in on active paid plans, not during the trial. Billed monthly at the same rates, rounded to the cent. No prepaid blocks.</dd></div>
              <div><dt>Tracking usage</dt><dd>Billing settings show how much of the allowance is used and reserved, and when it resets.</dd></div>
              <div><dt>Trial</dt><dd>The 14-day Growth trial includes ${AI_ALLOWANCE.trial} of AI usage in total.</dd></div>
            </dl>
            <a className="btn-link" href={DOCS.aiConnections}>AI connection guide →</a>
          </div>
        </details>
      </div>
    </section>
  );
}
