import type { ReactElement } from 'react'
import { renderToString } from 'react-dom/server'
import { AboutPage } from './pages/About.tsx'
import { DevelopersPage } from './pages/Developers.tsx'
import { PrivacyPage } from './pages/Privacy.tsx'
import { TermsPage } from './pages/Terms.tsx'

export type StaticPage = { slug: string; title: string; description: string; element: ReactElement }

// Pages with no client state are rendered to HTML at build time (see
// scripts/prerender.mjs) so crawlers and agents that do not run JavaScript
// still get their content. The create page and link pages stay client-only.
export const pages: StaticPage[] = [
  {
    slug: 'about',
    title: 'About - mol.la',
    description: 'What mol.la does and does not do: a free, open-source URL shortener with no account required.',
    element: <AboutPage />,
  },
  {
    slug: 'developers',
    title: 'Developers - mol.la',
    description: 'Public HTTP API for creating short links and reading click stats: endpoints, errors, and cURL examples.',
    element: <DevelopersPage />,
  },
  {
    slug: 'privacy',
    title: 'Privacy - mol.la',
    description: 'What data mol.la processes when you create or follow a short link.',
    element: <PrivacyPage />,
  },
  {
    slug: 'terms',
    title: 'Terms - mol.la',
    description: 'Terms of use for the mol.la URL shortener.',
    element: <TermsPage />,
  },
]

export function render(page: StaticPage): string {
  return renderToString(page.element)
}
