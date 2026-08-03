<script lang="ts">
  import { mergeDimensionAndMeasureFilters } from "@rilldata/web-common/features/dashboards/filters/measure-filters/measure-filter-utils";
  import { getStateManagers } from "@rilldata/web-common/features/dashboards/state-managers/state-managers";
  import { sanitiseExpression } from "@rilldata/web-common/features/dashboards/stores/filter-utils";
  import { useTimeControlStore } from "@rilldata/web-common/features/dashboards/time-controls/time-control-store";
  import CopyIcon from "@rilldata/web-common/components/icons/CopyIcon.svelte";
  import { copyToClipboard } from "@rilldata/web-common/lib/actions/copy-to-clipboard";
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

  // Sorting: defaults to the time dimension, newest first. Clicking a column
  // header sorts by it; clicking again flips the direction.
  let sortCol: string | undefined = undefined;
  let sortDesc = true;
  $: effectiveSortCol = sortCol ?? timeDimension;

  function toggleSort(col: string) {
    if (effectiveSortCol === col) {
      sortDesc = !sortDesc;
    } else {
      sortCol = col;
      sortDesc = true;
    }
  }

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
      sort: effectiveSortCol
        ? [{ name: effectiveSortCol, ascending: !sortDesc }]
        : [],
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

  // Detects a JSON payload (object/array value, or a string containing JSON)
  // and pretty-prints it; returns null when the value is not JSON.
  function prettyJson(value: unknown): string | null {
    if (value !== null && typeof value === "object") {
      return JSON.stringify(value, null, 2);
    }
    if (typeof value !== "string") return null;
    const t = value.trim();
    if (!t.startsWith("{") && !t.startsWith("[")) return null;
    try {
      return JSON.stringify(JSON.parse(t), null, 2);
    } catch {
      return null;
    }
  }

  // Expanded detail: JSON payloads pretty-printed, everything else verbatim
  // (newlines preserved via CSS).
  function formatDetail(value: unknown): string {
    if (value === null || value === undefined) return "";
    return prettyJson(value) ?? String(value);
  }

  function copyRow(row: Record<string, unknown>) {
    copyToClipboard(JSON.stringify(row, null, 2));
  }

  function copyField(value: unknown) {
    copyToClipboard(formatDetail(value));
  }

  // Row cells show only the first line of multiline values (e.g. stack traces),
  // with a line-count hint; the expanded detail shows the full text.
  function formatCell(value: unknown): string {
    const s = format(value);
    const nl = s.indexOf("\n");
    if (nl === -1) return s;
    const lines = s.split("\n").length;
    return `${s.slice(0, nl)}  (+${lines - 1} lines)`;
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
              <th
                class:time-col={col === timeDimension}
                aria-sort={effectiveSortCol === col
                  ? sortDesc
                    ? "descending"
                    : "ascending"
                  : undefined}
              >
                <button type="button" onclick={() => toggleSort(col)}>
                  {col}
                  {#if effectiveSortCol === col}
                    <span class="sort-arrow">{sortDesc ? "\u2193" : "\u2191"}</span>
                  {/if}
                </button>
              </th>
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
                  {formatCell(row[col])}
                </td>
              {/each}
            </tr>
            {#if expanded === i}
              <tr class="detail-row">
                <td colspan={columns.length}>
                  <div class="detail-toolbar">
                    <button
                      type="button"
                      class="copy-btn"
                      title={m.dashboard_logs_copy_row()}
                      aria-label={m.dashboard_logs_copy_row()}
                      onclick={(e) => {
                        e.stopPropagation();
                        copyRow(row);
                      }}
                    >
                      <CopyIcon size="12px" />
                      {m.dashboard_logs_copy_row()}
                    </button>
                  </div>
                  <dl>
                    {#each allColumns as col (col)}
                      <div>
                        <dt>{col}</dt>
                        <dd>
                          {formatDetail(row[col])}
                          <button
                            type="button"
                            class="copy-btn field-copy"
                            title={m.dashboard_logs_copy_field({ field: col })}
                            aria-label={m.dashboard_logs_copy_field({
                              field: col,
                            })}
                            onclick={(e) => {
                              e.stopPropagation();
                              copyField(row[col]);
                            }}
                          >
                            <CopyIcon size="11px" />
                          </button>
                        </dd>
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
    @apply sticky top-0 z-10 bg-surface-subtle text-left p-0;
    @apply font-semibold text-fg-secondary border-b whitespace-nowrap;
  }

  thead th button {
    @apply w-full text-left px-3 py-1.5 font-semibold cursor-pointer;
  }

  thead th button:hover {
    @apply text-fg-primary;
  }

  .sort-arrow {
    @apply text-theme-600;
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
  .detail-toolbar {
    @apply flex justify-end px-2 pt-2;
  }

  .copy-btn {
    @apply inline-flex items-center gap-x-1 text-fg-muted;
    @apply hover:text-fg-primary cursor-pointer;
  }

  .field-copy {
    @apply ml-1.5 align-middle opacity-0;
  }

  .detail-row dd:hover .field-copy {
    opacity: 0.7;
  }
  .field-copy:hover {
    opacity: 1 !important;
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
    @apply break-words whitespace-pre-wrap;
  }
</style>
