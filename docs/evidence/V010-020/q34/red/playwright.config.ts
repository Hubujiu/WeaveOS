import {defineConfig} from '@playwright/test';
import base from './q32-full.config';
export default defineConfig({...base,expect:{timeout:1500},outputDir:'q34-results'});
