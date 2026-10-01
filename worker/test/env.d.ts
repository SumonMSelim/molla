// Secrets come from `wrangler secret put`, not wrangler.jsonc, so the
// generated Cloudflare.Env lacks them; TEST_MIGRATIONS is injected by
// vitest.config.ts.
declare namespace Cloudflare {
  interface Env {
    PERMUTATION_KEY: string
    CF_ZONE_ID?: string
    CF_PURGE_TOKEN?: string
    TEST_MIGRATIONS: D1Migration[]
  }
}
