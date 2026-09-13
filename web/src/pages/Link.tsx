import { useEffect, useState } from 'react'
import { ApiError, deleteLink, forgetLink, getApiKey, getStats, setApiKey, type LinkStats } from '../api.ts'

type LinkPageProps = {
  code: string
  onBack: () => void
  onDeleted?: (code: string) => void
}

export function LinkPage({ code, onBack, onDeleted }: LinkPageProps) {
  const [apiKey, setApiKeyField] = useState(getApiKey)
  const [stats, setStats] = useState<LinkStats | null>(null)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    let cancelled = false
    if (apiKey.trim() === '') {
      return
    }
    setApiKey(apiKey)
    void (async () => {
      try {
        const result = await getStats(code, apiKey)
        if (!cancelled) {
          setStats(result)
          setError('')
        }
      } catch (err) {
        if (!cancelled) {
          setStats(null)
          setError(err instanceof ApiError ? err.message : 'Something went wrong.')
        }
      }
    })()
    return () => {
      cancelled = true
    }
  }, [apiKey, code])

  async function onDelete() {
    if (!window.confirm(`Delete ${code}? This cannot be undone from the UI.`)) {
      return
    }
    setBusy(true)
    setApiKey(apiKey)
    try {
      await deleteLink(code, apiKey)
      forgetLink(code)
      onDeleted?.(code)
      onBack()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Something went wrong.')
    } finally {
      setBusy(false)
    }
  }

  return (
    <section className="border border-border bg-card p-5">
      <div className="mb-5 flex items-start justify-between gap-3">
        <div>
          <h2 className="text-section font-semibold">Link</h2>
          <p className="mt-1 font-mono text-xs text-brand">{code}</p>
        </div>
        <button type="button" className="text-xs text-muted-foreground hover:text-foreground" onClick={onBack}>
          Close
        </button>
      </div>

      <div className="flex flex-col gap-2">
        <label className="text-sm font-medium" htmlFor="api-key">
          API key
        </label>
        <input
          id="api-key"
          type="password"
          autoComplete="off"
          value={apiKey}
          onChange={(e) => setApiKeyField(e.target.value)}
          className="flex h-9 w-full border border-input bg-background px-3.5 text-sm outline-none focus-visible:border-brand focus-visible:glow"
        />
      </div>

      {error ? (
        <p className="mt-5 bg-destructive-soft px-3 py-2 text-sm text-destructive" role="alert">
          {error}
        </p>
      ) : null}

      {stats ? (
        <dl className="mt-5 border border-border">
          <div className="flex justify-between gap-4 px-3 py-2">
            <dt className="text-sm text-muted-foreground">Clicks</dt>
            <dd className="font-mono text-sm tabular-nums">{stats.clicks}</dd>
          </div>
          <div className="flex justify-between gap-4 border-t border-border px-3 py-2">
            <dt className="text-sm text-muted-foreground">Created</dt>
            <dd className="font-mono text-xs">{stats.created_at}</dd>
          </div>
          <div className="flex justify-between gap-4 border-t border-border px-3 py-2">
            <dt className="text-sm text-muted-foreground">Last click</dt>
            <dd className="font-mono text-xs">{stats.last_click_at ?? '—'}</dd>
          </div>
        </dl>
      ) : null}

      <button
        type="button"
        disabled={busy}
        onClick={() => void onDelete()}
        className="mt-5 inline-flex h-9 items-center bg-destructive-soft px-3.5 text-sm font-medium text-destructive hover:bg-destructive/20 disabled:opacity-50"
      >
        {busy ? 'Deleting…' : 'Delete link'}
      </button>
    </section>
  )
}
