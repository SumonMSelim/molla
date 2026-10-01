import { describe, expect, it } from 'vitest'
import { BASE62_LIMIT, decodeBase62, encodeBase62, InvalidBase62Error } from '../src/core/base62'
import { Permuter } from '../src/core/permute'
import { decideRedirect } from '../src/core/redirect'
import { validateAlias, validateExpiry, validateURL } from '../src/core/validate'
import golden from './golden.json'

// golden.json is generated from the Go implementation; see docs/RUNBOOK.md.
describe('golden vectors from Go', () => {
  it('permute + encode match byte for byte', async () => {
    const p = await Permuter.create(golden.key)
    for (const v of golden.vectors) {
      const permuted = await p.permute(BigInt(v.id))
      expect(permuted).toBe(BigInt(v.permuted))
      expect(encodeBase62(permuted)).toBe(v.code)
      expect(await p.inverse(permuted)).toBe(BigInt(v.id))
      expect(decodeBase62(v.code)).toBe(permuted)
    }
  })

  it('url validation agrees', () => {
    for (const u of golden.url_ok) expect(validateURL(u), u).toBe(true)
    for (const u of golden.url_bad) expect(validateURL(u), u).toBe(false)
  })

  it('alias validation agrees', () => {
    for (const a of golden.alias_ok) expect(validateAlias(a), a).toBe(true)
    for (const a of golden.alias_bad) expect(validateAlias(a), a).toBe(false)
  })
})

describe('base62', () => {
  it('rejects out-of-range and malformed input', () => {
    expect(() => encodeBase62(BASE62_LIMIT)).toThrow(InvalidBase62Error)
    expect(() => encodeBase62(-1n)).toThrow(InvalidBase62Error)
    expect(() => decodeBase62('abc')).toThrow(InvalidBase62Error)
    expect(() => decodeBase62('abc!def')).toThrow(InvalidBase62Error)
    expect(encodeBase62(0n)).toBe('0000000')
    expect(encodeBase62(BASE62_LIMIT - 1n)).toBe('ZZZZZZZ')
  })
})

describe('permuter', () => {
  it('rejects empty key and out-of-range values', async () => {
    await expect(Permuter.create('')).rejects.toThrow()
    const p = await Permuter.create('k')
    await expect(p.permute(BASE62_LIMIT)).rejects.toThrow()
    await expect(p.inverse(-1n)).rejects.toThrow()
  })
})

describe('validate', () => {
  it('expiry bounds', () => {
    expect(validateExpiry(59)).toBe(false)
    expect(validateExpiry(60)).toBe(true)
    expect(validateExpiry(157680000)).toBe(true)
    expect(validateExpiry(157680001)).toBe(false)
    expect(validateExpiry(60.5)).toBe(false)
  })
  it('url length limit counts code points', () => {
    expect(validateURL('https://example.com/' + 'a'.repeat(2028))).toBe(true)
    expect(validateURL('https://example.com/' + 'a'.repeat(2029))).toBe(false)
    expect(validateURL('https://example.com/' + '例'.repeat(2028))).toBe(true)
  })
})

describe('decideRedirect', () => {
  const link = { longURL: 'https://x', isActive: true, expiresAt: 100 }
  it('found only when active and unexpired', () => {
    expect(decideRedirect(link, 99)).toEqual({ found: true, longURL: 'https://x' })
    expect(decideRedirect(link, 100)).toEqual({ found: false })
    expect(decideRedirect({ ...link, isActive: false }, 1)).toEqual({ found: false })
    expect(decideRedirect(null, 1)).toEqual({ found: false })
  })
})
