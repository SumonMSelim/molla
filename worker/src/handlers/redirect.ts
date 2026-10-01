import { decideRedirect } from '../core/redirect'
import { validateAlias } from '../core/validate'
import { noStore, nowSeconds } from '../http'
import { isPlatformError } from '../platform'
import type { LinkStore, StatsStore } from '../store'

// Browser TTL stays short so a takedown is honoured quickly; the edge cache
// (Cache API, per colo) shields D1 for a minute per code.
const BROWSER_CACHE = 'public, max-age=5'
export const EDGE_TTL_SECONDS = 60

export type RedirectDeps = {
  store: LinkStore
  stats: StatsStore
  cache: Cache
}

export async function redirect(
  request: Request,
  code: string,
  deps: RedirectDeps,
  ctx: ExecutionContext,
): Promise<Response> {
  if (!validateAlias(code)) {
    return noStore(404)
  }
  const now = nowSeconds()
  const cacheKey = cacheKeyFor(request.url, code)

  const cached = await deps.cache.match(cacheKey)
  if (cached !== undefined) {
    if (cached.status === 302) {
      ctx.waitUntil(countClick(deps.stats, code, now))
    }
    return cached
  }

  let response: Response
  try {
    const link = await deps.store.get(code)
    const decision = decideRedirect(
      { longURL: link.longURL, isActive: link.isActive, expiresAt: link.expiresAt },
      now,
    )
    if (decision.found) {
      response = new Response(null, {
        status: 302,
        headers: { Location: decision.longURL, 'Cache-Control': BROWSER_CACHE },
      })
      ctx.waitUntil(countClick(deps.stats, code, now))
    } else {
      response = new Response(null, { status: 404, headers: { 'Cache-Control': BROWSER_CACHE } })
    }
  } catch (err) {
    if (isPlatformError(err, 'not_found')) {
      response = new Response(null, { status: 404, headers: { 'Cache-Control': BROWSER_CACHE } })
    } else {
      console.error('link lookup failed', { short_code: code, error: String(err) })
      return noStore(503)
    }
  }

  // The stored copy carries the edge TTL; the client copy keeps the 5s one.
  const stored = new Response(null, response)
  stored.headers.set('Cache-Control', `${BROWSER_CACHE}, s-maxage=${EDGE_TTL_SECONDS}`)
  ctx.waitUntil(deps.cache.put(cacheKey, stored))
  return response
}

export function cacheKeyFor(requestURL: string, code: string): Request {
  const url = new URL(requestURL)
  url.pathname = `/${code}`
  url.search = ''
  return new Request(url.toString(), { method: 'GET' })
}

async function countClick(stats: StatsStore, code: string, now: number): Promise<void> {
  try {
    await stats.increment(code, 1, now)
  } catch (err) {
    console.error('click count failed', { short_code: code, error: String(err) })
  }
}
