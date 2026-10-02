import { createExecutionContext, env, waitOnExecutionContext } from 'cloudflare:test'
import { describe, expect, it } from 'vitest'
import worker from '../src/index'
import { DOCS_URL, ERROR_MESSAGES } from '../src/http'

const BASE = 'http://localhost:8787'

async function call(path: string, init: RequestInit = {}): Promise<Response> {
  const ctx = createExecutionContext()
  const request = new Request(BASE + path, init) as Request<unknown, IncomingRequestCfProperties>
  const response = await worker.fetch(request, env, ctx)
  await waitOnExecutionContext(ctx)
  return response
}

// Production serves a matching static asset before the Worker runs; mirror that
// order, since worker.fetch alone never sees the assets layer.
async function serve(path: string): Promise<Response> {
  const asset = await env.ASSETS.fetch(BASE + path)
  return asset.status === 404 ? call(path) : asset
}

type Schema = { properties: Record<string, unknown>; required?: string[]; enum?: string[] }
type Spec = {
  paths: Record<string, Record<string, unknown>>
  components: { schemas: Record<string, Schema & { properties: Record<string, Schema> }> }
}

async function spec(): Promise<Spec> {
  const res = await serve('/openapi.json')
  expect(res.status).toBe(200)
  return res.json<Spec>()
}

describe('error bodies', () => {
  it('carry a code, a message, and a docs link', async () => {
    const res = await call('/api/v1/links', { method: 'POST', body: '{}' })
    expect(res.status).toBe(400)
    expect(await res.json()).toEqual({
      error: 'INVALID_URL',
      message: ERROR_MESSAGES.INVALID_URL,
      docs_url: DOCS_URL,
    })
  })

  it('advertise the OpenAPI spec on every /api/ response', async () => {
    for (const path of ['/api/v1/links', '/api/v1/nope', '/api/v1/links/zzzzzzz/stats']) {
      const res = await call(path)
      expect(res.headers.get('Link'), path).toBe('</openapi.json>; rel="service-desc"; type="application/json"')
    }
    expect((await call('/zzzzzzz')).headers.has('Link')).toBe(false)
  })
})

describe('discovery files', () => {
  it('serves robots.txt that keeps crawlers off short links and the API', async () => {
    const res = await serve('/robots.txt')
    expect(res.status).toBe(200)
    expect(res.headers.get('Content-Type')).toContain('text/plain')
    const body = await res.text()
    expect(body).toContain('Disallow: /\n')
    expect(body).toContain('Sitemap: https://mol.la/sitemap.xml')
  })

  it('serves llms.txt pointing at the spec and docs', async () => {
    const body = await (await serve('/llms.txt')).text()
    expect(body).toMatch(/^# mol\.la/)
    expect(body).toContain('https://mol.la/openapi.json')
    expect(body).toContain('https://mol.la/app/developers/')
  })

  it('lists only URLs that resolve', async () => {
    const xml = await (await serve('/sitemap.xml')).text()
    const paths = [...xml.matchAll(/<loc>https:\/\/mol\.la([^<]*)<\/loc>/g)].map((m) => m[1])
    expect(paths.length).toBeGreaterThan(0)
    for (const path of paths) {
      const res = await serve(path)
      expect(res.status, path).toBe(200)
      expect(await res.text(), path).toContain('<title>')
    }
  })

  it('prerenders static pages with their own metadata', async () => {
    const html = await (await serve('/app/developers/')).text()
    expect(html).toContain('<h1')
    expect(html).toContain('Create short link')
    expect(html).toContain('<link rel="canonical" href="https://mol.la/app/developers/" />')
    expect(html).not.toContain('application/ld+json')
    expect(html).not.toContain('<noscript>')
  })

  it('keeps structured data and a no-JS summary on the home page', async () => {
    const html = await (await serve('/')).text()
    expect(html).toContain('application/ld+json')
    expect(html).toContain('<noscript>')
    expect(html).toContain('rel="canonical" href="https://mol.la/"')
  })
})

describe('openapi.json', () => {
  it('lists exactly the public error codes', async () => {
    const { components } = await spec()
    const adminOnly = new Set(['UNAUTHORIZED', 'FORBIDDEN'])
    const documented = components.schemas.Error.properties.error.enum ?? []
    const emitted = Object.keys(ERROR_MESSAGES).filter((code) => !adminOnly.has(code))
    expect([...documented].sort()).toEqual(emitted.sort())
  })

  it('describes the fields the handlers actually return', async () => {
    const { components } = await spec()
    const created = await call('/api/v1/links', {
      method: 'POST',
      headers: { 'CF-Connecting-IP': '198.51.100.7' },
      body: JSON.stringify({ long_url: 'https://example.com/spec', alias: 'specdrift' }),
    })
    expect(created.status).toBe(201)
    expect(Object.keys(await created.json()).sort()).toEqual(Object.keys(components.schemas.Link.properties).sort())

    const stats = await (await call('/api/v1/links/specdrift/stats')).json<Record<string, unknown>>()
    const documented = components.schemas.Stats
    for (const field of documented.required ?? []) {
      expect(stats, field).toHaveProperty(field)
    }
    for (const field of Object.keys(stats)) {
      expect(documented.properties, field).toHaveProperty(field)
    }
  })

  it('covers every public route', async () => {
    const { paths } = await spec()
    expect(Object.keys(paths).sort()).toEqual(['/api/v1/links', '/api/v1/links/{short_code}/stats', '/{short_code}'])
  })
})

describe('well-known', () => {
  it('serves security.txt with the request origin and a contact', async () => {
    const res = await call('/.well-known/security.txt')
    expect(res.status).toBe(200)
    expect(res.headers.get('Content-Type')).toContain('text/plain')
    const body = await res.text()
    expect(body).toMatch(/^Contact: /m)
    expect(body).toMatch(/^Expires: \d{4}-\d{2}-\d{2}T/m)
    expect(body).toContain(`Canonical: ${env.PUBLIC_BASE}/.well-known/security.txt`)
  })

  it('serves an RFC 9727 api-catalog linkset', async () => {
    const res = await call('/.well-known/api-catalog')
    expect(res.status).toBe(200)
    expect(res.headers.get('Content-Type')).toBe('application/linkset+json')
    const body = await res.json<{ linkset: { anchor: string; 'service-desc': { href: string }[] }[] }>()
    expect(body.linkset[0].anchor).toBe(`${env.PUBLIC_BASE}/api/v1/links`)
    expect(body.linkset[0]['service-desc'][0].href).toBe(`${env.PUBLIC_BASE}/openapi.json`)
  })

  it('404s unknown paths and rejects non-GET methods', async () => {
    expect((await call('/.well-known/nope')).status).toBe(404)
    expect((await call('/.well-known/security.txt', { method: 'POST' })).status).toBe(404)
  })
})
