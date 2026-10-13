import test from 'node:test';import assert from 'node:assert/strict';import {readFile} from 'node:fs/promises';import {stripTypeScriptTypes} from 'node:module';
const data=s=>'data:text/javascript;base64,'+Buffer.from(s).toString('base64');async function load(u){let s=stripTypeScriptTypes(await readFile(u,'utf8'),{mode:'transform'});for(const m of [...s.matchAll(/from ['"]([^'"]+)['"]/g)])s=s.replace(m[0],"from '"+await load(new URL(m[1]+'.ts',u))+"'");return data(s);}
const {insertWorkflowNode,removeWorkflowNode}=await import(await load(new URL('../../apps/web/src/applications/workflows/graphEditing.ts',import.meta.url)));
const id=n=>'ecececec-ecec-4cec-8cec-'+String(n).padStart(12,'0'),start={id:id(1),kind:'start'},end={id:id(2),kind:'end'},edge={from:id(1),to:id(2)},base={version:1,nodes:[end,start],edges:[edge]};
const approval={id:id(3),kind:'approval',approval:{mode:'all',assigneeIds:[id(20),id(21)],editableFieldIds:[id(30)]}},condition={id:id(4),kind:'condition',condition:{operator:'or',children:[{fieldId:id(30),operator:'eq',value:'A'},{operator:'and',children:[{fieldId:id(31),operator:'gte',value:'9007199254740993.01'},{fieldId:id(32),operator:'eq',value:false}]}]}};
function frozen(x){if(x&&typeof x==='object'){Object.values(x).forEach(frozen);Object.freeze(x);}return x;}
test('V019 approval insertion changes exactly the selected edge and preserves configured IDs and input',()=>{
 const source=frozen(structuredClone(base)),node=frozen(structuredClone(approval)),result=insertWorkflowNode(source,edge,node);
 assert.deepEqual(result,{version:1,nodes:[end,start,approval],edges:[{from:id(1),to:id(3)},{from:id(3),to:id(2)}]});assert.deepEqual(source,base);assert.deepEqual(node,approval);
 result.nodes[2].approval.assigneeIds.push(id(22));assert.deepEqual(node,approval);
});
test('V019 condition insertion retains the exact AST and initially rejoins both branches at original target',()=>{
 assert.deepEqual(insertWorkflowNode(base,edge,condition),{version:1,nodes:[end,start,condition],edges:[{from:id(1),to:id(4)},{from:id(4),to:id(2),branch:'true'},{from:id(4),to:id(2),branch:'false'}]});
});
const branched={version:1,nodes:[end,condition,start],edges:[{from:id(1),to:id(4)},{from:id(4),to:id(2),branch:'true'},{from:id(4),to:id(2),branch:'false'}]};
for(const branch of ['true','false'])test('V019 branch-qualified insertion edits only '+branch+' path, not the same-target sibling',()=>{
 const result=insertWorkflowNode(branched,{from:id(4),to:id(2),branch},approval);assert.ok(result);assert.deepEqual(result.nodes,[end,condition,start,approval]);
 const sort=es=>es.map(e=>JSON.stringify(e)).sort();assert.deepEqual(sort(result.edges),sort([{from:id(1),to:id(4)},{from:id(4),to:id(3),branch},{from:id(3),to:id(2)},{from:id(4),to:id(2),branch:branch==='true'?'false':'true'}]));assert.deepEqual(branched.nodes,[end,condition,start]);
});
for(const[label,node]of[['duplicate id',{...approval,id:start.id}],['start',start],['end',{...end,id:id(99)}],['no assignees',{...approval,approval:{...approval.approval,assigneeIds:[]}}],['wrong mode',{...approval,approval:{...approval.approval,mode:'majority'}}],['empty condition',{...condition,condition:null}],['extra metadata',{...approval,name:'not execution data'}]])test('V019 insert rejects '+label+' without inventing defaults',()=>assert.equal(insertWorkflowNode(base,edge,node),null));
for(const[label,g,e]of[['missing edge',base,{from:id(1),to:id(99)}],['unqualified condition branch',branched,{from:id(4),to:id(2)}],['extra edge key',base,{...edge,order:1}],['invalid source graph',{...base,version:2},edge]])test('V019 insert rejects '+label,()=>assert.equal(insertWorkflowNode(g,e,approval),null));
function chain(n){const nodes=[start,...Array.from({length:n-2},(_,i)=>({...approval,id:id(100+i)})),end];return {version:1,nodes,edges:nodes.slice(1).map((node,i)=>({from:nodes[i].id,to:node.id}))};}
test('V019 insert permits the hundredth node without truncation',()=>{const graph=chain(99),result=insertWorkflowNode(graph,graph.edges[0],{...approval,id:id(999)});assert.equal(result?.nodes.length,100);assert.equal(result.edges.length,99);assert.equal(graph.nodes.length,99);});
test('V019 insert rejects the hundred-and-first node without modifying the graph',()=>{const graph=chain(100),copy=structuredClone(graph);assert.equal(insertWorkflowNode(graph,graph.edges[0],{...approval,id:id(999)}),null);assert.deepEqual(graph,copy);});

