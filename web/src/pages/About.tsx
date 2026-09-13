import { Chrome, LegalDoc } from './Chrome.tsx'

export function AboutPage() {
  return (
    <Chrome width="prose">
      <LegalDoc title="About" updated="14 September 2026">
        <p>
          mol.la turns a long web address into a short link that redirects to it. Anyone can create a short link; no account
          or sign-up is required.
        </p>
        <h2>What it does</h2>
        <ul>
          <li>Shortens any http or https address into a short code you can share.</li>
          <li>Optional custom alias and expiry, up to 1,825 days (5 years) by default.</li>
          <li>Click counts for any short link, viewable by anyone who has the code.</li>
        </ul>
        <h2>What it does not do</h2>
        <p>
          Creating a link never visits or previews the destination. There is no crawler, no scanner, and no ranking of
          links. mol.la does not endorse, verify, or vouch for what a short link points to.
        </p>
        <h2>Open source</h2>
        <p>mol.la is open-source and MIT-licensed.</p>
      </LegalDoc>
    </Chrome>
  )
}
