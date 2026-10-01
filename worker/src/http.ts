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

export function error(status: number, code: string): Response {
  return json(status, { error: code })
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
