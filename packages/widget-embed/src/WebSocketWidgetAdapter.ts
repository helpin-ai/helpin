import type { WidgetAdapter, Message, CustomerInfo, WidgetConfig } from '@helpin/widget-core';

export class WebSocketWidgetAdapter implements WidgetAdapter {
  getMessages(_conversationId: string): Message[] {
    return [];
  }

  onMessagesUpdate(_cb: (messages: Message[]) => void): () => void {
    return () => {};
  }

  async sendMessage(_content: string, _attachments?: File[]): Promise<void> {}

  async startConversation(_customer: CustomerInfo): Promise<string> {
    return '';
  }

  markAsRead(_messageId: string): void {}

  sendTypingIndicator(_isTyping: boolean): void {}

  getConfig(): WidgetConfig {
    return {
      workspaceId: '',
      branding: {
        primaryColor: '#6366f1',
        welcomeMessage: 'Hi! How can we help?',
        widgetPosition: 'bottom-right',
      },
      features: {
        aiEnabled: true,
        fileUploads: true,
        preChatForm: false,
        requirePhone: false,
        csatRating: false,
      },
    };
  }
}
