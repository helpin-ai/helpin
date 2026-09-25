import { test, expect } from '@playwright/test';
for (const mode of ['light', 'dark', 'narrow']) test(`compact AI composer ${mode}`, async ({page}) => {
  if (mode === 'narrow') await page.setViewportSize({width:390,height:600});
  const profiles = [
    {id:'personal',name:'Alpha personal',scope:'personal'},
    {id:'fast',name:'Fast',scope:'workspace'},
    {id:'balanced',name:'Balanced',scope:'workspace'},
    {id:'research',name:'Deep research',scope:'workspace'},
  ].map(p=>({...p,workspace_id:'ws',user_id:p.scope==='personal'?'me':null,revision:1,primary:{connection_id:'private-connection',model:{provider:'openai',model:'private-model',controls:{}}},fallback:null}));
  await page.route('**/api/ai-profiles/?*',route=>route.fulfill({json:profiles}));
  await page.route('**/api/ai-settings?*',route=>route.fulfill({json:{default_profile_id:'balanced'}}));
  await page.goto(`/e2e/crm/harness/ai-composer.html${mode==='dark'?'?dark':''}`);
  const picker=page.getByRole('combobox',{name:'Change AI profile: Balanced'});
  await expect(picker).toHaveText('Balanced');
  await expect(page.getByText('private-model')).toHaveCount(0);
  const input=await page.getByRole('textbox').boundingBox();
  const actions=await page.locator('[data-composer-actions]').boundingBox();
  expect(actions!.y).toBeGreaterThanOrEqual(input!.y+input!.height);
  const send=await page.getByTitle('Send',{exact:true}).boundingBox();
  const selection=await picker.boundingBox();
  expect(send!.x-selection!.x-selection!.width).toBeLessThan(12);
  await picker.click();
  const rows=page.locator('[cmdk-item]');
  await expect(rows).toHaveCount(4);
  expect(await rows.evaluateAll(els=>els.map(el=>el.getAttribute('data-value')))).toEqual(['balanced','research','fast','personal']);
  await expect(rows.first()).toHaveText('BalancedDefaultWorkspace');
  await page.screenshot({path:`/tmp/helpin-ai-composer-${mode}.png`});
  await page.locator('[cmdk-item][data-value="personal"]').click();
  await expect(page.getByRole('combobox',{name:'Change AI profile: Alpha personal'})).toHaveText('Alpha personal');
  await page.getByRole('combobox').click();
  await page.locator('[cmdk-item][data-value="balanced"]').click();
  await expect(page.getByRole('combobox')).toHaveText('Balanced');
  await page.getByRole('textbox').fill('Plan the launch');
  await expect(page.getByTitle('Send',{exact:true})).toBeEnabled();
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
});
