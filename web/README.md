# Web UI

Static SPA for the mol.la JSON API. React, TypeScript, Vite, Tailwind v4. Same visual language as [sumonselim.com](https://sumonselim.com): Inter, Fira Code, square corners, brand green `#00e654`, glow on primary hover/focus. No cloud SDK. API calls use relative `/api` URLs.

## Two-terminal loop

Terminal 1, API with in-memory adapters (Docker):

```sh
make dev-api
```

Listens on `http://127.0.0.1:8080`. Create and stats are unauthenticated. Create returns a `short_url` on the same origin; `GET /{code}` is a 302.

Terminal 2, Vite (proxies `/api` to the Go process):

```sh
cd web
npm ci
npm run dev
```

Open `http://127.0.0.1:5173/app/`. Links created in this browser are listed from `localStorage` so this browser can find them again; that list is local only and isn't fetched from the API.

Toolchain in Docker instead of host Node:

```sh
make web-lint
make web-test
make web-build
```

CI and native Node use `NPM=npm`.

Build output is `web/dist/` (gitignored). Deploying that tree to the Slice 7 UI bucket is an operator step, not part of this slice.
