import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  ApiError,
  ERROR_MESSAGES,
  LINKS_STORAGE,
  API_KEY_STORAGE,
  createLink,
  deleteLink,
  forgetLink,
  getApiKey,
  getStats,
  loadLinks,
  messageFor,
  rememberLink,
  setApiKey,
} from './api.ts'

afterEach(() => {
  sessionStorage.clear()
  localStorage.clear()
  vi.unstubAllGlobals()
})

describe('messageFor', () => {
  it('maps every API contract error code', () => {
    for (const [code, message] of Object.entries(ERROR_MESSAGES)) {
      expect(messageFor(code)).toBe(message)
    }
  })

  it('falls back for unknown codes', () => {
    expect(messageFor('NOPE')).toBe('Something went wrong.')
  })
})

describe('api key storage', () => {
  it('keeps the key in sessionStorage only', () => {
    setApiKey('dev-local-key')
    expect(getApiKey()).toBe('dev-local-key')
    expect(sessionStorage.getItem(API_KEY_STORAGE)).toBe('dev-local-key')
    expect(localStorage.getItem(API_KEY_STORAGE)).toBeNull()
  })
})

describe('remembered links', () => {
  it('stores created links in localStorage and drops them on forget', () => {
    const link = {
      short_code: 'abc',
      short_url: 'http://127.0.0.1:8080/abc',
      long_url: 'https://example.com',
      created_at: '2026-09-13T00:00:00Z',
    }
    rememberLink(link)
    expect(loadLinks()).toEqual([link])
    expect(localStorage.getItem(LINKS_STORAGE)).toContain('abc')
    forgetLink('abc')
    expect(loadLinks()).toEqual([])
  })
})

describe('createLink', () => {
  it('sends X-Api-Key and a fresh Idempotency-Key', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      status: 201,
      text: async () =>
        JSON.stringify({
          short_code: 'aB3xK9c',
          short_url: 'http://127.0.0.1:8080/aB3xK9c',
          long_url: 'https://example.com',
          created_at: '2026-09-13T00:00:00Z',
          expires_at: '2031-09-11T00:00:00Z',
        }),
    })
    vi.stubGlobal('fetch', fetchMock)
    vi.stubGlobal('crypto', { randomUUID: () => 'idem-1' })
    setApiKey('dev-local-key')

    const result = await createLink({ long_url: 'https://example.com', alias: 'mine' })
    expect(result.short_code).toBe('aB3xK9c')
    expect(fetchMock).toHaveBeenCalledTimes(1)
    const [path, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(path).toBe('/api/v1/links')
    const headers = init.headers as Record<string, string>
    expect(headers['X-Api-Key']).toBe('dev-local-key')
    expect(headers['Idempotency-Key']).toBe('idem-1')
    expect(headers['Content-Type']).toBe('application/json')
    expect(JSON.parse(String(init.body))).toEqual({ long_url: 'https://example.com', alias: 'mine' })
  })

  it('uses a new idempotency key on each create', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      status: 201,
      text: async () =>
        JSON.stringify({
          short_code: 'aaaaaaa',
          short_url: 'http://127.0.0.1:8080/aaaaaaa',
          long_url: 'https://example.com',
          created_at: '2026-09-13T00:00:00Z',
          expires_at: '2031-09-11T00:00:00Z',
        }),
    })
    vi.stubGlobal('fetch', fetchMock)
    const keys = ['first-key', 'second-key']
    vi.stubGlobal('crypto', { randomUUID: () => keys.shift() ?? 'overflow' })
    setApiKey('k')
    await createLink({ long_url: 'https://example.com/one' })
    await createLink({ long_url: 'https://example.com/two' })
    const first = (fetchMock.mock.calls[0][1] as RequestInit).headers as Record<string, string>
    const second = (fetchMock.mock.calls[1][1] as RequestInit).headers as Record<string, string>
    expect(first['Idempotency-Key']).toBe('first-key')
    expect(second['Idempotency-Key']).toBe('second-key')
  })

  it('throws mapped API errors', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false,
      status: 409,
      text: async () => JSON.stringify({ error: 'ALIAS_TAKEN' }),
    }))
    await expect(createLink({ long_url: 'https://example.com', alias: 'taken' })).rejects.toMatchObject({
      code: 'ALIAS_TAKEN',
      message: ERROR_MESSAGES.ALIAS_TAKEN,
    } satisfies Partial<ApiError>)
  })
})

describe('getStats and deleteLink', () => {
  it('sends the API key on stats', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      text: async () => JSON.stringify({ short_code: 'abc', clicks: 2, created_at: '2026-09-13T00:00:00Z' }),
    })
    vi.stubGlobal('fetch', fetchMock)
    setApiKey('dev-local-key')
    await getStats('abc')
    const headers = (fetchMock.mock.calls[0][1] as RequestInit).headers as Record<string, string>
    expect(headers['X-Api-Key']).toBe('dev-local-key')
    expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/links/abc/stats')
  })

  it('treats 204 as success on delete', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      status: 204,
      text: async () => '',
    })
    vi.stubGlobal('fetch', fetchMock)
    setApiKey('dev-local-key')
    await deleteLink('abc')
    expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/links/abc')
    expect((fetchMock.mock.calls[0][1] as RequestInit).method).toBe('DELETE')
  })
})
