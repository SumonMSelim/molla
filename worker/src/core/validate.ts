export const MAX_URL_CHARACTERS = 2048
export const MIN_ALIAS_LENGTH = 3
export const MAX_ALIAS_LENGTH = 32
export const MIN_EXPIRY_SECONDS = 60
export const DEFAULT_EXPIRY_SECONDS = 157680000
export const MAX_EXPIRY_SECONDS = DEFAULT_EXPIRY_SECONDS

const ALIAS_PATTERN = /^[A-Za-z0-9_-]+$/

// validateURL mirrors Go's checks: valid UTF-8 (always true for a JS string
// unless it holds lone surrogates), at most 2048 code points, no control
// characters, http(s) scheme, non-empty host, no userinfo. Never fetches.
export function validateURL(raw: string): boolean {
  if (raw === '' || !raw.isWellFormed()) {
    return false
  }
  let count = 0
  for (const char of raw) {
    count++
    if (count > MAX_URL_CHARACTERS || isControl(char)) {
      return false
    }
  }
  let parsed: URL
  try {
    parsed = new URL(raw)
  } catch {
    return false
  }
  if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
    return false
  }
  if (parsed.hostname === '' || parsed.username !== '' || parsed.password !== '') {
    return false
  }
  // WHATWG tolerates a bare "@" with empty credentials; Go rejects any userinfo.
  const afterScheme = raw.slice(raw.indexOf('//') + 2)
  const authority = afterScheme.slice(0, firstIndexOf(afterScheme, '/?#'))
  return !authority.includes('@')
}

function firstIndexOf(s: string, chars: string): number {
  for (let i = 0; i < s.length; i++) {
    if (chars.includes(s[i])) {
      return i
    }
  }
  return s.length
}

function isControl(char: string): boolean {
  const cp = char.codePointAt(0) ?? 0
  return cp < 0x20 || (cp >= 0x7f && cp <= 0x9f)
}

export function validateAlias(alias: string): boolean {
  if (alias.length < MIN_ALIAS_LENGTH || alias.length > MAX_ALIAS_LENGTH) {
    return false
  }
  if (alias === 'api' || alias === 'app') {
    return false
  }
  return ALIAS_PATTERN.test(alias)
}

export function validateExpiry(seconds: number): boolean {
  return Number.isInteger(seconds) && seconds >= MIN_EXPIRY_SECONDS && seconds <= MAX_EXPIRY_SECONDS
}
