// A small canvas confetti effect in Helpin colours. It draws on its own fixed,
// click-through canvas and removes it when the last piece has fallen.

interface Point {
  x: number;
  y: number;
}

interface Area {
  left: number;
  right: number;
  top: number;
  bottom: number;
}

interface Piece {
  x: number;
  y: number;
  vx: number;
  vy: number;
  gravity: number;
  drag: number;
  terminal: number;
  rot: number;
  spin: number;
  tilt: number;
  flip: number;
  wobble: number;
  wobbleSpeed: number;
  sway: number;
  shape: 'strip' | 'square' | 'circle' | 'check';
  size: number;
  color: string;
  life: number;
  max: number;
  delay: number;
}

const PALETTES = {
  light: ['#c2410c', '#ea580c', '#fb923c', '#0f766e', '#14b8a6', '#7c5cff', '#a78bfa', '#1c1a17', '#d9c7a7'],
  dark: ['#fb923c', '#fdba74', '#f97316', '#5eead4', '#2dd4bf', '#a78bfa', '#c4b5fd', '#ecebe8', '#a8a29e'],
};
const SHAPES: Piece['shape'][] = ['strip', 'strip', 'strip', 'square', 'square', 'circle', 'check'];

const rand = (min: number, max: number) => min + Math.random() * (max - min);
const pick = <T,>(items: T[]) => items[Math.floor(Math.random() * items.length)];

/**
 * Launches confetti scaled to the size of the clear: one conversation gets a
 * small sparkle, a large backlog gets bursts from the corners as well.
 * Returns a function that stops it early.
 */
