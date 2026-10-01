#!/usr/bin/env node
// Converts `aws dynamodb scan --output json` dumps of the links, stats, and
// counters tables into D1 INSERT statements. Usage:
//   node dynamodb-to-d1.mjs links.json stats.json counters.json > import.sql
import { readFileSync } from 'node:fs'

const [linksFile, statsFile, countersFile] = process.argv.slice(2)
if (!linksFile || !statsFile || !countersFile) {
  console.error('usage: dynamodb-to-d1.mjs links.json stats.json counters.json')
  process.exit(2)
}

const items = (file) => JSON.parse(readFileSync(file, 'utf8')).Items ?? []
const s = (item, key) => item[key]?.S ?? ''
const n = (item, key) => (item[key]?.N === undefined ? null : Number(item[key].N))
const b = (item, key) => (item[key]?.BOOL ? 1 : 0)
const q = (value) => (value === null ? 'NULL' : typeof value === 'number' ? String(value) : `'${value.replace(/'/g, "''")}'`)

const out = []
for (const item of items(linksFile)) {
  const row = [
    s(item, 'short_code'),
    s(item, 'long_url'),
    s(item, 'owner_id'),
    b(item, 'is_custom'),
    b(item, 'is_active'),
    n(item, 'version') ?? 1,
    n(item, 'created_at') ?? 0,
    n(item, 'expires_at') ?? 0,
    n(item, 'deleted_at'),
    n(item, 'purge_at') ?? n(item, 'expires_at') ?? 0,
    s(item, 'deleted_by'),
    s(item, 'delete_role'),
    s(item, 'delete_reason'),
  ]
  out.push(
    `INSERT OR IGNORE INTO links (short_code, long_url, owner_id, is_custom, is_active, version, created_at, expires_at, deleted_at, purge_at, deleted_by, delete_role, delete_reason) VALUES (${row.map(q).join(', ')});`,
  )
}
for (const item of items(statsFile)) {
  out.push(
    `INSERT OR IGNORE INTO stats (short_code, clicks, last_click_at) VALUES (${[s(item, 'short_code'), n(item, 'clicks') ?? 0, n(item, 'last_click_at') ?? 0].map(q).join(', ')});`,
  )
}
let max = 0
for (const item of items(countersFile)) {
  max = Math.max(max, n(item, 'counter') ?? 0)
}
// Seed above every block ever leased on AWS so new IDs never overlap.
out.push(`UPDATE counters SET counter = max(counter, ${max + 1000}) WHERE region = 'global';`)
process.stdout.write(out.join('\n') + '\n')
