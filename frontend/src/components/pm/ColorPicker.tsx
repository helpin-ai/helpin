import { useCallback, useEffect, useRef, useState } from 'react';
import { PaintBoardIcon } from '@/lib/icons';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Input } from '@/components/ui/input';
import { cn } from '@/lib/utils';
import { PRESET_COLORS } from '@/lib/colorPresets';

export { PRESET_COLORS, EPIC_PRESET_COLORS } from '@/lib/colorPresets';

// ── Color conversion utilities ──────────────────────────────────────

interface HSV { h: number; s: number; v: number }

function hexToHsv(hex: string): HSV {
  const r = parseInt(hex.slice(1, 3), 16) / 255;
  const g = parseInt(hex.slice(3, 5), 16) / 255;
  const b = parseInt(hex.slice(5, 7), 16) / 255;
  const max = Math.max(r, g, b);
  const min = Math.min(r, g, b);
  const d = max - min;
  let h = 0;
  if (d !== 0) {
    if (max === r) h = ((g - b) / d + 6) % 6;
    else if (max === g) h = (b - r) / d + 2;
    else h = (r - g) / d + 4;
    h *= 60;
  }
  const s = max === 0 ? 0 : d / max;
  return { h, s, v: max };
}

function hsvToHex({ h, s, v }: HSV): string {
  const c = v * s;
  const x = c * (1 - Math.abs(((h / 60) % 2) - 1));
  const m = v - c;
  let r = 0, g = 0, b = 0;
  if (h < 60) { r = c; g = x; }
  else if (h < 120) { r = x; g = c; }
  else if (h < 180) { g = c; b = x; }
  else if (h < 240) { g = x; b = c; }
  else if (h < 300) { r = x; b = c; }
  else { r = c; b = x; }
  const toHex = (n: number) => Math.round((n + m) * 255).toString(16).padStart(2, '0');
  return `#${toHex(r)}${toHex(g)}${toHex(b)}`;
}

function hueToHex(h: number): string {
  return hsvToHex({ h, s: 1, v: 1 });
}

const HEX_REGEX = /^#([0-9A-Fa-f]{3}){1,2}$/;

function normalizeHex(hex: string): string {
  let h = hex.startsWith('#') ? hex : `#${hex}`;
  if (/^#[0-9A-Fa-f]{3}$/i.test(h)) {
    h = `#${h[1]}${h[1]}${h[2]}${h[2]}${h[3]}${h[3]}`;
  }
  return h.toLowerCase();
}

// ── Saturation Area ─────────────────────────────────────────────────

function SaturationArea({
  hsv,
  onChange,
}: {
  hsv: HSV;
  onChange: (s: number, v: number) => void;
}) {
  const areaRef = useRef<HTMLDivElement>(null);
  const dragging = useRef(false);

  const update = useCallback(
    (clientX: number, clientY: number) => {
      const el = areaRef.current;
      if (!el) return;
      const rect = el.getBoundingClientRect();
      const s = Math.max(0, Math.min(1, (clientX - rect.left) / rect.width));
      const v = Math.max(0, Math.min(1, 1 - (clientY - rect.top) / rect.height));
      onChange(s, v);
    },
    [onChange],
  );

  const onPointerDown = useCallback(
    (e: React.PointerEvent) => {
      e.preventDefault();
      dragging.current = true;
      (e.target as HTMLElement).setPointerCapture(e.pointerId);
      update(e.clientX, e.clientY);
    },
    [update],
  );

  const onPointerMove = useCallback(
    (e: React.PointerEvent) => {
      if (!dragging.current) return;
      update(e.clientX, e.clientY);
    },
    [update],
  );

  const onPointerUp = useCallback(() => {
    dragging.current = false;
  }, []);

  return (
    <div
      ref={areaRef}
      className="relative h-[150px] w-full cursor-crosshair rounded-md"
      style={{ backgroundColor: hueToHex(hsv.h) }}
      onPointerDown={onPointerDown}
      onPointerMove={onPointerMove}
      onPointerUp={onPointerUp}
    >
      {/* White gradient left → right */}
      <div
        className="absolute inset-0 rounded-md"
        style={{ background: 'linear-gradient(to right, #fff, transparent)' }}
      />
      {/* Black gradient bottom → top */}
      <div
        className="absolute inset-0 rounded-md"
        style={{ background: 'linear-gradient(to top, #000, transparent)' }}
      />
      {/* Thumb */}
      <div
        className="pointer-events-none absolute h-3.5 w-3.5 -translate-x-1/2 -translate-y-1/2 rounded-full border-2 border-white shadow-md"
        style={{
          left: `${hsv.s * 100}%`,
          top: `${(1 - hsv.v) * 100}%`,
          backgroundColor: hsvToHex(hsv),
        }}
      />
    </div>
  );
}

