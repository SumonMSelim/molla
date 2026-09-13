import { useEffect, useState } from 'react'
import { ApiError, forgetLink, getStats, type LinkStats } from '../api.ts'

type LinkPageProps = {
  code: string
  onBack: () => void
  onForgotten?: (code: string) => void
}

export function LinkPage({ code, onBack, onForgotten }: LinkPageProps) {
  const [stats, setStats] = useState<LinkStats | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    let cancelled = false
    setError('')
    setStats(null)
    void (async () => {
      try {
        const result = await getStats(code)
        if (!cancelled) {
          setStats(result)
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof ApiError ? err.message : 'Something went wrong.')
        }
      }
    })()
    return () => {
      cancelled = true
    }
  }, [code])

  function onForget() {
    forgetLink(code)
    onForgotten?.(code)
    onBack()
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

      {error ? (
        <p className="bg-destructive-soft px-3 py-2 text-sm text-destructive" role="alert">
          {error}
        </p>
      ) : null}

      {stats ? (
        <dl className="border border-border">
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
        onClick={onForget}
        className="mt-5 inline-flex h-9 items-center border border-input px-3.5 text-sm text-muted-foreground hover:border-brand hover:text-foreground"
      >
        Remove from this list
      </button>
      <p className="mt-2 text-xs text-muted-foreground">
        This only forgets the link in this browser. The short link itself keeps working until it expires.
      </p>
    </section>
  )
}
