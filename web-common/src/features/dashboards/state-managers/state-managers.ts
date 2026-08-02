import { type ExploreState } from "@rilldata/web-common/features/dashboards/stores/explore-state";
import { getDefaultExplorePreset } from "@rilldata/web-common/features/dashboards/url-state/getDefaultExplorePreset";
import {
  ResourceKind,
  useResource,
} from "@rilldata/web-common/features/entity-management/resource-selectors";
import { type ExploreValidSpecResponse } from "@rilldata/web-common/features/explores/selectors";
import {
  type V1ExplorePreset,
  type V1MetricsViewTimeRangeResponse,
} from "@rilldata/web-common/runtime-client";
import type { RuntimeClient } from "@rilldata/web-common/runtime-client/v2";
import { createRuntimeServiceGetExplore } from "@rilldata/web-common/runtime-client";
import { createQueryServiceMetricsViewTimeRange } from "@rilldata/web-common/runtime-client";
import type { QueryClient, QueryObserverResult } from "@tanstack/svelte-query";
import { getContext } from "svelte";
import {
  type Readable,
  type Writable,
  derived,
  get,
  writable,
} from "svelte/store";
import {
  type MetricsExplorerStoreType,
  metricsExplorerStore,
  updateMetricsExplorerByName,
  useExploreState,
} from "web-common/src/features/dashboards/stores/dashboard-stores";
import { type StateManagerActions, createStateManagerActions } from "./actions";
import type { DashboardCallbackExecutor } from "./actions/types";
import {
  type StateManagerReadables,
  createStateManagerReadables,
} from "./selectors";
import {
  contextColWidthDefaults,
  type ContextColWidths,
} from "../leaderboard-context-column";

export type StateManagers = {
  runtimeClient: RuntimeClient;
  metricsViewName: Writable<string>;
  exploreName: Writable<string>;
  metricsStore: Readable<MetricsExplorerStoreType>;
  dashboardStore: Readable<ExploreState>;
  timeDimension: Writable<string | undefined>;
  timeRangeSummaryStore: Readable<
    QueryObserverResult<V1MetricsViewTimeRangeResponse, unknown>
  >;
  validSpecStore: Readable<
    QueryObserverResult<ExploreValidSpecResponse, Error>
  >;
  queryClient: QueryClient;
  updateDashboard: DashboardCallbackExecutor;
  /**
   * A collection of Readables that can be used to select data from the dashboard.
   */
  selectors: StateManagerReadables;
  /**
   * A collection of functions that update the dashboard data model.
   */
  actions: StateManagerActions;
  /**
   * Store to track the width of the context columns in leaderboards.
   * FIXME: this was implemented as a low-risk fix for in advance of
   * the new branding release 2024-01-31, but should be revisted since
   * it's a one-off solution that introduces another new pattern.
   */
  contextColumnWidths: Writable<ContextColWidths>;
  defaultExploreState: Readable<V1ExplorePreset>;
};

export const DEFAULT_STORE_KEY = Symbol("state-managers");

export function getStateManagers(): StateManagers {
  return getContext(DEFAULT_STORE_KEY);
}

