import { expect, test, type Page } from '@playwright/test';
test.setTimeout(120_000);
async function setup(page: Page, existing = false) {
 const writes: Record<string, unknown>[] = [];
 const contacts = [{id:'alex',first_name:'Alex',last_name:'Smith'},{id:'sam',first_name:'Sam',last_name:'Lee'}];
 const companies = [{id:'acme',name:'Acme'},{id:'other',name:'Other Co'}];
 await page.route('**/api/**', async route => {
  const req=route.request(), path=new URL(req.url()).pathname;
  if(path.endsWith('/pipelines')) return route.fulfill({json:[{id:'secondary',name:'Secondary',is_default:false,stages:[]},{id:'sales',name:'Sales',is_default:true,stages:[{id:'lead',pipeline_id:'sales',name:'Lead',position:0,probability:20,stage_type:'open'},{id:'proposal',pipeline_id:'sales',name:'Proposal Sent',position:1,probability:80,stage_type:'open'}]}]});
  if(path.includes('assignable')) return route.fulfill({json:[{id:'member-1',user_id:'user-1',display_name:'Waqar',email:'waqar@example.com',status:'active'}]});
  if(path.endsWith('/associations')) {
   if(req.method()==='POST') return route.fulfill({json:{id:'assoc-new',...req.postDataJSON()}});
   return route.fulfill({json:path.includes('/alex/')?[{from_object_type:'contact',from_object_id:'alex',to_object_type:'company',to_object_id:'acme',association_label:'primary',linked_object_name:'Acme'}]:[]});
  }
  if(path.endsWith('/contacts')) {
   if(req.method()==='POST') { const body=req.postDataJSON();return route.fulfill({json:{id:'new-person',...body}}); }
   return route.fulfill({json:{data:contacts,total:contacts.length}});
  }
  if(path.endsWith('/companies')) {
   if(req.method()==='POST') return route.fulfill({json:{id:'new-company',...req.postDataJSON()}});
   return route.fulfill({json:{data:companies,total:companies.length}});
  }
  if(path.endsWith('/deals')) {
   if(req.method()==='POST') {const body=req.postDataJSON();writes.push(body);return route.fulfill({json:{id:'deal-new',display_id:'DEAL-42',...body}});}
   const hasDeals=existing || writes.length > 0;
   return route.fulfill({json:{data:hasDeals?[{id:'deal-1',name:'Existing deal',pipeline_id:'sales',stage_id:'lead',currency:'USD',custom_properties:{}}]:[],total:hasDeals?1:0}});
  }
  return route.fulfill({json:[]});
 });
 return writes;
}
async function pick(page:Page,trigger:string,name:string) {
 await page.getByRole('button',{name:trigger,exact:true}).click();
 await page.getByRole('option',{name,exact:true}).click();
}
test('inherits context and owner, generates editable name, saves primary and additional people',async({page})=>{
 const writes=await setup(page);
 await page.goto('/e2e/crm/harness/deal-creation.html?context');
 await expect(page.getByLabel('Stage',{exact:true})).toContainText('Proposal Sent');
 await expect(page.getByLabel('Owner',{exact:true})).toContainText('Waqar');
 await expect(page.getByLabel('Probability (%)')).toHaveValue('80');
 await pick(page,'Add contact','Alex Smith');
 await expect(page.getByLabel('Deal name')).toHaveValue('Acme — New deal');
 await page.getByLabel('Deal name').fill('Custom opportunity');
 await pick(page,'Add contact','Sam Lee');
 await page.getByRole('button',{name:'Make primary'}).click();
 await expect(page.getByLabel('Deal name')).toHaveValue('Custom opportunity');
 await page.getByLabel('Amount',{exact:true}).fill('1200');
 await page.getByLabel('Revenue type',{exact:true}).click();await page.getByRole('option',{name:'Annual',exact:true}).click();
 await page.getByLabel('Probability (%)').fill('45');
 await page.getByLabel('Stage',{exact:true}).click();await page.getByRole('option',{name:'Lead',exact:true}).click();
 await expect(page.getByLabel('Probability (%)')).toHaveValue('45');
 await page.getByRole('button',{name:'Create deal',exact:true}).click();
 await expect.poll(()=>writes.length).toBe(1);
 await expect(page.getByText('Deal created',{exact:true})).toBeVisible();
 await expect(page.getByRole('button',{name:'Copy Deal ID',exact:true})).toBeVisible();
 await expect(page.getByRole('button',{name:'Open',exact:true})).toBeVisible();
 expect(writes[0]).toMatchObject({company_id:'acme',contact_id:'sam',contact_ids:['alex'],name:'Custom opportunity',revenue_type:'annual',amount:1200,owner_member_id:'member-1',pipeline_id:'sales',stage_id:'lead',probability:45});
 await page.getByRole('button',{name:'Open',exact:true}).click();
 await expect(page.getByText('Opened deal deal-new',{exact:true})).toBeVisible();
 await page.getByRole('button',{name:'Reopen'}).click();await expect(page.getByLabel('Deal name')).toHaveValue('');
});
test('suggests company contacts without adding them and asks before replacing company',async({page})=>{
 await setup(page);await page.goto('/e2e/crm/harness/deal-creation.html');
 await pick(page,'Select company','Other Co');
 await expect(page.getByText('Primary',{exact:true})).toHaveCount(0);
 await pick(page,'Add contact','Alex Smith');
 await expect(page.getByRole('button',{name:'Keep Other Co'})).toBeVisible();
 await page.getByRole('button',{name:'Keep Other Co'}).click();
 await expect(page.getByLabel('Deal name')).toHaveValue('Other Co — New deal');
 await expect(page.getByText('Primary',{exact:true})).toBeVisible();
});
test('creates an inline company and retains the deal draft',async({page})=>{
 const writes=await setup(page);await page.goto('/e2e/crm/harness/deal-creation.html');
 await page.getByLabel('Deal name').fill('Preserved draft');
 await page.getByRole('button',{name:'Select company',exact:true}).click();
 await page.getByRole('combobox').last().fill('New venture');
 await page.getByRole('option',{name:'Add “New venture” as new company'}).click();
 await expect(page.getByRole('button',{name:'Create deal',exact:true})).toBeDisabled();
 await page.getByRole('button',{name:'Add company',exact:true}).click();
 await expect(page.getByLabel('Deal name')).toHaveValue('Preserved draft');
 await page.getByRole('button',{name:'Create deal',exact:true}).click();
 await expect.poll(()=>writes.length).toBe(1);expect(writes[0]).toMatchObject({company_id:'new-company',name:'Preserved draft'});
});
for(const existing of [false,true]) test(`view controls reflect workspace deal existence (${existing})`,async({page})=>{
 await setup(page,existing);await page.goto('/e2e/crm/harness/deal-creation.html?page');
 await expect(page.getByRole('heading',{name:'Deals',exact:true})).toBeVisible();
 if(existing) await expect(page.getByRole('banner',{name:'Deal view controls'})).toBeVisible();
 else {await expect(page.getByRole('button',{name:'Create deal',exact:true})).toBeVisible();await expect(page.getByRole('banner',{name:'Deal view controls'})).toHaveCount(0);}
});

