import { createRoot } from 'react-dom/client';
import { ApplicationStructurePanel } from './ApplicationStructurePanel';
import { FormDesigner } from './FormDesigner';

const query = new URLSearchParams(location.search);
const appId = query.get('appId') ?? '';
const viewId = query.get('viewId') ?? '';
createRoot(document.getElementById('root')!).render(
  query.get('mode') === 'structure'
    ? <ApplicationStructurePanel appId={appId} />
    : <FormDesigner appId={appId} viewId={viewId} />,
);
