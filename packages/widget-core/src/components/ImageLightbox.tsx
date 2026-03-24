import { FunctionComponent } from 'preact';
import { useEffect } from 'preact/hooks';
import { XIcon } from './icons';

interface ImageLightboxProps {
  src: string;
  alt: string;
  onClose: () => void;
}

export const ImageLightbox: FunctionComponent<ImageLightboxProps> = ({ src, alt, onClose }) => {
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [onClose]);

  return (
    <div className="helpin-lightbox" onClick={onClose} role="dialog" aria-label="Image preview">
      <button className="helpin-lightbox-close" onClick={onClose} aria-label="Close preview">
        <XIcon size={20} />
      </button>
      <img
        src={src}
        alt={alt}
        onClick={(e) => e.stopPropagation()}
        loading="lazy"
      />
    </div>
  );
};
