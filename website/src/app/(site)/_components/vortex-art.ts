// Line geometry and drawing for HeroVortex. DOM-free, so the same code runs in the render
// worker or, where OffscreenCanvas is missing, on the main thread. Every frame rewrites
// preallocated point buffers in place: no arrays are created while animating.
// Adapted from the teammate's VortexLines.jsx handoff (September 2026).
// Source: https://storage.googleapis.com/vm-dev-screenshots/Helpin.ai%20Open%20Source%20Alternative.zip

export type MotionVariant = 'vortex' | 'flow' | 'connections' | 'orbit' | 'converge';
export type Line = { pts: Float32Array; opacity: number };
export type Art = { lines: Line[]; update: (t: number) => void };

export const W = 1400, H = 700;
const CX = 700, CY = 350, LENGTH = 1100, SPREAD = 190, GAP = 185;
const SAMPLES = 48;
const TAU = Math.PI * 2;

const scratch = new Float32Array(512);

function line(samples: number, opacity: number): Line {
  return { pts: new Float32Array(samples * 2), opacity };
}

function quadInto(p: Float32Array, x0: number, y0: number, x1: number, y1: number, x2: number, y2: number) {
  const n = p.length / 2 - 1;
  for (let i = 0; i <= n; i++) {
    const t = i / n, a = (1 - t) * (1 - t), b = 2 * (1 - t) * t, c = t * t;
    p[i * 2] = a * x0 + b * x1 + c * x2;
    p[i * 2 + 1] = a * y0 + b * y1 + c * y2;
  }
}

function cubicInto(p: Float32Array, x0: number, y0: number, x1: number, y1: number, x2: number, y2: number, x3: number, y3: number) {
  const n = p.length / 2 - 1;
  for (let i = 0; i <= n; i++) {
    const t = i / n, u = 1 - t, a = u * u * u, b = 3 * u * u * t, c = 3 * u * t * t, d = t * t * t;
    p[i * 2] = a * x0 + b * x1 + c * x2 + d * x3;
    p[i * 2 + 1] = a * y0 + b * y1 + c * y2 + d * y3;
  }
}

// A ripple that travels along the line, tapered to zero at both ends.
function ripple(p: Float32Array, amplitude: number, waves: number, phase: number) {
  const count = p.length / 2, last = count - 1;
  scratch.set(p);
  for (let i = 0; i < count; i++) {
    const a = Math.max(0, i - 1) * 2, b = Math.min(last, i + 1) * 2;
    const dx = scratch[b] - scratch[a], dy = scratch[b + 1] - scratch[a + 1];
    const length = Math.hypot(dx, dy) || 1, s = i / last;
    const offset = amplitude * Math.sin(s * waves * TAU - phase) * Math.sin(s * Math.PI) / length;
    p[i * 2] = scratch[i * 2] - dy * offset;
    p[i * 2 + 1] = scratch[i * 2 + 1] + dx * offset;
  }
}

function rotate(p: Float32Array, degrees: number) {
  const r = degrees * Math.PI / 180, cos = Math.cos(r), sin = Math.sin(r);
  for (let i = 0; i < p.length; i += 2) {
    const x = p[i] - CX, y = p[i + 1] - CY;
    p[i] = CX + x * cos - y * sin;
    p[i + 1] = CY + x * sin + y * cos;
  }
}

function mirror(p: Float32Array) {
  for (let i = 0; i < p.length; i += 2) p[i] = W - p[i];
}

// Two crossing pairs of bundles that twist against each other, like a slow vortex.
function vortex(): Art {
  const lines = Array.from({ length: 56 }, (_, k) => line(SAMPLES, Math.max(0.2, 0.9 - Math.abs((k % 14) / 13 - 0.5) * 1.1)));
  return {
    lines,
    update(t) {
      for (let bundle = 0; bundle < 4; bundle++) {
        const vertical = bundle < 2, side = bundle % 2 ? 1 : -1;
        const twist = (vertical ? 1 : -1) * (Math.sin(t * 0.32) * 4.5 + Math.sin(t * 0.13 + bundle) * 1.5);
        for (let index = 0; index < 14; index++) {
          const p = lines[bundle * 14 + index].pts, u = index / 13 - 0.5;
          const position = side * GAP + u * SPREAD + Math.sin(t * 0.5 + index * 0.45 + bundle) * 7;
          const bow = u * 130 * (1 + 0.35 * Math.sin(t * 0.55 - index * 0.32 + bundle * 1.3));
          const edge = position - bow * 0.4, middle = position + bow;
          if (vertical) quadInto(p, CX + edge, CY - LENGTH, CX + middle, CY, CX + edge, CY + LENGTH);
          else quadInto(p, CX - LENGTH, CY + edge, CX, CY + middle, CX + LENGTH, CY + edge);
          ripple(p, 6, 2.2, t * 1.1 + index * 0.35 + bundle);
          rotate(p, (vertical ? 9 : -9) + twist);
        }
      }
    },
  };
}

