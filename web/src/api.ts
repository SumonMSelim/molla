export const LINKS_STORAGE = 'molla.links'

export type ApiErrorCode =
  | 'INVALID_URL'
  | 'INVALID_ALIAS'
  | 'INVALID_EXPIRY'
  | 'INVALID_REQUEST'
  | 'ALIAS_TAKEN'
  | 'IDEMPOTENCY_CONFLICT'
  | 'RATE_LIMITED'
  | 'TEMPORARILY_UNAVAILABLE'
  | 'NOT_FOUND'

export const ERROR_MESSAGES: Record<ApiErrorCode, string> = {
  INVALID_URL: 'URL is not a valid http(s) address.',
  INVALID_ALIAS: 'Alias fails charset or length rules.',
  INVALID_EXPIRY: 'Expiry must be between 60 and 157680000 seconds.',
  INVALID_REQUEST: 'Request is invalid.',
  ALIAS_TAKEN: 'That alias is already taken.',
  IDEMPOTENCY_CONFLICT: 'Idempotency key was used for a different request.',
  RATE_LIMITED: 'Too many requests; try again later.',
  TEMPORARILY_UNAVAILABLE: 'Service temporarily unavailable.',
  NOT_FOUND: 'Link not found.',
}

export class ApiError extends Error {
  readonly code: string
  readonly status: number

  constructor(code: string, status: number) {
    super(messageFor(code))
    this.name = 'ApiError'
    this.code = code
    this.status = status
  }
}

export function messageFor(code: string): string {
  if (code in ERROR_MESSAGES) {
    return ERROR_MESSAGES[code as ApiErrorCode]
  }
  return 'Something went wrong.'
}

export type StoredLink = {
  short_code: string
  short_url: string
  long_url: string
  created_at: string
}

export function loadLinks(): StoredLink[] {
  const raw = localStorage.getItem(LINKS_STORAGE)
  if (!raw) {
    return []
  }
  try {
    const parsed: unknown = JSON.parse(raw)
    if (!Array.isArray(parsed)) {
      return []
    }
    return parsed.filter(isStoredLink)
  } catch {
    return []
  }
}

export function rememberLink(link: StoredLink): StoredLink[] {
  const next = [link, ...loadLinks().filter((item) => item.short_code !== link.short_code)]
  localStorage.setItem(LINKS_STORAGE, JSON.stringify(next))
  return next
}

export function forgetLink(shortCode: string): StoredLink[] {
  const next = loadLinks().filter((item) => item.short_code !== shortCode)
  localStorage.setItem(LINKS_STORAGE, JSON.stringify(next))
  return next
}

export type CreateLinkInput = {
  long_url: string
  alias?: string
  expires_in?: number
}

export type CreateLinkResult = {
  short_code: string
  short_url: string
  long_url: string
  created_at: string
  expires_at: string
}

export type LinkStats = {
  short_code: string
  clicks: number
  created_at: string
  last_click_at?: string
}

export async function createLink(input: CreateLinkInput): Promise<CreateLinkResult> {
  const body: Record<string, unknown> = { long_url: input.long_url }
  if (input.alias) {
    body.alias = input.alias
  }
  if (input.expires_in != null) {
    body.expires_in = input.expires_in
  }
  return request<CreateLinkResult>('/api/v1/links', {
    method: 'POST',
    idempotencyKey: crypto.randomUUID(),
    body: JSON.stringify(body),
  })
}

export async function getStats(shortCode: string): Promise<LinkStats> {
  return request<LinkStats>(`/api/v1/links/${encodeURIComponent(shortCode)}/stats`, {
    method: 'GET',
  })
}

type RequestOptions = {
  method: string
  body?: string
  idempotencyKey?: string
}

async function request<T>(path: string, options: RequestOptions): Promise<T> {
  const headers: Record<string, string> = {}
  if (options.body) {
    headers['Content-Type'] = 'application/json'
  }
  if (options.idempotencyKey) {
    headers['Idempotency-Key'] = options.idempotencyKey
  }
  const response = await fetch(path, {
    method: options.method,
    headers,
    body: options.body,
  })
  const text = await response.text()
  let parsed: { error?: string } & T
  try {
    parsed = text ? (JSON.parse(text) as { error?: string } & T) : ({} as { error?: string } & T)
  } catch {
    throw new ApiError('TEMPORARILY_UNAVAILABLE', response.status)
  }
  if (!response.ok) {
    throw new ApiError(parsed.error ?? 'TEMPORARILY_UNAVAILABLE', response.status)
  }
  return parsed
}

function isStoredLink(value: unknown): value is StoredLink {
  if (typeof value !== 'object' || value === null) {
    return false
  }
  const record = value as Record<string, unknown>
  return (
    typeof record.short_code === 'string' &&
    typeof record.short_url === 'string' &&
    typeof record.long_url === 'string' &&
    typeof record.created_at === 'string'
  )
}
