import express from 'express';
import cors from 'cors';
import { WebSocketServer, WebSocket } from 'ws';

const app = express();
const PORT = 3456;

app.use(cors());
app.use(express.json());

const widgetSessions = new Map();
const conversations = new Map();
let sessionCounter = 1;

app.post('/v1/widget/session', (req, res) => {
  const { widget_key, email, name, userId, metadata } = req.body;
  
  const sessionToken = `session_${Date.now()}_${sessionCounter++}`;
  const conversationId = `conv_${Date.now()}`;
  
  widgetSessions.set(sessionToken, {
    widget_key,
    email,
    name,
    conversationId,
    createdAt: new Date().toISOString(),
  });

  conversations.set(conversationId, {
    id: conversationId,
    status: 'open',
    messages: [],
    customer: { email, name },
  });

  res.json({
    session_token: sessionToken,
    conversation_id: conversationId,
  });
});

app.get('/v1/widget/config', (req, res) => {
  const { key } = req.query;
  
  if (!key || key !== 'test-widget-key') {
    return res.status(404).json({ error: 'Widget not found' });
  }

  res.json({
    workspaceId: 'ws_test',
    branding: {
      primaryColor: '#6366f1',
      logoUrl: null,
      welcomeMessage: 'Hello! How can we help you today?',
      widgetPosition: 'bottom-right',
    },
    features: {
      aiEnabled: true,
      fileUploads: true,
      preChatForm: true,
      requirePhone: true,
      csatRating: true,
    },
  });
});

app.get('/v1/widget/conversations/:id/messages', (req, res) => {
  const { id } = req.params;
  const conversation = conversations.get(id);
  
  if (!conversation) {
    return res.status(404).json({ error: 'Conversation not found' });
  }

  res.json(conversation.messages);
});

app.post('/v1/widget/conversations/:id/messages', (req, res) => {
  const { id } = req.params;
  const { content, sender_type } = req.body;
  const conversation = conversations.get(id);
  
  if (!conversation) {
    return res.status(404).json({ error: 'Conversation not found' });
  }

  const message = {
    id: `msg_${Date.now()}`,
    conversationId: id,
    content,
    role: sender_type || 'customer',
    createdAt: new Date().toISOString(),
    isInternal: false,
  };

  conversation.messages.push(message);

  broadcastToWs(id, {
    entity: 'support_conversation_message',
    type: 'create',
    parent_id: id,
    entity_id: message.id,
    data: message,
  });

  res.json(message);
});

app.post('/v1/widget/conversations/:id/typing', (req, res) => {
  const { id } = req.params;
  const { is_typing } = req.body;

  broadcastToWs(id, {
    entity: 'typing',
    conversation_id: id,
    is_typing,
  });

  res.json({ success: true });
});

app.put('/v1/widget/messages/:id/csat', (req, res) => {
  const { id } = req.params;
  const { rating, feedback } = req.body;

  res.json({
    id,
    rating,
    feedback,
    submitted_at: new Date().toISOString(),
  });
});

app.post('/v1/widget/conversations/:id/transcript', (req, res) => {
  const { id } = req.params;
  const { email } = req.body;
  const conversation = conversations.get(id);

  if (!conversation) {
    return res.status(404).json({ error: 'Conversation not found' });
  }

  res.json({
    success: true,
    message: `Transcript sent to ${email}`,
  });
});

const wss = new WebSocketServer({ port: 3457 });
const wsClients = new Map();

wss.on('connection', (ws, req) => {
  const url = new URL(req.url || '', `http://localhost:${PORT}`);
  const sessionToken = url.searchParams.get('session_token');

  if (!sessionToken) {
    ws.close();
    return;
  }

  const session = widgetSessions.get(sessionToken);
  if (!session) {
    ws.close();
    return;
  }

  wsClients.set(session.conversationId, ws);

  ws.on('close', () => {
    wsClients.delete(session.conversationId);
  });
});

function broadcastToWs(conversationId: string, message: object) {
  const ws = wsClients.get(conversationId);
  if (ws && ws.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify(message));
  }
}

app.listen(PORT, () => {
  console.log(`Widget test server running at http://localhost:${PORT}`);
  console.log(`WebSocket server running at ws://localhost:3457`);
});

export { app, PORT };
