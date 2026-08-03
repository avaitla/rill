# 7. Drill-Through Context Carry

**Branch:** `avaitla/drill-through-context` (stacked on `avaitla/dimension-drill-through`)

## What it does

Explore-type dimension links (drill-through) now carry the **full dashboard context**
to the target explore instead of just the clicked value: the active filters (a
pre-existing filter on the clicked dimension is replaced by the clicked value), the
selected time range **including grain**, and the timezone. Drill paths continue from
where the user was — no more landing on "All time" with a single filter.

Example resulting URL: `/explore/red_detail_explore?tr=P7D&grain=day&f=service+IN+('checkout')`.

## How it works

`gotoDrillThroughExplore` (`web-common/src/features/dashboards/drill-through.ts`) takes
a `DrillThroughContext` (`whereFilter`, `dimensionsWithInlistFilter`,
`selectedTimeRange`, `selectedTimezone`), merges the clicked value via
`getFiltersForOtherDimensions` + `createInExpression`, and hands the partial
`ExploreState` to `generateExploreLink`. Callers (`Leaderboard.svelte`,
`DimensionValueHeader.svelte`) read the state at click time via `get(dashboardStore)` —
canvas-safe (no context → no carry, same as before).

## Demo

In the all-features demo: set a time range and filters on RED Overview, drill via a
service value's "Service detail" link → the detail explore opens with the same range,
grain and filters plus the service filter.
