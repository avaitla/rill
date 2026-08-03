import { goto } from "$app/navigation";
import { getFiltersForOtherDimensions } from "@rilldata/web-common/features/dashboards/selectors";
import type { DashboardTimeControls } from "@rilldata/web-common/lib/time/types";
import type {
  MetricsViewSpecDimensionValueLink,
  V1Expression,
} from "@rilldata/web-common/runtime-client";
import {
  createAndExpression,
  createInExpression,
} from "@rilldata/web-common/features/dashboards/stores/filter-utils";
import { generateExploreLink } from "@rilldata/web-common/features/explore-mappers/generate-explore-link";
import type { RuntimeClient } from "@rilldata/web-common/runtime-client/v2";

/**
 * Resolves a dimension value link's URL template by replacing "{{ value }}"
 * placeholders with the URL-encoded dimension value.
 */
export function resolveDimensionValueLink(
  urlTemplate: string,
  dimensionValue: unknown,
): string {
  return urlTemplate.replace(
    /\{\{\s*value\s*\}\}/g,
    encodeURIComponent(String(dimensionValue ?? "")),
  );
}

/**
 * Opens a dimension value link in a new tab.
 */
export function openDimensionValueLink(
  link: MetricsViewSpecDimensionValueLink,
  dimensionValue: unknown,
) {
  if (!link.url) return;
  window.open(
    resolveDimensionValueLink(link.url, dimensionValue),
    "_blank",
    "noopener,noreferrer",
  );
}

/**
 * Dashboard context carried through an explore drill-through so the target
 * dashboard continues from where the user was: active filters, time range
 * (incl. grain) and timezone.
 */
export interface DrillThroughContext {
  whereFilter?: V1Expression;
  dimensionsWithInlistFilter?: string[];
  selectedTimeRange?: DashboardTimeControls;
  selectedTimezone?: string;
  /** Target web view on the destination explore (e.g. the Logs view for "View rows"). */
  activePage?: number;
}

/**
 * Extracts the carry-over context from an explore's state.
 */
export function pickDrillThroughContext(
  state:
    | {
        whereFilter?: V1Expression;
        dimensionsWithInlistFilter?: string[];
        selectedTimeRange?: DashboardTimeControls;
        selectedTimezone?: string;
      }
    | undefined,
): DrillThroughContext | undefined {
  if (!state) return undefined;
  return {
    whereFilter: state.whereFilter,
    dimensionsWithInlistFilter: state.dimensionsWithInlistFilter,
    selectedTimeRange: state.selectedTimeRange,
    selectedTimezone: state.selectedTimezone,
  };
}

/**
 * Navigates to the explore dashboard named by an explore-type dimension link,
 * with the clicked dimension value applied as a filter. Existing filters, the
 * active time range/grain and the timezone are carried over; a pre-existing
 * filter on the clicked dimension is replaced by the clicked value.
 */
export async function gotoDrillThroughExplore(
  client: RuntimeClient,
  targetExploreName: string,
  dimensionName: string,
  dimensionValue: string,
  context?: DrillThroughContext,
  organization?: string,
  project?: string,
) {
  const otherFilters = context?.whereFilter
    ? getFiltersForOtherDimensions(context.whereFilter, dimensionName)
    : undefined;
  const exploreLink = await generateExploreLink(
    client,
    {
      whereFilter: createAndExpression([
        ...(otherFilters?.cond?.exprs ?? []),
        createInExpression(dimensionName, [dimensionValue]),
      ]),
      dimensionsWithInlistFilter:
        context?.dimensionsWithInlistFilter?.filter(
          (d) => d !== dimensionName,
        ) ?? [],
      ...(context?.selectedTimeRange
        ? { selectedTimeRange: context.selectedTimeRange }
        : {}),
      ...(context?.selectedTimezone
        ? { selectedTimezone: context.selectedTimezone }
        : {}),
      ...(context?.activePage ? { activePage: context.activePage } : {}),
    },
    targetExploreName,
    organization,
    project,
  );
  await goto(exploreLink);
}
