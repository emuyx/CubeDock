import React from 'react';
import { createRoot } from 'react-dom/client';
import App from './App.jsx';
import { installDemoApi } from './demoApi.js';

window.__CUBEDOCK_DEMO__ = true;
installDemoApi();

createRoot(document.getElementById('root')).render(<App />);
