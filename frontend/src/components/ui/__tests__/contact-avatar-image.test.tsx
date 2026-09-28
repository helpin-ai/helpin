// @vitest-environment jsdom
import { webcrypto } from 'node:crypto';
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { Avatar, AvatarFallback } from '../avatar';
import { ContactAvatarImage } from '../contact-avatar-image';
import * as gravatar from '@/lib/gravatar';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

describe('ContactAvatarImage', () => {
  let container: HTMLDivElement;
  let root: Root;
  let images: MockImage[];
  class MockImage extends EventTarget {
    complete = true;
    naturalWidth = 1;
    src = '';
    referrerPolicy = '';
    crossOrigin: string | null = null;
    constructor() { super(); images.push(this); }
  }

  beforeEach(() => {
    images = [];
    vi.stubGlobal('Image', MockImage);
    vi.stubGlobal('crypto', webcrypto);
    container = document.createElement('div');
    document.body.append(container);
    root = createRoot(container);
  });
  afterEach(() => {
    act(() => root.unmount());
    container.remove();
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  async function render(email?: string, src?: string) {
    await act(async () => {
      root.render(<Avatar><ContactAvatarImage email={email} src={src} alt="Alice" /><AvatarFallback>AJ</AvatarFallback></Avatar>);
    });
  }
  const imageSrc = () => container.querySelector('img')?.getAttribute('src');
  async function waitForGravatar() {
    await vi.waitFor(async () => {
      await act(async () => { await new Promise(resolve => setTimeout(resolve, 0)); });
      expect(imageSrc()).toContain('gravatar.com/avatar/');
    });
  }

  it('resolves an existing contact from email without a saved avatar', async () => {
    await render('alice@example.com');
    await waitForGravatar();
    expect(imageSrc()).toContain('gravatar.com/avatar/');
    expect(imageSrc()).not.toContain('alice@example.com');
  });

  it('prefers a stored photo and falls back to Gravatar when that photo fails', async () => {
    await render('alice@example.com', '/uploaded.png');
    expect(imageSrc()).toBe('/uploaded.png');
    expect(images.some(image => image.src.includes('gravatar.com'))).toBe(false);
    await act(async () => { images.at(-1)!.dispatchEvent(new Event('error')); });
    await waitForGravatar();
    expect(imageSrc()).toContain('gravatar.com/avatar/');
    await act(async () => { images.at(-1)!.dispatchEvent(new Event('error')); });
    expect(container.querySelector('img')).toBeNull();
    expect(container.textContent).toBe('AJ');
  });

  it('clears the previous contact photo when the identity changes and has no email', async () => {
    await render('alice@example.com');
    await waitForGravatar();
    await render();
    expect(container.querySelector('img')).toBeNull();
    expect(container.textContent).toBe('AJ');
  });

  it('ignores a delayed lookup for a contact that is no longer displayed', async () => {
    let resolvePrevious!: (value: string) => void;
    vi.spyOn(gravatar, 'getGravatarUrl').mockImplementationOnce(() => new Promise(resolve => { resolvePrevious = resolve; }));
    await render('alice@example.com');
    expect(container.textContent).toBe('AJ');
    await render('bob@example.com', '/bob.png');
    await act(async () => { resolvePrevious('https://www.gravatar.com/avatar/alice'); });
    expect(imageSrc()).toBe('/bob.png');
  });

  it('accepts a newly uploaded photo after a Gravatar failure', async () => {
    await render('alice@example.com');
    await waitForGravatar();
    await act(async () => { images.at(-1)!.dispatchEvent(new Event('error')); });
    expect(container.textContent).toBe('AJ');
    await render('alice@example.com', '/new-photo.png');
    expect(imageSrc()).toBe('/new-photo.png');
  });
});
