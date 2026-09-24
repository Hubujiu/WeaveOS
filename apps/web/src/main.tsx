import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';

// No login UI is fabricated here. V010-006 owns approved component composition.
const root = document.getElementById('root');
if (!root) throw new Error('Missing application mount point');
createRoot(root).render(<StrictMode>{null}</StrictMode>);
