<script lang="ts">
  import * as DropdownMenu from "@rilldata/web-common/components/dropdown-menu";
  import CaretDownIcon from "@rilldata/web-common/components/icons/CaretDownIcon.svelte";
  import { m } from "@rilldata/web-common/lib/i18n/gen/messages";
  import type { MetricsViewSpecTableOption } from "@rilldata/web-common/runtime-client";

  interface Props {
    /** Selectable tables, from the metrics view's `table_options`. The first entry is the default. */
    options: MetricsViewSpecTableOption[];
    /** Table currently backing the dashboard. */
    selected: string;
    onSelect: (table: string) => void;
  }

  let { options, selected, onSelect }: Props = $props();

  let open = $state(false);
</script>

<DropdownMenu.Root bind:open>
  <DropdownMenu.Trigger>
    {#snippet child({ props })}
      <button
        {...props}
        type="button"
        class="table-selector"
        aria-label={m.dashboard_table_selector()}
      >
        <span class="text-fg-muted">{m.dashboard_table()}</span>
        {selected}
        <span class="flex-none transition-transform" class:-rotate-180={open}>
          <CaretDownIcon />
        </span>
      </button>
    {/snippet}
  </DropdownMenu.Trigger>

  <DropdownMenu.Content align="end" class="w-56">
    {#each options as option (option.table)}
      <DropdownMenu.CheckboxItem
        checkRight
        closeOnSelect
        checked={selected === option.table}
        onSelect={() => onSelect(option.table ?? "")}
      >
        {option.table}
      </DropdownMenu.CheckboxItem>
    {/each}
  </DropdownMenu.Content>
</DropdownMenu.Root>

<style lang="postcss">
  .table-selector {
    @apply flex flex-none items-center gap-x-1 px-2 h-[26px];
    @apply rounded-full border border-gray-300 bg-surface-background;
    @apply text-xs font-medium text-fg-secondary;
  }

  .table-selector:hover {
    @apply bg-gray-100;
  }
</style>
