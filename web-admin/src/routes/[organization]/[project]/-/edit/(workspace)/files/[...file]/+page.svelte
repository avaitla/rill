<script lang="ts">
  import { afterNavigate, goto } from "$app/navigation";
  import { page } from "$app/stores";
  import WorkspaceDispatcher from "@rilldata/web-common/features/workspaces/WorkspaceDispatcher.svelte";
  import { applyViewSearchParam } from "@rilldata/web-common/layout/workspace/workspace-stores";
  import { useRuntimeClient } from "@rilldata/web-common/runtime-client/v2";
  import type { PageData } from "./$types";

  let { data }: { data: PageData } = $props();

  const client = useRuntimeClient();

  let { fileArtifact } = $derived(data);

  // Fetch file content reactively once the runtime is available.
  // Unlike web-local, the runtime credentials aren't ready during +page.ts load.
  // Data files like .parquet have no editable text content and are rendered as
  // a data preview instead, so skip fetching their (binary) content.
  $effect(() => {
    if (
      client.host &&
      client.instanceId &&
      fileArtifact &&
      !fileArtifact.isPreviewableDataFile
    ) {
      void fileArtifact.fetchContent();
    }
  });

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
