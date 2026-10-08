import { fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { AdminPage } from './Admin.tsx'

const STATS = JSON.stringify({ short_code: 'abc1234', short_url: 'https://mol.la/abc1234', clicks: 3, created_at: '2026-09-13T00:00:00Z' })

const AUDIT_EVENT = {
  actor_id: 'ops@example.com',
  role: 'operator',
  owner_id: '',
  short_code: 'old0000',
  reason: 'malware',
  outcome: 'deleted',
  ts: '2026-10-01T00:00:00Z',
}

function jsonResponse(status: number, body: string) {
  return { ok: status < 400, status, type: 'basic', text: async () => body }
}

// The page loads the audit log on mount, so every test answers that route
// and queues the lookup/takedown responses for the others.
function fetchFor(audit: unknown, ...rest: unknown[]) {
  const queue = [...rest]
  return vi.fn(async (path: string, _init?: RequestInit) => (path === '/admin/v1/audit' ? audit : queue.shift()))
}

const AUDIT_EMPTY = jsonResponse(200, JSON.stringify({ events: [] }))

afterEach(() => {
  vi.unstubAllGlobals()
})

async function lookUp(code: string) {
  fireEvent.change(screen.getByLabelText('Short code'), { target: { value: code } })
  fireEvent.click(screen.getByRole('button', { name: 'Look up' }))
}

describe('AdminPage', () => {
  it('looks up a link and takes it down after confirmation', async () => {
    const fetchMock = fetchFor(
      AUDIT_EMPTY,
      jsonResponse(200, STATS),
      jsonResponse(200, JSON.stringify({ short_code: 'abc1234', deleted_at: '2026-10-08T00:00:00Z', version: 2 })),
    )
    vi.stubGlobal('fetch', fetchMock)
    render(<AdminPage />)
    expect(await screen.findByText('No takedowns yet.')).toBeInTheDocument()

    await lookUp('abc1234')
    expect(await screen.findByText('3')).toBeInTheDocument()

    const takeDown = screen.getByRole('button', { name: 'Take down' })
    expect(takeDown).toBeDisabled()
    fireEvent.change(screen.getByLabelText('Reason'), { target: { value: 'phishing' } })
    expect(takeDown).toBeEnabled()
    fireEvent.click(takeDown)
    expect(fetchMock).toHaveBeenCalledTimes(2)

    fireEvent.click(screen.getByRole('button', { name: 'Confirm takedown' }))
    expect(await screen.findByRole('status')).toHaveTextContent('Deleted at 2026-10-08T00:00:00Z')
    const [path, init] = fetchMock.mock.calls[2] as [string, RequestInit]
    expect(path).toBe('/admin/v1/links/abc1234/takedown')
    expect(init.method).toBe('POST')
    expect(JSON.parse(String(init.body))).toEqual({ reason: 'phishing' })
    expect(screen.queryByRole('button', { name: 'Confirm takedown' })).not.toBeInTheDocument()
    // The audit log reloads after a takedown.
    expect(fetchMock.mock.calls.filter(([p]) => p === '/admin/v1/audit')).toHaveLength(2)
  })

  it('lists recent takedowns from the audit log', async () => {
    vi.stubGlobal('fetch', fetchFor(jsonResponse(200, JSON.stringify({ events: [AUDIT_EVENT] }))))
    render(<AdminPage />)
    const row = (await screen.findByText('old0000')).closest('tr')
    expect(row).toHaveTextContent('2026-10-01T00:00:00Z')
    expect(row).toHaveTextContent('ops@example.com')
    expect(row).toHaveTextContent('malware')
    expect(row).toHaveTextContent('deleted')
  })

  it('shows an alert when the audit log cannot load', async () => {
    vi.stubGlobal('fetch', fetchFor(jsonResponse(503, JSON.stringify({ error: 'TEMPORARILY_UNAVAILABLE' }))))
    render(<AdminPage />)
    expect(await screen.findByRole('alert')).toHaveTextContent('Service temporarily unavailable.')
  })

  it('cancel keeps the link', async () => {
    const fetchMock = fetchFor(AUDIT_EMPTY, jsonResponse(200, STATS))
    vi.stubGlobal('fetch', fetchMock)
    render(<AdminPage />)

    await lookUp('abc1234')
    await screen.findByText('3')
    fireEvent.change(screen.getByLabelText('Reason'), { target: { value: 'spam' } })
    fireEvent.click(screen.getByRole('button', { name: 'Take down' }))
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(screen.getByRole('button', { name: 'Take down' })).toBeInTheDocument()
    expect(fetchMock).toHaveBeenCalledTimes(2)
  })

  it('shows not found on lookup', async () => {
    vi.stubGlobal('fetch', fetchFor(AUDIT_EMPTY, jsonResponse(404, JSON.stringify({ error: 'NOT_FOUND' }))))
    render(<AdminPage />)
    await lookUp('missing')
    expect(await screen.findByRole('alert')).toHaveTextContent('Link not found.')
  })

  it('explains an expired Access session', async () => {
    vi.stubGlobal(
      'fetch',
      fetchFor(AUDIT_EMPTY, jsonResponse(200, STATS), { ok: false, status: 0, type: 'opaqueredirect', text: async () => '' }),
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
