import {test,expect} from '@playwright/test';
import {LeaveGuards} from './shell/leaveGuards';
import type {LeaveController,LeaveScope} from './forms';
const actorId='11111111-1111-4111-8111-111111111111',appId='22222222-2222-4222-8222-222222222222',viewId='33333333-3333-4333-8333-333333333333';
const a='44444444-4444-4444-8444-444444444444',b='55555555-5555-4555-8555-555555555555';
const controller=(name:string,calls:string[]):LeaveController=>({getStatus:()=> 'draft',prepareLeave:decision=>{calls.push(name+':'+decision);return {ok:true};}});
test('Root record drafts, saved records and persistent drafts have distinct leave scopes',()=>{
 const guards=new LeaveGuards(),calls:string[]=[];
 const scopes:LeaveScope[]=[
  {kind:'record',actorId,appId,viewId,clientDraftId:a},
  {kind:'record',actorId,appId,viewId,clientDraftId:b},
  {kind:'record',actorId,appId,viewId,recordId:a},
  {kind:'draft',actorId,appId,viewId,draftId:a},
 ];
 scopes.forEach((scope,index)=>guards.register(scope,controller(String(index),calls)));
 const snapshot=guards.snapshot(actorId);expect(snapshot).toHaveLength(4);
 expect(guards.prepare(snapshot)).toEqual({ok:true});
 expect(calls).toEqual(['0:discard','1:discard','2:discard','3:discard']);
});
test('Root old record-scope cleanup cannot remove its replacement',()=>{
 const guards=new LeaveGuards(),calls:string[]=[];
 const scope:LeaveScope={kind:'record',actorId,appId,viewId,clientDraftId:a};
 const old=guards.register(scope,controller('old',calls));
 guards.register(scope,controller('new',calls));old();
 expect(guards.snapshot(actorId)).toHaveLength(1);
 expect(guards.prepare(guards.snapshot(actorId))).toEqual({ok:true});expect(calls).toEqual(['new:discard']);
});
test('Root record leave guards remain actor-isolated',()=>{
 const guards=new LeaveGuards(),calls:string[]=[];
 guards.register({kind:'record',actorId,appId,viewId,clientDraftId:a},controller('A',calls));
 guards.register({kind:'record',actorId:b,appId,viewId,clientDraftId:a},controller('B',calls));
 expect(guards.snapshot(actorId)).toHaveLength(1);
 expect(guards.prepare(guards.snapshot(actorId))).toEqual({ok:true});expect(calls).toEqual(['A:discard']);
});
