import { useId, useState, type FormEvent } from 'react'
import { ApiError, createLink, rememberLink, type StoredLink } from '../api.ts'
import { Chrome } from './Chrome.tsx'
import { LinkPage } from './Link.tsx'

const fieldClass =
  'flex h-12 w-full border border-input bg-background px-3.5 text-sm outline-none placeholder:text-muted-foreground focus-visible:border-brand focus-visible:glow'

type CreatePageProps = {
  selected: string | null
  onOpen: (code: string) => void
  onClear: () => void
}

export function CreatePage({ selected, onOpen, onClear }: CreatePageProps) {
  const urlId = useId()
  const aliasId = useId()
  const expiryId = useId()
  const [longUrl, setLongUrl] = useState('')
  const [alias, setAlias] = useState('')
  const [expiresIn, setExpiresIn] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [created, setCreated] = useState<StoredLink | null>(null)
  const [copied, setCopied] = useState(false)

  async function onSubmit(event: FormEvent) {
    event.preventDefault()
    setError('')
    setCopied(false)
    setBusy(true)
    try {
      const expires_in = expiresIn.trim() === '' ? undefined : Number(expiresIn)
      const result = await createLink({
        long_url: longUrl.trim(),
        alias: alias.trim() || undefined,
        expires_in,
      })
      rememberLink({
        short_code: result.short_code,
        short_url: result.short_url,
        long_url: result.long_url,
        created_at: result.created_at,
      })
      setCreated({
        short_code: result.short_code,
        short_url: result.short_url,
        long_url: result.long_url,
        created_at: result.created_at,
      })
      onOpen(result.short_code)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Something went wrong.')
    } finally {
      setBusy(false)
    }
  }

  async function copy(url: string) {
    await navigator.clipboard.writeText(url)
    setCopied(true)
  }

  return (
    <Chrome>
        <div className="animate-fade-up pb-10 text-center sm:pb-14">
          <p className="font-mono text-xs font-medium tracking-[0.2em] text-brand-text uppercase glow-text">
            open · public · no personal data collected
          </p>
          <h1 className="mt-4 text-display font-semibold text-balance">
            Long links,<br />
            <span className="text-brand glow-text">shortened.</span>
          </h1>
          <p className="mx-auto mt-5 max-w-md text-prose text-muted-foreground text-balance">
            Paste a URL, get a short one back. No sign-up, no waiting: anyone can create a link.
          </p>
          <p className="mt-5 text-xs text-muted-foreground">
            Integrating from your app?{' '}
            <a href="/app/developers" className="font-medium text-brand-text hover:text-foreground">
              Developers API docs
            </a>
          </p>
        </div>

        <form onSubmit={onSubmit} className="relative animate-fade-up pt-5 [animation-delay:80ms]">
          <div className="absolute top-0 left-6 z-10 border border-b-0 border-border bg-card px-4 py-2 text-sm font-medium">
            Short link
          </div>
          <div className="border border-border bg-card px-6 pb-6 pt-10 sm:px-8 sm:pb-8">
            <h1 className="animate-fade-up text-title font-semibold [animation-delay:140ms]">Shorten a long link</h1>
            <div className="mt-6 flex flex-col gap-2">
              <label className="text-sm font-medium text-brand-text" htmlFor={urlId}>
                Long URL
              </label>
              <p className="text-xs text-muted-foreground">Paste your long link here</p>
              <input
                id={urlId}
                name="long_url"
                type="url"
                value={longUrl}
                onChange={(e) => setLongUrl(e.target.value)}
                className={`${fieldClass} font-mono text-code`}
                placeholder="https://sumonselim.com/molla-url-shortener"
                required
              />
            </div>
            <div className="mt-5 flex flex-col gap-4 sm:flex-row sm:items-start">
              <button
                type="submit"
                disabled={busy}
                className="inline-flex h-12 shrink-0 items-center justify-center bg-brand px-5 text-sm font-medium text-brand-foreground hover:bg-brand/90 hover:glow focus-visible:glow disabled:opacity-50"
              >
                {busy ? 'Creating…' : 'Create short link'}
              </button>
              <details className="min-w-0 flex-1 border border-border">
                <summary className="cursor-pointer px-3.5 py-3 text-sm text-muted-foreground hover:text-foreground">
                  Alias and expiry
                </summary>
                <div className="grid gap-5 border-t border-border p-3.5 sm:grid-cols-2">
                  <div className="flex flex-col gap-2">
                    <label className="text-sm font-medium" htmlFor={aliasId}>
                      Alias (optional)
                    </label>
                    <input
                      id={aliasId}
                      name="alias"
                      value={alias}
                      onChange={(e) => setAlias(e.target.value)}
                      className={fieldClass}
                    />
                  </div>
                  <div className="flex flex-col gap-2">
                    <label className="text-sm font-medium" htmlFor={expiryId}>
                      Expires in seconds (optional)
                    </label>
                    <input
                      id={expiryId}
                      name="expires_in"
                      type="number"
                      min={60}
                      max={157680000}
                      value={expiresIn}
                      onChange={(e) => setExpiresIn(e.target.value)}
                      className={fieldClass}
                    />
                  </div>
                </div>
              </details>
            </div>
            {error ? (
              <p className="mt-5 bg-destructive-soft px-3 py-2 text-sm text-destructive" role="alert">
                {error}
              </p>
            ) : null}
          </div>
        </form>

        {created ? (
          <div className="relative mt-8 animate-fade-up overflow-hidden border border-brand/40 bg-card p-6 glow">
            <div className="pointer-events-none absolute inset-0 animate-sheen" />
            <p className="text-xs font-medium tracking-[0.04em] text-muted-foreground uppercase">Short URL</p>
            <p className="mt-3 break-all font-mono text-title text-brand glow-text">{created.short_url}</p>
            <button
              type="button"
              className="mt-5 inline-flex h-9 items-center border border-input px-3.5 text-sm hover:border-brand hover:glow"
              onClick={() => void copy(created.short_url)}
            >
              {copied ? 'Copied' : 'Copy'}
            </button>
          </div>
        ) : null}

        {selected ? (
          <div className="mt-8">
            <LinkPage
              code={selected}
              onBack={onClear}
              onForgotten={(code) => {
                if (created?.short_code === code) {
                  setCreated(null)
                }
                onClear()
              }}
            />
          </div>
        ) : (
          <dl className="mt-16 grid gap-6 sm:grid-cols-3">
            {FEATURES.map((feature, i) => (
              <div
                key={feature.title}
                className="animate-fade-up border border-border bg-card p-5"
                style={{ animationDelay: `${200 + i * 80}ms` }}
              >
                <dt className="font-mono text-xs text-brand-text">{feature.tag}</dt>
                <dd className="mt-2 text-sm font-medium">{feature.title}</dd>
                <dd className="mt-1 text-xs text-muted-foreground">{feature.body}</dd>
              </div>
            ))}
          </dl>
        )}
    </Chrome>
  )
}

const FEATURES = [
  { tag: '01', title: 'No account needed', body: 'Create short links for any link without signing up.' },
  { tag: '02', title: 'Kept for 5 years', body: 'Links last up to 1,825 days by default, or set your own expiry.' },
  { tag: '03', title: 'No personal data collected', body: 'We do not collect any personal data from you.' },
] as const
