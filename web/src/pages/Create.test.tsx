import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { CreatePage } from './Create.tsx'

describe('CreatePage', () => {
  it('creates a link with no auth header and shows the short URL', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      status: 201,
      text: async () =>
        JSON.stringify({
          short_code: 'alias01',
          short_url: 'http://127.0.0.1:8080/alias01',
          long_url: 'https://example.com/path',
          created_at: '2026-09-13T00:00:00Z',
          expires_at: '2031-09-11T00:00:00Z',
        }),
    })
    vi.stubGlobal('fetch', fetchMock)
    vi.stubGlobal('crypto', { randomUUID: () => 'idem-create' })
    render(<CreatePage selected={null} onOpen={() => undefined} onClear={() => undefined} />)

    fireEvent.change(screen.getByLabelText('Long URL'), { target: { value: 'https://example.com/path' } })
    fireEvent.click(screen.getByRole('button', { name: 'Create short link' }))

    expect(await screen.findByText('http://127.0.0.1:8080/alias01')).toBeInTheDocument()
    expect(screen.getByTitle('QR code for http://127.0.0.1:8080/alias01').closest('svg')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Download PNG' })).toBeInTheDocument()
    const headers = (fetchMock.mock.calls[0][1] as RequestInit).headers as Record<string, string>
    expect(headers['X-Api-Key']).toBeUndefined()
    expect(headers['Idempotency-Key']).toBe('idem-create')
  })

  it('converts the expiry amount and unit into seconds', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      status: 201,
      text: async () =>
        JSON.stringify({
          short_code: 'abc1234',
          short_url: 'http://127.0.0.1:8080/abc1234',
          long_url: 'https://example.com',
          created_at: '2026-09-13T00:00:00Z',
          expires_at: '2026-09-15T00:00:00Z',
        }),
    })
    vi.stubGlobal('fetch', fetchMock)
    vi.stubGlobal('crypto', { randomUUID: () => 'idem-expiry' })
    render(<CreatePage selected={null} onOpen={() => undefined} onClear={() => undefined} />)

    fireEvent.change(screen.getByLabelText('Long URL'), { target: { value: 'https://example.com' } })
    fireEvent.change(screen.getByLabelText('Expires in (optional)'), { target: { value: '2' } })
    fireEvent.change(screen.getByLabelText('Expiry unit'), { target: { value: 'hours' } })
    fireEvent.click(screen.getByRole('button', { name: 'Create short link' }))

    expect(await screen.findByText('http://127.0.0.1:8080/abc1234')).toBeInTheDocument()
    const body = JSON.parse((fetchMock.mock.calls[0][1] as RequestInit).body as string)
    expect(body.expires_in).toBe(7200)
  })

  it('renders alias-taken as a specific message', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false,
      status: 409,
      text: async () => JSON.stringify({ error: 'ALIAS_TAKEN' }),
    }))
    vi.stubGlobal('crypto', { randomUUID: () => 'idem-taken' })
    render(<CreatePage selected={null} onOpen={() => undefined} onClear={() => undefined} />)

    fireEvent.change(screen.getByLabelText('Long URL'), { target: { value: 'https://example.com' } })
    fireEvent.click(screen.getByRole('button', { name: 'Create short link' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('That alias is already taken.')
  })
})
