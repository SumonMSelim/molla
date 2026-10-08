import { useId, useState, type FormEvent } from 'react'
import { ApiError, getStats, takedownLink, type LinkStats, type TakedownResult } from '../api.ts'
import { Chrome } from './Chrome.tsx'

const fieldClass =
  'flex h-12 w-full border border-input bg-background px-3.5 text-sm outline-none placeholder:text-muted-foreground focus-visible:border-brand focus-visible:glow'

function describe(err: unknown): string {
  return err instanceof ApiError ? err.message : 'Something went wrong.'
}

export function AdminPage() {
  const codeId = useId()
  const reasonId = useId()
  const [code, setCode] = useState('')
  const [reason, setReason] = useState('')
  const [stats, setStats] = useState<LinkStats | null>(null)
  const [confirming, setConfirming] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [result, setResult] = useState<TakedownResult | null>(null)

  async function onLookup(event: FormEvent) {
    event.preventDefault()
    setError('')
    setResult(null)
    setStats(null)
    setConfirming(false)
    setBusy(true)
    try {
      setStats(await getStats(code.trim()))
    } catch (err) {
      setError(describe(err))
    } finally {
      setBusy(false)
    }
  }

  async function onTakedown() {
    if (stats === null) {
      return
    }
    setError('')
    setBusy(true)
    try {
      setResult(await takedownLink(stats.short_code, reason.trim()))
      setStats(null)
      setConfirming(false)
    } catch (err) {
      setError(describe(err))
    } finally {
      setBusy(false)
    }
  }

  const reasonOk = reason.trim().length > 0 && reason.trim().length <= 256

  return (
    <Chrome>
      <div className="animate-fade-up pb-10">
        <p className="font-mono text-xs font-medium tracking-[0.12em] text-brand-text uppercase glow-text">operator</p>
        <h1 className="mt-4 text-display font-semibold">Takedown</h1>
        <p className="mt-5 text-prose text-muted-foreground">
          Look up a short link, then remove it. Removal is a soft delete: the link stops resolving immediately and
          the action is recorded in the audit log under your Access identity.
        </p>
      </div>

      <form onSubmit={onLookup} className="border border-border bg-card p-6 sm:p-8">
        <div className="flex flex-col gap-2">
          <label className="text-sm font-medium text-brand-text" htmlFor={codeId}>
            Short code
          </label>
          <input
            id={codeId}
            name="short_code"
            value={code}
            onChange={(e) => setCode(e.target.value)}
            className={`${fieldClass} font-mono text-code`}
            placeholder="aB3xK9c"
            required
          />
        </div>
        <button
          type="submit"
          disabled={busy}
          className="mt-5 inline-flex h-12 items-center justify-center border border-input px-5 text-sm font-medium hover:border-brand hover:glow disabled:opacity-50"
        >
          {busy && stats === null ? 'Looking up…' : 'Look up'}
        </button>
      </form>

      {error ? (
        <p className="mt-5 bg-destructive-soft px-3 py-2 text-sm text-destructive" role="alert">
          {error}
        </p>
      ) : null}

      {stats ? (
        <section className="mt-8 border border-border bg-card p-6 sm:p-8">
          <h2 className="text-section font-semibold">Link</h2>
          <dl className="mt-4 border border-border">
            <div className="flex justify-between gap-4 px-3 py-2">
              <dt className="text-sm text-muted-foreground">Short URL</dt>
              <dd className="break-all font-mono text-xs">{stats.short_url}</dd>
            </div>
            <div className="flex justify-between gap-4 border-t border-border px-3 py-2">
              <dt className="text-sm text-muted-foreground">Clicks</dt>
              <dd className="font-mono text-sm tabular-nums">{stats.clicks}</dd>
            </div>
            <div className="flex justify-between gap-4 border-t border-border px-3 py-2">
              <dt className="text-sm text-muted-foreground">Created</dt>
              <dd className="font-mono text-xs">{stats.created_at}</dd>
            </div>
            <div className="flex justify-between gap-4 border-t border-border px-3 py-2">
              <dt className="text-sm text-muted-foreground">Last click</dt>
              <dd className="font-mono text-xs">{stats.last_click_at ?? 'None yet'}</dd>
            </div>
          </dl>

          <div className="mt-6 flex flex-col gap-2">
            <label className="text-sm font-medium" htmlFor={reasonId}>
              Reason
            </label>
            <p className="text-xs text-muted-foreground">Recorded in the audit log. Up to 256 characters.</p>
            <input
              id={reasonId}
              name="reason"
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              className={fieldClass}
              placeholder="phishing"
              maxLength={256}
            />
          </div>

          {confirming ? (
            <div className="mt-5 border border-destructive/40 bg-destructive-soft p-4">
              <p className="text-sm">
                Take down <span className="font-mono text-brand">{stats.short_code}</span>? This cannot be undone from
                here.
              </p>
              <div className="mt-4 flex gap-3">
                <button
                  type="button"
                  disabled={busy}
                  onClick={() => void onTakedown()}
                  className="inline-flex h-10 items-center bg-destructive px-4 text-sm font-medium text-background disabled:opacity-50"
                >
                  {busy ? 'Taking down…' : 'Confirm takedown'}
                </button>
                <button
                  type="button"
                  disabled={busy}
                  onClick={() => setConfirming(false)}
                  className="inline-flex h-10 items-center border border-input px-4 text-sm"
                >
                  Cancel
                </button>
              </div>
            </div>
          ) : (
            <button
              type="button"
              disabled={!reasonOk || busy}
              onClick={() => setConfirming(true)}
              className="mt-5 inline-flex h-12 items-center justify-center bg-destructive px-5 text-sm font-medium text-background disabled:opacity-50"
            >
              Take down
            </button>
          )}
        </section>
      ) : null}

      {result ? (
        <div className="mt-8 border border-brand/40 bg-card p-6 glow" role="status">
          <p className="text-xs font-medium tracking-[0.04em] text-muted-foreground uppercase">Taken down</p>
          <p className="mt-3 font-mono text-title text-brand glow-text">{result.short_code}</p>
          <p className="mt-2 text-xs text-muted-foreground">Deleted at {result.deleted_at}</p>
        </div>
      ) : null}
    </Chrome>
  )
}
