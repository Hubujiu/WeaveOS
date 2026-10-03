import { open as realRedisObserver } from '../../../tests/acceptance/redis-observer.mjs';
export const open = options => realRedisObserver({...options,redisURL:'redis://127.0.0.1:26380/0',generation:'weaveos-v010-007-v030-011-status'});
