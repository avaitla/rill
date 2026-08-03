<script lang="ts">
  import { mergeDimensionAndMeasureFilters } from "@rilldata/web-common/features/dashboards/filters/measure-filters/measure-filter-utils";
  import { getStateManagers } from "@rilldata/web-common/features/dashboards/state-managers/state-managers";
  import { sanitiseExpression } from "@rilldata/web-common/features/dashboards/stores/filter-utils";
  import { useTimeControlStore } from "@rilldata/web-common/features/dashboards/time-controls/time-control-store";
  import { m } from "@rilldata/web-common/lib/i18n/gen/messages";
  import { createQueryServiceMetricsViewRows } from "@rilldata/web-common/runtime-client/v2/gen/query-service";

  const LOGS_ROW_LIMIT = 200;

  const stateManagers = getStateManagers();
  const { metricsViewName, validSpecStore, dashboardStore, runtimeClient } =
    stateManagers;
  const timeControlsStore = useTimeControlStore(stateManagers);

  $: metricsViewSpec = $validSpecStore.data?.metricsView ?? {};
  $: exploreSpec = $validSpecStore.data?.explore ?? {};
  $: timeDimension = metricsViewSpec.timeDimension;
  $: configuredColumns = exploreSpec.logsViewColumns ?? [];

  $: ({ timeStart, timeEnd, ready: timeControlsReady } = $timeControlsStore);

  $: where = sanitiseExpression(
    mergeDimensionAndMeasureFilters(
      $dashboardStore?.whereFilter,
      $dashboardStore?.dimensionThresholdFilters ?? [],
    ),
    undefined,
  );

  $: rowsQuery = createQueryServiceMetricsViewRows(
    runtimeClient,
    {
      metricsViewName: $metricsViewName,
      timeStart,
      timeEnd,
      where,
      sort: timeDimension ? [{ name: timeDimension, ascending: false }] : [],
      limit: LOGS_ROW_LIMIT,
    },
    {
      query: {
        enabled: !!$metricsViewName && (timeControlsReady ?? false),
      },
    },
  );

  $: rows = ($rowsQuery.data?.data ?? []) as Record<string, unknown>[];
  $: allColumns = ($rowsQuery.data?.meta ?? [])
    .map((c) => c.name ?? "")
    .filter(Boolean);
  $: columns = configuredColumns.length
    ? configuredColumns.filter((c) => allColumns.includes(c))
    : allColumns;

  let expanded: number | null = null;

  function format(value: unknown): string {
    if (value === null || value === undefined) return "";
    if (typeof value === "object") return JSON.stringify(value);
    return String(value);
  }
</script>

<div class="logs-view" aria-label="Logs view">
  <div class="header-row">
    <span class="text-xs text-fg-muted">
      {rows.length === LOGS_ROW_LIMIT
        ? m.dashboard_logs_showing_latest({ count: String(LOGS_ROW_LIMIT) })
        : m.dashboard_logs_showing({ count: String(rows.length) })}
    </span>
  </div>

  {#if $rowsQuery.error}
    <div class="p-4 text-sm text-destructive">
      {$rowsQuery.error?.message}
    </div>
  {:else}
    <div class="table-wrapper">
      <table>
        <thead>
          <tr>
            {#each columns as col (col)}
              <th class:time-col={col === timeDimension}>{col}</th>
            {/each}
          </tr>
        </thead>
        <tbody>
          {#each rows as row, i (i)}
            <tr
              class:expanded={expanded === i}
              onclick={() => (expanded = expanded === i ? null : i)}
            >
              {#each columns as col (col)}
                <td class:time-col={col === timeDimension}>
                  {format(row[col])}
                </td>
              {/each}
            </tr>
            {#if expanded === i}
              <tr class="detail-row">
                <td colspan={columns.length}>
                  <dl>
                    {#each allColumns as col (col)}
                      <div>
                        <dt>{col}</dt>
                        <dd>{format(row[col])}</dd>
                      </div>
                    {/each}
                  </dl>
                </td>
              </tr>
            {/if}
          {/each}
        </tbody>
      </table>
      {#if !rows.length && !$rowsQuery.isLoading}
        <div class="p-6 text-sm text-fg-muted text-center">
          {m.dashboard_logs_empty()}
        </div>
      {/if}
    </div>
  {/if}
</div>

<style lang="postcss">
  .logs-view {
    @apply flex flex-col size-full overflow-hidden bg-surface-background;
  }

  .header-row {
    @apply flex items-center justify-end px-4 py-1.5 border-b flex-none;
  }

  .table-wrapper {
    @apply overflow-auto size-full;
  }

  table {
    @apply w-full text-xs font-mono border-collapse;
  }

  thead th {
    @apply sticky top-0 z-10 bg-surface-subtle text-left px-3 py-1.5;
    @apply font-semibold text-fg-secondary border-b whitespace-nowrap;
  }

  tbody tr {
    @apply cursor-pointer border-b border-gray-100;
  }

  tbody tr:hover,
  tbody tr.expanded {
    @apply bg-popover-accent;
  }

  td {
    @apply px-3 py-1 align-top whitespace-nowrap max-w-[480px] truncate;
  }

  td.time-col,
  th.time-col {
    @apply text-fg-muted;
  }

  .detail-row {
    @apply cursor-default;
  }
  .detail-row td {
    @apply whitespace-normal bg-surface-subtle;
  }
  .detail-row dl {
    @apply grid gap-x-6 gap-y-1 p-2;
    grid-template-columns: max-content 1fr;
  }
  .detail-row dl div {
    @apply contents;
  }
  .detail-row dt {
    @apply text-fg-muted;
  }
  .detail-row dd {
    @apply break-all;
  }
</style>
