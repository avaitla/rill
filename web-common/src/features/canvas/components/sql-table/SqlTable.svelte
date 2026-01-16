<script lang="ts">
  import { createQuery } from "@tanstack/svelte-query";
  import { queryClient } from "@rilldata/web-common/lib/svelte-query/globalQueryClient";
  import { runtime } from "@rilldata/web-common/runtime-client/runtime-store";
  import { httpClient } from "@rilldata/web-common/runtime-client/http-client";
  import type { SqlTableComponent } from "./";
  import type { V1QueryResolverResponse } from "@rilldata/web-common/runtime-client";
  import VegaLiteRenderer from "@rilldata/web-common/components/vega/VegaLiteRenderer.svelte";
  import type { View } from "svelte-vega";

  export let component: SqlTableComponent;

  $: ({ instanceId } = $runtime);
  $: specStore = component?.specStore;
  $: spec = specStore ? $specStore : undefined;

  $: connector = spec?.connector ?? "";
  $: sql = spec?.sql ?? "";
  $: limit = spec?.limit ?? 1000;
  $: title = spec?.title ?? "SQL Query Results";
  $: displayType = spec?.display_type ?? "table";
  $: vegaPreset = spec?.vega_preset ?? "";
  $: vegaLiteSpec = spec?.vega_lite_spec ?? "";

  // Query hints for each preset (spec population is handled in SqlTableComponent.updateProperty)
  const PRESET_HINTS: Record<string, string> = {
    line: "Query should return: date (datetime), value (number)",
    stacked_line: "Query should return: date (datetime), value (number), category (string)",
    bar: "Query should return: category (string), value (number)",
    pie: "Query should return: category (string), value (number)",
    single_number: "Query should return single row with: value (number). Use LIMIT 1.",
  };

  // Get the effective Vega-Lite spec (from the vega_lite_spec field)
  $: effectiveVegaSpec = (() => {
    if (displayType !== "vega_lite") return null;

    // Parse the vega_lite_spec field
    if (vegaLiteSpec && vegaLiteSpec.trim()) {
      try {
        return JSON.parse(vegaLiteSpec);
      } catch {
        return null;
      }
    }

    return null;
  })();

  // Get hint for current preset
  $: presetHint = vegaPreset && PRESET_HINTS[vegaPreset]
    ? PRESET_HINTS[vegaPreset]
    : "";

  // Build query options
  $: queryEnabled = connector.trim().length > 0 && sql.trim().length > 0;

  // Create the query
  $: dataQuery = createQuery(
    {
      queryKey: ["sql-table", instanceId, connector, sql, limit],
      queryFn: async () => {
        const response = await httpClient<V1QueryResolverResponse>({
          url: `/v1/instances/${instanceId}/query/resolver`,
          method: "POST",
          headers: { "Content-Type": "application/json" },
          data: {
            resolver: "sql",
            resolverProperties: {
              connector,
              sql,
            },
            limit,
          },
        });
        return response;
      },
      enabled: queryEnabled,
      staleTime: 30000, // Cache for 30 seconds
    },
    queryClient,
  );

  // Extract schema and data
  $: schema = $dataQuery.data?.schema?.fields ?? [];
  $: data = $dataQuery.data?.data ?? [];
  $: isLoading = $dataQuery.isLoading;
  $: isError = $dataQuery.isError;
  $: errorMessage = (() => {
    const error = $dataQuery.error as any;
    return (
      error?.response?.data?.message ?? error?.message ?? "Failed to execute query"
    );
  })();

  // Prepare data for Vega-Lite
  $: vegaData = { table: data };

  let viewVL: View;
</script>

<div class="size-full flex flex-col bg-surface overflow-hidden" style:max-height="inherit">
  {#if title}
    <div class="px-3 py-2 border-b border-gray-200 shrink-0">
      <h3 class="text-sm font-medium text-gray-700">{title}</h3>
    </div>
  {/if}

  {#if presetHint && displayType === "vega_lite"}
    <div class="px-3 py-1.5 bg-blue-50 border-b border-blue-100 shrink-0">
      <p class="text-xs text-blue-700">
        <strong>Expected query format:</strong> {presetHint}
      </p>
    </div>
  {/if}

  <div class="flex-1 min-h-0 overflow-auto">
    {#if !queryEnabled}
      <div class="p-4 text-gray-500 text-sm">
        Configure a connector and SQL query to see results.
      </div>
    {:else if isLoading}
      <div class="p-4 text-gray-500 text-sm flex items-center gap-2">
        <svg class="animate-spin h-4 w-4" viewBox="0 0 24 24">
          <circle
            class="opacity-25"
            cx="12"
            cy="12"
            r="10"
            stroke="currentColor"
            stroke-width="4"
            fill="none"
          />
          <path
            class="opacity-75"
            fill="currentColor"
            d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
          />
        </svg>
        Loading...
      </div>
    {:else if isError}
      <div class="p-4 bg-red-50 text-red-700 text-sm">
        <strong>Error:</strong>
        {errorMessage}
      </div>
    {:else if data.length === 0}
      <div class="p-4 text-gray-500 text-sm">No results returned.</div>
    {:else if displayType === "vega_lite" && effectiveVegaSpec}
      <VegaLiteRenderer
        data={vegaData}
        spec={effectiveVegaSpec}
        bind:viewVL
      />
    {:else}
      <!-- Table display -->
      <table class="w-full text-sm">
        <thead class="bg-gray-50 sticky top-0">
          <tr>
            {#each schema as field}
              <th
                class="px-3 py-2 text-left font-medium text-gray-600 border-b border-gray-200"
              >
                {field.name}
              </th>
            {/each}
          </tr>
        </thead>
        <tbody>
          {#each data as row, i}
            <tr class:bg-gray-50={i % 2 === 1} class="hover:bg-blue-50">
              {#each schema as field}
                <td class="px-3 py-2 border-b border-gray-100 text-gray-700">
                  {row[field.name ?? ""] ?? ""}
                </td>
              {/each}
            </tr>
          {/each}
        </tbody>
      </table>
      {#if data.length >= limit}
        <div class="p-2 text-xs text-gray-400 text-center border-t">
          Results limited to {limit} rows
        </div>
      {/if}
    {/if}
  </div>
</div>
