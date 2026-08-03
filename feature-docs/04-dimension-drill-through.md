# 4. Dimension Value Links (incl. Drill-Through)

**Branch:** `avaitla/dimension-drill-through` (off `main`)

## What it does

A per-dimension `links` list attaches actions to **dimension values** in the leaderboard
and the expanded dimension table. Each link is one of:

- **`url`** — an external link template (Datadog service page, GitHub search, Grafana,
  any http(s) URL). `{{ value }}` is replaced with the URL-encoded clicked value; opens
  in a new tab.
- **`explore`** — the name of another explore dashboard; clicking navigates there with
  the clicked value applied as a filter (drill-through), e.g.
  `/explore/red_detail_explore?f=service+IN+('checkout')`.

One link renders as a direct hover icon (external-link icon for `url`, explore icon for
`explore`); multiple render as a menu of labeled entries. Icons compose with the
existing `uri` affordance, hover state is held while the menu is open, and **dimension
headers show a small link indicator** (tooltip lists the link targets) when links are
configured.

There is intentionally **no separate `drill_through` property** — an earlier revision
had one, and it was unified into `links` (an `explore`-type link) by request.

```yaml
type: metrics_view
dimensions:
  - column: service
    links:
      - label: Service detail
        explore: red_detail_explore     # drill-through with value filtered
      - label: Datadog APM
        url: "https://app.datadoghq.com/apm/services/{{ value }}"
      - label: GitHub search
        url: "https://github.com/search?q={{ value }}&type=code"
```

`label` is optional: it defaults to the URL's hostname for `url` links and to the
explore name for `explore` links. Exactly one of `url`/`explore` must be set per link.

## How it works

- Proto: `MetricsViewSpec.Dimension.ValueLink {label = 1, url = 2, explore = 3}`,
  `repeated ValueLink links = 22` (field 17, formerly `drill_through`, is reserved)
- Parser (`runtime/parser/parse_metrics_view.go`): `validateDimensionLink` checks
  absolute http(s) URLs (with `{{ value }}` placeholders substituted before parsing);
  enforces url/explore mutual exclusivity; fills default labels
- `web-common/src/features/dashboards/drill-through.ts`:
  `resolveDimensionValueLink` / `openDimensionValueLink` for url links;
  `gotoDrillThroughExplore` builds explore-link navigation via the existing
  explore-mappers (`generateExploreLink` + `createInExpression`), so URLs resolve
  correctly in Rill Developer, Rill Cloud (org/project from `$page.params`), and embeds
- Leaderboard: `Leaderboard.svelte` (data + `onExploreLink` handler) →
  `LeaderboardRow.svelte` (hover icons/menu; menu-open state holds the row hover since
  the dropdown is portaled) and `LeaderboardHeader.svelte` (header indicator)
- Dimension table: `DimensionValueHeader.svelte` (spec lookup, handler, header
  indicator) → generic `virtualized-table/core/Cell.svelte` (icons/menu per cell)
- i18n: `dashboard_dimension_links` ("Links for {value}") and
  `dashboard_dimension_has_links` ("Values link to: {targets}") in en/es

## Tests

Parser: `TestMetricsViewDimensionValueLinks` in
`runtime/parser/parse_metrics_view_test.go` (url + explore parsing, default labels,
invalid: relative URL, both url+explore set).

## Demo runbook

The all-features demo (`make demo`, see `demo/README.md`) exercises this on the RED
dashboards: the `service` dimension has an explore link ("Service detail" →
`red_detail_explore`) plus Datadog/GitHub url links; the ClickHouse logs dashboard's
`ServiceName` has a single Grafana url link.

Standalone: any DuckDB project — declare two explores over one metrics view and give a
dimension a `links` list mixing an `explore` entry and a url template. **What to show:**

1. Hover a value row → link icon fades in at the right edge; the dimension header shows
   the small indicator (tooltip: "Values link to: …")
2. Click the icon → menu with the labeled entries; picking the explore entry lands on
   the target explore with `?f=<dim> IN ('<value>')`; url entries open a new tab with
   `{{ value }}` resolved
3. Playwright: `getByRole("button", { name: "Links for <value>", exact: true })` opens
   the menu; menu items via `getByRole("menuitem")`
