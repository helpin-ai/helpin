'use client';

import { useId, useState, type CSSProperties } from 'react';
import { AI_ALLOWANCE, PLANS } from '../../pricing/pricing-data';
import type { Calculator, CalculatorPlan } from './compare-data';

type Billing = 'annual' | 'monthly';
type Mode = 'cloud' | 'self';
type Line = readonly [label: string, value: string];

const MAX_SEATS = 100;

const money = (value: number) => `$${Math.round(value).toLocaleString('en-US')}`;
// Unit prices keep their cents (Jira's $9.05), so the line's arithmetic reads true.
const unitMoney = (value: number) => (Number.isInteger(value) ? money(value) : `$${value.toFixed(2)}`);
const share = (value: number, total: number) => `${total > 0 ? (value / total) * 100 : 0}%`;
const unitPrice = (plan: CalculatorPlan, billing: Billing) => (billing === 'monthly' && plan.monthly !== undefined ? plan.monthly : plan.annual);
const annualTier = (plan: CalculatorPlan, billing: Billing, seats: number) => billing === 'annual' ? plan.annualTiers?.find(tier => seats <= tier.maxSeats) : undefined;
const count = (value: number, [one, many]: readonly [string, string]) => `${value.toLocaleString('en-US')} ${value === 1 ? one : many}`;

function Segmented<T extends string | number>({ label, value, options, onChange }: { label: string; value: T; options: readonly (readonly [T, string])[]; onChange: (value: T) => void }) {
  return (
    <div className="cmp-calc-field">
      <span className="cmp-calc-label">{label}</span>
      <div className="cmp-calc-segmented" role="group" aria-label={label}>
        {options.map(([option, text]) => <button key={String(option)} type="button" aria-pressed={option === value} onClick={() => onChange(option)}>{text}</button>)}
      </div>
    </div>
  );
}

function Slider({ id, label, value, min, max, step = 1, onChange }: { id: string; label: string; value: number; min: number; max: number; step?: number; onChange: (value: number) => void }) {
  return (
    <div className="cmp-calc-field">
      <label className="cmp-calc-label" htmlFor={id}>{label}<output htmlFor={id}>{value.toLocaleString('en-US')}</output></label>
      <input id={id} className="cmp-range" type="range" min={min} max={max} step={step} value={value} style={{ '--fill': share(value - min, max - min) } as CSSProperties} onChange={event => onChange(Number(event.target.value))} />
      <span className="cmp-range-scale" aria-hidden="true"><span>{min.toLocaleString('en-US')}</span><span>{max.toLocaleString('en-US')}</span></span>
    </div>
  );
}

// One product's price: the monthly and yearly totals first, then how they're made up.
function Quote({ name, plan, lines, monthly, note, helpin }: { name: string; plan: string; lines: Line[]; monthly: number; note?: string; helpin?: boolean }) {
  return (
    <div className="cmp-calc-quote" data-product={helpin ? 'helpin' : 'rival'}>
      <div className="cmp-calc-quote-head">
        {helpin ? <img className="cmp-mark" src="/brand/helpin-icon-ink.svg" width={20} height={20} alt="" /> : null}
        <strong>{name}</strong>
        <span>{plan}</span>
      </div>
      <p className="cmp-calc-quote-total"><strong>{money(monthly)}</strong> a month</p>
      <p className="cmp-calc-quote-year">{money(monthly * 12)} a year</p>
      <dl>{lines.map(([label, value]) => <div key={label}><dt>{label}</dt><dd>{value}</dd></div>)}</dl>
      {note ? <p className="cmp-calc-quote-note">{note}</p> : null}
    </div>
  );
}

// Per-seat pricing climbs with the team while Helpin's workspace price stays flat, so
// one sentence says where, if anywhere, the two cross.
function crossover(name: string, unit: readonly [string, string], rival: (seats: number) => number, helpin: number) {
  const sizes = Array.from({ length: MAX_SEATS }, (_, index) => index + 1);
  const rivalCheaperUpTo = sizes.filter(size => rival(size) < helpin).at(-1);
  const helpinCheaperFrom = sizes.find(size => rival(size) > helpin);
  if (helpinCheaperFrom === 1) return `Helpin costs less at every team size up to ${count(MAX_SEATS, unit)}.`;
  if (helpinCheaperFrom === undefined) return rivalCheaperUpTo === undefined ? 'Both cost the same at every team size.' : `${name} costs less at every team size up to ${count(MAX_SEATS, unit)}.`;
  if (rivalCheaperUpTo === undefined) return `Helpin costs less from ${count(helpinCheaperFrom, unit)} up.`;
  return `${name} costs less up to ${count(rivalCheaperUpTo, unit)}. From ${count(helpinCheaperFrom, unit)}, Helpin costs less.`;
}

