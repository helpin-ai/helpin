import { useRef, useEffect } from 'react';
import { render as preactRender, h } from 'preact';
import { ChatWindow, WidgetLauncher } from '@helpin/widget-core';

// Widget-core CSS — helpin-* prefixed classes, no conflicts with dashboard
import '@helpin/widget-core/styles';

interface WidgetPreviewProps {
  brandColor: string;
  showBranding: boolean;
  launcherPosition: string;
  launcherIcon: string;
  welcomeMessage: string;
  workspaceName?: string;
  workspaceLogoUrl?: string;
}

/**
 * Renders actual @helpin/widget-core Preact components inside the React dashboard
 * using an imperative Preact render bridge. The preview container uses CSS `contain: paint`
 * to trap `position: fixed` elements within the preview bounds.
 */
export function WidgetPreview({
  brandColor,
  showBranding,
  launcherPosition,
  launcherIcon,
  welcomeMessage,
  workspaceName = 'Your Company',
  workspaceLogoUrl,
}: WidgetPreviewProps) {
  const mountRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const el = mountRef.current;
    if (!el) return;

    const config = {
      workspaceId: workspaceName,
      branding: {
        primaryColor: brandColor,
        logoUrl: workspaceLogoUrl,
        welcomeMessage: welcomeMessage || 'Hi! How can we help you today?',
        widgetPosition: (launcherPosition === 'bottom_left' ? 'bottom-left' : 'bottom-right') as const,
        showBranding,
        launcherIcon: launcherIcon as 'chat_bubble' | 'question_mark' | 'help',
      },
      features: {
        aiEnabled: false,
        fileUploads: false,
        preChatForm: true,
        csatRating: false,
      },
    };

    const tree = h(
      'div',
      { className: 'helpin-widget', style: { width: '100%', height: '100%' } },
      h(ChatWindow, {
        config,
        messages: [],
        isOpen: true,
        onClose: () => {},
        onSendMessage: () => {},
        onQuickReply: () => {},
        showPreChatForm: true,
        onPreChatSubmit: () => {},
      }),
      h(WidgetLauncher, {
        onClick: () => {},
        isOpen: false,
        unreadCount: 0,
        brandColor,
        icon: launcherIcon as 'chat_bubble' | 'question_mark' | 'help',
      }),
    );

    preactRender(tree, el);
    return () => {
      preactRender(null, el);
    };
  }, [brandColor, showBranding, launcherPosition, launcherIcon, welcomeMessage, workspaceName, workspaceLogoUrl]);

  const positionSide = launcherPosition === 'bottom_left' ? 'left' : 'right';

  return (
    <>
      <style>{`
        .widget-preview-scope {
          position: relative;
          height: 420px;
          overflow: hidden;
          contain: paint;
        }
        .widget-preview-scope .helpin-chat-window {
          position: absolute;
          width: 300px;
          height: 320px;
          max-height: none;
          bottom: 70px;
          ${positionSide}: 16px;
          animation: none;
        }
        .widget-preview-scope .helpin-launcher {
          position: absolute;
          width: 48px;
          height: 48px;
          bottom: 12px;
          ${positionSide}: 16px;
          left: ${positionSide === 'left' ? '16px' : 'auto'};
          right: ${positionSide === 'right' ? '16px' : 'auto'};
        }
        .widget-preview-scope .helpin-compose-input {
          font-size: 12px;
          padding: 8px 12px;
        }
        .widget-preview-scope .helpin-compose-send {
          width: 32px;
          height: 32px;
        }
        .widget-preview-scope .helpin-pre-chat-welcome {
          font-size: 13px;
        }
        .widget-preview-scope .helpin-input {
          padding: 8px 12px;
          font-size: 12px;
        }
        .widget-preview-scope .helpin-btn-primary {
          padding: 8px 12px;
          font-size: 12px;
        }
        .widget-preview-scope .helpin-header-branding {
          font-size: 9px;
        }
      `}</style>
      <div className="widget-preview-scope rounded-lg border bg-muted/20">
        <div ref={mountRef} style={{ width: '100%', height: '100%' }} />
      </div>
    </>
  );
}
