import { describe, expect, it } from 'vitest'
import { pages, render } from './prerender.tsx'

describe('prerender', () => {
  it.each(pages)('renders $slug with a heading and unique metadata', (page) => {
    expect(render(page)).toContain('<h1')
    expect(page.title).toMatch(/ - mol\.la$/)
    expect(page.description.length).toBeGreaterThan(20)
  })

  it('has unique slugs and titles', () => {
    expect(new Set(pages.map((p) => p.slug)).size).toBe(pages.length)
    expect(new Set(pages.map((p) => p.title)).size).toBe(pages.length)
  })
})
