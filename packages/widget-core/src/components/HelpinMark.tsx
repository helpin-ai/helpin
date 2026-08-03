import { FunctionComponent } from 'preact';
import helpinMarkUrl from '../assets/helpin-icon-black.svg';

interface HelpinMarkProps {
  className?: string;
}

export const HelpinMark: FunctionComponent<HelpinMarkProps> = ({ className }) => (
  <img
    src={helpinMarkUrl}
    alt=""
    aria-hidden="true"
    className={className}
  />
);
