type InstallWithAIIconProps = {
  className?: string;
};

export function InstallWithAIIcon({ className }: InstallWithAIIconProps) {
  return (
    <svg
      aria-hidden="true"
      className={className}
      data-icon="install-with-ai"
      fill="none"
      focusable="false"
      viewBox="0 0 24 24"
      xmlns="http://www.w3.org/2000/svg"
    >
      <path
        d="M14.25 5.5H7A2.5 2.5 0 0 0 4.5 8v9A2.5 2.5 0 0 0 7 19.5h10a2.5 2.5 0 0 0 2.5-2.5v-6.25"
        stroke="currentColor"
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="1.75"
      />
      <path
        d="m7.75 10 2.25 2.25-2.25 2.25M11.75 15h2.5"
        stroke="currentColor"
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="1.75"
      />
      <path
        d="M18 2.75c.2 1.66 1.34 2.8 3 3-1.66.2-2.8 1.34-3 3-.2-1.66-1.34-2.8-3-3 1.66-.2 2.8-1.34 3-3Z"
        fill="currentColor"
      />
    </svg>
  );
}
