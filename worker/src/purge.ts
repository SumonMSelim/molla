// Global cache purge by URL. The Cache API only clears the current colo, so a
// takedown also asks the zone to drop the URL everywhere. Optional: without a
// token the 60s edge TTL bounds staleness instead.
export function zonePurger(zoneID: string | undefined, token: string | undefined, fetchImpl: typeof fetch = fetch) {
  return async (url: string): Promise<void> => {
    if (!zoneID || !token) {
      return
    }
    const response = await fetchImpl(`https://api.cloudflare.com/client/v4/zones/${zoneID}/purge_cache`, {
      method: 'POST',
      headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
      body: JSON.stringify({ files: [url] }),
    })
    if (!response.ok) {
      throw new Error(`purge_cache: ${response.status}`)
    }
  }
}
