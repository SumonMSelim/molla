import { StrictMode } from 'react'
import { createRoot, hydrateRoot } from 'react-dom/client'
import { App } from './App.tsx'
import './index.css'

const root = document.getElementById('root')
if (!root) {
  throw new Error('root element missing')
}

const app = (
  <StrictMode>
    <App />
  </StrictMode>
)

// Static pages ship prerendered (scripts/prerender.mjs); hydrate those and
// render the client-only pages from scratch.
if (root.hasChildNodes()) {
  hydrateRoot(root, app)
} else {
  createRoot(root).render(app)
}