// ── Hue Slider ──────────────────────────────────────────────────────

function HueSlider({
  hue,
  onChange,
}: {
  hue: number;
  onChange: (h: number) => void;
}) {
  const trackRef = useRef<HTMLDivElement>(null);
  const dragging = useRef(false);

  const update = useCallback(
    (clientX: number) => {
      const el = trackRef.current;
      if (!el) return;
      const rect = el.getBoundingClientRect();
      const ratio = Math.max(0, Math.min(1, (clientX - rect.left) / rect.width));
      onChange(ratio * 360);
    },
    [onChange],
  );

  const onPointerDown = useCallback(
    (e: React.PointerEvent) => {
      e.preventDefault();
      dragging.current = true;
      (e.target as HTMLElement).setPointerCapture(e.pointerId);
      update(e.clientX);
    },
    [update],
  );

  const onPointerMove = useCallback(
    (e: React.PointerEvent) => {
      if (!dragging.current) return;
      update(e.clientX);
    },
    [update],
  );

  const onPointerUp = useCallback(() => {
    dragging.current = false;
  }, []);

  return (
    <div
      ref={trackRef}
      className="relative h-3 w-full cursor-pointer rounded-full"
      style={{
        background:
          'linear-gradient(to right, #f00 0%, #ff0 17%, #0f0 33%, #0ff 50%, #00f 67%, #f0f 83%, #f00 100%)',
      }}
      onPointerDown={onPointerDown}
      onPointerMove={onPointerMove}
      onPointerUp={onPointerUp}
    >
      <div
        className="pointer-events-none absolute top-1/2 h-4 w-4 -translate-x-1/2 -translate-y-1/2 rounded-full border-2 border-white shadow-md"
        style={{
          left: `${(hue / 360) * 100}%`,
          backgroundColor: hueToHex(hue),
        }}
      />
    </div>
  );
}

// ── Brand Color Picker (swatch + hex, no presets) ───────────────────

export function BrandColorPicker({
  value,
  onChange,
}: {
  value: string;
  onChange: (color: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const [hsv, setHsv] = useState<HSV>(() => hexToHsv(value || '#6366f1'));
  const [hexInput, setHexInput] = useState(value);

  useEffect(() => {
    if (open) {
      const valid = HEX_REGEX.test(normalizeHex(value)) ? normalizeHex(value) : '#6366f1';
      setHsv(hexToHsv(valid));
      setHexInput(valid);
    }
  }, [open, value]);

  const handleHsvChange = useCallback(
    (next: HSV) => {
      setHsv(next);
      const hex = hsvToHex(next);
      setHexInput(hex);
      onChange(hex);
    },
    [onChange],
  );

  const applyHex = (hex: string) => {
    const normalized = normalizeHex(hex);
    if (HEX_REGEX.test(normalized)) {
      onChange(normalized);
      setHsv(hexToHsv(normalized));
      setHexInput(normalized);
    }
  };

  return (
    <div className="flex items-center gap-2">
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger asChild>
          <button
            type="button"
            className="h-9 w-9 shrink-0 rounded-md border shadow-sm cursor-pointer transition-all hover:ring-2 hover:ring-ring hover:ring-offset-1"
            style={{ backgroundColor: value }}
          />
        </PopoverTrigger>
        <PopoverContent className="w-[260px] p-3" align="start" side="top">
          <div className="space-y-3">
            <SaturationArea
              hsv={hsv}
              onChange={(s, v) => handleHsvChange({ ...hsv, s, v })}
            />
            <HueSlider
              hue={hsv.h}
              onChange={(h) => handleHsvChange({ ...hsv, h })}
            />
            <div className="flex items-center gap-2">
              <div
                className="h-8 w-8 shrink-0 rounded-md border border-border"
                style={{ backgroundColor: hsvToHex(hsv) }}
              />
              <Input
                value={hexInput}
                onChange={(e) => setHexInput(e.target.value)}
                onBlur={() => applyHex(hexInput)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    e.preventDefault();
                    applyHex(hexInput);
                  }
                }}
                placeholder="#000000"
                className="h-8 font-mono text-xs"
              />
            </div>
          </div>
        </PopoverContent>
      </Popover>
      <Input
        value={value}
        onChange={(e) => {
          const v = e.target.value;
          setHexInput(v);
          if (HEX_REGEX.test(normalizeHex(v))) {
            onChange(normalizeHex(v));
          }
        }}
        placeholder="#6366F1"
        className="w-28 font-mono text-sm"
      />
    </div>
  );
}

