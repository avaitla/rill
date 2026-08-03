import { goto } from "$app/navigation";
import { getFiltersForOtherDimensions } from "@rilldata/web-common/features/dashboards/selectors";
import {
  createAndExpression,
  createInExpression,
} from "@rilldata/web-common/features/dashboards/stores/filter-utils";
import { generateExploreLink } from "@rilldata/web-common/features/explore-mappers/generate-explore-link";
import { DashboardState_ActivePage } from "@rilldata/web-common/proto/gen/rill/ui/v1/dashboard_pb";
import type { V1Expression } from "@rilldata/web-common/runtime-client";
import type { RuntimeClient } from "@rilldata/web-common/runtime-client/v2";
import type { DashboardTimeControls } from "@rilldata/web-common/lib/time/types";

/**
 * Navigates to the explore's Logs view with the clicked dimension value applied
 * as a filter, carrying the current filters, time range and timezone.
 */
export async function gotoViewRows(
  client: RuntimeClient,
  exploreName: string,
  dimensionName: string,
  dimensionValue: string,
  state:
    | {
        whereFilter?: V1Expression;
        dimensionsWithInlistFilter?: string[];
        selectedTimeRange?: DashboardTimeControls;
        selectedTimezone?: string;
      }
    | undefined,
  organization?: string,
  project?: string,
) {
  const otherFilters = state?.whereFilter
    ? getFiltersForOtherDimensions(state.whereFilter, dimensionName)
    : undefined;
  const exploreLink = await generateExploreLink(
    client,
    {
      whereFilter: createAndExpression([
        ...(otherFilters?.cond?.exprs ?? []),
        createInExpression(dimensionName, [dimensionValue]),
      ]),
      dimensionsWithInlistFilter:
        state?.dimensionsWithInlistFilter?.filter((d) => d !== dimensionName) ??
        [],
      ...(state?.selectedTimeRange
        ? { selectedTimeRange: state.selectedTimeRange }
        : {}),
      ...(state?.selectedTimezone
        ? { selectedTimezone: state.selectedTimezone }
        : {}),
      activePage: DashboardState_ActivePage.LOGS,
    },
    exploreName,
    organization,
    project,
  );
  await goto(exploreLink);
}
