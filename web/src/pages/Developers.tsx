import { Chrome, LegalDoc } from './Chrome.tsx'

export function DevelopersPage() {
  return (
    <Chrome width="prose">
      <LegalDoc title="Developers" updated="20 September 2026">
        <p>
          mol.la exposes a small public HTTP API for link creation and stats. Requests and responses are JSON. Base URL in
          production is <code>https://mol.la</code>.
        </p>

        <h2>Create short link</h2>
        <p>
          <code>POST /api/v1/links</code>
        </p>
        <p>Headers:</p>
        <ul>
          <li>
            <code>Content-Type: application/json</code>
          </li>
          <li>
            <code>Idempotency-Key: &lt;uuid&gt;</code> (recommended)
          </li>
        </ul>
        <p>Body:</p>
        <pre className="overflow-x-auto border border-border bg-card p-3 text-xs text-foreground">
{`{
  "long_url": "https://example.com/article",
  "alias": "my-link",
  "expires_in": 86400
}`}
        </pre>
        <p>
          <code>alias</code> and <code>expires_in</code> are optional. <code>expires_in</code> range is 60 to 157680000
          seconds.
        </p>
        <p>Success response (<code>201</code>):</p>
        <pre className="overflow-x-auto border border-border bg-card p-3 text-xs text-foreground">
{`{
  "short_code": "abc1234",
  "short_url": "https://mol.la/abc1234",
  "long_url": "https://example.com/article",
  "created_at": "2026-09-20T00:00:00Z",
  "expires_at": "2026-09-21T00:00:00Z"
}`}
        </pre>

        <h2>Read stats</h2>
        <p>
          <code>GET /api/v1/links/{'{short_code}'}/stats</code>
        </p>
        <p>Success response (<code>200</code>):</p>
        <pre className="overflow-x-auto border border-border bg-card p-3 text-xs text-foreground">
{`{
  "short_code": "abc1234",
  "clicks": 42,
  "created_at": "2026-09-20T00:00:00Z",
  "last_click_at": "2026-09-20T12:30:00Z"
}`}
        </pre>

        <h2>Redirect behavior</h2>
        <p>
          Visiting <code>GET /{'{short_code}'}</code> returns HTTP <code>302</code> to the destination URL when active.
        </p>

        <h2>Error model</h2>
        <p>Errors return JSON:</p>
        <pre className="overflow-x-auto border border-border bg-card p-3 text-xs text-foreground">
{`{ "error": "INVALID_URL" }`}
        </pre>
        <p>Common error codes:</p>
        <ul>
          <li>
            <code>INVALID_URL</code>
          </li>
          <li>
            <code>INVALID_ALIAS</code>
          </li>
          <li>
            <code>INVALID_EXPIRY</code>
          </li>
          <li>
            <code>ALIAS_TAKEN</code>
          </li>
          <li>
            <code>IDEMPOTENCY_CONFLICT</code>
          </li>
          <li>
            <code>RATE_LIMITED</code>
          </li>
          <li>
            <code>TEMPORARILY_UNAVAILABLE</code>
          </li>
          <li>
            <code>NOT_FOUND</code>
          </li>
        </ul>

        <h2>cURL examples</h2>
        <pre className="overflow-x-auto border border-border bg-card p-3 text-xs text-foreground">
{`curl -X POST "https://mol.la/api/v1/links" \\
  -H "Content-Type: application/json" \\
  -H "Idempotency-Key: 7f1df4f4-9989-4e7a-8fd0-e2f012345678" \\
  -d '{"long_url":"https://example.com/article","alias":"my-link"}'`}
        </pre>
        <pre className="overflow-x-auto border border-border bg-card p-3 text-xs text-foreground">
{`curl "https://mol.la/api/v1/links/abc1234/stats"`}
        </pre>
      </LegalDoc>
    </Chrome>
  )
}
