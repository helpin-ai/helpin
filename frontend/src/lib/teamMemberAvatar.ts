import { createAvatar, type Style } from '@dicebear/core';
import { personas, adventurer, micah, miniavs, toonHead } from '@dicebear/collection';

export type TeamMemberAvatarStyle = 'personas' | 'adventurer' | 'micah' | 'miniavs' | 'toonHead';
export type TeamMemberAvatarBackgroundMode = 'auto' | 'color';

const TEAM_MEMBER_AVATAR_STYLE_REGISTRY: Record<TeamMemberAvatarStyle, Style<Record<string, unknown>>> = {
  personas: personas as unknown as Style<Record<string, unknown>>,
  adventurer: adventurer as unknown as Style<Record<string, unknown>>,
  micah: micah as unknown as Style<Record<string, unknown>>,
  miniavs: miniavs as unknown as Style<Record<string, unknown>>,
  toonHead: toonHead as unknown as Style<Record<string, unknown>>,
};

export const TEAM_MEMBER_AVATAR_STYLES: Array<{ value: TeamMemberAvatarStyle; label: string }> = [
  { value: 'personas', label: 'Personas' },
  { value: 'adventurer', label: 'Adventurer' },
  { value: 'micah', label: 'Micah' },
  { value: 'miniavs', label: 'Miniavs' },
  { value: 'toonHead', label: 'Toon Head' },
];

export const TEAM_MEMBER_AVATAR_BACKGROUND_COLORS = [
  '#fbbf24',
  '#f59e0b',
  '#fb7185',
  '#ef4444',
  '#ec4899',
  '#f472b6',
  '#a78bfa',
  '#8b5cf6',
  '#818cf8',
  '#3b82f6',
  '#38bdf8',
  '#14b8a6',
  '#2dd4bf',
  '#4ade80',
  '#22c55e',
  '#a3e635',
  '#f97316',
  '#84cc16',
] as const;

const avatarCache = new Map<string, string>();

function cleanValue(value?: string | null): string | undefined {
  const trimmed = value?.trim();
  return trimmed ? trimmed : undefined;
}

function normalizeHexColor(value?: string | null): string | undefined {
  const cleaned = cleanValue(value);
  if (!cleaned) {
    return undefined;
  }
  const withHash = cleaned.startsWith('#') ? cleaned : `#${cleaned}`;
  return /^#[a-fA-F0-9]{6}$/.test(withHash) ? withHash.toLowerCase() : undefined;
}

function toDicebearColor(color: string): string {
  return color.replace(/^#/, '');
}

export function normalizeTeamMemberAvatarStyle(style?: string | null): TeamMemberAvatarStyle | undefined {
  const value = cleanValue(style);
  if (!value) {
    return undefined;
  }
  return value in TEAM_MEMBER_AVATAR_STYLE_REGISTRY
    ? (value as TeamMemberAvatarStyle)
    : undefined;
}

export function hasGeneratedTeamMemberAvatar(style?: string | null, seed?: string | null): boolean {
  return Boolean(normalizeTeamMemberAvatarStyle(style) && cleanValue(seed));
}

export function normalizeTeamMemberAvatarBackgroundMode(mode?: string | null): TeamMemberAvatarBackgroundMode {
  return cleanValue(mode) === 'auto' ? 'auto' : 'color';
}

export function normalizeTeamMemberAvatarBackgroundColor(color?: string | null): string | undefined {
  const value = normalizeHexColor(color);
  if (!value) {
    return undefined;
  }
  return TEAM_MEMBER_AVATAR_BACKGROUND_COLORS.includes(value as (typeof TEAM_MEMBER_AVATAR_BACKGROUND_COLORS)[number])
    ? value
    : undefined;
}

export function createTeamMemberAvatarSeed(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID();
  }
  return Math.random().toString(36).slice(2, 14);
}

function resolveGeneratedAvatarBackgroundColor(mode?: string | null, color?: string | null): string | undefined {
  const normalizedMode = normalizeTeamMemberAvatarBackgroundMode(mode);
  if (normalizedMode === 'color') {
    return normalizeTeamMemberAvatarBackgroundColor(color) ?? TEAM_MEMBER_AVATAR_BACKGROUND_COLORS[0];
  }
  return undefined;
}

export function resolveTeamMemberAvatarSrc({
  avatarUrl,
  avatarStyle,
  avatarSeed,
  avatarBackgroundMode,
  avatarBackgroundColor,
  fallbackSeed,
}: {
  avatarUrl?: string | null;
  avatarStyle?: string | null;
  avatarSeed?: string | null;
  avatarBackgroundMode?: string | null;
  avatarBackgroundColor?: string | null;
  fallbackSeed?: string | null;
}): string | undefined {
  const uploadedAvatar = cleanValue(avatarUrl);
  if (uploadedAvatar) {
    return uploadedAvatar;
  }

  const style = normalizeTeamMemberAvatarStyle(avatarStyle);
  const seed = cleanValue(avatarSeed) ?? cleanValue(fallbackSeed);
  if (!style || !seed) {
    return undefined;
  }

  const backgroundColor = resolveGeneratedAvatarBackgroundColor(avatarBackgroundMode, avatarBackgroundColor);
  const cacheKey = `${style}:${seed}:${backgroundColor ?? 'none'}`;
  const cached = avatarCache.get(cacheKey);
  if (cached) {
    return cached;
  }

  const src = createAvatar(TEAM_MEMBER_AVATAR_STYLE_REGISTRY[style], {
    seed,
    ...(backgroundColor ? { scale: 120 } : {}),
    ...(backgroundColor ? { backgroundType: ['solid'], backgroundColor: [toDicebearColor(backgroundColor)] } : {}),
  }).toDataUri();
  avatarCache.set(cacheKey, src);
  return src;
}
