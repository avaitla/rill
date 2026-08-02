import { goto } from "$app/navigation";
import {
  createAndExpression,
  createInExpression,
} from "@rilldata/web-common/features/dashboards/stores/filter-utils";
import { generateExploreLink } from "@rilldata/web-common/features/explore-mappers/generate-explore-link";
import type { RuntimeClient } from "@rilldata/web-common/runtime-client/v2";

/**
 * Navigates to a dimension's `drill_through` explore dashboard,
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
