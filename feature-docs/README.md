# Rill Feature Branches — Demo Documentation

Nine feature branches on `github.com:avaitla/rill.git`, each with one logical feature.
Every doc in this directory describes the feature, its YAML surface, and a self-contained
demo runbook that can be executed from scratch in a fresh session.

| # | Branch | Feature | Base |
|---|--------|---------|------|
| 1 | [`avaitla/postgres-olap-connector`](01-postgres-olap-connector.md) | Postgres as a full OLAP engine | `main` |
| 2 | [`avaitla/dashboard-refresh-intervals`](02-dashboard-refresh-intervals.md) | Grafana-style refresh intervals (explicit timers) | `main` |
| 3 | [`avaitla/skip-invalid-dimensions`](03-skip-invalid-dimensions.md) | Tolerate missing dimension columns | `main` |
| 4 | [`avaitla/dimension-drill-through`](04-dimension-drill-through.md) | Dimension value links: external URLs + explore drill-through | `main` |
| 5 | [`avaitla/dynamic-map-dimensions`](05-dynamic-map-dimensions.md) | Schemaless dimensions: map keys, column wildcards, empty-hiding | `avaitla/skip-invalid-dimensions` |
| 6 | [`avaitla/metrics-view-table-options`](06-metrics-view-table-options.md) | Multiple selectable tables behind one metrics view | `avaitla/skip-invalid-dimensions` |
| 7 | [`avaitla/drill-through-context`](07-drill-through-context.md) | Drill-through carries filters, time range and grain | `avaitla/dimension-drill-through` |
| 8 | [`avaitla/logs-view`](08-logs-view.md) | Logs tab: live raw-events tail per filters/time range | `main` |
| 9 | [`avaitla/row-links`](09-row-links.md) | Row links (`{{ column }}` templates) + "View rows" | `avaitla/logs-view` |

Each doc includes a screenshot of the feature captured from the live all-features demo
(`feature-docs/screenshots/`).

**Quick start:** the `avaitla/all-features` branch merges all six and ships a one-command
demo — `make demo` builds the CLI, provisions seeded ClickHouse + Postgres via docker
compose, and serves a project exercising every feature at http://localhost:9009
(`make demo-down` tears down the databases). See `demo/README.md` on that branch.
The per-branch runbooks below remain the way to demo a feature in isolation.

Branches 5 and 6 are stacked on branch 3 (they use its pruning machinery); merge 3 first.
Two proto field numbers are deliberately reserved to avoid cross-branch collisions:
`MetricsViewSpec.Dimension` field 17 (held for `drill_through`) and `ExploreSpec` field 22
(held for `refresh_intervals`); `ExploreSpec` field 39 is held for `skip_empty_dimensions`
and `ExplorePreset` field 40 for `refresh_interval`.

## Building any branch

```bash
cd rill && git checkout <branch>
# Node 20+ is required for the frontend build (paraglide i18n needs the global crypto API;
# Node 18 fails with "crypto is not defined")
export PATH="$HOME/.nvm/versions/node/v20.20.1/bin:$PATH"
make cli          # full build: npm install + web build + go build -> ./rill (~2-4 min)
go build -o rill ./cli   # backend-only rebuild; reuses the last embedded frontend
```

Run a demo project with:

```bash
cd <project-dir> && /path/to/rill/rill start --no-open --port <port> --port-grpc <port+40000>
```

Each doc assigns a stable port so demos can run side by side.

## Regenerating protos (only needed when editing `.proto` files)

`make proto.generate`'s remote `buf.build/connectrpc/es:v1.4.0` plugin silently returns an
empty response (the debug log shows a 6-byte reply). Work around it with local plugins:

```bash
go install github.com/bufbuild/buf/cmd/buf@latest
npm install --no-save @bufbuild/protoc-gen-es@1.10.0 @connectrpc/protoc-gen-connect-es@1.4.0
export PATH="$(go env GOPATH)/bin:$PWD/node_modules/.bin:$PATH"
cd proto
buf generate --exclude-path rill/ui                                      # Go bindings
buf generate --template buf.gen.openapi-runtime.yaml --path rill/runtime # swagger
cat > buf.gen.runtime.local.yaml <<'EOF'
version: v2
plugins:
  - local: protoc-gen-es
    out: ../web-common/src/proto/gen
    opt: [target=ts]
  - local: protoc-gen-connect-es
    out: ../web-common/src/proto/gen
    opt: [target=ts]
EOF
buf generate --template buf.gen.runtime.local.yaml --path rill/runtime   # TS bindings
rm buf.gen.runtime.local.yaml
```

Note: `make cli` runs `npm install`, which removes the `--no-save` plugins — reinstall
before regenerating. Also, `web-common/src/runtime-client/gen/index.schemas.ts` is
hand-maintained (no longer Orval-generated): add new proto fields to it manually.

## Verifying demos headlessly

Playwright is available through the repo's node_modules (run probe scripts from the repo
root so module resolution works; `npx playwright install chromium-headless-shell` on first
use). Standard probe skeleton:

```js
import { chromium } from "playwright";
const browser = await chromium.launch();
const page = await browser.newPage({ viewport: { width: 1500, height: 900 } });
await page.goto("http://localhost:<port>/explore/<name>", { waitUntil: "domcontentloaded" });
await page.waitForTimeout(9000); // never networkidle: the app holds an SSE connection
await page.screenshot({ path: "/tmp/shot.png" });
await browser.close();
```

Useful assertions: `page.getByRole("table")` elements carry per-dimension leaderboard
aria-labels; query traffic can be counted by parsing `QueryService/*` request bodies
(`body.metricsView` / `body.metricsViewName`).

## Shared external dependencies

- **Postgres 16** on `localhost:5432` (Homebrew: `postgresql@16`), used by doc 1
- **Docker** for the ClickHouse container, used by doc 5
- **DuckDB CLI** (`duckdb`, v1.5.x) for creating demo database files, docs 5 and 6
- **RawDuck extension** (github.com/quackscience/rawduck) release binaries, doc 5
