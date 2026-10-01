export type Env = {
  DB: D1Database
  ASSETS: Fetcher
  CREATE_LIMITER: RateLimit
  PUBLIC_BASE: string
  ACCESS_TEAM_DOMAIN: string
  ACCESS_AUD: string
  // Secrets (wrangler secret put). CF_ZONE_ID + CF_PURGE_TOKEN are optional;
  // without them a takedown relies on the 60s edge cache TTL instead of a
  // global purge.
  PERMUTATION_KEY: string
  CF_ZONE_ID?: string
  CF_PURGE_TOKEN?: string
}
