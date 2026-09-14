export function emailTextToHTML(value: string) {
  return value.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;').replace(/'/g, '&#039;').replace(/\n/g, '<br>');
}

export function withEmailSignature(html: string, signature?: string) {
  return signature?.trim() ? `${html}<p><br></p><div>${emailTextToHTML(signature)}</div>` : html;
}
