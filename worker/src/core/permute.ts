import { BASE62_LIMIT } from './base62'

const ROUNDS = 6
const HALF_BITS = 21n
const HALF_MASK = (1n << HALF_BITS) - 1n

export class InvalidPermutationError extends Error {
  constructor() {
    super('invalid permutation input')
    this.name = 'InvalidPermutationError'
  }
}

// Permuter hides allocator order with a keyed Feistel permutation over the
// 42-bit code space, cycle-walking values that land outside [0, BASE62_LIMIT).
export class Permuter {
  private constructor(private readonly key: CryptoKey) {}

  static async create(key: string | Uint8Array): Promise<Permuter> {
    const raw = typeof key === 'string' ? new TextEncoder().encode(key) : key
    if (raw.length === 0) {
      throw new InvalidPermutationError()
    }
    const cryptoKey = await crypto.subtle.importKey(
      'raw',
      raw,
      { name: 'HMAC', hash: 'SHA-256' },
      false,
      ['sign'],
    )
    return new Permuter(cryptoKey)
  }

  async permute(value: bigint): Promise<bigint> {
    if (value < 0n || value >= BASE62_LIMIT) {
      throw new InvalidPermutationError()
    }
    value = await this.permute42(value)
    while (value >= BASE62_LIMIT) {
      value = await this.permute42(value)
    }
    return value
  }

  async inverse(value: bigint): Promise<bigint> {
    if (value < 0n || value >= BASE62_LIMIT) {
      throw new InvalidPermutationError()
    }
    value = await this.inverse42(value)
    while (value >= BASE62_LIMIT) {
      value = await this.inverse42(value)
    }
    return value
  }

  private async permute42(value: bigint): Promise<bigint> {
    let left = value >> HALF_BITS
    let right = value & HALF_MASK
    for (let round = 0; round < ROUNDS; round++) {
      const next = left ^ (await this.round(round, right))
      left = right
      right = next
    }
    return (left << HALF_BITS) | right
  }

  private async inverse42(value: bigint): Promise<bigint> {
    let left = value >> HALF_BITS
    let right = value & HALF_MASK
    for (let round = ROUNDS - 1; round >= 0; round--) {
      const next = right ^ (await this.round(round, left))
      right = left
      left = next
    }
    return (left << HALF_BITS) | right
  }

  private async round(round: number, right: bigint): Promise<bigint> {
    const message = new Uint8Array(9)
    message[0] = round
    new DataView(message.buffer).setBigUint64(1, right)
    const sum = new Uint8Array(await crypto.subtle.sign('HMAC', this.key, message))
    const word = new DataView(sum.buffer).getUint32(0)
    return BigInt(word) & HALF_MASK
  }
}
