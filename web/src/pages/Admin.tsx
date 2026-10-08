import { useEffect, useId, useState, type FormEvent } from 'react'
import {
  ApiError,
  getAudit,
  getStats,
  getTopLinks,
  takedownLink,
  type AuditEvent,
  type LinkStats,
  type TakedownResult,
  type TopLink,
} from '../api.ts'
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
  const [events, setEvents] = useState<AuditEvent[] | null>(null)
  const [auditError, setAuditError] = useState('')
  const [top, setTop] = useState<TopLink[] | null>(null)
  const [topError, setTopError] = useState('')

  useEffect(() => {
    void loadTop()
    void loadAudit()
  }, [])

  async function loadTop() {
    try {
      setTop(await getTopLinks())
      setTopError('')
    } catch (err) {
      setTopError(describe(err))
    }
  }

  async function loadAudit() {
    try {
      setEvents(await getAudit())
      setAuditError('')
    } catch (err) {
      setAuditError(describe(err))
    }
  }

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
      void loadTop()
      void loadAudit()
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

      <section className="mt-8 border border-border bg-card p-6 sm:p-8">
        <h2 className="text-section font-semibold">Top links</h2>
        <p className="mt-2 text-xs text-muted-foreground">
          The 20 most-clicked live links. Pick a code to load it into the lookup above.
        </p>
        {topError ? (
          <p className="mt-4 bg-destructive-soft px-3 py-2 text-sm text-destructive" role="alert">
            {topError}
          </p>
        ) : top === null ? (
          <p className="mt-4 text-sm text-muted-foreground">Loading…</p>
        ) : top.length === 0 ? (
          <p className="mt-4 text-sm text-muted-foreground">No clicks yet.</p>
        ) : (
          <table className="mt-4 w-full border border-border text-left text-xs">
            <thead className="text-muted-foreground">
              <tr>
                <th className="px-3 py-2 font-medium">Code</th>
                <th className="px-3 py-2 font-medium">Destination</th>
                <th className="px-3 py-2 text-right font-medium">Clicks</th>
                <th className="px-3 py-2 font-medium">Last click</th>
              </tr>
            </thead>
            <tbody>
              {top.map((link) => (
                <tr key={link.short_code} className="border-t border-border">
                  <td className="px-3 py-2">
                    <button
                      type="button"
                      onClick={() => setCode(link.short_code)}
                      className="font-mono text-brand hover:glow-text"
                    >
                      {link.short_code}
                    </button>
                  </td>
                  <td className="break-all px-3 py-2">{link.long_url}</td>
                  <td className="px-3 py-2 text-right font-mono tabular-nums">{link.clicks}</td>
                  <td className="px-3 py-2 font-mono whitespace-nowrap">{link.last_click_at ?? 'None yet'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </section>

      <section className="mt-8 border border-border bg-card p-6 sm:p-8">
        <h2 className="text-section font-semibold">Recent takedowns</h2>
        <p className="mt-2 text-xs text-muted-foreground">Last 50 events from the audit log, newest first.</p>
        {auditError ? (
          <p className="mt-4 bg-destructive-soft px-3 py-2 text-sm text-destructive" role="alert">
            {auditError}
          </p>
        ) : events === null ? (
          <p className="mt-4 text-sm text-muted-foreground">Loading…</p>
        ) : events.length === 0 ? (
          <p className="mt-4 text-sm text-muted-foreground">No takedowns yet.</p>
        ) : (
          <table className="mt-4 w-full border border-border text-left text-xs">
            <thead className="text-muted-foreground">
              <tr>
                <th className="px-3 py-2 font-medium">When</th>
                <th className="px-3 py-2 font-medium">Code</th>
                <th className="px-3 py-2 font-medium">Actor</th>
                <th className="px-3 py-2 font-medium">Reason</th>
                <th className="px-3 py-2 font-medium">Outcome</th>
              </tr>
            </thead>
            <tbody>
              {events.map((event, i) => (
                <tr key={`${event.ts}-${event.short_code}-${i}`} className="border-t border-border">
                  <td className="px-3 py-2 font-mono whitespace-nowrap">{event.ts}</td>
                  <td className="px-3 py-2 font-mono text-brand">{event.short_code}</td>
                  <td className="break-all px-3 py-2">{event.actor_id}</td>
                  <td className="px-3 py-2">{event.reason}</td>
                  <td className="px-3 py-2 font-mono">{event.outcome}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </section>
    </Chrome>
  )
}
