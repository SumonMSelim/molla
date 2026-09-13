import { Chrome, LegalDoc } from './Chrome.tsx'

export function AboutPage() {
  return (
    <Chrome width="prose">
      <LegalDoc title="About" updated="13 September 2026">
        <p>
          mol.la is an open-source URL shortener. It turns a long http(s) URL into a short code, redirects with HTTP 302, and
          exposes create, stats, and delete over a JSON API.
        </p>
        <h2>What it does</h2>
        <ul>
          <li>Optional custom aliases and expiry.</li>
          <li>Click counts that are eventually consistent, not billing-grade.</li>
          <li>Owner delete and operator takedown without fetching the destination.</li>
        </ul>
        <h2>What it does not do</h2>
        <p>
          The write path never fetches the target URL. There is no preview, liveness check, or reputation crawl on create. The
          first release does not issue credentials from this UI.
        </p>
        <h2>Project</h2>
        <p>
          mol.la is MIT-licensed. Source, issues, and the security policy live in the public repository. Report suspected
          vulnerabilities privately as SECURITY.md describes — not in a public issue.
        </p>
      </LegalDoc>
    </Chrome>
  )
}
