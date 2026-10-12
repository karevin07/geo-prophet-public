import {lazy, StrictMode, Suspense} from 'react';
import {createRoot} from 'react-dom/client';
import App from './App.tsx';
import './index.css';
import { IS_DEMO } from './lib/demo.ts';
import { LanguageProvider } from './lib/i18n.tsx';

// The admin console is its own area with its own chunk; the data status page moved there.
const AdminApp = lazy(() => import('./admin/AdminApp.tsx'));
if (window.location.pathname.replace(/\/+$/, '') === '/data') window.history.replaceState(null, '', '/admin/sync');
// The online demo has no admin console.
const isAdmin = !IS_DEMO && (window.location.pathname === '/admin' || window.location.pathname.startsWith('/admin/'));

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <LanguageProvider>
      {isAdmin ? <Suspense fallback={null}><AdminApp /></Suspense> : <App />}
    </LanguageProvider>
  </StrictMode>,
);
