import { randomBytes } from 'node:crypto';
export function recoverRuntime(o){
 if(!o.generation||['pause','restore','switchGeneration','resume'].some(key=>typeof o[key]!=='function'))throw new Error('Controlled recovery configuration required');
 o.pause();o.restore();
 const generation=`recovered_${randomBytes(16).toString('hex')}`;
 o.switchGeneration(generation);o.resume();return {generation};
}