// List-price arithmetic only: every input and exclusion is visible, and the result
// says plainly when the other tool costs less.
export function PriceCalculator({ name, calculator }: { name: string; calculator: Calculator }) {
  const id = useId();
  const [mode, setMode] = useState<Mode>('cloud');
  const [billing, setBilling] = useState<Billing>('annual');
  const [seats, setSeats] = useState(calculator.seats);
  const [planIndex, setPlanIndex] = useState(calculator.plan);
  const [helpinKey, setHelpinKey] = useState(calculator.helpinPlan);
  const [volume, setVolume] = useState(calculator.ai?.volume ?? 0);

  const selfHosted = mode === 'self' && calculator.selfHosted;
  const plans = selfHosted ? calculator.selfHosted!.plans : calculator.plans;
  const plan = plans[Math.min(planIndex, plans.length - 1)];
  const perSeat = unitPrice(plan, billing);
  const ai = !selfHosted ? calculator.ai : undefined;
  const aiCost = ai ? volume * ai.price : 0;
  const aiName = ai?.label.replace(' a month', '');
  const seatPriceAt = (size: number) => {
    const band = annualTier(plan, billing, size);
    return band ? band.price / 12 : Math.max(size, plan.minSeats ?? 1) * perSeat;
  };
  const rivalAt = (size: number) => seatPriceAt(size) + aiCost;
  const billedSeats = Math.max(seats, plan.minSeats ?? 1);
  const tier = annualTier(plan, billing, seats);
  const seatCost = seatPriceAt(seats);
  const rivalMonthly = seatCost + aiCost;
  const rivalLines: Line[] = [[tier ? `${plan.name}, up to ${tier.maxSeats} people` : `${billedSeats} × ${plan.name} at ${unitMoney(perSeat)}`, money(seatCost)]];
  if (ai) rivalLines.push([`${volume.toLocaleString('en-US')} ${aiName} × $${ai.price.toFixed(2)}`, money(aiCost)]);
  const rivalNote = [
    tier ? `${money(tier.price)} billed annually for up to ${tier.maxSeats} people.` : '',
    plan.minSeats && seats < plan.minSeats ? `${plan.name} has a ${plan.minSeats}-seat minimum.` : '',
    billing === 'monthly' && !selfHosted && plan.monthly === undefined ? 'Monthly price not published; annual rate shown.' : '',
  ].filter(Boolean).join(' ');

  const helpinPlan = PLANS.find(item => item.key === helpinKey)!;
  const helpinMonthly = selfHosted ? 0 : billing === 'annual' ? helpinPlan.annual : helpinPlan.price;
  const allowance = AI_ALLOWANCE[helpinKey][billing];
  const helpinLines: Line[] = selfHosted
    ? [['Community edition license', '$0'], [count(seats, calculator.seatsUnit), 'Included'], ['AI agents', 'Your AI provider']]
    : [[`${helpinPlan.name} plan, one workspace`, money(helpinMonthly)], [count(seats, calculator.seatsUnit), 'Included'], ['AI usage allowance', `${money(allowance)} included`]];
  const helpinNote = selfHosted ? '' : `${helpinKey === 'starter' ? 'Starter includes 10 teams, 5,000 contacts and 500 documents. ' : ''}Every Cloud plan includes an AI allowance. Extra usage is metered only if you enable it.`;

  const billed = selfHosted ? 'self-hosted' : `billed ${billing === 'annual' ? 'annually' : 'monthly'}`;
  const yearlyGap = Math.abs(rivalMonthly - helpinMonthly) * 12;
  const leader = rivalMonthly > helpinMonthly ? 'helpin' : rivalMonthly < helpinMonthly ? 'rival' : 'even';
  const verdict = leader === 'helpin'
    ? <>Helpin is an estimated <em>{money(yearlyGap)} less</em> a year on these inputs.</>
    : leader === 'rival'
      ? <>{name} is an estimated <em>{money(yearlyGap)} less</em> a year on these inputs.</>
      : <>Helpin and {name} cost the same at these list prices.</>;

  return (
    <div className="cmp-calc">
      <div className="cmp-calc-controls">
        {calculator.selfHosted ? (
          <Segmented label="Hosting" value={mode} options={[['cloud', 'Cloud'], ['self', 'Self-hosted']]} onChange={value => { setMode(value); setPlanIndex(value === 'self' ? calculator.selfHosted!.plan : calculator.plan); }} />
        ) : null}
        <div className="cmp-calc-sliders">
          <Slider id={`${id}-seats`} label={calculator.seatsLabel} value={seats} min={1} max={MAX_SEATS} onChange={setSeats} />
          {ai ? <Slider id={`${id}-volume`} label={ai.label} value={volume} min={0} max={ai.max} step={ai.step} onChange={setVolume} /> : null}
        </div>
        <div className="cmp-calc-options">
          <Segmented label={`${name} plan`} value={Math.min(planIndex, plans.length - 1)} options={plans.map((item, index) => [index, item.name] as const)} onChange={setPlanIndex} />
          {!selfHosted ? <Segmented label="Helpin plan" value={helpinKey} options={[['starter', 'Starter'], ['growth', 'Growth']]} onChange={setHelpinKey} /> : null}
          {!selfHosted ? <Segmented label="Billing" value={billing} options={[['annual', 'Annual'], ['monthly', 'Monthly']]} onChange={setBilling} /> : null}
        </div>
      </div>

      <div className="cmp-calc-result" data-leader={leader}>
        <p className="cmp-calc-context">Estimated at list prices · {count(seats, calculator.seatsUnit)}{ai ? ` · ${volume.toLocaleString('en-US')} ${aiName} a month` : ''} · {billed}</p>
        <div className="cmp-calc-quotes">
          <Quote name="Helpin" helpin plan={selfHosted ? 'Community edition' : helpinPlan.name} lines={helpinLines} monthly={helpinMonthly} note={helpinNote} />
          <Quote name={name} plan={plan.name} lines={rivalLines} monthly={rivalMonthly} note={rivalNote} />
        </div>
      </div>

      <div className="cmp-calc-outcome" data-leader={leader}>
        <p className="cmp-calc-verdict" aria-live="polite">{verdict}</p>
        <p className="cmp-calc-crossover">{crossover(name, calculator.seatsUnit, rivalAt, helpinMonthly)}</p>
      </div>

      <ul className="cmp-calc-notes">
        {(selfHosted ? [calculator.selfHosted!.note] : calculator.notes).map(note => <li key={note}>{note}</li>)}
        {!selfHosted ? <li>Helpin measures AI usage in tokens, not resolutions. Paid plans can turn on metered overage beyond the allowance.</li> : null}
      </ul>
    </div>
  );
}
