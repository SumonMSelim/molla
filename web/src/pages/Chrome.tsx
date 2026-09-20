import type { ReactNode } from 'react'

const links = [
  { href: '/app/about', label: 'About' },
  { href: '/app/developers', label: 'Developers' },
  { href: '/app/terms', label: 'Terms' },
  { href: '/app/privacy', label: 'Privacy' },
] as const

export function Chrome({
  trailing,
  children,
  width = 'composer',
}: {
  trailing?: ReactNode
  children: ReactNode
  width?: 'composer' | 'prose'
}) {
  const max = width === 'prose' ? 'max-w-2xl' : 'max-w-3xl'
  return (
    <div className="relative flex min-h-svh flex-col bg-background bg-dot-grid">
      <div className="pointer-events-none absolute inset-x-0 top-0 h-[32rem] origin-top animate-ambient-pulse bg-[radial-gradient(ellipse_at_top,var(--glow),transparent_60%)]" />
      <div className={`relative mx-auto flex w-full flex-1 flex-col ${max} px-6 py-10 sm:py-16`}>
        <header className="mb-10 flex animate-fade-up items-center justify-between gap-4">
          <a href="/app/" className="font-mono text-sm font-medium tracking-tight text-brand glow-text">
            mol.la
          </a>
          {trailing}
        </header>
        <div className="flex-1">{children}</div>
        <footer className="mt-16 flex flex-col gap-3 border-t border-border pt-6 text-xs text-muted-foreground sm:flex-row sm:items-center sm:justify-between">
          <nav className="flex gap-4" aria-label="Legal">
            {links.map((link) => (
              <a key={link.href} href={link.href} className="hover:text-foreground">
                {link.label}
              </a>
            ))}
          </nav>
          <p>© 2026 Muhammad Sumon Molla Selim</p>
        </footer>
      </div>
    </div>
  )
}

export function LegalDoc({ title, updated, children }: { title: string; updated: string; children: ReactNode }) {
  return (
    <article>
      <p className="font-mono text-xs font-medium tracking-tight text-muted-foreground">mol.la</p>
      <h1 className="mt-2 text-title font-semibold">{title}</h1>
      <p className="mt-2 text-xs text-muted-foreground">Last updated {updated}</p>
      <div className="mt-8 space-y-4 text-prose text-muted-foreground [&_a]:text-brand-text [&_a]:underline-offset-4 hover:[&_a]:underline [&_h2]:pt-4 [&_h2]:text-section [&_h2]:font-semibold [&_h2]:text-foreground [&_ul]:list-disc [&_ul]:space-y-2 [&_ul]:pl-5">
        {children}
      </div>
    </article>
  )
}
