// Machine-readable discovery documents under /.well-known/. Generated from
// PUBLIC_BASE so every environment advertises its own origin.

const SECURITY_EXPIRES = '2027-10-01T00:00:00Z'
const CACHE = 'public, max-age=3600'

export function wellKnown(request: Request, publicBase: string): Response | null {
  if (request.method !== 'GET' && request.method !== 'HEAD') {
    return null
  }
  switch (new URL(request.url).pathname) {
    case '/.well-known/security.txt':
      return new Response(securityTxt(publicBase), {
        headers: { 'Content-Type': 'text/plain; charset=utf-8', 'Cache-Control': CACHE },
      })
    case '/.well-known/api-catalog':
      return new Response(JSON.stringify(apiCatalog(publicBase)), {
        headers: {
          'Content-Type': 'application/linkset+json',
          'Cache-Control': CACHE,
          Link: '<https://www.rfc-editor.org/info/rfc9727>; rel="profile"',
        },
      })
    default:
      return null
  }
}

// RFC 9116. Renew Expires before it lapses (docs/RUNBOOK.md).
function securityTxt(base: string): string {
  return [
    'Contact: https://github.com/SumonMSelim/molla/security/advisories/new',
    'Contact: mailto:sumonmselim@gmail.com',
    `Expires: ${SECURITY_EXPIRES}`,
    'Preferred-Languages: en',
    `Canonical: ${base}/.well-known/security.txt`,
    'Policy: https://github.com/SumonMSelim/molla/blob/main/SECURITY.md',
    '',
  ].join('\n')
}

// RFC 9727 API catalog (a linkset, RFC 9264).
function apiCatalog(base: string) {
  return {
    linkset: [
      {
        anchor: `${base}/api/v1/links`,
        'service-desc': [{ href: `${base}/openapi.json`, type: 'application/vnd.oai.openapi+json' }],
        'service-doc': [{ href: `${base}/app/developers/`, type: 'text/html' }],
      },
    ],
  }
}
