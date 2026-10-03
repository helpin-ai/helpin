import type { compareVideo } from './compare-data';

type Props = { id: string; video: ReturnType<typeof compareVideo> };

// Native controls work without hydration. Only the poster downloads before play.
export function CompareFilm({ id, video }: Props) {
  return <figure id={id} className="cmp-film" aria-label={video.title}>
    <div className="cmp-film-frame">
      <video src={video.src} poster={video.poster} preload="none" controls playsInline width={1920} height={1080} aria-label={video.title} aria-describedby={`${id}-summary`} />
    </div>
    <figcaption><strong>{video.title}</strong><span id={`${id}-summary`} className="sr-only">{video.summary}</span></figcaption>
  </figure>;
}
