// Deterministic, isolated acceptance fixture. Never proxy requests to a provider.
import http from 'node:http';
import net from 'node:net';
const metrics = { completions: 0, embeddings: 0, mail: 0, credential_rejections: 0 };
const mails = [];
http.createServer(async (req, res) => {
  if (req.url === '/image.png') { res.setHeader('Content-Type', 'image/png'); return res.end(Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aZ1cAAAAASUVORK5CYII=', 'base64')); }
  res.setHeader('Content-Type', 'application/json');
  if (req.url === '/metrics') return res.end(JSON.stringify(metrics));
  if (req.url === '/mail') return res.end(JSON.stringify(mails));
  if (req.headers.authorization !== 'Bearer community-fixture-key') {
    metrics.credential_rejections++;
    res.writeHead(401); return res.end('{"error":{"message":"fixture credential missing"}}');
  }
  let raw = '';
  for await (const chunk of req) { raw += chunk; if (raw.length > 4e6) { res.writeHead(413); return res.end(); } }
  const input = JSON.parse(raw || '{}');
  if (req.url === '/v1/embeddings') {
    metrics.embeddings++;
    const items = Array.isArray(input.input) ? input.input : [input.input];
    return res.end(JSON.stringify({ object: 'list', model: input.model,
      data: items.map((_, index) => ({ object: 'embedding', index, embedding: Array.from({ length: 1536 }, (_, i) => i === 0 ? 1 : 0) })),
      usage: { prompt_tokens: 12, total_tokens: 12 } }));
  }
  if (req.url === '/v1/chat/completions') {
    metrics.completions++;
    const common = { id: `community-${metrics.completions}`, model: input.model, created: Math.floor(Date.now() / 1000) };
    const finish = input.tools?.some(tool => tool.function?.name === 'finish_turn');
    const toolCall = { index: 0, id: `finish-${metrics.completions}`, type: 'function', function: {
      name: 'finish_turn', arguments: JSON.stringify({ outcome: 'completed', summary: 'Community fixture answer.' }),
    } };
    if (input.stream) {
      res.setHeader('Content-Type', 'text/event-stream');
      for (const chunk of [
        { choices: [{ index: 0, delta: finish ? { role: 'assistant', tool_calls: [toolCall] } : { role: 'assistant', content: 'Community fixture answer.' }, finish_reason: null }] },
        { choices: [{ index: 0, delta: {}, finish_reason: finish ? 'tool_calls' : 'stop' }], usage: { prompt_tokens: 12, completion_tokens: 5, total_tokens: 17 } },
      ]) res.write(`data: ${JSON.stringify({ ...common, object: 'chat.completion.chunk', ...chunk })}\n\n`);
      return res.end('data: [DONE]\n\n');
    }
    return res.end(JSON.stringify({ ...common, object: 'chat.completion', choices: [{ index: 0, message: { role: 'assistant', content: 'Community fixture answer.' }, finish_reason: 'stop' }], usage: { prompt_tokens: 12, completion_tokens: 5, total_tokens: 17 } }));
  }
  res.writeHead(404); res.end('{}');
}).listen(8080, '0.0.0.0');
net.createServer(socket => {
  socket.write('220 community-fixture ESMTP\r\n');
  let buffer = '', data = false, message = '';
  socket.on('data', chunk => {
    buffer += chunk;
    while (buffer.includes('\r\n')) {
      const end = buffer.indexOf('\r\n'), line = buffer.slice(0, end); buffer = buffer.slice(end + 2);
      if (data) {
        if (line === '.') { data = false; mails.push(message); metrics.mail++; socket.write('250 queued\r\n'); }
        else message += line + '\r\n';
      } else if (/^EHLO|^HELO/.test(line)) socket.write('250 community-fixture\r\n');
      else if (line === 'DATA') { data = true; message = ''; socket.write('354 send message\r\n'); }
      else if (line === 'QUIT') socket.end('221 bye\r\n');
      else socket.write('250 ok\r\n');
    }
  });
}).listen(2525, '0.0.0.0');
