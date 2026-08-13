import { AI_PRICING } from '@/generated/aiPricing';
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectLabel,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';

const TIERS = ['small', 'medium', 'large', 'flagship'] as const;

interface CuratedModelSelectProps {
  id?: string;
  provider: string;
  value: string;
  disabled?: boolean;
  onValueChange: (value: string) => void;
}

export function CuratedModelSelect({ id, provider, value, disabled, onValueChange }: CuratedModelSelectProps) {
  const normalizedProvider = provider === 'openrouter-responses' ? 'openrouter' : provider;
  const models = AI_PRICING.models.filter((model) => model.provider === normalizedProvider && model.enabled);
  const known = models.some((model) => model.selection_model === value || model.canonical_model === value);

  return (
    <Select value={value} onValueChange={onValueChange} disabled={disabled || models.length === 0}>
      <SelectTrigger id={id} className="h-9">
        <SelectValue placeholder={models.length ? 'Choose an approved model' : 'No approved models'} />
      </SelectTrigger>
      <SelectContent>
        {!known && value && <SelectItem value={value} disabled>Unsupported legacy model · {value}</SelectItem>}
        {TIERS.map((tier) => {
          const tierModels = models.filter((model) => model.tier === tier);
          if (!tierModels.length) return null;
          return (
            <SelectGroup key={tier}>
              <SelectLabel>{tier[0].toUpperCase() + tier.slice(1)}</SelectLabel>
              {tierModels.map((model) => (
                <SelectItem key={`${model.provider}:${model.selection_model}`} value={model.selection_model}>
                  {model.label} · {tier[0].toUpperCase() + tier.slice(1)}
                </SelectItem>
              ))}
            </SelectGroup>
          );
        })}
      </SelectContent>
    </Select>
  );
}
