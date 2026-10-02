import {defineConfig} from '@playwright/test';
import base from './components.config';
export default defineConfig({...base,testMatch:'personnel*.component.spec.ts',projects:[base.projects![0]],workers:2,outputDir:'../../../../.work/q36-b2-legacy-results'});
