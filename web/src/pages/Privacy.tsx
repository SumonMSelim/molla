import { Chrome, LegalDoc } from './Chrome.tsx'

export function PrivacyPage() {
  return (
    <Chrome width="prose">
      <LegalDoc title="Privacy" updated="14 September 2026">
        <p>
          This notice describes what the mol.la UI and a default mol.la deployment handle. A self-hosted instance may log more;
          ask that operator.
        </p>
        <h2>This browser</h2>
        <ul>
          <li>The API key stays in sessionStorage for this tab session. It is never written to localStorage.</li>
          <li>Short codes created in this UI may be cached in localStorage so this browser can show them again later.</li>
          <li>We do not set advertising cookies.</li>
        </ul>
        <h2>API and redirects</h2>
        <ul>
          <li>Create stores the destination URL, owner id, timestamps, and optional alias, kept for up to 1,825 days by default.</li>
          <li>Developer tokens are stored as SHA-256 hashes, not in the raw form you type.</li>
          <li>
            Redirects that reach the origin may emit a click event with a hashed source address (HMAC with a rotating daily
            key). The first release does not collect user-agent or referrer.
          </li>
          <li>Click totals are approximate. Edge cache hits are not counted the same way as origin hits.</li>
          <li>Raw click-event records are retained for 90 days in an encrypted archive, then deleted.</li>
          <li>
            A deleted link is marked inactive immediately; the underlying record is purged roughly 30 days later, not
            instantly.
          </li>
        </ul>
        <h2>Network path</h2>
        <p>
          mol.la sits behind Cloudflare, which proxies every request before it reaches our infrastructure and can see
          connection metadata (IP address, timing) the same way any reverse proxy in front of a website can. See{' '}
          <a href="https://www.cloudflare.com/privacypolicy/">Cloudflare's privacy policy</a> for how they handle that.
        </p>
        <h2>What we do not collect here</h2>
        <p>
          This UI has no account signup, no payment form, and no analytics SDK. It talks only to same-origin API paths.
        </p>
        <h2>Contact</h2>
        <p>
          For a suspected vulnerability, use the private reporting channels in SECURITY.md. Do not attach API keys or live
          malicious URLs.
        </p>
      </LegalDoc>
    </Chrome>
  )
}
