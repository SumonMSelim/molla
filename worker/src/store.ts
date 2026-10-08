import {
  DELETE_RETENTION_SECONDS,
  PlatformError,
  type AuditEvent,
  type Deletion,
  type Idempotency,
  type Link,
  type Principal,
  type Stats,
} from './platform'

type AuditRow = {
  actor_id: string
  role: AuditEvent['role']
  owner_id: string
  short_code: string
  reason: string
  outcome: string
  ts: number
}

type LinkRow = {
  short_code: string
  long_url: string
  owner_id: string
  is_custom: number
  is_active: number
  version: number
  created_at: number
  expires_at: number
  deleted_at: number | null
  purge_at: number
  deleted_by: string
  delete_role: string
  delete_reason: string
}

function linkFromRow(row: LinkRow): Link {
  return {
    shortCode: row.short_code,
    longURL: row.long_url,
    ownerID: row.owner_id,
    isCustom: row.is_custom === 1,
    isActive: row.is_active === 1,
    version: row.version,
    createdAt: row.created_at,
    expiresAt: row.expires_at,
    deletedAt: row.deleted_at,
    purgeAt: row.purge_at,
    deletedBy: row.deleted_by,
    deleteRole: row.delete_role,
    deleteReason: row.delete_reason,
  }
}

function isUniqueViolation(err: unknown): boolean {
  return err instanceof Error && /UNIQUE constraint failed/.test(err.message)
}

// LinkStore is the D1 implementation of the authoritative link port.
export class LinkStore {
  constructor(private readonly db: D1Database) {}

  async get(code: string): Promise<Link> {
    const row = await this.db
      .prepare('SELECT * FROM links WHERE short_code = ?')
      .bind(code)
      .first<LinkRow>()
    if (row === null) {
      throw new PlatformError('not_found')
    }
    return linkFromRow(row)
  }

  // create inserts the link and, when present, the idempotency record in one
  // transaction. A duplicate code is a collision; a replayed idempotency key
  // returns the original link or reports a conflicting payload.
  async create(input: Link, idem: Idempotency | null): Promise<Link> {
    const link: Link = {
      ...input,
      version: 1,
      isActive: true,
      deletedAt: null,
      deletedBy: '',
      deleteRole: '',
      deleteReason: '',
      purgeAt: input.expiresAt,
    }
    const statements = [
      this.db
        .prepare(
          'INSERT INTO links (short_code, long_url, owner_id, is_custom, is_active, version, created_at, expires_at, purge_at) VALUES (?, ?, ?, ?, 1, 1, ?, ?, ?)',
        )
        .bind(
          link.shortCode,
          link.longURL,
          link.ownerID,
          link.isCustom ? 1 : 0,
          link.createdAt,
          link.expiresAt,
          link.purgeAt,
        ),
    ]
    if (idem !== null) {
      statements.push(
        this.db
          .prepare(
            'INSERT INTO idempotency (owner_key, short_code, request_hash, ttl) VALUES (?, ?, ?, ?)',
          )
          .bind(ownerKey(link.ownerID, idem.key), link.shortCode, idem.requestHash, idem.expiresAt),
      )
    }
    try {
      await this.db.batch(statements)
      return link
    } catch (err) {
      if (!isUniqueViolation(err)) {
        throw new PlatformError('dependency', err)
      }
      if (idem !== null) {
        return this.replayIdempotency(link.ownerID, idem)
      }
      throw new PlatformError('collision')
    }
  }

  private async replayIdempotency(ownerID: string, idem: Idempotency): Promise<Link> {
    const row = await this.db
      .prepare('SELECT short_code, request_hash FROM idempotency WHERE owner_key = ?')
      .bind(ownerKey(ownerID, idem.key))
      .first<{ short_code: string; request_hash: string }>()
    if (row === null) {
      throw new PlatformError('collision')
    }
    if (row.request_hash !== idem.requestHash) {
      throw new PlatformError('idempotency_conflict')
    }
    return this.get(row.short_code)
  }

  async softDelete(principal: Principal, code: string, now: number, reason: string): Promise<Deletion> {
    const link = await this.get(code)
    if (!authorized(principal, link, reason)) {
      throw new PlatformError('forbidden')
    }
    if (!link.isActive) {
      if (link.deletedBy !== principal.actorID || link.deleteRole !== principal.role) {
        throw new PlatformError('forbidden')
      }
      return deletionFrom(link)
    }
    const purgeAt = now + DELETE_RETENTION_SECONDS
    const result = await this.db
      .prepare(
        'UPDATE links SET is_active = 0, version = ?, deleted_at = ?, purge_at = ?, deleted_by = ?, delete_role = ?, delete_reason = ? WHERE short_code = ? AND is_active = 1 AND version = ?',
      )
      .bind(link.version + 1, now, purgeAt, principal.actorID, principal.role, reason, code, link.version)
      .run()
    if (result.meta.changes !== 1) {
      throw new PlatformError('dependency')
    }
    return deletionFrom({
      ...link,
      isActive: false,
      version: link.version + 1,
      deletedAt: now,
      purgeAt,
      deletedBy: principal.actorID,
      deleteRole: principal.role,
      deleteReason: reason,
    })
  }

