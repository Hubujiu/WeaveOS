// File admission only. V070's server preflight still validates the complete
// field/graph/reference contract and current authority before an import.
export function decodeTemplateFile(bytes:Uint8Array):Record<string,unknown>|null{
 if(!(bytes instanceof Uint8Array)||bytes.byteLength>1048576)return null;
 try{
  const text=new TextDecoder('utf-8',{fatal:true,ignoreBOM:true}).decode(bytes);
  let at=0;
  const whitespace=()=>{while(at<text.length&&/[\x20\t\r\n]/.test(text[at]))at++;};
  const invalid=():never=>{throw new SyntaxError('Invalid template JSON');};
  const string=():string=>{
   if(text[at]!=='"')return invalid();
   const start=at++;
   while(at<text.length){const character=text[at++];if(character==='\\'){at++;continue;}if(character==='"')return JSON.parse(text.slice(start,at)) as string;}
   return invalid();
  };
  const value=(depth:number):void=>{
   if(depth>32)invalid();
   whitespace();
   if(text[at]==='"'){string();return;}
   if(text[at]==='{'){
    at++;whitespace();const keys=new Set<string>();
    if(text[at]==='}'){at++;return;}
    for(;;){
     whitespace();const key=string();if(keys.has(key))invalid();keys.add(key);
     whitespace();if(text[at++]!==':')invalid();value(depth+1);whitespace();
     const separator=text[at++];if(separator==='}')return;if(separator!==',')invalid();
    }
   }
   if(text[at]==='['){
    at++;whitespace();if(text[at]===']'){at++;return;}
    for(;;){value(depth+1);whitespace();const separator=text[at++];if(separator===']')return;if(separator!==',')invalid();}
   }
   const start=at;
   while(at<text.length&&!/[\x20\t\r\n,\]}]/.test(text[at]))at++;
   if(start===at)invalid();
   // JSON.parse below validates the exact literal/number grammar.
  };
  value(0);whitespace();if(at!==text.length)return null;
  const result:unknown=JSON.parse(text);
  if(!result||typeof result!=='object'||Array.isArray(result))return null;
  const root=result as Record<string,unknown>,arrays=['directories','tables','forms','workflows','permissionGroups'];
  const keys=['format','version','application',...arrays];
  if(Object.keys(root).length!==keys.length||!keys.every(key=>Object.hasOwn(root,key))||root.format!=='weaveos.structure-template'||root.version!==1||!root.application||typeof root.application!=='object'||Array.isArray(root.application)||!arrays.every(key=>Array.isArray(root[key])))return null;
  return root;
 }catch{return null;}
}
