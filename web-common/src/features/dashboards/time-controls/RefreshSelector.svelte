<script lang="ts">
  import * as DropdownMenu from "@rilldata/web-common/components/dropdown-menu";
  import CaretDownIcon from "@rilldata/web-common/components/icons/CaretDownIcon.svelte";
  import RefreshIcon from "@rilldata/web-common/components/icons/RefreshIcon.svelte";
  import { m } from "@rilldata/web-common/lib/i18n/gen/messages";
  import { queryClient } from "@rilldata/web-common/lib/svelte-query/globalQueryClient";
  import { timeAgo } from "@rilldata/web-common/lib/time/relative-time";
  import { getQueryServiceMetricsViewTimeRangeQueryOptions } from "@rilldata/web-common/runtime-client";
  import { invalidateMetricsViewData } from "@rilldata/web-common/runtime-client/invalidation";
  import type { RuntimeClient } from "@rilldata/web-common/runtime-client/v2";
  import {
    AUTO_REFRESH_POLL_INTERVAL_MS,
    DEFAULT_REFRESH_INTERVALS,
    isValidRefreshInterval,
    REFRESH_INTERVAL_AUTO,
    REFRESH_INTERVAL_OFF,
    refreshIntervalToMs,
  } from "./refresh-intervals";

  interface Props {
    runtimeClient: RuntimeClient;
    metricsViewName: string;
    /** Selectable durations, from the explore YAML's `refresh_intervals`. Falls back to a default list. */
    refreshIntervals?: string[];
    selected: string;
    onSelect: (interval: string) => void;
  }

  let {
    runtimeClient,
    metricsViewName,
    refreshIntervals,
    selected,
    onSelect,
  }: Props = $props();

  let open = $state(false);
  let refreshing = $state(false);
  let lastRefreshedAt = $state(new Date());

  // Re-render the relative "last refreshed" label periodically
  let now = $state(Date.now());
  $effect(() => {
    const timer = setInterval(() => (now = Date.now()), 15_000);
    return () => clearInterval(timer);
  });
  const lastRefreshedLabel = $derived.by(() => {
    void now;
    return m.dashboard_last_refreshed_ago({ time: timeAgo(lastRefreshedAt) });
  });

  const intervalOptions = $derived(
    refreshIntervals?.length
      ? refreshIntervals.filter(isValidRefreshInterval)
      : DEFAULT_REFRESH_INTERVALS,
  );

  async function refresh() {
    if (refreshing) return;
    refreshing = true;
    try {
      await invalidateMetricsViewData(queryClient, metricsViewName, false);
      lastRefreshedAt = new Date();
      now = Date.now();
    } finally {
      refreshing = false;
    }
  }

  // In "auto" mode, poll the metrics view's watermark and refresh only when it changes,
  // so dashboards refetch when new data lands instead of on a fixed cadence.
  let lastWatermark: string | undefined;
  async function pollWatermark() {
    try {
      const options = getQueryServiceMetricsViewTimeRangeQueryOptions(
        runtimeClient,
        { metricsViewName },
      );
      const resp = await queryClient.fetchQuery({ ...options, staleTime: 0 });
      const watermark =
        resp.timeRangeSummary?.watermark ?? resp.timeRangeSummary?.max;
      if (lastWatermark !== undefined && watermark !== lastWatermark) {
        await refresh();
      }
      lastWatermark = watermark;
    } catch {
      // Ignore polling errors and retry on the next tick
    }
  }

  $effect(() => {
    lastWatermark = undefined;
    if (selected === REFRESH_INTERVAL_AUTO) {
      // Capture the baseline watermark immediately so the first poll tick can already detect changes
      void pollWatermark();
      const timer = setInterval(
        () => void pollWatermark(),
        AUTO_REFRESH_POLL_INTERVAL_MS,
      );
      return () => clearInterval(timer);
    }
    const ms = refreshIntervalToMs(selected);
    if (ms === undefined) return;
    const timer = setInterval(() => void refresh(), ms);
    return () => clearInterval(timer);
  });

  function labelFor(interval: string): string {
    if (interval === REFRESH_INTERVAL_OFF) return m.dashboard_refresh_off();
    if (interval === REFRESH_INTERVAL_AUTO) return m.dashboard_refresh_auto();
    return interval;
  }
</script>

<div class="flex flex-col items-end gap-y-0.5">
  <div class="wrapper">
    <button
      type="button"
      class="refresh-button"
      aria-label={m.dashboard_refresh()}
      title={m.dashboard_refresh()}
      onclick={() => void refresh()}
    >
      <span class={["flex-none", refreshing && "animate-spin"]}>
        <RefreshIcon size="14px" />
      </span>
      <span class="hidden lg:inline">{m.dashboard_refresh()}</span>
    </button>

    <DropdownMenu.Root bind:open>
      <DropdownMenu.Trigger>
        {#snippet child({ props })}
          <button
            {...props}
            type="button"
            class="interval-button"
            aria-label={m.dashboard_refresh_interval_selector()}
          >
            {#if selected !== REFRESH_INTERVAL_OFF}
              {labelFor(selected)}
            {/if}
            <span
              class="flex-none transition-transform"
              class:-rotate-180={open}
            >
              <CaretDownIcon />
            </span>
          </button>
        {/snippet}
      </DropdownMenu.Trigger>

      <DropdownMenu.Content align="end" class="w-44">
        <DropdownMenu.CheckboxItem
          checkRight
          closeOnSelect
          checked={selected === REFRESH_INTERVAL_OFF}
          onSelect={() => onSelect(REFRESH_INTERVAL_OFF)}
        >
          {m.dashboard_refresh_off()}
        </DropdownMenu.CheckboxItem>
        <DropdownMenu.CheckboxItem
          checkRight
          closeOnSelect
          checked={selected === REFRESH_INTERVAL_AUTO}
          onSelect={() => onSelect(REFRESH_INTERVAL_AUTO)}
        >
          <div class="flex flex-col">
            <span>{m.dashboard_refresh_auto()}</span>
            <span class="text-xs text-fg-muted">
              {m.dashboard_refresh_auto_hint()}
            </span>
          </div>
        </DropdownMenu.CheckboxItem>
        <DropdownMenu.Separator />
        {#each intervalOptions as interval (interval)}
          <DropdownMenu.CheckboxItem
            checkRight
            closeOnSelect
            checked={selected === interval}
            onSelect={() => onSelect(interval)}
          >
            {interval}
          </DropdownMenu.CheckboxItem>
        {/each}
      </DropdownMenu.Content>
    </DropdownMenu.Root>
  </div>

  <span class="text-[11px] leading-none text-fg-muted">
    {lastRefreshedLabel}
  </span>
</div>

<style lang="postcss">
  .wrapper {
    @apply flex flex-none items-stretch;
    @apply rounded-full border border-gray-300 bg-surface-background overflow-hidden;
  }

  .wrapper button {
    @apply flex items-center gap-x-1 px-2 text-fg-secondary;
    @apply text-xs font-medium h-[26px];
  }

  .wrapper button:hover {
    @apply bg-gray-100;
  }

  .refresh-button {
    @apply border-r border-gray-300;
  }
</style>