export function createStateManagers({
  queryClient,
  metricsViewName,
  exploreName,
  runtimeClient,
}: {
  queryClient: QueryClient;
  metricsViewName: string;
  exploreName: string;
  runtimeClient: RuntimeClient;
}): StateManagers {
  const primaryMetricsViewNameStore = writable(metricsViewName);
  const exploreNameStore = writable(exploreName);
  const timeDimension = writable<string | undefined>(undefined);

  const dashboardStore: Readable<ExploreState> = derived(
    [exploreNameStore],
    ([name], set) => {
      const exploreState = useExploreState(name);
      return exploreState.subscribe(set);
    },
  );

  const primaryValidSpecStore: Readable<
    QueryObserverResult<ExploreValidSpecResponse, Error>
  > = derived([exploreNameStore], ([exploreName], set) =>
    createRuntimeServiceGetExplore(
      runtimeClient,
      { name: exploreName },
      {
        query: {
          select: (data) =>
            <ExploreValidSpecResponse>{
              explore: data.explore?.explore?.state?.validSpec,
              metricsView: data.metricsView?.metricsView?.state?.validSpec,
            },
          enabled: !!exploreName,
        },
      },
      queryClient,
    ).subscribe(set),
  );

  // When the user selects a table option, swap in the valid spec of the variant metrics view backing
  // that table so all dashboard queries and field lists follow the selection.
  // The primary's table options are carried over so the table selector stays populated,
  // and metricsViewNameStore is kept in sync so queries target the selected variant.
  const validSpecStore: Readable<
    QueryObserverResult<ExploreValidSpecResponse, Error>
  > = derived(
    [primaryValidSpecStore, dashboardStore],
    ([primary, exploreState], set) => {
      const primaryName = primary.data?.explore?.metricsView ?? metricsViewName;
      const option = exploreState?.selectedTableOption
        ? primary.data?.metricsView?.tableOptions?.find(
            (o) => o.table === exploreState.selectedTableOption,
          )
        : undefined;
      if (!option?.metricsView || option.metricsView === primaryName) {
        set(primary);
        return;
      }
      return useResource<V1MetricsViewSpec | undefined>(
        runtimeClient,
        option.metricsView,
        ResourceKind.MetricsView,
        {
          select: (data) => data?.resource?.metricsView?.state?.validSpec,
        },
        queryClient,
      ).subscribe((variant) => {
        if (!variant.data) {
          // Keep serving the primary spec while the variant loads.
          set(primary);
          return;
        }
        set(<QueryObserverResult<ExploreValidSpecResponse, Error>>{
          ...primary,
          data: <ExploreValidSpecResponse>{
            explore: primary.data?.explore,
            metricsView: {
              ...variant.data,
              tableOptions: primary.data?.metricsView?.tableOptions,
            },
          },
        });
      });
    },
  );

  // The metrics view name backing all dashboard queries. Reads resolve to the variant metrics view
  // when a table option is selected; writes update the primary name (used by visual editing).
  const effectiveMetricsViewNameStore: Readable<string> = derived(
    [primaryMetricsViewNameStore, primaryValidSpecStore, dashboardStore],
    ([primaryNameFallback, primary, exploreState]) => {
      const primaryName =
        primary.data?.explore?.metricsView ?? primaryNameFallback;
      const option = exploreState?.selectedTableOption
        ? primary.data?.metricsView?.tableOptions?.find(
            (o) => o.table === exploreState.selectedTableOption,
          )
        : undefined;
      return option?.metricsView || primaryName;
    },
  );
  const metricsViewNameStore: Writable<string> = {
    subscribe: effectiveMetricsViewNameStore.subscribe,
    set: primaryMetricsViewNameStore.set,
    update: primaryMetricsViewNameStore.update,
  };

  const timeRangeSummaryStore: Readable<
    QueryObserverResult<V1MetricsViewTimeRangeResponse, unknown>
  > = derived(
    [metricsViewNameStore, validSpecStore, dashboardStore],
    ([mvName, validSpec, $dashboardStore], set) =>
      createQueryServiceMetricsViewTimeRange(
        runtimeClient,
        {
          metricsViewName: mvName,
          timeDimension: $dashboardStore?.selectedTimeDimension,
        },
        {
          query: {
            enabled: !!validSpec?.data?.metricsView?.timeDimension,
            staleTime: Infinity,
            gcTime: Infinity,
          },
        },
        queryClient,
      ).subscribe(set),
  );

  const updateDashboard = (callback: (exploreState: ExploreState) => void) => {
    const name = get(dashboardStore).name;

    // TODO: Remove dependency on MetricsExplorerStore singleton and its exports
    updateMetricsExplorerByName(name, callback);
  };

  const contextColumnWidths = writable<ContextColWidths>(
    contextColWidthDefaults,
  );

  const defaultExploreState = derived(
    [validSpecStore, timeRangeSummaryStore],
    ([validSpec, timeRangeSummary]) => {
      if (!validSpec.data?.explore) {
        return {};
      }
      return getDefaultExplorePreset(
        validSpec.data?.explore ?? {},
        validSpec.data.metricsView ?? {},
        timeRangeSummary.data?.timeRangeSummary,
      );
    },
  );

  return {
    runtimeClient,
    metricsViewName: metricsViewNameStore,
    exploreName: exploreNameStore,
    metricsStore: metricsExplorerStore,
    timeRangeSummaryStore,
    validSpecStore,
    queryClient,
    dashboardStore,
    timeDimension,
    updateDashboard,
    /**
     * A collection of Readables that can be used to select data from the dashboard.
     */
    selectors: createStateManagerReadables({
      dashboardStore,
      validSpecStore,
      timeRangeSummaryStore,
      queryClient,
    }),
    /**
     * A collection of functions that update the dashboard data model.
     */
    actions: createStateManagerActions({
      updateDashboard,
    }),
    contextColumnWidths,
    defaultExploreState,
  };
}
