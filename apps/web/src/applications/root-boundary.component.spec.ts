import {test,expect} from '@playwright/test';
const actor='11111111-1111-4111-8111-111111111111';
const path='applications/22222222-2222-4222-8222-222222222222/forms/33333333-3333-4333-8333-333333333333/records';
test.beforeEach(async({page})=>{await page.goto('/src/applications/root-boundary-fixture.html');});

test('Root recovery owns an immutable deep snapshot of the sent packet',async({page})=>{
 const result=await page.evaluate(()=>{
  const {recovery}= (window as any).__rootApplicationBoundary;
  const packet={actorId:'actor-A',scope:'form-A',path:'applications/a/forms/v/records',method:'POST',expectedStatus:201,body:{operationId:'operation-A',values:{amount:'9007199254740993.01',choices:['A']}}};
  recovery.keepPacket(packet,true);
  packet.body.values.amount='0';packet.body.values.choices.push('B');
  const saved=recovery.getRecovery('actor-A','form-A');
  return {amount:saved.packet.body.values.amount,choices:saved.packet.body.values.choices,
   frozen:Object.isFrozen(saved.packet)&&Object.isFrozen(saved.packet.body.values)&&Object.isFrozen(saved.packet.body.values.choices),
   otherActor:recovery.getRecovery('actor-B','form-A')??null,
   otherScope:recovery.getRecovery('actor-A','form-B')??null};
 });
 expect(result).toEqual({amount:'9007199254740993.01',choices:['A'],frozen:true,otherActor:null,otherScope:null});
});

test('Root successful PATCH retains actor and exact JSON body',async({page})=>{
 let observed:any;
 await page.route('**/api/v1/'+path+'/record-A',async route=>{
  observed={method:route.request().method(),actor:route.request().headers()['x-expected-actor-id'],body:route.request().postDataJSON()};
  await route.fulfill({status:200,contentType:'application/json',body:JSON.stringify({code:'OK',data:{id:'record-A'},meta:null})});
 });
 const result=await page.evaluate(async({actor,path})=>(window as any).__rootApplicationBoundary.api.applicationApi(actor,path+'/record-A','PATCH',{operationId:'operation-A',changes:{amount:'1.20'}},undefined,200),{actor,path});
 expect(result).toEqual({id:'record-A'});
 expect(observed).toEqual({method:'PATCH',actor,body:{operationId:'operation-A',changes:{amount:'1.20'}}});
});

test('Root DELETE confirms only an exact empty 204',async({page})=>{
 await page.route('**/api/v1/**',route=>route.fulfill({status:204,body:''}));
 const result=await page.evaluate(async actor=>{
  const reply=await (window as any).__rootApplicationBoundary.api.applicationApiEnvelope(actor,'applications/a/forms/v/drafts/d?operationId=o&expectedDraftVersion=1','DELETE',undefined,undefined,204);
  return {empty:reply.data===undefined,meta:reply.meta};
 },actor);
 expect(result).toEqual({empty:true,meta:null});
});

test('Root DELETE unexpected successful status stays unconfirmed',async({page})=>{
 await page.route('**/api/v1/**',route=>route.fulfill({status:200,contentType:'application/json',body:JSON.stringify({code:'OK',data:{},meta:null})}));
 const result=await page.evaluate(async actor=>{
  try{await (window as any).__rootApplicationBoundary.api.applicationApi(actor,'applications/a/forms/v/drafts/d','DELETE',undefined,undefined,204);return {confirmed:true};}
  catch(error){const e=error as any;return {confirmed:false,status:e.status,unknown:e.unconfirmed};}
 },actor);
 expect(result).toEqual({confirmed:false,status:200,unknown:true});
});

for(const readOnly of [false,true]){
 test('Root network failure uncertainty follows '+(readOnly?'read-only search':'mutation'),async({page})=>{
  await page.route('**/api/v1/**',route=>route.abort('failed'));
  const result=await page.evaluate(async({actor,path,readOnly})=>{
   const api=(window as any).__rootApplicationBoundary.api;
   try{if(readOnly)await api.applicationReadPost(actor,path+'/search',{page:1,pageSize:20});else await api.applicationApi(actor,path,'POST',{operationId:'operation-A'},undefined,201);return null;}
   catch(error){const e=error as any;return {status:e.status,unknown:e.unconfirmed};}
  },{actor,path,readOnly});
  expect(result).toEqual({status:0,unknown:!readOnly});
 });
}

test('Root read-only POST does not grant mutation paths read semantics',async({page})=>{
 let requests=0;await page.route('**/api/v1/**',route=>{requests++;return route.abort();});
 const result=await page.evaluate(async({actor,path})=>{
  try{await (window as any).__rootApplicationBoundary.api.applicationReadPost(actor,path,{values:{}});return false;}
  catch(error){return {typeError:error instanceof TypeError,message:(error as Error).message};}
 },{actor,path});
 expect(result).toEqual({typeError:true,message:'Read-only POST is restricted to record search'});expect(requests).toBe(0);
});

test('Root malformed read-only search response does not create unknown write state',async({page})=>{
 await page.route('**/api/v1/**',route=>route.fulfill({status:200,contentType:'application/json',body:'{"code":"OK"}'}));
 const result=await page.evaluate(async({actor,path})=>{
  try{await (window as any).__rootApplicationBoundary.api.applicationReadPost(actor,path+'/search',{page:1,pageSize:20});return null;}
  catch(error){const e=error as any;return {unknown:e.unconfirmed};}
 },{actor,path});
 expect(result).toEqual({unknown:false});
});

for(const badPath of [
 'applications/22222222-2222-4222-8222-222222222222#fragment/forms/33333333-3333-4333-8333-333333333333/records/search',
 'applications/../forms/33333333-3333-4333-8333-333333333333/records/search',
 'applications/22222222-2222-4222-8222-222222222222/forms/%2e%2e/records/search',
]){
 test('Root read-only search rejects normalized path '+badPath,async({page})=>{
  let requests=0;await page.route('**/api/v1/**',route=>{requests++;return route.abort();});
  const result=await page.evaluate(async({actor,badPath})=>{
   try{await (window as any).__rootApplicationBoundary.api.applicationReadPost(actor,badPath,{page:1,pageSize:20});return null;}
   catch(error){return {typeError:error instanceof TypeError,message:(error as Error).message};}
  },{actor,badPath});
  expect(result).toEqual({typeError:true,message:'Read-only POST is restricted to record search'});
  expect(requests).toBe(0);
 });
}
test('Root read-only search HTTP 503 remains a read failure',async({page})=>{
 await page.route('**/api/v1/**',route=>route.fulfill({status:503,json:{code:'COMMON_SERVICE_UNAVAILABLE',data:null,meta:null}}));
 const result=await page.evaluate(async({actor,path})=>{
  try{await (window as any).__rootApplicationBoundary.api.applicationReadPost(actor,path+'/search',{page:1,pageSize:20});return null;}
  catch(error){const e=error as any;return {status:e.status,unknown:e.unconfirmed};}
 },{actor,path});
 expect(result).toEqual({status:503,unknown:false});
});