test('fits all visible fields on mobile without horizontal scrolling', async ({page}) => {
 await page.setViewportSize({width:390,height:844});await setup(page);
 await page.goto('/e2e/crm/harness/deal-creation.html');
 await expect(page.getByRole('heading',{name:'Create deal',exact:true})).toBeVisible();
 await page.getByLabel('Probability (%)').scrollIntoViewIfNeeded();
 expect(await page.getByRole('dialog').evaluate(el => el.scrollWidth <= el.clientWidth)).toBe(true);
 await page.screenshot({path:'/tmp/helpin-deal-create-mobile.png'});
});
test('new contacts inherit company and retry linking without duplicate creation', async ({page}) => {
 const writes=await setup(page);let contactCreates=0, links=0;
 page.on('request',request=>{if(request.method()==='POST' && new URL(request.url()).pathname.endsWith('/contacts')) contactCreates++;});
 await page.route('**/api/crm/associations?**', route => {
  if(route.request().method() !== 'POST') return route.fallback();
  links++;
  if(links===1) return route.fulfill({status:500,json:{error:'Link temporarily unavailable'}});
  expect(route.request().postDataJSON()).toMatchObject({from_object_id:'new-person',to_object_id:'acme',association_label:'primary'});
  return route.fulfill({json:{id:'linked',...route.request().postDataJSON()}});
 });
 await page.goto('/e2e/crm/harness/deal-creation.html');
 await pick(page,'Select company','Acme');
 await page.getByRole('button',{name:'Add contact',exact:true}).click();
 await page.getByRole('combobox').last().fill('Robin Jones');
 await page.getByRole('option',{name:'Add “Robin Jones” as new contact'}).click();
 await page.getByLabel('Contact email').fill('robin@example.com');
 await page.getByRole('button',{name:'Add contact',exact:true}).click();
 await expect(page.getByRole('button',{name:'Retry linking'})).toBeVisible();
 await expect(page.getByRole('button',{name:'Create deal',exact:true})).toBeDisabled();
 await page.getByRole('button',{name:'Retry linking'}).click();
 await expect(page.getByText('Primary',{exact:true})).toBeVisible();
 await page.getByRole('button',{name:'Create deal',exact:true}).click();
 await expect.poll(()=>writes.length).toBe(1);expect(contactCreates).toBe(1);expect(links).toBe(2);
 expect(writes[0]).toMatchObject({company_id:'acme',contact_id:'new-person'});
});
