import {validWorkflowEventDetail,type WorkflowEventDetail} from './historyContract';
export function validPersonalApprovalDetail(data:unknown,eventId:string,actorId:string):data is WorkflowEventDetail{
 if(!data||typeof data!=='object'||!('event'in data))return false;
 // Validate the entire original summary and basis first. The route and current
 // actor then constrain that valid event; a router-state object is not authority.
 const event=(data as WorkflowEventDetail).event;
 return validWorkflowEventDetail(data,event)&&event.id===eventId&&event.actorId===actorId&&event.outcome==='success'&&['agree','reject','return'].includes(event.action);
}
