// Ports and records shared by handlers and the D1 adapter. Mirrors the Go
// internal/platform package; nothing here talks to Cloudflare directly.

export type Role = 'developer' | 'operator'

export type Principal = {
  actorID: string
  role: Role
  ownerID: string
}

export type Link = {
  shortCode: string
  longURL: string
  ownerID: string
  isCustom: boolean
  isActive: boolean
  version: number
  createdAt: number
  expiresAt: number
  deletedAt: number | null
  purgeAt: number
  deletedBy: string
  deleteRole: string
  deleteReason: string
}

export type Deletion = {
  shortCode: string
  ownerID: string
  version: number
  deletedAt: number
  purgeAt: number
}

export type Idempotency = {
  key: string
  requestHash: string
  expiresAt: number
}

export type Stats = {
  shortCode: string
  clicks: number
  lastClickAt: number
}

export type TopLink = {
  shortCode: string
  longURL: string
  clicks: number
  createdAt: number
  lastClickAt: number
}

export type AuditEvent = {
  actorID: string
  role: Role
  ownerID: string
  shortCode: string
  reason: string
  outcome: string
  timestamp: number
}

export const DELETE_RETENTION_SECONDS = 2_592_000

export class PlatformError extends Error {
  constructor(readonly code: PlatformErrorCode, cause?: unknown) {
    super(code, { cause })
    this.name = 'PlatformError'
  }
}

export type PlatformErrorCode =
  | 'not_found'
  | 'forbidden'
  | 'collision'
  | 'idempotency_conflict'
  | 'dependency'

export function isPlatformError(err: unknown, code: PlatformErrorCode): boolean {
  return err instanceof PlatformError && err.code === code
}
