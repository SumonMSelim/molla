import { afterEach, describe, expect, it, vi } from 'vitest'
import { AccessVerifier } from '../src/access'

const TEAM = 'team.cloudflareaccess.com'
const AUD = 'aud-123'

function b64url(bytes: Uint8Array | string): string {
  const raw = typeof bytes === 'string' ? bytes : String.fromCharCode(...bytes)
  return btoa(raw).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
}

async function signer() {
  const pair = (await crypto.subtle.generateKey(
    { name: 'RSASSA-PKCS1-v1_5', modulusLength: 2048, publicExponent: new Uint8Array([1, 0, 1]), hash: 'SHA-256' },
    true,
    ['sign', 'verify'],
  )) as CryptoKeyPair
  const exported = (await crypto.subtle.exportKey('jwk', pair.publicKey)) as JsonWebKey
  const { key_ops: _ops, ...jwk } = { ...exported, kid: 'k1' }
  const certs: typeof fetch = async () => Response.json({ keys: [jwk] })
  const jwks = { keys: [jwk] }
  const sign = async (claims: Record<string, unknown>, kid = 'k1') => {
    const head = b64url(JSON.stringify({ alg: 'RS256', kid }))
    const body = b64url(JSON.stringify(claims))
    const sig = new Uint8Array(
      await crypto.subtle.sign('RSASSA-PKCS1-v1_5', pair.privateKey, new TextEncoder().encode(`${head}.${body}`)),
    )
    return `${head}.${body}.${b64url(sig)}`
  }
  return { certs, sign, jwks }
}

function request(token?: string): Request {
  return new Request('https://mol.la/admin/v1/links/abc1234/takedown', {
    method: 'POST',
    headers: token === undefined ? {} : { 'Cf-Access-Jwt-Assertion': token },
  })
}

const NOW = 1_700_000_000
const good = { aud: [AUD], iss: `https://${TEAM}`, exp: NOW + 60, nbf: NOW - 60, email: 'ops@example.com' }

describe('AccessVerifier', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('fetches the team certs through the global fetch by default', async () => {
    const { sign, jwks } = await signer()
    const stub = vi.fn(async (input: RequestInfo | URL) => {
      expect(String(input)).toBe(`https://${TEAM}/cdn-cgi/access/certs`)
      return Response.json(jwks)
    })
    vi.stubGlobal('fetch', stub)
    const v = new AccessVerifier(TEAM, AUD)
    expect((await v.verify(request(await sign(good)), NOW))?.actorID).toBe('ops@example.com')
    expect(stub).toHaveBeenCalledTimes(1)
  })

  it('accepts a valid assertion and returns the operator principal', async () => {
    const { certs, sign } = await signer()
    const v = new AccessVerifier(TEAM, AUD, certs)
    expect(await v.verify(request(await sign(good)), NOW)).toEqual({ actorID: 'ops@example.com', role: 'operator', ownerID: '' })
  })

  it('uses common_name for service tokens', async () => {
    const { certs, sign } = await signer()
    const v = new AccessVerifier(TEAM, AUD, certs)
    const token = await sign({ ...good, email: undefined, common_name: 'svc.access' })
    expect((await v.verify(request(token), NOW))?.actorID).toBe('svc.access')
  })

  it('rejects missing, malformed, expired, wrong-audience, wrong-issuer, and forged tokens', async () => {
    const { certs, sign } = await signer()
    const v = new AccessVerifier(TEAM, AUD, certs)
    expect(await v.verify(request(), NOW)).toBeNull()
    expect(await v.verify(request('a.b'), NOW)).toBeNull()
    expect(await v.verify(request(await sign({ ...good, exp: NOW })), NOW)).toBeNull()
    expect(await v.verify(request(await sign({ ...good, nbf: NOW + 1 })), NOW)).toBeNull()
    expect(await v.verify(request(await sign({ ...good, aud: 'other' })), NOW)).toBeNull()
    expect(await v.verify(request(await sign({ ...good, iss: 'https://evil' })), NOW)).toBeNull()
    expect(await v.verify(request(await sign(good, 'unknown')), NOW)).toBeNull()
    const other = await signer()
    expect(await v.verify(request(await other.sign(good)), NOW)).toBeNull()
  })

  it('refuses everything when not configured', async () => {
    const { certs, sign } = await signer()
    const v = new AccessVerifier('', '', certs)
    expect(await v.verify(request(await sign(good)), NOW)).toBeNull()
  })
})
