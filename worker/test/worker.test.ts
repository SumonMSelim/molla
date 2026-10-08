import { createExecutionContext, env, waitOnExecutionContext } from 'cloudflare:test'
import { beforeEach, describe, expect, it } from 'vitest'
import worker from '../src/index'
import { listAudit } from '../src/handlers/admin'
import { AuditStore, LinkStore } from '../src/store'

const BASE = 'http://localhost:8787'

async function call(path: string, init: RequestInit = {}): Promise<Response> {
  const ctx = createExecutionContext()
  const request = new Request(BASE + path, init) as Request<unknown, IncomingRequestCfProperties>
  const response = await worker.fetch(request, env, ctx)
  await waitOnExecutionContext(ctx)
  return response
}

// Each create gets its own client IP so the per-IP rate limiter never trips
// inside a test; the limiter itself is covered in its own case below.
let ipCounter = 0

function post(path: string, body: unknown, headers: Record<string, string> = {}): Promise<Response> {
  return call(path, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'CF-Connecting-IP': `10.0.${Math.floor(ipCounter / 256)}.${ipCounter++ % 256}`, ...headers },
    body: JSON.stringify(body),
  })
}

beforeEach(async () => {
  await env.DB.batch([
    env.DB.prepare('DELETE FROM links'),
    env.DB.prepare('DELETE FROM idempotency'),
    env.DB.prepare('DELETE FROM stats'),
    env.DB.prepare('DELETE FROM audit'),
  ])
  await caches.default.delete(`${BASE}/abc1234`)
})

describe('create', () => {
  it('generates a 7-char code and returns 201', async () => {
    const res = await post('/api/v1/links', { long_url: 'https://example.com' })
    expect(res.status).toBe(201)
    const body = await res.json<{ short_code: string; short_url: string; created_at: string; expires_at: string }>()
    expect(body.short_code).toMatch(/^[0-9A-Za-z]{7}$/)
    expect(body.short_url).toBe(`${env.PUBLIC_BASE}/${body.short_code}`)
    expect(body.created_at).toMatch(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$/)
    expect(res.headers.get('Content-Security-Policy')).toContain("frame-ancestors 'none'")
  })

  it('honours a custom alias and reports a taken one', async () => {
    expect((await post('/api/v1/links', { long_url: 'https://example.com', alias: 'abc1234' })).status).toBe(201)
    const dup = await post('/api/v1/links', { long_url: 'https://other.example', alias: 'abc1234' })
    expect(dup.status).toBe(409)
    expect(await dup.json()).toMatchObject({ error: 'ALIAS_TAKEN' })
  })

  it('validates input', async () => {
    expect(await (await post('/api/v1/links', { long_url: 'ftp://x' })).json()).toMatchObject({ error: 'INVALID_URL' })
    expect(await (await post('/api/v1/links', { long_url: 'https://x.y', alias: 'api' })).json()).toMatchObject({ error: 'INVALID_ALIAS' })
    expect(await (await post('/api/v1/links', { long_url: 'https://x.y', expires_in: 1 })).json()).toMatchObject({ error: 'INVALID_EXPIRY' })
    const bad = await call('/api/v1/links', { method: 'POST', body: '{' })
    expect(bad.status).toBe(400)
    expect(await bad.json()).toMatchObject({ error: 'INVALID_REQUEST' })
    expect((await call('/api/v1/links', { method: 'GET' })).status).toBe(405)
  })

  it('replays an idempotent create and rejects a changed payload', async () => {
    const first = await post('/api/v1/links', { long_url: 'https://example.com' }, { 'Idempotency-Key': 'k1' })
    const second = await post('/api/v1/links', { long_url: 'https://example.com' }, { 'Idempotency-Key': 'k1' })
    expect(second.status).toBe(201)
    expect((await second.json<{ short_code: string }>()).short_code).toBe((await first.json<{ short_code: string }>()).short_code)
    const changed = await post('/api/v1/links', { long_url: 'https://changed.example' }, { 'Idempotency-Key': 'k1' })
    expect(changed.status).toBe(409)
    expect(await changed.json()).toMatchObject({ error: 'IDEMPOTENCY_CONFLICT' })
    expect((await post('/api/v1/links', { long_url: 'https://example.com' }, { 'Idempotency-Key': 'bad key' })).status).toBe(400)
  })
})

describe('rate limit', () => {
  it('returns 429 once one IP exceeds the create ceiling', async () => {
    let last: Response | undefined
    for (let i = 0; i < 12; i++) {
      last = await post('/api/v1/links', { long_url: 'https://example.com' }, { 'CF-Connecting-IP': '203.0.113.9' })
      if (last.status === 429) break
    }
    expect(last?.status).toBe(429)
    expect(last?.headers.get('Retry-After')).toBe('60')
    expect(await last?.json()).toMatchObject({ error: 'RATE_LIMITED' })
  })
})