function flow(): Art {
  const lines = Array.from({ length: 24 }, (_, k) => line(SAMPLES, 0.3 + (1 - Math.abs((k % 12) - 5.5) / 6) * 0.4));
  return {
    lines,
    update(t) {
      for (let group = 0; group < 2; group++) for (let i = 0; i < 12; i++) {
        const p = lines[group * 12 + i].pts, sway = t * 0.45 + i * 0.22 + group * 2;
        cubicInto(p,
          -120, 130 + group * 250 + i * 13,
          330, -80 + group * 370 + i * 17 + Math.sin(sway) * 45,
          810, 530 - group * 220 + i * 9 + Math.cos(sway * 0.8) * 45,
          1520, 150 + group * 260 + i * 12);
        ripple(p, 5, 2.5, t * 1.2 + i * 0.3);
        rotate(p, (group ? -7 : 7) + Math.sin(t * 0.2 + group) * 1.5);
      }
    },
  };
}

// Routed traces with rounded corners, sampled evenly by arc length so streaks keep a steady
// pace. The vertical legs drift, so the traces reroute gently.
const CORNER = 40, CORNER_LENGTH = CORNER * 1.53; // arc length of a quadratic corner of radius 40
function connections(): Art {
  const samples = SAMPLES * 2;
  const lines = Array.from({ length: 24 }, (_, k) => line(samples, 0.32 + ((k % 12) % 4) * 0.08));
  return {
    lines,
    update(t) {
      for (let side = 0; side < 2; side++) for (let i = 0; i < 12; i++) {
        const p = lines[side * 12 + i].pts;
        const x = 270 + i * 20 + Math.sin(t * 0.4 + i * 0.3 + side) * 14;
        const top = 70 + i * 23, bottom = 480 + i * 11;
        const legs = [x - CORNER + 100, CORNER_LENGTH, bottom - top - CORNER * 2, CORNER_LENGTH, 1500 - x - CORNER];
        const total = legs[0] + legs[1] + legs[2] + legs[3] + legs[4];
        for (let k = 0; k < samples; k++) {
          let s = k / (samples - 1) * total, px: number, py: number;
          if (s <= legs[0]) { px = -100 + s; py = top; }
          else if ((s -= legs[0]) <= legs[1]) {
            const u = s / legs[1];
            px = (1 - u) * (1 - u) * (x - CORNER) + 2 * (1 - u) * u * x + u * u * x;
            py = (1 - u) * (1 - u) * top + 2 * (1 - u) * u * top + u * u * (top + CORNER);
          } else if ((s -= legs[1]) <= legs[2]) { px = x; py = top + CORNER + s; }
          else if ((s -= legs[2]) <= legs[3]) {
            const u = s / legs[3];
            px = (1 - u) * (1 - u) * x + 2 * (1 - u) * u * x + u * u * (x + CORNER);
            py = (1 - u) * (1 - u) * (bottom - CORNER) + 2 * (1 - u) * u * bottom + u * u * bottom;
          } else { px = x + CORNER + (s - legs[3]); py = bottom; }
          p[k * 2] = px;
          p[k * 2 + 1] = py;
        }
        if (side === 0) mirror(p);
      }
    },
  };
}

// Nested rings turning at slightly different speeds, so the field shears like a whirlpool.
function orbit(): Art {
  const samples = SAMPLES * 2;
  const lines = Array.from({ length: 18 }, (_, i) => line(samples, 0.26 + (1 - Math.abs(i - 8.5) / 9) * 0.4));
  return {
    lines,
    update(t) {
      for (let i = 0; i < 18; i++) {
        const p = lines[i].pts;
        const rx = 340 + i * 18 + Math.sin(t * 0.4 + i * 0.3) * 8, ry = 150 + i * 11, spin = t * (0.05 + i * 0.004);
        for (let k = 0; k < samples; k++) {
          const a = spin + k / (samples - 1) * TAU;
          p[k * 2] = CX + Math.cos(a) * rx;
          p[k * 2 + 1] = CY + Math.sin(a) * ry;
        }
        rotate(p, -14 + Math.sin(t * 0.18) * 3);
      }
    },
  };
}

