import type {RuntimeView} from './contracts';
import type {ResourceFilterField} from '../../QueryFilterState';
import type {BusinessColumn} from '../../TablePresetState';
export function presetDescriptors(_view:RuntimeView):{fields:ResourceFilterField[];columns:BusinessColumn[]}{return {fields:[],columns:[]};}
