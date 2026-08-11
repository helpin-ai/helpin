import { FunctionComponent } from 'preact';

interface HelpinMarkProps {
  className?: string;
}

export const HelpinMark: FunctionComponent<HelpinMarkProps> = ({ className }) => (
  <svg
    viewBox="13.636 13.636 72.727 72.727"
    fill="currentColor"
    aria-hidden="true"
    focusable="false"
    className={className}
  >
    <path d="M55.818 41.273H26.727a8.727 8.727 0 0 1 0-17.455h29.091zM79.091 26.041a8.727 8.727 0 0 1 0 13.009zM20.909 73.959a8.727 8.727 0 0 1 0-13.009zM44.182 58.727h29.091a8.727 8.727 0 0 1 0 17.455H44.182zM26.041 20.909a8.727 8.727 0 0 1 13.009 0zM41.273 44.182v29.091a8.727 8.727 0 0 1-17.455 0V44.182zM58.727 55.818V26.727a8.727 8.727 0 0 1 17.455 0v29.091zM73.959 79.091a8.727 8.727 0 0 1-13.009 0z" />
  </svg>
);
