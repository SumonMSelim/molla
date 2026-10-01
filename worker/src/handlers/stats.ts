import { validateAlias } from '../core/validate'
import { error, json, rfc3339 } from '../http'
import { isPlatformError, type Stats } from '../platform'
import type { LinkStore, StatsStore } from '../store'

export type StatsDeps = {
  store: LinkStore
  stats: StatsStore
  publicBase: string
}

export async function stats(code: string, deps: StatsDeps): Promise<Response> {
  if (!validateAlias(code)) {
    return error(404, 'NOT_FOUND')
  }
  try {
    const link = await deps.store.get(code)
    // A taken-down link must not confirm its own existence or leak history.
    if (!link.isActive) {
      return error(404, 'NOT_FOUND')
    }
    let record: Stats
    try {
      record = await deps.stats.get(code)
    } catch (err) {
      if (!isPlatformError(err, 'not_found')) {
        throw err
      }
      record = { shortCode: code, clicks: 0, lastClickAt: 0 }
    }
    const body: Record<string, unknown> = {
      short_code: link.shortCode,
      short_url: `${deps.publicBase}/${link.shortCode}`,
      clicks: record.clicks,
      created_at: rfc3339(link.createdAt),
    }
    if (record.lastClickAt !== 0) {
      body.last_click_at = rfc3339(record.lastClickAt)
    }
    return json(200, body)
  } catch (err) {
    if (isPlatformError(err, 'not_found')) {
      return error(404, 'NOT_FOUND')
    }
    console.error('stats lookup failed', { short_code: code, error: String(err) })
    return error(503, 'TEMPORARILY_UNAVAILABLE')
  }
}