export function launchInboxClearedConfetti({
  origin,
  area,
  size,
  dark,
}: {
  origin: Point;
  area: Area;
  size: number;
  dark: boolean;
}): () => void {
  const canvas = document.createElement('canvas');
  const context = canvas.getContext('2d');
  if (!context) return () => {};
  canvas.setAttribute('aria-hidden', 'true');
  canvas.dataset.inboxClearedConfetti = 'true';
  Object.assign(canvas.style, {
    position: 'fixed',
    inset: '0',
    width: '100vw',
    height: '100vh',
    pointerEvents: 'none',
    zIndex: '60',
  });
  document.body.appendChild(canvas);

  const colors = dark ? PALETTES.dark : PALETTES.light;
  const pieces: Piece[] = [];
  const timers: number[] = [];
  let width = 0;
  let height = 0;
  let dpr = 1;
  let frameId = 0;
  let last = performance.now();
  let stopped = false;

  const resize = () => {
    dpr = Math.min(window.devicePixelRatio || 1, 2);
    width = window.innerWidth;
    height = window.innerHeight;
    canvas.width = Math.round(width * dpr);
    canvas.height = Math.round(height * dpr);
  };
  resize();
  window.addEventListener('resize', resize);

  const piece = (x: number, y: number, angle: number, speed: number): Piece => {
    const radians = (angle * Math.PI) / 180;
    return {
      x, y,
      vx: Math.cos(radians) * speed,
      vy: Math.sin(radians) * speed,
      gravity: rand(0.18, 0.28),
      drag: rand(0.972, 0.986),
      terminal: rand(2.2, 4.2),
      rot: rand(0, Math.PI * 2),
      spin: rand(-0.18, 0.18),
      tilt: rand(0, Math.PI * 2),
      flip: rand(0.06, 0.16),
      wobble: rand(0, Math.PI * 2),
      wobbleSpeed: rand(0.04, 0.09),
      sway: rand(0.4, 1.4),
      shape: pick(SHAPES),
      size: rand(5, 9),
      color: pick(colors),
      life: 0,
      max: rand(170, 270),
      delay: 0,
    };
  };

  const burst = (at: Point, { count, angle = -90, spread = 70, power = 1 }: { count: number; angle?: number; spread?: number; power?: number }) => {
    for (let i = 0; i < count; i++) pieces.push(piece(at.x, at.y, angle + rand(-spread / 2, spread / 2), rand(6, 15) * power));
  };

  const rain = (count: number) => {
    for (let i = 0; i < count; i++) {
      const p = piece(rand(area.left, area.right), area.top - rand(0, 40), 90 + rand(-15, 15), rand(0.5, 2));
      p.delay = rand(0, 80);
      p.vy = rand(1, 2.5);
      pieces.push(p);
    }
  };

  const draw = (p: Piece, alpha: number) => {
    const s = p.size;
    context.save();
    context.translate(p.x, p.y);
    context.rotate(p.rot);
    context.scale(1, Math.cos(p.tilt));
    context.globalAlpha = alpha;
    context.fillStyle = p.color;
    context.strokeStyle = p.color;
    if (p.shape === 'strip') {
      context.fillRect(-s * 0.22, -s * 0.85, s * 0.44, s * 1.7);
    } else if (p.shape === 'square') {
      context.fillRect(-s * 0.45, -s * 0.45, s * 0.9, s * 0.9);
    } else if (p.shape === 'circle') {
      context.beginPath();
      context.arc(0, 0, s * 0.4, 0, Math.PI * 2);
      context.fill();
    } else {
      // A tiny tick, the resolved mark, tumbling with the rest.
      context.lineWidth = Math.max(1.6, s * 0.24);
      context.lineCap = 'round';
      context.lineJoin = 'round';
      context.beginPath();
      context.moveTo(-s * 0.55, 0);
      context.lineTo(-s * 0.12, s * 0.42);
      context.lineTo(s * 0.6, -s * 0.45);
      context.stroke();
    }
    context.restore();
  };

  const stop = () => {
    if (stopped) return;
    stopped = true;
    cancelAnimationFrame(frameId);
    timers.forEach((timer) => window.clearTimeout(timer));
    window.removeEventListener('resize', resize);
    canvas.remove();
  };

  const frame = (now: number) => {
    const k = Math.min(2.5, (now - last) / 16.667);
    last = now;
    context.setTransform(dpr, 0, 0, dpr, 0, 0);
    context.clearRect(0, 0, width, height);
    for (let i = pieces.length - 1; i >= 0; i--) {
      const p = pieces[i];
      if (p.delay > 0) {
        p.delay -= k;
        continue;
      }
      p.vx *= Math.pow(p.drag, k);
      p.vy = p.vy * Math.pow(p.drag, k) + p.gravity * k;
      if (p.vy > p.terminal) p.vy = p.terminal;
      p.wobble += p.wobbleSpeed * k;
      p.x += (p.vx + Math.sin(p.wobble) * p.sway) * k;
      p.y += p.vy * k;
      p.rot += p.spin * k;
      p.tilt += p.flip * k;
      p.life += k;
      if (p.life >= p.max || p.y > height + 30 || p.x < -40 || p.x > width + 40) {
        pieces.splice(i, 1);
        continue;
      }
      const fadeStart = p.max * 0.75;
      draw(p, p.life > fadeStart ? Math.max(0, 1 - (p.life - fadeStart) / (p.max - fadeStart)) : 1);
    }
    if (pieces.length > 0 || timers.length > 0) frameId = requestAnimationFrame(frame);
    else stop();
  };

  if (size <= 1) {
    burst(origin, { count: 28, spread: 110, power: 0.6 });
  } else if (size <= 4) {
    burst(origin, { count: 90, spread: 75 });
    rain(40);
  } else if (size <= 12) {
    burst(origin, { count: 130, spread: 80, power: 1.1 });
    rain(90);
  } else {
    burst(origin, { count: 140, spread: 85, power: 1.15 });
    rain(160);
    const corners = window.setTimeout(() => {
      timers.splice(timers.indexOf(corners), 1);
      burst({ x: area.left + 8, y: area.bottom - 8 }, { count: 90, angle: -60, spread: 60, power: 1.3 });
      burst({ x: area.right - 8, y: area.bottom - 8 }, { count: 90, angle: -120, spread: 60, power: 1.3 });
    }, 180);
    timers.push(corners);
  }
  frameId = requestAnimationFrame(frame);
  return stop;
}
