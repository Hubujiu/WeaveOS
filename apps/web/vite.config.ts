import { defineConfig } from 'vite';
import tailwindcss from '@tailwindcss/vite';

const proxy = {
  '/api': 'http://127.0.0.1:8080',
  '/health': 'http://127.0.0.1:8080',
};
// Preview is a CI/development host, not the production web server.
export default defineConfig({ plugins: [tailwindcss()], server: { proxy }, preview: { proxy } });
