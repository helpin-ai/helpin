// Widget bootstrap entry point
// Will inject the chat widget into the host page DOM

const WIDGET_CONTAINER_ID = 'helpin-chatbox';

function bootstrap() {
  if (document.getElementById(WIDGET_CONTAINER_ID)) return;

  const container = document.createElement('div');
  container.className = 'helpin-client';
  container.innerHTML = `<div id="${WIDGET_CONTAINER_ID}"></div>`;
  document.body.appendChild(container);
}

if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', bootstrap);
} else {
  bootstrap();
}
