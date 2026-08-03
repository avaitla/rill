import { goto } from "$app/navigation";
import type { MetricsViewSpecDimensionValueLink } from "@rilldata/web-common/runtime-client";
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
 * Navigates to the explore dashboard named by an explore-type dimension link,
 * with the clicked dimension value applied as a filter.
 */
export async function gotoDrillThroughExplore(
  client: RuntimeClient,
  targetExploreName: string,
  dimensionName: string,
  dimensionValue: string,
  organization?: string,
  project?: string,
) {
  const exploreLink = await generateExploreLink(
    client,
    {
      whereFilter: createAndExpression([
        createInExpression(dimensionName, [dimensionValue]),
      ]),
      dimensionsWithInlistFilter: [],
    },
    targetExploreName,
    organization,
    project,
  );
  await goto(exploreLink);
}
