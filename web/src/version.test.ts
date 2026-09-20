import { describe, expect, it, vi } from 'vitest'
import { appVersion } from './version.ts'

describe('appVersion', () => {
  it('returns the injected build version', () => {
    vi.stubEnv('VITE_APP_VERSION', 'v1.0.0')
    expect(appVersion()).toBe('v1.0.0')
    vi.unstubAllEnvs()
  })

  it('falls back to dev when unset', () => {
    vi.stubEnv('VITE_APP_VERSION', '')
    expect(appVersion()).toBe('dev')
    vi.unstubAllEnvs()
  })
})
