import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter } from 'react-router';
import { App } from './App';
import './style.css';

const root = document.getElementById('root');
if (!root) throw new Error('Missing application mount point');
createRoot(root).render(<StrictMode><BrowserRouter><App /></BrowserRouter></StrictMode>);
