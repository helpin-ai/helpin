import {expect,test,type Page} from '@playwright/test';
import {installSupportAppMocks,CONVERSATION_ID,WORKSPACE_SLUG,SUPPORT_MESSAGES} from '../fixtures/supportE2E';
async function setup(page:Page,options:{fail?:boolean;own?:boolean}={}) {
 await installSupportAppMocks(page);
 let note={...SUPPORT_MESSAGES[0],id:'note-own',sender_type:'user',sender_user_id:options.own===false?'user-a':'user-b',sender_display_name:'Bob Agent',message_type:'reply',is_internal:true,content:'Checking the **reset link** with engineering.',metadata:'{}',updated_at:'2026-03-27T20:00:00Z',attachments:[{id:'file-kept',file_name:'details.txt',file_type:'text/plain',file_size:20,url:'https://example.com/details.txt',file_key:'details'}]};
 const writes:Record<string,unknown>[]=[];let sends=0;
 await page.route(`**/api/support/inbox/conversations/${CONVERSATION_ID}/messages?**`,route=>{
  if(route.request().method()==='POST'){sends++;return route.fulfill({json:{...note,id:'new'}})}
  return route.fulfill({json:[...SUPPORT_MESSAGES,note]});
 });
 await page.route(`**/api/support/inbox/conversations/${CONVERSATION_ID}/messages/note-own?**`,route=>{
  const payload=route.request().postDataJSON();writes.push(payload);
  if(options.fail)return route.fulfill({status:409,json:{error:'This note changed; reopen it to edit the latest version'}});
  note={...note,content:payload.content,metadata:JSON.stringify({edited_at:new Date().toISOString()}),updated_at:new Date().toISOString()};return route.fulfill({json:note});
 });
 await page.route(`**/api/support/inbox/conversations/${CONVERSATION_ID}/message-pages?**`, route => route.fulfill({json:{data:[...SUPPORT_MESSAGES,note],has_more:false}}));
 await page.goto(`/w/${WORKSPACE_SLUG}/support/${CONVERSATION_ID}`);
 await expect(page.locator('[data-support-reply-composer]:visible [contenteditable=true]')).toBeVisible({timeout:90000});return {writes,sends:()=>sends};
}
async function editNote(page:Page){const note=page.locator('[data-support-internal-note]').first();await note.hover();await note.locator('..').getByRole('button',{name:'Message actions'}).click();await page.getByRole('menuitem',{name:'Edit note',exact:true}).click();return page.locator('[data-support-reply-composer]:visible');}
test('edit saves in place, keeps attachments, and restores the reply draft',async({page})=>{
 test.setTimeout(120000);const {writes,sends}=await setup(page);await page.locator('[data-support-reply-composer]:visible [contenteditable=true]').fill('My unsent reply');
 const composer=await editNote(page);await expect(composer.getByText('Editing note',{exact:true})).toBeVisible();await expect(composer.locator('[contenteditable=true]')).toBeFocused();await expect(composer.getByRole('button',{name:'Save changes',exact:true})).toBeDisabled();await expect(composer).toContainText('details.txt');
 if (process.env.NOTE_EDIT_REVIEW) await page.screenshot({path:'/tmp/helpin-note-editor-light.png'});
 await composer.locator('[contenteditable=true]').fill('The reset link expires in 30 minutes.');await composer.getByRole('button',{name:'Save changes',exact:true}).click();
 await expect(page.locator('[data-support-internal-note]')).toHaveCount(1);await expect(page.locator('[data-support-internal-note]')).toContainText('Edited');await expect(page.locator('[data-support-reply-composer]:visible [contenteditable=true]')).toHaveText('My unsent reply');
 expect(writes).toHaveLength(1);expect(writes[0]).toMatchObject({content:'The reset link expires in 30 minutes.',attachment_ids:['file-kept'],expected_updated_at:'2026-03-27T20:00:00Z'});expect(sends()).toBe(0);
});
test('cancel preserves the original note and note draft',async({page})=>{
 test.setTimeout(120000);const {writes}=await setup(page);const normal=page.locator('[data-support-reply-composer]:visible');await normal.getByRole('button',{name:'Note',exact:true}).click();await normal.locator('[contenteditable=true]').fill('A new note draft');
 const composer=await editNote(page);await composer.locator('[contenteditable=true]').fill('Discard this edit');await composer.getByRole('button',{name:'Cancel',exact:true}).click();await expect(page.locator('[data-support-internal-note]')).toContainText('Checking the reset link');await expect(page.locator('[data-support-reply-composer]:visible [contenteditable=true]')).toHaveText('A new note draft');expect(writes).toHaveLength(0);
});
test('failed saves keep edited text and do not send a reply',async({page})=>{
 test.setTimeout(120000);const {writes,sends}=await setup(page,{fail:true});const composer=await editNote(page);await composer.locator('[contenteditable=true]').fill('Keep this unsaved edit');await composer.locator('[contenteditable=true]').press('Control+Enter');await expect.poll(()=>writes.length).toBe(1);await expect(composer.locator('[contenteditable=true]')).toHaveText('Keep this unsaved edit');await expect(composer.getByText('Editing note',{exact:true})).toBeVisible();expect(sends()).toBe(0);
});
test('another teammate’s note has no edit action',async({page})=>{
 test.setTimeout(120000);await setup(page,{own:false});const note=page.locator('[data-support-internal-note]');await note.hover();await note.locator('..').getByRole('button',{name:'Message actions'}).click();await expect(page.getByRole('menuitem',{name:'Edit note',exact:true})).toHaveCount(0);
});
for (const width of [390,1440]) {
 test(`wide customer bubbles keep their action button inside the thread (${width}px)`,async({page})=>{
  test.setTimeout(120000);await page.setViewportSize({width,height:950});await installSupportAppMocks(page);
  const messages=[
   {...SUPPORT_MESSAGES[0],id:'wide-email',via_channel:'email',html_body:'<table width="1000"><tr><td>This is a wide customer email with a fixed-width table.</td></tr></table>',content:'A customer email'},
   {...SUPPORT_MESSAGES[0],id:'wide-table',created_at:'2026-03-27T19:57:00Z',content:'| Column one | Column two | Column three |\n| --- | --- | --- |\n| Very long customer detail repeated repeatedly here | Another long detail from the customer | More details here |'},
   {...SUPPORT_MESSAGES[0],id:'wide-text',created_at:'2026-03-27T19:58:00Z',content:'A long customer message with plenty of text to fill the row. '.repeat(6)},
  ];
  await page.route(`**/api/support/inbox/conversations/${CONVERSATION_ID}/message-pages?**`,route=>route.fulfill({json:{data:messages,has_more:false}}));
  await page.goto(`/w/${WORKSPACE_SLUG}/support/${CONVERSATION_ID}`);
  for (const message of messages) {
   const row=page.locator(`[data-support-message-id="${message.id}"]:visible`);
   await expect(row).toBeVisible({timeout:90000});await row.hover();
   const box=await row.boundingBox();const bubble=await row.locator('[data-slot="support-message-bubble"]').boundingBox();const button=await row.getByRole('button',{name:'Message actions'}).boundingBox();
   expect(bubble!.x+bubble!.width).toBeLessThanOrEqual(box!.x+box!.width - await page.evaluate(() => parseFloat(getComputedStyle(document.documentElement).fontSize) * 2.25) + 0.5);
   expect(button!.x+button!.width).toBeLessThanOrEqual(box!.x+box!.width);
   expect(button!.x).toBeGreaterThanOrEqual(bubble!.x+bubble!.width);
   await row.getByRole('button',{name:'Message actions'}).click();await expect(page.getByRole('menuitem',{name:'Copy',exact:true})).toBeVisible();await page.keyboard.press('Escape');
  }
 });
}
test('editing keeps an in-flight reply attachment and never previews private text', async ({ page }) => {
 test.setTimeout(120000);
 await setup(page);
 let releaseUpload!: () => void;
 const uploadReady = new Promise<void>(resolve => { releaseUpload = resolve; });
 await page.route(`**/api/support/inbox/conversations/${CONVERSATION_ID}/attachments?**`, route => route.fulfill({json:{attachment:{id:'draft-file'},upload_url:'https://example.com/note-test-upload',public_url:''}}));
 await page.route('https://example.com/note-test-upload', async route => { await uploadReady; await route.fulfill({status:200}); });
 await page.route('**/api/support/inbox/attachments/draft-file/confirm?**', route => route.fulfill({json:{message:'confirmed'}}));
 const normal = page.locator('[data-support-reply-composer]:visible');
 await normal.locator('[contenteditable=true]').fill('Unsent public reply');
 await normal.locator('input[type=file]').setInputFiles({name:'reply-draft.txt',mimeType:'text/plain',buffer:Buffer.from('Draft attachment')});
 await expect(normal).toContainText('reply-draft.txt');
 const editing = await editNote(page);
 await page.evaluate(() => window.__supportE2E.clearSentMessages());
 await editing.locator('[contenteditable=true]').fill('Private internal update that must not reach the visitor');
 releaseUpload();
 await expect(editing).not.toContainText('reply-draft.txt');
 await editing.getByRole('button',{name:'Cancel',exact:true}).click();
 const restored = page.locator('[data-support-reply-composer]:visible');
 await expect(restored).toContainText('reply-draft.txt');
 await expect(restored.locator('[contenteditable=true]')).toHaveText('Unsent public reply');
 const events = await page.evaluate(() => window.__supportE2E.getSentMessages());
 expect(JSON.stringify(events)).not.toContain('Private internal update');
});