function converge(): Art {
  const lines = Array.from({ length: 20 }, (_, k) => line(SAMPLES, 0.36 + ((k % 10) % 3) * 0.09));
  return {
    lines,
    update(t) {
      for (let side = 0; side < 2; side++) for (let i = 0; i < 10; i++) {
        const p = lines[side * 10 + i].pts, sway = t * 0.4 + i * 0.35 + side * 1.7;
        cubicInto(p,
          -120, 40 + i * 44,
          200, 40 + i * 44 + Math.sin(sway) * 30,
          350 + Math.cos(sway) * 25, 470 + i * 5,
          700, 560);
        ripple(p, 4, 2, t * 1.2 + i * 0.5);
        if (side) mirror(p);
      }
    },
  };
}

const FACTORIES: Record<MotionVariant, () => Art> = { vortex, flow, connections, orbit, converge };
export const createArt = (variant: MotionVariant) => FACTORIES[variant]();

function trace(ctx: Ctx, p: Float32Array, from: number, to: number) {
  ctx.beginPath();
  ctx.moveTo(p[from * 2], p[from * 2 + 1]);
  for (let i = from + 1; i <= to; i++) ctx.lineTo(p[i * 2], p[i * 2 + 1]);
}

type Ctx = CanvasRenderingContext2D | OffscreenCanvasRenderingContext2D;
export type Viewport = { width: number; height: number; dpr: number; scale: number; originX: number; originY: number };

const baseGradients = new WeakMap<Ctx, CanvasGradient>();

// Draws one frame. `scale` maps artwork units to CSS pixels; line widths stay in CSS pixels.
export function drawArt(ctx: Ctx, art: Art, t: number, view: Viewport, playing: boolean) {
  const { dpr, scale } = view;
  ctx.setTransform(1, 0, 0, 1, 0, 0);
  ctx.clearRect(0, 0, ctx.canvas.width, ctx.canvas.height);
  if (!ctx.canvas.width || !ctx.canvas.height) return;
  art.update(t);
  ctx.setTransform(dpr * scale, 0, 0, dpr * scale, dpr * view.originX, dpr * view.originY);
  ctx.lineCap = 'round';
  ctx.lineJoin = 'round';
  const px = 1 / scale;

  let base = baseGradients.get(ctx);
  if (!base) {
    base = ctx.createLinearGradient(0, 0, W, H);
    base.addColorStop(0, '#0B7A4E');
    base.addColorStop(0.5, '#0F9D63');
    base.addColorStop(1, '#3FC48F');
    baseGradients.set(ctx, base);
  }
  ctx.strokeStyle = base;
  for (const { pts, opacity } of art.lines) {
    ctx.globalAlpha = opacity;
    ctx.lineWidth = (0.9 + opacity * 0.8) * px;
    trace(ctx, pts, 0, pts.length / 2 - 1);
    ctx.stroke();
  }

  // A paused scene shows only the underlying lines, never a stranded light streak.
  if (!playing) {
    ctx.globalAlpha = 1;
    return;
  }

  // Streaks of light run along every other line, fading in and out at the path ends.
  ctx.lineWidth = 2 * px;
  for (let j = 1; j < art.lines.length; j += 2) {
    const p = art.lines[j].pts, count = p.length / 2;
    const length = count * 0.14;
    const progress = ((t + j * 1.7) / (6 + (j % 3) * 1.2)) % 1;
    const head = progress * (count - 1 + length);
    const from = Math.max(0, head - length), to = Math.min(count - 1, head);
    if (to - from < 2) continue;
    const start = Math.floor(from), end = Math.floor(to);
    const startOffset = from - start, endOffset = to - end;
    const x0 = p[start * 2] + (p[(start + 1) * 2] - p[start * 2]) * startOffset;
    const y0 = p[start * 2 + 1] + (p[(start + 1) * 2 + 1] - p[start * 2 + 1]) * startOffset;
    const x1 = p[end * 2] + (p[Math.min(end + 1, count - 1) * 2] - p[end * 2]) * endOffset;
    const y1 = p[end * 2 + 1] + (p[Math.min(end + 1, count - 1) * 2 + 1] - p[end * 2 + 1]) * endOffset;
    const streak = ctx.createLinearGradient(x0, y0, x1, y1);
    streak.addColorStop(0, 'rgba(63,184,133,0)');
    streak.addColorStop(0.6, 'rgba(63,184,133,.9)');
    streak.addColorStop(1, 'rgba(226,250,238,1)');
    ctx.strokeStyle = streak;
    ctx.globalAlpha = Math.min(1, progress / 0.14, (1 - progress) / 0.14);
    ctx.beginPath();
    ctx.moveTo(x0, y0);
    for (let i = Math.ceil(from); i < to; i++) ctx.lineTo(p[i * 2], p[i * 2 + 1]);
    ctx.lineTo(x1, y1);
    ctx.stroke();
  }
  ctx.globalAlpha = 1;
}
