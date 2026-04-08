# Testing

## Unit tests (`ui/`)

**Stack:** Vitest + jsdom + React Testing Library — `npm run test`, `npm run test:watch`.

See the table in git history or `src/**/*.test.tsx` for coverage areas (API helper, pages, contexts, `utils/date`, etc.).

---

## E2E (`ui/e2e/`)

**Stack:** Playwright — `npm run test:e2e` (from `ui/`).

- **No Docker / no Go server:** `playwright.config.mjs` starts **`next dev`** on `127.0.0.1:3000`.
- **Mocked HTTP:** `e2e/mock-api.ts` uses `page.route('**/api/v1/**', …)` so browser `fetch` to `/api/v1/*` never hits a real backend. Extend the handler when adding flows.
- **Browsers:** Chromium only by default (fast CI). Run `npm run test:e2e:install` once locally to install browsers.

```bash
cd ui && npm run test:e2e
```

---

## Backend (Go)

`go test ./...` from the repo root.
