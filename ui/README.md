# Alfred Web UI

Next.js 15 (App Router) + React 19 + TypeScript + Tailwind 4.

## Setup

```bash
npm install
```

## Run

```bash
npm run dev
```

Open [http://localhost:3000](http://localhost:3000). Set `NEXT_PUBLIC_API_BASE_URL` to the Go API (default `http://localhost:8080`) if the API runs elsewhere.

## Scripts

- `npm run dev` – development server
- `npm run build` – production build
- `npm run start` – run production build
- `npm run lint` – ESLint
- `npm run format` – Prettier (write)
- `npm run format:check` – Prettier (check)
- `npm run test` / `npm run test:watch` – Vitest (unit)
- `npm run test:e2e` – Playwright (Chromium; `playwright.config.mjs` starts `next dev` on `127.0.0.1:3000`)
- `npm run test:e2e:install` – install Playwright browsers (run once locally)

## Testing

**Unit:** Vitest + jsdom + Testing Library. Tests live next to source as `*.test.tsx` / `*.test.ts` (`src/**`).

**E2E:** Playwright under `e2e/`. `e2e/mock-api.ts` uses `page.route('**/api/v1/**', …)` so browser calls to `/api/v1/*` are mocked (fixtures in `e2e/fixtures.ts`). No backend required for `npm run test:e2e`.
