import {lazy, StrictMode, Suspense} from 'react';
import {createRoot} from 'react-dom/client';
import App from './App.tsx';
import './index.css';
import { LanguageProvider } from './lib/i18n.tsx';

// The admin console is its own area with its own chunk; the data status page moved there.
const AdminApp = lazy(() => import('./admin/AdminApp.tsx'));
if (window.location.pathname.replace(/\/+$/, '') === '/data') window.history.replaceState(null, '', '/admin/sync');
const isAdmin = window.location.pathname === '/admin' || window.location.pathname.startsWith('/admin/');

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <LanguageProvider>
      {isAdmin ? <Suspense fallback={null}><AdminApp /></Suspense> : <App />}
    </LanguageProvider>
  </StrictMode>,
);
