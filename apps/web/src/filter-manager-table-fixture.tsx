import {useState} from 'react';
import {createRoot} from 'react-dom/client';
import {Table} from './vendor/arca/components/motion/table';
import './vendor/arca/arca.css';
function Fixture(){
 const [hidden,setHidden]=useState<string[]>([]),[widths,setWidths]=useState({a:160,b:200,c:240}),[order,setOrder]=useState(['c','b','a']);
 return <><button onClick={()=>setHidden(v=>v.length?[]:['b'])}>切换乙列</button><output>{JSON.stringify({widths,order})}</output><Table ariaLabel="显隐保序" data={[{a:'甲数据',b:'乙数据',c:'丙数据'}]} columns={[{key:'a',header:'甲'},{key:'b',header:'乙'},{key:'c',header:'丙'}]} hiddenColumnIds={hidden} columnWidths={widths} onColumnWidthsChange={v=>setWidths(v as typeof widths)} columnOrder={order} onColumnOrderChange={setOrder} resizable reorderable height={300}/></>;
}
createRoot(document.getElementById('root')!).render(<Fixture/>);
