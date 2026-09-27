export function open(options?: { baseURL?: string; redisURL?: string; generation?: string }): Promise<{
  storage: 'isolated-redis';
  expireSession(cookie: string): Promise<void>;
  close(): Promise<void>;
}>;
