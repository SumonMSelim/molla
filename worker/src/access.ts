import type { Principal } from './platform'

// Verifies the Cf-Access-Jwt-Assertion header that Cloudflare Access adds
// after authenticating a request to a protected path. Access already blocks
// unauthenticated traffic at the edge; verifying again here means a
// misconfigured or removed Access policy fails closed instead of open.

type Jwk = JsonWebKey & { kid: string }
type CertsResponse = { keys: Jwk[] }

type AccessClaims = {
  aud?: string | string[]
  iss?: string
  exp?: number
  nbf?: number
  email?: string
  common_name?: string
  sub?: string
}

const CERT_CACHE_SECONDS = 600

export class AccessVerifier {
  private keys: Map<string, CryptoKey> | null = null
  private fetchedAt = 0

  constructor(
    private readonly teamDomain: string,
    private readonly aud: string,
    private readonly fetchImpl: typeof fetch = fetch,
  ) {}

  async verify(request: Request, nowSeconds: number): Promise<Principal | null> {
    const token = request.headers.get('Cf-Access-Jwt-Assertion')
    if (token === null || this.teamDomain === '' || this.aud === '') {
      return null
    }
    const parts = token.split('.')
    if (parts.length !== 3) {
      return null
    }
    let header: { alg?: string; kid?: string }
    let claims: AccessClaims
    try {
      header = JSON.parse(decodeBase64Url(parts[0])) as { alg?: string; kid?: string }
      claims = JSON.parse(decodeBase64Url(parts[1])) as AccessClaims
    } catch {
      return null
    }
    if (header.alg !== 'RS256' || typeof header.kid !== 'string') {
      return null
    }
    const key = await this.keyFor(header.kid, nowSeconds)
    if (key === null) {
      return null
    }
    const data = new TextEncoder().encode(`${parts[0]}.${parts[1]}`)
    const signature = base64UrlToBytes(parts[2])
    const ok = await crypto.subtle.verify('RSASSA-PKCS1-v1_5', key, signature, data)
    if (!ok) {
      return null
    }
    const audiences = Array.isArray(claims.aud) ? claims.aud : [claims.aud]
    if (!audiences.includes(this.aud)) {
      return null
    }
    if (claims.iss !== `https://${this.teamDomain}`) {
      return null
    }
    if (typeof claims.exp !== 'number' || claims.exp <= nowSeconds) {
      return null
    }
    if (typeof claims.nbf === 'number' && claims.nbf > nowSeconds) {
      return null
    }
    const actor = claims.email ?? claims.common_name ?? claims.sub
    if (typeof actor !== 'string' || actor === '') {
      return null
    }
    return { actorID: actor, role: 'operator', ownerID: '' }
  }

  private async keyFor(kid: string, nowSeconds: number): Promise<CryptoKey | null> {
    if (this.keys === null || nowSeconds - this.fetchedAt > CERT_CACHE_SECONDS || !this.keys.has(kid)) {
      await this.refresh(nowSeconds)
    }
    return this.keys?.get(kid) ?? null
  }

  private async refresh(nowSeconds: number): Promise<void> {
    const response = await this.fetchImpl(`https://${this.teamDomain}/cdn-cgi/access/certs`)
    if (!response.ok) {
      throw new Error(`access certs: ${response.status}`)
    }
    const body = (await response.json()) as CertsResponse
    const keys = new Map<string, CryptoKey>()
    for (const jwk of body.keys) {
      const key = await crypto.subtle.importKey(
        'jwk',
        jwk,
        { name: 'RSASSA-PKCS1-v1_5', hash: 'SHA-256' },
        false,
        ['verify'],
      )
      keys.set(jwk.kid, key)
    }
    this.keys = keys
    this.fetchedAt = nowSeconds
  }
}

function base64UrlToBytes(input: string): Uint8Array {
  const padded = input.replace(/-/g, '+').replace(/_/g, '/').padEnd(Math.ceil(input.length / 4) * 4, '=')
  const binary = atob(padded)
  const bytes = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i++) {
    bytes[i] = binary.charCodeAt(i)
  }
  return bytes
}

function decodeBase64Url(input: string): string {
  return new TextDecoder().decode(base64UrlToBytes(input))
}
