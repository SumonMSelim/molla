import { render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { LinkPage } from './Link.tsx'

describe('LinkPage', () => {
  it('loads stats and deletes after confirmation', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        text: async () => JSON.stringify({ short_code: 'statzzz', clicks: 4, created_at: '2026-09-13T00:00:00Z' }),
      })
      .mockResolvedValueOnce({
        ok: true,
        status: 204,
        text: async () => '',
      })
    vi.stubGlobal('fetch', fetchMock)
    vi.stubGlobal('confirm', () => true)
    const onBack = vi.fn()
    sessionStorage.setItem('molla.apiKey', 'dev-local-key')
    render(<LinkPage code="statzzz" onBack={onBack} />)

    expect(await screen.findByText('4')).toBeInTheDocument()
    screen.getByRole('button', { name: 'Delete link' }).click()
    await vi.waitFor(() => {
      expect(onBack).toHaveBeenCalled()
    })
    expect((fetchMock.mock.calls[1][1] as RequestInit).method).toBe('DELETE')
  })

  it('maps not found', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false,
      status: 404,
      text: async () => JSON.stringify({ error: 'NOT_FOUND' }),
    }))
    sessionStorage.setItem('molla.apiKey', 'dev-local-key')
    render(<LinkPage code="missing" onBack={() => undefined} />)
    expect(await screen.findByRole('alert')).toHaveTextContent('Link not found.')
  })
})
