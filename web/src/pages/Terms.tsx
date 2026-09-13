import { Chrome, LegalDoc } from './Chrome.tsx'

export function TermsPage() {
  return (
    <Chrome width="prose">
      <LegalDoc title="Terms and conditions" updated="14 September 2026">
        <p>These terms apply to anyone using mol.la to create or follow a short link.</p>
        <h2>No warranty</h2>
        <p>
          mol.la is provided "as is", without warranty of any kind. Availability, stats accuracy, and continued operation
          of any individual link are not guaranteed.
        </p>
        <h2>Acceptable use</h2>
        <ul>
          <li>Only shorten http and https addresses you are allowed to share.</li>
          <li>Do not use mol.la for phishing, malware, fraud, spam, or other unlawful activity.</li>
          <li>Do not attempt to bypass rate limits or takedown.</li>
          <li>Do not probe, scrape, or overload the service beyond ordinary use.</li>
        </ul>
        <h2>Links and takedown</h2>
        <p>
          Destination addresses are stored as submitted. We may disable any link at our discretion, including for
          suspected abuse, without prior notice. Deleted or expired codes return a not-found response. A short link may
          stop resolving if the underlying record is removed or expires.
        </p>
        <h2>Liability</h2>
        <p>
          To the extent permitted by law, mol.la's operators are not liable for damages arising from use of the service,
          including lost clicks, broken redirects, or the content of any destination behind a short link.
        </p>
        <h2>Changes</h2>
        <p>These terms may change as the service changes. The date at the top is the current version.</p>
      </LegalDoc>
    </Chrome>
  )
}
