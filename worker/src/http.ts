// Response helpers. Security headers match the former CloudFront response
// headers policy; the zone-level transform rule applies the same set to
// responses served straight from static assets.

export const SECURITY_HEADERS: Record<string, string> = {
  'X-Content-Type-Options': 'nosniff',
  'X-Frame-Options': 'DENY',
  'Referrer-Policy': 'no-referrer',
  'Strict-Transport-Security': 'max-age=31536000',
  'Content-Security-Policy':
    "default-src 'self'; script-src 'self' https://static.cloudflareinsights.com; style-src 'self'; connect-src 'self' https://cloudflareinsights.com; img-src 'self'; font-src 'self'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'; object-src 'none'",
}

export function withSecurityHeaders(response: Response): Response {
  const out = new Response(response.body, response)
  for (const [name, value] of Object.entries(SECURITY_HEADERS)) {
    out.headers.set(name, value)
  }
  return out
}

export function json(status: number, body: unknown, headers: HeadersInit = {}): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json', 'Cache-Control': 'no-store', ...headers },
  })
}

// Public error codes with a human-readable message; docs/openapi.json lists the
// same set (a worker test keeps the two in step). UNAUTHORIZED and FORBIDDEN
// are admin-only and deliberately absent from the public spec.
export const ERROR_MESSAGES: Record<string, string> = {
  ALIAS_TAKEN: 'The requested alias is already in use.',
  FORBIDDEN: 'This action is not permitted for the caller.',
  IDEMPOTENCY_CONFLICT: 'The Idempotency-Key was already used with a different request body.',
  INVALID_ALIAS: 'alias must be 3 to 32 letters, digits, hyphens or underscores, and not "api" or "app".',
  INVALID_EXPIRY: 'expires_in must be a number of seconds between 60 and 157680000.',
  INVALID_REQUEST: 'The request body or headers are malformed.',
  INVALID_URL: 'long_url must be an absolute http or https URL of at most 2048 characters, without credentials.',
  METHOD_NOT_ALLOWED: 'This method is not supported for this path.',
  NOT_FOUND: 'No active link exists for this code.',
  RATE_LIMITED: 'Too many requests; retry after the interval in the Retry-After header.',
  TEMPORARILY_UNAVAILABLE: 'The service could not complete the request; retry shortly.',
  UNAUTHORIZED: 'A valid Cloudflare Access assertion is required.',
}

export const DOCS_URL = 'https://mol.la/app/developers/'

export function error(status: number, code: string, headers: HeadersInit = {}): Response {
  return json(status, { error: code, message: ERROR_MESSAGES[code] ?? code, docs_url: DOCS_URL }, headers)
}

export function noStore(status: number): Response {
  return new Response(null, { status, headers: { 'Cache-Control': 'no-store' } })
}

export function rfc3339(unixSeconds: number): string {
  return new Date(unixSeconds * 1000).toISOString().replace(/\.000Z$/, 'Z')
}

export function nowSeconds(): number {
  return Math.floor(Date.now() / 1000)
}

export async function sha256Hex(data: string | Uint8Array): Promise<string> {
  const bytes = typeof data === 'string' ? new TextEncoder().encode(data) : data
  const sum = new Uint8Array(await crypto.subtle.digest('SHA-256', bytes))
  return Array.from(sum, (b) => b.toString(16).padStart(2, '0')).join('')
}
