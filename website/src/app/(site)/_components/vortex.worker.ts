/// <reference lib="webworker" />
// One worker draws every HeroVortex canvas on the page, so line motion never competes with
// scrolling, React or the product demos on the main thread.
import { createArt, drawArt, type Art, type MotionVariant, type Viewport } from './vortex-art';
import type { VortexMessage } from './vortex-renderer';

type Scene = { ctx: OffscreenCanvasRenderingContext2D; art: Art; view: Viewport | null; playing: boolean; clock: number };

const scenes = new Map<number, Scene>();
const frame: (callback: (now: number) => void) => number = typeof self.requestAnimationFrame === 'function'
  ? callback => self.requestAnimationFrame(callback)
  : callback => self.setTimeout(() => callback(performance.now()), 1000 / 60);
let scheduled = false, last = 0;

function paint(scene: Scene) {
  const { ctx, view } = scene;
  if (!view) return;
  const width = Math.round(view.width * view.dpr), height = Math.round(view.height * view.dpr);
  if (ctx.canvas.width !== width || ctx.canvas.height !== height) {
    ctx.canvas.width = width;
    ctx.canvas.height = height;
  }
  drawArt(ctx, scene.art, scene.clock, view, scene.playing);
}

function tick(now: number) {
  scheduled = false;
  // Advance only while playing, and never jump after a stalled frame.
  const delta = Math.min(now - last, 50) / 1000;
  last = now;
  let active = false;
  for (const scene of scenes.values()) {
    if (!scene.playing) continue;
    scene.clock += delta;
    paint(scene);
    active = true;
  }
  if (active) schedule();
}

function schedule() {
  if (scheduled) return;
  if (![...scenes.values()].some(scene => scene.playing)) return;
  scheduled = true;
  frame(tick);
}

self.onmessage = ({ data }: MessageEvent<VortexMessage>) => {
  const scene = scenes.get(data.id);
  switch (data.type) {
    case 'init': {
      const ctx = data.canvas.getContext('2d');
      if (ctx) scenes.set(data.id, { ctx, art: createArt(data.variant as MotionVariant), view: null, playing: false, clock: 0 });
      break;
    }
    case 'resize':
      if (!scene) break;
      scene.view = data.view;
      paint(scene);
      break;
    case 'play':
      if (!scene) break;
      if (data.playing && !scene.playing && !scheduled) last = performance.now();
      scene.playing = data.playing;
      if (!data.playing) paint(scene);
      schedule();
      break;
    case 'dispose':
      scenes.delete(data.id);
      break;
  }
};
