// Post-build step: writes dist/<slug>/index.html for every static page with
// the page rendered into #root and its own title, description and canonical.
// Run after `vite build` and `vite build --ssr src/prerender.tsx --outDir dist-ssr`.
import { mkdir, readFile, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { pathToFileURL } from 'node:url'

const root = path.resolve(import.meta.dirname, '..')
const ORIGIN = 'https://mol.la'

const { pages, render } = await import(pathToFileURL(path.join(root, 'dist-ssr/prerender.js')).href)
const template = await readFile(path.join(root, 'dist/index.html'), 'utf8')

function replaceOnce(html, pattern, replacement) {
  if (!pattern.test(html)) {
    throw new Error(`prerender: template is missing ${pattern}`)
  }
  return html.replace(pattern, () => replacement)
}

const escape = (value) => value.replaceAll('&', '&amp;').replaceAll('"', '&quot;').replaceAll('<', '&lt;')

for (const page of pages) {
  const url = `${ORIGIN}/app/${page.slug}/`
  let html = template
  html = replaceOnce(html, /<div id="root"><\/div>/, `<div id="root">${render(page)}</div>`)
  html = replaceOnce(html, /<title>[^<]*<\/title>/, `<title>${escape(page.title)}</title>`)
  html = replaceOnce(html, /<meta\s+name="description"[^>]*>/, `<meta name="description" content="${escape(page.description)}" />`)
  html = replaceOnce(html, /<link rel="canonical"[^>]*>/, `<link rel="canonical" href="${url}" />`)
  html = replaceOnce(html, /<meta\s+property="og:title"[^>]*>/, `<meta property="og:title" content="${escape(page.title)}" />`)
  html = replaceOnce(html, /<meta\s+property="og:description"[^>]*>/, `<meta property="og:description" content="${escape(page.description)}" />`)
  html = replaceOnce(html, /<meta\s+property="og:url"[^>]*>/, `<meta property="og:url" content="${url}" />`)
  // Home-page-only content: the structured data and the no-JavaScript summary.
  html = replaceOnce(html, /<script type="application\/ld\+json">[\s\S]*?<\/script>\s*/, '')
  html = replaceOnce(html, /<noscript>[\s\S]*?<\/noscript>\s*/, '')
  await mkdir(path.join(root, 'dist', page.slug), { recursive: true })
  await writeFile(path.join(root, 'dist', page.slug, 'index.html'), html)
}
console.log(`prerendered ${pages.length} pages`)