const line={version:1,nodes:[end,approval,start],edges:[{from:start.id,to:approval.id},{from:approval.id,to:end.id}]};
test('V019 remove approval reconnects only its incoming edge and preserves immutable input',()=>{
 const source=frozen(structuredClone(line));assert.deepEqual(removeWorkflowNode(source,approval.id),base);assert.deepEqual(source,line);
});
test('V019 remove approval shared by true and false preserves both separately qualified edges',()=>{
 const source={version:1,nodes:[start,condition,approval,end],edges:[{from:start.id,to:condition.id},{from:condition.id,to:approval.id,branch:'true'},{from:condition.id,to:approval.id,branch:'false'},{from:approval.id,to:end.id}]};
 assert.deepEqual(removeWorkflowNode(source,approval.id),{version:1,nodes:[start,condition,end],edges:[{from:start.id,to:condition.id},{from:condition.id,to:end.id,branch:'true'},{from:condition.id,to:end.id,branch:'false'}]});
});
test('V019 remove converged condition keeps its common successor instead of inventing a chosen branch',()=>{
 assert.deepEqual(removeWorkflowNode(branched,condition.id),base);
});
test('V019 removal of a converged nested condition preserves its incoming true branch and other path',()=>{
 const nested={...condition,id:id(40)},source={version:1,nodes:[start,condition,nested,approval,end],edges:[{from:start.id,to:condition.id},{from:condition.id,to:nested.id,branch:'true'},{from:condition.id,to:approval.id,branch:'false'},{from:nested.id,to:end.id,branch:'true'},{from:nested.id,to:end.id,branch:'false'},{from:approval.id,to:end.id}]};
 const next=removeWorkflowNode(source,nested.id);assert.deepEqual(next,{version:1,nodes:[start,condition,approval,end],edges:[{from:start.id,to:condition.id},{from:condition.id,to:end.id,branch:'true'},{from:condition.id,to:approval.id,branch:'false'},{from:approval.id,to:end.id}]});next.nodes[1].condition.children[0].value='changed';assert.equal(source.nodes[1].condition.children[0].value,'A');
});
test('V019 removal rejects divergent condition without dropping either downstream branch',()=>{
 const source={version:1,nodes:[start,condition,approval,end],edges:[{from:start.id,to:condition.id},{from:condition.id,to:approval.id,branch:'true'},{from:condition.id,to:end.id,branch:'false'},{from:approval.id,to:end.id}]},before=structuredClone(source);assert.equal(removeWorkflowNode(source,condition.id),null);assert.deepEqual(source,before);
});
for(const [name,nodeId,g]of[['start',start.id,line],['end',end.id,line],['missing',id(99),line],['noncanonical',approval.id.toUpperCase(),line],['invalid graph',approval.id,{...line,edges:[]}],['extra graph key',approval.id,{...line,extra:true}]])test('V019 removal rejects '+name+' without guessing repair',()=>assert.equal(removeWorkflowNode(g,nodeId),null));
