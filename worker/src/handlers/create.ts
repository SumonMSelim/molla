import { encodeBase62 } from '../core/base62'
import type { Permuter } from '../core/permute'
import {
  DEFAULT_EXPIRY_SECONDS,
  validateAlias,
  validateExpiry,
  validateURL,
} from '../core/validate'
import { error, json, nowSeconds, rfc3339, sha256Hex } from '../http'
import { isPlatformError, PlatformError, type Idempotency, type Link } from '../platform'
import type { IDAllocator, LinkStore } from '../store'

const MAX_CREATE_BODY = 16 * 1024
const IDEMPOTENCY_TTL_SECONDS = 24 * 60 * 60
const MAX_CODE_ATTEMPTS = 32

export type CreateDeps = {
  store: LinkStore
  allocator: IDAllocator
  permuter: Permuter
  publicBase: string
}

type CreateRequest = {
  long_url?: unknown
  alias?: unknown
  expires_in?: unknown
}

export async function create(request: Request, deps: CreateDeps): Promise<Response> {
  const length = Number(request.headers.get('Content-Length') ?? 0)
  if (length > MAX_CREATE_BODY) {
    return error(400, 'INVALID_REQUEST')
  }
  const raw = await request.text()
  if (raw.length > MAX_CREATE_BODY) {
    return error(400, 'INVALID_REQUEST')
  }
  let body: CreateRequest
  try {
    body = JSON.parse(raw) as CreateRequest
  } catch {
    return error(400, 'INVALID_REQUEST')
  }
  if (body === null || typeof body !== 'object') {
    return error(400, 'INVALID_REQUEST')
  }

  const longURL = body.long_url
  if (typeof longURL !== 'string' || !validateURL(longURL)) {
    return error(400, 'INVALID_URL')
  }
  const alias = body.alias ?? ''
  if (typeof alias !== 'string') {
    return error(400, 'INVALID_ALIAS')
  }
  if (alias !== '' && !validateAlias(alias)) {
    return error(400, 'INVALID_ALIAS')
  }
  let expiresIn = DEFAULT_EXPIRY_SECONDS
  if (body.expires_in !== undefined && body.expires_in !== null) {
    if (typeof body.expires_in !== 'number') {
      return error(400, 'INVALID_EXPIRY')
    }
    expiresIn = body.expires_in
  }
  if (!validateExpiry(expiresIn)) {
    return error(400, 'INVALID_EXPIRY')
  }

  const now = nowSeconds()
  let idem: Idempotency | null = null
  if (request.headers.has('Idempotency-Key')) {
    const key = request.headers.get('Idempotency-Key') ?? ''
    if (!validIdempotencyKey(key)) {
      return error(400, 'INVALID_REQUEST')
    }
    idem = {
      key,
      requestHash: await canonicalCreateHash(longURL, alias, expiresIn),
      expiresAt: now + IDEMPOTENCY_TTL_SECONDS,
    }
  }

  const link: Link = {
    shortCode: '',
    longURL,
    ownerID: '',
    isCustom: alias !== '',
    isActive: true,
    version: 1,
    createdAt: now,
    expiresAt: now + expiresIn,
    deletedAt: null,
    purgeAt: now + expiresIn,
    deletedBy: '',
    deleteRole: '',
    deleteReason: '',
  }

  try {
    const created = await persist(deps, link, alias, idem)
    return json(201, {
      short_code: created.shortCode,
      short_url: `${deps.publicBase}/${created.shortCode}`,
      long_url: created.longURL,
      created_at: rfc3339(created.createdAt),
      expires_at: rfc3339(created.expiresAt),
    })
  } catch (err) {
    if (isPlatformError(err, 'collision') && alias !== '') {
      return error(409, 'ALIAS_TAKEN')
    }
    if (isPlatformError(err, 'idempotency_conflict')) {
      return error(409, 'IDEMPOTENCY_CONFLICT')
    }
    console.error('link create failed', { custom_alias: alias !== '', error: String(err) })
    return error(503, 'TEMPORARILY_UNAVAILABLE')
  }
}

async function persist(deps: CreateDeps, link: Link, alias: string, idem: Idempotency | null): Promise<Link> {
  if (alias !== '') {
    return deps.store.create({ ...link, shortCode: alias }, idem)
  }
  let last: unknown = new PlatformError('dependency')
  for (let attempt = 0; attempt < MAX_CODE_ATTEMPTS; attempt++) {
    const id = await deps.allocator.lease()
    const code = encodeBase62(await deps.permuter.permute(id))
    try {
      return await deps.store.create({ ...link, shortCode: code }, idem)
    } catch (err) {
      if (!isPlatformError(err, 'collision')) {
        throw err
      }
      last = err
    }
  }
  throw last
}

function validIdempotencyKey(key: string): boolean {
  if (key.length < 1 || key.length > 128) {
    return false
  }
  for (let i = 0; i < key.length; i++) {
    const c = key.charCodeAt(i)
    if (c < 33 || c > 126) {
      return false
    }
  }
  return true
}

// Same field order as Go's canonicalCreateRequest so hashes stay comparable
// across the two implementations.
function canonicalCreateHash(longURL: string, alias: string, expiresIn: number): Promise<string> {
  return sha256Hex(JSON.stringify({ long_url: longURL, alias, expires_in: expiresIn }))
}
