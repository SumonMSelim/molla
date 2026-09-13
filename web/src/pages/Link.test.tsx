import { render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { LinkPage } from './Link.tsx'

describe('LinkPage', () => {
  it('loads stats without authentication', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      text: async () => JSON.stringify({ short_code: 'statzzz', clicks: 4, created_at: '2026-09-13T00:00:00Z' }),
    })
    vi.stubGlobal('fetch', fetchMock)
    render(<LinkPage code="statzzz" onBack={() => undefined} />)

    expect(await screen.findByText('4')).toBeInTheDocument()
    const headers = (fetchMock.mock.calls[0][1] as RequestInit).headers as Record<string, string>
    expect(headers['X-Api-Key']).toBeUndefined()
  })

  it('forgets the link locally without calling the API', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      text: async () => JSON.stringify({ short_code: 'statzzz', clicks: 4, created_at: '2026-09-13T00:00:00Z' }),
    }))
    const onBack = vi.fn()
    render(<LinkPage code="statzzz" onBack={onBack} />)

    await screen.findByText('4')
    screen.getByRole('button', { name: 'Remove from this list' }).click()
    expect(onBack).toHaveBeenCalled()
  })

  it('maps not found', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false,
      status: 404,
      text: async () => JSON.stringify({ error: 'NOT_FOUND' }),
    }))
    render(<LinkPage code="missing" onBack={() => undefined} />)
    expect(await screen.findByRole('alert')).toHaveTextContent('Link not found.')
  })
})
