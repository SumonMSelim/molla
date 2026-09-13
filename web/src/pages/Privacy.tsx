import { Chrome, LegalDoc } from './Chrome.tsx'

export function PrivacyPage() {
  return (
    <Chrome width="prose">
      <LegalDoc title="Privacy" updated="14 September 2026">
        <p>This notice describes what mol.la collects when you create or follow a short link.</p>
        <h2>This browser</h2>
        <ul>
          <li>Short links you create in this browser are kept in this browser's local storage so you can find them again.</li>
          <li>This list is local only; it is never sent to us or shown to anyone else.</li>
          <li>We do not set advertising cookies.</li>
        </ul>
        <h2>Creating a link</h2>
        <ul>
          <li>We store the destination address, creation time, and optional alias, kept for up to 1,825 days by default.</li>
          <li>Click counts are public: anyone who has a short code can view its click count.</li>
        </ul>
        <h2>Following a link</h2>
        <ul>
          <li>
            A redirect may record a click event with your source address hashed (not stored in plain form) and rotated
            daily. We do not collect browser or referrer information.
          </li>
          <li>Click totals are approximate; edge cache hits are not counted the same way as direct hits.</li>
          <li>Raw click-event records are kept for 90 days in an encrypted archive, then deleted.</li>
          <li>A removed link is marked inactive immediately; the underlying record is deleted roughly 30 days later.</li>
        </ul>
        <h2>Network path</h2>
        <p>
          mol.la is served through Cloudflare, which sits in front of our infrastructure and can see connection metadata
          (address, timing) the way any reverse proxy in front of a website can. See{' '}
          <a href="https://www.cloudflare.com/privacypolicy/">Cloudflare's privacy policy</a> for how they handle that.
        </p>
        <h2>What we do not collect</h2>
        <p>This site has no account sign-up, no payment form, and no analytics or tracking scripts of our own.</p>
      </LegalDoc>
    </Chrome>
  )
}