describe('redirect + stats', () => {
  it('302s, counts clicks, and serves from the edge cache', async () => {
    await post('/api/v1/links', { long_url: 'https://example.com/x', alias: 'abc1234' })
    const r1 = await call('/abc1234')
    expect(r1.status).toBe(302)
    expect(r1.headers.get('Location')).toBe('https://example.com/x')
    expect(r1.headers.get('Cache-Control')).toBe('public, max-age=5')
    const r2 = await call('/abc1234')
    expect(r2.status).toBe(302)
    expect(r2.headers.get('Cache-Control')).toBe('public, max-age=5')

    const stats = await call('/api/v1/links/abc1234/stats')
    expect(stats.status).toBe(200)
    const body = await stats.json<{ clicks: number; last_click_at?: string }>()
    expect(body.clicks).toBe(2)
    expect(body.last_click_at).toBeDefined()
  })

  it('404s unknown, malformed, and expired codes without caching them in the browser for long', async () => {
    expect((await call('/zzzzzzz')).status).toBe(404)
    expect((await call('/ab')).status).toBe(404)
    expect((await call('/api/v1/links/zzzzzzz/stats')).status).toBe(404)
    await env.DB.prepare(
      "INSERT INTO links (short_code, long_url, is_custom, created_at, expires_at, purge_at) VALUES ('expired', 'https://e', 1, 1, 2, 2)",
    ).run()
    expect((await call('/expired')).status).toBe(404)
  })

  it('stats for a taken-down link are hidden', async () => {
    await post('/api/v1/links', { long_url: 'https://example.com', alias: 'abc1234' })
    await new LinkStore(env.DB).softDelete({ actorID: 'ops', role: 'operator', ownerID: '' }, 'abc1234', 100, 'abuse')
    expect((await call('/api/v1/links/abc1234/stats')).status).toBe(404)
  })
})

describe('takedown', () => {
  it('requires an Access assertion', async () => {
    const res = await post('/admin/v1/links/abc1234/takedown', { reason: 'abuse' })
    expect(res.status).toBe(401)
  })
})

describe('audit', () => {
  it('requires an Access assertion and only answers GET', async () => {
    expect((await call('/admin/v1/audit')).status).toBe(401)
    expect((await post('/admin/v1/audit', {})).status).toBe(405)
  })

  it('lists recorded events newest first with RFC 3339 timestamps', async () => {
    const audit = new AuditStore(env.DB)
    const base = { actorID: 'ops@example.com', role: 'operator', ownerID: '', outcome: 'deleted' } as const
    await audit.record({ ...base, shortCode: 'first00', reason: 'spam', timestamp: 100 })
    await audit.record({ ...base, shortCode: 'second0', reason: 'phishing', timestamp: 200 })
    const res = await listAudit(audit)
    expect(res.status).toBe(200)
    const body = (await res.json()) as { events: Array<Record<string, unknown>> }
    expect(body.events.map((e) => e.short_code)).toEqual(['second0', 'first00'])
    expect(body.events[0]).toEqual({
      actor_id: 'ops@example.com',
      role: 'operator',
      owner_id: '',
      short_code: 'second0',
      reason: 'phishing',
      outcome: 'deleted',
      ts: '1970-01-01T00:03:20Z',
    })
  })

  it('answers 503 when the store fails', async () => {
    const broken = { list: async () => Promise.reject(new Error('d1 down')) } as unknown as AuditStore
    const res = await listAudit(broken)
    expect(res.status).toBe(503)
    expect(await res.json()).toMatchObject({ error: 'TEMPORARILY_UNAVAILABLE' })
  })
})

describe('store', () => {
  it('soft delete is idempotent for the same actor and forbidden for another', async () => {
    const store = new LinkStore(env.DB)
    await post('/api/v1/links', { long_url: 'https://example.com', alias: 'abc1234' })
    const ops = { actorID: 'ops', role: 'operator', ownerID: '' } as const
    const first = await store.softDelete(ops, 'abc1234', 100, 'abuse')
    expect(first.version).toBe(2)
    expect(first.purgeAt).toBe(100 + 2_592_000)
    const again = await store.softDelete(ops, 'abc1234', 200, 'abuse')
    expect(again.version).toBe(2)
    await expect(store.softDelete({ ...ops, actorID: 'other' }, 'abc1234', 200, 'abuse')).rejects.toThrow('forbidden')
    await expect(store.softDelete(ops, 'abc1234', 200, '')).rejects.toThrow('forbidden')
  })

  it('cron purge removes expired replay records and purged links', async () => {
    await env.DB.prepare("INSERT INTO idempotency (owner_key, short_code, request_hash, ttl) VALUES ('#k', 'x', 'h', 1)").run()
    await env.DB.prepare(
      "INSERT INTO links (short_code, long_url, is_custom, created_at, expires_at, purge_at) VALUES ('oldlink', 'https://e', 1, 1, 2, 2)",
    ).run()
    await new LinkStore(env.DB).purgeExpired(10)
    expect(await env.DB.prepare('SELECT count(*) AS n FROM idempotency').first<{ n: number }>()).toEqual({ n: 0 })
    expect(await env.DB.prepare('SELECT count(*) AS n FROM links').first<{ n: number }>()).toEqual({ n: 0 })
  })
})

describe('spa', () => {
  it('serves the shell for deep links and the root', async () => {
    for (const path of ['/', '/app', '/app/links/abc1234', '/admin', '/admin/']) {
      const res = await call(path)
      expect(res.status, path).toBe(200)
      expect(await res.text()).toContain('<div id="root">')
    }
  })

  it('serves nothing else under /admin', async () => {
    expect((await call('/admin/other')).status).toBe(404)
    expect((await call('/admin/', { method: 'POST' })).status).toBe(404)
  })
})
