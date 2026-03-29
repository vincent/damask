<script lang="ts">
  interface Props {
    label: string
    onremove?: () => void
    color?: string
  }

  let { label, onremove, color }: Props = $props()
</script>

<span
  class="inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-xs font-medium
    bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-200"
  style={color ? `background-color: ${color}20; color: ${color}` : ''}
>
  {label}
  {#if onremove}
    <button
      type="button"
      onclick={onremove}
      class="ml-0.5 rounded-full p-0.5 hover:bg-black/10 dark:hover:bg-white/10 focus:outline-none"
      aria-label="Remove {label}"
    >
      <svg class="h-3 w-3" viewBox="0 0 12 12" fill="currentColor">
        <path d="M6 4.586L9.293 1.293a1 1 0 111.414 1.414L7.414 6l3.293 3.293a1 1 0 01-1.414 1.414L6 7.414l-3.293 3.293a1 1 0 01-1.414-1.414L4.586 6 1.293 2.707A1 1 0 012.707 1.293L6 4.586z" />
      </svg>
    </button>
  {/if}
</span>
