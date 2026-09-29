import {StrictMode} from 'react';
import {createRoot} from 'react-dom/client';
import {LivePage} from './features/live/LivePage';
import {SimulatorPage} from './features/simulator/SimulatorPage';
import './app.css';

function App() {
  const path = decodeURI(location.pathname);
  if (path === '/' || path === '/simulator') return <SimulatorPage />;
  const match = /^\/live\/([^/]+)$/.exec(path);
  if (match) return <LivePage id={decodeURIComponent(match[1])} />;
  return (
    <main>
      <h1>Page not found</h1>
      <a href="/simulator">Open simulator</a>
    </main>
  );
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
