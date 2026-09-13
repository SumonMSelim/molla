import { Chrome, LegalDoc } from './Chrome.tsx'

export function TermsPage() {
  return (
    <Chrome width="prose">
      <LegalDoc title="Terms and conditions" updated="13 September 2026">
        <p>
          These terms apply to use of mol.la and this web UI. The software itself is also licensed under the MIT
          License; that license governs the code. These terms govern use of a running instance.
        </p>
        <h2>No warranty</h2>
        <p>
          mol.la is provided “as is”, without warranty of any kind. Availability, stats accuracy, and continued operation are
          not guaranteed. The project is under active development and has no supported release yet.
        </p>
        <h2>Acceptable use</h2>
        <ul>
          <li>Only shorten http and https URLs you are allowed to share.</li>
          <li>Do not use mol.la for phishing, malware, fraud, or other unlawful activity.</li>
          <li>Do not attempt to bypass authentication, quotas, or takedown.</li>
          <li>Do not probe, scrape, or overload the redirect path beyond ordinary use.</li>
        </ul>
        <h2>Your credentials</h2>
        <p>
          API keys identify a caller. You must keep them secret, treat them as passwords, and stop using a key if it leaks.
          Requests you make with a key are your responsibility.
        </p>
        <h2>Links and takedown</h2>
        <p>
          Destination URLs are stored as you submitted them. Operators may disable any link. Deleted or expired codes return
          404. Short URLs may stop working if the instance is shut down or the record is purged.
        </p>
        <h2>Liability</h2>
        <p>
          To the extent permitted by law, the authors and operators are not liable for damages arising from use of mol.la,
          including lost clicks, broken redirects, or third-party content behind a short link.
        </p>
        <h2>Changes</h2>
        <p>These terms may change as the service changes. The date at the top is the current version.</p>
      </LegalDoc>
    </Chrome>
  )
}
