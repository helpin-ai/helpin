import { describe, expect, it } from 'vitest';
import { parseVideoUrl } from '../videoProviders';

describe('parseVideoUrl', () => {
  it('converts YouTube watch URLs to clean embed and source URLs', () => {
    expect(parseVideoUrl('https://www.youtube.com/watch?v=yHb4Ke3WbUg')).toEqual({
      provider: 'youtube',
      sourceUrl: 'https://www.youtube.com/watch?v=yHb4Ke3WbUg',
      embedUrl: 'https://www.youtube.com/embed/yHb4Ke3WbUg',
    });
  });

  it('drops YouTube tracking params from stored source URLs', () => {
    expect(parseVideoUrl('https://www.youtube.com/watch?v=yHb4Ke3WbUg&source_ve_path=MTc4NDI0')).toEqual({
      provider: 'youtube',
      sourceUrl: 'https://www.youtube.com/watch?v=yHb4Ke3WbUg',
      embedUrl: 'https://www.youtube.com/embed/yHb4Ke3WbUg',
    });
  });
});
