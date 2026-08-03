<script lang="ts">
  import { afterNavigate, goto } from "$app/navigation";
  import { page } from "$app/stores";
  import WorkspaceDispatcher from "@rilldata/web-common/features/workspaces/WorkspaceDispatcher.svelte";
  import { applyViewSearchParam } from "@rilldata/web-common/layout/workspace/workspace-stores";
  import type { PageData } from "./$types";

  export let data: PageData;

  $: ({ fileArtifact } = data);

  // Apply a workspace `?view=` param on real navigations (afterNavigate never
  // fires for hover preloads, unlike `load`). Unknown values are left in the
  // URL for embedded consumers (e.g. the dashboard preview's web view).
  afterNavigate(() => {
    const clean = applyViewSearchParam($page.url, fileArtifact.path);
    if (clean) {
      void goto(clean, { replaceState: true, keepFocus: true, noScroll: true });
    }
  });
</script>

<WorkspaceDispatcher {fileArtifact} />
