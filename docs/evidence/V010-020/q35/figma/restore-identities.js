const page=await figma.getNodeByIdAsync("0:1");await figma.setCurrentPageAsync(page);
const ids=["163:3461","163:3462","163:3485","163:3542","163:3548","163:3557","159:460","279:1893","279:1895","279:1894"];
const ns=await Promise.all(ids.map(id=>figma.getNodeByIdAsync(id)));const [list,search,note,template,selected,templateDivider,main,toolbar,viewport,footer]=ns;
const textNodes=[...list.findAllWithCriteria({types:["TEXT"]}),...main.findAllWithCriteria({types:["TEXT"]})];
const fonts=[...new Map(textNodes.flatMap(t=>t.getStyledTextSegments(["fontName"]).map(s=>[JSON.stringify(s.fontName),s.fontName])).map(([k,v])=>[k,v])).values()];
await Promise.all(fonts.map(f=>figma.loadFontAsync(f)));
const created=[],mutated=[list.id,search.id,note.id],removed=[];
function removedTree(n){removed.push(n.id);if("children"in n)n.children.forEach(removedTree);}
list.insertChild(0,search);list.appendChild(note);
for(const n of [toolbar,viewport,footer]){removedTree(n);n.remove();}
const props=["layoutMode","primaryAxisAlignItems","counterAxisAlignItems","paddingTop","paddingRight","paddingBottom","paddingLeft","itemSpacing","clipsContent","cornerRadius","fills","strokes","strokeWeight","strokeAlign","strokeTopWeight","strokeRightWeight","strokeBottomWeight","strokeLeftWeight"];
for(const k of props)list[k]=JSON.parse(JSON.stringify(template[k]));
list.resize(352,698);list.layoutSizingHorizontal="FIXED";list.layoutSizingVertical="FILL";list.layoutAlign="STRETCH";
const bindings=template.boundVariables;const varIds=[...new Set(Object.values(bindings).filter(v=>v&&v.type==="VARIABLE_ALIAS").map(v=>v.id))];const fetched=await Promise.all(varIds.map(id=>figma.variables.getVariableByIdAsync(id)));const vars=new Map(fetched.map(v=>[v.id,v]));for(const [field,v]of Object.entries(bindings))if(v&&v.type==="VARIABLE_ALIAS")list.setBoundVariable(field,vars.get(v.id));
search.resize(318,40);search.layoutSizingHorizontal="FILL";search.layoutSizingVertical="FIXED";search.layoutAlign="STRETCH";
const entries=[["企业管理员","管理组织、成员和中央权限",1],["普通员工","基础成员身份",4],["部门负责人","组织中的负责人身份",1],["财务成员","财务岗位身份",1]];
const cards=[];for(let i=0;i<entries.length;i++){const [name,desc,count]=entries[i];const card=main.createInstance();list.insertChild(i+1,card);card.name="Identity / "+name;card.setProperties({"Name#159:21":name,"Description#159:22":desc,"Meta#159:23":count+" 位成员 · 0 项直接权限"});card.resize(318,108);card.layoutSizingHorizontal="FILL";card.layoutSizingVertical="FIXED";card.layoutAlign="STRETCH";if(i===0)card.fills=JSON.parse(JSON.stringify(selected.fills));created.push(card.id,...card.findAll(()=>true).map(n=>n.id));cards.push({id:card.id,name,memberCount:count,directPermissionCount:0,selected:i===0});}
const spacer=figma.createAutoLayout("VERTICAL",{name:"Flexible space"});list.insertChild(5,spacer);spacer.fills=[];spacer.resize(1,67);spacer.layoutSizingHorizontal="FIXED";spacer.layoutSizingVertical="FILL";spacer.layoutGrow=1;created.push(spacer.id);
const divider=templateDivider.clone();list.insertChild(6,divider);divider.name="Divider";divider.resize(318,1);divider.layoutSizingHorizontal="FILL";divider.layoutSizingVertical="FIXED";created.push(divider.id);
list.appendChild(note);note.textAutoResize="HEIGHT";note.resize(310,40);note.layoutSizingHorizontal="FIXED";note.layoutSizingVertical="HUG";note.layoutAlign="INHERIT";
return {createdNodeIds:created,mutatedNodeIds:mutated,deletedNodeIds:removed,retainedRoot:list.id,cards,dimensions:{width:list.width,height:list.height},detailRetained:"163:3486",templateUnmodified:"163:3542",children:list.children.map(n=>({id:n.id,name:n.name,width:n.width,height:n.height}))};
