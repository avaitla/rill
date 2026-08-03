# All-Features Demo

Self-contained demo of every feature on the `avaitla/all-features` branch, running
against docker-provisioned databases with pre-seeded example data. From the repo root:

```bash
make demo        # builds the CLI if needed, starts the databases, serves http://localhost:9009
make demo-down   # stops and wipes the demo databases
```

Requirements: Docker, plus the usual build toolchain (Go 1.24+, Node 20+) for the
one-time `make cli`. If you already have a `./rill` binary, no build happens.

## What's inside

`docker-compose.yaml` provisions two seeded databases (first start seeds in ~15s):

| Service | Port | Credentials | Data |
|---|---|---|---|
| ClickHouse 24.8 | 18123 | rill / rillpass | `otel_logs`: 20k ClickStack/OTel-style logs with `Map` attribute columns |
| Postgres 16 | 15432 | rill / rillpass, db `rill_demo` | `orders`: 50k e-commerce orders over ~6 months |

`project/` is a Rill project using three OLAP engines at once (per-metrics-view
`connector:`); the DuckDB datasets are built in-project as models (`events_v1`,
`events_v2`, and the wide union-by-name `red_requests` table — a stand-in for a
RawDuck-style schemaless store).

## Dashboards and the features they demonstrate

- **Orders (Postgres)** — `/explore/orders_explore`
  - *Postgres OLAP connector*: every query runs live against Postgres, no ingestion.
  - *Refresh intervals*: header dropdown (defaults to `30s`), `refresh` URL param,
    "Last refreshed" caption. Try `?refresh=5m`.
- **Logs (ClickHouse)** — `/explore/logs_explore`
  - *Dynamic map dimensions*: leaderboards for `http.method`, `http.status_code`,
    `user.id`, `k8s.namespace.name` are discovered from the `Map` columns at reconcile
    time — none are declared in the YAML. Insert a log with a new attribute key and
    touch `metrics/logs.yaml` to see a new leaderboard appear.
  - *skip_empty / hide_empty dimensions* on a real log schema.
- **RED Overview → RED Service Detail** — `/explore/red_overview_explore`
  - *Dimension drill-through*: hover a service in the leaderboard and click the drill
    icon → lands on the detail explore with that service filtered.
  - *Dimension value links*: the service dimension also carries external links (Datadog
    APM, GitHub search) — the hover link icon opens a menu; `{{ value }}` in each URL
    template resolves to the clicked service. The logs dashboard's ServiceName has a
    single Grafana link (direct icon, no menu).
  - *`columns: '*'` wildcard dimensions*: the detail explore's ~10 attribute
    leaderboards come from one wildcard dimension over the wide table.
  - *hide_empty_dimensions*: on the detail explore, filtering to `worker` hides the
    checkout/search attribute leaderboards (they're all-NULL for worker rows).
- **Events (Versioned Tables)** — `/explore/events_explore`
  - *table_options*: "Table events_v2" dropdown in the header, `table` URL param.
    Switching to `events_v1` (which lacks `http_method`/`region`) shows
    *skip_invalid_dimensions* pruning those leaderboards instead of erroring.
    Try `?table=events_v1`.

Full per-feature docs (design, protos, gotchas, standalone runbooks) live in the
`feature-docs/` directory committed at each feature branch's root.

## Notes

- **If the demo server is restarted, hard-refresh any open browser tabs.** A tab that
  outlives the server renders blank panes on in-app navigation (file editors, dashboards)
  until reloaded. This is stock Rill dev-server behavior (reproduced on upstream `main`),
  not specific to this branch.

- Host ports 15432/18123 avoid clashing with local Postgres/ClickHouse installs; if
  they're taken, edit `docker-compose.yaml` and the DSNs in `project/connectors/`.
- Data volumes are ephemeral (`down -v` wipes them); every fresh `up` re-seeds.
- The DuckDB models regenerate random data on every reconcile, so numbers shift
  between runs by design.
