<script lang="ts">
  import Chart from "@rilldata/web-common/components/icons/Chart.svelte";
  import File from "@rilldata/web-common/components/icons/File.svelte";
  import Pivot from "@rilldata/web-common/components/icons/Pivot.svelte";
  import Tag from "@rilldata/web-common/components/tag/Tag.svelte";
  import { ExploreStateURLParams } from "@rilldata/web-common/features/dashboards/url-state/url-params";
  import { behaviourEvent } from "@rilldata/web-common/metrics/initMetrics";
  import { BehaviourEventMedium } from "@rilldata/web-common/metrics/service/BehaviourEventTypes";
  import {
    MetricsEventScreenName,
    MetricsEventSpace,
  } from "@rilldata/web-common/metrics/service/MetricsTypes";
  import { m } from "@rilldata/web-common/lib/i18n/gen/messages";
  import type { ComponentType } from "svelte";
  import Tab from "./Tab.svelte";

  type TabName =
    | MetricsEventScreenName.Pivot
    | MetricsEventScreenName.Explore
    | MetricsEventScreenName.Logs;
  type TabData = { label: string; Icon: ComponentType; beta?: true };

  const tabs = new Map<TabName, TabData>([
    [
      MetricsEventScreenName.Explore,
      {
        label: m.dashboard_explore(),
        Icon: Chart,
      },
    ],
    [
      MetricsEventScreenName.Pivot,
      {
        label: m.dashboard_pivot(),
        Icon: Pivot,
      },
    ],
    [
      MetricsEventScreenName.Logs,
      {
        label: m.dashboard_logs(),
        Icon: File,
      },
    ],
  ]);

  export let hidePivot: boolean = false;
  export let showLogsTab: boolean = false;
  export let exploreName: string;
  export let onPivot = false;
  export let onLogs = false;

  $: currentTab = onLogs
    ? MetricsEventScreenName.Logs
    : onPivot
      ? MetricsEventScreenName.Pivot
      : MetricsEventScreenName.Explore;

  async function handleTabChange(tab: MetricsEventScreenName) {
    // We do not have behaviour events in cloud
    await behaviourEvent?.fireNavigationEvent(
      exploreName,
      BehaviourEventMedium.Tab,
      MetricsEventSpace.Workspace,
      MetricsEventScreenName.Dashboard,
      tab,
    );
  }
</script>

<div class="mr-4">
  <div class="flex gap-x-2">
    {#each tabs as [tab, { label, Icon, beta }] (tab)}
      {#if (tab === MetricsEventScreenName.Explore) || (tab === MetricsEventScreenName.Pivot && !hidePivot) || (tab === MetricsEventScreenName.Logs && showLogsTab)}
        {@const selected = tab === currentTab}
        <Tab
          theme
          {selected}
          href="?{ExploreStateURLParams.WebView}={tab}"
          onclick={() => handleTabChange(tab)}
        >
          <Icon />
          <div class="flex gap-x-1 items-center group">
            {label}
            {#if beta}
              <Tag height={18} color={selected ? "blue" : "gray"}>BETA</Tag>
            {/if}
          </div>
        </Tab>
      {/if}
    {/each}
  </div>
</div>