// ── Main ColorPicker ────────────────────────────────────────────────

export function ColorPicker({
  value,
  onChange,
  shape = 'circle',
  presets = PRESET_COLORS,
}: {
  value: string;
  onChange: (color: string) => void;
  shape?: 'circle' | 'square';
  presets?: readonly string[];
}) {
  const isCustom = Boolean(value) && !presets.includes(value);
  const [customOpen, setCustomOpen] = useState(false);
  const [hsv, setHsv] = useState<HSV>(() => hexToHsv(value || '#3b82f6'));
  const [hexInput, setHexInput] = useState(value);

  // Sync HSV when popover opens or value changes externally
  useEffect(() => {
    if (customOpen) {
      const valid = HEX_REGEX.test(normalizeHex(value)) ? normalizeHex(value) : '#3b82f6';
      setHsv(hexToHsv(valid));
      setHexInput(valid);
    }
  }, [customOpen, value]);

  const handleHsvChange = useCallback(
    (next: HSV) => {
      setHsv(next);
      const hex = hsvToHex(next);
      setHexInput(hex);
      onChange(hex);
    },
    [onChange],
  );

  const applyHex = (hex: string) => {
    const normalized = normalizeHex(hex);
    if (HEX_REGEX.test(normalized)) {
      onChange(normalized);
      setHsv(hexToHsv(normalized));
      setHexInput(normalized);
    }
  };

  return (
    <div className="flex flex-wrap items-center gap-1.5">
      {presets.map((c) => (
        <button
          key={c}
          type="button"
          aria-label={`Select color ${c}`}
          aria-pressed={value === c}
          title={c}
          className={cn(
            'h-6 w-6 border-2 transition-colors cursor-pointer focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-quiet-text-primary',
            shape === 'square' ? 'rounded-[4px]' : 'rounded-full',
            value === c
              ? 'border-foreground scale-110'
              : 'border-transparent hover:border-muted-foreground/40',
          )}
          style={{ backgroundColor: c }}
          onClick={() => { onChange(c); setCustomOpen(false); }}
        />
      ))}

      <Popover open={customOpen} onOpenChange={setCustomOpen}>
        <PopoverTrigger asChild>
          <button
            type="button"
            aria-label="Custom color"
            aria-pressed={isCustom}
            className={cn(
              'relative flex h-6 w-6 items-center justify-center border-2 transition-colors cursor-pointer focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-quiet-text-primary',
              shape === 'square' ? 'rounded-[4px]' : 'rounded-full',
              isCustom
                ? 'border-foreground scale-110'
                : 'border-dashed border-muted-foreground/40 hover:border-muted-foreground/70',
            )}
            style={isCustom ? { backgroundColor: value } : undefined}
            title="Custom color"
          >
            {!isCustom && <PaintBoardIcon className="h-3 w-3 text-muted-foreground" />}
          </button>
        </PopoverTrigger>
        <PopoverContent className="w-[260px] p-3" align="start" side="top">
          <div className="space-y-3">
            <SaturationArea
              hsv={hsv}
              onChange={(s, v) => handleHsvChange({ ...hsv, s, v })}
            />
            <HueSlider
              hue={hsv.h}
              onChange={(h) => handleHsvChange({ ...hsv, h })}
            />
            <div className="flex items-center gap-2">
              <div
                className="h-8 w-8 shrink-0 rounded-md border border-border"
                style={{ backgroundColor: hsvToHex(hsv) }}
              />
              <Input
                aria-label="Hex color"
                aria-invalid={!HEX_REGEX.test(normalizeHex(hexInput))}
                value={hexInput}
                onChange={(e) => setHexInput(e.target.value)}
                onBlur={() => applyHex(hexInput)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    e.preventDefault();
                    applyHex(hexInput);
                  }
                }}
                placeholder="#000000"
                className={cn('h-8 font-mono text-xs', shape === 'square' && 'focus-visible:ring-quiet-text-primary')}
              />
            </div>
          </div>
        </PopoverContent>
      </Popover>
    </div>
  );
}
