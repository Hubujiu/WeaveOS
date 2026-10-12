import type {HistoryChange,HistoryEvent} from './contracts';
export function validRecordHistory(data:unknown,pagination:unknown,pageSize:number):data is {items:HistoryEvent[]}{return false;}
export function recordHistoryValue(change:HistoryChange,side:'before'|'after'):string{return '';}
