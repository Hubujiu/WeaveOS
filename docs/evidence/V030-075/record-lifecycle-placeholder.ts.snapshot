export type RecordLifecycleState={id:string;recordVersion:number;schemaVersion:number;deleted:boolean};
export type RecordLifecycleResult=RecordLifecycleState&{operationId:string};
export function validRecordLifecycleRequest(_resource:unknown,_request:unknown):boolean{return false;}
export function validRecordLifecycleState(_value:unknown,_recordId:string):_value is RecordLifecycleState{return false;}
export function validRecordLifecycleReceipt(_value:unknown,_operationId:string,_recordId:string,_recordVersion:number,_schemaVersion:number,_action:'deletion'|'restoration'):_value is RecordLifecycleResult{return false;}
