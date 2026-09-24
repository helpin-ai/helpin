import { createArt, drawArt, type MotionVariant, type Viewport } from './vortex-art';

export type VortexMessage =
  | { type: 'init'; id: number; canvas: OffscreenCanvas; variant: MotionVariant }
  | { type: 'resize'; id: number; view: Viewport }
  | { type: 'play'; id: number; playing: boolean }
  | { type: 'dispose'; id: number };

export type VortexRenderer = {
  resize: (view: Viewport) => void;
  play: (playing: boolean) => void;
  dispose: () => void;
};

let worker: Worker | null = null;
let nextId = 0;

function sharedWorker() {
  worker ??= new Worker(new URL('./vortex.worker.ts', import.meta.url), { type: 'module' });
  return worker;
}

// Draw in the shared worker when the browser can hand a canvas over; otherwise draw on the
// main thread with the same code.
export function createVortexRenderer(canvas: HTMLCanvasElement, variant: MotionVariant): VortexRenderer {
  if (typeof canvas.transferControlToOffscreen === 'function' && typeof Worker === 'function') {
    try {
      const target = sharedWorker(), id = nextId++;
      const offscreen = canvas.transferControlToOffscreen();
      const post = (message: VortexMessage, transfer: Transferable[] = []) => target.postMessage(message, transfer);
      post({ type: 'init', id, canvas: offscreen, variant }, [offscreen]);
      return {
        resize: view => post({ type: 'resize', id, view }),
        play: playing => post({ type: 'play', id, playing }),
        dispose: () => post({ type: 'dispose', id }),
      };
    } catch {
      // Fall through to main-thread drawing.
    }
  }
  return mainThreadRenderer(canvas, variant);
}

function mainThreadRenderer(canvas: HTMLCanvasElement, variant: MotionVariant): VortexRenderer {
  const ctx = canvas.getContext('2d');
  const art = createArt(variant);
  let view: Viewport | null = null, clock = 0, frame = 0, last = 0;
  const paint = () => {
    if (!ctx || !view) return;
    canvas.width = Math.round(view.width * view.dpr);
    canvas.height = Math.round(view.height * view.dpr);
    drawArt(ctx, art, clock, view);
  };
  const tick = (now: number) => {
    clock += Math.min(now - last, 50) / 1000;
    last = now;
    if (ctx && view) drawArt(ctx, art, clock, view);
    frame = requestAnimationFrame(tick);
  };
  return {
    resize: next => { view = next; paint(); },
    play: playing => {
      cancelAnimationFrame(frame);
      if (!playing) return;
      last = performance.now();
      frame = requestAnimationFrame(tick);
    },
    dispose: () => cancelAnimationFrame(frame),
  };
}
