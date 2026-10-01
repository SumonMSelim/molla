import { validateAlias } from '../core/validate'
import { error, json, nowSeconds } from '../http'
import { isPlatformError, type Principal } from '../platform'
import type { AuditStore, LinkStore } from '../store'
import { cacheKeyFor } from './redirect'

// Takedown is operator-only. The route sits behind Cloudflare Access and the
// Worker verifies the Access JWT itself (see access.ts), so the principal is
// whatever identity Access attested, never a client-supplied value.
export type AdminDeps = {
  store: LinkStore
  audit: AuditStore
  cache: Cache
  purge: (url: string) => Promise<void>
}

export async function takedown(
  request: Request,
  code: string,
  principal: Principal,
  deps: AdminDeps,
): Promise<Response> {
  if (!validateAlias(code)) {
    return error(404, 'NOT_FOUND')
  }
  let reason = ''
  try {
    const body = (await request.json()) as { reason?: unknown }
    if (typeof body.reason === 'string') {
      reason = body.reason.trim()
    }
  } catch {
    return error(400, 'INVALID_REQUEST')
  }
  if (reason === '' || reason.length > 256) {
    return error(400, 'INVALID_REQUEST')
  }

  const now = nowSeconds()
  try {
    const deletion = await deps.store.softDelete(principal, code, now, reason)
    const key = cacheKeyFor(request.url, code)
    let outcome = 'deleted'
    try {
      await deps.cache.delete(key)
      await deps.purge(key.url)
    } catch (err) {
      console.error('cache invalidation failed after soft delete', { short_code: code, error: String(err) })
      outcome = 'invalidate_failed'
    }
    await deps.audit.record({
      actorID: principal.actorID,
      role: principal.role,
      ownerID: deletion.ownerID,
      shortCode: deletion.shortCode,
      reason,
      outcome,
      timestamp: now,
    })
    if (outcome !== 'deleted') {
      return error(503, 'TEMPORARILY_UNAVAILABLE')
    }
    return json(200, { short_code: deletion.shortCode, deleted_at: deletion.deletedAt, version: deletion.version })
  } catch (err) {
    if (isPlatformError(err, 'not_found')) {
      return error(404, 'NOT_FOUND')
    }
    if (isPlatformError(err, 'forbidden')) {
      return error(403, 'FORBIDDEN')
    }
    console.error('soft delete failed', { short_code: code, actor_id: principal.actorID, error: String(err) })
    return error(503, 'TEMPORARILY_UNAVAILABLE')
  }
}
