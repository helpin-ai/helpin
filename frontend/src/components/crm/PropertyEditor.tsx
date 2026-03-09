import { useState } from 'react';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Badge } from '@/components/ui/badge';
import { X } from 'lucide-react';
import type { CRMPropertyDefinition, CRMFieldType } from '@/lib/crmTypes';

interface PropertyEditorProps {
  properties: CRMPropertyDefinition[];
  values: Record<string, unknown>;
  onChange: (values: Record<string, unknown>) => void;
  readOnly?: boolean;
}

export function PropertyEditor({ properties, values, onChange, readOnly = false }: PropertyEditorProps) {
  const handleChange = (internalName: string, value: unknown) => {
    onChange({ ...values, [internalName]: value });
  };

  if (properties.length === 0) {
    return null;
  }

  return (
    <div className="space-y-4">
      {properties.map((prop) => (
        <PropertyField
          key={prop.id}
          property={prop}
          value={values[prop.internal_name]}
          onChange={(val) => handleChange(prop.internal_name, val)}
          readOnly={readOnly}
        />
      ))}
    </div>
  );
}

interface PropertyFieldProps {
  property: CRMPropertyDefinition;
  value: unknown;
  onChange: (value: unknown) => void;
  readOnly?: boolean;
}

function PropertyField({ property, value, onChange, readOnly }: PropertyFieldProps) {
  return (
    <div className="space-y-1.5">
      <Label className="text-sm font-medium">
        {property.label}
        {property.is_required && <span className="text-destructive ml-1">*</span>}
      </Label>
      <FieldRenderer
        fieldType={property.field_type}
        options={property.options}
        value={value}
        onChange={onChange}
        readOnly={readOnly}
      />
    </div>
  );
}

interface FieldRendererProps {
  fieldType: CRMFieldType;
  options?: Record<string, unknown>;
  value: unknown;
  onChange: (value: unknown) => void;
  readOnly?: boolean;
}

function FieldRenderer({ fieldType, options, value, onChange, readOnly }: FieldRendererProps) {
  switch (fieldType) {
    case 'text':
    case 'url':
    case 'email':
    case 'phone':
      return (
        <Input
          type={fieldType === 'email' ? 'email' : fieldType === 'url' ? 'url' : fieldType === 'phone' ? 'tel' : 'text'}
          value={(value as string) ?? ''}
          onChange={(e) => onChange(e.target.value)}
          disabled={readOnly}
          placeholder={`Enter ${fieldType}...`}
        />
      );

    case 'number':
    case 'currency':
      return (
        <Input
          type="number"
          value={(value as number) ?? ''}
          onChange={(e) => onChange(e.target.value ? Number(e.target.value) : null)}
          disabled={readOnly}
          placeholder={fieldType === 'currency' ? '0.00' : 'Enter number...'}
          step={fieldType === 'currency' ? '0.01' : undefined}
        />
      );

    case 'date':
      return (
        <Input
          type="date"
          value={(value as string) ?? ''}
          onChange={(e) => onChange(e.target.value)}
          disabled={readOnly}
        />
      );

    case 'boolean':
      return (
        <div className="flex items-center gap-2">
          <Switch
            checked={!!value}
            onCheckedChange={onChange}
            disabled={readOnly}
          />
          <span className="text-sm text-muted-foreground">
            {value ? 'Yes' : 'No'}
          </span>
        </div>
      );

    case 'select':
      return (
        <SelectField
          options={options}
          value={value as string}
          onChange={onChange}
          readOnly={readOnly}
        />
      );

    case 'multiselect':
      return (
        <MultiSelectField
          options={options}
          value={(value as string[]) ?? []}
          onChange={onChange}
          readOnly={readOnly}
        />
      );

    default:
      return (
        <Input
          value={(value as string) ?? ''}
          onChange={(e) => onChange(e.target.value)}
          disabled={readOnly}
        />
      );
  }
}

function SelectField({
  options,
  value,
  onChange,
  readOnly,
}: {
  options?: Record<string, unknown>;
  value?: string;
  onChange: (value: unknown) => void;
  readOnly?: boolean;
}) {
  const choices = getSelectOptions(options);

  return (
    <Select value={value ?? ''} onValueChange={onChange} disabled={readOnly}>
      <SelectTrigger>
        <SelectValue placeholder="Select..." />
      </SelectTrigger>
      <SelectContent>
        {choices.map((choice) => (
          <SelectItem key={choice} value={choice}>
            {choice}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}

function MultiSelectField({
  options,
  value,
  onChange,
  readOnly,
}: {
  options?: Record<string, unknown>;
  value: string[];
  onChange: (value: unknown) => void;
  readOnly?: boolean;
}) {
  const [inputValue, setInputValue] = useState('');
  const choices = getSelectOptions(options);
  const available = choices.filter((c) => !value.includes(c));

  const addItem = (item: string) => {
    if (!value.includes(item)) {
      onChange([...value, item]);
    }
    setInputValue('');
  };

  const removeItem = (item: string) => {
    onChange(value.filter((v) => v !== item));
  };

  return (
    <div className="space-y-2">
      <div className="flex flex-wrap gap-1">
        {value.map((item) => (
          <Badge key={item} variant="secondary" className="gap-1">
            {item}
            {!readOnly && (
              <button onClick={() => removeItem(item)} className="ml-1 hover:text-destructive">
                <X className="h-3 w-3" />
              </button>
            )}
          </Badge>
        ))}
      </div>
      {!readOnly && available.length > 0 && (
        <Select value={inputValue} onValueChange={addItem}>
          <SelectTrigger>
            <SelectValue placeholder="Add value..." />
          </SelectTrigger>
          <SelectContent>
            {available.map((choice) => (
              <SelectItem key={choice} value={choice}>
                {choice}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      )}
    </div>
  );
}

function getSelectOptions(options?: Record<string, unknown>): string[] {
  if (!options) return [];
  // Options can be stored as { "choices": ["a","b","c"] } or as an array directly
  if (Array.isArray(options)) return options.map(String);
  if (options.choices && Array.isArray(options.choices)) return (options.choices as string[]);
  // Try values
  return Object.values(options).filter((v) => typeof v === 'string') as string[];
}