  // purgeExpired removes replay records past their TTL and soft-deleted or
  // expired links past purge_at. Called from the cron trigger.
  async purgeExpired(now: number): Promise<void> {
    await this.db.batch([
      this.db.prepare('DELETE FROM idempotency WHERE ttl < ?').bind(now),
      this.db.prepare('DELETE FROM links WHERE purge_at < ?').bind(now),
    ])
  }
}

function ownerKey(ownerID: string, key: string): string {
  return `${ownerID}#${key}`
}

function authorized(principal: Principal, link: Link, reason: string): boolean {
  if (principal.actorID === '') {
    return false
  }
  switch (principal.role) {
    case 'developer':
      return principal.ownerID !== '' && principal.ownerID === link.ownerID
    case 'operator':
      return reason !== ''
    default:
      return false
  }
}

function deletionFrom(link: Link): Deletion {
  return {
    shortCode: link.shortCode,
    ownerID: link.ownerID,
    version: link.version,
    deletedAt: link.deletedAt ?? 0,
    purgeAt: link.purgeAt,
  }
}

// StatsStore holds the click counter per short code. Redirect increments it
// after the response is sent; the counter is approximate (at-least-once is
// not guaranteed if the isolate dies mid-write).
export class StatsStore {
  constructor(private readonly db: D1Database) {}

  async get(code: string): Promise<Stats> {
    const row = await this.db
      .prepare('SELECT short_code, clicks, last_click_at FROM stats WHERE short_code = ?')
      .bind(code)
      .first<{ short_code: string; clicks: number; last_click_at: number }>()
    if (row === null) {
      throw new PlatformError('not_found')
    }
    return { shortCode: row.short_code, clicks: row.clicks, lastClickAt: row.last_click_at }
  }

  async increment(code: string, delta: number, lastClickAt: number): Promise<void> {
    if (code === '' || delta === 0) {
      return
    }
    await this.db
      .prepare(
        'INSERT INTO stats (short_code, clicks, last_click_at) VALUES (?, ?, ?) ON CONFLICT (short_code) DO UPDATE SET clicks = clicks + excluded.clicks, last_click_at = max(last_click_at, excluded.last_click_at)',
      )
      .bind(code, delta, lastClickAt)
      .run()
  }
}

// IDAllocator leases monotonic IDs in blocks from the counters table. Each
// isolate keeps its block in memory; unused tail IDs are simply skipped.
export class IDAllocator {
  private next = 0n
  private end = 0n

  constructor(
    private readonly db: D1Database,
    private readonly region = 'global',
    private readonly blockSize = 100n,
  ) {}

  async lease(): Promise<bigint> {
    if (this.next >= this.end) {
      const row = await this.db
        .prepare('UPDATE counters SET counter = counter + ? WHERE region = ? RETURNING counter')
        .bind(Number(this.blockSize), this.region)
        .first<{ counter: number }>()
      if (row === null) {
        throw new PlatformError('dependency')
      }
      this.end = BigInt(row.counter)
      this.next = this.end - this.blockSize
    }
    return this.next++
  }
}

export class AuditStore {
  constructor(private readonly db: D1Database) {}

  async record(event: AuditEvent): Promise<void> {
    await this.db
      .prepare(
        'INSERT INTO audit (actor_id, role, owner_id, short_code, reason, outcome, ts) VALUES (?, ?, ?, ?, ?, ?, ?)',
      )
      .bind(
        event.actorID,
        event.role,
        event.ownerID,
        event.shortCode,
        event.reason,
        event.outcome,
        event.timestamp,
      )
      .run()
  }

  async list(limit: number): Promise<AuditEvent[]> {
    const rows = await this.db
      .prepare('SELECT actor_id, role, owner_id, short_code, reason, outcome, ts FROM audit ORDER BY id DESC LIMIT ?')
      .bind(limit)
      .all<AuditRow>()
    return rows.results.map((row) => ({
      actorID: row.actor_id,
      role: row.role,
      ownerID: row.owner_id,
      shortCode: row.short_code,
      reason: row.reason,
      outcome: row.outcome,
      timestamp: row.ts,
    }))
  }
}
