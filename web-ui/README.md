# manigot web UI

The control-plane client for `mg serve` — a Svelte 5 single-page app that
renders the daemon's read-only API today and the mutating surface as it lands.
It is a port of the TUI's information design: same data, same job pipeline,
same attention ordering — built for a browser, so the "review from a phone,
get a ping when a job needs you, act on it" story has a surface.

## Running

```sh
npm install
npm run dev        # vite dev server on :5173
npm run dev:mock   # same, with the in-browser mock daemon (no mg serve needed)
npm run build      # production build to dist/
npm run test       # vitest suite
```

The app talks to the daemon through the connection setting (Settings modal /
`?api=` URL param). The daemon is a same-origin JSON API — it sends no CORS
headers, so the browser blocks any direct cross-origin URL (the error will
say "cross-origin URL blocked"). In dev, use the vite proxy path instead:

```sh
npm run dev
# open http://localhost:5173/?api=/api
```

`/api` is same-origin (vite proxies it to `127.0.0.1:8080`, forwarding the
bearer token). Served by the daemon itself (production, same origin) there
is nothing to configure; against a remote daemon behind a reverse proxy the
proxy serves both the UI and `/api`. For a non-loopback daemon bind, set the
bearer token in the settings modal (`mg serve-token` writes one to the
daemon's `.env` — tokens never ride in URLs).

Development conveniences:

- `?api=<url>` — point the session at a daemon without touching settings
  (deep-link/testing only; tokens stay in the settings modal).
- `?mock=1` (or `--mode mock`) — swap the API layer for the in-browser
  fixture backend (`src/lib/api/mock.ts`), so the UI is fully explorable with
  no daemon running. The mock mirrors the real API shapes exactly — its
  routes are pinned by `mock.test.ts` so the dev environment cannot silently
  drift from the shipped daemon.

## Layout

```
src/
  App.svelte            shell: rail, router, connection bar, palette
  main.ts               bootstrap
  lib/
    api/                client, endpoint paths, types, mock backend, SSE stream
    components/         Pipeline, MarkdownView, DiffView, RunConsole, modals, …
    state/              connection, data (polling), toasts stores
    views/              JobsView, JobDetailView, AgentsView, HealthView
    diff.ts             parse git log/stat/patch into renderable structures
    runlog.ts           parse run.log event timeline (incl. NEEDS-HUMAN-INPUT)
    markdown.ts         sanitized markdown render + verdict/status extraction
    stage.ts            pipeline stage ordering + attention sorting
    router.ts           hash router (#/p/<project>[/j/<job>[/<tab>]])
    time.ts             relative time formatting
```

## Design notes

- **Part 1 / Part 2 capability probing** — the client treats a 404/405 from a
  mutating endpoint as a *capability miss* (`ApiError.capabilityMiss`), so
  the same frontend runs against a read-only daemon (actions greyed out with
  an explanation) and against the full control plane once job two's endpoints
  land.
- **Attention-first job list** — `stopped:needs-human` outranks `running`
  outranks quiet; the segmented filter and the sort share one `stage.ts`.
- **Live run console** — the Run tab polls `/jdi` for the event timeline and
  attaches the `session.log` stream over SSE when the daemon offers it,
  degrading to polling otherwise; the mode is shown, never hidden.
- **Job files** — the daemon serves job files by their on-disk name
  (`brief.md`, not `brief`); the client maps tab ids to filenames.
- **Accessibility** — WCAG AA contrast on all informative text, keyboard
  navigation throughout, ARIA roles on interactive affordances, verified per
  render with the `shot` tool at 375/768/1280.