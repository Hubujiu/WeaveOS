import {readFileSync} from 'node:fs';
import {resolve} from 'node:path';
import {pathToFileURL} from 'node:url';
const prefix='github.com/Hubujiu/WeaveOS/services/bff/';
const packages=['cmd/bff','internal/apprecordservice','internal/appstructure'].map(p=>prefix+p).sort();
const target=/^TestRoot(?:NativeCatalog|RuntimeAdmission|Host|MainProcess)[A-Za-z0-9_]*$/;
const sha=value=>typeof value==='string'&&/^[a-f0-9]{40}$/.test(value);
const key=item=>item.package+'#'+item.name;
const invalid=()=>{throw new Error('Incomplete or invalid Go required-test evidence');};
function parse(input){
 if(typeof input!=='string'||!input.trim())return invalid();
 return input.split('\n').filter(line=>line.trim()).map(line=>{
  let value;try{value=JSON.parse(line);}catch{return invalid();}
  if(!value||typeof value!=='object'||Array.isArray(value)||typeof value.Action!=='string')return invalid();
  if(['fail','build-fail'].includes(value.Action))return invalid();
  return value;
 });
}
function successfulPackages(events){
 const passes=new Map();
 for(const event of events)if(!event.Test&&event.Action==='pass')passes.set(event.Package,(passes.get(event.Package)??0)+1);
 if(!packages.every(pkg=>passes.get(pkg)===1))return invalid();
}
export function discoverRequiredGoTests(input,sourceSha){
 if(!sha(sourceSha))return invalid();
 const events=parse(input);successfulPackages(events);
 const tests=[],seen=new Set();
 for(const event of events){
  if(event.Action!=='output'||!packages.includes(event.Package)||typeof event.Output!=='string')continue;
  const name=event.Output.trim();if(!target.test(name))continue;
  const item={package:event.Package,name};if(seen.has(key(item)))return invalid();seen.add(key(item));tests.push(item);
 }
 if(!packages.every(pkg=>tests.some(test=>test.package===pkg)))return invalid();
 tests.sort((a,b)=>key(a).localeCompare(key(b)));
 return {schemaVersion:1,sourceSha,packages:[...packages],tests};
}
export function verifyRequiredGoTests(manifest,input,sourceSha){
 if(!sha(sourceSha)||!manifest||manifest.schemaVersion!==1||manifest.sourceSha!==sourceSha||JSON.stringify(manifest.packages)!==JSON.stringify(packages)||!Array.isArray(manifest.tests)||!manifest.tests.length)return invalid();
 const expected=new Map();
 for(const item of manifest.tests){
  if(!item||!packages.includes(item.package)||!target.test(item.name)||expected.has(key(item)))return invalid();
  expected.set(key(item),{run:0,pass:0});
 }
 if(!packages.every(pkg=>manifest.tests.some(test=>test.package===pkg)))return invalid();
 const events=parse(input);successfulPackages(events);
 for(const event of events){
  if(typeof event.Test!=='string')continue;
  const name=event.Test.split('/')[0];if(!target.test(name))continue;
  const counts=expected.get(key({package:event.Package,name}));if(!counts)return invalid();
  if(event.Action==='skip')return invalid();
  if(event.Test!==name)continue;
  if(event.Action==='run')counts.run++;
  if(event.Action==='pass'){if(counts.run!==1)return invalid();counts.pass++;}
 }
 if([...expected.values()].some(counts=>counts.run!==1||counts.pass!==1))return invalid();
 return expected.size;
}
if(process.argv[1]&&import.meta.url===pathToFileURL(resolve(process.argv[1])).href){
 try{
  const [mode,...args]=process.argv.slice(2);
  if(mode==='discover'&&args.length===2)console.log(JSON.stringify(discoverRequiredGoTests(readFileSync(args[0],'utf8'),args[1])));
  else if(mode==='verify'&&args.length===3)console.log(`Verified ${verifyRequiredGoTests(JSON.parse(readFileSync(args[0],'utf8')),readFileSync(args[1],'utf8'),args[2])} independently discovered critical Go tests in the single full run.`);
  else throw new Error('Invalid Go evidence command');
 }catch(error){console.error(error.message);process.exitCode=1;}
}
