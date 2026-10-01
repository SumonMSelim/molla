// Provider-neutral domain logic ported 1:1 from the Go internal/core package.
// test/golden.json pins the Go output so the two never drift.

export const CODE_LENGTH = 7
export const BASE62_LIMIT = 62n ** 7n

const ALPHABET = '0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ'

export class InvalidBase62Error extends Error {
  constructor() {
    super('invalid Base62 value')
    this.name = 'InvalidBase62Error'
  }
}

export function encodeBase62(value: bigint): string {
  if (value < 0n || value >= BASE62_LIMIT) {
    throw new InvalidBase62Error()
  }
  let out = ''
  for (let i = 0; i < CODE_LENGTH; i++) {
    out = ALPHABET[Number(value % 62n)] + out
    value /= 62n
  }
  return out
}

export function decodeBase62(code: string): bigint {
  if (code.length !== CODE_LENGTH) {
    throw new InvalidBase62Error()
  }
  let value = 0n
  for (let i = 0; i < code.length; i++) {
    const digit = ALPHABET.indexOf(code[i])
    if (digit < 0) {
      throw new InvalidBase62Error()
    }
    value = value * 62n + BigInt(digit)
  }
  return value
}
