import { fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { AdminPage } from './Admin.tsx'

const STATS = JSON.stringify({ short_code: 'abc1234', short_url: 'https://mol.la/abc1234', clicks: 3, created_at: '2026-09-13T00:00:00Z' })

function jsonResponse(status: number, body: string) {
  return { ok: status < 400, status, type: 'basic', text: async () => body }
}

afterEach(() => {
  vi.unstubAllGlobals()
})

async function lookUp(code: string) {
  fireEvent.change(screen.getByLabelText('Short code'), { target: { value: code } })
  fireEvent.click(screen.getByRole('button', { name: 'Look up' }))
}

describe('AdminPage', () => {
  it('looks up a link and takes it down after confirmation', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(jsonResponse(200, STATS))
      .mockResolvedValueOnce(jsonResponse(200, JSON.stringify({ short_code: 'abc1234', deleted_at: '2026-10-08T00:00:00Z', version: 2 })))
    vi.stubGlobal('fetch', fetchMock)
    render(<AdminPage />)

    await lookUp('abc1234')
    expect(await screen.findByText('3')).toBeInTheDocument()

    const takeDown = screen.getByRole('button', { name: 'Take down' })
    expect(takeDown).toBeDisabled()
    fireEvent.change(screen.getByLabelText('Reason'), { target: { value: 'phishing' } })
    expect(takeDown).toBeEnabled()
    fireEvent.click(takeDown)
    expect(fetchMock).toHaveBeenCalledTimes(1)

    fireEvent.click(screen.getByRole('button', { name: 'Confirm takedown' }))
    expect(await screen.findByRole('status')).toHaveTextContent('Deleted at 2026-10-08T00:00:00Z')
    const [path, init] = fetchMock.mock.calls[1] as [string, RequestInit]
    expect(path).toBe('/admin/v1/links/abc1234/takedown')
    expect(init.method).toBe('POST')
    expect(JSON.parse(String(init.body))).toEqual({ reason: 'phishing' })
    expect(screen.queryByRole('button', { name: 'Confirm takedown' })).not.toBeInTheDocument()
  })

  it('cancel keeps the link', async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, STATS))
    vi.stubGlobal('fetch', fetchMock)
    render(<AdminPage />)

    await lookUp('abc1234')
    await screen.findByText('3')
    fireEvent.change(screen.getByLabelText('Reason'), { target: { value: 'spam' } })
    fireEvent.click(screen.getByRole('button', { name: 'Take down' }))
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(screen.getByRole('button', { name: 'Take down' })).toBeInTheDocument()
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })

  it('shows not found on lookup', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(404, JSON.stringify({ error: 'NOT_FOUND' }))))
    render(<AdminPage />)
    await lookUp('missing')
    expect(await screen.findByRole('alert')).toHaveTextContent('Link not found.')
  })

  it('explains an expired Access session', async () => {
    vi.stubGlobal(
      'fetch',
      vi
        .fn()
        .mockResolvedValueOnce(jsonResponse(200, STATS))
        .mockResolvedValueOnce({ ok: false, status: 0, type: 'opaqueredirect', text: async () => '' }),
    )
    render(<AdminPage />)
    await lookUp('abc1234')
    await screen.findByText('3')
    fireEvent.change(screen.getByLabelText('Reason'), { target: { value: 'spam' } })
    fireEvent.click(screen.getByRole('button', { name: 'Take down' }))
    fireEvent.click(screen.getByRole('button', { name: 'Confirm takedown' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('session has expired')
  })
})
