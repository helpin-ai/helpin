import { test, expect, type Page } from '@playwright/test';
test.setTimeout(120_000);
async function open(page:Page,query='') {
 await page.route('**/api/**', route=>route.fulfill({json:{data:[]}}));
 await page.goto(`/e2e/crm/harness/email-compose.html${query}`);
 await expect(page.locator('.tiptap')).toBeVisible();
}
async function write(page:Page) {
 await page.getByRole('textbox',{name:'Subject',exact:true}).fill('Renewal next steps');
 await page.locator('.tiptap').fill('Hello Amna, here is our proposal.');
}
test('sends recipient, visible signature and deal context once with keyboard',async({page})=>{
 await open(page);
 const sent:Record<string,unknown>[]=[];
 await page.route('**/api/crm/email/send?*',async route=>{sent.push(route.request().postDataJSON());await new Promise(resolve=>setTimeout(resolve,500));await route.fulfill({json:{data:{id:'sent'}}});});
 await write(page);
 await page.locator('.tiptap').press('Control+Enter');
 await page.locator('.tiptap').press('Control+Enter');
 await expect(page.getByRole('dialog')).not.toBeVisible();
 expect(sent).toHaveLength(1);
 expect(sent[0]).toMatchObject({account_id:'mailbox',to:['amna@contentstudio.io'],deal_id:'deal-1',subject:'Renewal next steps'});
 expect(sent[0].body_html).toContain('Waqar Azeem');
 await page.getByText('Compose another',{exact:true}).click();
 await expect(page.getByText('second@example.com',{exact:true})).toBeVisible();
 await expect(page.getByRole('textbox',{name:'Subject',exact:true})).toHaveValue('');
 await expect(page.locator('.tiptap')).toHaveText('');
});
test('blocks malformed recipients and commits typed Cc before send',async({page})=>{
 await open(page);await write(page);
 await page.getByRole('textbox',{name:'To',exact:true}).fill('not-an-email');
 await expect(page.getByRole('button',{name:'Send',exact:true})).toBeDisabled();
 await page.getByRole('textbox',{name:'To',exact:true}).fill('buyer@example.com; reviewer@example.com');
 await page.getByRole('button',{name:'Cc',exact:true}).click();
 await page.getByRole('textbox',{name:'Cc',exact:true}).fill('finance@example.com');
 const request=page.waitForRequest('**/api/crm/email/send?*');
 await page.route('**/api/crm/email/send?*',route=>route.fulfill({json:{data:{id:'sent'}}}));
 await page.getByRole('button',{name:'Send',exact:true}).click();
 const payload=(await request).postDataJSON();
 expect(payload.to).toEqual(['amna@contentstudio.io','buyer@example.com','reviewer@example.com']);
 expect(payload.cc).toEqual(['finance@example.com']);
});
test('protects a draft on Escape and preserves it after failed delivery',async({page})=>{
 await open(page);await write(page);
 await page.keyboard.press('Escape');
 await expect(page.getByRole('alertdialog')).toBeVisible();
 await page.getByRole('button',{name:'Keep writing'}).click();
 await expect(page.locator('.tiptap')).toContainText('Hello Amna');
 await page.route('**/api/crm/email/send?*',route=>route.fulfill({status:400,json:{error:'Mailbox unavailable'}}));
 await page.getByRole('button',{name:'Send',exact:true}).click();
 await expect(page.getByText('Mailbox unavailable')).toBeVisible();
 await expect(page.locator('.tiptap')).toContainText('Hello Amna');
 await expect(page.getByRole('textbox',{name:'Subject',exact:true})).toHaveValue('Renewal next steps');
});
test('changes signature with sender and allows omission',async({page})=>{
 await open(page);await write(page);
 await page.getByRole('combobox',{name:'From'}).click();
 await page.getByRole('option',{name:'other@example.com'}).click();
 await expect(page.getByText('Other sender',{exact:true})).toBeVisible();
 await page.getByRole('checkbox',{name:'Include signature'}).uncheck();
 const request=page.waitForRequest('**/api/crm/email/send?*');
 await page.route('**/api/crm/email/send?*',route=>route.fulfill({json:{data:{id:'sent'}}}));
 await page.getByRole('button',{name:'Send',exact:true}).click();
 const payload=(await request).postDataJSON();
 expect(payload.account_id).toBe('other');expect(payload.body_html).not.toContain('Other sender');
});
test('shows a direct mailbox setup action',async({page})=>{
 await open(page,'?empty');
 await expect(page.getByRole('link',{name:'Connect mailbox'})).toHaveAttribute('href','/w/email-test/settings/crm-email');
 await expect(page.getByRole('button',{name:'Send',exact:true})).toBeDisabled();
});
test('signature settings save and can be cleared',async({page})=>{
 await page.route('**/api/**',route=>route.fulfill({json:{data:{saved:true}}}));
 await page.goto('/e2e/crm/harness/email-compose.html?settings');
 await page.getByText('Email signature',{exact:true}).click();
 await page.getByRole('textbox',{name:'Signature for waqar@contentstudio.io'}).fill('');
 const request=page.waitForRequest('**/signature?*');
 await page.getByRole('button',{name:'Save signature'}).click();
 expect((await request).postDataJSON()).toEqual({signature:''});
 await expect(page.getByText('Signature saved',{exact:true})).toBeVisible();
});
test('reply keyboard sends once and retains content on failure',async({page})=>{
 await open(page,'?reply');
 let calls=0;
 let release!: () => void;
 const pending = new Promise<void>(resolve => { release = resolve; });
 await page.route('**/mock-reply',async route=>{calls++;await pending;await route.fulfill({status:400,json:{error:'Unavailable'}});});
 await page.locator('.tiptap').press('Control+Enter');expect(calls).toBe(0);
 await page.locator('.tiptap').fill('Thanks, Amna.');
 await page.locator('.tiptap').press('Control+Enter');
 await expect.poll(() => calls).toBe(1);
 await page.keyboard.press('Control+Enter');
 expect(calls).toBe(1);
 release();
 await expect(page.getByRole('button',{name:'Send reply'})).toBeEnabled();
 expect(calls).toBe(1);await expect(page.locator('.tiptap')).toContainText('Thanks, Amna.');
});
for(const mode of ['light','dark','narrow']) test(`composer visual ${mode}`,async({page})=>{
 if(mode==='narrow')await page.setViewportSize({width:390,height:844});
 await open(page,mode==='dark'?'?dark':'');await write(page);
 const dialog=page.getByRole('dialog');
 const bounds=await dialog.boundingBox();expect(bounds!.width).toBeLessThanOrEqual(page.viewportSize()!.width);
 await expect(page.getByRole('button',{name:'Send',exact:true})).toBeInViewport();
 await page.screenshot({path:`/tmp/crm-email-${mode}.png`,fullPage:true});
});
