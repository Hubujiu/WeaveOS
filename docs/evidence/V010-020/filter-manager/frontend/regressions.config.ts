import {defineConfig} from '@playwright/test';
import base from './components.config';
export default defineConfig({...base,testMatch:['q36-b2.component.spec.ts','personnel*.component.spec.ts'],use:{...base.use,baseURL:'http://127.0.0.1:43123'},webServer:{...base.webServer,command:'pnpm --dir apps/web exec vite --host 127.0.0.1 --port 43123 --strictPort',url:'http://127.0.0.1:43123'}});
