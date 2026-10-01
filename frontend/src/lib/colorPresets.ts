import { DEFAULT_EPIC_COLOR } from '@/components/pm/epicColor';

export const PRESET_COLORS = [
  '#5e6ad2', // indigo
  '#4e8fea', // blue
  '#3daed4', // cyan
  '#2da88e', // teal
  '#45a557', // green
  '#7da642', // olive
  '#c7a53d', // amber
  '#e58c3a', // orange
  '#e2564a', // red
  '#e54e78', // rose
  '#d44ca0', // pink
  '#b44ec9', // purple
  '#8b5cf6', // violet
  '#4a9ed6', // sky
  '#a08060', // brown
  '#788596', // slate
];

// Pastel epic presets mix in 45% white before selection and saving.
export const EPIC_PRESET_COLORS = PRESET_COLORS.map((color) => {
  if (color === '#788596') return DEFAULT_EPIC_COLOR;
  const channels = [1, 3, 5].map((offset) => {
    const channel = parseInt(color.slice(offset, offset + 2), 16);
    return Math.round(channel + (255 - channel) * 0.45).toString(16).padStart(2, '0');
  });
  return `#${channels.join('')}`;
});

