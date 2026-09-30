import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { createBrowserRouter, RouterProvider } from 'react-router';
import { App } from './App';
import './style.css';
const root=document.getElementById('root');
if(!root)throw new Error('Missing application mount point');
const router=createBrowserRouter([{path:'*',element:<App/>}]);
createRoot(root).render(<StrictMode><RouterProvider router={router}/></StrictMode>);