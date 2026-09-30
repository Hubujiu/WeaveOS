import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { createBrowserRouter, Navigate, RouterProvider } from 'react-router';
import { Login, Register, ProtectedApp } from './App';
import './style.css';
const root=document.getElementById('root');
if(!root)throw new Error('Missing application mount point');
const router=createBrowserRouter([
 {path:'/login',element:<Login/>}, {path:'/register',element:<Register/>},
 {path:'/app/*',element:<ProtectedApp/>}, {path:'*',element:<Navigate to="/app" replace/>},
]);
createRoot(root).render(<StrictMode><RouterProvider router={router}/></StrictMode>);
