export type RedirectLink = {
  longURL: string
  isActive: boolean
  expiresAt: number // unix seconds
}

export type RedirectDecision = { found: false } | { found: true; longURL: string }

// decideRedirect returns not found for unknown, inactive, or expired links.
export function decideRedirect(link: RedirectLink | null, now: number): RedirectDecision {
  if (link === null || !link.isActive || now >= link.expiresAt) {
    return { found: false }
  }
  return { found: true, longURL: link.longURL }
}
