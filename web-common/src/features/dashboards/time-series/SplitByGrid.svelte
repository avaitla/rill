<script lang="ts">
  import {
    createAndExpression,
    createInExpression,
  } from "@rilldata/web-common/features/dashboards/stores/filter-utils";
  import Spinner from "@rilldata/web-common/features/entity-management/Spinner.svelte";
  import { EntityStatus } from "@rilldata/web-common/features/entity-management/types";
  import {
    createQueryServiceMetricsViewAggregation,
    type MetricsViewSpecDimension,
    type MetricsViewSpecMeasure,
    type V1Expression,
    type V1TimeGrain,
  } from "@rilldata/web-common/runtime-client";
  import { useRuntimeClient } from "@rilldata/web-common/runtime-client/v2";
  import { keepPreviousData } from "@tanstack/svelte-query";
  import type { Interval } from "luxon";
  import MeasureBigNumber from "../big-number/MeasureBigNumber.svelte";
  import MeasureChart from "./measure-chart/MeasureChart.svelte";

  export let metricsViewName: string;
  export let dimension: MetricsViewSpecDimension;
  export let measures: MetricsViewSpecMeasure[];
  // Sanitised where clause of the whole dashboard; each facet ANDs its own dimension value on top.
  export let where: V1Expression | undefined = undefined;
  export let timeDimension: string | undefined = undefined;
  export let timeStart: string | undefined = undefined;
  export let timeEnd: string | undefined = undefined;
  export let interval: Interval<true> | undefined = undefined;
  export let timeGranularity: V1TimeGrain | undefined = undefined;
  export let timeZone: string = "UTC";
  export let ready: boolean = true;
  export let connectNulls: boolean = true;
  export let limit = 12;

  const client = useRuntimeClient();

  $: dimensionName = dimension.name ?? "";
  $: sortMeasureName = measures[0]?.name ?? "";

  // Top dimension values under the current filters, ranked by the first visible measure.
  $: topValuesQuery = createQueryServiceMetricsViewAggregation(
    client,
    {
      metricsView: metricsViewName,
      dimensions: [{ name: dimensionName }],
      measures: sortMeasureName ? [{ name: sortMeasureName }] : [],
      sort: sortMeasureName ? [{ name: sortMeasureName, desc: true }] : [],
      where,
      timeRange: {
        start: timeStart as any,
        end: timeEnd as any,
        timeDimension,
      },
      limit: limit.toString(),
      offset: "0",
    },
    {
      query: {
        enabled: ready && !!dimensionName,
        placeholderData: keepPreviousData,
      },
    },
  );

  $: facetValues = ($topValuesQuery.data?.data ?? []).map(
    (row) => row[dimensionName] as string | null,
  );

  function facetWhere(value: string | null): V1Expression {
    const inExpr = createInExpression(dimensionName, [value]);
    if (!where) return inExpr;
    return createAndExpression([where, inExpr]);
  }
</script>

{#if $topValuesQuery.isLoading}
  <div class="flex items-center justify-center h-40">
    <Spinner status={EntityStatus.Running} />
  </div>
{:else if facetValues.length === 0}
  <div class="text-fg-muted text-sm p-4">
    No values found for {dimension.displayName || dimensionName} under the current
    filters.
  </div>
{:else}
  <div
    class="grid gap-x-3 gap-y-3 px-2.5 pt-2"
    style:grid-template-columns="repeat(auto-fill, minmax(340px, 1fr))"
  >
    {#each facetValues as facetValue (facetValue)}
      {@const whereForFacet = facetWhere(facetValue)}
      <div
        class="border border-gray-200 rounded-sm p-2 flex flex-col gap-y-1 min-w-0"
      >
        <h3
          class="text-sm font-semibold text-fg-primary truncate"
          title={facetValue ?? "null"}
        >
          {facetValue ?? "null"}
        </h3>
        {#each measures as measure (measure.name)}
          <div
            class="grid items-center gap-x-2"
            style:grid-template-columns="110px minmax(0, 1fr)"
          >
            <MeasureBigNumber
              {measure}
              withTimeseries={false}
              skipLink
              {metricsViewName}
              where={whereForFacet}
              {timeDimension}
              {timeStart}
              {timeEnd}
              {ready}
            />
            {#if timeGranularity}
              <MeasureChart
                {measure}
                {connectNulls}
                {metricsViewName}
                where={whereForFacet}
                {timeDimension}
                {interval}
                {timeGranularity}
                {timeZone}
                {ready}
                chartHeight={56}
              />
            {/if}
          </div>
        {/each}
      </div>
    {/each}
  </div>
{/if}
