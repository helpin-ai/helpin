import { FunctionComponent } from 'preact';
import type { ActiveTeammate, Conversation, WidgetConfig } from '../types';
import { MessageSquareIcon, CircleHelpIcon, ChevronRightIcon } from './icons';
import { timeAgo } from '../utils';

interface HomeViewProps {
  config: WidgetConfig;
  teammates?: ActiveTeammate[];
  recentConversation?: Conversation;
  onSendMessage?: (content: string) => void;
  onNavigate: (view: 'conversation' | 'messages' | 'help') => void;
  onSelectConversation?: (conversationId: string) => void;
}

export const HomeView: FunctionComponent<HomeViewProps> = ({
  config,
  teammates = [],
  recentConversation,
  onNavigate,
  onSelectConversation,
}) => {
  const brandColor = config.branding?.primaryColor || '#6366f1';
  const logoUrl = config.branding?.logoUrl;
  const workspaceName = config.workspaceName || 'Support';
  const aiFirst = Boolean(config.features?.aiFirst);
  const replyTimeText = config.availability?.replyTimeText || 'We typically reply in a few minutes';
  const primaryActionTitle = aiFirst ? 'Ask a question' : 'Send us a message';
  const primaryActionDescription = aiFirst
    ? 'Get an instant answer from our AI assistant'
    : replyTimeText;

  const visitorFirstName = config.visitorName?.trim().split(/\s+/)[0];

  return (
    <div className="helpin-home-view">
      {/* Gradient header area */}
      <div className="helpin-home-header" style={{ background: `linear-gradient(135deg, ${brandColor}, ${brandColor}88)` }}>
        <div className="helpin-home-header-row">
          {logoUrl ? (
            <img src={logoUrl} alt={workspaceName} className="helpin-home-logo" />
          ) : (
            <div className="helpin-home-logo-placeholder">
              <span>{workspaceName.charAt(0).toUpperCase()}</span>
            </div>
          )}

          {teammates.length > 0 && (
            <div className="helpin-home-header-team">
              {teammates.map((teammate, index) => (
                <div
                  key={teammate.userId || `${teammate.name}-${index}`}
                  className="helpin-home-header-presence helpin-avatar-tooltip"
                  aria-label={`${teammate.name} is ${teammate.status || 'online'}`}
                  data-tooltip={teammate.name}
                >
                  <div className="helpin-home-teammate-avatar-wrap">
                    {teammate.avatarUrl ? (
                      <img src={teammate.avatarUrl} alt={teammate.name} className="helpin-home-teammate-avatar" />
                    ) : (
                      <div className="helpin-home-teammate-avatar helpin-home-teammate-avatar--placeholder">
                        <span>{teammate.name.charAt(0).toUpperCase()}</span>
                      </div>
                    )}
                    <span
                      className={`helpin-presence-dot helpin-presence-dot--${teammate.status || 'online'}`}
                      aria-label={`${teammate.name} is ${teammate.status || 'online'}`}
                    />
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>

        <h2 className="helpin-home-welcome helpin-home-welcome--header">
          {visitorFirstName && <span className="helpin-home-greeting-name">Hello {visitorFirstName}.</span>}
          <span>How can we help?</span>
        </h2>
      </div>

      {/* Action cards */}
      <div className="helpin-home-content">
        {/* Quick action cards */}
        <div className="helpin-home-actions">
          <button className="helpin-home-action" onClick={() => onNavigate('conversation')}>
            <MessageSquareIcon size={18} />
            <div className="helpin-home-action-text">
              <span className="helpin-home-action-title">{primaryActionTitle}</span>
              <span className="helpin-home-action-desc">{primaryActionDescription}</span>
            </div>
            <ChevronRightIcon size={16} class="helpin-home-action-arrow" />
          </button>

          <button className="helpin-home-action" onClick={() => onNavigate('help')}>
            <CircleHelpIcon size={18} />
            <div className="helpin-home-action-text">
              <span className="helpin-home-action-title">Help center</span>
              <span className="helpin-home-action-desc">Find answers to common questions</span>
            </div>
            <ChevronRightIcon size={16} class="helpin-home-action-arrow" />
          </button>
        </div>

        {/* Recent message */}
        {recentConversation?.lastMessage && (
          <div className="helpin-home-recent">
            <span className="helpin-home-recent-label">Recent message</span>
            <button
              className="helpin-conversation-item"
              onClick={() => onSelectConversation?.(recentConversation.id)}
            >
              <div className="helpin-conversation-item-avatar-wrap">
                {recentConversation.activeTeammate?.avatarUrl ? (
                  <img
                    src={recentConversation.activeTeammate.avatarUrl}
                    alt={recentConversation.activeTeammate.name}
                    className="helpin-conversation-item-avatar-img"
                  />
                ) : (
                  <div className="helpin-conversation-item-avatar" style={{ backgroundColor: brandColor }}>
                    {recentConversation.activeTeammate?.name
                      ? recentConversation.activeTeammate.name.charAt(0).toUpperCase()
                      : <MessageSquareIcon size={14} />}
                  </div>
                )}
              </div>
              <div className="helpin-conversation-item-content">
                <span className={`helpin-conversation-item-preview${recentConversation.unreadCount ? ' helpin-conversation-item-preview--unread' : ''}`}>
                  {recentConversation.lastMessage.length > 50
                    ? recentConversation.lastMessage.slice(0, 50) + '...'
                    : recentConversation.lastMessage}
                </span>
                <span className="helpin-conversation-item-meta">
                  {recentConversation.activeTeammate?.name || 'Agent'}
                  {recentConversation.lastMessageAt && ` \u00B7 ${timeAgo(recentConversation.lastMessageAt)}`}
                </span>
              </div>
              {recentConversation.unreadCount ? (
                <span className="helpin-conversation-item-badge">
                  {recentConversation.unreadCount > 99 ? '99+' : recentConversation.unreadCount}
                </span>
              ) : (
                <ChevronRightIcon size={16} class="helpin-conversation-item-arrow" />
              )}
            </button>
          </div>
        )}
      </div>
    </div>
  );
};
