import { AccessVerifier } from './access'
import { Permuter } from './core/permute'
import type { Env } from './env'
import { takedown } from './handlers/admin'
import { create } from './handlers/create'
import { redirect } from './handlers/redirect'
import { stats } from './handlers/stats'
import { error, noStore, nowSeconds, withSecurityHeaders } from './http'
import { zonePurger } from './purge'
import { AuditStore, IDAllocator, LinkStore, StatsStore } from './store'

// Module-level state survives across requests within one isolate: the HMAC
// key import and the allocator's leased ID block are the two things worth
// keeping warm.
let permuter: Promise<Permuter> | null = null
let allocator: IDAllocator | null = null
let verifier: AccessVerifier | null = null

const LINKS_PATH = '/api/v1/links'
const STATS_PATTERN = /^\/api\/v1\/links\/([^/]+)\/stats$/
const TAKEDOWN_PATTERN = /^\/admin\/v1\/links\/([^/]+)\/takedown$/
const CODE_PATTERN = /^\/([^/]+)$/

export default {
  async fetch(request, env, ctx): Promise<Response> {
    const response = await route(request, env, ctx)
    return withSecurityHeaders(response)
  },

  async scheduled(_event, env): Promise<void> {
    await new LinkStore(env.DB).purgeExpired(nowSeconds())
  },
} satisfies ExportedHandler<Env>

async function route(request: Request, env: Env, ctx: ExecutionContext): Promise<Response> {
  const url = new URL(request.url)
  const path = url.pathname
  const method = request.method

  if (path === '/' || path === '/app' || path.startsWith('/app/')) {
    return spa(request, env)
  }

  if (path === LINKS_PATH) {
    if (method !== 'POST') {
      return error(405, 'METHOD_NOT_ALLOWED')
    }
    const ip = request.headers.get('CF-Connecting-IP') ?? ''
    const { success } = await env.CREATE_LIMITER.limit({ key: ip })
    if (!success) {
      return error(429, 'RATE_LIMITED')
    }
    return create(request, {
      store: new LinkStore(env.DB),
      allocator: (allocator ??= new IDAllocator(env.DB)),
      permuter: await (permuter ??= Permuter.create(env.PERMUTATION_KEY)),
      publicBase: env.PUBLIC_BASE,
    })
  }

  const statsMatch = STATS_PATTERN.exec(path)
  if (statsMatch !== null) {
    if (method !== 'GET') {
      return error(405, 'METHOD_NOT_ALLOWED')
    }
    return stats(statsMatch[1], {
      store: new LinkStore(env.DB),
      stats: new StatsStore(env.DB),
      publicBase: env.PUBLIC_BASE,
    })
  }

  const takedownMatch = TAKEDOWN_PATTERN.exec(path)
  if (takedownMatch !== null) {
    if (method !== 'POST') {
      return error(405, 'METHOD_NOT_ALLOWED')
    }
    verifier ??= new AccessVerifier(env.ACCESS_TEAM_DOMAIN, env.ACCESS_AUD)
    let principal
    try {
      principal = await verifier.verify(request, nowSeconds())
    } catch (err) {
      console.error('access verification failed', { error: String(err) })
      return error(503, 'TEMPORARILY_UNAVAILABLE')
    }
    if (principal === null) {
      return error(401, 'UNAUTHORIZED')
    }
    return takedown(request, takedownMatch[1], principal, {
      store: new LinkStore(env.DB),
      audit: new AuditStore(env.DB),
      cache: caches.default,
      purge: zonePurger(env.CF_ZONE_ID, env.CF_PURGE_TOKEN),
    })
  }

  if (path.startsWith('/api/') || path.startsWith('/admin/')) {
    return error(404, 'NOT_FOUND')
  }

  const codeMatch = CODE_PATTERN.exec(path)
  if (codeMatch !== null) {
    if (method !== 'GET' && method !== 'HEAD') {
      return noStore(405)
    }
    return redirect(request, codeMatch[1], {
      store: new LinkStore(env.DB),
      stats: new StatsStore(env.DB),
      cache: caches.default,
    }, ctx)
  }

  return noStore(404)
}

// Static assets are served before the Worker runs when the path matches a
// file; everything else under /app (deep links, "/") gets the SPA shell.
async function spa(request: Request, env: Env): Promise<Response> {
  const url = new URL(request.url)
  url.pathname = '/app/index.html'
  return env.ASSETS.fetch(new Request(url.toString(), request))
}
