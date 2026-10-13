import type {HistoricalField} from './historyContract';
export type ComparisonSnapshot={recordVersion:number;updatedAt:string;fields:HistoricalField[]};
export function parseComparisonSnapshot(_view:unknown,_record:unknown,_location:{appId:string;viewId:string;recordId:string},_actorId:string):ComparisonSnapshot|null{return null;}
