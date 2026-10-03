import type {RecordItem,RecordPage,RecordSort,RuntimeView,UUID} from './contracts';

export type RecordsPanelProps={view:RuntimeView;actorId:UUID;page:RecordPage;loading:boolean;error:string|null;hiddenColumnIds:readonly string[];columnWidths:Record<string,number>;columnOrder:string[];selectedRowIds:string[];onSelectionChange:(ids:string[])=>void;onPageChange:(page:number)=>void;onPageSizeChange:(size:number)=>void;onSortChange:(sort:RecordSort)=>void;onColumnWidthsChange:(widths:Record<string,number>)=>void;onColumnOrderChange:(order:string[])=>void;onOpenRecord:(record:RecordItem)=>void};
export function RecordsPanel(_props:RecordsPanelProps){return null;}
